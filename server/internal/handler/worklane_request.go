package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"

	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// A worklane request makes or removes a worklane's worktree on its machine
// (design decisions 2 to 4). It lives outside PostgreSQL, in memory or Redis
// like RepositoryCheckoutStore: the worklane row is saved only by the create
// request's last step, so a failed or lost request leaves nothing behind.

// Statuses of a worklane request.
const (
	worklaneRequestRunning   = "running"
	worklaneRequestFailed    = "failed"
	worklaneRequestCompleted = "completed"
	worklaneRequestRefused   = "refused"
)

// Steps of a create request, in order. open runs on the server.
const (
	worklaneStepFetch       = "fetch"
	worklaneStepAddWorktree = "add_worktree"
	worklaneStepInstall     = "install"
	worklaneStepOpen        = "open"
)

// States of a step.
const (
	worklaneStepPending = "pending"
	worklaneStepRunning = "running"
	worklaneStepDone    = "done"
	worklaneStepSkipped = "skipped"
	worklaneStepFailed  = "failed"
)

// Outcomes a daemon reports for a remove request.
const (
	worklaneOutcomeRemoved = "removed"
	worklaneOutcomeRefused = "refused"
	worklaneOutcomeFailed  = "failed"
)

const (
	// A finished request is kept this long after its last change.
	worklaneRequestRetention = 30 * time.Minute
	// A running request without progress for this long fails with lost.
	worklaneRequestLostAfter = 15 * time.Minute
)

var worklaneCreateSteps = []string{worklaneStepFetch, worklaneStepAddWorktree, worklaneStepInstall, worklaneStepOpen}

var worklaneFolderPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type WorklaneRequestStep struct {
	Key        string `json:"key"`
	State      string `json:"state"`
	Detail     string `json:"detail"`
	Output     string `json:"output"`
	DurationMs int64  `json:"duration_ms"`
}

type WorklaneRequestReason struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
	Count  *int   `json:"count,omitempty"`
}

// WorklaneRequest is the request record the UI polls. FullName,
// DefaultBranch, CheckoutPath, FromStep and IssueWorktreePaths are what the
// heartbeat hands the daemon.
type WorklaneRequest struct {
	ID                 string                                  `json:"id"`
	Kind               string                                  `json:"kind"`
	WorkspaceID        string                                  `json:"workspace_id"`
	RepositoryID       string                                  `json:"repository_id"`
	WorklaneID         *string                                 `json:"worklane_id"`
	Name               string                                  `json:"name"`
	Folder             string                                  `json:"folder"`
	Path               string                                  `json:"path"`
	Base               string                                  `json:"base"`
	InstallCommand     string                                  `json:"install_command"`
	DaemonID           string                                  `json:"daemon_id"`
	RuntimeID          string                                  `json:"runtime_id"`
	RuntimeName        string                                  `json:"runtime_name"`
	Status             string                                  `json:"status"`
	Steps              []WorklaneRequestStep                   `json:"steps"`
	Reason             *WorklaneRequestReason                  `json:"reason"`
	CreatedBy          *string                                 `json:"created_by"`
	FullName           string                                  `json:"full_name"`
	DefaultBranch      string                                  `json:"default_branch"`
	CheckoutPath       string                                  `json:"checkout_path"`
	FromStep           string                                  `json:"from_step,omitempty"`
	FromBranch         string                                  `json:"from_branch,omitempty"`
	ToBranch           string                                  `json:"to_branch,omitempty"`
	IssueWorktreePaths []protocol.WorklaneRequestIssueWorktree `json:"issue_worktree_paths,omitempty"`
	CreatedAt          time.Time                               `json:"created_at"`
	UpdatedAt          time.Time                               `json:"updated_at"`
}

func (req *WorklaneRequest) step(key string) *WorklaneRequestStep {
	for i := range req.Steps {
		if req.Steps[i].Key == key {
			return &req.Steps[i]
		}
	}
	return nil
}

// failedStep is the first failed step, or "".
func (req *WorklaneRequest) failedStep() string {
	for _, s := range req.Steps {
		if s.State == worklaneStepFailed {
			return s.Key
		}
	}
	return ""
}

