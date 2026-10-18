package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestDeciderUniqueLabelsCutover(t *testing.T) {
	migration, err := os.ReadFile("../../migrations/801_decider_unique_squad_labels.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	const first = "aaaaaaaa-1111-4111-8111-111111111111"
	const second = "aaaaaaaa-2222-4222-8222-222222222222"
	const literal = "bbbbbbbb-3333-4333-8333-333333333333"
	for _, collision := range []string{"prefix", "literal", "full UUID"} {
		for _, state := range []string{"unanswered", "automatic", "answered"} {
			for _, text := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/text=%t", collision, state, text), func(t *testing.T) {
					s, f, _, lead, settings := deciderFixture(t, "", 0.96)
					f.Exec(t, `UPDATE squad SET name='Operations' WHERE id=$1`, settings.Rules[0].SquadByKind["backend"])
					names := map[string]string{first: "Backend", second: "Backend"}
					if collision != "prefix" {
						names[literal] = "Backend (aaaaaaaa-1)"
						names["cccccccc-4444-4444-8444-444444444444"] = "Backend (aaaaaaaa)"
					}
					for id, name := range names {
						f.Squad(t, name, lead, testutil.Cols{"id": id})
					}
					if text {
						for i := range 21 {
							f.Squad(t, fmt.Sprintf("Other %02d", i), lead)
						}
					}
					s.Judge = fakeDeciderJudge(t, first, 0.96)
					settings.Rules = []DeciderRule{{ID: "squad", Event: "status_entered", Status: "todo", Decision: "squad", Question: "Which squad takes this work?", Reasons: map[string]string{"urgent": "The work fits this squad", "minor": "Another squad fits better"}}}
					count := 1
					if state == "automatic" {
						count = 5
					} else {
						settings.StartPeriod = nil
					}
					setDeciderPolicy(t, f, settings)
					ctx := context.Background()
					workspace := util.MustParseUUID(f.WorkspaceID)
					var issue string
					for i := range count {
						issue = f.Issue(t, fmt.Sprintf("Squad work %d", i), testutil.Cols{"status": "todo"})
						if err := s.Handle(ctx, workspace, util.MustParseUUID(issue), "status_entered"); err != nil {
							t.Fatal(err)
						}
					}
					id := routineQuestion(t, f, issue)
					var input deciderInput
					var raw []byte
					f.QueryRow(t, `SELECT input FROM routine_decision WHERE question_id=$1`, id).Scan(&raw)
					if err := json.Unmarshal(raw, &input); err != nil {
						t.Fatal(err)
					}
					field := "option"
					if text {
						field = "text"
					}
					questions := QuestionService{Queries: s.Queries, TxStarter: s.TxStarter, Tasks: s.Tasks, Bus: s.Bus}
					answer := func(label string) {
						t.Helper()
						value, _ := json.Marshal(map[string]string{field: label})
						if _, err := questions.Answer(ctx, workspace, id, QuestionActor{Type: "member", ID: util.MustParseUUID(f.UserID)}, AnswerQuestionInput{Value: value, Note: "Use this squad"}); err != nil {
							t.Fatal(err)
						}
					}
					if state == "answered" {
						answer(input.Rule.choiceLabel(second))
					}
					// Store the ambiguous labels produced by migration 791 and its runtime.
					var ambiguous []string
					for squad, option := range input.Rule.Squads {
						label := option.Name
						if option.Name == "Backend" {
							label += " (" + squad[:8] + ")"
						}
						ambiguous = append(ambiguous, label)
					}
					sort.Strings(ambiguous)
					options, _ := json.Marshal(ambiguous)
					mode := "before"
					if state == "automatic" {
						mode = "after"
					}
					proposed := "Backend (aaaaaaaa)"
					if collision == "full UUID" {
						proposed = "Backend (" + first + ")"
					}
					title := input.Issue.Title + `: "Backend (aaaaaaaa)" squad take work`
					const place = "Project Backend (aaaaaaaa)"
					prompt := deciderPrompt(input.Rule, title, place, proposed, mode)
					wantPrompt := deciderPrompt(input.Rule, title, place, input.Rule.choiceLabel(first), mode)
					if text {
						prompt += "\nSubmit a squad name from the list below.\n" + strings.Join(ambiguous, "\n")
						var labels []string
						for squad := range input.Rule.Squads {
							labels = append(labels, input.Rule.choiceLabel(squad))
						}
						sort.Strings(labels)
						wantPrompt += "\nSubmit a squad name from the list below.\n" + strings.Join(labels, "\n")
					}
					f.Exec(t, `UPDATE question SET prompt=$2 WHERE id=$1`, id, prompt)
					f.Exec(t, `UPDATE todo SET title=$2,body=$3 WHERE question_id=$1`, id, "Confirm routine choice: "+proposed, prompt)
					f.Exec(t, `UPDATE question SET settings=CASE WHEN answer_type='single_choice' THEN jsonb_set(settings,'{options}',$2::jsonb) ELSE settings END, suggested_answer=jsonb_build_object($3::text,'Backend (aaaaaaaa)') WHERE id=$1`, id, options, field)
					f.Exec(t, `UPDATE question_answer SET value=jsonb_build_object($2::text,'Backend (aaaaaaaa)') WHERE question_id=$1`, id, field)
					if state != "unanswered" {
						// Current names must not change a frozen question's mapping.
						f.Exec(t, `UPDATE squad SET name='Renamed' WHERE id=$1`, second)
					}
					f.Cleanup(t, `DELETE FROM question_answer WHERE question_id=$1`, id)
					f.Cleanup(t, `DELETE FROM question_answerer WHERE question_id=$1`, id)
					f.Cleanup(t, `DELETE FROM question_link WHERE question_id=$1`, id)
					f.Cleanup(t, `DELETE FROM question_event WHERE question_id=$1`, id)
					const ownershipSQL = `SELECT jsonb_build_object('decision',(SELECT to_jsonb(d) FROM routine_decision d WHERE question_id=$1),'tasks',(SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM agent_task_queue t WHERE issue_id=$2))::text`
					var before, after string
					const todoMetadataSQL = `SELECT jsonb_agg(to_jsonb(t)-'title'-'body' ORDER BY id)::text FROM todo t WHERE question_id=$1`
					var todoBefore, todoAfter string
					f.QueryRow(t, todoMetadataSQL, id).Scan(&todoBefore)
					f.QueryRow(t, ownershipSQL, id, issue).Scan(&before)
					f.Exec(t, string(migration))
					f.QueryRow(t, ownershipSQL, id, issue).Scan(&after)
					if before != after {
						t.Fatal("cutover changed decision identity or owned tasks")
					}
					f.QueryRow(t, todoMetadataSQL, id).Scan(&todoAfter)
					if todoBefore != todoAfter {
						t.Fatal("cutover changed linked to-do metadata")
					}
					var todoTitle, todoBody string
					f.QueryRow(t, `SELECT title,body FROM todo WHERE question_id=$1`, id).Scan(&todoTitle, &todoBody)
					if todoTitle != "Confirm routine choice: "+input.Rule.choiceLabel(first) || todoBody != wantPrompt {
						t.Fatalf("linked to-do title=%q body=%q; want prompt=%q", todoTitle, todoBody, wantPrompt)
					}
					row, err := s.Queries.GetQuestion(ctx, db.GetQuestionParams{ID: id, WorkspaceID: workspace})
					if err != nil {
						t.Fatal(err)
					}
					if row.Prompt != wantPrompt {
						t.Fatalf("prompt=%q; want %q", row.Prompt, wantPrompt)
					}
					var suggestion map[string]string
					if err := json.Unmarshal(row.SuggestedAnswer, &suggestion); err != nil || suggestion[field] != input.Rule.choiceLabel(first) {
						t.Fatalf("suggestion=%s err=%v", row.SuggestedAnswer, err)
					}
					if state == "answered" {
						var selected string
						f.QueryRow(t, `SELECT value->>$2 FROM question_answer WHERE question_id=$1`, id, field).Scan(&selected)
						if selected != input.Rule.choiceLabel(second) {
							t.Fatalf("saved selection=%q", selected)
						}
						answer(selected)
						f.QueryRow(t, ownershipSQL, id, issue).Scan(&after)
						if before != after {
							t.Fatal("unchanged selection replaced the owned task")
						}
					}
					var offered []string
					if text {
						for squad := range input.Rule.Squads {
							offered = append(offered, input.Rule.choiceLabel(squad))
						}
					} else {
						var stored struct {
							Options []string `json:"options"`
						}
						if err := json.Unmarshal(row.Settings, &stored); err != nil {
							t.Fatal(err)
						}
						offered = stored.Options
					}
					want := make(map[string]string)
					for squad := range input.Rule.Squads {
						want[input.Rule.choiceLabel(squad)] = squad
					}
					for _, label := range offered {
						squad, exists := want[label]
						if !exists {
							t.Fatalf("unmapped option %q", label)
						}
						answer(label)
						if n := f.Count(t, `SELECT count(*) FROM routine_decision d JOIN agent_task_queue t ON t.id=d.work_task_id JOIN issue i ON i.id=d.issue_id WHERE d.question_id=$1 AND d.applied_choice=$2 AND i.assignee_id=$2::uuid AND t.squad_id=$2::uuid AND t.status='queued'`, id, squad); n != 1 {
							t.Fatalf("option %q did not select squad %s", label, squad)
						}
						delete(want, label)
					}
					if len(want) != 0 {
						t.Fatalf("missing squad options: %v", want)
					}
					var questionBefore, questionAfter []byte
					const presentationSQL = `SELECT jsonb_build_object('question',to_jsonb(q),'todos',(SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM todo t WHERE t.question_id=q.id)) FROM question q WHERE q.id=$1`
					f.QueryRow(t, presentationSQL, id).Scan(&questionBefore)
					f.Exec(t, string(migration))
					f.QueryRow(t, presentationSQL, id).Scan(&questionAfter)
					if !reflect.DeepEqual(questionBefore, questionAfter) {
						t.Fatal("reapplying cutover changed the question")
					}
				})
			}
		}
	}
}
