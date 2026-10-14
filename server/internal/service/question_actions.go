package service

// Inbox item types and the activity action a question writes. One spelling
// for the notification listener, the overdue job and the tests.
const (
	// InboxTypeQuestionAsked notifies every answerer of a new ask.
	InboxTypeQuestionAsked = "question_asked"
	// InboxTypeQuestionReassigned notifies the new answerer, with the note.
	InboxTypeQuestionReassigned = "question_reassigned"
	// InboxTypeQuestionOverdue notifies every current answerer once per
	// ask when its due date has passed.
	InboxTypeQuestionOverdue = "question_overdue"

	// ActivityQuestionWake is the activity_log action Wake writes, with
	// details {agent_id, answer_count, task_id}.
	ActivityQuestionWake = "question_wake"
)

// Inbox details shared by the question notification producers and consumers.
const (
	QuestionDetailAskID         = "ask_id"
	QuestionDetailQuestionCount = "question_count"
)
