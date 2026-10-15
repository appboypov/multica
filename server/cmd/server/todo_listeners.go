package main

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/question"
	"github.com/multica-ai/multica/server/internal/todo"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func registerTodoListeners(bus *events.Bus, queries *db.Queries) {
	svc := todo.Service{Queries: queries, Bus: bus}
	for _, kind := range []string{protocol.EventQuestionCreated, protocol.EventQuestionUpdated} {
		bus.Subscribe(kind, func(e events.Event) {
			if err := questionTodos(context.Background(), svc, e); err != nil {
				slog.Error("question todo transition failed", "error", err)
			}
		})
	}
	for _, kind := range []string{protocol.EventCommentCreated, protocol.EventIssueCreated, protocol.EventIssueUpdated} {
		bus.Subscribe(kind, func(e events.Event) {
			if err := mentionTodos(context.Background(), svc, e); err != nil {
				slog.Error("mention todo transition failed", "error", err)
			}
		})
	}
	for _, kind := range []string{protocol.EventIssueCreated, protocol.EventIssueUpdated} {
		bus.Subscribe(kind, func(e events.Event) {
			if err := assignedTodos(context.Background(), svc, e); err != nil {
				slog.Error("assigned todo transition failed", "error", err)
			}
			if err := reviewTodos(context.Background(), svc, e); err != nil {
				slog.Error("review todo transition failed", "error", err)
			}
		})
	}
	for _, kind := range []string{protocol.EventIssueUpdated, protocol.EventTaskFailed, protocol.EventTaskDispatch} {
		bus.Subscribe(kind, func(e events.Event) {
			if err := agentTodos(context.Background(), svc, e); err != nil {
				slog.Error("agent todo transition failed", "error", err)
			}
		})
	}
}

func questionTodos(ctx context.Context, svc todo.Service, e events.Event) error {
	p, ok := e.Payload.(protocol.QuestionEventPayload)
	if !ok {
		return nil
	}
	workspace, err := util.ParseUUID(e.WorkspaceID)
	if err != nil {
		return err
	}
	ids := p.QuestionIDs
	if len(ids) == 0 {
		ids = []string{p.QuestionID}
	}
	for _, id := range ids {
		questionID, err := util.ParseUUID(id)
		if err != nil {
			return err
		}
		row, err := svc.Queries.GetQuestion(ctx, db.GetQuestionParams{ID: questionID, WorkspaceID: workspace})
		if err != nil {
			return err
		}
		close := db.CloseTodosParams{WorkspaceID: workspace, Kind: todo.Question, SourceKey: id}
		if !question.IsOpen(row.Status) {
			if err := svc.Close(ctx, close); err != nil {
				return err
			}
			continue
		}
		if p.Reason == protocol.QuestionReasonAnswered || p.Reason == protocol.QuestionReasonReassigned {
			close.MemberID, err = util.ParseUUID(e.ActorID)
			if err != nil {
				return err
			}
			if err := svc.Close(ctx, close); err != nil {
				return err
			}
		}
		if e.Type != protocol.EventQuestionCreated && p.Reason != protocol.QuestionReasonReassigned {
			continue
		}
		answerers, err := svc.Queries.ListQuestionAnswerersByQuestions(ctx, db.ListQuestionAnswerersByQuestionsParams{WorkspaceID: workspace, QuestionIds: []pgtype.UUID{questionID}})
		if err != nil {
			return err
		}
		answers, err := svc.Queries.ListQuestionAnswersByQuestions(ctx, db.ListQuestionAnswersByQuestionsParams{WorkspaceID: workspace, QuestionIds: []pgtype.UUID{questionID}})
		if err != nil {
			return err
		}
		answered := make(map[pgtype.UUID]bool, len(answers))
		for _, answer := range answers {
			answered[answer.AnswererUserID] = true
		}
		var reassignedTo string
		if p.Reason == protocol.QuestionReasonReassigned {
			history, err := svc.Queries.ListQuestionEventsByQuestions(ctx, db.ListQuestionEventsByQuestionsParams{WorkspaceID: workspace, QuestionIds: []pgtype.UUID{questionID}})
			if err != nil {
				return err
			}
			for _, event := range history {
				if util.UUIDToString(event.ID) != p.EventID {
					continue
				}
				var details struct {
					UserID string `json:"user_id"`
				}
				if err := json.Unmarshal(event.Details, &details); err != nil {
					return err
				}
				reassignedTo = details.UserID
			}
		}
		issueID, err := util.ParseUUID(p.IssueID)
		if err != nil {
			return err
		}
		issue, err := svc.Queries.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: issueID, WorkspaceID: workspace})
		if err != nil {
			return err
		}
		for _, answerer := range answerers {
			if p.Reason == protocol.QuestionReasonReassigned && util.UUIDToString(answerer.UserID) != reassignedTo {
				continue
			}
			if answered[answerer.UserID] {
				continue
			}
			if err := svc.Open(ctx, db.OpenTodoParams{WorkspaceID: workspace, MemberID: answerer.UserID, Kind: todo.Question, SourceKey: id, IssueID: issueID, QuestionID: questionID, ActorType: util.StrToText("agent"), ActorID: row.AskerAgentID, Title: issue.Title, Body: util.StrToText(row.Prompt)}); err != nil {
				return err
			}
		}
	}
	return nil
}

