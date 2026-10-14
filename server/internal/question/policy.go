package question

import (
	"slices"

	"github.com/jackc/pgx/v5/pgtype"
)

const (
	PolicyAnyOne = "any_one"
	PolicyEach   = "each"
)

// DeriveStatus counts only current answerers. Callers must not recompute closed
// or withdrawn questions; those are explicit, terminal decisions by the asker.
func DeriveStatus(policy string, answererIDs, answers []pgtype.UUID) string {
	answered, total := 0, 0
	for i, id := range answererIDs {
		if slices.Contains(answererIDs[:i], id) {
			continue
		}
		total++
		if slices.Contains(answers, id) {
			answered++
		}
	}
	if answered == 0 {
		return "open"
	}
	if policy == PolicyEach && answered < total {
		return "partly_answered"
	}
	return "answered"
}

func IsOpen(status string) bool { return status == "open" || status == "partly_answered" }

// PolicyQuestion is the membership and answer state needed for an open list.
type PolicyQuestion struct {
	Status      string
	AnswererIDs []pgtype.UUID
	Answers     []pgtype.UUID
}

func IsOpenFor(question PolicyQuestion, userID pgtype.UUID) bool {
	return IsOpen(question.Status) && slices.Contains(question.AnswererIDs, userID) && !slices.Contains(question.Answers, userID)
}
