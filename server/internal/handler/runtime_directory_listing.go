package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type RuntimeDirectoryEntry struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Hidden bool   `json:"hidden"`
}

type RuntimeDirectoryListingRequest struct {
	ID           string                         `json:"id"`
	RuntimeID    string                         `json:"runtime_id"`
	Path         string                         `json:"path"`
	Status       RuntimeLocalSkillRequestStatus `json:"status"`
	ResolvedPath string                         `json:"resolved_path"`
	Entries      []RuntimeDirectoryEntry        `json:"entries"`
	Error        string                         `json:"error,omitempty"`
	CreatedAt    time.Time                      `json:"created_at"`
	UpdatedAt    time.Time                      `json:"updated_at"`
	RunStartedAt *time.Time                     `json:"-"`
}

type DirectoryListingStore interface {
	Create(ctx context.Context, runtimeID, path string) (*RuntimeDirectoryListingRequest, error)
	Get(ctx context.Context, id string) (*RuntimeDirectoryListingRequest, error)
	HasPending(ctx context.Context, runtimeID string) (bool, error)
	PopPending(ctx context.Context, runtimeID string) (*RuntimeDirectoryListingRequest, error)
	Complete(ctx context.Context, id, path string, entries []RuntimeDirectoryEntry) error
	Fail(ctx context.Context, id, errMsg string) error
}

// InMemoryDirectoryListingStore serves the fork's single-server deployment.
// Reads return snapshots so heartbeat transitions cannot race HTTP encoding.
type InMemoryDirectoryListingStore struct {
	mu       sync.Mutex
	requests map[string]*RuntimeDirectoryListingRequest
}

func NewInMemoryDirectoryListingStore() *InMemoryDirectoryListingStore {
	return &InMemoryDirectoryListingStore{requests: make(map[string]*RuntimeDirectoryListingRequest)}
}

func cloneDirectoryListingRequest(req *RuntimeDirectoryListingRequest) *RuntimeDirectoryListingRequest {
	if req == nil {
		return nil
	}
	copy := *req
	copy.Entries = slices.Clone(req.Entries)
	if req.RunStartedAt != nil {
		startedAt := *req.RunStartedAt
		copy.RunStartedAt = &startedAt
	}
	return &copy
}

func applyDirectoryListingTimeout(req *RuntimeDirectoryListingRequest, now time.Time) {
	switch req.Status {
	case RuntimeLocalSkillPending:
		if now.Sub(req.CreatedAt) > runtimeLocalSkillPendingTimeout {
			req.Status = RuntimeLocalSkillTimeout
			req.Error = "daemon did not respond within 3 minutes"
			req.UpdatedAt = now
		}
	case RuntimeLocalSkillRunning:
		if req.RunStartedAt != nil && now.Sub(*req.RunStartedAt) > runtimeLocalSkillRunningTimeout {
			req.Status = RuntimeLocalSkillTimeout
			req.Error = "daemon did not finish within 60 seconds"
			req.UpdatedAt = now
		}
	}
}

func (s *InMemoryDirectoryListingStore) Create(_ context.Context, runtimeID, path string) (*RuntimeDirectoryListingRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for id, req := range s.requests {
		if now.Sub(req.CreatedAt) > runtimeLocalSkillStoreRetention {
			delete(s.requests, id)
		}
	}
	req := &RuntimeDirectoryListingRequest{
		ID: randomID(), RuntimeID: runtimeID, Path: path,
		Status: RuntimeLocalSkillPending, Entries: []RuntimeDirectoryEntry{},
		CreatedAt: now, UpdatedAt: now,
	}
	s.requests[req.ID] = req
	return cloneDirectoryListingRequest(req), nil
}

func (s *InMemoryDirectoryListingStore) Get(_ context.Context, id string) (*RuntimeDirectoryListingRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req := s.requests[id]
	if req != nil {
		applyDirectoryListingTimeout(req, time.Now())
	}
	return cloneDirectoryListingRequest(req), nil
}

func (s *InMemoryDirectoryListingStore) HasPending(_ context.Context, runtimeID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, req := range s.requests {
		applyDirectoryListingTimeout(req, now)
		if req.RuntimeID == runtimeID && req.Status == RuntimeLocalSkillPending {
			return true, nil
		}
	}
	return false, nil
}