func cloneWorklaneRequest(req *WorklaneRequest) *WorklaneRequest {
	if req == nil {
		return nil
	}
	c := *req
	c.Steps = slices.Clone(req.Steps)
	if c.Steps == nil {
		c.Steps = []WorklaneRequestStep{}
	}
	c.IssueWorktreePaths = slices.Clone(req.IssueWorktreePaths)
	if req.Reason != nil {
		reason := *req.Reason
		if req.Reason.Count != nil {
			count := *req.Reason.Count
			reason.Count = &count
		}
		c.Reason = &reason
	}
	if req.WorklaneID != nil {
		id := *req.WorklaneID
		c.WorklaneID = &id
	}
	if req.CreatedBy != nil {
		by := *req.CreatedBy
		c.CreatedBy = &by
	}
	return &c
}

// applyWorklaneRequestLost fails a running request that has had no progress
// for worklaneRequestLostAfter. It reports whether it changed the request.
func applyWorklaneRequestLost(req *WorklaneRequest, now time.Time) bool {
	if req.Status != worklaneRequestRunning || now.Sub(req.UpdatedAt) <= worklaneRequestLostAfter {
		return false
	}
	machine := req.RuntimeName
	if machine == "" {
		machine = "The machine"
	}
	detail := fmt.Sprintf("%s stopped answering", machine)
	for i := range req.Steps {
		if req.Steps[i].State == worklaneStepRunning || req.Steps[i].State == worklaneStepPending {
			req.Steps[i].State = worklaneStepFailed
			req.Steps[i].Detail = detail
			break
		}
	}
	req.Status = worklaneRequestFailed
	req.Reason = &WorklaneRequestReason{Code: "lost", Detail: detail}
	req.UpdatedAt = now
	return true
}

// WorklaneRequestStore keeps worklane requests and a pending queue per
// runtime that the heartbeat pops.
type WorklaneRequestStore interface {
	// Save writes the request record.
	Save(ctx context.Context, req *WorklaneRequest) error
	// Get returns the request, failing it with lost when it stalled, or nil.
	Get(ctx context.Context, id string) (*WorklaneRequest, error)
	// Enqueue queues the request for the runtime's next heartbeat.
	Enqueue(ctx context.Context, runtimeID, requestID string) error
	HasPending(ctx context.Context, runtimeID string) (bool, error)
	// PopPending returns the runtime's oldest queued running request, or nil.
	PopPending(ctx context.Context, runtimeID string) (*WorklaneRequest, error)
	// RunningCreate returns the running create request of a repository's
	// folder, or nil.
	RunningCreate(ctx context.Context, repositoryID, folder string) (*WorklaneRequest, error)
}

// InMemoryWorklaneRequestStore serves a single-server deployment.
type InMemoryWorklaneRequestStore struct {
	mu       sync.Mutex
	requests map[string]*WorklaneRequest
	pending  map[string][]string
}

func NewInMemoryWorklaneRequestStore() *InMemoryWorklaneRequestStore {
	return &InMemoryWorklaneRequestStore{
		requests: make(map[string]*WorklaneRequest),
		pending:  make(map[string][]string),
	}
}

func (s *InMemoryWorklaneRequestStore) sweep(now time.Time) {
	for id, req := range s.requests {
		applyWorklaneRequestLost(req, now)
		if req.Status != worklaneRequestRunning && now.Sub(req.UpdatedAt) > worklaneRequestRetention {
			delete(s.requests, id)
		}
	}
}

func (s *InMemoryWorklaneRequestStore) Save(_ context.Context, req *WorklaneRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweep(time.Now())
	s.requests[req.ID] = cloneWorklaneRequest(req)
	return nil
}

func (s *InMemoryWorklaneRequestStore) Get(_ context.Context, id string) (*WorklaneRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	req := s.requests[id]
	if req == nil {
		return nil, nil
	}
	applyWorklaneRequestLost(req, time.Now())
	return cloneWorklaneRequest(req), nil
}

