package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func setDeciderPolicy(t *testing.T, f *testutil.Fixture, settings DeciderSettings) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"decider": settings})
	if err != nil { t.Fatal(err) }
	f.Exec(t, `UPDATE workspace SET settings=$2 WHERE id=$1`, f.WorkspaceID, raw)
}

func routineQuestion(t *testing.T, f *testutil.Fixture, issue string) pgtype.UUID {
	t.Helper()
	var id pgtype.UUID
	if err := f.QueryRow(t, `SELECT question_id FROM routine_decision WHERE issue_id=$1`, issue).Scan(&id); err != nil { t.Fatal(err) }
	if !id.Valid { t.Fatal("routine choice has no confirmation question") }
	return id
}

func answerRoutine(t *testing.T, s *DeciderService, f *testutil.Fixture, id pgtype.UUID, choice, note string) {
	t.Helper()
	questions := QuestionService{Queries: s.Queries, TxStarter: s.TxStarter, Tasks: s.Tasks, Bus: s.Bus}
	value, _ := json.Marshal(map[string]string{"option": choice})
	_, err := questions.Answer(context.Background(), util.MustParseUUID(f.WorkspaceID), id, QuestionActor{Type: "member", ID: util.MustParseUUID(f.UserID)}, AnswerQuestionInput{Value: value, Note: note})
	if err != nil { t.Fatal(err) }
}

func TestDeciderStartPeriodConfirmsBeforeApplying(t *testing.T) {
	for _, selected := range []string{"fix_now", "fix_later"} {
		t.Run(selected, func(t *testing.T) {
			s, f, issue, lead, settings := deciderFixture(t, "fix_now", 0.96)
			settings.StartPeriod = nil
			setDeciderPolicy(t, f, settings)
			ctx := context.Background()
			if err := s.Handle(ctx, util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(issue), "status_entered"); err != nil { t.Fatal(err) }
			if n := f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issue); n != 0 { t.Fatalf("work ran before yes: %d", n) }
			if n := f.Count(t, `SELECT count(*) FROM todo WHERE issue_id=$1 AND kind='review'`, issue); n != 0 { t.Fatal("action todo opened before yes") }
			current, err := s.Queries.GetIssue(ctx, util.MustParseUUID(issue))
			if err != nil { t.Fatal(err) }
			if current.Status != "backlog" || current.AssigneeID.Valid { t.Fatal("proposal changed issue") }
			id := routineQuestion(t, f, issue)
			question, err := s.Queries.GetQuestion(ctx, db.GetQuestionParams{ID: id, WorkspaceID: util.MustParseUUID(f.WorkspaceID)})
			if err != nil { t.Fatal(err) }
			if !strings.Contains(question.Prompt, "Jev (confidence 0.960)") || !strings.Contains(question.Prompt, "Proposed choice: fix_now") { t.Fatalf("missing proposal: %s", question.Prompt) }
			answerRoutine(t, s, f, id, selected, "My choice")
			current, err = s.Queries.GetIssue(ctx, util.MustParseUUID(issue))
			if err != nil { t.Fatal(err) }
			wantStatus, verdict, runs := "todo", "agreed", 1
			if selected == "fix_later" { wantStatus, verdict, runs = "backlog", "corrected", 0 }
			if current.Status != wantStatus { t.Fatalf("status=%s", current.Status) }
			if selected == "fix_now" && util.UUIDToString(current.AssigneeID) != lead { t.Fatal("wrong kind lead") }
			if n := f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issue); n != runs { t.Fatalf("runs=%d", n) }
			if n := f.Count(t, `SELECT count(*) FROM routine_decision WHERE issue_id=$1 AND reviews->0->>'verdict'=$2 AND (reviews->0->>'jev_confidence')::float=0.96`, issue, verdict); n != 1 { t.Fatal("missing agreement record") }
			if n := f.Count(t, `SELECT count(*) FROM todo WHERE question_id=$1 AND done_at IS NULL`, id); n != 0 { t.Fatal("answered confirmation still open") }
			answerRoutine(t, s, f, id, selected, "My choice")
			if n := f.Count(t, `SELECT count(*) FROM routine_decision WHERE issue_id=$1 AND jsonb_array_length(reviews)=1`, issue); n != 1 { t.Fatal("duplicate answer appended twice") }
		})
	}
}

