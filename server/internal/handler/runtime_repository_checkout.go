package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// A checkout check reads only what is on the machine; a clone fetches the
// whole repository first (design.md decision 3).
const (
	repositoryCheckoutCheckTimeout = 30 * time.Second
	repositoryCheckoutCloneTimeout = 10 * time.Minute
	// A finished request stays for 30 minutes so POST /api/repositories can
	// still find the check its dialog ran.
	repositoryCheckoutStoreRetention = 30 * time.Minute
)

// RuntimeRepositoryCheckoutRequest is one checkout check, optionally with a
// clone, on a runtime (ADR-0001). DaemonID and WorkspaceID are the runtime's,
// so a repository write takes its checkout from the check, never the client.
type RuntimeRepositoryCheckoutRequest struct {
	ID            string                         `json:"id"`
	RuntimeID     string                         `json:"runtime_id"`
	DaemonID      string                         `json:"daemon_id"`
	WorkspaceID   string                         `json:"workspace_id"`
	FullName      string                         `json:"full_name"`
	DefaultBranch string                         `json:"default_branch"`
	Path          string                         `json:"path"`
	Clone         bool                           `json:"clone"`
	Status        RuntimeLocalSkillRequestStatus `json:"status"`
	// Result is the daemon's answer once Status is completed.
	Result       *protocol.RepositoryCheckoutResult `json:"result,omitempty"`
	Error        string                             `json:"error,omitempty"`
	CreatedAt    time.Time                          `json:"created_at"`
	UpdatedAt    time.Time                          `json:"updated_at"`
	RunStartedAt *time.Time                         `json:"-"`
}

// passedFor reports whether this request is a completed, passed check of the
// repository on its current default branch.
func (r *RuntimeRepositoryCheckoutRequest) passedFor(workspaceID, fullName, defaultBranch string) bool {
	return r != nil &&
		r.Status == RuntimeLocalSkillCompleted &&
		r.Result != nil && r.Result.Passed &&
		r.WorkspaceID == workspaceID &&
		strings.EqualFold(r.FullName, fullName) &&
		r.DefaultBranch == defaultBranch &&
		r.Result.ResolvedPath != ""
}

// RepositoryCheckoutResponse is the UI's view of a check: the request's
// status with the daemon's result flattened in, as design.md §3 names it.
// LastTag is a bare version, as a repository's last_tag is.
type RepositoryCheckoutResponse struct {
	ID              string                               `json:"id"`
	RuntimeID       string                               `json:"runtime_id"`
	Status          RuntimeLocalSkillRequestStatus       `json:"status"`
	ResolvedPath    string                               `json:"resolved_path"`
	Passed          bool                                 `json:"passed"`
	Failures        []protocol.RepositoryCheckoutFailure `json:"failures"`
	VersionFiles    []string                             `json:"version_files"`
	LastTag         string                               `json:"last_tag"`
	InstallCommands []protocol.RepositoryInstallCommand  `json:"install_commands"`
	Error           string                               `json:"error"`
	CreatedAt       time.Time                            `json:"created_at"`
	UpdatedAt       time.Time                            `json:"updated_at"`
}

func repositoryCheckoutToResponse(req *RuntimeRepositoryCheckoutRequest) RepositoryCheckoutResponse {
	resp := RepositoryCheckoutResponse{
		ID:              req.ID,
		RuntimeID:       req.RuntimeID,
		Status:          req.Status,
		Failures:        []protocol.RepositoryCheckoutFailure{},
		VersionFiles:    []string{},
		InstallCommands: []protocol.RepositoryInstallCommand{},
		Error:           req.Error,
		CreatedAt:       req.CreatedAt,
		UpdatedAt:       req.UpdatedAt,
	}
	if req.Result == nil {
		return resp
	}
	resp.ResolvedPath = req.Result.ResolvedPath
	resp.Passed = req.Result.Passed
	resp.LastTag = checkLastTag(*req.Result).String
	if req.Result.Failures != nil {
		resp.Failures = req.Result.Failures
	}
	if req.Result.VersionFiles != nil {
		resp.VersionFiles = req.Result.VersionFiles
	}
	if req.Result.InstallCommands != nil {
		resp.InstallCommands = req.Result.InstallCommands
	}
	return resp
}

