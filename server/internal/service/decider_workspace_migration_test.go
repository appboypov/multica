package service

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestDeciderWorkspaceSquadCutover(t *testing.T) {
	migration, err := os.ReadFile("../../migrations/802_disable_workspace_squad_rules.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	s, f, bug, _, settings := deciderFixture(t, "", 0.96, testutil.Cols{"id": "30e8e92f-9a49-4167-95e1-195ab4ec38be"})
	other, otherFixture, _, _, otherSettings := deciderFixture(t, "", 0.96)
	type workspaceCase struct {
		service  *DeciderService
		fixture  *testutil.Fixture
		settings DeciderSettings
		disabled bool
		want     map[string]any
	}
	cases := []workspaceCase{{service: s, fixture: f, settings: settings, disabled: true}, {service: other, fixture: otherFixture, settings: otherSettings}}
	for i := range cases {
		c := &cases[i]
		c.service.Judge = fakeDeciderJudge(t, c.settings.Rules[0].SquadByKind["backend"], 0.96)
		c.settings.StartPeriod = nil
		c.settings.Rules[0].ID = "post-merge-bug"
		reasons := map[string]string{"urgent": "The work fits this squad", "minor": "Another squad fits better"}
		c.settings.Rules = append([]DeciderRule{
			{ID: "todo-squad", Event: "status_entered", Status: "todo", Decision: "squad", Question: "Which squad takes this work?", Reasons: reasons},
			{ID: "release-squad", Event: "release_joined", Decision: "squad", Question: "Which squad takes this work?", Reasons: reasons},
		}, c.settings.Rules...)
		raw, err := json.Marshal(map[string]any{"decider": c.settings, "unrelated": map[string]any{"keep": true}})
		if err != nil {
			t.Fatal(err)
		}
		c.fixture.Exec(t, `UPDATE workspace SET settings=$2 WHERE id=$1`, c.fixture.WorkspaceID, raw)
		if err := json.Unmarshal(raw, &c.want); err != nil {
			t.Fatal(err)
		}
		if c.disabled {
			rules := c.want["decider"].(map[string]any)["rules"].([]any)
			rules[0].(map[string]any)["disabled"] = true
			rules[1].(map[string]any)["disabled"] = true
		}
	}
	f.Exec(t, string(migration))
	ctx := context.Background()
	for _, c := range cases {
		workspace := util.MustParseUUID(c.fixture.WorkspaceID)
		stored, err := c.service.Queries.GetWorkspace(ctx, workspace)
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		if err := json.Unmarshal(stored.Settings, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Fatalf("workspace %s settings=%s; want %#v", c.fixture.WorkspaceID, stored.Settings, c.want)
		}
		project := c.fixture.Project(t, "Release project")
		repo := c.fixture.Insert(t, "repository", testutil.Cols{"workspace_id": c.fixture.WorkspaceID, "project_id": project, "github_repo_id": 1, "full_name": "test/" + c.fixture.WorkspaceID, "private": false, "default_branch": "main", "checkout_daemon_id": "test", "checkout_path": "/tmp/test", "version_policy": "tags", "last_tag": ""})
		release := c.fixture.Insert(t, "release", testutil.Cols{"workspace_id": c.fixture.WorkspaceID, "repository_id": repo, "position": 0, "version": "1.0.0", "title": "Test release"})
		issue := c.fixture.Issue(t, "Unrouted work", testutil.Cols{"status": "todo", "release_id": release})
		for i, event := range []string{"status_entered", "release_joined"} {
			if err := c.service.Handle(ctx, workspace, util.MustParseUUID(issue), event); err != nil {
				t.Fatal(err)
			}
			want := i + 1
			if c.disabled {
				want = 0
			}
			if n := c.fixture.Count(t, `SELECT count(*) FROM routine_decision WHERE issue_id=$1`, issue); n != want {
				t.Fatalf("workspace %s after %s: decisions=%d; want %d", c.fixture.WorkspaceID, event, n, want)
			}
			if n := c.fixture.Count(t, `SELECT count(*) FROM question q JOIN routine_decision d ON d.question_id=q.id WHERE d.issue_id=$1`, issue); n != want {
				t.Fatalf("workspace %s after %s: questions=%d; want %d", c.fixture.WorkspaceID, event, n, want)
			}
		}
	}
	s.Judge = fakeDeciderJudge(t, "fix_now", 0.96)
	if err := s.Handle(ctx, util.MustParseUUID(f.WorkspaceID), util.MustParseUUID(bug), "status_entered"); err != nil {
		t.Fatal(err)
	}
	routineQuestion(t, f, bug)
}
