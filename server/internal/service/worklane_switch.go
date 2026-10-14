package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/worklanehead"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// WorklaneSwitch asks the worklane's daemon to move its worktree from the
// merged branch to the next issue's branch, or to detached at
// origin/{default_branch} when ToBranch is empty (UJ-6 design.md decision 7).
// For main, Path and DaemonID are the repository's checkout.
type WorklaneSwitch struct {
	Worklane      db.Worklane
	Repository    db.Repository
	DaemonID      string
	Path          string
	FromBranch    string
	ToBranch      string
	DefaultBranch string
}

// SwitchAfterMerge runs after a pull request of fullName merged baseRef from
// headRef. For each worklane of a linked issue whose repository is fullName,
// whose default branch is baseRef and whose recorded branch is headRef, it
// computes the next issue with worklanehead.Head, skipping the linked issues,
// and hands one switch per worklane to enqueue.
func SwitchAfterMerge(
	ctx context.Context,
	q *db.Queries,
	workspaceID pgtype.UUID,
	issueIDs []pgtype.UUID,
	fullName, baseRef, headRef string,
	enqueue func(context.Context, WorklaneSwitch) error,
) error {
	if headRef == "" || baseRef == "" || len(issueIDs) == 0 {
		return nil
	}
	merged := make(map[string]bool, len(issueIDs))
	for _, id := range issueIDs {
		merged[util.UUIDToString(id)] = true
	}
	seen := make(map[string]bool)
	var errs []error
	for _, issueID := range issueIDs {
		issue, err := q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: issueID, WorkspaceID: workspaceID})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("load issue %s: %w", util.UUIDToString(issueID), err))
			continue
		}
		if !issue.WorklaneID.Valid {
			continue
		}
		worklaneID := util.UUIDToString(issue.WorklaneID)
		if seen[worklaneID] {
			continue
		}
		seen[worklaneID] = true
		sw, ok, err := worklaneSwitchFor(ctx, q, workspaceID, issue.WorklaneID, fullName, baseRef, headRef, merged)
		if err != nil {
			errs = append(errs, fmt.Errorf("worklane %s: %w", worklaneID, err))
			continue
		}
		if !ok {
			continue
		}
		if err := enqueue(ctx, sw); err != nil {
			errs = append(errs, fmt.Errorf("enqueue switch of worklane %s: %w", worklaneID, err))
		}
	}
	return errors.Join(errs...)
}

func worklaneSwitchFor(
	ctx context.Context,
	q *db.Queries,
	workspaceID, worklaneID pgtype.UUID,
	fullName, baseRef, headRef string,
	merged map[string]bool,
) (WorklaneSwitch, bool, error) {
	worklane, err := q.GetWorklaneInWorkspace(ctx, db.GetWorklaneInWorkspaceParams{ID: worklaneID, WorkspaceID: workspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return WorklaneSwitch{}, false, nil
	}
	if err != nil {
		return WorklaneSwitch{}, false, err
	}
	if !worklane.Branch.Valid || worklane.Branch.String != headRef {
		return WorklaneSwitch{}, false, nil
	}
	repo, err := q.GetRepositoryInWorkspace(ctx, db.GetRepositoryInWorkspaceParams{ID: worklane.RepositoryID, WorkspaceID: workspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return WorklaneSwitch{}, false, nil
	}
	if err != nil {
		return WorklaneSwitch{}, false, err
	}
	if !strings.EqualFold(repo.FullName, fullName) || repo.DefaultBranch != baseRef {
		return WorklaneSwitch{}, false, nil
	}
	sw := WorklaneSwitch{
		Worklane:      worklane,
		Repository:    repo,
		DaemonID:      worklane.DaemonID.String,
		Path:          worklane.Path.String,
		FromBranch:    headRef,
		DefaultBranch: repo.DefaultBranch,
	}
	if worklane.IsMain {
		sw.DaemonID = repo.CheckoutDaemonID
		sw.Path = repo.CheckoutPath
	}
	if sw.DaemonID == "" || sw.Path == "" {
		return WorklaneSwitch{}, false, nil
	}
	rows, err := q.ListOpenWorklaneIssues(ctx, db.ListOpenWorklaneIssuesParams{
		WorkspaceID: workspaceID,
		WorklaneIds: []pgtype.UUID{worklaneID},
	})
	if err != nil {
		return WorklaneSwitch{}, false, err
	}
	open := worklanehead.Without(worklanehead.FromRows(rows)[util.UUIDToString(worklaneID)], merged)
	if head := worklanehead.Head("", open); head.Next != nil {
		sw.ToBranch = worklanehead.Branch(head.Next.Identifier)
	}
	return sw, true, nil
}
