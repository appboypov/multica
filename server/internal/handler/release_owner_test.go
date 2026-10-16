package handler

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestReleaseOwnerOmittedSetClearAndUnchanged(t *testing.T) {
	_, repo, release := milestoneReleaseFixture(t, testWorkspaceID)
	agent := dbfx.Agent(t, "Release coordinator", "")
	var out struct {
		Releases []ReleaseResponse `json:"releases"`
	}
	set := map[string]any{"owner_type": "agent", "owner_id": agent}
	testutil.Call(t, testHandler.SetReleaseOwner, milestoneRequest("PUT", release, set)).Want(200).JSON(&out)
	if len(out.Releases) != 1 || out.Releases[0].OwnerID == nil || *out.Releases[0].OwnerID != agent || out.Releases[0].OwnerName == nil || *out.Releases[0].OwnerName != "Release coordinator" {
		t.Fatalf("owner projection = %+v", out.Releases)
	}
	testutil.Call(t, testHandler.SetReleaseOwner, milestoneRequest("PUT", release, set)).Want(200)
	if n := dbfx.Count(t, "SELECT count(*) FROM activity_log WHERE repository_id=$1 AND action='release_owner_changed'", repo); n != 1 {
		t.Fatalf("unchanged owner wrote %d activities", n)
	}
	testutil.Call(t, testHandler.EditRelease, milestoneRequest("PATCH", release, map[string]any{"title": "Updated title", "version": "1.0.0"})).Want(200)
	var got struct {
		Release ReleaseResponse `json:"release"`
	}
	testutil.Call(t, testHandler.GetRelease, milestoneRequest("GET", release, nil)).Want(200).JSON(&got)
	if got.Release.OwnerID == nil || *got.Release.OwnerID != agent || got.Release.Title != "Updated title" {
		t.Fatalf("omitted owner changed assignment: %+v", got.Release)
	}
	testutil.Call(t, testHandler.SetReleaseOwner, milestoneRequest("PUT", release, map[string]any{"owner_type": nil, "owner_id": nil})).Want(200)
	if n := dbfx.Count(t, "SELECT count(*) FROM release WHERE id=$1 AND owner_id IS NULL AND owner_type IS NULL", release); n != 1 {
		t.Fatal("owner not cleared")
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE release_id=$1", release); n != 0 {
		t.Fatalf("owner without runtime enqueued %d runs", n)
	}
}

func TestReleaseOwnerRefusalsAreAtomic(t *testing.T) {
	_, _, release := milestoneReleaseFixture(t, testWorkspaceID)
	agent := dbfx.Agent(t, "Archived coordinator", "")
	dbfx.Exec(t, "UPDATE agent SET archived_at=now() WHERE id=$1", agent)
	for _, body := range []map[string]any{
		{"owner_type": "member", "owner_id": testUserID},
		{"owner_type": "agent"},
		{"owner_id": agent},
		{"owner_type": nil, "owner_id": agent},
		{"owner_type": "agent", "owner_id": "bad"},
		{"owner_type": "agent", "owner_id": agent},
	} {
		testutil.Call(t, testHandler.SetReleaseOwner, milestoneRequest("PUT", release, body)).Want(400)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM activity_log WHERE details->>'release_id'=$1 AND action='release_owner_changed'", release); n != 0 {
		t.Fatalf("refused owner wrote %d activities", n)
	}
	dbfx.Exec(t, "UPDATE release SET shipped_at=$2 WHERE id=$1", release, time.Now())
	testutil.Call(t, testHandler.SetReleaseOwner, milestoneRequest("PUT", release, map[string]any{"owner_type": nil, "owner_id": nil})).Want(409)
	foreign := dbfx.Workspace(t, "Foreign release", "release-owner-"+uuid.NewString())
	_, _, other := milestoneReleaseFixture(t, foreign)
	testutil.Call(t, testHandler.SetReleaseOwner, milestoneRequest("PUT", other, map[string]any{"owner_type": nil, "owner_id": nil})).Want(404)
}

func TestReleaseOwnerPrivateAndSquadRefusalsUseAssignmentPolicy(t *testing.T) {
	_, _, release := milestoneReleaseFixture(t, testWorkspaceID)
	otherUser := dbfx.User(t, "Other coordinator owner", "release-owner-"+uuid.NewString()+"@example.com")
	private := dbfx.Agent(t, "Private release owner", "", testutil.Cols{"owner_id": otherUser})
	testutil.Call(t, testHandler.SetReleaseOwner, milestoneRequest("PUT", release, map[string]any{"owner_type": "agent", "owner_id": private})).Want(403)
	leader := dbfx.Agent(t, "Archived leader", "")
	squad := dbfx.Squad(t, "Release squad", leader)
	dbfx.Exec(t, "UPDATE agent SET archived_at=now() WHERE id=$1", leader)
	testutil.Call(t, testHandler.SetReleaseOwner, milestoneRequest("PUT", release, map[string]any{"owner_type": "squad", "owner_id": squad})).Want(400)
	dbfx.Exec(t, "UPDATE agent SET archived_at=NULL WHERE id=$1", leader)
	dbfx.Exec(t, "UPDATE squad SET archived_at=now() WHERE id=$1", squad)
	testutil.Call(t, testHandler.SetReleaseOwner, milestoneRequest("PUT", release, map[string]any{"owner_type": "squad", "owner_id": squad})).Want(400)
	if n := dbfx.Count(t, "SELECT count(*) FROM activity_log WHERE details->>'release_id'=$1 AND action='release_owner_changed'", release); n != 0 {
		t.Fatalf("refusal wrote %d owner activities", n)
	}
}
