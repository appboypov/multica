package question

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// AskQuestion holds the fields that vary between questions in one ask.
type AskQuestion struct {
	Prompt          string          `json:"prompt"`
	ContextLine     string          `json:"context_line"`
	AnswerType      string          `json:"answer_type"`
	Settings        json.RawMessage `json:"settings"`
	SuggestedAnswer json.RawMessage `json:"suggested_answer"`
}

func ValidateAsk(title string, questions []AskQuestion, policy string) error {
	if len(questions) < 1 || len(questions) > 20 {
		return fmt.Errorf("questions must contain 1 to 20 questions")
	}
	if len(questions) == 1 && title != "" {
		return fmt.Errorf("title is not allowed with one question")
	}
	if len(questions) > 1 && (strings.TrimSpace(title) == "" || utf8.RuneCountInString(title) > 200) {
		return fmt.Errorf("title must contain 1 to 200 characters with several questions")
	}
	if policy != "" && policy != PolicyAnyOne && policy != PolicyEach {
		return fmt.Errorf("answer_policy must be any_one or each")
	}
	for i, question := range questions {
		if strings.TrimSpace(question.Prompt) == "" {
			return fmt.Errorf("question %d: prompt is required", i+1)
		}
		if err := ValidateSettings(question.AnswerType, question.Settings); err != nil {
			return fmt.Errorf("question %d: %w", i+1, err)
		}
		if suggested := bytes.TrimSpace(question.SuggestedAnswer); len(suggested) > 0 && !bytes.Equal(suggested, []byte("null")) {
			if err := ValidateSuggested(question.AnswerType, question.Settings, suggested); err != nil {
				return fmt.Errorf("question %d: %w", i+1, err)
			}
		}
	}
	return nil
}