func mentionTodos(ctx context.Context, svc todo.Service, e events.Event) error {
	p, ok := e.Payload.(map[string]any)
	if !ok {
		return nil
	}
	workspace, err := util.ParseUUID(e.WorkspaceID)
	if err != nil {
		return err
	}
	var issueID, source, content, previous string
	var commentID pgtype.UUID
	if e.Type == protocol.EventCommentCreated {
		var authorType string
		switch c := p["comment"].(type) {
		case handler.CommentResponse:
			issueID, source, content, authorType = c.IssueID, c.ID, c.Content, c.AuthorType
		case map[string]any:
			issueID, _ = c["issue_id"].(string)
			source, _ = c["id"].(string)
			content, _ = c["content"].(string)
			authorType, _ = c["author_type"].(string)
		default:
			return nil
		}
		if authorType == "system" {
			return nil
		}
		commentID, err = util.ParseUUID(source)
		if err != nil {
			return err
		}
		if authorType == "member" {
			member, err := util.ParseUUID(e.ActorID)
			if err != nil {
				return err
			}
			issue, err := util.ParseUUID(issueID)
			if err != nil {
				return err
			}
			if err := svc.CloseOnIssue(ctx, db.CloseTodosOnIssueParams{WorkspaceID: workspace, IssueID: issue, MemberID: member, Kinds: []string{todo.Mention}}); err != nil {
				return err
			}
		}
	} else {
		issue, ok := p["issue"].(handler.IssueResponse)
		if !ok || issue.Description == nil {
			return nil
		}
		if e.Type == protocol.EventIssueUpdated {
			changed, _ := p["description_changed"].(bool)
			if !changed {
				return nil
			}
			if prev, ok := p["prev_description"].(*string); ok && prev != nil {
				previous = *prev
			}
		}
		issueID, source, content = issue.ID, issue.ID, *issue.Description
	}
	issueUUID, err := util.ParseUUID(issueID)
	if err != nil {
		return err
	}
	issue, err := svc.Queries.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: issueUUID, WorkspaceID: workspace})
	if err != nil {
		return err
	}
	prev := map[string]bool{}
	for _, m := range util.ParseMentions(previous) {
		prev[m.Type+":"+m.ID] = true
	}
	recipients := map[pgtype.UUID]bool{}
	for _, m := range util.ParseMentions(content) {
		if prev[m.Type+":"+m.ID] {
			continue
		}
		switch m.Type {
		case "member":
			id, err := util.ParseUUID(m.ID)
			if err == nil {
				recipients[id] = true
			}
		case "all":
			members, err := svc.Queries.ListMembers(ctx, workspace)
			if err != nil {
				return err
			}
			for _, member := range members {
				recipients[member.UserID] = true
			}
		case "squad":
			id, err := util.ParseUUID(m.ID)
			if err != nil {
				continue
			}
			members, err := svc.Queries.ListSquadMembers(ctx, id)
			if err != nil {
				return err
			}
			for _, member := range members {
				if member.MemberType == "member" {
					recipients[member.MemberID] = true
				}
			}
		}
	}
	for member := range recipients {
		if e.ActorType == "member" && util.UUIDToString(member) == e.ActorID {
			continue
		}
		if err := svc.Open(ctx, db.OpenTodoParams{WorkspaceID: workspace, MemberID: member, Kind: todo.Mention, SourceKey: source, IssueID: issueUUID, CommentID: commentID, ActorType: util.StrToText(e.ActorType), ActorID: optionalUUID(e.ActorID), Title: issue.Title}); err != nil {
			return err
		}
	}
	return nil
}

func assignedTodos(ctx context.Context, svc todo.Service, e events.Event) error {
	p, ok := e.Payload.(map[string]any)
	if !ok {
		return nil
	}
	issue, ok := p["issue"].(handler.IssueResponse)
	if !ok {
		return nil
	}
	workspace, err := util.ParseUUID(e.WorkspaceID)
	if err != nil {
		return err
	}
	issueID, err := util.ParseUUID(issue.ID)
	if err != nil {
		return err
	}
	if e.Type == protocol.EventIssueUpdated {
		statusChanged, _ := p["status_changed"].(bool)
		assigneeChanged, _ := p["assignee_changed"].(bool)
		if !statusChanged && !assigneeChanged {
			return nil
		}
		if err := svc.Close(ctx, db.CloseTodosParams{WorkspaceID: workspace, Kind: todo.Assigned, SourceKey: issue.ID}); err != nil {
			return err
		}
	}
	if issue.AssigneeType == nil || *issue.AssigneeType != "member" || issue.AssigneeID == nil {
		return nil
	}
	switch issue.Status {
	case "todo", "back_from_qa", "blocked", "in_review":
	default:
		return nil
	}
	member, err := util.ParseUUID(*issue.AssigneeID)
	if err != nil {
		return err
	}
	return svc.Open(ctx, db.OpenTodoParams{WorkspaceID: workspace, MemberID: member, Kind: todo.Assigned, SourceKey: issue.ID, IssueID: issueID, ActorType: util.StrToText(e.ActorType), ActorID: optionalUUID(e.ActorID), Title: issue.Title})
}

