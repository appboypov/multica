// Package issueplacement holds the rules that place an issue in a worklane
// and a release (ADR-0013). It is pure: the caller looks up the worklane and
// the release under the repository lock, and writes what Resolve returns in
// the same transaction. IDs are canonical UUID strings, "" meaning none.
package issueplacement

// Activity actions of a placement write.
const (
	ActionPlannedInRelease   = "planned_in_release"
	ActionAddedToRelease     = "added_to_release"
	ActionRemovedFromRelease = "removed_from_release"
	ActionWorklaneChanged    = "worklane_changed"
)

// Refusal codes of a placement write.
const (
	CodeReleaseNeedsWorklane   = "release_needs_worklane"
	CodeReleaseNotOfRepository = "release_not_of_repository"
	CodeReleaseShipped         = "release_shipped"
)

// Placement is an issue's worklane and release.
type Placement struct {
	WorklaneID string
	ReleaseID  string
}

// Worklane is the looked-up worklane of a placement.
type Worklane struct {
	ID           string
	RepositoryID string
	ProjectID    string
	Name         string
}

// Release is the looked-up release of a placement.
type Release struct {
	ID             string
	RepositoryID   string
	RepositoryName string
	Version        string
	Title          string
	Shipped        bool
}

// Input is everything Resolve decides from.
type Input struct {
	// Creating is true for a new issue: a release then writes one
	// planned_in_release entry and nothing else.
	Creating bool
	// Current is the issue's placement before the write.
	Current Placement
	// CurrentProjectID is the issue's project before the write.
	CurrentProjectID string
	// CurrentRelease is the issue's release before the write, nil when none.
	CurrentRelease *Release
	// Want is the placement the caller asks for.
	Want Placement
	// Worklane is the wanted worklane, nil when Want has none.
	Worklane *Worklane
	// Release is the wanted release, nil when Want has none.
	Release *Release
}

// Entry is one activity_log row the write adds to the issue.
type Entry struct {
	Action  string
	Details map[string]any
}

// Result is the placement to write, the issue's project and its entries.
type Result struct {
	Placement Placement
	ProjectID string
	Entries   []Entry
	// Changed is false when the write would change nothing: the caller then
	// answers with the issue as it is and writes nothing.
	Changed bool
}

// Refusal is a placement the rules refuse.
type Refusal struct {
	Code       string
	Version    string
	Title      string
	Repository string
}

// Resolve applies the placement rules:
//   - No worklane: the release must be none and the project stays.
//   - A worklane: the project becomes its project, and a release must be of
//     the worklane's repository and, when it is new to the issue, unshipped.
//     Taking an issue out of a shipped release is allowed.
func Resolve(in Input) (Result, *Refusal) {
	want := in.Want
	if want.WorklaneID == "" {
		in.Worklane = nil
	}
	if want.ReleaseID == "" {
		in.Release = nil
	}
	if want.ReleaseID != "" && want.WorklaneID == "" {
		return Result{}, &Refusal{Code: CodeReleaseNeedsWorklane}
	}
	if in.Release != nil {
		if in.Worklane == nil || in.Release.RepositoryID != in.Worklane.RepositoryID {
			return Result{}, &Refusal{Code: CodeReleaseNotOfRepository, Version: in.Release.Version, Repository: in.Release.RepositoryName}
		}
		if in.Release.Shipped && (in.Creating || want.ReleaseID != in.Current.ReleaseID) {
			return Result{}, &Refusal{Code: CodeReleaseShipped, Version: in.Release.Version, Title: in.Release.Title}
		}
	}

	projectID := in.CurrentProjectID
	if in.Worklane != nil {
		projectID = in.Worklane.ProjectID
	}
	result := Result{Placement: want, ProjectID: projectID}

	if in.Creating {
		result.Changed = true
		if in.Release != nil {
			result.Entries = append(result.Entries, Entry{Action: ActionPlannedInRelease, Details: map[string]any{
				"release_id":    in.Release.ID,
				"version":       in.Release.Version,
				"title":         in.Release.Title,
				"worklane_id":   in.Worklane.ID,
				"worklane_name": in.Worklane.Name,
			}})
		}
		return result, nil
	}

	if want == in.Current && projectID == in.CurrentProjectID {
		return result, nil
	}
	result.Changed = true

	if want.WorklaneID != in.Current.WorklaneID {
		details := map[string]any{"worklane_id": nil, "worklane_name": nil}
		if in.Worklane != nil {
			details["worklane_id"] = in.Worklane.ID
			details["worklane_name"] = in.Worklane.Name
		}
		result.Entries = append(result.Entries, Entry{Action: ActionWorklaneChanged, Details: details})
	}
	if want.ReleaseID != in.Current.ReleaseID {
		if in.Current.ReleaseID != "" {
			result.Entries = append(result.Entries, Entry{Action: ActionRemovedFromRelease, Details: releaseDetails(in.Current.ReleaseID, in.CurrentRelease)})
		}
		if in.Release != nil {
			result.Entries = append(result.Entries, Entry{Action: ActionAddedToRelease, Details: releaseDetails(in.Release.ID, in.Release)})
		}
	}
	return result, nil
}

// ReleaseDetails are the details of an entry that names a release, kept as
// the release was at the write.
func ReleaseDetails(r Release) map[string]any {
	return releaseDetails(r.ID, &r)
}

func releaseDetails(id string, r *Release) map[string]any {
	details := map[string]any{"release_id": id, "version": "", "title": ""}
	if r != nil {
		details["version"] = r.Version
		details["title"] = r.Title
	}
	return details
}
