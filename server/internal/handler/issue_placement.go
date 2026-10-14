package handler

// Issue placement activity actions preserve the names and versions at the write.
const (
	issueActionPlannedInRelease   = "planned_in_release"
	issueActionTypeSet            = "type_set"
	issueActionAddedToRelease     = "added_to_release"
	issueActionRemovedFromRelease = "removed_from_release"
	issueActionWorklaneChanged    = "worklane_changed"
	issueActionReleaseGone        = "release_gone"
)

// Refusal codes of the type and placement endpoints (ADR-0013).
const (
	issueCodeTypeInvalid            = "type_invalid"
	issueCodeTypeFixed              = "type_fixed"
	issueCodeReleaseNeedsWorklane   = "release_needs_worklane"
	issueCodeReleaseNotOfRepository = "release_not_of_repository"
	issueCodeReleaseShipped         = "release_shipped"
	issueCodeNotInRepository        = "issue_not_in_repository"
	issueCodeInRelease              = "issue_in_release"
)
