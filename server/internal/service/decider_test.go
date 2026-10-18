package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	questiontype "github.com/multica-ai/multica/server/internal/question"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type deciderTransport func(*http.Request) (*http.Response, error)

func (f deciderTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func fakeDeciderJudge(t *testing.T, choice string, confidence float64) TypeSafeDecider {
	t.Helper()
	return TypeSafeDecider{APIKey: "test-secret", Client: &http.Client{Transport: deciderTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://api.typesafe.ai/v1/systemone" || r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Error("wrong TypeSafe request")
		}
		var body struct {
			State     map[string]json.RawMessage `json:"state"`
			Questions map[string]struct {
				Type string `json:"type"`
			} `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Questions["action"].Type != "choice" || body.Questions["reason"].Type != "choice" || len(body.State["issue"]) == 0 {
			t.Error("missing typed questions or issue context")
		}
		text := fmt.Sprintf(`{"answers":{"action":{"type":"choice","choice":%q,"confidence":%f,"probabilities":{%q:1}},"reason":{"type":"choice","choice":"urgent","confidence":%f,"probabilities":{"urgent":1}}}}`, choice, confidence, choice, confidence)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(text)), Header: http.Header{}}, nil
	})}}
}

func deciderFixture(t *testing.T, choice string, confidence float64) (*DeciderService, *testutil.Fixture, string, string, DeciderSettings) {
	t.Helper()
	pool := sharedTestPool(t)
	f := testutil.New(pool, "", "")
	suffix := time.Now().UnixNano()
	f.UserID = f.User(t, "Brian", fmt.Sprintf("decider-%d@example.test", suffix))
	f.WorkspaceID = f.Workspace(t, "Decisions", fmt.Sprintf("decider-%d", suffix))
	f.Member(t, f.WorkspaceID, f.UserID, "owner")
	runtime := f.Runtime(t, "Decider runtime")
	agent := f.Agent(t, "Decider", runtime, testutil.Cols{"model": "claude-sonnet-5-5"})
	lead := f.Agent(t, "Backend lead", runtime)
	squad := f.Squad(t, "Backend", lead, testutil.Cols{"description": "Builds server code"})
	postMerge := "11111111-1111-4111-8111-111111111111"
	kind := "22222222-2222-4222-8222-222222222222"
	rule := DeciderRule{ID: "post-merge", Event: "status_entered", Status: "backlog", Decision: "bug", Question: "Should this post-merge bug be fixed now or later?", Reasons: map[string]string{"urgent": "The bug blocks current work", "minor": "The bug can wait"}, PostMergeProperty: postMerge, KindProperty: kind, SquadByKind: map[string]string{"backend": squad}}
	start := false
	settings := DeciderSettings{AgentID: agent, MemberID: f.UserID, StartPeriod: &start, Rules: []DeciderRule{rule}}
	raw, _ := json.Marshal(map[string]any{"decider": settings})
	f.Exec(t, `UPDATE workspace SET settings=$2 WHERE id=$1`, f.WorkspaceID, raw)
	issue := f.Issue(t, "Bug after merge", testutil.Cols{"status": "backlog", "type": "bug", "properties": fmt.Sprintf(`{%q:true,%q:"backend"}`, postMerge, kind)})
	f.Cleanup(t, `DELETE FROM routine_decision WHERE workspace_id=$1`, f.WorkspaceID)
	f.Cleanup(t, `DELETE FROM comment WHERE workspace_id=$1`, f.WorkspaceID)
	f.Cleanup(t, `DELETE FROM todo WHERE workspace_id=$1`, f.WorkspaceID)
	f.Cleanup(t, `DELETE FROM agent_task_queue WHERE agent_id IN (SELECT id FROM agent WHERE workspace_id=$1)`, f.WorkspaceID)
	f.Cleanup(t, `DELETE FROM question WHERE workspace_id=$1`, f.WorkspaceID)
	bus := events.New()
	q := db.New(pool)
	tasks := NewTaskService(q, pool, nil, bus)
	return &DeciderService{Queries: q, TxStarter: pool, Tasks: tasks, Bus: bus, Judge: fakeDeciderJudge(t, choice, confidence)}, f, issue, lead, settings
}

func TestDeciderConfidentChoiceApplied(t *testing.T) {
	s, f, id, lead, settings := deciderFixture(t, "fix_now", 0.96)
	if err := s.Handle(context.Background(), util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(id), "status_entered"); err != nil {
		t.Fatal(err)
	}
	issue, err := s.Queries.GetIssue(context.Background(), util.MustParseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	if issue.Status != "todo" || issue.AssigneeType.String != "squad" || util.UUIDToString(issue.AssigneeID) != settings.Rules[0].SquadByKind["backend"] {
		t.Fatalf("wrong fix-now outcome: %#v", issue)
	}
	if n := f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND agent_id=$2 AND squad_id=$3`, id, lead, settings.Rules[0].SquadByKind["backend"]); n != 1 {
		t.Fatalf("squad leader runs=%d", n)
	}
	var body string
	f.QueryRow(t, `SELECT content FROM comment WHERE issue_id=$1`, id).Scan(&body)
	if !strings.Contains(body, "Jev (confidence 0.960)") || !strings.Contains(body, "blocks current work") {
		t.Fatalf("missing audit: %s", body)
	}
}

func TestDeciderSquadForUnassignedTodo(t *testing.T) {
	s, f, parent, lead, settings := deciderFixture(t, "", 0.96)
	f.Squad(t, "Design", lead, testutil.Cols{"description": "Designs screens"})
	squad := settings.Rules[0].SquadByKind["backend"]
	s.Judge = fakeDeciderJudge(t, squad, 0.96)
	settings.StartPeriod = nil
	settings.Rules = []DeciderRule{{ID: "todo-squad", Event: "status_entered", Status: "todo", Decision: "squad", Question: "Which squad takes this work?", Reasons: map[string]string{"urgent": "The work fits this squad", "minor": "Another squad fits better"}}}
	setDeciderPolicy(t, f, settings)
	ctx := context.Background()
	open := f.Issue(t, "Login screen API", testutil.Cols{"status": "todo"})
	held := f.Issue(t, "Held work", testutil.Cols{"status": "todo", "assignee_type": "agent", "assignee_id": lead})
	sub := f.Issue(t, "Worker step", testutil.Cols{"status": "todo", "parent_issue_id": parent})
	for _, id := range []string{open, held, sub} {
		if err := s.Handle(ctx, util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(id), "status_entered"); err != nil {
			t.Fatal(err)
		}
	}
	if n := f.Count(t, `SELECT count(*) FROM routine_decision WHERE issue_id = ANY($1::uuid[])`, []string{held, sub}); n != 0 {
		t.Fatalf("held work got a squad decision: %d", n)
	}
	id := routineQuestion(t, f, open)
	question, err := s.Queries.GetQuestion(ctx, db.GetQuestionParams{ID: id, WorkspaceID: util.MustParseUUID(f.WorkspaceID)})
	if err != nil {
		t.Fatal(err)
	}
	var suggested questiontype.SingleChoiceAnswerValue
	if err := json.Unmarshal(question.SuggestedAnswer, &suggested); err != nil || suggested.Option == nil || *suggested.Option != "Backend" {
		t.Fatalf("Brian should see the squad by its name: %s", question.SuggestedAnswer)
	}
	if n := f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, open); n != 0 {
		t.Fatalf("squad ran before yes: %d", n)
	}
	answerRoutine(t, s, f, id, "Backend", "")
	issue, err := s.Queries.GetIssue(ctx, util.MustParseUUID(open))
	if err != nil {
		t.Fatal(err)
	}
	if issue.Status != "todo" || issue.AssigneeType.String != "squad" || util.UUIDToString(issue.AssigneeID) != squad {
		t.Fatalf("issue not with the squad: %#v", issue)
	}
	if n := f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND agent_id=$2 AND squad_id=$3`, open, lead, squad); n != 1 {
		t.Fatalf("squad leader runs=%d", n)
	}
}

func TestDeciderFormerMemberGetsNoTodo(t *testing.T) {
	s, f, id, _, _ := deciderFixture(t, "fix_later", 0.96)
	f.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.WorkspaceID, f.UserID)
	if err := s.Handle(context.Background(), util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(id), "status_entered"); err != nil {
		t.Fatal(err)
	}
	if n := f.Count(t, `SELECT count(*) FROM todo WHERE workspace_id=$1 AND member_id=$2`, f.WorkspaceID, f.UserID); n != 0 {
		t.Fatalf("former member got %d to-dos", n)
	}
	var outcome string
	f.QueryRow(t, `SELECT outcome FROM routine_decision WHERE issue_id=$1`, id).Scan(&outcome)
	if outcome != "failed" {
		t.Fatalf("outcome=%s", outcome)
	}
	var body string
	f.QueryRow(t, `SELECT content FROM comment WHERE issue_id=$1`, id).Scan(&body)
	if !strings.Contains(body, "Not applied: the decider member is not in this workspace") {
		t.Fatalf("audit does not report why: %s", body)
	}
}

func TestDeciderUnsureEnqueuesRunAndAppliesAnswer(t *testing.T) {
	s, f, id, _, settings := deciderFixture(t, "fix_now", 0.4)
	ctx := context.Background()
	if err := s.Handle(ctx, util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(id), "status_entered"); err != nil {
		t.Fatal(err)
	}
	issue, err := s.Queries.GetIssue(ctx, util.MustParseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	if issue.Status != "backlog" || issue.AssigneeID.Valid {
		t.Fatalf("uncertain answer changed work: %#v", issue)
	}
	var taskID string
	var note string
	f.QueryRow(t, `SELECT id,handoff_note FROM agent_task_queue WHERE issue_id=$1 AND agent_id=$2`, id, settings.AgentID).Scan(&taskID, &note)
	if !strings.Contains(note, "fix_later") || !strings.Contains(note, "Bug after merge") || !strings.Contains(note, "Should this post-merge") {
		t.Fatalf("missing task context: %s", note)
	}
	payload, _ := json.Marshal(protocol.TaskCompletedPayload{Output: `{"choice":"fix_later","reason":"minor"}`})
	f.Exec(t, `UPDATE agent_task_queue SET status='completed',result=$2 WHERE id=$1`, taskID, payload)
	if err := s.Complete(ctx, util.MustParseUUID(taskID)); err != nil {
		t.Fatal(err)
	}
	if err := s.Complete(ctx, util.MustParseUUID(taskID)); err != nil {
		t.Fatal(err)
	}
	if n := f.Count(t, `SELECT count(*) FROM todo WHERE issue_id=$1 AND member_id=$2 AND kind='review'`, id, f.UserID); n != 1 {
		t.Fatalf("fix-later todos=%d", n)
	}
	if n := f.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1 AND content LIKE '%Decider agent Decider%'`, id); n != 1 {
		t.Fatalf("Decider audit comments=%d", n)
	}
}