func TestDeciderStartPeriodSonnetRetainsJevConfidence(t *testing.T) {
	s, f, issue, _, settings := deciderFixture(t, "fix_now", 0.4)
	settings.StartPeriod = nil
	setDeciderPolicy(t, f, settings)
	ctx := context.Background()
	if err := s.Handle(ctx, util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(issue), "status_entered"); err != nil { t.Fatal(err) }
	var task pgtype.UUID
	if err := f.QueryRow(t, `SELECT task_id FROM routine_decision WHERE issue_id=$1`, issue).Scan(&task); err != nil { t.Fatal(err) }
	payload, _ := json.Marshal(protocol.TaskCompletedPayload{Output: `{"choice":"fix_later","reason":"minor"}`})
	f.Exec(t, `UPDATE agent_task_queue SET status='completed',result=$2 WHERE id=$1`, task, payload)
	if err := s.Complete(ctx, task); err != nil { t.Fatal(err) }
	if n := f.Count(t, `SELECT count(*) FROM todo WHERE issue_id=$1 AND kind='review'`, issue); n != 0 { t.Fatal("Sonnet applied before Brian") }
	id := routineQuestion(t, f, issue)
	question, err := s.Queries.GetQuestion(ctx, db.GetQuestionParams{ID: id, WorkspaceID: util.MustParseUUID(f.WorkspaceID)})
	if err != nil { t.Fatal(err) }
	if !strings.Contains(question.Prompt, "Sonnet 5.5") || !strings.Contains(question.Prompt, "Jev confidence: 0.400") { t.Fatalf("wrong maker/confidence: %s", question.Prompt) }
	answerRoutine(t, s, f, id, "fix_later", "")
	if n := f.Count(t, `SELECT count(*) FROM routine_decision WHERE issue_id=$1 AND reviews->0->>'verdict'='agreed' AND (reviews->0->>'jev_confidence')::float=0.4`, issue); n != 1 { t.Fatal("fallback confidence lost") }
}

func TestDeciderEveryFifthAutomaticChoiceAndCorrection(t *testing.T) {
	s, f, first, lead, _ := deciderFixture(t, "fix_now", 0.96)
	ctx := context.Background()
	base, err := s.Queries.GetIssue(ctx, util.MustParseUUID(first))
	if err != nil { t.Fatal(err) }
	var fifth string
	for i := 1; i <= 10; i++ {
		id := first
		if i > 1 { id = f.Issue(t, fmt.Sprintf("Automatic bug %d", i), testutil.Cols{"status":"backlog", "type":"bug", "properties":string(base.Properties)}) }
		if err := s.Handle(ctx, util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(id), "status_entered"); err != nil { t.Fatal(err) }
		if err := s.Handle(ctx, util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(id), "status_entered"); err != nil { t.Fatal(err) }
		if i == 5 { fifth = id }
		want := 0
		if i%5 == 0 { want = 1 }
		if n := f.Count(t, `SELECT count(*) FROM todo WHERE issue_id=$1 AND kind='question' AND done_at IS NULL`, id); n != want { t.Fatalf("choice %d confirmations=%d, want %d", i, n, want) }
	}
	if n := f.Count(t, `SELECT count(*) FROM routine_decision WHERE workspace_id=$1 AND automatic_number BETWEEN 1 AND 10`, f.WorkspaceID); n != 10 { t.Fatal("automatic sequence is not deterministic") }
	id := routineQuestion(t, f, fifth)
	answerRoutine(t, s, f, id, "fix_later", "Please plan this later")
	current, err := s.Queries.GetIssue(ctx, util.MustParseUUID(fifth))
	if err != nil { t.Fatal(err) }
	if current.Status != "backlog" || current.AssigneeID.Valid { t.Fatal("correction did not restore backlog outcome") }
	if n := f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND agent_id=$2 AND status='cancelled'`, fifth, lead); n != 1 { t.Fatal("receipt's lead task not cancelled") }
	if n := f.Count(t, `SELECT count(*) FROM todo WHERE issue_id=$1 AND kind='review' AND done_at IS NULL`, fifth); n != 1 { t.Fatal("planning todo missing") }
	answerRoutine(t, s, f, id, "fix_now", "Actually fix this now")
	if n := f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND agent_id=$2 AND status='queued'`, fifth, lead); n != 1 { t.Fatal("edited answer did not queue lead") }
	if n := f.Count(t, `SELECT count(*) FROM todo WHERE issue_id=$1 AND kind='review' AND done_at IS NULL`, fifth); n != 0 { t.Fatal("opposite action todo not closed") }
	if n := f.Count(t, `SELECT count(*) FROM routine_decision WHERE issue_id=$1 AND jsonb_array_length(reviews)=2 AND reviews->0->>'verdict'='corrected' AND reviews->1->>'verdict'='agreed'`, fifth); n != 1 { t.Fatal("answer edits lost history") }
}

