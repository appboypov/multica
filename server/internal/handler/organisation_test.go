package handler

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestOrganisationCreateListGetAndKeyUpdate(t *testing.T) {
	workspaceID := dbfx.Workspace(t, "Organisation endpoints", "org-endpoints-"+uuid.NewString())
	dbfx.Cleanup(t, `DELETE FROM organisation WHERE workspace_id = $1`, workspaceID)
	h := *testHandler
	h.Bus = events.New()
	var empty struct {
		Organisations []OrganisationResponse `json:"organisations"`
		Total         int                    `json:"total"`
	}
	testutil.Call(t, testHandler.ListOrganisations, testutil.WithHeaders(newRequest("GET", "/api/organisations", nil), "X-Workspace-ID", workspaceID)).Want(http.StatusOK).JSON(&empty)
	if empty.Organisations == nil || len(empty.Organisations) != 0 || empty.Total != 0 {
		t.Fatalf("empty list = %+v, want a non-null empty array and zero total", empty)
	}
	var published []events.Event
	h.Bus.SubscribeAll(func(event events.Event) { published = append(published, event) })

	create := func(name, key string) OrganisationResponse {
		t.Helper()
		var org OrganisationResponse
		req := testutil.WithHeaders(newRequest("POST", "/api/organisations", map[string]any{"name": name, "key": key}), "X-Workspace-ID", workspaceID)
		testutil.Call(t, h.CreateOrganisation, req).Want(http.StatusCreated).JSON(&org)
		if org.ID == "" || org.WorkspaceID != workspaceID || org.CreatedAt == "" || org.UpdatedAt == "" {
			t.Fatalf("incomplete organisation response: %+v", org)
		}
		return org
	}
	rebl := create("  Rebl Records  ", "rr")
	das := create("De App Specialist", "das")
	if rebl.Name != "Rebl Records" {
		t.Fatalf("name was not trimmed: %q", rebl.Name)
	}

	var list struct {
		Organisations []OrganisationResponse `json:"organisations"`
		Total         int                    `json:"total"`
	}
	testutil.Call(t, h.ListOrganisations, testutil.WithHeaders(newRequest("GET", "/api/organisations", nil), "X-Workspace-ID", workspaceID)).Want(http.StatusOK).JSON(&list)
	if list.Total != 2 || len(list.Organisations) != 2 || list.Organisations[0].ID != das.ID || list.Organisations[1].ID != rebl.ID {
		t.Fatalf("organisations must list by name: %+v", list)
	}

	var fetched OrganisationResponse
	get := testutil.WithHeaders(withURLParam(newRequest("GET", "/api/organisations/"+rebl.ID, nil), "id", rebl.ID), "X-Workspace-ID", workspaceID)
	testutil.Call(t, h.GetOrganisation, get).Want(http.StatusOK).JSON(&fetched)
	if fetched.ID != rebl.ID || fetched.Name != rebl.Name || fetched.Key != "rr" {
		t.Fatalf("get organisation = %+v, want %+v", fetched, rebl)
	}

	var updated OrganisationResponse
	update := testutil.WithHeaders(withURLParam(newRequest("PUT", "/api/organisations/"+rebl.ID, map[string]any{"key": "rebl", "name": "Cannot rename"}), "id", rebl.ID), "X-Workspace-ID", workspaceID)
	testutil.Call(t, h.UpdateOrganisation, update).Want(http.StatusOK).JSON(&updated)
	if updated.Key != "rebl" || updated.Name != "Rebl Records" || updated.ID != rebl.ID {
		t.Fatalf("key-only update = %+v", updated)
	}
	if len(published) != 3 {
		t.Fatalf("write events = %d, want 3", len(published))
	}
	for i, want := range []struct{ eventType, id string }{
		{protocol.EventOrganisationCreated, rebl.ID},
		{protocol.EventOrganisationCreated, das.ID},
		{protocol.EventOrganisationUpdated, rebl.ID},
	} {
		event := published[i]
		payload, ok := event.Payload.(protocol.OrganisationEventPayload)
		if !ok || payload.OrganisationID != want.id || event.Type != want.eventType || event.WorkspaceID != workspaceID || event.ActorID != testUserID {
			t.Fatalf("write event %d = %+v, payload %+v", i, event, payload)
		}
	}
}