func (s *InMemoryWorklaneRequestStore) Enqueue(_ context.Context, runtimeID, requestID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !slices.Contains(s.pending[runtimeID], requestID) {
		s.pending[runtimeID] = append(s.pending[runtimeID], requestID)
	}
	return nil
}

func (s *InMemoryWorklaneRequestStore) HasPending(_ context.Context, runtimeID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending[runtimeID]) > 0, nil
}

func (s *InMemoryWorklaneRequestStore) PopPending(_ context.Context, runtimeID string) (*WorklaneRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for len(s.pending[runtimeID]) > 0 {
		id := s.pending[runtimeID][0]
		s.pending[runtimeID] = s.pending[runtimeID][1:]
		req := s.requests[id]
		if req == nil {
			continue
		}
		applyWorklaneRequestLost(req, now)
		if req.Status == worklaneRequestRunning {
			if len(s.pending[runtimeID]) == 0 {
				delete(s.pending, runtimeID)
			}
			return cloneWorklaneRequest(req), nil
		}
	}
	delete(s.pending, runtimeID)
	return nil, nil
}

func (s *InMemoryWorklaneRequestStore) RunningCreate(_ context.Context, repositoryID, folder string) (*WorklaneRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for _, req := range s.requests {
		applyWorklaneRequestLost(req, now)
		if req.Kind == protocol.WorklaneRequestKindCreate && req.Status == worklaneRequestRunning &&
			req.RepositoryID == repositoryID && req.Folder == folder {
			return cloneWorklaneRequest(req), nil
		}
	}
	return nil, nil
}

// Redis layout:
//
//	<prefix><id>                         → JSON record, TTL = retention
//	<prefix>pending:<runtime_id>         → ZSET of queued ids by enqueue time
//	<prefix>folder:<repository>:<folder> → id of the folder's newest create
const worklaneRequestKeyPrefix = "mul:" + runtimePendingRedisHashTag + ":worklane_request:"

func worklaneRequestKey(id string) string { return worklaneRequestKeyPrefix + id }
func worklaneRequestPendingKey(runtimeID string) string {
	return worklaneRequestKeyPrefix + "pending:" + runtimeID
}
func worklaneRequestFolderKey(repositoryID, folder string) string {
	return worklaneRequestKeyPrefix + "folder:" + repositoryID + ":" + folder
}

// RedisWorklaneRequestStore lets every API node see the same requests.
type RedisWorklaneRequestStore struct {
	rdb redis.UniversalClient
}

func NewRedisWorklaneRequestStore(rdb redis.UniversalClient) *RedisWorklaneRequestStore {
	return &RedisWorklaneRequestStore{rdb: rdb}
}

func (s *RedisWorklaneRequestStore) Save(ctx context.Context, req *WorklaneRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal worklane request: %w", err)
	}
	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, worklaneRequestKey(req.ID), data, worklaneRequestRetention)
	if req.Kind == protocol.WorklaneRequestKindCreate {
		pipe.Set(ctx, worklaneRequestFolderKey(req.RepositoryID, req.Folder), req.ID, worklaneRequestRetention)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("persist worklane request: %w", err)
	}
	return nil
}

func (s *RedisWorklaneRequestStore) Get(ctx context.Context, id string) (*WorklaneRequest, error) {
	raw, err := s.rdb.Get(ctx, worklaneRequestKey(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get worklane request: %w", err)
	}
	var req WorklaneRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("decode worklane request: %w", err)
	}
	if applyWorklaneRequestLost(&req, time.Now()) {
		if err := s.Save(ctx, &req); err != nil {
			return nil, err
		}
	}
	return &req, nil
}

func (s *RedisWorklaneRequestStore) Enqueue(ctx context.Context, runtimeID, requestID string) error {
	key := worklaneRequestPendingKey(runtimeID)
	pipe := s.rdb.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(time.Now().UnixNano()), Member: requestID})
	pipe.Expire(ctx, key, worklaneRequestRetention*2)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("enqueue worklane request: %w", err)
	}
	return nil
}

func (s *RedisWorklaneRequestStore) HasPending(ctx context.Context, runtimeID string) (bool, error) {
	count, err := s.rdb.ZCard(ctx, worklaneRequestPendingKey(runtimeID)).Result()
	if err != nil {
		return false, fmt.Errorf("zcard worklane requests: %w", err)
	}
	return count > 0, nil
}