func TestDeciderConfirmationRefusesOtherActorsAndReassignment(t *testing.T) {
	s, f, issue, _, settings := deciderFixture(t, "fix_now", 0.96)
	settings.StartPeriod = nil
	setDeciderPolicy(t, f, settings)
	ctx := context.Background()
	if err := s.Handle(ctx, util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(issue), "status_entered"); err != nil { t.Fatal(err) }
	id := routineQuestion(t, f, issue)
	other := f.User(t, "Other member", f.WorkspaceID+"-other@example.test")
	f.Member(t, f.WorkspaceID, other, "admin")
	questions := QuestionService{Queries:s.Queries, TxStarter:s.TxStarter, Tasks:s.Tasks, Bus:s.Bus}
	workspace := util.MustParseUUID(f.WorkspaceID)
	_, err := questions.Answer(ctx, workspace, id, QuestionActor{Type:"member", ID:util.MustParseUUID(other)}, AnswerQuestionInput{Value:json.RawMessage(`{"option":"fix_now"}`)})
	wantQuestionRefusal(t, err, 403)
	_, err = questions.Reassign(ctx, workspace, id, QuestionActor{Type:"member", ID:util.MustParseUUID(f.UserID)}, util.MustParseUUID(other), "Please answer")
	wantQuestionRefusal(t, err, 422)
	_, err = questions.Answer(ctx, workspace, id, QuestionActor{Type:"member", ID:util.MustParseUUID(f.UserID)}, AnswerQuestionInput{Value:json.RawMessage(`{"declined":true}`)})
	wantQuestionRefusal(t, err, 422)
	if n := f.Count(t, `SELECT count(*) FROM question_answer WHERE question_id=$1`, id); n != 0 { t.Fatal("refused answer committed") }
	answerRoutine(t, s, f, id, "fix_now", "")
}

func TestDeciderConfirmationStaleAndAtomic(t *testing.T) {
	for _, mode := range []string{"stale", "apply failure"} {
		t.Run(mode, func(t *testing.T) {
			s, f, issue, lead, settings := deciderFixture(t, "fix_now", 0.96)
			settings.StartPeriod = nil
			setDeciderPolicy(t, f, settings)
			ctx := context.Background()
			if err := s.Handle(ctx, util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(issue), "status_entered"); err != nil { t.Fatal(err) }
			id := routineQuestion(t, f, issue)
			if mode == "stale" {
				f.Exec(t, `UPDATE issue SET title='Changed question' WHERE id=$1`, issue)
				answerRoutine(t, s, f, id, "fix_now", "")
				if n := f.Count(t, `SELECT count(*) FROM routine_decision WHERE issue_id=$1 AND outcome='stale' AND jsonb_array_length(reviews)=1`, issue); n != 1 { t.Fatal("stale answer not recorded") }
			} else {
				f.Exec(t, `UPDATE agent SET archived_at=now() WHERE id=$1`, lead)
				questions := QuestionService{Queries:s.Queries, TxStarter:s.TxStarter, Tasks:s.Tasks, Bus:s.Bus}
				_, err := questions.Answer(ctx, util.MustParseUUID(f.WorkspaceID), id, QuestionActor{Type:"member", ID:util.MustParseUUID(f.UserID)}, AnswerQuestionInput{Value:json.RawMessage(`{"option":"fix_now"}`)})
				if err == nil { t.Fatal("unavailable lead accepted") }
				if n := f.Count(t, `SELECT count(*) FROM question_answer WHERE question_id=$1`, id); n != 0 { t.Fatal("answer committed without outcome") }
				if n := f.Count(t, `SELECT count(*) FROM routine_decision WHERE issue_id=$1 AND jsonb_array_length(reviews)=0 AND outcome='awaiting_brian'`, issue); n != 1 { t.Fatal("failed application changed receipt") }
			}
			if n := f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issue); n != 0 { t.Fatal("stale or failed confirmation started work") }
		})
	}
}

func TestDeciderSettingsBrianOnly(t *testing.T) {
	brian := "11111111-1111-4111-8111-111111111111"
	stored := []byte(`{"decider":{"member_id":"`+brian+`"}}`)
	ended := []byte(`{"decider":{"member_id":"`+brian+`","start_period":false,"confidence_threshold":0.9}}`)
	for _, tc := range []struct{name string; before, after []byte; actor, id string; allowed bool}{
		{"Brian ends and tunes",stored,ended,"member",brian,true},
		{"other member",stored,ended,"member","someone",false},
		{"agent impersonates member",stored,ended,"agent",brian,false},
		{"removing policy",stored,[]byte(`{}`),"member","someone",false},
		{"changing Brian",stored,[]byte(`{"decider":{"member_id":"22222222-2222-4222-8222-222222222222"}}`),"member","someone",false},
		{"unrelated setting",stored,[]byte(`{"decider":{"member_id":"`+brian+`"},"extra":true}`),"member","someone",true},
		{"setup defaults",[]byte(`{}`),stored,"member",brian,true},
		{"setup ends period",[]byte(`{}`),ended,"member",brian,false},
		{"setup by someone else",[]byte(`{}`),stored,"member","someone",false},
	} {
		t.Run(tc.name,func(t *testing.T){
			err := AuthorizeDeciderSettings(tc.before,tc.after,tc.actor,tc.id)
			if (err == nil) != tc.allowed { t.Fatalf("allowed=%v err=%v",tc.allowed,err) }
		})
	}
}

