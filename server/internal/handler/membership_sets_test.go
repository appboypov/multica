package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func assertOrganisationSet(t *testing.T, got, want []string) {
	t.Helper()
	expected := slices.Clone(want)
	slices.Sort(expected)
	expected = slices.Compact(expected)
	if got == nil || !slices.Equal(got, expected) {
		t.Fatalf("organisation set = %v, want non-null ordered %v", got, expected)
	}
}

func TestClientOrganisationSets_InviteAcceptAndReplace(t *testing.T) {
	h := *testHandler
	h.EmailService = nil
	h.SeatCapacity = nil
	h.Bus = events.New()
	a := clientTestOrganisation(t, testWorkspaceID)
	b := clientTestOrganisation(t, testWorkspaceID)
	c := clientTestOrganisation(t, testWorkspaceID)
	email := "organisation-sets-" + uuid.NewString() + "@example.invalid"
	userID := dbfx.User(t, "Organisation set contact", email)
	var acceptedEvent, updatedEvent MemberWithUserResponse
	h.Bus.Subscribe(protocol.EventInvitationAccepted, func(e events.Event) {
		acceptedEvent = e.Payload.(map[string]any)["member"].(MemberWithUserResponse)
	})
	h.Bus.Subscribe(protocol.EventMemberUpdated, func(e events.Event) {
		updatedEvent = e.Payload.(map[string]any)["member"].(MemberWithUserResponse)
	})
	inviteReq := withURLParam(newRequest("POST", "/api/workspaces/"+testWorkspaceID+"/members", CreateMemberRequest{
		Email: email, Role: "client", OrganisationIDs: []string{b, a, b},
	}), "id", testWorkspaceID)
	var invitation InvitationResponse
	testutil.Call(t, h.CreateInvitation, inviteReq).Want(http.StatusCreated).JSON(&invitation)
	dbfx.Cleanup(t, `DELETE FROM workspace_invitation WHERE id=$1`, invitation.ID)
	assertOrganisationSet(t, invitation.OrganisationIDs, []string{a, b})
	var stored []string
	dbfx.QueryRow(t, `SELECT organisation_ids::text[] FROM workspace_invitation WHERE id=$1`, invitation.ID).Scan(&stored)
	assertOrganisationSet(t, stored, []string{a, b})

	var invitations []InvitationResponse
	testutil.Call(t, h.ListWorkspaceInvitations, withURLParam(newRequest("GET", "/api/workspaces/"+testWorkspaceID+"/invitations", nil), "id", testWorkspaceID)).Want(http.StatusOK).JSON(&invitations)
	found := false
	for _, row := range invitations {
		if row.ID == invitation.ID {
			assertOrganisationSet(t, row.OrganisationIDs, []string{a, b})
			found = true
		}
	}
	if !found {
		t.Fatal("created invitation missing from workspace list")
	}

	acceptReq := withURLParam(newRequest("POST", "/api/invitations/"+invitation.ID+"/accept", nil), "id", invitation.ID)
	acceptReq.Header.Set("X-User-ID", userID)
	var member MemberWithUserResponse
	testutil.Call(t, h.AcceptInvitation, acceptReq).Want(http.StatusOK).JSON(&member)
	dbfx.Cleanup(t, `DELETE FROM member WHERE id=$1`, member.ID)
	assertOrganisationSet(t, member.OrganisationIDs, []string{a, b})
	assertOrganisationSet(t, acceptedEvent.OrganisationIDs, []string{a, b})
	if member.Role != "client" || member.UserID != userID || dbfx.Count(t, `SELECT count(*) FROM member WHERE user_id=$1`, userID) != 1 {
		t.Fatalf("acceptance changed membership identity: %+v", member)
	}
	var onboarded bool
	dbfx.QueryRow(t, `SELECT onboarded_at IS NOT NULL FROM "user" WHERE id=$1`, userID).Scan(&onboarded)
	if !onboarded {
		t.Fatal("acceptance did not complete onboarding")
	}

	for _, set := range [][]string{{c, a, b, c}, {c, b}, {c}} {
		patch := withURLParams(newRequest("PATCH", "/api/workspaces/"+testWorkspaceID+"/members/"+member.ID, UpdateMemberRequest{
			Role: "client", OrganisationIDs: set,
		}), "id", testWorkspaceID, "memberId", member.ID)
		testutil.Call(t, h.UpdateMember, patch).Want(http.StatusOK).JSON(&member)
		assertOrganisationSet(t, member.OrganisationIDs, set)
		assertOrganisationSet(t, updatedEvent.OrganisationIDs, set)
		dbfx.QueryRow(t, `SELECT organisation_ids::text[] FROM member WHERE id=$1`, member.ID).Scan(&stored)
		assertOrganisationSet(t, stored, set)
	}
	patch := withURLParams(newRequest("PATCH", "/api/workspaces/"+testWorkspaceID+"/members/"+member.ID, UpdateMemberRequest{
		Role: "client", OrganisationIDs: []string{},
	}), "id", testWorkspaceID, "memberId", member.ID)
	testutil.Call(t, h.UpdateMember, patch).Want(http.StatusBadRequest)
	dbfx.QueryRow(t, `SELECT organisation_ids::text[] FROM member WHERE id=$1`, member.ID).Scan(&stored)
	assertOrganisationSet(t, stored, []string{c})

	// Restore B/C to exercise partial and total deletion on every member view.
	dbfx.Exec(t, `UPDATE member SET organisation_ids=$2::uuid[] WHERE id=$1`, member.ID, []string{b, c})
	for _, remaining := range [][]string{{c}, {}} {
		deleted := b
		if len(remaining) == 0 {
			deleted = c
		}
		dbfx.Exec(t, `DELETE FROM organisation WHERE id=$1`, deleted)
		var current struct {
			Memberships []MyMembershipResponse `json:"memberships"`
		}
		currentReq := newRequest("GET", "/api/me/memberships", nil)
		currentReq.Header.Set("X-User-ID", userID)
		testutil.Call(t, h.ListMyMemberships, currentReq).Want(http.StatusOK).JSON(&current)
		if len(current.Memberships) != 1 || current.Memberships[0].Role != "client" {
			t.Fatalf("deletion changed membership role/count: %+v", current)
		}
		ids := make([]string, len(current.Memberships[0].Organisations))
		for i, organisation := range current.Memberships[0].Organisations {
			ids[i] = organisation.ID
			if organisation.Name == "" {
				t.Fatal("current membership omits organisation name")
			}
		}
		assertOrganisationSet(t, ids, remaining)
		var members []MemberResponse
		testutil.Call(t, h.ListMembers, withURLParam(newRequest("GET", "/api/workspaces/"+testWorkspaceID+"/members", nil), "id", testWorkspaceID)).Want(http.StatusOK).JSON(&members)
		found = false
		for _, row := range members {
			if row.ID == member.ID {
				assertOrganisationSet(t, row.OrganisationIDs, remaining)
				found = true
			}
		}
		if !found {
			t.Fatal("member missing from basic member list")
		}
		var enriched []MemberWithUserResponse
		testutil.Call(t, h.ListMembersWithUser, withURLParam(newRequest("GET", "/api/workspaces/"+testWorkspaceID+"/members", nil), "id", testWorkspaceID)).Want(http.StatusOK).JSON(&enriched)
		found = false
		for _, row := range enriched {
			if row.ID == member.ID {
				assertOrganisationSet(t, row.OrganisationIDs, remaining)
				found = true
			}
		}
		if !found {
			t.Fatal("member missing from enriched member list")
		}
		var retried MemberWithUserResponse
		testutil.Call(t, h.AcceptInvitation, acceptReq).Want(http.StatusOK).JSON(&retried)
		assertOrganisationSet(t, retried.OrganisationIDs, remaining)
		if retried.Role != "client" || retried.ID != member.ID {
			t.Fatalf("retry upgraded/replaced membership: %+v", retried)
		}
	}
}

