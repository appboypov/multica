package service

import (
	"context"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestDeciderSquadReleaseJoinConfirmation(t *testing.T) {
	s, f, _, lead, settings := deciderFixture(t, "", 0.96)
	f.Squad(t, "Design", lead, testutil.Cols{"description": "Designs screens"})
	squad := settings.Rules[0].SquadByKind["backend"]
	s.Judge = fakeDeciderJudge(t, squad, 0.96)
	settings.StartPeriod = nil
	settings.Rules = []DeciderRule{{ID: "release-squad", Event: "release_joined", Decision: "squad", Question: "Which squad takes this release work?", Reasons: map[string]string{"urgent": "The work fits this squad", "minor": "Another squad fits better"}}}
	setDeciderPolicy(t, f, settings)
	project := f.Project(t, "Release project")
	repo := f.Insert(t, "repository", testutil.Cols{"workspace_id": f.WorkspaceID, "project_id": project, "github_repo_id": 1, "full_name": "test/" + f.WorkspaceID, "private": false, "default_branch": "main", "checkout_daemon_id": "test", "checkout_path": "/tmp/test", "version_policy": "tags", "last_tag": ""})
	release := f.Insert(t, "release", testutil.Cols{"workspace_id": f.WorkspaceID, "repository_id": repo, "position": 0, "version": "1.0.0", "title": "Test release"})
	issue := f.Issue(t, "Release work", testutil.Cols{"status": "todo", "release_id": release})
	ctx := context.Background()
	if err := s.Handle(ctx, util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(issue), "status_entered"); err != nil {
		t.Fatal(err)
	}
	if n := f.Count(t, `SELECT count(*) FROM routine_decision WHERE issue_id=$1`, issue); n != 0 {
		t.Fatal("release-only rule reacted to a status event")
	}
	if err := s.Handle(ctx, util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(issue), "release_joined"); err != nil {
		t.Fatal(err)
	}
	question := routineQuestion(t, f, issue)
	if n := f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issue); n != 0 {
		t.Fatal("release squad ran before confirmation")
	}
	answerRoutine(t, s, f, question, "Backend", "")
	current, err := s.Queries.GetIssue(ctx, util.MustParseUUID(issue))
	if err != nil {
		t.Fatal(err)
	}
	if current.AssigneeType.String != "squad" || util.UUIDToString(current.AssigneeID) != squad || util.UUIDToString(current.ReleaseID) != release {
		t.Fatal("release choice did not assign the selected squad while preserving release membership")
	}
	if n := f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND squad_id=$2 AND agent_id=$3 AND status='queued'`, issue, squad, lead); n != 1 {
		t.Fatal("release choice did not queue the selected squad leader")
	}
}

func TestDeciderSquadAutomaticChoice(t *testing.T) {
	s, f, _, lead, settings := deciderFixture(t, "", 0.96)
	squad := settings.Rules[0].SquadByKind["backend"]
	s.Judge = fakeDeciderJudge(t, squad, 0.96)
	settings.Rules = []DeciderRule{{ID: "todo-squad", Event: "status_entered", Status: "todo", Decision: "squad", Question: "Which squad takes this work?", Reasons: map[string]string{"urgent": "The work fits this squad", "minor": "Another squad fits better"}}}
	setDeciderPolicy(t, f, settings)
	issue := f.Issue(t, "Automatic squad work", testutil.Cols{"status": "todo"})
	ctx := context.Background()
	for range 2 {
		if err := s.Handle(ctx, util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(issue), "status_entered"); err != nil {
			t.Fatal(err)
		}
	}
	current, err := s.Queries.GetIssue(ctx, util.MustParseUUID(issue))
	if err != nil {
		t.Fatal(err)
	}
	if current.AssigneeType.String != "squad" || util.UUIDToString(current.AssigneeID) != squad {
		t.Fatal("automatic choice did not assign the squad")
	}
	if n := f.Count(t, `SELECT count(*) FROM routine_decision WHERE issue_id=$1 AND automatic_number=1 AND question_id IS NULL AND applied_choice=$2`, issue, squad); n != 1 {
		t.Fatal("first automatic squad choice was not applied without a question")
	}
	if n := f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND squad_id=$2 AND agent_id=$3 AND status='queued'`, issue, squad, lead); n != 1 {
		t.Fatal("automatic choice did not queue exactly one selected leader task")
	}
}
