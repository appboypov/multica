package question

import (
	"encoding/json"
	"strings"
	"testing"
)

func validAskQuestions(count int) []AskQuestion {
	questions := make([]AskQuestion, count)
	for i := range questions {
		questions[i] = AskQuestion{Prompt: "Can we ship?", AnswerType: TypeYesNo, Settings: json.RawMessage(`{"why_on_no":false}`)}
	}
	return questions
}

func TestValidateAsk(t *testing.T) {
	for _, tc := range []struct {
		name, title, policy string
		count               int
		wantError           bool
	}{
		{"zero", "", "", 0, true},
		{"one default", "", "", 1, false},
		{"one each", "", PolicyEach, 1, false},
		{"twenty", "Build decisions", PolicyAnyOne, 20, false},
		{"twenty one", "Build decisions", PolicyAnyOne, 21, true},
		{"three missing title", "", "", 3, true},
		{"three blank title", "  ", "", 3, true},
		{"one with title", "Build decisions", "", 1, true},
		{"unknown policy", "", "everyone", 1, true},
		{"maximum unicode title", strings.Repeat("é", 200), "", 3, false},
		{"long title", strings.Repeat("é", 201), "", 3, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateAsk(tc.title, validAskQuestions(tc.count), tc.policy)
			if (err != nil) != tc.wantError {
				t.Fatalf("validation = %v, want error %v", err, tc.wantError)
			}
		})
	}
}

func TestValidateAskNamesRefusedQuestion(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*AskQuestion)
	}{
		{"prompt", func(q *AskQuestion) { q.Prompt = "  " }},
		{"settings", func(q *AskQuestion) { q.Settings = json.RawMessage(`{"unknown":true}`) }},
		{"type", func(q *AskQuestion) { q.AnswerType = "unknown" }},
		{"suggested answer", func(q *AskQuestion) { q.SuggestedAnswer = json.RawMessage(`{"choice":"maybe"}`) }},
		{"unsupported suggestion", func(q *AskQuestion) {
			q.AnswerType = TypeApproval
			q.Settings = json.RawMessage(`{"subject":"Release"}`)
			q.SuggestedAnswer = json.RawMessage(`{"decision":"approve"}`)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			questions := validAskQuestions(3)
			tc.change(&questions[1])
			err := ValidateAsk("Build decisions", questions, PolicyEach)
			if err == nil || !strings.HasPrefix(err.Error(), "question 2:") {
				t.Fatalf("refusal must identify question 2: %v", err)
			}
		})
	}
	for _, suggested := range []json.RawMessage{nil, json.RawMessage("null"), json.RawMessage(" \n null "), json.RawMessage(`{"choice":"yes"}`)} {
		questions := validAskQuestions(1)
		questions[0].SuggestedAnswer = suggested
		if err := ValidateAsk("", questions, ""); err != nil {
			t.Fatalf("valid suggestion %s: %v", suggested, err)
		}
	}
}
