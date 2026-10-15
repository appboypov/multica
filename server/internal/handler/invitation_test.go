package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/seatcapacity"
	"github.com/multica-ai/multica/server/internal/testutil"
)

const invitationTestEmail = "invitation-test@multica.ai"

type stubSeatCapacity struct {
	reserveDecision seatcapacity.Decision
	reserveErr      error
	reserveCalls    int
	releaseCalls    int
	consumeCalls    int
	consumeDecision *seatcapacity.Decision
	consumeErr      error
}

func (*stubSeatCapacity) RecoveryAvailable() bool { return true }

func (s *stubSeatCapacity) ReserveInvitation(context.Context, uuid.UUID, uuid.UUID, time.Time) (seatcapacity.Decision, error) {
	s.reserveCalls++
	return s.reserveDecision, s.reserveErr
}
func (s *stubSeatCapacity) ClaimShareJoin(context.Context, uuid.UUID, uuid.UUID) (seatcapacity.Decision, error) {
	return seatcapacity.Decision{Managed: true, Allowed: true}, nil
}
func (s *stubSeatCapacity) Consume(context.Context, uuid.UUID, uuid.UUID) (seatcapacity.Decision, error) {
	s.consumeCalls++
	decision := seatcapacity.Decision{Managed: true, Allowed: true}
	if s.consumeDecision != nil {
		decision = *s.consumeDecision
	}
	return decision, s.consumeErr
}
func (s *stubSeatCapacity) Confirm(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (seatcapacity.Decision, error) {
	return seatcapacity.Decision{Managed: true, Allowed: true}, nil
}
func (s *stubSeatCapacity) Release(context.Context, uuid.UUID, uuid.UUID) (seatcapacity.Decision, error) {
	s.releaseCalls++
	return seatcapacity.Decision{Managed: true, Allowed: true}, nil
}
func (s *stubSeatCapacity) ReleaseMember(context.Context, uuid.UUID, uuid.UUID) (seatcapacity.Decision, error) {
	return seatcapacity.Decision{Managed: true, Allowed: true}, nil
}
func (s *stubSeatCapacity) GetOperation(context.Context, uuid.UUID, uuid.UUID) (seatcapacity.Decision, error) {
	return seatcapacity.Decision{}, nil
}

func useSeatCapacity(t *testing.T, executor seatcapacity.Executor) {
	t.Helper()
	previous := testHandler.SeatCapacity
	testHandler.SeatCapacity = executor
	t.Cleanup(func() {
		testHandler.SeatCapacity = previous
	})
}

type stubInvitationRateLimiter struct {
	allowed     bool
	allowResult *bool
	err         error
	allowErr    error
	retryAfter  time.Duration
	checkKeys   []string
	allowKeys   []string
}

func (l *stubInvitationRateLimiter) Allow(ctx context.Context, key string) bool {
	allowed, _ := l.AllowWithError(ctx, key)
	return allowed
}

func (l *stubInvitationRateLimiter) AllowWithError(_ context.Context, key string) (bool, error) {
	l.allowKeys = append(l.allowKeys, key)
	if l.allowResult != nil {
		return *l.allowResult, l.allowErr
	}
	return l.allowed, l.allowErr
}

func (l *stubInvitationRateLimiter) Check(ctx context.Context, key string) bool {
	allowed, _ := l.CheckWithError(ctx, key)
	return allowed
}

func (l *stubInvitationRateLimiter) CheckWithError(_ context.Context, key string) (bool, error) {
	l.checkKeys = append(l.checkKeys, key)
	return l.allowed, l.err
}

func (l *stubInvitationRateLimiter) RetryAfter(_ context.Context, _ string) time.Duration {
	return l.retryAfter
}

func useInvitationRateLimiters(t *testing.T, limiters InvitationRateLimiters) {
	t.Helper()
	previous := testHandler.InvitationRateLimiters
	testHandler.InvitationRateLimiters = limiters
	t.Cleanup(func() {
		testHandler.InvitationRateLimiters = previous
	})
}

func clearInvitationsForTestWorkspace(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	if _, err := testPool.Exec(ctx,
		`DELETE FROM workspace_invitation WHERE workspace_id = $1`,
		parseUUID(testWorkspaceID),
	); err != nil {
		t.Fatalf("clear invitations: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(),
			`DELETE FROM workspace_invitation WHERE workspace_id = $1`,
			parseUUID(testWorkspaceID),
		)
	})
}

type rollbackOnCommitTx struct {
	pgx.Tx
}

func (tx rollbackOnCommitTx) Commit(ctx context.Context) error {
	_ = tx.Tx.Rollback(ctx)
	return errors.New("forced commit failure")
}

type rollbackOnCommitTxStarter struct {
	pool *pgxpool.Pool
}

func (s rollbackOnCommitTxStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return rollbackOnCommitTx{Tx: tx}, nil
}

// Sanity check: a fresh, live pending invitation must block re-invitation.
func TestCreateInvitation_BlocksWhilePending(t *testing.T) {
	clearInvitationsForTestWorkspace(t)
	actor := &stubInvitationRateLimiter{allowed: true}
	workspace := &stubInvitationRateLimiter{allowed: true}
	recipient := &stubInvitationRateLimiter{allowed: true}
	useInvitationRateLimiters(t, InvitationRateLimiters{Actor: actor, Workspace: workspace, Recipient: recipient})

	req := newRequest("POST", "/api/workspaces/"+testWorkspaceID+"/members", CreateMemberRequest{
		Email: invitationTestEmail,
		Role:  "member",
	})
	req = withURLParam(req, "id", testWorkspaceID)
	w := httptest.NewRecorder()
	testHandler.CreateInvitation(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("first invite: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	req2 := newRequest("POST", "/api/workspaces/"+testWorkspaceID+"/members", CreateMemberRequest{
		Email: invitationTestEmail,
		Role:  "member",
	})
	req2 = withURLParam(req2, "id", testWorkspaceID)
	w2 := httptest.NewRecorder()
	testHandler.CreateInvitation(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Fatalf("second invite: expected 409 while still pending, got %d: %s", w2.Code, w2.Body.String())
	}
	for name, calls := range map[string]int{
		"actor checks": len(actor.checkKeys), "workspace checks": len(workspace.checkKeys), "recipient checks": len(recipient.checkKeys),
		"actor allows": len(actor.allowKeys), "workspace allows": len(workspace.allowKeys), "recipient allows": len(recipient.allowKeys),
	} {
		if calls != 1 {
			t.Errorf("%s limiter calls = %d, want 1; a pending retry must not consume budget", name, calls)
		}
	}
}

