package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	questiontype "github.com/multica-ai/multica/server/internal/question"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestDeciderSquadCorrectionCancelsPreviousTask(t *testing.T) {
	for _, mode := range []string{"answer_edit", "sample"} {
		t.Run(mode, func(t *testing.T) {
			s, f, _, lead, settings := deciderFixture(t, "", 0.96)
			backend := settings.Rules[0].SquadByKind["backend"]
			s.Judge = fakeDeciderJudge(t, backend, 0.96)
			runtime := f.Runtime(t, "Design runtime")
			designLead := f.Agent(t, "Design lead", runtime)
			design := f.Squad(t, "Design", designLead, testutil.Cols{"description": "Designs screens"})
			settings.Rules = []DeciderRule{{ID: "squad", Event: "status_entered", Status: "todo", Decision: "squad", Question: "Which squad takes this work?", Reasons: map[string]string{"urgent": "The work fits this squad", "minor": "Another squad fits better"}}}
			count := 5
			if mode == "answer_edit" {
				settings.StartPeriod = nil
				count = 1
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
			question := routineQuestion(t, f, issue)
			if mode == "answer_edit" {
				answerRoutine(t, s, f, question, fmt.Sprintf("Backend (%s)", backend), "")
			}
			var previous pgtype.UUID
			f.QueryRow(t, `SELECT work_task_id FROM routine_decision WHERE issue_id=$1`, issue).Scan(&previous)
			if mode == "sample" {
				f.Exec(t, `UPDATE agent_task_queue SET status='running',started_at=now() WHERE id=$1`, previous)
				comment := f.Comment(t, issue, "Pending input for the previous leader")
				f.Exec(t, `INSERT INTO task_supplement(task_id,workspace_id,issue_id,comment_id,item_id,author_id,client_request_id,status) VALUES($1,$2,$3,$4,$4,$5,$4,'pending')`, previous, f.WorkspaceID, issue, comment, f.UserID)
				f.Cleanup(t, `DELETE FROM task_supplement WHERE task_id=$1`, previous)
			}
			// The receipt owns this task. Other work for the issue must survive.
			unrelated := f.Task(t, settings.AgentID, testutil.Cols{"issue_id": issue, "runtime_id": runtime, "status": "queued"})
			answerRoutine(t, s, f, question, fmt.Sprintf("Design (%s)", design), "This needs design")
			old, err := s.Queries.GetAgentTask(ctx, previous)
			if err != nil {
				t.Fatal(err)
			}
			if old.Status != "cancelled" {
				t.Fatalf("previous squad task status=%s", old.Status)
			}
			if mode == "sample" {
				if n := f.Count(t, `SELECT count(*) FROM task_supplement WHERE task_id=$1 AND status='failed' AND failure_reason='turn_ended'`, previous); n != 1 {
					t.Fatal("previous task's pending input was not settled")
				}
			}
			if n := f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='queued'`, unrelated); n != 1 {
				t.Fatal("correction cancelled unrelated work")
			}
			current, err := s.Queries.GetIssue(ctx, util.MustParseUUID(issue))
			if err != nil {
				t.Fatal(err)
			}
			if current.AssigneeType.String != "squad" || util.UUIDToString(current.AssigneeID) != design {
				t.Fatal("correction did not assign the design squad")
			}
			if n := f.Count(t, `SELECT count(*) FROM routine_decision d JOIN agent_task_queue t ON t.id=d.work_task_id WHERE d.issue_id=$1 AND t.squad_id=$2 AND t.agent_id=$3 AND t.status='queued'`, issue, design, designLead); n != 1 {
				t.Fatal("receipt does not own the replacement squad task")
			}
			answerRoutine(t, s, f, question, fmt.Sprintf("Backend (%s)", backend), "Return to backend")
			if n := f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND squad_id=$2 AND status='cancelled'`, issue, design); n != 1 {
				t.Fatal("edited correction left the design task active")
			}
			if n := f.Count(t, `SELECT count(*) FROM routine_decision d JOIN agent_task_queue t ON t.id=d.work_task_id WHERE d.issue_id=$1 AND t.squad_id=$2 AND t.agent_id=$3 AND t.status='queued'`, issue, backend, lead); n != 1 {
				t.Fatal("edited correction did not restore backend work")
			}
		})
	}
}