func TestDeciderStaleAnswerDoesNotApply(t *testing.T) {
	s, f, id, _, _ := deciderFixture(t, "fix_now", 0.4)
	ctx := context.Background()
	if err := s.Handle(ctx, util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(id), "status_entered"); err != nil {
		t.Fatal(err)
	}
	var taskID string
	f.QueryRow(t, `SELECT id FROM agent_task_queue WHERE issue_id=$1`, id).Scan(&taskID)
	f.Exec(t, `UPDATE issue SET title='A changed question' WHERE id=$1`, id)
	payload, _ := json.Marshal(protocol.TaskCompletedPayload{Output: `{"choice":"fix_later","reason":"minor"}`})
	f.Exec(t, `UPDATE agent_task_queue SET status='completed',result=$2 WHERE id=$1`, taskID, payload)
	if err := s.Complete(ctx, util.MustParseUUID(taskID)); err != nil {
		t.Fatal(err)
	}
	if n := f.Count(t, `SELECT count(*) FROM todo WHERE issue_id=$1`, id); n != 0 {
		t.Fatalf("stale answer opened %d todos", n)
	}
	var outcome string
	f.QueryRow(t, `SELECT outcome FROM routine_decision WHERE task_id=$1`, taskID).Scan(&outcome)
	if outcome != "stale" {
		t.Fatalf("outcome=%s", outcome)
	}
}