func TestCreateInvitation_BlocksWhenPurchasedCapacityIsFull(t *testing.T) {
	clearInvitationsForTestWorkspace(t)
	if _, err := testPool.Exec(context.Background(), `DELETE FROM seat_capacity_outbox WHERE workspace_id = $1`, parseUUID(testWorkspaceID)); err != nil {
		t.Fatalf("clear capacity outbox: %v", err)
	}
	capacity := &stubSeatCapacity{reserveDecision: seatcapacity.Decision{
		Managed: true, Allowed: false, Reason: "capacity_full",
	}}
	useSeatCapacity(t, capacity)
	actor := &stubInvitationRateLimiter{allowed: true}
	workspace := &stubInvitationRateLimiter{allowed: true}
	recipient := &stubInvitationRateLimiter{allowed: true}
	useInvitationRateLimiters(t, InvitationRateLimiters{Actor: actor, Workspace: workspace, Recipient: recipient})

	req := newRequest(http.MethodPost, "/api/workspaces/"+testWorkspaceID+"/members", CreateMemberRequest{
		Email: "capacity-full-invite@multica.ai", Role: "member",
	})
	req = withURLParam(req, "id", testWorkspaceID)
	rec := httptest.NewRecorder()
	testHandler.CreateInvitation(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "seat_capacity_full" {
		t.Fatalf("code = %q, want seat_capacity_full", body.Code)
	}
	if capacity.reserveCalls != 1 {
		t.Fatalf("reserve calls = %d, want 1", capacity.reserveCalls)
	}
	for name, calls := range map[string]int{
		"actor checks": len(actor.checkKeys), "workspace checks": len(workspace.checkKeys), "recipient checks": len(recipient.checkKeys),
	} {
		if calls != 1 {
			t.Errorf("%s = %d, want 1", name, calls)
		}
	}
	if calls := len(actor.allowKeys); calls != 1 {
		t.Errorf("actor allows = %d, want 1 when persistent capacity_full is rejected", calls)
	}
	for name, calls := range map[string]int{
		"workspace allows": len(workspace.allowKeys), "recipient allows": len(recipient.allowKeys),
	} {
		if calls != 0 {
			t.Errorf("%s = %d, want 0 so a later purchased-seat invitation keeps shared budget", name, calls)
		}
	}
	var invitationCount, outboxCount int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM workspace_invitation WHERE workspace_id = $1 AND invitee_email = $2`, parseUUID(testWorkspaceID), "capacity-full-invite@multica.ai").Scan(&invitationCount); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM seat_capacity_outbox WHERE workspace_id = $1`, parseUUID(testWorkspaceID)).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if invitationCount != 0 || outboxCount != 0 {
		t.Fatalf("persisted invitation=%d outbox=%d, want both zero", invitationCount, outboxCount)
	}
}

func TestCreateInvitation_BlocksOvercommittedCapacityWithoutOfferingSingleSeatSemantics(t *testing.T) {
	clearInvitationsForTestWorkspace(t)
	capacity := &stubSeatCapacity{reserveErr: &seatcapacity.HTTPError{
		StatusCode: http.StatusConflict,
		Code:       "capacity_overcommitted",
	}}
	useSeatCapacity(t, capacity)
	actor := &stubInvitationRateLimiter{allowed: true}
	workspace := &stubInvitationRateLimiter{allowed: true}
	recipient := &stubInvitationRateLimiter{allowed: true}
	useInvitationRateLimiters(t, InvitationRateLimiters{Actor: actor, Workspace: workspace, Recipient: recipient})

	req := newRequest(http.MethodPost, "/api/workspaces/"+testWorkspaceID+"/members", CreateMemberRequest{
		Email: "capacity-overcommitted-invite@multica.ai", Role: "member",
	})
	req = withURLParam(req, "id", testWorkspaceID)
	rec := httptest.NewRecorder()
	testHandler.CreateInvitation(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "seat_capacity_overcommitted" {
		t.Fatalf("code = %q, want seat_capacity_overcommitted", body.Code)
	}
	if capacity.reserveCalls != 1 {
		t.Fatalf("reserve calls = %d, want 1", capacity.reserveCalls)
	}
	if calls := len(actor.allowKeys); calls != 1 {
		t.Errorf("actor allows = %d, want 1 when persistent capacity_overcommitted is rejected", calls)
	}
	for name, calls := range map[string]int{
		"workspace allows": len(workspace.allowKeys), "recipient allows": len(recipient.allowKeys),
	} {
		if calls != 0 {
			t.Errorf("%s = %d, want 0 while capacity remains overcommitted", name, calls)
		}
	}
}

func TestCreateInvitation_MapsCloudCapacityRateLimitWithoutConsumingInvitationBudget(t *testing.T) {
	clearInvitationsForTestWorkspace(t)
	if _, err := testPool.Exec(context.Background(), `DELETE FROM seat_capacity_outbox WHERE workspace_id = $1`, parseUUID(testWorkspaceID)); err != nil {
		t.Fatalf("clear capacity outbox: %v", err)
	}
	capacity := &stubSeatCapacity{reserveErr: &seatcapacity.HTTPError{
		StatusCode: http.StatusTooManyRequests,
		RetryAfter: 3 * time.Second,
	}}
	useSeatCapacity(t, capacity)
	actor := &stubInvitationRateLimiter{allowed: true}
	workspace := &stubInvitationRateLimiter{allowed: true}
	recipient := &stubInvitationRateLimiter{allowed: true}
	useInvitationRateLimiters(t, InvitationRateLimiters{Actor: actor, Workspace: workspace, Recipient: recipient})

	req := newRequest(http.MethodPost, "/api/workspaces/"+testWorkspaceID+"/members", CreateMemberRequest{
		Email: "capacity-rate-limited@multica.ai", Role: "member",
	})
	req = withURLParam(req, "id", testWorkspaceID)
	rec := httptest.NewRecorder()
	testHandler.CreateInvitation(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Retry-After") != "3" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected retry/cache headers: %v", rec.Header())
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "seat_capacity_rate_limited" {
		t.Fatalf("code = %q, want seat_capacity_rate_limited", body.Code)
	}
	if capacity.reserveCalls != 1 || capacity.releaseCalls != 0 {
		t.Fatalf("capacity calls reserve=%d release=%d, want 1/0", capacity.reserveCalls, capacity.releaseCalls)
	}
	for name, calls := range map[string]int{
		"actor allows": len(actor.allowKeys), "workspace allows": len(workspace.allowKeys), "recipient allows": len(recipient.allowKeys),
	} {
		if calls != 0 {
			t.Errorf("%s = %d, want 0 after Cloud rate limit", name, calls)
		}
	}
	if intents := dbfx.Count(t, `SELECT count(*) FROM seat_capacity_outbox WHERE workspace_id = $1`, parseUUID(testWorkspaceID)); intents != 0 {
		t.Fatalf("capacity intents = %d, want 0 after Cloud rate limit", intents)
	}
}

func TestCreateInvitation_CompensatesCapacityWhenCommitRollsBack(t *testing.T) {
	clearInvitationsForTestWorkspace(t)
	ctx := context.Background()
	workspaceID := parseUUID(testWorkspaceID)
	if _, err := testPool.Exec(ctx, `DELETE FROM seat_capacity_outbox WHERE workspace_id = $1`, workspaceID); err != nil {
		t.Fatalf("clear capacity outbox: %v", err)
	}

	capacity := &stubSeatCapacity{reserveDecision: seatcapacity.Decision{Managed: true, Allowed: true}}
	useSeatCapacity(t, capacity)
	previousTxStarter := testHandler.TxStarter
	testHandler.TxStarter = rollbackOnCommitTxStarter{pool: testPool}
	t.Cleanup(func() { testHandler.TxStarter = previousTxStarter })

	const email = "capacity-commit-failure@multica.ai"
	req := newRequest(http.MethodPost, "/api/workspaces/"+testWorkspaceID+"/members", CreateMemberRequest{
		Email: email, Role: "member",
	})
	req = withURLParam(req, "id", testWorkspaceID)
	rec := httptest.NewRecorder()
	testHandler.CreateInvitation(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body.String())
	}
	if capacity.reserveCalls != 1 || capacity.releaseCalls != 1 {
		t.Fatalf("reserve calls = %d, release calls = %d; want one each", capacity.reserveCalls, capacity.releaseCalls)
	}

	var invitationCount, outboxCount int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM workspace_invitation WHERE workspace_id = $1 AND invitee_email = $2`, workspaceID, email).Scan(&invitationCount); err != nil {
		t.Fatal(err)
	}
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM seat_capacity_outbox WHERE workspace_id = $1`, workspaceID).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if invitationCount != 0 || outboxCount != 0 {
		t.Fatalf("persisted invitation=%d outbox=%d, want both zero after compensation", invitationCount, outboxCount)
	}
}