// PopPending claims with ZPOPMIN, so each queued id goes to one node only.
func (s *RedisWorklaneRequestStore) PopPending(ctx context.Context, runtimeID string) (*WorklaneRequest, error) {
	key := worklaneRequestPendingKey(runtimeID)
	for {
		popped, err := s.rdb.ZPopMin(ctx, key, 1).Result()
		if err != nil {
			return nil, fmt.Errorf("pop worklane request: %w", err)
		}
		if len(popped) == 0 {
			return nil, nil
		}
		id, _ := popped[0].Member.(string)
		req, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if req != nil && req.Status == worklaneRequestRunning {
			return req, nil
		}
	}
}

func (s *RedisWorklaneRequestStore) RunningCreate(ctx context.Context, repositoryID, folder string) (*WorklaneRequest, error) {
	id, err := s.rdb.Get(ctx, worklaneRequestFolderKey(repositoryID, folder)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get worklane folder request: %w", err)
	}
	req, err := s.Get(ctx, id)
	if err != nil || req == nil || req.Status != worklaneRequestRunning {
		return nil, err
	}
	return req, nil
}

// writeWorklaneRefusal answers a refused worklane action with its code and,
// for a taken name or folder or an offline machine, the name it is about.
func writeWorklaneRefusal(w http.ResponseWriter, status int, code, msg, detail string) {
	writeJSON(w, status, map[string]string{"error": msg, "code": code, "detail": detail})
}

// worklaneMachine is a daemon's runtimes in a workspace, seen as one machine.
type worklaneMachine struct {
	// runtime is the newest online runtime row, which carries the request.
	runtime db.AgentRuntime
	online  bool
	name    string
}

var runtimeHostnamePattern = regexp.MustCompile(`^.+?\s+\(([^)]+)\)$`)

// loadWorklaneMachine reads the machine of daemonID: its name follows the
// web's machine title (a custom name every runtime shares, else the host in
// the runtime name).
func (h *Handler) loadWorklaneMachine(ctx context.Context, workspaceID pgtype.UUID, daemonID string) (worklaneMachine, error) {
	all, err := h.Queries.ListAgentRuntimes(ctx, workspaceID)
	if err != nil {
		return worklaneMachine{}, err
	}
	var runtimes []db.AgentRuntime
	for _, rt := range all {
		if rt.DaemonID.Valid && rt.DaemonID.String == daemonID {
			runtimes = append(runtimes, rt)
		}
	}
	m := worklaneMachine{name: daemonID}
	if len(runtimes) == 0 {
		return m, nil
	}
	sort.SliceStable(runtimes, func(i, j int) bool {
		return runtimes[i].UpdatedAt.Time.After(runtimes[j].UpdatedAt.Time)
	})
	m.name = machineDisplayName(runtimes)
	for _, rt := range runtimes {
		if rt.Status == "online" {
			m.runtime = rt
			m.online = true
			break
		}
	}
	return m, nil
}

func machineDisplayName(runtimes []db.AgentRuntime) string {
	shared := ""
	for i, rt := range runtimes {
		name := ""
		if rt.CustomName.Valid {
			name = strings.TrimSpace(rt.CustomName.String)
		}
		if i == 0 {
			shared = name
		} else if name != shared {
			shared = ""
			break
		}
	}
	if shared != "" {
		return shared
	}
	if m := runtimeHostnamePattern.FindStringSubmatch(runtimes[0].Name); m != nil {
		return m[1]
	}
	return runtimes[0].Name
}

// worklanePath is where a new worklane's worktree goes: under the
// organisation key read now, or without an organisation directly under
// Worktrees (ADR-0006).
func worklanePath(organisationKey, fullName, folder string) string {
	repo := fullName
	if i := strings.LastIndex(fullName, "/"); i >= 0 {
		repo = fullName[i+1:]
	}
	if organisationKey == "" {
		return "~/Worktrees/" + repo + "/" + folder
	}
	return "~/Worktrees/" + organisationKey + "/" + repo + "/" + folder
}