func TestClientOrganisationSets_InvitationSurvivors(t *testing.T) {
	for _, allDeleted := range []bool{false, true} {
		name := "partial deletion"
		if allDeleted {
			name = "all deleted"
		}
		t.Run(name, func(t *testing.T) {
			h := *testHandler
			h.EmailService = nil
			h.SeatCapacity = nil
			a := clientTestOrganisation(t, testWorkspaceID)
			b := clientTestOrganisation(t, testWorkspaceID)
			email := "organisation-survivors-" + uuid.NewString() + "@example.invalid"
			userID := dbfx.User(t, "Survivor contact", email)
			var invitation InvitationResponse
			req := withURLParam(newRequest("POST", "/api/workspaces/"+testWorkspaceID+"/members", CreateMemberRequest{
				Email: email, Role: "client", OrganisationIDs: []string{a, b},
			}), "id", testWorkspaceID)
			testutil.Call(t, h.CreateInvitation, req).Want(http.StatusCreated).JSON(&invitation)
			dbfx.Cleanup(t, `DELETE FROM workspace_invitation WHERE id=$1`, invitation.ID)
			// Soft references may outlive their organisation or have been written
			// by older imports; neither foreign nor missing IDs may survive.
			otherWorkspace := dbfx.Workspace(t, "Foreign invitation binding", "foreign-invitation-"+uuid.NewString())
			foreign := clientTestOrganisation(t, otherWorkspace)
			dbfx.Exec(t, `UPDATE workspace_invitation SET organisation_ids=$2::uuid[] WHERE id=$1`, invitation.ID, []string{a, b, foreign, uuid.NewString(), b})
			dbfx.Exec(t, `DELETE FROM organisation WHERE id=$1`, a)
			remaining := []string{b}
			if allDeleted {
				dbfx.Exec(t, `DELETE FROM organisation WHERE id=$1`, b)
				remaining = []string{}
			}
			getReq := withURLParam(newRequest("GET", "/api/invitations/"+invitation.ID, nil), "id", invitation.ID)
			getReq.Header.Set("X-User-ID", userID)
			var projected InvitationResponse
			testutil.Call(t, h.GetMyInvitation, getReq).Want(http.StatusOK).JSON(&projected)
			assertOrganisationSet(t, projected.OrganisationIDs, remaining)
			listReq := newRequest("GET", "/api/invitations", nil)
			listReq.Header.Set("X-User-ID", userID)
			var pending []InvitationResponse
			testutil.Call(t, h.ListMyInvitations, listReq).Want(http.StatusOK).JSON(&pending)
			if len(pending) != 1 || pending[0].ID != invitation.ID {
				t.Fatalf("pending invitation not discoverable: %+v", pending)
			}
			assertOrganisationSet(t, pending[0].OrganisationIDs, remaining)
			var workspacePending []InvitationResponse
			testutil.Call(t, h.ListWorkspaceInvitations, withURLParam(newRequest("GET", "/api/workspaces/"+testWorkspaceID+"/invitations", nil), "id", testWorkspaceID)).Want(http.StatusOK).JSON(&workspacePending)
			found := false
			for _, row := range workspacePending {
				if row.ID == invitation.ID {
					assertOrganisationSet(t, row.OrganisationIDs, remaining)
					found = true
				}
			}
			if !found {
				t.Fatal("pending invitation missing from workspace list")
			}
			acceptReq := withURLParam(newRequest("POST", "/api/invitations/"+invitation.ID+"/accept", nil), "id", invitation.ID)
			acceptReq.Header.Set("X-User-ID", userID)
			if allDeleted {
				capacity := &stubSeatCapacity{}
				h.SeatCapacity = capacity
				testutil.Call(t, h.AcceptInvitation, acceptReq).Want(http.StatusConflict)
				var status string
				dbfx.QueryRow(t, `SELECT status FROM workspace_invitation WHERE id=$1`, invitation.ID).Scan(&status)
				var onboarded bool
				dbfx.QueryRow(t, `SELECT onboarded_at IS NOT NULL FROM "user" WHERE id=$1`, userID).Scan(&onboarded)
				if status != "pending" || onboarded || capacity.consumeCalls != 0 || dbfx.Count(t, `SELECT count(*) FROM member WHERE user_id=$1`, userID) != 0 {
					t.Fatal("all-deleted acceptance mutated invitation, onboarding, capacity or membership")
				}
				return
			}
			var member MemberWithUserResponse
			testutil.Call(t, h.AcceptInvitation, acceptReq).Want(http.StatusOK).JSON(&member)
			dbfx.Cleanup(t, `DELETE FROM member WHERE id=$1`, member.ID)
			assertOrganisationSet(t, member.OrganisationIDs, remaining)
			var persisted []string
			dbfx.QueryRow(t, `SELECT organisation_ids::text[] FROM member WHERE id=$1`, member.ID).Scan(&persisted)
			assertOrganisationSet(t, persisted, remaining)
		})
	}
}

