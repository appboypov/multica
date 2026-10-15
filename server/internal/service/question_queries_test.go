package service

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestQuestionQueriesChatUnseenScopeAndRetryChain(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	row := f.ask(t)
	id := util.MustParseUUID(row.ID)
	f.answer(t, row, f.answerer, "per line")
	chat := util.MustParseUUID(f.fx.ChatSession(t, util.UUIDToString(f.agent)))
	f.fx.Exec(t, `UPDATE agent_task_queue SET issue_id = NULL, chat_session_id = $2,
		status = 'completed', dispatched_at = '2026-10-15 08:00:00Z',
		started_at = '2026-10-15 08:01:00Z' WHERE id = $1`, f.task, chat)
	f.fx.Exec(t, `UPDATE question_link SET link_type = 'chat', link_id = $2
		WHERE question_id = $1 AND position = 0`, id, chat)
	f.fx.Exec(t, `UPDATE question_answer SET updated_at = '2026-10-15 10:00:00Z' WHERE question_id = $1`, id)
	f.fx.Exec(t, `UPDATE question SET status = 'cancelled' WHERE id = $1`, id)
	for _, event := range []struct {
		kind, details string
		at            time.Time
	}{
		{"ask_back", `{"text":"Which customer?"}`, time.Date(2026, 10, 15, 10, 1, 0, 0, time.UTC)},
		{"cancelled", `{"note":"already decided in the PRD"}`, time.Date(2026, 10, 15, 10, 2, 0, 0, time.UTC)},
	} {
		f.fx.Insert(t, "question_event", testutil.Cols{
			"workspace_id": f.workspace, "question_id": id, "kind": event.kind,
			"actor_type": "member", "actor_id": f.brian, "details": event.details, "created_at": event.at,
		})
	}
	// A newer issue run must not advance this chat's cutoff.
	f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{
		"issue_id": f.issue, "status": "completed",
		"dispatched_at": testutil.Raw("'2026-10-15 12:00:00Z'::timestamptz"), "started_at": testutil.Raw("'2026-10-15 12:01:00Z'::timestamptz"),
	})
	q := f.svc.Queries
	ids, err := q.ListChatQuestionIDs(ctx, db.ListChatQuestionIDsParams{WorkspaceID: f.workspace, AgentID: f.agent, ChatSessionID: chat})
	if err != nil || len(ids) != 1 || ids[0] != id {
		t.Fatalf("chat question IDs = %v, %v, want cancelled question %v", ids, err, id)
	}
	count, err := q.CountUnseenQuestionAnswers(ctx, db.CountUnseenQuestionAnswersParams{WorkspaceID: f.workspace, AgentID: f.agent, ChatSessionID: chat, QuestionIds: ids})
	if err != nil || count != 1 {
		t.Fatalf("chat unseen count after issue run = %d, %v, want 1", count, err)
	}
	count, err = q.CountUnseenQuestionAnswers(ctx, db.CountUnseenQuestionAnswersParams{WorkspaceID: f.workspace, AgentID: f.agent, IssueID: f.issue, QuestionIds: ids})
	if err != nil || count != 0 {
		t.Fatalf("issue unseen count = %d, %v, want 0", count, err)
	}
	ancestor := f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{
		"chat_session_id": chat, "status": "failed",
		"dispatched_at": testutil.Raw("'2026-10-15 11:00:00Z'::timestamptz"), "started_at": testutil.Raw("'2026-10-15 11:01:00Z'::timestamptz"),
	})
	retry := f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{
		"chat_session_id": chat, "status": "failed", "retry_of_task_id": ancestor,
		"dispatched_at": testutil.Raw("'2026-10-15 12:00:00Z'::timestamptz"), "started_at": testutil.Raw("'2026-10-15 12:01:00Z'::timestamptz"),
	})
	claim := util.MustParseUUID(f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{
		"chat_session_id": chat, "status": "dispatched", "retry_of_task_id": retry, "dispatched_at": testutil.Raw("'2026-10-15 13:00:00Z'::timestamptz"),
	}))
	count, err = q.CountUnseenQuestionAnswers(ctx, db.CountUnseenQuestionAnswersParams{WorkspaceID: f.workspace, AgentID: f.agent, ChatSessionID: chat, QuestionIds: ids, ExcludeTaskID: claim})
	if err != nil || count != 1 {
		t.Fatalf("retry unseen count = %d, %v, want 1", count, err)
	}
	answers, err := q.ListUnseenQuestionAnswers(ctx, db.ListUnseenQuestionAnswersParams{WorkspaceID: f.workspace, AgentID: f.agent, ChatSessionID: chat, QuestionIds: ids, ExcludeTaskID: claim})
	if err != nil || len(answers) != 1 || answers[0].QuestionID != id {
		t.Fatalf("retry unseen answers = %+v, %v", answers, err)
	}
	events, err := q.ListUnseenQuestionEvents(ctx, db.ListUnseenQuestionEventsParams{WorkspaceID: f.workspace, AgentID: f.agent, ChatSessionID: chat, QuestionIds: ids, ExcludeTaskID: claim})
	if err != nil || len(events) != 2 || events[0].Kind != "ask_back" || events[1].Kind != "cancelled" || events[1].ActorID != f.brian {
		t.Fatalf("retry unseen events = %+v, %v", events, err)
	}
	count, err = q.CountUnseenQuestionAnswers(ctx, db.CountUnseenQuestionAnswersParams{WorkspaceID: f.workspace, AgentID: f.agent, ChatSessionID: chat, QuestionIds: ids})
	if err != nil || count != 0 {
		t.Fatalf("started chat retry advances cutoff outside chain exclusion: %d, %v", count, err)
	}
}

