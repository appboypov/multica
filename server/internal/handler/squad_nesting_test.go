package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func addSquadMemberAs(t *testing.T, userID, squadID, memberType, memberID string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	testHandler.AddSquadMember(w, squadScopeReq(userID, "POST", "/api/squads/members", map[string]any{
		"member_type": memberType,
		"member_id":   memberID,
	}, map[string]string{"id": squadID}))
	return w
}

func wantRefusal(t *testing.T, w *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, want %d: %s", w.Code, status, w.Body.String())
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error != message {
		t.Fatalf("error = %q, want %q", body.Error, message)
	}
}

func TestAddSquadMember_SquadMember(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	tech := createSquadAs(t, "", "Nest Tech", createHandlerTestAgent(t, "nest-add-mitchel", nil))
	backend := createSquadAs(t, "", "Nest Backend", createHandlerTestAgent(t, "nest-add-joeri", nil))

	w := addSquadMemberAs(t, "", tech.ID, "squad", backend.ID)
	if w.Code != http.StatusCreated {
		t.Fatalf("add squad: status %d: %s", w.Code, w.Body.String())
	}
	var sm SquadMemberResponse
	if err := json.NewDecoder(w.Body).Decode(&sm); err != nil {
		t.Fatal(err)
	}
	if sm.MemberType != "squad" || sm.MemberID != backend.ID {
		t.Fatalf("member = %+v, want squad %s", sm, backend.ID)
	}

	// A squad can sit in several squads.
	ops := createSquadAs(t, "", "Nest Ops", createHandlerTestAgent(t, "nest-add-anouk", nil))
	if w := addSquadMemberAs(t, "", ops.ID, "squad", backend.ID); w.Code != http.StatusCreated {
		t.Fatalf("add squad to second main squad: status %d: %s", w.Code, w.Body.String())
	}
}

func TestAddSquadMember_RefusesUnknownOrArchivedSquad(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	tech := createSquadAs(t, "", "Nest Missing Tech", createHandlerTestAgent(t, "nest-missing-mitchel", nil))
	archived := createSquadAs(t, "", "Nest Archived", createHandlerTestAgent(t, "nest-missing-joeri", nil))
	dbfx.Exec(t, `UPDATE squad SET archived_at = now() WHERE id = $1`, archived.ID)

	for name, id := range map[string]string{"unknown": uuid.NewString(), "archived": archived.ID} {
		t.Run(name, func(t *testing.T) {
			wantRefusal(t, addSquadMemberAs(t, "", tech.ID, "squad", id), http.StatusBadRequest, "squad not found in this workspace")
		})
	}
}

// A sub-squad routes work to its leader, so a non-admin creator may add it
// only when they may add that leader agent (MUL-4223); the owner may still.
func TestAddSquadMember_CreatorSquadLeaderAccessGate(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	privateAgentID, _, memberID := privateAgentTestFixture(t)
	tech := createSquadAs(t, memberID, "Nest Gate Tech", createHandlerTestAgent(t, "nest-gate-mitchel", nil))
	private := createSquadAs(t, "", "Nest Gate Private", privateAgentID)

	wantRefusal(t, addSquadMemberAs(t, memberID, tech.ID, "squad", private.ID), http.StatusForbidden,
		"you can only add a squad whose leader you have access to")
	if w := addSquadMemberAs(t, "", tech.ID, "squad", private.ID); w.Code != http.StatusCreated {
		t.Fatalf("add squad as owner: status %d: %s", w.Code, w.Body.String())
	}
}

