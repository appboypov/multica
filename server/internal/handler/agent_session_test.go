package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func agentSessionHandlerFixture(t *testing.T) (*Handler, string, string, string) {
	t.Helper()
	if testPool == nil || testHandler == nil {
		t.Skip("database not available")
	}
	runtimeID, daemonID := agentSessionTestRuntime(t, testWorkspaceID, "online")
	agentID := agentSessionTestAgent(t, testWorkspaceID, runtimeID)
	h := *testHandler
	h.Bus = events.New()
	return &h, agentID, runtimeID, daemonID
}

func agentSessionDaemonRequest(method, path, workspaceID, daemonID string, body any) *http.Request {
	req := newRequest(method, path, body)
	return req.WithContext(middleware.WithDaemonContext(req.Context(), workspaceID, daemonID))
}

func agentSessionWriteRequest(agentID, daemonID string, pid int32) RegisterAgentSessionRequest {
	pane, repository, branch := "w12:p0", "appboypov/multica", "work-1057"
	return RegisterAgentSessionRequest{
		WorkspaceID: testWorkspaceID, DaemonID: daemonID, AgentID: agentID, PID: pid,
		ProcessStartedAt: time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond),
		HerdrPaneID:      &pane, WorkDir: "/work/multica", Repository: &repository, Branch: &branch, Status: "idle",
	}
}

func agentSessionRegisterHTTP(t *testing.T, h *Handler, req RegisterAgentSessionRequest) string {
	t.Helper()
	var result struct {
		ID string `json:"id"`
	}
	testutil.Call(t, h.RegisterAgentSession, agentSessionDaemonRequest(http.MethodPost, "/api/daemon/agent-sessions", req.WorkspaceID, req.DaemonID, req)).Want(http.StatusCreated).JSON(&result)
	if _, err := uuid.Parse(result.ID); err != nil {
		t.Fatalf("registration returned invalid id %q: %v", result.ID, err)
	}
	return result.ID
}

func agentSessionListHTTP(t *testing.T, h *Handler, agentID string) AgentSessionsResponse {
	t.Helper()
	var result AgentSessionsResponse
	req := withURLParam(newRequest(http.MethodGet, "/api/agents/"+agentID+"/sessions", nil), "id", agentID)
	testutil.Call(t, h.ListAgentSessions, req).Want(http.StatusOK).JSON(&result)
	return result
}

func agentSessionDaemonURL(id, workspaceID, daemonID string) string {
	path := "/api/daemon/agent-sessions"
	if id != "" {
		path += "/" + id
	}
	return path + "?" + url.Values{"workspace_id": {workspaceID}, "daemon_id": {daemonID}}.Encode()
}

