package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type clientReadFixture struct {
	rows                                   *testutil.Fixture
	user, other, member                    string
	orgs, projects, repositories, releases [3]string
	waits, planned, hidden                 []string
}

func newClientReadFixture(t *testing.T) clientReadFixture {
	t.Helper()
	ws := dbfx.Workspace(t, "Client reads", "client-read-"+uuid.NewString())
	f := clientReadFixture{rows: testutil.New(testPool, ws, testUserID)}
	for i, name := range []string{"Alpha", "Beta", "Gamma"} {
		f.orgs[i] = dbfx.Insert(t, "organisation", testutil.Cols{"workspace_id": ws, "name": name, "key": strings.ToLower(name)})
		f.projects[i] = dbfx.Project(t, name+" project", testutil.Cols{"workspace_id": ws, "organisation_id": f.orgs[i]})
		f.repositories[i] = dbfx.Insert(t, "repository", testutil.Cols{
			"workspace_id": ws, "project_id": f.projects[i], "github_repo_id": i + 1,
			"full_name": "test/" + uuid.NewString(), "default_branch": "main", "version_policy": "tags",
			"private": false, "checkout_daemon_id": "fixture", "checkout_path": "/fixture",
		})
		cols := testutil.Cols{"workspace_id": ws, "repository_id": f.repositories[i], "position": 0, "version": "1.2.3", "title": "PRIVATE-RELEASE"}
		if i == 1 {
			cols["shipped_at"] = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
		}
		f.releases[i] = dbfx.Insert(t, "release", cols)
	}
	f.user = dbfx.User(t, "Contact A", "client-a-"+uuid.NewString()+"@example.invalid")
	f.member = dbfx.Member(t, ws, f.user, "client", testutil.Cols{"organisation_ids": []string{f.orgs[0], f.orgs[1]}})
	f.other = dbfx.User(t, "Other contact", "client-other-"+uuid.NewString()+"@example.invalid")
	dbfx.Member(t, ws, f.other, "client", testutil.Cols{"organisation_ids": []string{f.orgs[0]}})
	b := dbfx.User(t, "Contact B", "client-b-"+uuid.NewString()+"@example.invalid")
	dbfx.Member(t, ws, b, "client", testutil.Cols{"organisation_ids": []string{f.orgs[2]}})
	for _, category := range []string{"done", "closed"} {
		dbfx.Insert(t, "issue_status", testutil.Cols{
			"workspace_id": ws, "key": "custom_" + category, "name": "Custom " + category,
			"category": category, "color": "#123456", "icon": "circle", "archived_at": time.Now(),
		})
	}
	dbfx.Insert(t, "issue_status", testutil.Cols{"workspace_id": ws, "key": "todo", "name": "Ready for client", "category": "unstarted", "color": "#123456", "icon": "circle"})
	add := func(title string, cols testutil.Cols) string {
		t.Helper()
		base := testutil.Cols{
			"project_id": f.projects[0], "type": "chore", "assignee_type": "member", "assignee_id": f.user,
			"description": "Own wait detail", "created_at": time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
			"properties": testutil.Raw(`'{"Points":99,"secret":"PRIVATE-PROPERTIES"}'::jsonb`),
			"metadata":   testutil.Raw(`'{"secret":"PRIVATE-METADATA"}'::jsonb`),
		}
		for k, v := range cols {
			base[k] = v
		}
		return f.rows.Issue(t, title, base)
	}
	// Unassigned work reaches a client only as planned work, never as a wait.
	unassigned := func(cols testutil.Cols) testutil.Cols {
		cols["assignee_type"], cols["assignee_id"] = nil, nil
		return cols
	}
	early := add("Unreleased own wait", testutil.Cols{"due_date": "2026-10-01"})
	tieA := add("Released own wait", testutil.Cols{"due_date": "2026-10-05", "release_id": f.releases[0], "position": 2})
	tieB := add("Second organisation own wait", testutil.Cols{"due_date": "2026-10-05", "project_id": f.projects[1]})
	// A wait is any open issue assigned to the client, whatever its type.
	null := add("Undated own story", testutil.Cols{"type": "story", "description": nil})
	ties := []string{tieA, tieB}
	slices.Sort(ties)
	f.waits = append([]string{early}, ties...)
	f.waits = append(f.waits, null)
	story := add("Released story", unassigned(testutil.Cols{"type": "story", "release_id": f.releases[0], "position": 0, "description": "PRIVATE-DESCRIPTION", "due_date": "2026-12-31"}))
	otherWait := add("Other contact released wait", testutil.Cols{"assignee_id": f.other, "release_id": f.releases[0], "position": 1, "description": "PRIVATE-OTHER-CONTACT"})
	doneStory := add("Shipped closed work", testutil.Cols{"type": "story", "project_id": f.projects[1], "release_id": f.releases[1], "status": "custom_done", "description": "PRIVATE-DESCRIPTION"})
	f.planned = []string{story, otherWait, tieA, doneStory}
	for _, issue := range []string{early, story} {
		comment := f.rows.Comment(t, issue, "PRIVATE-COMMENT")
		dbfx.Insert(t, "attachment", testutil.Cols{
			"workspace_id": ws, "issue_id": issue, "uploader_type": "member", "uploader_id": testUserID,
			"filename": "PRIVATE-ATTACHMENT", "url": "https://example.invalid/private", "content_type": "text/plain", "size_bytes": 1,
		})
		dbfx.Insert(t, "issue_source_context", testutil.Cols{
			"id": uuid.NewString(), "workspace_id": ws, "issue_id": issue, "source_issue_id": issue,
			"anchor_comment_id": comment, "captured_by_user_id": testUserID, "snapshot_version": 1,
			"snapshot": testutil.Raw(`'{"secret":"PRIVATE-SOURCE"}'::jsonb`), "capture_digest": "fixture",
			"state": "attached", "attached_at": time.Now(),
		})
	}
	f.hidden = append(f.hidden, otherWait, story, doneStory)
	for _, category := range []string{"done", "cancelled", "custom_done", "custom_closed"} {
		f.hidden = append(f.hidden, add("Terminal wait", testutil.Cols{"status": category}))
	}
	f.hidden = append(f.hidden,
		add("Wrong assignee type", testutil.Cols{"assignee_type": "agent"}),
		add("Unreleased story", unassigned(testutil.Cols{"type": "story"})),
		add("PRIVATE-FOREIGN", testutil.Cols{"project_id": f.projects[2], "release_id": f.releases[2]}),
		add("Wrong release project", unassigned(testutil.Cols{"type": "story", "release_id": f.releases[2]})),
		add("Dangling release", unassigned(testutil.Cols{"type": "story", "release_id": uuid.NewString()})),
	)
	noOrg := dbfx.Project(t, "No organisation", testutil.Cols{"workspace_id": ws})
	f.hidden = append(f.hidden, add("No organisation wait", testutil.Cols{"project_id": noOrg, "release_id": f.releases[0]}))
	return f
}

