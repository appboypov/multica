package question

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Display returns the same one-line answer text for cards, inbox items and run
// input. Notes remain separate. Person and attachment names require service
// lookups; their immutable IDs are retained here rather than inventing names.
func Display(answerType string, settings, value json.RawMessage) string {
	def, ok := answerTypes[answerType]
	if !ok {
		return ""
	}
	return def.display(settings, value)
}

func oneLine(text string) string {
	if !strings.ContainsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' || r == '\t' || r == '\u2028' || r == '\u2029' }) {
		return strings.TrimSpace(text)
	}
	return strings.Join(strings.FieldsFunc(text, unicode.IsSpace), " ")
}

func displayText(_ TextAnswerSettings, v TextAnswerValue) string { return v.Text }

func displayYesNo(_ YesNoAnswerSettings, v YesNoAnswerValue) string {
	switch v.Choice {
	case "yes":
		return "Yes"
	case "no":
		return "No"
	default:
		return ""
	}
}

func displaySingleChoice(_ SingleChoiceAnswerSettings, v SingleChoiceAnswerValue) string {
	if v.Option != nil && v.Other == nil {
		return *v.Option
	}
	if v.Other != nil && v.Option == nil {
		return "Other: " + *v.Other
	}
	return ""
}

func displayMultipleChoice(_ MultipleChoiceAnswerSettings, v MultipleChoiceAnswerValue) string {
	return strings.Join(v.Options, ", ")
}

func displayFacts(s ConfirmFactsAnswerSettings, v ConfirmFactsAnswerValue) string {
	if len(s.Facts) != len(v.Facts) {
		return ""
	}
	parts := make([]string, len(v.Facts))
	for i, fact := range v.Facts {
		if fact.Correct == nil {
			return ""
		}
		if *fact.Correct {
			parts[i] = s.Facts[i] + ": Correct"
		} else {
			parts[i] = s.Facts[i] + ": Not correct - " + fact.Correction
		}
	}
	return strings.Join(parts, "; ")
}

func displayRanking(_ RankingAnswerSettings, v RankingAnswerValue) string {
	return strings.Join(v.Order, " → ")
}

func displayScale(_ ScaleAnswerSettings, v ScaleAnswerValue) string {
	return strconv.Itoa(v.Point)
}

func displayNumber(s NumberAnswerSettings, v NumberAnswerValue) string {
	text := strconv.FormatFloat(v.Number, 'f', -1, 64)
	if s.Unit != "" {
		text += " " + s.Unit
	}
	return text
}

func displayDate(s DateAnswerSettings, v DateAnswerValue) string {
	switch s.Kind {
	case "date":
		if v.Date != nil {
			return *v.Date
		}
	case "datetime":
		if v.Datetime != nil {
			return *v.Datetime
		}
	case "range":
		if v.Start != nil && v.End != nil {
			return *v.Start + " – " + *v.End
		}
	}
	return ""
}

func displayPerson(_ PersonAnswerSettings, v PersonAnswerValue) string {
	return v.Type + ":" + v.ID
}

func displayFile(_ FileAnswerSettings, v FileAnswerValue) string {
	return fmt.Sprintf("%d file(s): %s", len(v.AttachmentIDs), strings.Join(v.AttachmentIDs, ", "))
}

func displayApproval(_ ApprovalAnswerSettings, v ApprovalAnswerValue) string {
	switch v.Decision {
	case "approve":
		return "Approve"
	case "request_changes":
		return "Request changes"
	case "decline":
		return "Decline"
	default:
		return ""
	}
}
