// Package worklanehead computes a worklane's state, its active issue and its
// next issue from the branch its worktree is on and its open issues. Nothing
// here is stored: every worklane response computes it again (UJ-6 design.md
// decision 2), so the worklane page and the Worklanes rows read one rule.
package worklanehead

import (
	"sort"
	"strings"
	"time"
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
	CreatedAt       time.Time
}

// Result is a worklane's head: Active is set only while Busy, Next only
// while Free with an open issue left.
type Result struct {
	State  string
	Active *Issue
	Next   *Issue
}

// Head returns the worklane's state with its active or next issue. branch is
// the branch the worktree is on, empty while detached. The active issue is
// the open issue whose lowercased identifier is the branch and whose status
// category is started, except blocked. Without one the worklane is Free and
// its next issue is the first open issue in release order: issues in a
// release by the release's position, then issues in no release, each by
// created_at, then id.
func Head(branch string, issues []Issue) Result {
	if branch != "" {
		for i := range issues {
			issue := &issues[i]
			if strings.ToLower(issue.Identifier) == branch && isWorking(issue) {
				active := *issue
				return Result{State: StateBusy, Active: &active}
			}
		}
	}
	if len(issues) == 0 {
		return Result{State: StateFree}
	}
	ordered := make([]*Issue, len(issues))
	for i := range issues {
		ordered[i] = &issues[i]
	}
	sort.SliceStable(ordered, func(a, b int) bool { return before(ordered[a], ordered[b]) })
	next := *ordered[0]
	return Result{State: StateFree, Next: &next}
}

// Without returns issues without the issue of the given id, as the switch
// after a merge skips the merged issue.
func Without(issues []Issue, id string) []Issue {
	out := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		if issue.ID != id {
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
	if !a.CreatedAt.Equal(b.CreatedAt) {
		return a.CreatedAt.Before(b.CreatedAt)
	}
	return a.ID < b.ID
}