func (f clientReadFixture) request(path string) *http.Request {
	return testutil.WithHeaders(testutil.JSONRequest("GET", path, nil), "X-User-ID", f.user)
}

func requireClientReadKeys(t *testing.T, value map[string]any, want ...string) {
	t.Helper()
	got := make([]string, 0, len(value))
	for key := range value {
		got = append(got, key)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("response keys = %v, want %v", got, want)
	}
}

func TestClientReadVisibilityAndProjection(t *testing.T) {
	f := newClientReadFixture(t)
	var waits ClientWaitListResponse
	response := testutil.Call(t, testHandler.ListClientWaits, f.request("/api/me/waits")).Want(200).JSON(&waits)
	if waits.Total != 4 {
		t.Fatalf("wait total = %d", waits.Total)
	}
	got := make([]string, len(waits.Waits))
	for i, row := range waits.Waits {
		got[i] = row.ID
	}
	if !slices.Equal(got, f.waits) {
		t.Fatalf("wait order = %v, want %v", got, f.waits)
	}
	requireClientReadKeys(t, response.Map(), "waits", "total")
	for _, row := range waits.Waits {
		var detail map[string]any
		testutil.Call(t, testHandler.GetClientWait, testutil.WithURLParams(f.request("/api/me/waits/"+row.ID), "id", row.ID)).Want(200).JSON(&detail)
		requireClientReadKeys(t, detail, "id", "title", "description", "due_date", "organisation_name")
		encoded, _ := json.Marshal(row)
		var listed map[string]any
		if err := json.Unmarshal(encoded, &listed); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(detail, listed) {
			t.Fatalf("detail differs from list: %v vs %v", detail, listed)
		}
	}
	last := waits.Waits[len(waits.Waits)-1]
	if last.Description != nil || last.DueDate != nil {
		t.Fatalf("nullable wait fields = %+v", last)
	}
	if waits.Waits[0].Description == nil || *waits.Waits[0].Description != "Own wait detail" || *waits.Waits[0].DueDate != "2026-10-01" {
		t.Fatalf("own detail = %+v", waits.Waits[0])
	}
	orgs := map[string]bool{}
	for _, row := range waits.Waits {
		orgs[row.OrganisationName] = true
	}
	if !reflect.DeepEqual(orgs, map[string]bool{"Alpha": true, "Beta": true}) {
		t.Fatalf("organisations = %v", orgs)
	}

	var planned ClientPlannedWorkListResponse
	planResponse := testutil.Call(t, testHandler.ListClientPlannedWork, f.request("/api/me/planned-work")).Want(200).JSON(&planned)
	requireClientReadKeys(t, planResponse.Map(), "issues", "total")
	got = make([]string, len(planned.Issues))
	var raw struct {
		Issues []map[string]any `json:"issues"`
	}
	planResponse.JSON(&raw)
	for i, row := range planned.Issues {
		got[i] = row.ID
		requireClientReadKeys(t, raw.Issues[i], "id", "title", "status", "organisation_name", "project_name", "release")
		requireClientReadKeys(t, raw.Issues[i]["release"].(map[string]any), "version", "shipped")
		wantOrg, wantProject, wantStatus, shipped := "Alpha", "Alpha project", "Ready for client", false
		if i == 3 {
			wantOrg, wantProject, wantStatus, shipped = "Beta", "Beta project", "Custom done", true
		}
		if row.OrganisationName != wantOrg || row.ProjectName != wantProject || row.Status != wantStatus || row.Release.Version != "1.2.3" || row.Release.Shipped != shipped {
			t.Fatalf("planned context = %+v", row)
		}
	}
	if planned.Total != 4 || !slices.Equal(got, f.planned) {
		t.Fatalf("planned IDs = %v total=%d, want %v", got, planned.Total, f.planned)
	}
	for _, payload := range []string{response.Text(), planResponse.Text()} {
		for _, secret := range []string{"PRIVATE-", f.other, f.orgs[0], f.releases[0], f.projects[0]} {
			if strings.Contains(payload, secret) {
				t.Fatalf("client response disclosed %q: %s", secret, payload)
			}
		}
	}
}