func (h *Handler) organisationKeyOfProject(ctx context.Context, workspaceID, projectID pgtype.UUID) (string, error) {
	project, err := h.Queries.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: projectID, WorkspaceID: workspaceID})
	if err != nil {
		return "", err
	}
	if !project.OrganisationID.Valid {
		return "", nil
	}
	org, err := h.Queries.GetOrganisationInWorkspace(ctx, db.GetOrganisationInWorkspaceParams{ID: project.OrganisationID, WorkspaceID: workspaceID})
	if isNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return org.Key, nil
}

func (h *Handler) issueWorktreePathsOnDaemon(ctx context.Context, workspaceID, repositoryID pgtype.UUID, daemonID string) ([]protocol.WorklaneRequestIssueWorktree, error) {
	rows, err := h.Queries.ListIssueWorktreePathsForDaemon(ctx, db.ListIssueWorktreePathsForDaemonParams{
		WorkspaceID: workspaceID, RepositoryID: repositoryID, DaemonID: daemonID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]protocol.WorklaneRequestIssueWorktree, 0, len(rows))
	for _, row := range rows {
		out = append(out, protocol.WorklaneRequestIssueWorktree{Path: row.Path, IssueKey: row.IssueIdentifier})
	}
	return out, nil
}

// enqueueWorklaneRequest saves the request, queues it for the runtime and
// wakes its daemon.
func (h *Handler) enqueueWorklaneRequest(ctx context.Context, req *WorklaneRequest, runtimeID string) error {
	req.RuntimeID = runtimeID
	req.UpdatedAt = time.Now()
	if err := h.WorklaneRequestStore.Save(ctx, req); err != nil {
		return err
	}
	if err := h.WorklaneRequestStore.Enqueue(ctx, runtimeID, req.ID); err != nil {
		return err
	}
	h.requestDaemonPendingWork(runtimeID, protocol.PendingWorkKindWorklaneRequest)
	return nil
}