func TestDeciderSameNameSquadsRemainDistinct(t *testing.T) {
	s, f, _, _, settings := deciderFixture(t, "", 0.96)
	first := settings.Rules[0].SquadByKind["backend"]
	runtime := f.Runtime(t, "Other backend runtime")
	lead := f.Agent(t, "Other backend lead", runtime)
	squad := f.Squad(t, "Backend", lead, testutil.Cols{"description": "Builds payment services"})
	s.Judge = fakeDeciderJudge(t, squad, 0.96)
	settings.StartPeriod = nil
	settings.Rules = []DeciderRule{{ID: "squad", Event: "status_entered", Status: "todo", Decision: "squad", Question: "Which squad takes this work?", Reasons: map[string]string{"urgent": "The work fits this squad", "minor": "Another squad fits better"}}}
	setDeciderPolicy(t, f, settings)
	issue := f.Issue(t, "Payment API", testutil.Cols{"status": "todo"})
	ctx := context.Background()
	if err := s.Handle(ctx, util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(issue), "status_entered"); err != nil {
		t.Fatal(err)
	}
	question := routineQuestion(t, f, issue)
	row, err := s.Queries.GetQuestion(ctx, db.GetQuestionParams{ID: question, WorkspaceID: util.MustParseUUID(f.WorkspaceID)})
	if err != nil {
		t.Fatal(err)
	}
	var options questiontype.SingleChoiceAnswerSettings
	if err := json.Unmarshal(row.Settings, &options); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{fmt.Sprintf("Backend (%s)", first): false, fmt.Sprintf("Backend (%s)", squad): false}
	for _, option := range options.Options {
		if _, ok := want[option]; !ok {
			t.Fatalf("unexpected squad option %q", option)
		}
		want[option] = true
	}
	for option, found := range want {
		if !found {
			t.Fatalf("missing distinct squad option %q", option)
		}
	}
	if !strings.Contains(row.Prompt, "Builds server code") || !strings.Contains(row.Prompt, "Builds payment services") {
		t.Fatal("squad descriptions missing from confirmation")
	}
	answerRoutine(t, s, f, question, fmt.Sprintf("Backend (%s)", squad), "")
	current, err := s.Queries.GetIssue(ctx, util.MustParseUUID(issue))
	if err != nil {
		t.Fatal(err)
	}
	if util.UUIDToString(current.AssigneeID) != squad {
		t.Fatal("duplicate name routed to another squad")
	}
	if n := f.Count(t, `SELECT count(*) FROM routine_decision d JOIN agent_task_queue t ON t.id=d.work_task_id WHERE d.issue_id=$1 AND d.applied_choice=$2 AND t.squad_id=$2::uuid AND t.agent_id=$3`, issue, squad, lead); n != 1 {
		t.Fatal("selected squad identity was lost")
	}
	answerRoutine(t, s, f, question, fmt.Sprintf("Backend (%s)", first), "Use the other backend squad")
	if n := f.Count(t, `SELECT count(*) FROM routine_decision d JOIN agent_task_queue t ON t.id=d.work_task_id WHERE d.issue_id=$1 AND d.applied_choice=$2 AND t.squad_id=$2::uuid AND t.status='queued'`, issue, first); n != 1 {
		t.Fatal("same-name correction selected the wrong squad")
	}
}