// Regression for issue #2055: an expired pending invitation must NOT block a
// new invitation to the same email. The stale row should be flipped to
// 'expired' and a fresh pending row should be created.
func TestCreateInvitation_AllowsAfterExpiry(t *testing.T) {
	clearInvitationsForTestWorkspace(t)
	ctx := context.Background()

	var staleID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workspace_invitation (
			workspace_id, inviter_id, invitee_email, role, status, created_at, updated_at, expires_at
		)
		VALUES ($1, $2, $3, 'member', 'pending', now() - interval '10 days', now() - interval '10 days', now() - interval '3 days')
		RETURNING id
	`, parseUUID(testWorkspaceID), parseUUID(testUserID), invitationTestEmail).Scan(&staleID); err != nil {
		t.Fatalf("seed expired invitation: %v", err)
	}

	req := newRequest("POST", "/api/workspaces/"+testWorkspaceID+"/members", CreateMemberRequest{
		Email: invitationTestEmail,
		Role:  "member",
	})
	req = withURLParam(req, "id", testWorkspaceID)
	w := httptest.NewRecorder()
	testHandler.CreateInvitation(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("re-invite after expiry: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp InvitationResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID == "" || resp.ID == staleID {
		t.Fatalf("expected a new invitation row, got id=%q (stale=%q)", resp.ID, staleID)
	}

	var staleStatus string
	if err := testPool.QueryRow(ctx,
		`SELECT status FROM workspace_invitation WHERE id = $1`, staleID,
	).Scan(&staleStatus); err != nil {
		t.Fatalf("read stale row: %v", err)
	}
	if staleStatus != "expired" {
		t.Fatalf("expected stale row to be 'expired', got %q", staleStatus)
	}

	var pendingCount int
	if err := testPool.QueryRow(ctx, `
		SELECT COUNT(*) FROM workspace_invitation
		WHERE workspace_id = $1 AND invitee_email = $2 AND status = 'pending'
	`, parseUUID(testWorkspaceID), invitationTestEmail).Scan(&pendingCount); err != nil {
		t.Fatalf("count pending: %v", err)
	}
	if pendingCount != 1 {
		t.Fatalf("expected exactly 1 pending invitation after re-invite, got %d", pendingCount)
	}
}

func TestCreateInvitation_RateLimitChecksEveryGateBeforeCapacityReservation(t *testing.T) {
	clearInvitationsForTestWorkspace(t)
	if _, err := testPool.Exec(context.Background(), `DELETE FROM seat_capacity_outbox WHERE workspace_id = $1`, parseUUID(testWorkspaceID)); err != nil {
		t.Fatalf("clear capacity outbox: %v", err)
	}
	capacity := &stubSeatCapacity{reserveDecision: seatcapacity.Decision{Managed: true, Allowed: true}}
	useSeatCapacity(t, capacity)
	actor := &stubInvitationRateLimiter{allowed: false, retryAfter: 10 * time.Minute}
	workspace := &stubInvitationRateLimiter{allowed: true}
	recipient := &stubInvitationRateLimiter{allowed: false, retryAfter: 24 * time.Hour}
	useInvitationRateLimiters(t, InvitationRateLimiters{Actor: actor, Workspace: workspace, Recipient: recipient})

	const rawEmail = "  Invitation-Limited@Multica.AI  "
	req := newRequest(http.MethodPost, "/api/workspaces/"+testWorkspaceID+"/members", CreateMemberRequest{
		Email: rawEmail,
		Role:  "member",
	})
	req = withURLParam(req, "id", testWorkspaceID)
	rec := httptest.NewRecorder()
	testHandler.CreateInvitation(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Retry-After"); got != "86400" {
		t.Errorf("Retry-After = %q, want 86400", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	var body struct {
		Error             string `json:"error"`
		Code              string `json:"code"`
		RetryAfterSeconds int64  `json:"retry_after_seconds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error != "too many invitation requests" || body.Code != "invitation_rate_limited" || body.RetryAfterSeconds != 86400 {
		t.Errorf("unexpected body: %+v", body)
	}
	if strings.Contains(rec.Body.String(), "actor") || strings.Contains(rec.Body.String(), "workspace") || strings.Contains(rec.Body.String(), "recipient") {
		t.Errorf("response leaked the limiting gate: %s", rec.Body.String())
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(rawEmail))
	checks := []struct {
		name string
		got  []string
		want string
	}{
		{name: "actor", got: actor.checkKeys, want: testUserID},
		{name: "workspace", got: workspace.checkKeys, want: testWorkspaceID},
		{name: "recipient", got: recipient.checkKeys, want: invitationRecipientKey(normalizedEmail)},
	}
	for _, check := range checks {
		if len(check.got) != 1 || check.got[0] != check.want {
			t.Errorf("%s limiter keys = %v, want [%s]", check.name, check.got, check.want)
		}
	}
	if len(recipient.checkKeys) == 1 && (recipient.checkKeys[0] == normalizedEmail || strings.Contains(recipient.checkKeys[0], "@")) {
		t.Errorf("recipient limiter key contains the email instead of a digest: %q", recipient.checkKeys[0])
	}
	for name, calls := range map[string]int{
		"actor": len(actor.allowKeys), "workspace": len(workspace.allowKeys), "recipient": len(recipient.allowKeys),
	} {
		if calls != 0 {
			t.Errorf("%s limiter consumed %d times, want 0 after a rejected check", name, calls)
		}
	}

	var pendingCount int
	if err := testPool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM workspace_invitation
		WHERE workspace_id = $1 AND invitee_email = $2 AND status = 'pending'
	`, parseUUID(testWorkspaceID), normalizedEmail).Scan(&pendingCount); err != nil {
		t.Fatalf("count pending invitations: %v", err)
	}
	if pendingCount != 0 {
		t.Fatalf("pending invitation count = %d, want 0 after rate limit rejection", pendingCount)
	}
	if capacity.reserveCalls != 0 || capacity.releaseCalls != 0 {
		t.Fatalf("capacity calls reserve=%d release=%d, want 0/0", capacity.reserveCalls, capacity.releaseCalls)
	}
	if intents := dbfx.Count(t, `SELECT count(*) FROM seat_capacity_outbox WHERE workspace_id = $1`, parseUUID(testWorkspaceID)); intents != 0 {
		t.Fatalf("capacity intents = %d, want 0 after compensated rate-limit rejection", intents)
	}
}

func TestCreateInvitation_RateLimiterFailureReturnsServiceUnavailable(t *testing.T) {
	clearInvitationsForTestWorkspace(t)
	actor := &stubInvitationRateLimiter{allowed: false, retryAfter: 10 * time.Minute}
	workspace := &stubInvitationRateLimiter{allowed: true, err: errors.New("redis unavailable")}
	recipient := &stubInvitationRateLimiter{allowed: true}
	useInvitationRateLimiters(t, InvitationRateLimiters{Actor: actor, Workspace: workspace, Recipient: recipient})
	previousMetrics := testHandler.Metrics
	testHandler.Metrics = obsmetrics.NewBusinessMetrics()
	t.Cleanup(func() {
		testHandler.Metrics = previousMetrics
	})

	req := newRequest(http.MethodPost, "/api/workspaces/"+testWorkspaceID+"/members", CreateMemberRequest{
		Email: "invitation-limiter-error@multica.ai",
		Role:  "member",
	})
	req = withURLParam(req, "id", testWorkspaceID)
	rec := httptest.NewRecorder()
	testHandler.CreateInvitation(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Retry-After") != "5" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("unexpected retry/cache headers: %v", rec.Header())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["code"] != "invitation_rate_limiter_unavailable" {
		t.Errorf("code = %q, want invitation_rate_limiter_unavailable", body["code"])
	}
	metricFamily := obsmetrics.GatherForTest(t, testHandler.Metrics)["multica_email_rate_limited_total"]
	for _, metric := range metricFamily.GetMetric() {
		if metric.GetCounter().GetValue() != 0 {
			t.Errorf("rate-limit metric = %v, want 0 when the final response is 503", metric.GetCounter().GetValue())
		}
	}
	for name, calls := range map[string]int{
		"actor checks": len(actor.checkKeys), "workspace checks": len(workspace.checkKeys), "recipient checks": len(recipient.checkKeys),
	} {
		if calls != 1 {
			t.Errorf("%s = %d, want 1 even when another gate errors", name, calls)
		}
	}
	for name, calls := range map[string]int{
		"actor": len(actor.allowKeys), "workspace": len(workspace.allowKeys), "recipient": len(recipient.allowKeys),
	} {
		if calls != 0 {
			t.Errorf("%s limiter consumed %d times, want 0 after a backend error", name, calls)
		}
	}
	var pendingCount int
	if err := testPool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM workspace_invitation
		WHERE workspace_id = $1 AND invitee_email = 'invitation-limiter-error@multica.ai' AND status = 'pending'
	`, parseUUID(testWorkspaceID)).Scan(&pendingCount); err != nil {
		t.Fatalf("count pending invitations: %v", err)
	}
	if pendingCount != 0 {
		t.Fatalf("pending invitation count = %d, want 0 after limiter backend failure", pendingCount)
	}
}