func TestAgentSessionHandlersRegisterReplaceUpdateEndAndPublish(t *testing.T) {
	h, agentID, _, daemonID := agentSessionHandlerFixture(t)
	var published []events.Event
	h.Bus.Subscribe(protocol.EventAgentSessions, func(event events.Event) { published = append(published, event) })
	assertEvents := func(want int) {
		t.Helper()
		if len(published) != want {
			t.Fatalf("published %d session events, want %d: %+v", len(published), want, published)
		}
		for _, event := range published {
			payload, ok := event.Payload.(protocol.AgentSessionsEventPayload)
			if !ok || payload.AgentID != agentID || event.WorkspaceID != testWorkspaceID {
				t.Fatalf("event = %+v, want agent %s in workspace %s", event, agentID, testWorkspaceID)
			}
		}
	}

	request := agentSessionWriteRequest(agentID, daemonID, 5101)
	id := agentSessionRegisterHTTP(t, h, request)
	assertEvents(1)
	list := agentSessionListHTTP(t, h, agentID)
	if len(list.Sessions) != 1 {
		t.Fatalf("registered session list = %+v", list)
	}
	session := list.Sessions[0]
	if session.ID != id || session.Kind != "terminal" || session.PID == nil || *session.PID != request.PID || session.HerdrPaneID == nil || *session.HerdrPaneID != *request.HerdrPaneID || session.WorkDir != request.WorkDir || session.Status != "idle" {
		t.Fatalf("registered session = %+v, want registration %+v", session, request)
	}
	if session.Repository == nil || *session.Repository != *request.Repository || session.Branch == nil || *session.Branch != *request.Branch || session.Runtime != nil || session.TaskID != nil || session.Issue != nil || session.StartedAt == "" {
		t.Fatalf("terminal session JSON fields = %+v", session)
	}
	if _, err := time.Parse(time.RFC3339, session.StartedAt); err != nil {
		t.Fatalf("started_at = %q: %v", session.StartedAt, err)
	}

	// Re-registering a process replaces its metadata without duplicating it.
	request.WorkDir = "/work/replacement"
	request.ProcessStartedAt = request.ProcessStartedAt.Add(time.Second)
	request.HerdrPaneID, request.Repository, request.Branch = nil, nil, nil
	request.Status = "working"
	if replacementID := agentSessionRegisterHTTP(t, h, request); replacementID != id {
		t.Fatalf("replacement id = %s, want %s", replacementID, id)
	}
	assertEvents(2)
	list = agentSessionListHTTP(t, h, agentID)
	if len(list.Sessions) != 1 || list.Sessions[0].WorkDir != request.WorkDir || list.Sessions[0].Status != "working" || list.Sessions[0].HerdrPaneID != nil || list.Sessions[0].Repository != nil || list.Sessions[0].Branch != nil {
		t.Fatalf("replacement list = %+v", list)
	}
	var own struct {
		Sessions []db.AgentSession `json:"sessions"`
	}
	path := agentSessionDaemonURL("", testWorkspaceID, daemonID)
	testutil.Call(t, h.ListDaemonAgentSessions, agentSessionDaemonRequest(http.MethodGet, path, testWorkspaceID, daemonID, nil)).Want(http.StatusOK).JSON(&own)
	if len(own.Sessions) != 1 || uuidToString(own.Sessions[0].ID) != id || !own.Sessions[0].ProcessStartedAt.Time.Equal(request.ProcessStartedAt) || uuidToString(own.Sessions[0].AgentID) != agentID || own.Sessions[0].DaemonID != daemonID {
		t.Fatalf("daemon reload metadata = %+v", own)
	}

	repository, branch := "appboypov/multica", "work-1087"
	patch := UpdateAgentSessionRequest{WorkspaceID: testWorkspaceID, DaemonID: daemonID, Status: "blocked", Repository: &repository, Branch: &branch}
	patchReq := agentSessionDaemonRequest(http.MethodPatch, "/api/daemon/agent-sessions/"+id, testWorkspaceID, daemonID, patch)
	testutil.Call(t, h.UpdateAgentSession, withURLParam(patchReq, "id", id)).Want(http.StatusOK)
	assertEvents(3)
	list = agentSessionListHTTP(t, h, agentID)
	if len(list.Sessions) != 1 || list.Sessions[0].Status != "blocked" || list.Sessions[0].Branch == nil || *list.Sessions[0].Branch != branch || list.Sessions[0].Repository == nil || *list.Sessions[0].Repository != repository {
		t.Fatalf("updated list = %+v", list)
	}
	patch.Repository, patch.Branch = nil, nil
	patchReq = agentSessionDaemonRequest(http.MethodPatch, "/api/daemon/agent-sessions/"+id, testWorkspaceID, daemonID, patch)
	testutil.Call(t, h.UpdateAgentSession, withURLParam(patchReq, "id", id)).Want(http.StatusOK)
	assertEvents(4)
	list = agentSessionListHTTP(t, h, agentID)
	if len(list.Sessions) != 1 || list.Sessions[0].Repository != nil || list.Sessions[0].Branch != nil {
		t.Fatalf("cleared git metadata remains: %+v", list)
	}
	path = agentSessionDaemonURL(id, testWorkspaceID, daemonID)
	endReq := agentSessionDaemonRequest(http.MethodDelete, path, testWorkspaceID, daemonID, nil)
	testutil.Call(t, h.EndAgentSession, withURLParam(endReq, "id", id)).Want(http.StatusNoContent)
	assertEvents(5)
	if list = agentSessionListHTTP(t, h, agentID); list.Sessions == nil || len(list.Sessions) != 0 {
		t.Fatalf("ended session remains or list is null: %+v", list)
	}
	// A retry and an unknown id are both harmless, without fictitious events.
	for _, endedID := range []string{id, uuid.NewString()} {
		path = agentSessionDaemonURL(endedID, testWorkspaceID, daemonID)
		testutil.Call(t, h.EndAgentSession, withURLParam(agentSessionDaemonRequest(http.MethodDelete, path, testWorkspaceID, daemonID, nil), "id", endedID)).Want(http.StatusNoContent)
	}
	assertEvents(5)
}

