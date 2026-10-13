package protocol

// QuestionAnswers carries run-scoped answers hydrated when a question wake is claimed.
type QuestionAnswers struct {
	WakerName   string                `json:"waker_name"`
	AnswerCount int64                 `json:"answer_count"`
	Answers     []QuestionAnswerData  `json:"answers"`
	AskBacks    []QuestionAskBackData `json:"ask_backs"`
}

type QuestionAnswerData struct {
	QuestionID   string `json:"question_id"`
	Prompt       string `json:"prompt"`
	DisplayText  string `json:"display_text"`
	Note         string `json:"note"`
	AnswererName string `json:"answerer_name"`
	UpdatedAt    string `json:"updated_at"`
}

type QuestionAskBackData struct {
	QuestionID string `json:"question_id"`
	Prompt     string `json:"prompt"`
	Text       string `json:"text"`
	ActorName  string `json:"actor_name"`
	CreatedAt  string `json:"created_at"`
}