func TestInvitationAdmission_RejectedActorDoesNotConsumeWorkspaceBudget(t *testing.T) {
	limits := InvitationRateLimits{
		Actor:     SlidingWindowRateLimit{Limit: 1, Window: time.Hour},
		Workspace: SlidingWindowRateLimit{Limit: 2, Window: time.Hour},
		Recipient: SlidingWindowRateLimit{Limit: 10, Window: time.Hour},
	}
	h := *testHandler
	h.InvitationRateLimiters = NewMemoryInvitationRateLimiters(limits)

	admit := func(actorID, email string) bool {
		req := httptest.NewRequest(http.MethodPost, "/api/workspaces/workspace-a/members", nil)
		admission, ok := h.checkInvitationAdmission(httptest.NewRecorder(), req, actorID, "workspace-a", email)
		if ok {
			h.consumeInvitationAdmission(req, admission)
		}
		return ok
	}

	if !admit("actor-a", "first@multica.ai") {
		t.Fatal("first invitation was unexpectedly rejected")
	}
	for i := 0; i < 3; i++ {
		if admit("actor-a", fmt.Sprintf("rejected-%d@multica.ai", i)) {
			t.Fatalf("actor-limited invitation %d was unexpectedly admitted", i)
		}
	}
	if !admit("actor-b", "second@multica.ai") {
		t.Fatal("actor-limited retries consumed the shared workspace budget")
	}
}

func TestInvitationAdmission_RejectedActorDoesNotConsumeRecipientBudget(t *testing.T) {
	limits := InvitationRateLimits{
		Actor:     SlidingWindowRateLimit{Limit: 1, Window: time.Hour},
		Workspace: SlidingWindowRateLimit{Limit: 10, Window: time.Hour},
		Recipient: SlidingWindowRateLimit{Limit: 2, Window: time.Hour},
	}
	h := *testHandler
	h.InvitationRateLimiters = NewMemoryInvitationRateLimiters(limits)

	admit := func(actorID, workspaceID string) bool {
		req := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+workspaceID+"/members", nil)
		admission, ok := h.checkInvitationAdmission(httptest.NewRecorder(), req, actorID, workspaceID, "shared@multica.ai")
		if ok {
			h.consumeInvitationAdmission(req, admission)
		}
		return ok
	}

	if !admit("actor-a", "workspace-a") {
		t.Fatal("first invitation was unexpectedly rejected")
	}
	if admit("actor-a", "workspace-a") {
		t.Fatal("actor-limited retry was unexpectedly admitted")
	}
	if !admit("actor-b", "workspace-b") {
		t.Fatal("actor-limited retry consumed the global recipient budget")
	}
}

