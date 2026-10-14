package daemon

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Worklane requests (worklanes-of-a-repository design.md decisions 3 and 4).
// The daemon runs a create from its from_step and posts its whole step list
// after every step change; open is the server's step and never runs here. A
// remove answers one outcome: removed, refused with a reason, or failed.

const (
	worklaneStepFetch       = "fetch"
	worklaneStepAddWorktree = "add_worktree"
	worklaneStepInstall     = "install"

	worklaneStepPending = "pending"
	worklaneStepRunning = "running"
	worklaneStepDone    = "done"
	worklaneStepSkipped = "skipped"
	worklaneStepFailed  = "failed"

	worklaneOutcomeRemoved = "removed"
	worklaneOutcomeRefused = "refused"
	worklaneOutcomeFailed  = "failed"

	worklaneCodeUnknown = "unknown"
)

var worklaneDaemonSteps = []string{worklaneStepFetch, worklaneStepAddWorktree, worklaneStepInstall}

type worklaneStep struct {
	Key        string `json:"key"`
	State      string `json:"state"`
	Detail     string `json:"detail"`
	Output     string `json:"output"`
	DurationMs int64  `json:"duration_ms"`
}

type worklaneReason struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
	Count  *int   `json:"count,omitempty"`
}

func worklaneReasonOf(err error) *worklaneReason {
	var we *execenv.WorklaneError
	if errors.As(err, &we) {
		reason := &worklaneReason{Code: we.Code, Detail: we.Detail}
		if we.Count > 0 {
			count := we.Count
			reason.Count = &count
		}
		return reason
	}
	return &worklaneReason{Code: worklaneCodeUnknown, Detail: err.Error()}
}

func (d *Daemon) handleWorklaneRequest(ctx context.Context, rt Runtime, req PendingWorklaneRequest) {
	d.logger.Info("worklane request received", "runtime_id", rt.ID, "request_id", req.ID, "kind", req.Kind)
	switch req.Kind {
	case protocol.WorklaneRequestKindCreate:
		d.runWorklaneCreate(ctx, rt, req)
	case protocol.WorklaneRequestKindRemove:
		d.runWorklaneRemove(ctx, rt, req)
	default:
		d.reportWorklaneProgress(ctx, rt, req.ID, map[string]any{
			"outcome": worklaneOutcomeFailed,
			"reason":  worklaneReason{Code: worklaneCodeUnknown, Detail: fmt.Sprintf("this Multica does not know worklane request kind %q", req.Kind)},
		})
	}
}

func (d *Daemon) reportWorklaneProgress(ctx context.Context, rt Runtime, requestID string, progress map[string]any) {
	d.reportRuntimeResultWithRetry(ctx, "worklane_request", rt.ID, requestID, func(ctx context.Context) error {
		return d.client.ReportWorklaneRequestProgress(ctx, rt.ID, requestID, progress)
	})
}

// worklaneCreateRun is one create request's step list from its from_step on.
// Steps before from_step are done on the server and are not sent again.
type worklaneCreateRun struct {
	d     *Daemon
	ctx   context.Context
	rt    Runtime
	req   PendingWorklaneRequest
	steps []worklaneStep
	path  string
}

func (r *worklaneCreateRun) step(key string) *worklaneStep {
	for i := range r.steps {
		if r.steps[i].Key == key {
			return &r.steps[i]
		}
	}
	return nil
}

func (r *worklaneCreateRun) post(reason *worklaneReason) {
	progress := map[string]any{"steps": r.steps}
	if r.path != "" {
		progress["path"] = r.path
	}
	if reason != nil {
		progress["reason"] = reason
	}
	r.d.reportWorklaneProgress(r.ctx, r.rt, r.req.ID, progress)
}

func (r *worklaneCreateRun) start(key string) (*worklaneStep, time.Time) {
	s := r.step(key)
	s.State = worklaneStepRunning
	s.Detail, s.Output, s.DurationMs = "", "", 0
	r.post(nil)
	return s, time.Now()
}

func (r *worklaneCreateRun) fail(s *worklaneStep, started time.Time, err error) {
	s.State = worklaneStepFailed
	s.DurationMs = time.Since(started).Milliseconds()
	reason := worklaneReasonOf(err)
	s.Detail = reason.Detail
	var we *execenv.WorklaneError
	if errors.As(err, &we) {
		s.Output = we.Output
	}
	r.post(reason)
	r.d.logger.Warn("worklane create step failed", "request_id", r.req.ID, "step", s.Key, "code", reason.Code, "error", err)
}