func agentTodos(ctx context.Context, svc todo.Service, e events.Event) error {
	p, ok := e.Payload.(map[string]any)
	if !ok {
		return nil
	}
	workspace, err := util.ParseUUID(e.WorkspaceID)
	if err != nil {
		return err
	}
	var issueID, taskID string
	if e.Type == protocol.EventIssueUpdated {
		issue, ok := p["issue"].(handler.IssueResponse)
		if !ok {
			return nil
		}
		changed, _ := p["status_changed"].(bool)
		if !changed {
			return nil
		}
		issueID = issue.ID
		id, err := util.ParseUUID(issueID)
		if err != nil {
			return err
		}
		if issue.Status != "blocked" {
			return svc.CloseOnIssue(ctx, db.CloseTodosOnIssueParams{WorkspaceID: workspace, IssueID: id, Kinds: []string{todo.AgentBlocked, todo.RunFailed}})
		}
		if e.ActorType != "agent" || issue.AssigneeType == nil || (*issue.AssigneeType != "agent" && *issue.AssigneeType != "squad") {
			return nil
		}
		taskID, _ = p["source_task_id"].(string)
		if taskID == "" {
			return nil
		}
	} else {
		issueID, _ = p["issue_id"].(string)
		if issueID == "" {
			return nil
		}
		if e.Type == protocol.EventTaskDispatch {
			id, err := util.ParseUUID(issueID)
			if err != nil {
				return err
			}
			return svc.CloseOnIssue(ctx, db.CloseTodosOnIssueParams{WorkspaceID: workspace, IssueID: id, Kinds: []string{todo.AgentBlocked, todo.RunFailed}})
		}
		taskID, _ = p["task_id"].(string)
	}
	taskUUID, err := util.ParseUUID(taskID)
	if err != nil {
		return err
	}
	task, err := svc.Queries.GetAgentTaskInWorkspace(ctx, db.GetAgentTaskInWorkspaceParams{ID: taskUUID, WorkspaceID: workspace})
	if err != nil {
		return err
	}
	if !task.OriginatorUserID.Valid || util.UUIDToString(task.IssueID) != issueID {
		return nil
	}
	if e.Type == protocol.EventIssueUpdated && util.UUIDToString(task.AgentID) != e.ActorID {
		return nil
	}
	issue, err := svc.Queries.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: task.IssueID, WorkspaceID: workspace})
	if err != nil {
		return err
	}
	kind, source := todo.RunFailed, taskID
	if e.Type == protocol.EventIssueUpdated {
		kind, source = todo.AgentBlocked, issueID
	}
	return svc.Open(ctx, db.OpenTodoParams{WorkspaceID: workspace, MemberID: task.OriginatorUserID, Kind: kind, SourceKey: source, IssueID: task.IssueID, TaskID: task.ID, ActorType: util.StrToText("agent"), ActorID: task.AgentID, Title: issue.Title})
}

func reviewTodos(ctx context.Context, svc todo.Service, e events.Event) error {
	p, ok := e.Payload.(map[string]any)
	if !ok {
		return nil
	}
	issue, ok := p["issue"].(handler.IssueResponse)
	if !ok || issue.CreatorType != "member" {
		return nil
	}
	selfAssigned := issue.AssigneeType != nil && *issue.AssigneeType == "member" && issue.AssigneeID != nil && *issue.AssigneeID == issue.CreatorID
	if e.Type == protocol.EventIssueUpdated {
		statusChanged, _ := p["status_changed"].(bool)
		assigneeChanged, _ := p["assignee_changed"].(bool)
		previous, _ := p["prev_assignee_id"].(*string)
		previousType, _ := p["prev_assignee_type"].(*string)
		wasSelfAssigned := previous != nil && *previous == issue.CreatorID && previousType != nil && *previousType == "member"
		if !statusChanged && !(assigneeChanged && (selfAssigned || wasSelfAssigned)) {
			return nil
		}
	}
	workspace, err := util.ParseUUID(e.WorkspaceID)
	if err != nil {
		return err
	}
	member, err := util.ParseUUID(issue.CreatorID)
	if err != nil {
		return err
	}
	if issue.Status != "in_review" || selfAssigned {
		return svc.Close(ctx, db.CloseTodosParams{WorkspaceID: workspace, MemberID: member, Kind: todo.Review, SourceKey: issue.ID})
	}
	issueID, err := util.ParseUUID(issue.ID)
	if err != nil {
		return err
	}
	return svc.Open(ctx, db.OpenTodoParams{WorkspaceID: workspace, MemberID: member, Kind: todo.Review, SourceKey: issue.ID, IssueID: issueID, ActorType: util.StrToText(e.ActorType), ActorID: optionalUUID(e.ActorID), Title: issue.Title})
}
