package worklanehead

import (
	"testing"
	"time"
)

func TestHeadReleaseThenLaneOrder(t *testing.T) {
	one, two := int32(1), int32(2)
	first, last := 1.0, 20.0
	old := time.Unix(1, 0)
	newer := time.Unix(2, 0)
	cases := []struct {
		name   string
		issues []Issue
		want   string
	}{
		{"release first", []Issue{{ID: "a", ReleasePosition: &two, LanePosition: &first}, {ID: "b", ReleasePosition: &one, LanePosition: &last}}, "b"},
		{"lane beats creation", []Issue{{ID: "a", CreatedAt: old, LanePosition: &last}, {ID: "b", CreatedAt: newer, LanePosition: &first}}, "b"},
		{"lane nulls last", []Issue{{ID: "a", CreatedAt: old}, {ID: "b", CreatedAt: newer, LanePosition: &last}}, "b"},
		{"creation tie breaker", []Issue{{ID: "a", CreatedAt: newer, LanePosition: &first}, {ID: "b", CreatedAt: old, LanePosition: &first}}, "b"},
		{"id tie breaker", []Issue{{ID: "b", CreatedAt: old}, {ID: "a", CreatedAt: old}}, "a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := Head("", tc.issues)
			if result.Next == nil || result.Next.ID != tc.want {
				t.Fatalf("head = %+v, want %s", result.Next, tc.want)
			}
		})
	}
}

func TestHeadKeepsActiveBranchDespiteReorder(t *testing.T) {
	first, last := 1.0, 2.0
	issues := []Issue{{ID: "next", Identifier: "MUL-1", Status: "todo", LanePosition: &first}, {ID: "active", Identifier: "MUL-2", Status: "in_progress", StatusCategory: "started", LanePosition: &last}}
	result := Head("mul-2", issues)
	if result.State != StateBusy || result.Active == nil || result.Active.ID != "active" {
		t.Fatalf("active head = %+v", result)
	}
}