func TestDeciderPolicyChangeKeepsPendingConfirmation(t *testing.T) {
	s, f, issue, _, settings := deciderFixture(t, "fix_now", 0.96)
	settings.StartPeriod = nil
	setDeciderPolicy(t,f,settings)
	if err := s.Handle(context.Background(),util.MustParseUUID(f.WorkspaceID),util.MustParseUUID(issue),"status_entered"); err != nil { t.Fatal(err) }
	id := routineQuestion(t,f,issue)
	ended := false
	threshold := 0.9
	settings.StartPeriod,settings.ConfidenceThreshold = &ended,&threshold
	setDeciderPolicy(t,f,settings)
	answerRoutine(t,s,f,id,"fix_now","")
	if n := f.Count(t, `SELECT count(*) FROM routine_decision WHERE issue_id=$1 AND outcome='applied' AND automatic_number IS NULL`, issue); n != 1 { t.Fatal("Brian's pending answer became stale or automatic after policy edit") }
}

func TestDeciderReleaseConfirmationAndCorrection(t *testing.T) {
	for _, mode := range []string{"start", "sample"} {
		t.Run(mode, func(t *testing.T) {
			s, f, issue, _, settings := deciderFixture(t, "ready", 0.96)
			project := f.Project(t, "Release project")
			repo := f.Insert(t, "repository", testutil.Cols{"workspace_id": f.WorkspaceID, "project_id": project, "github_repo_id": 1, "full_name": "test/" + f.WorkspaceID, "private": false, "default_branch": "main", "checkout_daemon_id": "test", "checkout_path": "/tmp/test", "version_policy": "tags", "last_tag": ""})
			release := f.Insert(t, "release", testutil.Cols{"workspace_id": f.WorkspaceID, "repository_id": repo, "position": 0, "version": "1.0.0", "title": "Test release"})
			if mode == "sample" {
				// The sequence counts committed automatic choices across rule kinds.
				s.Judge = fakeDeciderJudge(t, "fix_later", 0.96)
				base, err := s.Queries.GetIssue(context.Background(), util.MustParseUUID(issue))
				if err != nil { t.Fatal(err) }
				for i := range 4 {
					id := f.Issue(t, fmt.Sprintf("Earlier bug %d", i), testutil.Cols{"status":"backlog", "type":"bug", "properties":string(base.Properties)})
					if err := s.Handle(context.Background(),util.MustParseUUID(f.WorkspaceID),util.MustParseUUID(id),"status_entered"); err != nil { t.Fatal(err) }
				}
				s.Judge = fakeDeciderJudge(t, "ready", 0.96)
			} else {
				settings.StartPeriod = nil
			}
			settings.Rules = []DeciderRule{{ID:"release", Event:"release_joined", Decision:"release", Question:"Is this release ready for Brian's check?", Reasons:map[string]string{"urgent":"All work is complete", "minor":"More review is needed"}}}
			setDeciderPolicy(t,f,settings)
			f.Exec(t, `UPDATE issue SET release_id=$2,status='done' WHERE id=$1`, issue, release)
			if err := s.Handle(context.Background(),util.MustParseUUID(f.WorkspaceID),util.MustParseUUID(issue),"release_joined"); err != nil { t.Fatal(err) }
			id := routineQuestion(t,f,issue)
			if mode == "start" {
				if n := f.Count(t, `SELECT count(*) FROM todo WHERE details->>'release_id'=$1 AND done_at IS NULL`, release); n != 0 { t.Fatal("release check opened before Brian's yes") }
				answerRoutine(t,s,f,id,"ready","")
			}
			if n := f.Count(t, `SELECT count(*) FROM todo WHERE details->>'release_id'=$1 AND done_at IS NULL`, release); n != 1 { t.Fatal("release check not open") }
			answerRoutine(t,s,f,id,"not_yet","Needs another review")
			if n := f.Count(t, `SELECT count(*) FROM todo WHERE details->>'release_id'=$1 AND done_at IS NULL`, release); n != 0 { t.Fatal("correction left release check open") }
			f.Exec(t, `UPDATE issue SET status='todo' WHERE id=$1`,issue)
			questions := QuestionService{Queries:s.Queries, TxStarter:s.TxStarter, Tasks:s.Tasks, Bus:s.Bus}
			_, err := questions.Answer(context.Background(),util.MustParseUUID(f.WorkspaceID),id,QuestionActor{Type:"member",ID:util.MustParseUUID(f.UserID)},AnswerQuestionInput{Value:json.RawMessage(`{"option":"ready"}`)})
			wantQuestionRefusal(t,err,422)
		})
	}
}