func TestClientReadHiddenAndMissingDetailsMatch(t *testing.T) {
	f := newClientReadFixture(t)
	var baseline string
	for _, id := range append([]string{"not-a-uuid", uuid.NewString(), "WORK-1071"}, f.hidden...) {
		response := testutil.Call(t, testHandler.GetClientWait, testutil.WithURLParams(f.request("/api/me/waits/"+id), "id", id)).Want(404)
		if baseline == "" {
			baseline = response.Text()
		}
		if response.Text() != baseline {
			t.Fatalf("hidden/missing responses differ for %s: %s vs %s", id, response.Text(), baseline)
		}
	}
}

func TestClientReadPagesAndForgedScope(t *testing.T) {
	f := newClientReadFixture(t)
	for _, table := range []struct {
		path, key string
		h         http.HandlerFunc
		want      []string
	}{
		{"/api/me/waits", "waits", testHandler.ListClientWaits, f.waits},
		{"/api/me/planned-work", "issues", testHandler.ListClientPlannedWork, f.planned},
	} {
		t.Run(table.key, func(t *testing.T) {
			var all []string
			for _, offset := range []int{0, 2, 4, 100} {
				req := f.request(fmt.Sprintf("%s?limit=2&offset=%d&organisation_id=%s&project_id=%s&assignee_id=%s&release_id=%s&issue_id=%s&open_only=false&fields=*&include=comments", table.path, offset, f.orgs[2], f.projects[2], f.other, f.releases[2], f.hidden[0]))
				req.Header.Set("X-Workspace-ID", testWorkspaceID)
				out := testutil.Call(t, table.h, req).Want(200).Map()
				if out["total"] != float64(4) {
					t.Fatalf("offset %d total=%v", offset, out["total"])
				}
				rows, ok := out[table.key].([]any)
				if !ok {
					t.Fatalf("list is not an array: %v", out)
				}
				wantN := 0
				if offset < 4 {
					wantN = 2
				}
				if len(rows) != wantN {
					t.Fatalf("page %d rows=%v", offset, rows)
				}
				for _, row := range rows {
					all = append(all, row.(map[string]any)["id"].(string))
				}
			}
			if !slices.Equal(all, table.want) {
				t.Fatalf("paged IDs = %v, want %v", all, table.want)
			}
			for _, query := range []string{"?limit=0&offset=-1", "?limit=bad&offset=bad", "?limit=2147483648&offset=2147483648", "?limit=999999999999999999999999&offset=999999999999999999999999"} {
				out := testutil.Call(t, table.h, f.request(table.path+query)).Want(200).Map()
				if len(out[table.key].([]any)) != 4 || out[table.key].([]any)[0].(map[string]any)["id"] != table.want[0] {
					t.Fatalf("unsafe pagination %s: %v", query, out)
				}
			}
		})
	}
}