type RepositoryCheckoutStore interface {
	Create(ctx context.Context, req RuntimeRepositoryCheckoutRequest) (*RuntimeRepositoryCheckoutRequest, error)
	Get(ctx context.Context, id string) (*RuntimeRepositoryCheckoutRequest, error)
	HasPending(ctx context.Context, runtimeID string) (bool, error)
	PopPending(ctx context.Context, runtimeID string) (*RuntimeRepositoryCheckoutRequest, error)
	Complete(ctx context.Context, id string, result protocol.RepositoryCheckoutResult) error
	Fail(ctx context.Context, id, errMsg string) error
}

// InMemoryRepositoryCheckoutStore serves the fork's single-server deployment,
// as InMemoryDirectoryListingStore does. Reads return snapshots so heartbeat
// transitions cannot race HTTP encoding.
type InMemoryRepositoryCheckoutStore struct {
	mu       sync.Mutex
	requests map[string]*RuntimeRepositoryCheckoutRequest
}

func NewInMemoryRepositoryCheckoutStore() *InMemoryRepositoryCheckoutStore {
	return &InMemoryRepositoryCheckoutStore{requests: make(map[string]*RuntimeRepositoryCheckoutRequest)}
}

func cloneRepositoryCheckoutRequest(req *RuntimeRepositoryCheckoutRequest) *RuntimeRepositoryCheckoutRequest {
	if req == nil {
		return nil
	}
	copy := *req
	if req.Result != nil {
		result := *req.Result
		result.Failures = slices.Clone(req.Result.Failures)
		result.VersionFiles = slices.Clone(req.Result.VersionFiles)
		result.InstallCommands = slices.Clone(req.Result.InstallCommands)
		copy.Result = &result
	}
	if req.RunStartedAt != nil {
		startedAt := *req.RunStartedAt
		copy.RunStartedAt = &startedAt
	}
	return &copy
}

func applyRepositoryCheckoutTimeout(req *RuntimeRepositoryCheckoutRequest, now time.Time) {
	switch req.Status {
	case RuntimeLocalSkillPending:
		if now.Sub(req.CreatedAt) > runtimeLocalSkillPendingTimeout {
			req.Status = RuntimeLocalSkillTimeout
			req.Error = "daemon did not respond within 3 minutes"
			req.UpdatedAt = now
		}
	case RuntimeLocalSkillRunning:
		limit, message := repositoryCheckoutCheckTimeout, "daemon did not finish the check within 30 seconds"
		if req.Clone {
			limit, message = repositoryCheckoutCloneTimeout, "daemon did not finish the clone within 10 minutes"
		}
		if req.RunStartedAt != nil && now.Sub(*req.RunStartedAt) > limit {
			req.Status = RuntimeLocalSkillTimeout
			req.Error = message
			req.UpdatedAt = now
		}
	}
}