func TestClientOrganisationSets_RejectCompleteInvalidWrite(t *testing.T) {
	a := clientTestOrganisation(t, testWorkspaceID)
	otherWorkspace := dbfx.Workspace(t, "Foreign organisation", "foreign-set-"+uuid.NewString())
	foreign := clientTestOrganisation(t, otherWorkspace)
	for _, tc := range []struct {
		name string
		role string
		ids  any
		omit bool
	}{
		{name: "omitted", role: "client", omit: true},
		{name: "null", role: "client"},
		{name: "empty", role: "client", ids: []string{}},
		{name: "malformed after valid", role: "client", ids: []string{a, "not-a-uuid"}},
		{name: "foreign after valid", role: "client", ids: []string{a, foreign}},
		{name: "staff nonempty", role: "member", ids: []string{a}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			email := "invalid-set-" + uuid.NewString() + "@example.invalid"
			body := map[string]any{"email": email, "role": tc.role}
			if !tc.omit {
				body["organisation_ids"] = tc.ids
			}
			h := *testHandler
			capacity := &stubSeatCapacity{}
			h.SeatCapacity = capacity
			req := withURLParam(newRequest("POST", "/api/workspaces/"+testWorkspaceID+"/members", body), "id", testWorkspaceID)
			testutil.Call(t, h.CreateInvitation, req).Want(http.StatusBadRequest)
			if capacity.reserveCalls != 0 || dbfx.Count(t, `SELECT count(*) FROM workspace_invitation WHERE invitee_email=$1`, email) != 0 {
				t.Fatal("invalid set reserved capacity or persisted a partial invitation")
			}
			_, memberID := clientTestMember(t, testWorkspaceID, "client", a)
			delete(body, "email")
			patch := withURLParams(newRequest("PATCH", "/api/workspaces/"+testWorkspaceID+"/members/"+memberID, body), "id", testWorkspaceID, "memberId", memberID)
			testutil.Call(t, h.UpdateMember, patch).Want(http.StatusBadRequest)
			var role string
			var ids []string
			dbfx.QueryRow(t, `SELECT role, organisation_ids::text[] FROM member WHERE id=$1`, memberID).Scan(&role, &ids)
			if role != "client" {
				t.Fatalf("invalid set changed role: %s", role)
			}
			assertOrganisationSet(t, ids, []string{a})
		})
	}
}

