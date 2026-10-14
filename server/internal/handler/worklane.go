package handler

// Worklane activity actions (ADR-0008). Entries are activity_log rows with
// worklane_id set and issue_id NULL; details carry {name, runtime_name} on
// worklane_created and {from, to} on worklane_renamed. Issue actions stay
// string literals in their listeners.
const (
	worklaneActionCreated = "worklane_created"
	worklaneActionRenamed = "worklane_renamed"
)
