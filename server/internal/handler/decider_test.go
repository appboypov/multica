package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestUpdateWorkspaceDeciderBrianOnly(t *testing.T) {
	workspace := todoWorkspace(t)
	policy := map[string]any{"member_id": testUserID}
	encoded, _ := json.Marshal(map[string]any{"decider": policy})
	dbfx.Exec(t, `UPDATE workspace SET settings=$2 WHERE id=$1`, workspace, encoded)
	for _, source := range []string{"task_token", "cloud_pat"} {
		req := withURLParam(newRequest(http.MethodPatch, "/api/workspaces/"+workspace, map[string]any{"settings": map[string]any{"decider": map[string]any{"member_id": testUserID, "start_period": false}}}), "id", workspace)
		req.Header.Set("X-Actor-Source", source)
		testutil.Call(t, testHandler.UpdateWorkspace, req).Want(http.StatusForbidden)
	}
	request := func(settings map[string]any, status int) {
		t.Helper()
		req := withURLParam(newRequest(http.MethodPatch, "/api/workspaces/"+workspace, map[string]any{"settings": settings}), "id", workspace)
		testutil.Call(t, testHandler.UpdateWorkspace, req).Want(status)
	}
	request(map[string]any{"decider": map[string]any{"member_id": testUserID, "start_period": false, "confidence_threshold": 0.9}}, http.StatusOK)
	var raw []byte
	if err := dbfx.QueryRow(t, `SELECT settings FROM workspace WHERE id=$1`, workspace).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var stored map[string]any
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	if stored["decider"].(map[string]any)["start_period"] != false {
		t.Fatal("Brian's policy not saved")
	}
	// Another named member's policy cannot be removed, replaced or tuned by this owner.
	brian := uuid.NewString()
	dbfx.Exec(t, `UPDATE workspace SET settings=$2 WHERE id=$1`, workspace, `{"decider":{"member_id":"`+brian+`"}}`)
	request(map[string]any{}, http.StatusForbidden)
	request(map[string]any{"decider": map[string]any{"member_id": testUserID}}, http.StatusForbidden)
	request(map[string]any{"decider": map[string]any{"member_id": brian, "start_period": false}}, http.StatusForbidden)
	request(map[string]any{"decider": map[string]any{"member_id": brian}, "other": true}, http.StatusOK)
}

func TestRoutineConfirmationTodoCannotBeDismissed(t *testing.T) {
	workspace := todoWorkspace(t)
	issue := dbfx.Issue(t, "Routine proposal", testutil.Cols{"workspace_id": workspace, "status": "backlog"})
	id := insertHandlerTodo(t, workspace, "question", testutil.Cols{"issue_id": issue, "details": testutil.Raw(`'{"routine_decision_id":"decision"}'::jsonb`)})
	h := Handler{Queries: testHandler.Queries, Bus: events.New()}
	testutil.Call(t, inboxWorkspaceHandler(h.MarkTodoDone), todoDoneRequest(id, workspace)).Want(http.StatusUnprocessableEntity)
	if n := dbfx.Count(t, `SELECT count(*) FROM todo WHERE id=$1 AND done_at IS NULL`, id); n != 1 {
		t.Fatal("confirmation disappeared without an answer")
	}
}