func TestOrganisationRejectsInvalidAndTakenValues(t *testing.T) {
	orgID := dbfx.Insert(t, "organisation", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Rebl Records", "key": "rebl"})
	dbfx.Insert(t, "organisation", testutil.Cols{"workspace_id": testWorkspaceID, "name": "De App Specialist", "key": "das"})
	h := *testHandler
	h.Bus = events.New()
	writes := 0
	h.Bus.SubscribeAll(func(events.Event) { writes++ })

	for _, tc := range []struct {
		name   string
		body   map[string]any
		status int
		error  string
	}{
		{"name taken ignoring case", map[string]any{"name": " rebl records ", "key": "rr"}, http.StatusConflict, "organisation name is taken"},
		{"key taken", map[string]any{"name": "Das Studio", "key": "das"}, http.StatusConflict, "organisation key is taken"},
		{"empty name", map[string]any{"name": "  ", "key": "empty"}, http.StatusBadRequest, "name is required"},
		{"missing name", map[string]any{"key": "empty"}, http.StatusBadRequest, "name is required"},
		{"empty key", map[string]any{"name": "Empty", "key": ""}, http.StatusBadRequest, ""},
		{"missing key", map[string]any{"name": "Empty"}, http.StatusBadRequest, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := testutil.Call(t, h.CreateOrganisation, newRequest("POST", "/api/organisations", tc.body)).Want(tc.status).Map()
			if tc.error != "" && body["error"] != tc.error {
				t.Fatalf("error = %v, want %q", body["error"], tc.error)
			}
		})
	}
	for _, key := range []string{"Rebl Records", "UPPER", "-leading", "../rebl", "rebl/key", "rebl_key", " rebl", "rebl\n"} {
		t.Run("invalid key "+key, func(t *testing.T) {
			testutil.Call(t, h.CreateOrganisation, newRequest("POST", "/api/organisations", map[string]any{"name": "Invalid", "key": key})).Want(http.StatusBadRequest)
			req := withURLParam(newRequest("PUT", "/api/organisations/"+orgID, map[string]any{"key": key}), "id", orgID)
			testutil.Call(t, h.UpdateOrganisation, req).Want(http.StatusBadRequest)
		})
	}
	update := withURLParam(newRequest("PUT", "/api/organisations/"+orgID, map[string]any{"key": "das"}), "id", orgID)
	body := testutil.Call(t, h.UpdateOrganisation, update).Want(http.StatusConflict).Map()
	if body["error"] != "organisation key is taken" {
		t.Fatalf("taken-key update error = %v", body["error"])
	}
	for _, body := range []map[string]any{{}, {"key": nil}} {
		testutil.Call(t, h.UpdateOrganisation, withURLParam(newRequest("PUT", "/api/organisations/"+orgID, body), "id", orgID)).Want(http.StatusBadRequest)
	}
	var key string
	dbfx.QueryRow(t, `SELECT key FROM organisation WHERE id = $1`, orgID).Scan(&key)
	if key != "rebl" || writes != 0 {
		t.Fatalf("rejected writes changed key or published events: key=%q events=%d", key, writes)
	}
}

func TestOrganisationWorkspaceIsolationAndMissingIDs(t *testing.T) {
	otherWorkspaceID := dbfx.Workspace(t, "Other organisation workspace", "other-org-"+uuid.NewString())
	foreignID := dbfx.Insert(t, "organisation", testutil.Cols{"workspace_id": otherWorkspaceID, "name": "Foreign", "key": "foreign"})
	localID := dbfx.Insert(t, "organisation", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Foreign", "key": "foreign"})
	var list struct {
		Organisations []OrganisationResponse `json:"organisations"`
	}
	testutil.Call(t, testHandler.ListOrganisations, newRequest("GET", "/api/organisations", nil)).Want(http.StatusOK).JSON(&list)
	foundLocal := false
	for _, org := range list.Organisations {
		if org.ID == foreignID {
			t.Fatal("another workspace's organisation appeared in list")
		}
		foundLocal = foundLocal || org.ID == localID
	}
	if !foundLocal {
		t.Fatal("local organisation is absent from list")
	}
	for _, id := range []string{foreignID, uuid.NewString(), "not-a-uuid"} {
		status := http.StatusNotFound
		if id == "not-a-uuid" {
			status = http.StatusBadRequest
		}
		testutil.Call(t, testHandler.GetOrganisation, withURLParam(newRequest("GET", "/api/organisations/"+id, nil), "id", id)).Want(status)
		testutil.Call(t, testHandler.UpdateOrganisation, withURLParam(newRequest("PUT", "/api/organisations/"+id, map[string]any{"key": "changed"}), "id", id)).Want(status)
	}
	var key string
	dbfx.QueryRow(t, `SELECT key FROM organisation WHERE id = $1`, foreignID).Scan(&key)
	if key != "foreign" {
		t.Fatalf("foreign key changed to %q", key)
	}
}
