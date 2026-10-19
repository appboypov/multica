package handler

import (
	"context"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func insertTestSkillFolder(t *testing.T, path string) protocol.DaemonSkillFolder {
	t.Helper()
	id := dbfx.Insert(t, "skill_folder", testutil.Cols{
		"workspace_id": testWorkspaceID,
		"daemon_id":    "skill-folder-daemon-" + t.Name(),
		"path":         path,
		"created_by":   testUserID,
	})
	dbfx.Cleanup(t, `DELETE FROM skill WHERE config->'origin'->>'folder_id' = $1`, id)
	return protocol.DaemonSkillFolder{ID: id, Path: path}
}

func folderSkillsByKey(t *testing.T, folderID string) map[string]db.Skill {
	t.Helper()
	rows, err := testHandler.Queries.ListSkillsByFolder(context.Background(), db.ListSkillsByFolderParams{
		WorkspaceID: parseUUID(testWorkspaceID),
		FolderID:    folderID,
	})
	if err != nil {
		t.Fatalf("list folder skills: %v", err)
	}
	byKey := make(map[string]db.Skill, len(rows))
	for _, row := range rows {
		byKey[folderSkillKey(row.Config)] = row
	}
	return byKey
}

func TestSyncFolderSkillsCreatesUpdatesAndRemoves(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	folder := insertTestSkillFolder(t, "/skills/"+t.Name())
	alpha := "alpha-" + t.Name()
	beta := "beta-" + t.Name()

	skipped, err := testHandler.syncFolderSkills(ctx, parseUUID(testWorkspaceID), []protocol.DaemonSkillFolder{folder}, []folderSkillReport{
		{FolderID: folder.ID, Key: "alpha", Name: alpha, Description: "first", Content: "# Alpha", SourcePath: folder.Path + "/alpha"},
		{FolderID: folder.ID, Key: "beta", Name: beta, Content: "# Beta", SourcePath: folder.Path + "/beta"},
	})
	if err != nil || len(skipped) != 0 {
		t.Fatalf("first sync: skipped=%+v err=%v", skipped, err)
	}
	first := folderSkillsByKey(t, folder.ID)
	if len(first) != 2 || first["alpha"].Name != alpha || first["alpha"].Content != "# Alpha" {
		t.Fatalf("first sync library = %+v", first)
	}
	if path, ok := service.FolderSkillPath(first["alpha"].Config); !ok || path != folder.Path+"/alpha" {
		t.Fatalf("folder skill path = %q, %v", path, ok)
	}

	agentID := createHandlerTestAgent(t, "folder-skill-agent", nil)
	dbfx.Exec(t, `INSERT INTO agent_skill (agent_id, skill_id) VALUES ($1, $2)`, agentID, first["beta"].ID)

	skipped, err = testHandler.syncFolderSkills(ctx, parseUUID(testWorkspaceID), []protocol.DaemonSkillFolder{folder}, []folderSkillReport{
		{FolderID: folder.ID, Key: "alpha", Name: alpha, Description: "changed", Content: "# Alpha v2", SourcePath: folder.Path + "/alpha"},
	})
	if err != nil || len(skipped) != 0 {
		t.Fatalf("second sync: skipped=%+v err=%v", skipped, err)
	}
	second := folderSkillsByKey(t, folder.ID)
	if len(second) != 1 {
		t.Fatalf("removed skill stayed in the library: %+v", second)
	}
	if got := second["alpha"]; got.ID != first["alpha"].ID || got.Content != "# Alpha v2" || got.Description != "changed" {
		t.Fatalf("changed skill not updated in place: %+v", got)
	}
	var assigned int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM agent_skill WHERE agent_id = $1`, agentID).Scan(&assigned); err != nil {
		t.Fatalf("count agent skills: %v", err)
	}
	if assigned != 0 {
		t.Fatalf("removed folder skill stayed on the agent: %d rows", assigned)
	}
}

func TestSyncFolderSkillsLeavesOutTakenName(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	folder := insertTestSkillFolder(t, "/skills/"+t.Name())
	takenID := insertHandlerTestSkill(t, "taken", "# Library")
	var taken string
	if err := testPool.QueryRow(ctx, `SELECT name FROM skill WHERE id = $1`, takenID).Scan(&taken); err != nil {
		t.Fatalf("load taken skill: %v", err)
	}

	skipped, err := testHandler.syncFolderSkills(ctx, parseUUID(testWorkspaceID), []protocol.DaemonSkillFolder{folder}, []folderSkillReport{
		{FolderID: folder.ID, Key: "taken", Name: taken, Content: "# Folder", SourcePath: folder.Path + "/taken"},
	})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(skipped) != 1 || skipped[0].Key != "taken" || skipped[0].Name != taken {
		t.Fatalf("skipped = %+v, want the taken skill", skipped)
	}
	if got := folderSkillsByKey(t, folder.ID); len(got) != 0 {
		t.Fatalf("taken name entered the library: %+v", got)
	}
	var content string
	if err := testPool.QueryRow(ctx, `SELECT content FROM skill WHERE id = $1`, takenID).Scan(&content); err != nil || content != "# Library" {
		t.Fatalf("existing skill changed: content=%q err=%v", content, err)
	}
}

func TestSyncFolderSkillsKeepsUnscannedFolders(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	scanned := insertTestSkillFolder(t, "/skills/scanned-"+t.Name())
	other := insertTestSkillFolder(t, "/skills/other-"+t.Name())
	name := "other-" + t.Name()

	if _, err := testHandler.syncFolderSkills(ctx, parseUUID(testWorkspaceID), []protocol.DaemonSkillFolder{other}, []folderSkillReport{
		{FolderID: other.ID, Key: "other", Name: name, Content: "# Other", SourcePath: other.Path + "/other"},
	}); err != nil {
		t.Fatalf("seed other folder: %v", err)
	}
	if _, err := testHandler.syncFolderSkills(ctx, parseUUID(testWorkspaceID), []protocol.DaemonSkillFolder{scanned}, nil); err != nil {
		t.Fatalf("sync scanned folder: %v", err)
	}
	if got := folderSkillsByKey(t, other.ID); len(got) != 1 {
		t.Fatalf("a scan of another folder removed this folder's skills: %+v", got)
	}
}