func TestIssueReleaseJoinEvent(t *testing.T) {
	bus := events.New()
	var got []events.Event
	bus.Subscribe(protocol.EventIssueReleaseJoined, func(e events.Event) { got = append(got, e) })
	issue := db.Issue{ID: util.MustParseUUID("11111111-1111-4111-8111-111111111111"), WorkspaceID: util.MustParseUUID("22222222-2222-4222-8222-222222222222"), ReleaseID: util.MustParseUUID("33333333-3333-4333-8333-333333333333")}
	PublishIssueReleaseJoined(bus, issue, pgtype.UUID{}, "member", "actor")
	PublishIssueReleaseJoined(bus, issue, issue.ReleaseID, "member", "actor")
	issue.ReleaseID = pgtype.UUID{}
	PublishIssueReleaseJoined(bus, issue, pgtype.UUID{}, "member", "actor")
	if len(got) != 1 {
		t.Fatalf("join events=%d", len(got))
	}
	p := got[0].Payload.(protocol.IssueReleaseJoinedPayload)
	if p.IssueID != util.UUIDToString(issue.ID) || p.ReleaseID == "" || got[0].WorkspaceID != util.UUIDToString(issue.WorkspaceID) {
		t.Fatalf("wrong event: %#v", got[0])
	}
}

