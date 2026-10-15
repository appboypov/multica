package question

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func policyID(n byte) pgtype.UUID { return pgtype.UUID{Bytes: [16]byte{n}, Valid: true} }

func TestDeriveStatus(t *testing.T) {
	a, b, c := policyID(1), policyID(2), policyID(3)
	for _, tc := range []struct {
		name, policy       string
		answerers, answers []pgtype.UUID
		want               string
	}{
		{"any none", PolicyAnyOne, []pgtype.UUID{a, b}, nil, "open"},
		{"any one", PolicyAnyOne, []pgtype.UUID{a, b}, []pgtype.UUID{a}, "answered"},
		{"any two", PolicyAnyOne, []pgtype.UUID{a, b}, []pgtype.UUID{a, b}, "answered"},
		{"each none", PolicyEach, []pgtype.UUID{a, b}, nil, "open"},
		{"each some", PolicyEach, []pgtype.UUID{a, b}, []pgtype.UUID{a}, "partly_answered"},
		{"each all", PolicyEach, []pgtype.UUID{a, b}, []pgtype.UUID{a, b}, "answered"},
		{"former answerer only", PolicyEach, []pgtype.UUID{b, c}, []pgtype.UUID{a}, "open"},
		{"former answerer not counted", PolicyEach, []pgtype.UUID{b, c}, []pgtype.UUID{a, b}, "partly_answered"},
		{"any former answerer", PolicyAnyOne, []pgtype.UUID{b, c}, []pgtype.UUID{a}, "open"},
		{"one each", PolicyEach, []pgtype.UUID{a}, []pgtype.UUID{a}, "answered"},
		{"duplicate answers not counted twice", PolicyEach, []pgtype.UUID{a, b}, []pgtype.UUID{a, a}, "partly_answered"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := DeriveStatus(tc.policy, tc.answerers, tc.answers); got != tc.want {
				t.Fatalf("status = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestIsOpenAndOpenFor(t *testing.T) {
	a, b, outsider := policyID(1), policyID(2), policyID(3)
	for _, status := range []string{"open", "partly_answered", "answered", "closed", "withdrawn", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			wantOpen := status == "open" || status == "partly_answered"
			if IsOpen(status) != wantOpen {
				t.Fatalf("IsOpen(%s) = %v", status, IsOpen(status))
			}
			state := PolicyQuestion{Status: status, AnswererIDs: []pgtype.UUID{a, b}, Answers: []pgtype.UUID{a}}
			for _, tc := range []struct {
				id   pgtype.UUID
				want bool
			}{{a, false}, {b, wantOpen}, {outsider, false}} {
				if got := IsOpenFor(state, tc.id); got != tc.want {
					t.Fatalf("open for %v = %v, want %v", tc.id, got, tc.want)
				}
			}
		})
	}
}