func TestAgentSessionHandlersMovePaneWithoutReplacingProcess(t *testing.T) {
	h, agentID, _, daemonID := agentSessionHandlerFixture(t)
	request := agentSessionWriteRequest(agentID, daemonID, 5106)
	id := agentSessionRegisterHTTP(t, h, request)
	dbfx.Exec(t, `UPDATE agent_session SET started_at = now() - interval '1 hour' WHERE id = $1`, id)
	params := db.ListAgentSessionsForDaemonParams{WorkspaceID: parseUUID(testWorkspaceID), DaemonID: daemonID}
	before, err := h.Queries.ListAgentSessionsForDaemon(context.Background(), params)
	if err != nil || len(before) != 1 {
		t.Fatalf("registered sessions = %+v, err = %v", before, err)
	}
	original := before[0]
	pane := "w13:p2"
	for _, update := range []struct {
		name  string
		patch UpdateAgentSessionRequest
	}{
		{"moved pane", UpdateAgentSessionRequest{WorkspaceID: testWorkspaceID, DaemonID: daemonID, Status: "working", HerdrPaneID: &pane}},
		{"retried pane", UpdateAgentSessionRequest{WorkspaceID: testWorkspaceID, DaemonID: daemonID, Status: "working", HerdrPaneID: &pane}},
		{"omitted pane", UpdateAgentSessionRequest{WorkspaceID: testWorkspaceID, DaemonID: daemonID, Status: "blocked"}},
	} {
		t.Run(update.name, func(t *testing.T) {
			req := agentSessionDaemonRequest(http.MethodPatch, "/api/daemon/agent-sessions/"+id, testWorkspaceID, daemonID, update.patch)
			testutil.Call(t, h.UpdateAgentSession, withURLParam(req, "id", id)).Want(http.StatusOK)
			after, err := h.Queries.ListAgentSessionsForDaemon(context.Background(), params)
			if err != nil || len(after) != 1 {
				t.Fatalf("updated sessions = %+v, err = %v", after, err)
			}
			session := after[0]
			if session.ID != original.ID || session.WorkspaceID != original.WorkspaceID || session.AgentID != original.AgentID || session.DaemonID != original.DaemonID || session.Pid != original.Pid || !session.ProcessStartedAt.Time.Equal(original.ProcessStartedAt.Time) || !session.StartedAt.Time.Equal(original.StartedAt.Time) || session.WorkDir != original.WorkDir {
				t.Fatalf("pane update replaced session identity: before=%+v after=%+v", original, session)
			}
			if !session.HerdrPaneID.Valid || session.HerdrPaneID.String != pane || session.Status != update.patch.Status {
				t.Fatalf("persisted pane/status = %+v, want pane %s and status %s", session, pane, update.patch.Status)
			}
			list := agentSessionListHTTP(t, h, agentID)
			if len(list.Sessions) != 1 || list.Sessions[0].ID != id || list.Sessions[0].HerdrPaneID == nil || *list.Sessions[0].HerdrPaneID != pane {
				t.Fatalf("listed pane after update = %+v, want session %s pane %s", list, id, pane)
			}
		})
	}
}