func (s *InMemoryDirectoryListingStore) PopPending(_ context.Context, runtimeID string) (*RuntimeDirectoryListingRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var oldest *RuntimeDirectoryListingRequest
	now := time.Now()
	for _, req := range s.requests {
		applyDirectoryListingTimeout(req, now)
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
	return cloneDirectoryListingRequest(oldest), nil
}

func (s *InMemoryDirectoryListingStore) Complete(_ context.Context, id, path string, entries []RuntimeDirectoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req := s.requests[id]; req != nil {
		applyDirectoryListingTimeout(req, time.Now())
		if !runtimeLocalSkillRequestTerminal(req.Status) {
			req.Status = RuntimeLocalSkillCompleted
			req.Path = path
			req.ResolvedPath = path
			req.Entries = slices.Clone(entries)
			if req.Entries == nil {
				req.Entries = []RuntimeDirectoryEntry{}
			}
			req.UpdatedAt = time.Now()
		}
	}
	return nil
}

func (s *InMemoryDirectoryListingStore) Fail(_ context.Context, id, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req := s.requests[id]; req != nil {
		applyDirectoryListingTimeout(req, time.Now())
		if !runtimeLocalSkillRequestTerminal(req.Status) {
			req.Status = RuntimeLocalSkillFailed
			req.Error = errMsg
			req.UpdatedAt = time.Now()
		}
	}
	return nil
}

func (h *Handler) InitiateDirectoryListing(w http.ResponseWriter, r *http.Request) {
	rt, ok := h.requireRuntimeLocalSkillAccess(w, r, obsmetrics.RuntimeLookupSourceRuntimeAPI, chi.URLParam(r, "runtimeId"))
	if !ok {
		return
	}
	if rt.status != "online" {
		writeError(w, http.StatusServiceUnavailable, "runtime is offline")
		return
	}

	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req, err := h.DirectoryListingStore.Create(r.Context(), rt.runtimeID, body.Path)
	if err != nil {
		slog.Error("directory listing enqueue failed", "error", err, "runtime_id", rt.runtimeID)
		writeError(w, http.StatusInternalServerError, "failed to enqueue directory listing")
		return
	}
	h.requestDaemonPendingWork(rt.runtimeID, protocol.PendingWorkKindDirectoryListing)
	writeJSON(w, http.StatusOK, req)
}

func (h *Handler) GetDirectoryListingRequest(w http.ResponseWriter, r *http.Request) {
	rt, ok := h.requireRuntimeLocalSkillAccess(w, r, obsmetrics.RuntimeLookupSourceRuntimeLocalSkillPoll, chi.URLParam(r, "runtimeId"))
	if !ok {
		return
	}
	if rt.status != "online" {
		writeError(w, http.StatusServiceUnavailable, "runtime is offline")
		return
	}
	req, err := h.DirectoryListingStore.Get(r.Context(), chi.URLParam(r, "requestId"))
	if err != nil {
		slog.Error("directory listing load failed", "error", err, "runtime_id", rt.runtimeID)
		writeError(w, http.StatusInternalServerError, "failed to load request")
		return
	}
	if req == nil || req.RuntimeID != rt.runtimeID {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}
	writeJSON(w, http.StatusOK, req)
}

func (h *Handler) ReportDirectoryListingResult(w http.ResponseWriter, r *http.Request) {
	rt, ok := h.requireDaemonRuntimeAccess(w, r, chi.URLParam(r, "runtimeId"))
	if !ok {
		return
	}
	runtimeID := uuidToString(rt.ID)
	requestID := chi.URLParam(r, "requestId")
	req, err := h.DirectoryListingStore.Get(r.Context(), requestID)
	if err != nil {
		slog.Error("directory listing load failed", "error", err, "request_id", requestID)
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
		Status  string                  `json:"status"`
		Path    string                  `json:"path"`
		Entries []RuntimeDirectoryEntry `json:"entries"`
		Error   string                  `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Status == "completed" {
		err = h.DirectoryListingStore.Complete(r.Context(), requestID, body.Path, body.Entries)
	} else {
		err = h.DirectoryListingStore.Fail(r.Context(), requestID, body.Error)
	}
	if err != nil {
		slog.Error("directory listing result failed", "error", err, "request_id", requestID)
		writeError(w, http.StatusInternalServerError, "failed to persist result")
		return
	}
	slog.Debug("runtime directory listing report", "runtime_id", runtimeID, "request_id", requestID, "status", body.Status, "count", len(body.Entries))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