func (s *InMemoryRepositoryCheckoutStore) Create(_ context.Context, in RuntimeRepositoryCheckoutRequest) (*RuntimeRepositoryCheckoutRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for id, req := range s.requests {
		applyRepositoryCheckoutTimeout(req, now)
		if runtimeLocalSkillRequestTerminal(req.Status) && now.Sub(req.UpdatedAt) > repositoryCheckoutStoreRetention {
			delete(s.requests, id)
		}
	}
	req := &RuntimeRepositoryCheckoutRequest{
		ID:            randomID(),
		RuntimeID:     in.RuntimeID,
		DaemonID:      in.DaemonID,
		WorkspaceID:   in.WorkspaceID,
		FullName:      in.FullName,
		DefaultBranch: in.DefaultBranch,
		Path:          in.Path,
		Clone:         in.Clone,
		Status:        RuntimeLocalSkillPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	s.requests[req.ID] = req
	return cloneRepositoryCheckoutRequest(req), nil
}

func (s *InMemoryRepositoryCheckoutStore) Get(_ context.Context, id string) (*RuntimeRepositoryCheckoutRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req := s.requests[id]
	if req != nil {
		now := time.Now()
		applyRepositoryCheckoutTimeout(req, now)
		if runtimeLocalSkillRequestTerminal(req.Status) && now.Sub(req.UpdatedAt) > repositoryCheckoutStoreRetention {
			delete(s.requests, id)
			return nil, nil
		}
	}
	return cloneRepositoryCheckoutRequest(req), nil
}

func (s *InMemoryRepositoryCheckoutStore) HasPending(_ context.Context, runtimeID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, req := range s.requests {
		applyRepositoryCheckoutTimeout(req, now)
		if req.RuntimeID == runtimeID && req.Status == RuntimeLocalSkillPending {
			return true, nil
		}
	}
	return false, nil
}

func (s *InMemoryRepositoryCheckoutStore) PopPending(_ context.Context, runtimeID string) (*RuntimeRepositoryCheckoutRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var oldest *RuntimeRepositoryCheckoutRequest
	for _, req := range s.requests {
		applyRepositoryCheckoutTimeout(req, now)
		if req.RuntimeID == runtimeID && req.Status == RuntimeLocalSkillPending {
			if oldest == nil || req.CreatedAt.Before(oldest.CreatedAt) {
				oldest = req
			}
		}
	}
	if oldest != nil {
		oldest.Status = RuntimeLocalSkillRunning
		oldest.RunStartedAt = &now
		oldest.UpdatedAt = now
	}
	return cloneRepositoryCheckoutRequest(oldest), nil
}

func (s *InMemoryRepositoryCheckoutStore) Complete(_ context.Context, id string, result protocol.RepositoryCheckoutResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req := s.requests[id]; req != nil {
		applyRepositoryCheckoutTimeout(req, time.Now())
		if !runtimeLocalSkillRequestTerminal(req.Status) {
			if result.Failures == nil {
				result.Failures = []protocol.RepositoryCheckoutFailure{}
			}
			if result.VersionFiles == nil {
				result.VersionFiles = []string{}
			}
			if result.InstallCommands == nil {
				result.InstallCommands = []protocol.RepositoryInstallCommand{}
			}
			req.Status = RuntimeLocalSkillCompleted
			req.Result = &result
			req.UpdatedAt = time.Now()
		}
	}
	return nil
}

func (s *InMemoryRepositoryCheckoutStore) Fail(_ context.Context, id, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req := s.requests[id]; req != nil {
		applyRepositoryCheckoutTimeout(req, time.Now())
		if !runtimeLocalSkillRequestTerminal(req.Status) {
			req.Status = RuntimeLocalSkillFailed
			req.Error = errMsg
			req.UpdatedAt = time.Now()
		}
	}
	return nil
}

// InitiateRepositoryCheckout enqueues a checkout check, or a clone and check,
// on the runtime. Only the runtime's owner may touch its disk (ADR-0001).
func (h *Handler) InitiateRepositoryCheckout(w http.ResponseWriter, r *http.Request) {
	rt, ok := h.requireRuntimeLocalSkillAccess(w, r, obsmetrics.RuntimeLookupSourceRuntimeAPI, chi.URLParam(r, "runtimeId"))
	if !ok {
		return
	}
	if rt.status != "online" {
		writeError(w, http.StatusServiceUnavailable, "runtime is offline")
		return
	}

	var body struct {
		FullName      string `json:"full_name"`
		DefaultBranch string `json:"default_branch"`
		Path          string `json:"path"`
		Clone         bool   `json:"clone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	body.FullName = strings.TrimSpace(body.FullName)
	body.DefaultBranch = strings.TrimSpace(body.DefaultBranch)
	body.Path = strings.TrimSpace(body.Path)
	if owner, name, found := strings.Cut(body.FullName, "/"); !found || owner == "" || name == "" || strings.Contains(name, "/") {
		writeError(w, http.StatusBadRequest, "full_name must be owner/name")
		return
	}
	if body.DefaultBranch == "" {
		writeError(w, http.StatusBadRequest, "default_branch is required")
		return
	}
	if body.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	runtime, err := h.Queries.GetAgentRuntime(r.Context(), parseUUID(rt.runtimeID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load runtime")
		return
	}
	req, err := h.RepositoryCheckoutStore.Create(r.Context(), RuntimeRepositoryCheckoutRequest{
		RuntimeID:     rt.runtimeID,
		DaemonID:      runtime.DaemonID.String,
		WorkspaceID:   rt.workspaceID,
		FullName:      body.FullName,
		DefaultBranch: body.DefaultBranch,
		Path:          body.Path,
		Clone:         body.Clone,
	})
	if err != nil {
		slog.Error("repository checkout enqueue failed", "error", err, "runtime_id", rt.runtimeID)
		writeError(w, http.StatusInternalServerError, "failed to enqueue repository checkout")
		return
	}
	h.requestDaemonPendingWork(rt.runtimeID, protocol.PendingWorkKindRepositoryCheckout)
	writeJSON(w, http.StatusOK, repositoryCheckoutToResponse(req))
}

// GetRepositoryCheckout is the UI's poll of one check.
func (h *Handler) GetRepositoryCheckout(w http.ResponseWriter, r *http.Request) {
	rt, ok := h.requireRuntimeLocalSkillAccess(w, r, obsmetrics.RuntimeLookupSourceRuntimeLocalSkillPoll, chi.URLParam(r, "runtimeId"))
	if !ok {
		return
	}
	req, err := h.RepositoryCheckoutStore.Get(r.Context(), chi.URLParam(r, "requestId"))
	if err != nil {
		slog.Error("repository checkout load failed", "error", err, "runtime_id", rt.runtimeID)
		writeError(w, http.StatusInternalServerError, "failed to load request")
		return
	}
	if req == nil || req.RuntimeID != rt.runtimeID {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}
	writeJSON(w, http.StatusOK, repositoryCheckoutToResponse(req))
}

// ReportRepositoryCheckoutResult is the daemon's answer to a claimed check.
func (h *Handler) ReportRepositoryCheckoutResult(w http.ResponseWriter, r *http.Request) {
	rt, ok := h.requireDaemonRuntimeAccess(w, r, chi.URLParam(r, "runtimeId"))
	if !ok {
		return
	}
	runtimeID := uuidToString(rt.ID)
	requestID := chi.URLParam(r, "requestId")
	req, err := h.RepositoryCheckoutStore.Get(r.Context(), requestID)
	if err != nil {
		slog.Error("repository checkout load failed", "error", err, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, "failed to load request")
		return
	}
	if req == nil || req.RuntimeID != runtimeID {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}
	if runtimeLocalSkillRequestTerminal(req.Status) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	var body struct {
		protocol.RepositoryCheckoutResult
		Error string `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Status == "completed" {
		err = h.RepositoryCheckoutStore.Complete(r.Context(), requestID, body.RepositoryCheckoutResult)
	} else {
		err = h.RepositoryCheckoutStore.Fail(r.Context(), requestID, body.Error)
	}
	if err != nil {
		slog.Error("repository checkout result failed", "error", err, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, "failed to persist result")
		return
	}
	if body.Status == "completed" && body.Passed && body.ResolvedPath != "" {
		h.backfillRepositoryLastTag(r.Context(), req, body.RepositoryCheckoutResult)
		h.backfillRepositoryInstallCommand(r.Context(), req, &body.RepositoryCheckoutResult)
	}
	slog.Debug("runtime repository checkout report", "runtime_id", runtimeID, "request_id", requestID, "status", body.Status, "passed", body.Passed)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// backfillRepositoryInstallCommand gives a repository from before install
// commands the first command a passed check found, or ” (None), when the
// check names its checkout and its command is still NULL (design.md
// decision 7). Each row it sets publishes repository:updated.
func (h *Handler) backfillRepositoryInstallCommand(ctx context.Context, req *RuntimeRepositoryCheckoutRequest, result *protocol.RepositoryCheckoutResult) {
	rows, err := h.Queries.BackfillRepositoryInstallCommand(ctx, db.BackfillRepositoryInstallCommandParams{
		InstallCommand:   pgtype.Text{String: foundInstallCommand(result), Valid: true},
		WorkspaceID:      parseUUID(req.WorkspaceID),
		FullName:         req.FullName,
		CheckoutDaemonID: req.DaemonID,
		CheckoutPath:     result.ResolvedPath,
	})
	if err != nil {
		slog.Error("repository install command backfill failed", "error", err, "request_id", req.ID, "full_name", req.FullName)
		return
	}
	for _, row := range rows {
		h.publish(protocol.EventRepositoryUpdated, req.WorkspaceID, "system", "", protocol.RepositoryEventPayload{
			RepositoryID: uuidToString(row.ID),
			ProjectID:    uuidToString(row.ProjectID),
		})
	}
}