func TestAgentSessionHandlersRefuseOtherDaemonAndWorkspace(t *testing.T) {
	h, agentID, _, daemonID := agentSessionHandlerFixture(t)
	id := agentSessionRegisterHTTP(t, h, agentSessionWriteRequest(agentID, daemonID, 5102))
	before := agentSessionListHTTP(t, h, agentID)
	var published []events.Event
	h.Bus.Subscribe(protocol.EventAgentSessions, func(event events.Event) { published = append(published, event) })
	_, otherDaemon := agentSessionTestRuntime(t, testWorkspaceID, "online")
	pane := "w99:p9"
	for _, caller := range []struct {
		name, workspaceID, daemonID string
	}{
		{"another daemon", testWorkspaceID, otherDaemon},
		{"another workspace", uuid.NewString(), daemonID},
	} {
		t.Run(caller.name, func(t *testing.T) {
			patch := UpdateAgentSessionRequest{WorkspaceID: testWorkspaceID, DaemonID: caller.daemonID, Status: "done", HerdrPaneID: &pane}
			req := agentSessionDaemonRequest(http.MethodPatch, "/api/daemon/agent-sessions/"+id, caller.workspaceID, caller.daemonID, patch)
			testutil.Call(t, h.UpdateAgentSession, withURLParam(req, "id", id)).Want(http.StatusNotFound)
			path := agentSessionDaemonURL(id, testWorkspaceID, caller.daemonID)
			req = agentSessionDaemonRequest(http.MethodDelete, path, caller.workspaceID, caller.daemonID, nil)
			testutil.Call(t, h.EndAgentSession, withURLParam(req, "id", id)).Want(http.StatusNotFound)
			if after := agentSessionListHTTP(t, h, agentID); !reflect.DeepEqual(before, after) {
				t.Fatalf("unauthorized writes changed session: before=%+v after=%+v", before, after)
			}
		})
	}
	// A token may not claim the owning daemon's id in a request.
	spoof := UpdateAgentSessionRequest{WorkspaceID: testWorkspaceID, DaemonID: daemonID, Status: "done", HerdrPaneID: &pane}
	testutil.Call(t, h.UpdateAgentSession, withURLParam(agentSessionDaemonRequest(http.MethodPatch, "/api/daemon/agent-sessions/"+id, testWorkspaceID, otherDaemon, spoof), "id", id)).Want(http.StatusForbidden)
	path := agentSessionDaemonURL(id, testWorkspaceID, daemonID)
	testutil.Call(t, h.EndAgentSession, withURLParam(agentSessionDaemonRequest(http.MethodDelete, path, testWorkspaceID, otherDaemon, nil), "id", id)).Want(http.StatusForbidden)
	if after := agentSessionListHTTP(t, h, agentID); !reflect.DeepEqual(before, after) {
		t.Fatalf("spoofed writes changed session: before=%+v after=%+v", before, after)
	}
	var other struct {
		Sessions []db.AgentSession `json:"sessions"`
	}
	path = agentSessionDaemonURL("", testWorkspaceID, otherDaemon)
	testutil.Call(t, h.ListDaemonAgentSessions, agentSessionDaemonRequest(http.MethodGet, path, testWorkspaceID, otherDaemon, nil)).Want(http.StatusOK).JSON(&other)
	if other.Sessions == nil || len(other.Sessions) != 0 || len(published) != 0 {
		t.Fatalf("other daemon saw sessions or writes published: sessions=%+v events=%+v", other, published)
	}
}

func TestAgentSessionHandlersListRunsUntilTheyEnd(t *testing.T) {
	h, agentID, runtimeID, _ := agentSessionHandlerFixture(t)
	issueID := dbfx.Issue(t, "Active session run")
	repositoryID, repository := agentSessionTestRepository(t, testWorkspaceID)
	agentSessionTestWorktree(t, issueID, repositoryID, "/work/run", "work-run", "active")
	taskID := agentSessionTestRun(t, agentID, runtimeID, issueID, "running", "/work/run")
	list := agentSessionListHTTP(t, h, agentID)
	if len(list.Sessions) != 1 {
		t.Fatalf("running session list = %+v", list)
	}
	run := list.Sessions[0]
	if run.ID != taskID || run.Kind != "run" || run.TaskID == nil || *run.TaskID != taskID || run.Runtime == nil || run.Runtime.ID != runtimeID || run.Runtime.Name == "" || run.Issue == nil || run.Issue.ID != issueID || run.Issue.Identifier == "" || run.Status != "working" || run.WorkDir != "/work/run" || run.PID != nil || run.HerdrPaneID != nil || run.Repository == nil || *run.Repository != repository || run.Branch == nil || *run.Branch != "work-run" {
		t.Fatalf("run response = %+v", run)
	}
	for _, status := range []string{"completed", "failed", "cancelled"} {
		dbfx.Exec(t, `UPDATE agent_task_queue SET status = $1 WHERE id = $2`, status, taskID)
		if after := agentSessionListHTTP(t, h, agentID); len(after.Sessions) != 0 {
			t.Fatalf("%s run remains: %+v", status, after)
		}
	}
}