func TestClientReadLiveMembershipAndPlacement(t *testing.T) {
	f := newClientReadFixture(t)
	// Reassignment removes both the list row and detail from the old contact.
	dbfx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, f.waits[0], f.other)
	var waits ClientWaitListResponse
	testutil.Call(t, testHandler.ListClientWaits, f.request("/api/me/waits")).Want(200).JSON(&waits)
	if waits.Total != 3 {
		t.Fatalf("reassigned wait total=%d", waits.Total)
	}
	testutil.Call(t, testHandler.GetClientWait, testutil.WithURLParams(f.request("/api/me/waits/"+f.waits[0]), "id", f.waits[0])).Want(404)
	// A completed released wait stays in planned work, never in personal detail.
	releasedWait := f.planned[2]
	dbfx.Exec(t, `UPDATE issue SET status='custom_closed' WHERE id=$1`, releasedWait)
	testutil.Call(t, testHandler.GetClientWait, testutil.WithURLParams(f.request("/api/me/waits/"+releasedWait), "id", releasedWait)).Want(404)
	testutil.Call(t, testHandler.ListClientWaits, f.request("/api/me/waits")).Want(200).JSON(&waits)
	if waits.Total != 2 {
		t.Fatalf("completed wait total=%d", waits.Total)
	}
	var planned ClientPlannedWorkListResponse
	testutil.Call(t, testHandler.ListClientPlannedWork, f.request("/api/me/planned-work")).Want(200).JSON(&planned)
	if planned.Total != 4 || planned.Issues[2].ID != releasedWait {
		t.Fatalf("completion changed planned visibility: %+v", planned)
	}
	// Removing an organisation takes effect on the next list and detail read.
	dbfx.Exec(t, `UPDATE member SET organisation_ids=$2::uuid[] WHERE id=$1`, f.member, []string{f.orgs[0]})
	testutil.Call(t, testHandler.ListClientWaits, f.request("/api/me/waits")).Want(200).JSON(&waits)
	if waits.Total != 1 || waits.Waits[0].ID != f.waits[3] {
		t.Fatalf("removed organisation waits=%+v", waits)
	}
	for _, id := range f.waits[1:3] {
		testutil.Call(t, testHandler.GetClientWait, testutil.WithURLParams(f.request("/api/me/waits/"+id), "id", id)).Want(404)
	}
	testutil.Call(t, testHandler.ListClientPlannedWork, f.request("/api/me/planned-work")).Want(200).JSON(&planned)
	if planned.Total != 3 {
		t.Fatalf("removed organisation planned total=%d", planned.Total)
	}
	dbfx.Exec(t, `UPDATE issue SET release_id=NULL WHERE id=$1`, f.planned[0])
	testutil.Call(t, testHandler.ListClientPlannedWork, f.request("/api/me/planned-work")).Want(200).JSON(&planned)
	if planned.Total != 2 || planned.Issues[0].ID != f.planned[1] {
		t.Fatalf("removed release planned=%+v", planned)
	}
	// Deleted soft references cannot keep a formerly visible row alive.
	dbfx.Exec(t, `DELETE FROM organisation WHERE id=$1`, f.orgs[0])
	testutil.Call(t, testHandler.ListClientWaits, f.request("/api/me/waits")).Want(200).JSON(&waits)
	testutil.Call(t, testHandler.ListClientPlannedWork, f.request("/api/me/planned-work")).Want(200).JSON(&planned)
	if waits.Total != 0 || waits.Waits == nil || len(waits.Waits) != 0 || planned.Total != 0 || planned.Issues == nil || len(planned.Issues) != 0 {
		t.Fatalf("deleted organisation retained rows: %+v %+v", waits, planned)
	}
}

