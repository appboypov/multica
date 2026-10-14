package main

import (
	"strings"
	"testing"
)

// validateProjectStatus must accept the five DB-backed statuses and reject
// anything else with a message that lists the valid values. `project create`,
// `project update`, and `project status` all share it (#3925: `--status active`
// used to reach the server and 500 on the CHECK constraint).
func TestValidateProjectStatus(t *testing.T) {
	for _, s := range validProjectStatuses {
		if err := validateProjectStatus(s); err != nil {
			t.Errorf("status %q should be valid, got: %v", s, err)
		}
	}
	err := validateProjectStatus("active")
	if err == nil {
		t.Fatal("status \"active\" should be rejected")
	}
	if !strings.Contains(err.Error(), "planned") {
		t.Errorf("error should list valid statuses, got: %v", err)
	}
}
