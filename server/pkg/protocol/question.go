package protocol

// QuestionAnswers carries run-scoped answers hydrated when a question wake is claimed.
type QuestionAnswers struct {
	WakerName   string                `json:"waker_name"`
	AgentID     string                `json:"agent_id,omitempty"`
	AnswerCount int64                 `json:"answer_count"`
	Answers     []QuestionAnswerData  `json:"answers"`
	AskBacks    []QuestionAskBackData `json:"ask_backs"`
}

type QuestionAnswerData struct {
	QuestionID   string           `json:"question_id"`
	AskerAgentID string           `json:"asker_agent_id,omitempty"`
	AskerName    string           `json:"asker_name,omitempty"`
	Via          *QuestionViaData `json:"via,omitempty"`
	Prompt       string           `json:"prompt"`
	DisplayText  string           `json:"display_text"`
	Note         string           `json:"note"`
	AnswererName string           `json:"answerer_name"`
	UpdatedAt    string           `json:"updated_at"`
}

type QuestionAskBackData struct {
	QuestionID   string           `json:"question_id"`
	AskerAgentID string           `json:"asker_agent_id,omitempty"`
	AskerName    string           `json:"asker_name,omitempty"`
	Via          *QuestionViaData `json:"via,omitempty"`
	Prompt       string           `json:"prompt"`
	Text         string           `json:"text"`
	ActorName    string           `json:"actor_name"`
	CreatedAt    string           `json:"created_at"`
}

type QuestionViaData struct {
	Type            string `json:"type"`
	ID              string `json:"id"`
	Identifier      string `json:"identifier,omitempty"`
	IssueIdentifier string `json:"issue_identifier,omitempty"`
}