// CreateWorklaneRequest starts a new worklane: every refusal below comes
// before anything runs on the machine (design decision 2).
func (h *Handler) CreateWorklaneRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID := ctxWorkspaceID(ctx)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	var body struct {
		RepositoryID string `json:"repository_id"`
		Name         string `json:"name"`
		Folder       string `json:"folder"`
		DaemonID     string `json:"daemon_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	name := strings.TrimSpace(body.Name)
	folder := strings.TrimSpace(body.Folder)
	if name == "" {
		writeWorklaneRefusal(w, http.StatusBadRequest, "name_required", "a worklane needs a name", "")
		return
	}
	repoUUID, ok := parseUUIDOrBadRequest(w, body.RepositoryID, "repository id")
	if !ok {
		return
	}
	repo, err := h.Queries.GetRepositoryInWorkspace(ctx, db.GetRepositoryInWorkspaceParams{ID: repoUUID, WorkspaceID: wsUUID})
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "repository not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load repository")
		return
	}
	worklanes, err := h.repositoryWorklanes(ctx, wsUUID, repo.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load worklanes")
		return
	}
	for _, wl := range worklanes {
		if strings.EqualFold(wl.Name, name) {
			writeWorklaneRefusal(w, http.StatusConflict, "name_taken", fmt.Sprintf("%s is taken in this repository", wl.Name), wl.Name)
			return
		}
	}
	if !worklaneFolderPattern.MatchString(folder) {
		writeWorklaneRefusal(w, http.StatusBadRequest, "folder_invalid", "a folder has lowercase letters, digits and dashes, and starts with a letter or digit", "")
		return
	}
	for _, wl := range worklanes {
		if wl.Folder.Valid && wl.Folder.String == folder {
			writeWorklaneRefusal(w, http.StatusConflict, "folder_taken", fmt.Sprintf("%s already uses this folder", wl.Name), wl.Name)
			return
		}
	}
	if body.DaemonID != repo.CheckoutDaemonID {
		writeWorklaneRefusal(w, http.StatusBadRequest, "runtime_no_checkout", fmt.Sprintf("this machine has no checkout of %s", repo.FullName), "")
		return
	}
	machine, err := h.loadWorklaneMachine(ctx, wsUUID, body.DaemonID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load runtimes")
		return
	}
	if !machine.online {
		writeWorklaneRefusal(w, http.StatusConflict, "runtime_offline", fmt.Sprintf("%s is offline", machine.name), machine.name)
		return
	}
	// The install command runs on the machine, so only its owner may start it.
	if _, ok := h.requireRuntimeLocalSkillAccess(w, r, obsmetrics.RuntimeLookupSourceRuntimeAPI, uuidToString(machine.runtime.ID)); !ok {
		return
	}
	if !runtimeHasCapability(machine.runtime.Metadata, protocol.DaemonCapabilityWorklaneWorktreeV1) {
		writeWorklaneRefusal(w, http.StatusConflict, "runtime_cannot_make_worktrees", fmt.Sprintf("%s cannot make worktrees yet: update its Multica", machine.name), machine.name)
		return
	}
	if !repo.InstallCommand.Valid {
		writeWorklaneRefusal(w, http.StatusConflict, "install_command_unknown", "the install command of this repository is not known yet", "")
		return
	}
	running, err := h.WorklaneRequestStore.RunningCreate(ctx, uuidToString(repo.ID), folder)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load worklane requests")
		return
	}
	if running != nil {
		writeWorklaneRefusal(w, http.StatusConflict, "request_running", "a worklane is already being made in this folder", running.Name)
		return
	}
	orgKey, err := h.organisationKeyOfProject(ctx, wsUUID, repo.ProjectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load the organisation")
		return
	}
	issuePaths, err := h.issueWorktreePathsOnDaemon(ctx, wsUUID, repo.ID, body.DaemonID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load issue worktrees")
		return
	}

	now := time.Now()
	createdBy := requestUserID(r)
	req := &WorklaneRequest{
		ID:                 randomID(),
		Kind:               protocol.WorklaneRequestKindCreate,
		WorkspaceID:        workspaceID,
		RepositoryID:       uuidToString(repo.ID),
		Name:               name,
		Folder:             folder,
		Path:               worklanePath(orgKey, repo.FullName, folder),
		Base:               "origin/" + repo.DefaultBranch,
		InstallCommand:     repo.InstallCommand.String,
		DaemonID:           body.DaemonID,
		RuntimeName:        machine.name,
		Status:             worklaneRequestRunning,
		Steps:              make([]WorklaneRequestStep, 0, len(worklaneCreateSteps)),
		CreatedBy:          &createdBy,
		FullName:           repo.FullName,
		DefaultBranch:      repo.DefaultBranch,
		CheckoutPath:       repo.CheckoutPath,
		IssueWorktreePaths: issuePaths,
		CreatedAt:          now,
	}
	for _, key := range worklaneCreateSteps {
		req.Steps = append(req.Steps, WorklaneRequestStep{Key: key, State: worklaneStepPending})
	}
	if err := h.enqueueWorklaneRequest(ctx, req, uuidToString(machine.runtime.ID)); err != nil {
		slog.Error("worklane request enqueue failed", "error", err, "repository_id", req.RepositoryID)
		writeError(w, http.StatusInternalServerError, "failed to start the worklane")
		return
	}
	writeJSON(w, http.StatusCreated, req)
}

// loadWorkspaceWorklaneRequest reads a request of the caller's workspace.
func (h *Handler) loadWorkspaceWorklaneRequest(w http.ResponseWriter, r *http.Request) (*WorklaneRequest, bool) {
	req, err := h.WorklaneRequestStore.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load request")
		return nil, false
	}
	if req == nil || req.WorkspaceID != ctxWorkspaceID(r.Context()) {
		writeError(w, http.StatusNotFound, "request not found")
		return nil, false
	}
	return req, true
}

func (h *Handler) GetWorklaneRequest(w http.ResponseWriter, r *http.Request) {
	req, ok := h.loadWorkspaceWorklaneRequest(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, req)
}

// RetryWorklaneRequest runs a failed create request again from its failed
// step. A failed open reruns on the server alone.
func (h *Handler) RetryWorklaneRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req, ok := h.loadWorkspaceWorklaneRequest(w, r)
	if !ok {
		return
	}
	if req.Kind != protocol.WorklaneRequestKindCreate || req.Status != worklaneRequestFailed {
		writeError(w, http.StatusConflict, "only a failed request can be tried again")
		return
	}
	from := req.failedStep()
	if from == "" {
		from = worklaneStepFetch
	}
	reset := false
	for i := range req.Steps {
		if req.Steps[i].Key == from {
			reset = true
		}
		if reset {
			req.Steps[i] = WorklaneRequestStep{Key: req.Steps[i].Key, State: worklaneStepPending}
		}
	}
	req.Status = worklaneRequestRunning
	req.Reason = nil
	req.FromStep = from

	if from == worklaneStepOpen {
		h.openWorklane(ctx, req)
		req.UpdatedAt = time.Now()
		if err := h.WorklaneRequestStore.Save(ctx, req); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save request")
			return
		}
		writeJSON(w, http.StatusOK, req)
		return
	}

	wsUUID, ok := parseUUIDOrBadRequest(w, req.WorkspaceID, "workspace id")
	if !ok {
		return
	}
	machine, err := h.loadWorklaneMachine(ctx, wsUUID, req.DaemonID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load runtimes")
		return
	}
	if !machine.online {
		writeWorklaneRefusal(w, http.StatusConflict, "runtime_offline", fmt.Sprintf("%s is offline", machine.name), machine.name)
		return
	}
	if _, ok := h.requireRuntimeLocalSkillAccess(w, r, obsmetrics.RuntimeLookupSourceRuntimeAPI, uuidToString(machine.runtime.ID)); !ok {
		return
	}
	if !runtimeHasCapability(machine.runtime.Metadata, protocol.DaemonCapabilityWorklaneWorktreeV1) {
		writeWorklaneRefusal(w, http.StatusConflict, "runtime_cannot_make_worktrees", fmt.Sprintf("%s cannot make worktrees yet: update its Multica", machine.name), machine.name)
		return
	}
	if paths, err := h.issueWorktreePathsOnDaemon(ctx, wsUUID, parseUUID(req.RepositoryID), req.DaemonID); err == nil {
		req.IssueWorktreePaths = paths
	}
	if err := h.enqueueWorklaneRequest(ctx, req, uuidToString(machine.runtime.ID)); err != nil {
		slog.Error("worklane request retry failed", "error", err, "request_id", req.ID)
		writeError(w, http.StatusInternalServerError, "failed to try again")
		return
	}
	writeJSON(w, http.StatusOK, req)
}

// pendingWorklaneRequestItem is what the heartbeat hands the daemon.
func pendingWorklaneRequestItem(req *WorklaneRequest) *protocol.DaemonHeartbeatPendingWorklaneRequest {
	item := &protocol.DaemonHeartbeatPendingWorklaneRequest{
		ID:                 req.ID,
		Kind:               req.Kind,
		FullName:           req.FullName,
		DefaultBranch:      req.DefaultBranch,
		CheckoutPath:       req.CheckoutPath,
		Path:               req.Path,
		InstallCommand:     req.InstallCommand,
		FromStep:           req.FromStep,
		FromBranch:         req.FromBranch,
		ToBranch:           req.ToBranch,
		IssueWorktreePaths: req.IssueWorktreePaths,
	}
	if req.WorklaneID != nil {
		item.WorklaneID = *req.WorklaneID
	}
	return item
}

// ReportWorklaneRequestProgress takes the daemon's whole step list after each
// step change. Progress from another runtime or for an unknown request is 404.
func (h *Handler) ReportWorklaneRequestProgress(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rt, ok := h.requireDaemonRuntimeAccess(w, r, chi.URLParam(r, "runtimeId"))
	if !ok {
		return
	}
	req, err := h.WorklaneRequestStore.Get(ctx, chi.URLParam(r, "requestId"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load request")
		return
	}
	if req == nil || req.RuntimeID != uuidToString(rt.ID) {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}
	if req.Status != worklaneRequestRunning {
		writeError(w, http.StatusConflict, "request is not running")
		return
	}
	var body struct {
		Steps   []WorklaneRequestStep  `json:"steps"`
		Path    string                 `json:"path"`
		Outcome string                 `json:"outcome"`
		Branch  string                 `json:"branch"`
		Reason  *WorklaneRequestReason `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Path != "" {
		req.Path = body.Path
	}
	switch req.Kind {
	case protocol.WorklaneRequestKindCreate:
		h.applyWorklaneCreateProgress(ctx, req, body.Steps, body.Reason)
	case protocol.WorklaneRequestKindRemove:
		h.finishWorklaneRemove(ctx, req, body.Steps, body.Outcome, body.Reason)
	case protocol.WorklaneRequestKindSwitch:
		if err := h.finishWorklaneSwitch(ctx, req, body.Steps, body.Outcome, body.Branch, body.Reason); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to record worklane switch")
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "unknown request kind")
		return
	}
	req.UpdatedAt = time.Now()
	if err := h.WorklaneRequestStore.Save(ctx, req); err != nil {
		slog.Error("worklane request progress save failed", "error", err, "request_id", req.ID)
		writeError(w, http.StatusInternalServerError, "failed to save progress")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// applyWorklaneCreateProgress takes the daemon's steps (open stays the
// server's) and runs open once install is done or skipped.
func (h *Handler) applyWorklaneCreateProgress(ctx context.Context, req *WorklaneRequest, steps []WorklaneRequestStep, reason *WorklaneRequestReason) {
	for _, s := range steps {
		if s.Key == worklaneStepOpen {
			continue
		}
		if existing := req.step(s.Key); existing != nil {
			*existing = s
		}
	}
	if req.failedStep() != "" {
		req.Status = worklaneRequestFailed
		req.Reason = reason
		return
	}
	if install := req.step(worklaneStepInstall); install != nil &&
		(install.State == worklaneStepDone || install.State == worklaneStepSkipped) {
		h.openWorklane(ctx, req)
	}
}

// finishWorklaneRemove ends a remove request: removed deletes the worklane
// and its activity in one transaction; a refusal keeps it with the reason.
func (h *Handler) finishWorklaneRemove(ctx context.Context, req *WorklaneRequest, steps []WorklaneRequestStep, outcome string, reason *WorklaneRequestReason) {
	if steps != nil {
		req.Steps = steps
	}
	switch outcome {
	case worklaneOutcomeRemoved:
		if err := h.deleteRemovedWorklane(ctx, req); err != nil {
			slog.Error("worklane delete after removal failed", "error", err, "request_id", req.ID)
			req.Status = worklaneRequestFailed
			req.Reason = &WorklaneRequestReason{Code: "unknown", Detail: "the worktree is removed, but the worklane could not be deleted"}
			return
		}
		req.Status = worklaneRequestCompleted
		req.Reason = nil
	case worklaneOutcomeRefused:
		req.Status = worklaneRequestRefused
		req.Reason = reason
	case worklaneOutcomeFailed:
		req.Status = worklaneRequestFailed
		req.Reason = reason
	}
}

func (h *Handler) deleteRemovedWorklane(ctx context.Context, req *WorklaneRequest) error {
	if req.WorklaneID == nil {
		return errors.New("remove request without worklane")
	}
	wsUUID := parseUUID(req.WorkspaceID)
	wlUUID := parseUUID(*req.WorklaneID)
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := h.Queries.WithTx(tx)
	if err := qtx.DeleteActivitiesByWorklane(ctx, db.DeleteActivitiesByWorklaneParams{WorklaneID: wlUUID, WorkspaceID: wsUUID}); err != nil {
		return err
	}
	if err := qtx.DeleteWorklane(ctx, db.DeleteWorklaneParams{ID: wlUUID, WorkspaceID: wsUUID}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	actorID := ""
	if req.CreatedBy != nil {
		actorID = *req.CreatedBy
	}
	h.publish(protocol.EventWorklaneDeleted, req.WorkspaceID, "member", actorID, protocol.WorklaneEventPayload{
		WorklaneID: *req.WorklaneID, RepositoryID: req.RepositoryID,
	})
	return nil
}