func TestDeciderSquadConfirmationCounts(t *testing.T) {
	for _, count := range []int{1, 2, 20, 21} {
		for _, mode := range []string{"start", "sample"} {
			t.Run(fmt.Sprintf("%d/%s", count, mode), func(t *testing.T) {
				s, f, _, lead, settings := deciderFixture(t, "", 0.96)
				squads := []string{settings.Rules[0].SquadByKind["backend"]}
				labels := []string{fmt.Sprintf("Backend (%s)", squads[0])}
				for i := 1; i < count; i++ {
					name := fmt.Sprintf("Squad %02d", i)
					id := f.Squad(t, name, lead, testutil.Cols{"description": "Handles this work"})
					squads = append(squads, id)
					labels = append(labels, fmt.Sprintf("%s (%s)", name, id))
				}
				s.Judge = fakeDeciderJudge(t, squads[count-1], 0.96)
				settings.Rules = []DeciderRule{{ID: "squad", Event: "status_entered", Status: "todo", Decision: "squad", Question: "Which squad takes this work?", Reasons: map[string]string{"urgent": "The work fits this squad"}}}
				issues := 5
				if mode == "start" {
					settings.StartPeriod = nil
					issues = 1
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
				if mode == "start" {
					current, err := s.Queries.GetIssue(ctx, util.MustParseUUID(issue))
					if err != nil {
						t.Fatal(err)
					}
					if current.AssigneeID.Valid || f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issue) != 0 {
						t.Fatal("squad work started before confirmation")
					}
				}
				id := routineQuestion(t, f, issue)
				row, err := s.Queries.GetQuestion(ctx, db.GetQuestionParams{ID: id, WorkspaceID: workspace})
				if err != nil {
					t.Fatal(err)
				}
				questions := QuestionService{Queries: s.Queries, TxStarter: s.TxStarter, Tasks: s.Tasks, Bus: s.Bus}
				actor := QuestionActor{Type: "member", ID: util.MustParseUUID(f.UserID)}
				if _, err := questions.Answer(ctx, workspace, id, actor, AnswerQuestionInput{Value: row.SuggestedAnswer}); err != nil {
					t.Fatalf("cannot confirm proposed squad: %v", err)
				}
				for i, squad := range squads {
					if !strings.Contains(row.Prompt, labels[i]) {
						t.Fatalf("eligible squad missing from prompt: %s", labels[i])
					}
					var value []byte
					if row.AnswerType == questiontype.TypeText {
						value, err = json.Marshal(questiontype.TextAnswerValue{Text: labels[i]})
					} else {
						value, err = json.Marshal(questiontype.SingleChoiceAnswerValue{Option: &labels[i]})
					}
					if err != nil {
						t.Fatal(err)
					}
					if _, err := questions.Answer(ctx, workspace, id, actor, AnswerQuestionInput{Value: value, Note: "Use this squad"}); err != nil {
						t.Fatalf("cannot select eligible squad %s: %v", squad, err)
					}
					if n := f.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND assignee_type='squad' AND assignee_id=$2`, issue, squad); n != 1 {
						t.Fatal("selected squad was not assigned")
					}
					if n := f.Count(t, `SELECT count(*) FROM routine_decision d JOIN agent_task_queue t ON t.id=d.work_task_id WHERE d.issue_id=$1 AND d.applied_choice=$2 AND t.squad_id=$2::uuid AND t.status='queued'`, issue, squad); n != 1 {
						t.Fatal("selected squad has no owned leader task")
					}
					if n := f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND status='queued'`, issue); n != 1 {
						t.Fatal("correction left another leader task queued")
					}
				}
				if row.AnswerType == questiontype.TypeText {
					value, _ := json.Marshal(questiontype.TextAnswerValue{Text: "Not an eligible squad"})
					_, err := questions.Answer(ctx, workspace, id, actor, AnswerQuestionInput{Value: value})
					wantQuestionRefusal(t, err, 422)
					if n := f.Count(t, `SELECT count(*) FROM routine_decision WHERE issue_id=$1 AND applied_choice=$2`, issue, squads[count-1]); n != 1 {
						t.Fatal("invalid answer changed the applied squad")
					}
				}
			})
		}
	}
}