func TestClientReadMembershipAndDatabaseErrors(t *testing.T) {
	f := newClientReadFixture(t)
	for _, h := range []http.HandlerFunc{testHandler.ListClientWaits, testHandler.GetClientWait, testHandler.ListClientPlannedWork} {
		testutil.Call(t, h, testutil.JSONRequest("GET", "/api/me/waits", nil)).Want(401)
		testutil.Call(t, h, testutil.WithHeaders(testutil.JSONRequest("GET", "/api/me/waits", nil), "X-User-ID", testUserID)).Want(403)
	}
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	broken := *testHandler
	broken.Queries = db.New(tx)
	for _, h := range []http.HandlerFunc{broken.ListClientWaits, broken.GetClientWait, broken.ListClientPlannedWork} {
		testutil.Call(t, h, f.request("/api/me/waits")).Want(500)
	}
	dbfx.Exec(t, `DELETE FROM member WHERE id=$1`, f.member)
	testutil.Call(t, testHandler.ListClientWaits, f.request("/api/me/waits")).Want(403)
}

func TestClientReadLimitCap(t *testing.T) {
	f := newClientReadFixture(t)
	for i := range 97 {
		f.rows.Issue(t, fmt.Sprintf("Paged wait %d", i), testutil.Cols{
			"project_id": f.projects[0], "release_id": f.releases[0], "type": "chore",
			"assignee_type": "member", "assignee_id": f.user,
		})
	}
	for _, list := range []struct {
		key string
		h   http.HandlerFunc
	}{{"waits", testHandler.ListClientWaits}, {"issues", testHandler.ListClientPlannedWork}} {
		for _, limit := range []string{"101", "2147483648", "999999999999999999999999"} {
			out := testutil.Call(t, list.h, f.request("/api/me/waits?limit="+limit)).Want(200).Map()
			if out["total"] != float64(101) || len(out[list.key].([]any)) != 100 {
				t.Fatalf("%s limit %s: %v", list.key, limit, out)
			}
		}
	}
}

func TestClientReadLiveSoftReferences(t *testing.T) {
	f := newClientReadFixture(t)
	dbfx.Exec(t, `UPDATE member SET organisation_ids=$2::uuid[] WHERE id=$1`, f.member, []string{f.orgs[0], f.orgs[0], f.orgs[1], uuid.NewString()})
	var waits ClientWaitListResponse
	var planned ClientPlannedWorkListResponse
	testutil.Call(t, testHandler.ListClientWaits, f.request("/api/me/waits")).Want(200).JSON(&waits)
	testutil.Call(t, testHandler.ListClientPlannedWork, f.request("/api/me/planned-work")).Want(200).JSON(&planned)
	if waits.Total != 4 || planned.Total != 4 {
		t.Fatalf("duplicate/stale membership multiplied rows: %+v %+v", waits, planned)
	}
	foreignWS := dbfx.Workspace(t, "Foreign scope", "client-scope-"+uuid.NewString())
	foreignOrg := clientTestOrganisation(t, foreignWS)
	dbfx.Exec(t, `UPDATE project SET organisation_id=$2 WHERE id=$1`, f.projects[0], foreignOrg)
	dbfx.Exec(t, `UPDATE member SET organisation_ids=$2::uuid[] WHERE id=$1`, f.member, []string{foreignOrg, f.orgs[1]})
	testutil.Call(t, testHandler.ListClientWaits, f.request("/api/me/waits")).Want(200).JSON(&waits)
	testutil.Call(t, testHandler.ListClientPlannedWork, f.request("/api/me/planned-work")).Want(200).JSON(&planned)
	if waits.Total != 1 || waits.Waits[0].OrganisationName != "Beta" || planned.Total != 1 || planned.Issues[0].ID != f.planned[3] {
		t.Fatalf("foreign organisation widened scope: %+v %+v", waits, planned)
	}
	dbfx.Exec(t, `UPDATE issue SET status='no_catalogue_row' WHERE id=$1`, f.planned[3])
	testutil.Call(t, testHandler.ListClientPlannedWork, f.request("/api/me/planned-work")).Want(200).JSON(&planned)
	if planned.Issues[0].Status != "no_catalogue_row" {
		t.Fatalf("missing catalogue fallback = %+v", planned)
	}
	dbfx.Exec(t, `UPDATE project SET workspace_id=$2 WHERE id=$1`, f.projects[1], foreignWS)
	testutil.Call(t, testHandler.ListClientWaits, f.request("/api/me/waits")).Want(200).JSON(&waits)
	testutil.Call(t, testHandler.ListClientPlannedWork, f.request("/api/me/planned-work")).Want(200).JSON(&planned)
	if waits.Total != 0 || planned.Total != 0 {
		t.Fatalf("foreign project widened scope: %+v %+v", waits, planned)
	}
}