func TestInvitationAdmission_AllowsBoundedOvershootWhenGateFillsAfterCheck(t *testing.T) {
	allowDenied := false
	actor := &stubInvitationRateLimiter{allowed: true, allowResult: &allowDenied}
	workspace := &stubInvitationRateLimiter{allowed: true}
	recipient := &stubInvitationRateLimiter{allowed: true}
	h := *testHandler
	h.InvitationRateLimiters = InvitationRateLimiters{Actor: actor, Workspace: workspace, Recipient: recipient}

	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/workspace-a/members", nil)
	admission, ok := h.checkInvitationAdmission(httptest.NewRecorder(), req, "actor-a", "workspace-a", "recipient@multica.ai")
	if !ok {
		t.Fatal("invitation was rejected during the non-consuming check")
	}
	h.consumeInvitationAdmission(req, admission)
	for name, calls := range map[string]int{
		"actor checks": len(actor.checkKeys), "workspace checks": len(workspace.checkKeys), "recipient checks": len(recipient.checkKeys),
		"actor allows": len(actor.allowKeys), "workspace allows": len(workspace.allowKeys), "recipient allows": len(recipient.allowKeys),
	} {
		if calls != 1 {
			t.Errorf("%s = %d, want 1", name, calls)
		}
	}
}

func TestInvitationAdmission_AllowsBoundedOvershootWhenBackendFailsAfterChecks(t *testing.T) {
	actor := &stubInvitationRateLimiter{allowed: true}
	workspace := &stubInvitationRateLimiter{allowed: true, allowErr: errors.New("redis unavailable")}
	recipient := &stubInvitationRateLimiter{allowed: true}
	h := *testHandler
	h.InvitationRateLimiters = InvitationRateLimiters{Actor: actor, Workspace: workspace, Recipient: recipient}

	req := httptest.NewRequest(http.MethodPost, "/api/workspaces/workspace-a/members", nil)
	rec := httptest.NewRecorder()
	admission, ok := h.checkInvitationAdmission(rec, req, "actor-a", "workspace-a", "recipient@multica.ai")
	if !ok {
		t.Fatalf("invitation was rejected during successful checks with %d: %s", rec.Code, rec.Body.String())
	}
	h.consumeInvitationAdmission(req, admission)
	if rec.Body.Len() != 0 || rec.Header().Get("Retry-After") != "" {
		t.Fatalf("backend failure after checks wrote an error response: headers=%v body=%s", rec.Header(), rec.Body.String())
	}
	for name, calls := range map[string]int{
		"actor checks": len(actor.checkKeys), "workspace checks": len(workspace.checkKeys), "recipient checks": len(recipient.checkKeys),
		"actor allows": len(actor.allowKeys), "workspace allows": len(workspace.allowKeys), "recipient allows": len(recipient.allowKeys),
	} {
		if calls != 1 {
			t.Errorf("%s = %d, want 1", name, calls)
		}
	}
}

