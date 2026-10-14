package handler

// Release activity actions (ADR-0008, ADR-0011). Each release line write adds
// one activity_log row with repository_id set and worklane_id and issue_id
// NULL. Details carry {release_id, version, title}, plus changes on edit,
// move and remove, and direction on move.
const (
	releaseActionPlanned = "release_planned"
	releaseActionEdited  = "release_edited"
	releaseActionMoved   = "release_moved"
	releaseActionRemoved = "release_removed"
)