func TestClientOrganisationSets_ResponsesFilterForeignSoftReferences(t *testing.T) {
	a := clientTestOrganisation(t, testWorkspaceID)
	otherWorkspace := dbfx.Workspace(t, "Foreign projection", "foreign-projection-"+uuid.NewString())
	foreign := clientTestOrganisation(t, otherWorkspace)
	userID, memberID := clientTestMember(t, testWorkspaceID, "client", a)
	dbfx.Exec(t, `UPDATE member SET organisation_ids=$2::uuid[] WHERE id=$1`, memberID, []string{foreign, a, uuid.NewString(), a})
	projected, err := testHandler.Queries.GetMemberForResponse(context.Background(), parseUUID(memberID))
	if err != nil {
		t.Fatal(err)
	}
	assertOrganisationSet(t, organisationIDsResponse(projected.OrganisationIds), []string{a})
	var current struct {
		Memberships []MyMembershipResponse `json:"memberships"`
	}
	req := newRequest("GET", "/api/me/memberships", nil)
	req.Header.Set("X-User-ID", userID)
	testutil.Call(t, testHandler.ListMyMemberships, req).Want(http.StatusOK).JSON(&current)
	if len(current.Memberships) != 1 || len(current.Memberships[0].Organisations) != 1 || current.Memberships[0].Organisations[0].ID != a {
		t.Fatalf("foreign or stale binding exposed: %+v", current)
	}
	// Inspect actual JSON so removed aliases cannot accidentally remain public.
	raw, err := json.Marshal(memberToResponse(db.Member(projected)))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if _, exists := fields["organisation_id"]; exists {
		t.Fatal("singular member alias remains public")
	}
}

func TestClientOrganisationSets_NilStaffQueryParametersPersistEmptyArrays(t *testing.T) {
	ctx := context.Background()
	userID := dbfx.User(t, "Nil staff binding", "nil-staff-"+uuid.NewString()+"@example.invalid")
	member, err := testHandler.Queries.CreateMember(ctx, db.CreateMemberParams{
		WorkspaceID: parseUUID(testWorkspaceID), UserID: parseUUID(userID), Role: "member",
	})
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, `DELETE FROM member WHERE id=$1`, uuidToString(member.ID))
	member, err = testHandler.Queries.UpdateMemberRole(ctx, db.UpdateMemberRoleParams{ID: member.ID, Role: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	assertOrganisationSet(t, organisationIDsResponse(member.OrganisationIds), []string{})
	invitation, err := testHandler.Queries.CreateInvitation(ctx, db.CreateInvitationParams{
		ID: parseUUID(uuid.NewString()), WorkspaceID: parseUUID(testWorkspaceID), InviterID: parseUUID(testUserID),
		InviteeEmail: "nil-staff-invite-" + uuid.NewString() + "@example.invalid", Role: "member",
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, `DELETE FROM workspace_invitation WHERE id=$1`, uuidToString(invitation.ID))
	var memberEmpty, invitationEmpty bool
	dbfx.QueryRow(t, `SELECT organisation_ids = '{}'::uuid[] FROM member WHERE id=$1`, uuidToString(member.ID)).Scan(&memberEmpty)
	dbfx.QueryRow(t, `SELECT organisation_ids = '{}'::uuid[] FROM workspace_invitation WHERE id=$1`, uuidToString(invitation.ID)).Scan(&invitationEmpty)
	if !memberEmpty || !invitationEmpty {
		t.Fatal("nil generated parameters did not persist empty staff arrays")
	}
}