func TestCreateInvitation_RouteRequiresAdminRole(t *testing.T) {
	clearInvitationsForTestWorkspace(t)
	ctx := context.Background()
	const memberEmail = "invitation-route-member@multica.ai"
	_, _ = testPool.Exec(ctx, `DELETE FROM "user" WHERE email = $1`, memberEmail)
	var memberUserID string
	if err := testPool.QueryRow(ctx, `INSERT INTO "user" (name, email) VALUES ('Invitation Route Member', $1) RETURNING id`, memberEmail).Scan(&memberUserID); err != nil {
		t.Fatalf("create member user: %v", err)
	}
	if _, err := testPool.Exec(ctx, `INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'member')`, testWorkspaceID, memberUserID); err != nil {
		t.Fatalf("create member: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM member WHERE workspace_id = $1 AND user_id = $2`, testWorkspaceID, memberUserID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, memberUserID)
	})

	router := chi.NewRouter()
	router.Route("/api/workspaces/{id}", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireWorkspaceRoleFromURL(testHandler.Queries, "id", "owner", "admin"))
			r.Post("/members", testHandler.CreateInvitation)
		})
	})

	call := func(userID, inviteeEmail string) *httptest.ResponseRecorder {
		var body bytes.Buffer
		if err := json.NewEncoder(&body).Encode(CreateMemberRequest{Email: inviteeEmail, Role: "member"}); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+testWorkspaceID+"/members", &body)
		req.Header.Set("X-User-ID", userID)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	if rec := call(memberUserID, "invitation-route-rejected@multica.ai"); rec.Code != http.StatusForbidden {
		t.Errorf("member status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
	if rec := call(testUserID, fmt.Sprintf("invitation-route-owner-%s@multica.ai", testWorkspaceID)); rec.Code != http.StatusCreated {
		t.Errorf("owner status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
}

// An installed client can still show a pending-invitation row the user
// already accepted from another surface (web invite page, another device).
// Re-accept must be idempotent for the membership the first accept created,
// so the client's refetch drops the stale row instead of sticking forever.
func TestAcceptInvitation_AlreadyAcceptedIsIdempotentForExistingMember(t *testing.T) {
	email := "idempotent-accept-" + uuid.NewString() + "@multica.ai"
	userID := dbfx.User(t, "Idempotent Accept Invitee", email)
	invitationID := dbfx.Insert(t, "workspace_invitation", testutil.Cols{
		"workspace_id":    testWorkspaceID,
		"inviter_id":      testUserID,
		"invitee_email":   email,
		"invitee_user_id": userID,
		"role":            "member",
		"status":          "accepted",
		"expires_at":      testutil.Raw("now() + interval '1 day'"),
	})
	memberID := dbfx.Member(t, testWorkspaceID, userID, "member")

	req := newRequest("POST", "/api/invitations/"+invitationID+"/accept", nil)
	req.Header.Set("X-User-ID", userID)
	req = withURLParam(req, "id", invitationID)
	var member MemberWithUserResponse
	testutil.Call(t, testHandler.AcceptInvitation, req).Want(http.StatusOK).JSON(&member)
	if member.ID != memberID {
		t.Fatalf("member id = %q, want %q", member.ID, memberID)
	}
	if members := dbfx.Count(t, `SELECT count(*) FROM member WHERE workspace_id = $1 AND user_id = $2`, parseUUID(testWorkspaceID), parseUUID(userID)); members != 1 {
		t.Fatalf("member count = %d, want 1", members)
	}
}

// A concluded invitation whose membership is gone (the user left the
// workspace afterwards) must not be re-acceptable: leaving was explicit.
func TestAcceptInvitation_AlreadyAcceptedWithoutMembershipStillFails(t *testing.T) {
	email := "stale-accept-no-member-" + uuid.NewString() + "@multica.ai"
	userID := dbfx.User(t, "Stale Accept Invitee", email)
	invitationID := dbfx.Insert(t, "workspace_invitation", testutil.Cols{
		"workspace_id":    testWorkspaceID,
		"inviter_id":      testUserID,
		"invitee_email":   email,
		"invitee_user_id": userID,
		"role":            "member",
		"status":          "accepted",
		"expires_at":      testutil.Raw("now() + interval '1 day'"),
	})

	req := newRequest("POST", "/api/invitations/"+invitationID+"/accept", nil)
	req.Header.Set("X-User-ID", userID)
	req = withURLParam(req, "id", invitationID)
	testutil.Call(t, testHandler.AcceptInvitation, req).Want(http.StatusBadRequest)
	if members := dbfx.Count(t, `SELECT count(*) FROM member WHERE workspace_id = $1 AND user_id = $2`, parseUUID(testWorkspaceID), parseUUID(userID)); members != 0 {
		t.Fatalf("member count = %d, want 0", members)
	}
}

// Declining a concluded invitation is a no-op: the row only lingers in
// clients that concluded the same invitation elsewhere, and a 204 lets their
// refetch drop it.
func TestDeclineInvitation_AlreadyAcceptedIsNoOp(t *testing.T) {
	email := "idempotent-decline-" + uuid.NewString() + "@multica.ai"
	userID := dbfx.User(t, "Idempotent Decline Invitee", email)
	invitationID := dbfx.Insert(t, "workspace_invitation", testutil.Cols{
		"workspace_id":    testWorkspaceID,
		"inviter_id":      testUserID,
		"invitee_email":   email,
		"invitee_user_id": userID,
		"role":            "member",
		"status":          "accepted",
		"expires_at":      testutil.Raw("now() + interval '1 day'"),
	})
	dbfx.Member(t, testWorkspaceID, userID, "member")

	req := newRequest("POST", "/api/invitations/"+invitationID+"/decline", nil)
	req.Header.Set("X-User-ID", userID)
	req = withURLParam(req, "id", invitationID)
	testutil.Call(t, testHandler.DeclineInvitation, req).Want(http.StatusNoContent)

	var status string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM workspace_invitation WHERE id = $1`, parseUUID(invitationID)).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "accepted" {
		t.Fatalf("invitation status = %q, want accepted", status)
	}
	if members := dbfx.Count(t, `SELECT count(*) FROM member WHERE workspace_id = $1 AND user_id = $2`, parseUUID(testWorkspaceID), parseUUID(userID)); members != 1 {
		t.Fatalf("member count = %d, want 1", members)
	}
}

func TestCreateInvitation_ClientRules(t *testing.T) {
	organisationID := clientTestOrganisation(t, testWorkspaceID)
	foreignWorkspaceID := dbfx.Insert(t, "workspace", testutil.Cols{
		"name": "Other client workspace", "slug": "client-other-" + uuid.NewString(),
	})
	foreignOrganisationID := clientTestOrganisation(t, foreignWorkspaceID)
	for _, tc := range []struct {
		name          string
		role          string
		organisation  *string
		existingRole  string
		requesterRole string
		status        int
	}{
		{name: "client", role: "client", organisation: &organisationID, status: http.StatusCreated},
		{name: "missing organisation", role: "client", status: http.StatusBadRequest},
		{name: "foreign organisation", role: "client", organisation: &foreignOrganisationID, status: http.StatusBadRequest},
		{name: "staff organisation", role: "member", organisation: &organisationID, status: http.StatusBadRequest},
		{name: "client with staff membership", role: "client", organisation: &organisationID, existingRole: "member", status: http.StatusConflict},
		{name: "staff invite to client account", role: "member", existingRole: "client", status: http.StatusConflict},
		{name: "member requester", role: "client", organisation: &organisationID, requesterRole: "member", status: http.StatusForbidden},
		{name: "admin requester", role: "client", organisation: &organisationID, requesterRole: "admin", status: http.StatusCreated},
		{name: "staff preserved", role: "admin", status: http.StatusCreated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			email := "client-invite-" + uuid.NewString() + "@multica.ai"
			userID := dbfx.User(t, "Client invitee", email)
			if tc.existingRole != "" {
				cols := testutil.Cols{"workspace_id": foreignWorkspaceID, "user_id": userID, "role": tc.existingRole}
				if tc.existingRole == "client" {
					cols["organisation_id"] = foreignOrganisationID
				}
				dbfx.Insert(t, "member", cols)
			}
			requesterID := testUserID
			if tc.requesterRole != "" {
				requesterID = dbfx.User(t, "Client inviter", "client-inviter-"+uuid.NewString()+"@multica.ai")
				dbfx.Member(t, testWorkspaceID, requesterID, tc.requesterRole)
			}
			req := newRequest("POST", "/api/workspaces/"+testWorkspaceID+"/members", CreateMemberRequest{
				Email: email, Role: tc.role, OrganisationID: tc.organisation,
			})
			req.Header.Set("X-User-ID", requesterID)
			req = withURLParam(req, "id", testWorkspaceID)
			rec := testutil.Call(t, testHandler.CreateInvitation, req).Want(tc.status)
			if tc.status != http.StatusCreated {
				if n := dbfx.Count(t, `SELECT count(*) FROM workspace_invitation WHERE invitee_email=$1`, email); n != 0 {
					t.Fatalf("refused invitation persisted: %d", n)
				}
				return
			}
			var invitation InvitationResponse
			rec.JSON(&invitation)
			dbfx.Cleanup(t, `DELETE FROM workspace_invitation WHERE id=$1`, invitation.ID)
			if invitation.Role != tc.role || invitation.Status != "pending" || invitation.InviteeUserID == nil || *invitation.InviteeUserID != userID {
				t.Fatalf("invitation identity/role changed: %+v", invitation)
			}
			if tc.organisation == nil {
				if invitation.OrganisationID != nil {
					t.Fatalf("staff has organisation: %+v", invitation)
				}
			} else if invitation.OrganisationID == nil || *invitation.OrganisationID != *tc.organisation {
				t.Fatalf("organisation not preserved: %+v", invitation)
			}
		})
	}
}

func TestAcceptInvitation_ClientRules(t *testing.T) {
	organisationID := clientTestOrganisation(t, testWorkspaceID)
	otherWorkspaceID := dbfx.Insert(t, "workspace", testutil.Cols{
		"name": "Other acceptance workspace", "slug": "client-accept-other-" + uuid.NewString(),
	})
	otherOrganisationID := clientTestOrganisation(t, otherWorkspaceID)
	for _, tc := range []struct {
		name         string
		role         string
		organisation string
		existingRole string
		status       int
	}{
		{name: "accept client", role: "client", organisation: organisationID, status: http.StatusOK},
		{name: "missing organisation", role: "client", organisation: uuid.NewString(), status: http.StatusConflict},
		{name: "foreign organisation", role: "client", organisation: otherOrganisationID, status: http.StatusConflict},
		{name: "existing staff membership", role: "client", organisation: organisationID, existingRole: "member", status: http.StatusConflict},
		{name: "staff invitation to client account", role: "member", existingRole: "client", status: http.StatusConflict},
		{name: "staff multiple workspaces preserved", role: "admin", existingRole: "member", status: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			email := "client-accept-" + uuid.NewString() + "@multica.ai"
			userID := dbfx.User(t, "Client acceptance", email)
			if tc.existingRole != "" {
				cols := testutil.Cols{"workspace_id": otherWorkspaceID, "user_id": userID, "role": tc.existingRole}
				if tc.existingRole == "client" {
					cols["organisation_id"] = otherOrganisationID
				}
				dbfx.Insert(t, "member", cols)
			}
			cols := testutil.Cols{
				"workspace_id": testWorkspaceID, "inviter_id": testUserID,
				"invitee_email": email, "invitee_user_id": userID, "role": tc.role,
				"expires_at": testutil.Raw("now() + interval '1 day'"),
			}
			if tc.organisation != "" {
				cols["organisation_id"] = tc.organisation
			}
			invitationID := dbfx.Insert(t, "workspace_invitation", cols)
			req := newRequest("POST", "/api/invitations/"+invitationID+"/accept", nil)
			req.Header.Set("X-User-ID", userID)
			req = withURLParam(req, "id", invitationID)
			rec := testutil.Call(t, testHandler.AcceptInvitation, req).Want(tc.status)
			var status string
			dbfx.QueryRow(t, `SELECT status FROM workspace_invitation WHERE id=$1`, invitationID).Scan(&status)
			if tc.status != http.StatusOK {
				if status != "pending" || dbfx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, userID) != 0 {
					t.Fatal("refused acceptance settled invitation or created membership")
				}
				return
			}
			var member MemberWithUserResponse
			rec.JSON(&member)
			dbfx.Cleanup(t, `DELETE FROM member WHERE id=$1`, member.ID)
			if status != "accepted" || member.Role != tc.role || member.UserID != userID {
				t.Fatalf("acceptance changed identity or role: %+v status=%s", member, status)
			}
			if tc.role == "client" && (member.OrganisationID == nil || *member.OrganisationID != organisationID) {
				t.Fatalf("acceptance lost organisation: %+v", member)
			}
			// A later acceptance retry must not upgrade the client either.
			var retried MemberWithUserResponse
			testutil.Call(t, testHandler.AcceptInvitation, req).Want(http.StatusOK).JSON(&retried)
			if retried.Role != member.Role || retried.ID != member.ID {
				t.Fatalf("retry changed membership: %+v", retried)
			}
		})
	}
}

func TestAcceptInvitation_ClientFailureLeavesInvitationPending(t *testing.T) {
	for _, commitFailure := range []bool{false, true} {
		name := "missing organisation"
		if commitFailure {
			name = "commit failure"
		}
		t.Run(name, func(t *testing.T) {
			email := "client-accept-failure-" + uuid.NewString() + "@multica.ai"
			userID := dbfx.User(t, "Client failed acceptance", email)
			organisationID := uuid.NewString()
			if commitFailure {
				organisationID = clientTestOrganisation(t, testWorkspaceID)
			}
			invitationID := dbfx.Insert(t, "workspace_invitation", testutil.Cols{
				"workspace_id": testWorkspaceID, "inviter_id": testUserID,
				"invitee_email": email, "role": "client", "organisation_id": organisationID,
				"expires_at": testutil.Raw("now() + interval '1 day'"),
			})
			h := *testHandler
			capacity := &stubSeatCapacity{}
			h.SeatCapacity = capacity
			wantStatus := http.StatusConflict
			if commitFailure {
				h.SeatCapacity = nil
				h.TxStarter = rollbackOnCommitTxStarter{pool: testPool}
				wantStatus = http.StatusInternalServerError
			}
			req := newRequest("POST", "/api/invitations/"+invitationID+"/accept", nil)
			req.Header.Set("X-User-ID", userID)
			req = withURLParam(req, "id", invitationID)
			testutil.Call(t, h.AcceptInvitation, req).Want(wantStatus)
			var status string
			dbfx.QueryRow(t, `SELECT status FROM workspace_invitation WHERE id=$1`, invitationID).Scan(&status)
			if status != "pending" || dbfx.Count(t, `SELECT count(*) FROM member WHERE user_id=$1`, userID) != 0 {
				t.Fatal("failed acceptance settled invitation or created membership")
			}
			if capacity.consumeCalls != 0 {
				t.Fatal("invalid organisation consumed capacity")
			}
		})
	}
}

func TestAcceptInvitation_ClientSerializesWithEveryStaffMembershipWrite(t *testing.T) {
	for _, competingWrite := range []string{"invitation", "share link", "create workspace"} {
		t.Run(competingWrite, func(t *testing.T) {
			email := "client-concurrent-" + uuid.NewString() + "@multica.ai"
			userID := dbfx.User(t, "Concurrent client membership", email)
			organisationID := clientTestOrganisation(t, testWorkspaceID)
			invitationID := dbfx.Insert(t, "workspace_invitation", testutil.Cols{
				"workspace_id": testWorkspaceID, "inviter_id": testUserID,
				"invitee_email": email, "role": "client", "organisation_id": organisationID,
				"expires_at": testutil.Raw("now() + interval '1 day'"),
			})
			dbfx.Cleanup(t, `DELETE FROM member WHERE user_id=$1`, userID)
			otherWorkspaceID := dbfx.Insert(t, "workspace", testutil.Cols{
				"name": "Concurrent staff workspace", "slug": "client-concurrent-other-" + uuid.NewString(),
			})
			h := *testHandler
			h.SeatCapacity = nil
			clientReq := newRequest("POST", "/api/invitations/"+invitationID+"/accept", nil)
			clientReq.Header.Set("X-User-ID", userID)
			clientReq = withURLParam(clientReq, "id", invitationID)
			var staffReq *http.Request
			var staffHandler http.HandlerFunc
			staffSuccess := http.StatusOK
			switch competingWrite {
			case "invitation":
				staffInvitationID := dbfx.Insert(t, "workspace_invitation", testutil.Cols{
					"workspace_id": otherWorkspaceID, "inviter_id": testUserID,
					"invitee_email": email, "role": "member",
					"expires_at": testutil.Raw("now() + interval '1 day'"),
				})
				staffReq = newRequest("POST", "/api/invitations/"+staffInvitationID+"/accept", nil)
				staffReq = withURLParam(staffReq, "id", staffInvitationID)
				staffHandler = h.AcceptInvitation
			case "share link":
				link := createTestShareLink(t, otherWorkspaceID, "member", 0, 24)
				staffReq = newRequest("POST", "/api/share-links/join", JoinByShareLinkRequest{Code: link.Code})
				staffHandler = h.JoinByShareLink
			case "create workspace":
				slug := "client-concurrent-created-" + uuid.NewString()
				dbfx.Cleanup(t, `DELETE FROM workspace WHERE slug=$1`, slug)
				staffReq = newRequest("POST", "/api/workspaces", CreateWorkspaceRequest{Name: "Concurrent created workspace", Slug: slug})
				staffHandler = h.CreateWorkspace
				staffSuccess = http.StatusCreated
			}
			staffReq.Header.Set("X-User-ID", userID)
			type result struct {
				client bool
				status int
				body   string
			}
			results := make(chan result, 2)
			start := make(chan struct{})
			go func() {
				<-start
				rec := httptest.NewRecorder()
				h.AcceptInvitation(rec, clientReq)
				results <- result{client: true, status: rec.Code, body: rec.Body.String()}
			}()
			go func() {
				<-start
				rec := httptest.NewRecorder()
				staffHandler(rec, staffReq)
				results <- result{status: rec.Code, body: rec.Body.String()}
			}()
			close(start)
			successes := 0
			for range 2 {
				result := <-results
				wantSuccess := staffSuccess
				if result.client {
					wantSuccess = http.StatusOK
				}
				if result.status == wantSuccess {
					successes++
				} else if result.status != http.StatusConflict {
					t.Errorf("unexpected concurrent response: %+v", result)
				}
			}
			if successes != 1 || dbfx.Count(t, `SELECT count(*) FROM member WHERE user_id=$1`, userID) != 1 {
				t.Fatalf("concurrent membership writes violated single membership: successes=%d", successes)
			}
		})
	}
}

func TestClientMembership_LaterLoginAndLogoutPreserveBinding(t *testing.T) {
	organisationID := clientTestOrganisation(t, testWorkspaceID)
	userID, memberID := clientTestMember(t, testWorkspaceID, "client", organisationID)
	var email string
	dbfx.QueryRow(t, `SELECT email FROM "user" WHERE id=$1`, userID).Scan(&email)
	dbfx.Insert(t, "verification_code", testutil.Cols{
		"email": email, "code": "123456", "expires_at": testutil.Raw("now() + interval '10 minutes'"),
	})
	loginReq := testutil.JSONRequest("POST", "/auth/verify-code", VerifyCodeRequest{Email: email, Code: "123456"})
	var login LoginResponse
	loginRec := testutil.Call(t, testHandler.VerifyCode, loginReq).Want(http.StatusOK)
	loginRec.JSON(&login)
	if login.Token == "" || login.User.ID != userID {
		t.Fatalf("client login changed identity: %+v", login)
	}
	protected := middleware.Auth(testHandler.Queries, nil, nil, nil)(http.HandlerFunc(testHandler.ListMyMemberships))
	membershipsReq := httptest.NewRequest("GET", "/api/me/memberships", nil)
	for _, cookie := range loginRec.Result().Cookies() {
		membershipsReq.AddCookie(cookie)
	}
	var response struct {
		Memberships []MyMembershipResponse `json:"memberships"`
	}
	testutil.Call(t, protected.ServeHTTP, membershipsReq).Want(http.StatusOK).JSON(&response)
	if len(response.Memberships) != 1 || response.Memberships[0].Role != "client" ||
		response.Memberships[0].Organisation == nil || response.Memberships[0].Organisation.ID != organisationID {
		t.Fatalf("later login changed membership: %+v", response)
	}
	logoutReq := httptest.NewRequest("POST", "/auth/logout", nil)
	logoutRec := testutil.Call(t, testHandler.Logout, logoutReq).Want(http.StatusOK)
	cleared := make(map[string]bool)
	for _, cookie := range logoutRec.Result().Cookies() {
		if cookie.Value == "" && cookie.MaxAge < 0 {
			cleared[cookie.Name] = true
		}
	}
	for _, cookie := range loginRec.Result().Cookies() {
		if !cleared[cookie.Name] {
			t.Errorf("logout did not clear session cookie %s", cookie.Name)
		}
	}
	testutil.Call(t, protected.ServeHTTP, httptest.NewRequest("GET", "/api/me/memberships", nil)).Want(http.StatusUnauthorized)
	var role, binding string
	dbfx.QueryRow(t, `SELECT role, organisation_id::text FROM member WHERE id=$1`, memberID).Scan(&role, &binding)
	if role != "client" || binding != organisationID {
		t.Fatalf("login/logout mutated membership: %s %s", role, binding)
	}
}

func TestUpdateMember_DemotionSerializesWithStaffInvitationAcceptance(t *testing.T) {
	fx := setupRevocationFixture(t, "handler-tests-client-concurrent-demotion", "daemon-client-concurrent-demotion")
	organisationID := clientTestOrganisation(t, fx.WorkspaceID)
	otherWorkspaceID := dbfx.Insert(t, "workspace", testutil.Cols{
		"name": "Concurrent demotion destination", "slug": "client-demotion-other-" + uuid.NewString(),
	})
	var email string
	dbfx.QueryRow(t, `SELECT email FROM "user" WHERE id=$1`, fx.TargetUserID).Scan(&email)
	invitationID := dbfx.Insert(t, "workspace_invitation", testutil.Cols{
		"workspace_id": otherWorkspaceID, "inviter_id": testUserID, "invitee_email": email,
		"role": "member", "expires_at": testutil.Raw("now() + interval '1 day'"),
	})
	dbfx.Cleanup(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, otherWorkspaceID, fx.TargetUserID)
	demotionReq := newRequest("PATCH", "/api/workspaces/"+fx.WorkspaceID+"/members/"+fx.MemberID, UpdateMemberRequest{
		Role: "client", OrganisationID: &organisationID,
	})
	demotionReq.Header.Set("X-Workspace-ID", fx.WorkspaceID)
	demotionReq = withURLParams(demotionReq, "id", fx.WorkspaceID, "memberId", fx.MemberID)
	acceptReq := newRequest("POST", "/api/invitations/"+invitationID+"/accept", nil)
	acceptReq.Header.Set("X-User-ID", fx.TargetUserID)
	acceptReq = withURLParam(acceptReq, "id", invitationID)
	h := *testHandler
	h.SeatCapacity = nil
	results := make(chan *httptest.ResponseRecorder, 2)
	start := make(chan struct{})
	go func() {
		<-start
		rec := httptest.NewRecorder()
		h.UpdateMember(rec, demotionReq)
		results <- rec
	}()
	go func() {
		<-start
		rec := httptest.NewRecorder()
		h.AcceptInvitation(rec, acceptReq)
		results <- rec
	}()
	close(start)
	successes := 0
	for range 2 {
		rec := <-results
		if rec.Code == http.StatusOK {
			successes++
		} else if rec.Code != http.StatusConflict {
			t.Errorf("concurrent demotion response = %d: %s", rec.Code, rec.Body.String())
		}
	}
	var role string
	dbfx.QueryRow(t, `SELECT role FROM member WHERE id=$1`, fx.MemberID).Scan(&role)
	count := dbfx.Count(t, `SELECT count(*) FROM member WHERE user_id=$1`, fx.TargetUserID)
	if successes != 1 || (role == "client" && count != 1) || (role != "client" && count != 2) {
		t.Fatalf("demotion raced past staff join: successes=%d role=%s count=%d", successes, role, count)
	}
}