func TestDeciderSettingsConfidence(t *testing.T) {
	s, err := ParseDeciderSettings([]byte(`{}`))
	if err != nil || s.threshold() != 0.85 || !s.startPeriod() {
		t.Fatalf("default: %#v %v", s, err)
	}
	if _, err := ParseDeciderSettings([]byte(`{"decider":{"agent_id":"bad","rules":[{}]}}`)); err == nil {
		t.Fatal("invalid settings accepted")
	}
}

func TestDeciderReleaseReadyAndNotYet(t *testing.T) {
	for _, choice := range []string{"ready", "not_yet"} {
		t.Run(choice, func(t *testing.T) {
			s, f, id, _, settings := deciderFixture(t, choice, 0.96)
			project := f.Project(t, "Release project")
			repo := f.Insert(t, "repository", testutil.Cols{"workspace_id": f.WorkspaceID, "project_id": project, "github_repo_id": 1, "full_name": "test/" + f.WorkspaceID, "private": false, "default_branch": "main", "checkout_daemon_id": "test", "checkout_path": "/tmp/test", "version_policy": "tags", "last_tag": ""})
			release := f.Insert(t, "release", testutil.Cols{"workspace_id": f.WorkspaceID, "repository_id": repo, "position": 0, "version": "1.0.0", "title": "Test release"})
			settings.Rules = []DeciderRule{{ID: "release", Event: "release_joined", Decision: "release", Question: "Is this release ready for Brian's check?", Reasons: map[string]string{"urgent": "All release work is complete", "minor": "More release review is needed"}}}
			raw, _ := json.Marshal(map[string]any{"decider": settings})
			f.Exec(t, `UPDATE workspace SET settings=$2 WHERE id=$1`, f.WorkspaceID, raw)
			f.Exec(t, `UPDATE issue SET release_id=$2,status='todo' WHERE id=$1`, id, release)
			ctx := context.Background()
			if err := s.Handle(ctx, util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(id), "release_joined"); err != nil {
				t.Fatal(err)
			}
			if n := f.Count(t, `SELECT count(*) FROM routine_decision WHERE workspace_id=$1`, f.WorkspaceID); n != 0 {
				t.Fatalf("unfinished release got %d decisions", n)
			}
			f.Issue(t, "Second release issue", testutil.Cols{"release_id": release})
			// The trigger sorts after the release's first issue, so a receipt on the first issue fails.
			var first, trigger string
			f.QueryRow(t, `SELECT min(id::text),max(id::text) FROM issue WHERE release_id=$1`, release).Scan(&first, &trigger)
			f.Exec(t, `UPDATE issue SET status='done' WHERE id=$1`, first)
			if err := s.Handle(ctx, util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(first), "status_entered"); err != nil {
				t.Fatal(err)
			}
			f.Exec(t, `UPDATE issue SET status='done' WHERE id=$1`, trigger)
			if err := s.Handle(ctx, util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(trigger), "status_entered"); err != nil {
				t.Fatal(err)
			}
			if err := s.Handle(ctx, util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(trigger), "release_joined"); err != nil {
				t.Fatal(err)
			}
			want := 0
			if choice == "ready" {
				want = 1
			}
			if n := f.Count(t, `SELECT count(*) FROM todo WHERE workspace_id=$1 AND details->>'release_id'=$2 AND issue_id=$3`, f.WorkspaceID, release, trigger); n != want {
				t.Fatalf("release todos on the trigger issue=%d, want %d", n, want)
			}
			if n := f.Count(t, `SELECT count(*) FROM routine_decision WHERE workspace_id=$1 AND issue_id=$2`, f.WorkspaceID, trigger); n != 1 {
				t.Fatalf("release receipts on the trigger issue=%d", n)
			}
			if n := f.Count(t, `SELECT count(*) FROM routine_decision WHERE workspace_id=$1`, f.WorkspaceID); n != 1 {
				t.Fatalf("release receipts=%d", n)
			}
			if n := f.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1 AND content LIKE 'Routine decision release%'`, trigger); n != 1 {
				t.Fatalf("audit comments on the trigger issue=%d", n)
			}
			if n := f.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1 AND content LIKE 'Routine decision release%'`, first); n != 0 {
				t.Fatalf("audit comments on the first issue=%d", n)
			}
		})
	}
}
