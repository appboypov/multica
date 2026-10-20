package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	questiontype "github.com/multica-ai/multica/server/internal/question"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestDeciderSquadTextQuestionLabelMigration(t *testing.T) {
	migration, err := os.ReadFile("../../migrations/801_decider_unique_squad_labels.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{1, 21} {
		for _, state := range []string{"unanswered", "automatic", "answered"} {
			t.Run(fmt.Sprintf("%d/%s", size, state), func(t *testing.T) {
				s, f, _, lead, settings := deciderFixture(t, "", 0.96)
				var runtime string
				f.QueryRow(t, `SELECT runtime_id FROM agent WHERE id=$1`, lead).Scan(&runtime)
				squads := []string{settings.Rules[0].SquadByKind["backend"]}
				names := []string{"Backend"}
				for i := 1; i < size; i++ {
					name := fmt.Sprintf("Squad %02d", i)
					if i == size-1 {
						name = "Backend"
					}
					squads = append(squads, f.Squad(t, name, lead, testutil.Cols{"id": fmt.Sprintf("%08x-1111-4111-8111-111111111111", i), "description": "Handles this work"}))
					names = append(names, name)
				}
				labels, fullLabels := make([]string, size), make([]string, size)
				for i, squad := range squads {
					labels[i] = names[i]
					if size > 1 && names[i] == "Backend" {
						labels[i] += " (" + squad[:8] + ")"
					}
					fullLabels[i] = names[i] + " (" + squad + ")"
				}
				s.Judge = fakeDeciderJudge(t, squads[size-1], 0.96)
				settings.Rules = []DeciderRule{{ID: "squad", Event: "status_entered", Status: "todo", Decision: "squad", Question: "Which squad takes this work?", Reasons: map[string]string{"urgent": "The work fits this squad", "minor": "Another squad fits better"}}}
				issues := 1
				if state == "automatic" {
					issues = 5
				} else {
					settings.StartPeriod = nil
				}
				setDeciderPolicy(t, f, settings)
				ctx := context.Background()
				workspace := util.MustParseUUID(f.WorkspaceID)
				var issue string
				for i := range issues {
					issue = f.Issue(t, fmt.Sprintf("Squad work %d", i), testutil.Cols{"status": "todo"})
					if err := s.Handle(ctx, workspace, util.MustParseUUID(issue), "status_entered"); err != nil {
						t.Fatal(err)
					}
				}
				id := routineQuestion(t, f, issue)
				questions := QuestionService{Queries: s.Queries, TxStarter: s.TxStarter, Tasks: s.Tasks, Bus: s.Bus}
				actor := QuestionActor{Type: "member", ID: util.MustParseUUID(f.UserID)}
				answer := func(value []byte, note string) {
					t.Helper()
					if _, err := questions.Answer(ctx, workspace, id, actor, AnswerQuestionInput{Value: value, Note: note}); err != nil {
						t.Fatal(err)
					}
				}
				if state == "answered" {
					value, _ := json.Marshal(questiontype.TextAnswerValue{Text: labels[0]})
					answer(value, "Choose backend")
					f.Exec(t, `UPDATE squad SET name='Renamed squad' WHERE id=$1`, squads[0])
				}
				// Recreate persisted text confirmations with full-UUID choices.
				fullChoices := append([]string(nil), fullLabels...)
				sort.Strings(fullChoices)
				prompt := "Confirm the proposed squad.\nSubmit a squad name from the list below.\n" + strings.Join(fullChoices, "\n")
				f.Exec(t, `UPDATE question SET prompt=$2,suggested_answer=jsonb_build_object('text',$3::text) WHERE id=$1`, id, prompt, fullLabels[size-1])
				f.Exec(t, `UPDATE todo SET body=$2 WHERE question_id=$1`, id, prompt)
				f.Exec(t, `UPDATE question_answer SET value=jsonb_set(value,'{text}',to_jsonb($2::text)) WHERE question_id=$1`, id, fullLabels[0])
				f.Cleanup(t, `DELETE FROM question_answer WHERE question_id=$1`, id)
				f.Cleanup(t, `DELETE FROM question_answerer WHERE question_id=$1`, id)
				f.Cleanup(t, `DELETE FROM question_link WHERE question_id=$1`, id)
				f.Cleanup(t, `DELETE FROM question_event WHERE question_id=$1`, id)
				unrelated := f.Task(t, settings.AgentID, testutil.Cols{"issue_id": issue, "runtime_id": runtime, "status": "queued"})
				const identitySQL = `SELECT jsonb_build_object('decision',to_jsonb(d),'issue',(SELECT to_jsonb(i) FROM issue i WHERE i.id=d.issue_id),'tasks',(SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM agent_task_queue t WHERE t.issue_id=d.issue_id),'question',(SELECT to_jsonb(q)-'prompt'-'suggested_answer' FROM question q WHERE q.id=d.question_id),'answers',(SELECT jsonb_agg(to_jsonb(a)-'value' ORDER BY id) FROM question_answer a WHERE a.question_id=d.question_id))::text FROM routine_decision d WHERE question_id=$1`
				var before, after string
				f.QueryRow(t, identitySQL, id).Scan(&before)
				f.Exec(t, string(migration))
				f.QueryRow(t, identitySQL, id).Scan(&after)
				if after != before {
					t.Fatal("migration changed selection identity, owned tasks or metadata")
				}
				row, err := s.Queries.GetQuestion(ctx, db.GetQuestionParams{ID: id, WorkspaceID: workspace})
				if err != nil {
					t.Fatal(err)
				}
				if row.AnswerType != questiontype.TypeText {
					t.Fatalf("answer type=%s", row.AnswerType)
				}
				// Submit the persisted suggestion rather than rebuilding the value.
				var saved []byte
				if state == "answered" {
					f.QueryRow(t, `SELECT value FROM question_answer WHERE question_id=$1`, id).Scan(&saved)
					answer(saved, "Keep backend")
					var tasksUnchanged bool
					f.QueryRow(t, `SELECT work_task_id=(($2::jsonb)->'decision'->>'work_task_id')::uuid FROM routine_decision WHERE question_id=$1`, id, before).Scan(&tasksUnchanged)
					if !tasksUnchanged {
						t.Fatal("note-only edit replaced the owned task")
					}
				}
				var previous pgtype.UUID
				f.QueryRow(t, `SELECT work_task_id FROM routine_decision WHERE question_id=$1`, id).Scan(&previous)
				answer(row.SuggestedAnswer, "Accept the suggestion")
				if n := f.Count(t, `SELECT count(*) FROM routine_decision d JOIN issue i ON i.id=d.issue_id JOIN agent_task_queue t ON t.id=d.work_task_id WHERE d.question_id=$1 AND d.applied_choice=$2 AND i.assignee_id=$2::uuid AND t.squad_id=$2::uuid AND t.agent_id=$3 AND t.status='queued'`, id, squads[size-1], lead); n != 1 {
					t.Fatal("stored suggestion did not preserve the proposed squad and owned task")
				}
				if previous.Valid {
					wantStatus := "queued"
					if state == "answered" && size > 1 {
						wantStatus = "cancelled"
					}
					if n := f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status=$2`, previous, wantStatus); n != 1 {
						t.Fatal("answer did not preserve the owned-task correction rule")
					}
				}
				if n := f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='queued'`, unrelated); n != 1 {
					t.Fatal("answer cancelled unrelated work")
				}
				choices := append([]string(nil), labels...)
				sort.Strings(choices)
				wantPrompt := "Confirm the proposed squad.\nSubmit a squad name from the list below.\n" + strings.Join(choices, "\n")
				if row.Prompt != wantPrompt {
					t.Fatalf("available choices=%s", row.Prompt)
				}
				if n := f.Count(t, `SELECT count(*) FROM todo WHERE question_id=$1 AND body=$2`, id, wantPrompt); n != 1 {
					t.Fatal("todo still displays full-UUID choices")
				}
				const valuesSQL = `SELECT jsonb_build_object('question',to_jsonb(q),'answers',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM question_answer a WHERE a.question_id=q.id),'todos',(SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM todo t WHERE t.question_id=q.id))::text FROM question q WHERE q.id=$1`
				f.QueryRow(t, valuesSQL, id).Scan(&before)
				f.Exec(t, string(migration))
				f.QueryRow(t, valuesSQL, id).Scan(&after)
				if before != after {
					t.Fatal("reapplying the migration changed readable values")
				}
			})
		}
	}
}