func TestAddSquadMember_RefusesLoops(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	tech := createSquadAs(t, "", "Loop Tech", createHandlerTestAgent(t, "loop-mitchel", nil))
	backend := createSquadAs(t, "", "Loop Backend", createHandlerTestAgent(t, "loop-joeri", nil))
	data := createSquadAs(t, "", "Loop Data", createHandlerTestAgent(t, "loop-anouk", nil))
	for _, pair := range [][2]string{{tech.ID, backend.ID}, {backend.ID, data.ID}} {
		if w := addSquadMemberAs(t, "", pair[0], "squad", pair[1]); w.Code != http.StatusCreated {
			t.Fatalf("seed nesting: status %d: %s", w.Code, w.Body.String())
		}
	}

	cases := []struct {
		name, parent, child, message string
	}{
		{"self", tech.ID, tech.ID, "adding Loop Tech would create a loop: Loop Tech → Loop Tech"},
		{"direct", backend.ID, tech.ID, "adding Loop Tech would create a loop: Loop Backend → Loop Tech → Loop Backend"},
		{"deep", data.ID, tech.ID, "adding Loop Tech would create a loop: Loop Data → Loop Tech → Loop Backend → Loop Data"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wantRefusal(t, addSquadMemberAs(t, "", tc.parent, "squad", tc.child), http.StatusConflict, tc.message)
		})
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM squad_member WHERE member_type = 'squad' AND squad_id IN ($1, $2, $3)`, tech.ID, backend.ID, data.ID); got != 2 {
		t.Fatalf("squad memberships = %d, want the 2 seeded", got)
	}
}

// A squad reachable through several sub-squads is walked once, and the loop
// refusal names the shortest path, not the one through the first-added
// sub-squad.
func TestAddSquadMember_DiamondNesting(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	top := createSquadAs(t, "", "Diamond Top", createHandlerTestAgent(t, "diamond-mitchel", nil))
	left := createSquadAs(t, "", "Diamond Left", createHandlerTestAgent(t, "diamond-joeri", nil))
	right := createSquadAs(t, "", "Diamond Right", createHandlerTestAgent(t, "diamond-thomas", nil))
	mid := createSquadAs(t, "", "Diamond Mid", createHandlerTestAgent(t, "diamond-anouk", nil))
	bottom := createSquadAs(t, "", "Diamond Bottom", createHandlerTestAgent(t, "diamond-pieter", nil))
	// Bottom sits under Top through Left and Mid, and through Right.
	for _, pair := range [][2]string{{top.ID, left.ID}, {top.ID, right.ID}, {left.ID, mid.ID}, {mid.ID, bottom.ID}, {right.ID, bottom.ID}} {
		if w := addSquadMemberAs(t, "", pair[0], "squad", pair[1]); w.Code != http.StatusCreated {
			t.Fatalf("seed diamond: status %d: %s", w.Code, w.Body.String())
		}
	}

	// Bottom is already below Top, but Top is not below Bottom: no loop.
	if w := addSquadMemberAs(t, "", top.ID, "squad", bottom.ID); w.Code != http.StatusCreated {
		t.Fatalf("add Bottom to Top: status %d: %s", w.Code, w.Body.String())
	}
	wantRefusal(t, addSquadMemberAs(t, "", bottom.ID, "squad", top.ID), http.StatusConflict,
		"adding Diamond Top would create a loop: Diamond Bottom → Diamond Top → Diamond Bottom")
	wantRefusal(t, addSquadMemberAs(t, "", mid.ID, "squad", top.ID), http.StatusConflict,
		"adding Diamond Top would create a loop: Diamond Mid → Diamond Top → Diamond Left → Diamond Mid")
}

func TestSquadNesting_RefusesSameLeader(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	mitchel := createHandlerTestAgent(t, "same-leader-mitchel", nil)
	joeri := createHandlerTestAgent(t, "same-leader-joeri", nil)
	tech := createSquadAs(t, "", "Same Tech", mitchel)
	design := createSquadAs(t, "", "Same Design", mitchel)
	backend := createSquadAs(t, "", "Same Backend", joeri)

	wantRefusal(t, addSquadMemberAs(t, "", tech.ID, "squad", design.ID), http.StatusConflict,
		"Same Design has the same leader as Same Tech")

	if w := addSquadMemberAs(t, "", tech.ID, "squad", backend.ID); w.Code != http.StatusCreated {
		t.Fatalf("add Backend: status %d: %s", w.Code, w.Body.String())
	}
	updateLeader := func(squadID, leaderID string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		testHandler.UpdateSquad(w, squadScopeReq("", "PUT", "/api/squads", map[string]any{
			"leader_id": leaderID,
		}, map[string]string{"id": squadID}))
		return w
	}
	wantRefusal(t, updateLeader(backend.ID, mitchel), http.StatusConflict,
		"Same Backend sits in Same Tech, which same-leader-mitchel leads; a sub-squad cannot share its leader with its main squad")
	wantRefusal(t, updateLeader(tech.ID, joeri), http.StatusConflict,
		"Same Backend, a member of Same Tech, is led by same-leader-joeri; a sub-squad cannot share its leader with its main squad")

	// A leader change that pairs nothing still works.
	if w := updateLeader(backend.ID, createHandlerTestAgent(t, "same-leader-anouk", nil)); w.Code != http.StatusOK {
		t.Fatalf("unrelated leader change: status %d: %s", w.Code, w.Body.String())
	}
}

func TestBuildSquadLeaderBriefing_SubSquadRows(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	leaderID, _ := seededLeaderAgent(t)
	tech := seedSquadForBriefing(t, leaderID, "Roster Tech", "")
	backendID := dbfx.Squad(t, "Roster Backend", createHandlerTestAgent(t, "roster-joeri", []byte("[]")))
	retiredID := dbfx.Squad(t, "Roster Retired", createHandlerTestAgent(t, "roster-anouk", []byte("[]")))
	dbfx.SquadMember(t, uuidToString(tech.ID), "squad", backendID, testutil.Cols{"role": "server work"})
	dbfx.SquadMember(t, uuidToString(tech.ID), "squad", retiredID)
	dbfx.Exec(t, `UPDATE squad SET archived_at = now() WHERE id = $1`, retiredID)

	out := buildSquadLeaderBriefing(context.Background(), testHandler.Queries, tech, true)
	want := formatRosterRow("Roster Backend", "squad", "server work", "", "[@Roster Backend](mention://squad/"+backendID+")")
	if !strings.Contains(out, want) {
		t.Fatalf("roster lacks sub-squad row %q:\n%s", want, out)
	}
	if strings.Contains(out, retiredID) {
		t.Fatalf("archived sub-squad listed in roster:\n%s", out)
	}
}