func TestAgentSessionHandlersAgentAndWorkspaceAccess(t *testing.T) {
	h, agentID, _, daemonID := agentSessionHandlerFixture(t)
	otherWorkspace := dbfx.Workspace(t, "Other session workspace", "session-other-"+uuid.NewString())
	otherRuntime, _ := agentSessionTestRuntime(t, otherWorkspace, "online")
	otherAgent := agentSessionTestAgent(t, otherWorkspace, otherRuntime)
	request := agentSessionWriteRequest(otherAgent, daemonID, 5103)
	testutil.Call(t, h.RegisterAgentSession, agentSessionDaemonRequest(http.MethodPost, "/api/daemon/agent-sessions", testWorkspaceID, daemonID, request)).Want(http.StatusNotFound)
	testutil.Call(t, h.ListAgentSessions, withURLParam(newRequest(http.MethodGet, "/api/agents/"+otherAgent+"/sessions", nil), "id", otherAgent)).Want(http.StatusNotFound)
	request = agentSessionWriteRequest(agentID, daemonID, 5103)
	request.WorkspaceID = otherWorkspace
	testutil.Call(t, h.RegisterAgentSession, agentSessionDaemonRequest(http.MethodPost, "/api/daemon/agent-sessions", testWorkspaceID, daemonID, request)).Want(http.StatusNotFound)
	path := agentSessionDaemonURL("", otherWorkspace, daemonID)
	testutil.Call(t, h.ListDaemonAgentSessions, agentSessionDaemonRequest(http.MethodGet, path, testWorkspaceID, daemonID, nil)).Want(http.StatusNotFound)
	if after := agentSessionListHTTP(t, h, agentID); len(after.Sessions) != 0 {
		t.Fatalf("rejected registration created a session: %+v", after)
	}

	privateAgent, ownerID, memberID := privateAgentTestFixture(t)
	for _, caller := range []struct {
		user string
		want int
	}{{ownerID, http.StatusOK}, {memberID, http.StatusForbidden}} {
		req := withURLParam(newRequestAs(caller.user, http.MethodGet, "/api/agents/"+privateAgent+"/sessions", nil), "id", privateAgent)
		testutil.Call(t, h.ListAgentSessions, req).Want(caller.want)
	}
}

func TestAgentSessionHandlersPATAndTokenIdentity(t *testing.T) {
	h, agentID, _, daemonID := agentSessionHandlerFixture(t)
	request := agentSessionWriteRequest(agentID, daemonID, 5104)
	// PAT/JWT fallback uses workspace membership and an explicit machine id.
	var registered struct {
		ID string `json:"id"`
	}
	testutil.Call(t, h.RegisterAgentSession, newRequest(http.MethodPost, "/api/daemon/agent-sessions", request)).Want(http.StatusCreated).JSON(&registered)
	path := agentSessionDaemonURL("", testWorkspaceID, daemonID)
	var own struct {
		Sessions []db.AgentSession `json:"sessions"`
	}
	testutil.Call(t, h.ListDaemonAgentSessions, newRequest(http.MethodGet, path, nil)).Want(http.StatusOK).JSON(&own)
	if len(own.Sessions) != 1 || uuidToString(own.Sessions[0].ID) != registered.ID {
		t.Fatalf("PAT daemon list = %+v", own)
	}
	// A daemon token needs no body machine id, and cannot spoof another one.
	request.PID++
	request.DaemonID = ""
	testutil.Call(t, h.RegisterAgentSession, agentSessionDaemonRequest(http.MethodPost, "/api/daemon/agent-sessions", testWorkspaceID, daemonID, request)).Want(http.StatusCreated)
	request.DaemonID = "other-machine"
	testutil.Call(t, h.RegisterAgentSession, agentSessionDaemonRequest(http.MethodPost, "/api/daemon/agent-sessions", testWorkspaceID, daemonID, request)).Want(http.StatusForbidden)
	path = agentSessionDaemonURL("", testWorkspaceID, "other-machine")
	testutil.Call(t, h.ListDaemonAgentSessions, agentSessionDaemonRequest(http.MethodGet, path, testWorkspaceID, daemonID, nil)).Want(http.StatusForbidden)
	request.DaemonID = ""
	testutil.Call(t, h.RegisterAgentSession, newRequest(http.MethodPost, "/api/daemon/agent-sessions", request)).Want(http.StatusBadRequest)
	path = agentSessionDaemonURL("", testWorkspaceID, "")
	testutil.Call(t, h.ListDaemonAgentSessions, newRequest(http.MethodGet, path, nil)).Want(http.StatusBadRequest)
	// The fallback must still refuse a nonmember workspace.
	otherWorkspace := dbfx.Workspace(t, "No session membership", "session-nonmember-"+uuid.NewString())
	path = agentSessionDaemonURL("", otherWorkspace, daemonID)
	testutil.Call(t, h.ListDaemonAgentSessions, newRequest(http.MethodGet, path, nil)).Want(http.StatusNotFound)
}

