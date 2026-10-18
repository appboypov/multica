package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestDeciderSquadQuestionLabelMigration(t *testing.T) {
	migration, err := os.ReadFile("../../migrations/791_decider_question_labels.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, duplicate := range []bool{false, true} {
		for _, state := range []string{"unanswered", "automatic", "answered"} {
			t.Run(fmt.Sprintf("duplicate=%t/%s", duplicate, state), func(t *testing.T) {
				s, f, _, backendLead, settings := deciderFixture(t, "", 0.96)
				backend := settings.Rules[0].SquadByKind["backend"]
				var runtime string
				f.QueryRow(t, `SELECT runtime_id FROM agent WHERE id=$1`, backendLead).Scan(&runtime)
				designLead := f.Agent(t, "Design lead", runtime)
				name := "Design"
				if duplicate {
					name = "Backend"
				}
				design := f.Squad(t, name, designLead, testutil.Cols{"id": "aaaaaaaa-1111-4111-8111-111111111111", "description": "Designs screens"})
				backendLabel, designLabel := "Backend", name
				if duplicate {
					backendLabel += " (" + backend[:8] + ")"
					designLabel += " (" + design[:8] + ")"
				}
				s.Judge = fakeDeciderJudge(t, backend, 0.96)
				settings.Rules = []DeciderRule{{ID: "todo-squad", Event: "status_entered", Status: "todo", Decision: "squad", Question: "Which squad takes this work?", Reasons: map[string]string{"urgent": "The work fits this squad", "minor": "Another squad fits better"}}}
				count := 1
				if state == "automatic" {
					count = 5
				} else {
					settings.StartPeriod = nil
				}
				setDeciderPolicy(t, f, settings)
				ctx := context.Background()
				var issue string
				for i := range count {
					issue = f.Issue(t, fmt.Sprintf("Squad work %d", i), testutil.Cols{"status": "todo"})
					if err := s.Handle(ctx, util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(issue), "status_entered"); err != nil {
						t.Fatal(err)
					}
				}
				id := routineQuestion(t, f, issue)
				if state == "answered" {
					answerRoutine(t, s, f, id, designLabel, "Choose design")
					// The label mapping belongs to the snapshot, not today's squad names.
					f.Exec(t, `UPDATE squad SET name='Renamed squad' WHERE id=$1`, design)
				}

				// Persist the full-UUID labels used before the readable-label cutover.
				backendFull := "Backend (" + backend + ")"
				designFull := name + " (" + design + ")"
				options, err := json.Marshal([]string{designFull, backendFull})
				if err != nil {
					t.Fatal(err)
				}
				f.Exec(t, `UPDATE question SET settings=jsonb_set(settings,'{options}',$2::jsonb), suggested_answer=jsonb_build_object('option',$3::text) WHERE id=$1`, id, options, backendFull)
				f.Exec(t, `UPDATE question_answer SET value=jsonb_set(value,'{option}',to_jsonb($2::text)) WHERE question_id=$1`, id, designFull)
				f.Cleanup(t, `DELETE FROM question_answer WHERE question_id=$1`, id)
				f.Cleanup(t, `DELETE FROM question_answerer WHERE question_id=$1`, id)
				f.Cleanup(t, `DELETE FROM question_link WHERE question_id=$1`, id)
				f.Cleanup(t, `DELETE FROM question_event WHERE question_id=$1`, id)

				var receiptBefore, tasksBefore, answerBefore, questionBefore string
				f.QueryRow(t, `SELECT to_jsonb(d)::text FROM routine_decision d WHERE question_id=$1`, id).Scan(&receiptBefore)
				f.QueryRow(t, `SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY id),'[]'::jsonb)::text FROM agent_task_queue t WHERE issue_id=$1`, issue).Scan(&tasksBefore)
				f.QueryRow(t, `SELECT (to_jsonb(q)-'settings'-'suggested_answer')::text FROM question q WHERE id=$1`, id).Scan(&questionBefore)
				if state == "answered" {
					f.QueryRow(t, `SELECT (to_jsonb(a)-'value')::text FROM question_answer a WHERE question_id=$1`, id).Scan(&answerBefore)
				}

				f.Exec(t, string(migration))

				var receiptAfter, tasksAfter, questionAfter string
				f.QueryRow(t, `SELECT to_jsonb(d)::text FROM routine_decision d WHERE question_id=$1`, id).Scan(&receiptAfter)
				f.QueryRow(t, `SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY id),'[]'::jsonb)::text FROM agent_task_queue t WHERE issue_id=$1`, issue).Scan(&tasksAfter)
				f.QueryRow(t, `SELECT (to_jsonb(q)-'settings'-'suggested_answer')::text FROM question q WHERE id=$1`, id).Scan(&questionAfter)
				if receiptAfter != receiptBefore || tasksAfter != tasksBefore || questionAfter != questionBefore {
					t.Fatal("label migration changed decision identity, owned tasks or question metadata")
				}
				if state == "answered" {
					var answerAfter, selectedLabel string
					f.QueryRow(t, `SELECT (to_jsonb(a)-'value')::text,value->>'option' FROM question_answer a WHERE question_id=$1`, id).Scan(&answerAfter, &selectedLabel)
					if answerAfter != answerBefore || selectedLabel != designLabel {
						t.Fatal("migration changed the saved selection or answer metadata")
					}
				}
				row, err := s.Queries.GetQuestion(ctx, db.GetQuestionParams{ID: id, WorkspaceID: util.MustParseUUID(f.WorkspaceID)})
				if err != nil {
					t.Fatal(err)
				}
				var stored struct {
					Options []string `json:"options"`
				}
				if err := json.Unmarshal(row.Settings, &stored); err != nil {
					t.Fatal(err)
				}
				// Reapplying the cutover leaves already-readable questions and answers intact.
				var valuesBefore, valuesAfter string
				const valuesSQL = `SELECT jsonb_build_object('question',to_jsonb(q),'answers',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM question_answer a WHERE a.question_id=q.id))::text FROM question q WHERE q.id=$1`
				f.QueryRow(t, valuesSQL, id).Scan(&valuesBefore)
				f.Exec(t, string(migration))
				f.QueryRow(t, valuesSQL, id).Scan(&valuesAfter)
				if valuesAfter != valuesBefore {
					t.Fatal("reapplying the migration changed a readable question or answer")
				}

				// Submit the persisted option as a client does, not a rebuilt label.
				answerRoutine(t, s, f, id, stored.Options[0], "Keep design")
				if !reflect.DeepEqual(stored.Options, []string{designLabel, backendLabel}) {
					t.Fatalf("options=%v", stored.Options)
				}
				var suggested struct {
					Option string `json:"option"`
				}
				if err := json.Unmarshal(row.SuggestedAnswer, &suggested); err != nil || suggested.Option != backendLabel {
					t.Fatalf("suggested answer=%s err=%v", row.SuggestedAnswer, err)
				}
				if state == "automatic" {
					if n := f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND squad_id=$2 AND status='cancelled'`, issue, backend); n != 1 {
						t.Fatal("answer did not cancel the automatic decision's owned task")
					}
				}
				var selected string
				var owned pgtype.UUID
				f.QueryRow(t, `SELECT applied_choice,work_task_id FROM routine_decision WHERE question_id=$1`, id).Scan(&selected, &owned)
				if selected != design || !owned.Valid {
					t.Fatalf("selected=%s owned=%v", selected, owned)
				}
				if n := f.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND assignee_type='squad' AND assignee_id=$2`, issue, design); n != 1 {
					t.Fatal("answer did not preserve the selected squad")
				}
				if n := f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE id=$1 AND squad_id=$2 AND agent_id=$3 AND status='queued'`, owned, design, designLead); n != 1 {
					t.Fatal("answer did not preserve the selected squad's task")
				}
				if state == "answered" {
					var unchangedTasks string
					f.QueryRow(t, `SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY id),'[]'::jsonb)::text FROM agent_task_queue t WHERE issue_id=$1`, issue).Scan(&unchangedTasks)
					if unchangedTasks != tasksBefore {
						t.Fatal("note-only answer edit replaced the owned task")
					}
				}

				answerRoutine(t, s, f, id, stored.Options[1], "Choose backend instead")
				if n := f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='cancelled'`, owned); n != 1 {
					t.Fatal("correction did not cancel the decision's owned task")
				}
				if n := f.Count(t, `SELECT count(*) FROM routine_decision d JOIN agent_task_queue t ON t.id=d.work_task_id JOIN issue i ON i.id=d.issue_id WHERE d.question_id=$1 AND d.applied_choice=$2 AND i.assignee_id=$2::uuid AND t.squad_id=$2::uuid AND t.agent_id=$3 AND t.status='queued'`, id, backend, backendLead); n != 1 {
					t.Fatal("edited answer did not select and start the backend squad")
				}
			})
		}
	}
}