func TestQuestionQueriesChatListsOverdueAndActiveTasks(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	row := f.ask(t)
	id := util.MustParseUUID(row.ID)
	chat := util.MustParseUUID(f.fx.ChatSession(t, util.UUIDToString(f.agent), testutil.Cols{"title": "Private title"}))
	f.fx.Exec(t, `UPDATE agent_task_queue SET issue_id = NULL, chat_session_id = $2 WHERE id = $1`, f.task, chat)
	f.fx.Exec(t, `UPDATE question_link SET link_type = 'chat', link_id = $2 WHERE question_id = $1 AND position = 0`, id, chat)
	q := f.svc.Queries
	cards, err := q.ListQuestionsLinkedTo(ctx, db.ListQuestionsLinkedToParams{WorkspaceID: f.workspace, LinkType: "chat", LinkID: chat})
	if err != nil || len(cards) != 1 || cards[0].ID != id {
		t.Fatalf("chat cards = %+v, %v", cards, err)
	}
	issueCards, err := q.ListQuestionsLinkedTo(ctx, db.ListQuestionsLinkedToParams{WorkspaceID: f.workspace, LinkType: "issue", LinkID: f.issue})
	if err != nil || len(issueCards) != 0 {
		t.Fatalf("unlinked issue cards = %+v, %v", issueCards, err)
	}
	// A secondary issue link must not duplicate the position-0 overdue row.
	if err := q.CreateQuestionLink(ctx, db.CreateQuestionLinkParams{QuestionID: id, WorkspaceID: f.workspace, LinkType: "issue", LinkID: f.issue, Position: 2}); err != nil {
		t.Fatal(err)
	}
	issueCards, err = q.ListQuestionsLinkedTo(ctx, db.ListQuestionsLinkedToParams{WorkspaceID: f.workspace, LinkType: "issue", LinkID: f.issue})
	if err != nil || len(issueCards) != 1 || issueCards[0].ID != id {
		t.Fatalf("linked issue cards = %+v, %v", issueCards, err)
	}
	overdue, err := q.ListOpenOverdueQuestions(ctx, db.ListOpenOverdueQuestionsParams{
		BeforeDate: pgtype.Date{Time: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), Valid: true}, MaxRows: 2147483647,
	})
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, due := range overdue {
		if due.ID == id {
			matches++
			if due.IssueID.Valid || due.ChatSessionID != chat {
				t.Fatalf("overdue scope = issue %v, chat %v", due.IssueID, due.ChatSessionID)
			}
		}
	}
	if matches != 1 {
		t.Fatalf("overdue rows for chat question = %d, want 1", matches)
	}
	rows, err := q.ListQuestionRowsForMember(ctx, db.ListQuestionRowsForMemberParams{WorkspaceID: f.workspace, ViewerID: f.brian, Limit: 100})
	if err != nil || len(rows) != 1 || rows[0].LinkType != "chat" || rows[0].LinkID != chat || rows[0].LinkMemberID != f.brian {
		t.Fatalf("chat viewer rows = %+v, %v", rows, err)
	}
	tasks, err := q.ListChatTaskIDs(ctx, db.ListChatTaskIDsParams{WorkspaceID: f.workspace, ChatSessionID: chat})
	if err != nil || len(tasks) != 1 || tasks[0] != f.task {
		t.Fatalf("chat task IDs = %v, %v", tasks, err)
	}
	for _, status := range []string{"queued", "dispatched", "running", "completed", "failed", "cancelled"} {
		f.fx.Exec(t, `UPDATE agent_task_queue SET status = $2 WHERE id = $1`, f.task, status)
		active, err := q.HasActiveChatTaskForAgent(ctx, db.HasActiveChatTaskForAgentParams{WorkspaceID: f.workspace, AgentID: f.agent, ChatSessionID: chat})
		want := status == "queued" || status == "dispatched" || status == "running"
		if err != nil || active != want {
			t.Fatalf("chat active for %s = %v, %v, want %v", status, active, err, want)
		}
	}
	otherWorkspace := util.MustParseUUID(f.fx.Workspace(t, "Other", "question-query-other-"+row.ID))
	cards, err = q.ListQuestionsLinkedTo(ctx, db.ListQuestionsLinkedToParams{WorkspaceID: otherWorkspace, LinkType: "chat", LinkID: chat})
	if err != nil || len(cards) != 0 {
		t.Fatalf("other workspace cards = %+v, %v", cards, err)
	}
	tasks, err = q.ListChatTaskIDs(ctx, db.ListChatTaskIDsParams{WorkspaceID: otherWorkspace, ChatSessionID: chat})
	if err != nil || len(tasks) != 0 {
		t.Fatalf("other workspace task IDs = %v, %v", tasks, err)
	}
}