func TestAgentSessionHandlersRejectMalformedInput(t *testing.T) {
	h, agentID, _, daemonID := agentSessionHandlerFixture(t)
	for _, tc := range []struct {
		name string
		edit func(*RegisterAgentSessionRequest)
	}{
		{"workspace uuid", func(req *RegisterAgentSessionRequest) { req.WorkspaceID = "bad" }},
		{"agent uuid", func(req *RegisterAgentSessionRequest) { req.AgentID = "bad" }},
		{"pid", func(req *RegisterAgentSessionRequest) { req.PID = 0 }},
		{"process start", func(req *RegisterAgentSessionRequest) { req.ProcessStartedAt = time.Time{} }},
		{"work directory", func(req *RegisterAgentSessionRequest) { req.WorkDir = "" }},
		{"status", func(req *RegisterAgentSessionRequest) { req.Status = "starting" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := agentSessionWriteRequest(agentID, daemonID, 5105)
			tc.edit(&req)
			testutil.Call(t, h.RegisterAgentSession, agentSessionDaemonRequest(http.MethodPost, "/api/daemon/agent-sessions", testWorkspaceID, daemonID, req)).Want(http.StatusBadRequest)
		})
	}
	for _, handle := range []http.HandlerFunc{h.RegisterAgentSession, h.UpdateAgentSession} {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{"))
		req = withURLParam(req, "id", uuid.NewString())
		testutil.Call(t, handle, req).Want(http.StatusBadRequest)
	}
	for _, handle := range []http.HandlerFunc{h.UpdateAgentSession, h.EndAgentSession} {
		testutil.Call(t, handle, withURLParam(newRequest(http.MethodDelete, "/", nil), "id", "bad")).Want(http.StatusBadRequest)
	}
	testutil.Call(t, h.ListAgentSessions, withURLParam(newRequest(http.MethodGet, "/", nil), "id", "bad")).Want(http.StatusBadRequest)
	path := agentSessionDaemonURL("", "bad", daemonID)
	testutil.Call(t, h.ListDaemonAgentSessions, agentSessionDaemonRequest(http.MethodGet, path, testWorkspaceID, daemonID, nil)).Want(http.StatusBadRequest)
	id := agentSessionRegisterHTTP(t, h, agentSessionWriteRequest(agentID, daemonID, 5105))
	patch := UpdateAgentSessionRequest{WorkspaceID: testWorkspaceID, DaemonID: daemonID, Status: "starting"}
	req := agentSessionDaemonRequest(http.MethodPatch, "/", testWorkspaceID, daemonID, patch)
	testutil.Call(t, h.UpdateAgentSession, withURLParam(req, "id", id)).Want(http.StatusBadRequest)
	var status string
	dbfx.QueryRow(t, `SELECT status FROM agent_session WHERE id = $1`, id).Scan(&status)
	if status != "idle" {
		t.Fatalf("invalid patch changed stored status to %q", status)
	}
	patch.Status = "working"
	req = agentSessionDaemonRequest(http.MethodPatch, "/", testWorkspaceID, daemonID, patch)
	testutil.Call(t, h.UpdateAgentSession, withURLParam(req, "id", uuid.NewString())).Want(http.StatusNotFound)
	var count int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM agent_session WHERE agent_id = $1`, agentID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("malformed writes affected rows: count=%d err=%v", count, err)
	}
}
