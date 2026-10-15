// Package worklanehead computes a worklane's state, its active issue and its
// next issue from the branch its worktree is on and its open issues. Nothing
// here is stored: every worklane response computes it again (UJ-6 design.md
// decision 2), so the worklane page and the Worklanes rows read one rule.
package worklanehead

import (
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// States of a worklane.
const (
	StateBusy = "busy"
	StateFree = "free"
)

// Issue is one open issue of a worklane. ReleasePosition is nil for an issue
// in no release. StatusCategory is the lifecycle category of its status, with
// the built-ins by their fixed category.
type Issue struct {
	ID              string
	Identifier      string
	Title           string
	Status          string
	StatusCategory  string
	ReleasePosition *int32
	LanePosition    *float64
	CreatedAt       time.Time
}

// Result is a worklane's head: Active is set only while Busy, Next only
// while Free with an open issue left.
type Result struct {
	State  string
	Active *Issue
	Next   *Issue
}

// FromRows groups the rows of ListOpenWorklaneIssues by worklane id, in the
// query's order.
func FromRows(rows []db.ListOpenWorklaneIssuesRow) map[string][]Issue {
	byWorklane := make(map[string][]Issue)
	for _, row := range rows {
		issue := Issue{
			ID:             util.UUIDToString(row.ID),
			Identifier:     row.Identifier,
			Title:          row.Title,
			Status:         row.Status,
			StatusCategory: row.StatusCategory,
			CreatedAt:      row.CreatedAt.Time,
		}
		if row.ReleasePosition.Valid {
			position := row.ReleasePosition.Int32
			issue.ReleasePosition = &position
		}
		if row.LanePosition.Valid {
			position := row.LanePosition.Float64
			issue.LanePosition = &position
		}
		worklaneID := util.UUIDToString(row.WorklaneID)
		byWorklane[worklaneID] = append(byWorklane[worklaneID], issue)
	}
	return byWorklane
}

// Branch is the branch a worklane works an issue on: its identifier in
// lowercase, as UJ-2 names an issue's branch.
func Branch(identifier string) string {
	return strings.ToLower(identifier)
}

// Head returns the worklane's state with its active or next issue. branch is
// the branch the worktree is on, empty while detached. The active issue is
// the open issue whose branch is the worktree's and whose status category is
// started, except blocked. Without one the worklane is Free and its next
// issue is the first open issue in release order: issues in a release by the
// release's position, then lane position, then created_at and id, with nulls last.
func Head(branch string, issues []Issue) Result {
	if branch != "" {
		for i := range issues {
			issue := &issues[i]
			if Branch(issue.Identifier) == branch && isWorking(issue) {
				return Result{State: StateBusy, Active: issue}
			}
		}
	}
	if len(issues) == 0 {
		return Result{State: StateFree}
	}
	next := &issues[0]
	for i := 1; i < len(issues); i++ {
		if before(&issues[i], next) {
			next = &issues[i]
		}
	}
	return Result{State: StateFree, Next: next}
}

// Without returns issues without the issues of the given ids, as the switch
// after a merge skips the merged issues.
func Without(issues []Issue, ids map[string]bool) []Issue {
	out := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		if !ids[issue.ID] {
			out = append(out, issue)
		}
	}
	return out
}

func isWorking(issue *Issue) bool {
	return issue.StatusCategory == "started" && issue.Status != "blocked"
}

func before(a, b *Issue) bool {
	switch {
	case a.ReleasePosition != nil && b.ReleasePosition == nil:
		return true
	case a.ReleasePosition == nil && b.ReleasePosition != nil:
		return false
	case a.ReleasePosition != nil && *a.ReleasePosition != *b.ReleasePosition:
		return *a.ReleasePosition < *b.ReleasePosition
	}
	switch {
	case a.LanePosition != nil && b.LanePosition == nil:
		return true
	case a.LanePosition == nil && b.LanePosition != nil:
		return false
	case a.LanePosition != nil && *a.LanePosition != *b.LanePosition:
		return *a.LanePosition < *b.LanePosition
	}
	if !a.CreatedAt.Equal(b.CreatedAt) {
		return a.CreatedAt.Before(b.CreatedAt)
	}
	return a.ID < b.ID
}