func (d *Daemon) runWorklaneCreate(ctx context.Context, rt Runtime, req PendingWorklaneRequest) {
	from := req.FromStep
	if from == "" {
		from = worklaneStepFetch
	}
	run := &worklaneCreateRun{d: d, ctx: ctx, rt: rt, req: req}
	started := false
	for _, key := range worklaneDaemonSteps {
		if key == from {
			started = true
		}
		if started {
			run.steps = append(run.steps, worklaneStep{Key: key, State: worklaneStepPending})
		}
	}
	if len(run.steps) == 0 {
		// from_step is the server's open step: nothing runs here.
		return
	}
	if path, err := execenv.ExpandWorklanePath(req.Path); err == nil {
		run.path = path
	}

	var commit string
	if s := run.step(worklaneStepFetch); s != nil {
		_, began := run.start(worklaneStepFetch)
		fetched, took, err := execenv.FetchDefaultBranch(ctx, req.CheckoutPath, req.DefaultBranch)
		if err != nil {
			run.fail(s, began, err)
			return
		}
		commit = fetched
		s.State = worklaneStepDone
		s.DurationMs = took.Milliseconds()
		s.Detail = fmt.Sprintf("Fetched origin/%s in %s", req.DefaultBranch, took.Round(100*time.Millisecond))
		run.post(nil)
	}

	if s := run.step(worklaneStepAddWorktree); s != nil {
		_, began := run.start(worklaneStepAddWorktree)
		if commit == "" {
			resolved, err := execenv.ResolveOriginDefaultBranch(req.CheckoutPath, req.DefaultBranch)
			if err != nil {
				run.fail(s, began, err)
				return
			}
			commit = resolved
		}
		issueWorktrees := make([]execenv.WorklaneIssueWorktree, 0, len(req.IssueWorktreePaths))
		for _, iw := range req.IssueWorktreePaths {
			issueWorktrees = append(issueWorktrees, execenv.WorklaneIssueWorktree{Path: iw.Path, IssueKey: iw.IssueKey})
		}
		result, err := execenv.AddWorklaneWorktree(execenv.AddWorklaneWorktreeParams{
			CheckoutPath:       req.CheckoutPath,
			FullName:           req.FullName,
			Path:               req.Path,
			Commit:             commit,
			IssueWorktreePaths: issueWorktrees,
		}, d.logger)
		if err != nil {
			run.fail(s, began, err)
			return
		}
		run.path = result.Path
		s.State = worklaneStepDone
		s.DurationMs = time.Since(began).Milliseconds()
		if result.Reused {
			s.Detail = "Uses the worktree that is there"
		} else {
			s.Detail = fmt.Sprintf("Added %s at origin/%s", result.Path, req.DefaultBranch)
		}
		run.post(nil)
	}

	if s := run.step(worklaneStepInstall); s != nil {
		if req.InstallCommand == "" {
			s.State = worklaneStepSkipped
			s.Detail = "No install command"
			run.post(nil)
			return
		}
		_, began := run.start(worklaneStepInstall)
		if run.path == "" {
			run.fail(s, began, fmt.Errorf("the worktree path %q does not resolve", req.Path))
			return
		}
		realPath, _ := resolveRealPath(run.path)
		release, err := d.localPathLocks.Acquire(ctx, realPath, "worklane:"+req.ID, nil)
		if err != nil {
			run.fail(s, began, err)
			return
		}
		output, err := execenv.RunInstall(ctx, run.path, req.InstallCommand)
		release()
		if err != nil {
			run.fail(s, began, err)
			return
		}
		s.State = worklaneStepDone
		s.DurationMs = time.Since(began).Milliseconds()
		s.Detail = req.InstallCommand
		s.Output = output
		run.post(nil)
	}
}

func (d *Daemon) runWorklaneRemove(ctx context.Context, rt Runtime, req PendingWorklaneRequest) {
	answer := func(outcome string, reason *worklaneReason) {
		progress := map[string]any{"outcome": outcome}
		if reason != nil {
			progress["reason"] = reason
		}
		d.reportWorklaneProgress(ctx, rt, req.ID, progress)
	}
	path, err := execenv.ExpandWorklanePath(req.Path)
	if err != nil {
		answer(worklaneOutcomeFailed, worklaneReasonOf(err))
		return
	}
	realPath, _ := resolveRealPath(path)
	release, ok := d.localPathLocks.TryAcquire(realPath, "worklane:"+req.ID)
	if !ok {
		answer(worklaneOutcomeRefused, &worklaneReason{Code: execenv.WorklaneCodeBusy, Detail: "a task is working in the worktree"})
		return
	}
	err = execenv.RemoveWorklaneWorktree(ctx, execenv.RemoveWorklaneWorktreeParams{
		CheckoutPath:  req.CheckoutPath,
		Path:          path,
		DefaultBranch: req.DefaultBranch,
	}, d.logger)
	release()
	if err == nil {
		d.logger.Info("worklane worktree removed", "request_id", req.ID, "path", path)
		answer(worklaneOutcomeRemoved, nil)
		return
	}
	reason := worklaneReasonOf(err)
	switch reason.Code {
	case execenv.WorklaneCodeBusy, execenv.WorklaneCodeUncommitted, execenv.WorklaneCodeUnpushed, execenv.WorklaneCodeStashed:
		answer(worklaneOutcomeRefused, reason)
	default:
		d.logger.Warn("worklane worktree remove failed", "request_id", req.ID, "path", path, "error", err)
		answer(worklaneOutcomeFailed, reason)
	}
}
