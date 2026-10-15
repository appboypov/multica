package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// registerActivityListeners wires up event bus listeners that record activity
// entries in the activity_log table. Each listener creates one or more activity
// records depending on what changed, then publishes an activity:created event
// for WS broadcasting.
func registerActivityListeners(bus *events.Bus, queries *db.Queries, runInputs *service.RunInputService) {
	ctx := context.Background()
	registerIssueChangeInputListener(bus, queries, runInputs)

	// issue:created — record "created" activity
	bus.Subscribe(protocol.EventIssueCreated, func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		// HTTP creates carry IssueResponse; autopilot creates carry IssueToMapResolved.
		issue, ok := extractIssueFields(payload["issue"])
		if !ok {
			return
		}
		// A create from a template names its type, so the entry reads
		// "created this story from the Story template" (decision 6).
		details := []byte("{}")
		if fromTemplate, _ := payload["from_template"].(bool); fromTemplate && issue.Type != nil {
			if encoded, err := json.Marshal(map[string]any{"type": *issue.Type, "template": true}); err == nil {
				details = encoded
			}
		}

		activity, err := queries.CreateActivity(ctx, db.CreateActivityParams{
			ID:          dbid.NewV7(),
			WorkspaceID: parseUUID(issue.WorkspaceID),
			IssueID:     parseUUID(issue.ID),
			ActorType:   util.StrToText(e.ActorType),
			ActorID:     optionalUUID(e.ActorID),
			Action:      "created",
			Details:     details,
		})
		if err != nil {
			slog.Error("activity: failed to record issue created",
				"issue_id", issue.ID, "error", err)
			return
		}

		publishActivityEvent(bus, e, activity)
		if issue.ParentIssueID != nil {
			recordSubIssueActivity(ctx, bus, queries, e, *issue.ParentIssueID, "sub_issue_added", issue)
		}
	})

	// issue:updated — record specific changes as separate activities
	bus.Subscribe(protocol.EventIssueUpdated, func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		issue, ok := payload["issue"].(handler.IssueResponse)
		if !ok {
			return
		}

		statusChanged, _ := payload["status_changed"].(bool)
		// A duplicate mark change is a status change with a better name: the
		// duplicate row names the other issue and where the status went, so
		// the generic status row would only repeat it (MUL-7349). Only a row
		// that was actually written stands in for it; when the original is
		// gone there is no duplicate row and the status row still records
		// the move.
		if recordDuplicateMarkActivities(ctx, bus, queries, e, issue, payload) {
			statusChanged = false
		}
		priorityChanged, _ := payload["priority_changed"].(bool)
		assigneeChanged, _ := payload["assignee_changed"].(bool)
		descriptionChanged, _ := payload["description_changed"].(bool)

		if statusChanged {
			prevStatus, _ := payload["prev_status"].(string)
			detailsMap := map[string]string{
				"from": prevStatus,
				"to":   issue.Status,
			}
			// PR auto-complete (MUL-7429): say why the status moved, so the
			// timeline explains the change without a hidden rule.
			if source, _ := payload["source"].(string); source == "pr_automation" {
				detailsMap["source"] = source
				if prs, _ := payload["pull_requests"].(string); prs != "" {
					detailsMap["pull_requests"] = prs
				}
			}
			details, _ := json.Marshal(detailsMap)
			activity, err := queries.CreateActivity(ctx, db.CreateActivityParams{
				ID:          dbid.NewV7(),
				WorkspaceID: parseUUID(issue.WorkspaceID),
				IssueID:     parseUUID(issue.ID),
				ActorType:   util.StrToText(e.ActorType),
				ActorID:     optionalUUID(e.ActorID),
				Action:      "status_changed",
				Details:     details,
			})
			if err != nil {
				slog.Error("activity: failed to record status change",
					"issue_id", issue.ID, "error", err)
			} else {
				publishActivityEvent(bus, e, activity)
			}
		}

		if priorityChanged {
			prevPriority, _ := payload["prev_priority"].(string)
			details, _ := json.Marshal(map[string]string{
				"from": prevPriority,
				"to":   issue.Priority,
			})
			activity, err := queries.CreateActivity(ctx, db.CreateActivityParams{
				ID:          dbid.NewV7(),
				WorkspaceID: parseUUID(issue.WorkspaceID),
				IssueID:     parseUUID(issue.ID),
				ActorType:   util.StrToText(e.ActorType),
				ActorID:     optionalUUID(e.ActorID),
				Action:      "priority_changed",
				Details:     details,
			})
			if err != nil {
				slog.Error("activity: failed to record priority change",
					"issue_id", issue.ID, "error", err)
			} else {
				publishActivityEvent(bus, e, activity)
			}
		}

		if assigneeChanged {
			prevAssigneeType, _ := payload["prev_assignee_type"].(*string)
			prevAssigneeID, _ := payload["prev_assignee_id"].(*string)

			detailsMap := map[string]string{}
			if prevAssigneeType != nil {
				detailsMap["from_type"] = *prevAssigneeType
			}
			if prevAssigneeID != nil {
				detailsMap["from_id"] = *prevAssigneeID
			}
			if issue.AssigneeType != nil {
				detailsMap["to_type"] = *issue.AssigneeType
			}
			if issue.AssigneeID != nil {
				detailsMap["to_id"] = *issue.AssigneeID
			}

			details, _ := json.Marshal(detailsMap)
			activity, err := queries.CreateActivity(ctx, db.CreateActivityParams{
				ID:          dbid.NewV7(),
				WorkspaceID: parseUUID(issue.WorkspaceID),
				IssueID:     parseUUID(issue.ID),
				ActorType:   util.StrToText(e.ActorType),
				ActorID:     optionalUUID(e.ActorID),
				Action:      "assignee_changed",
				Details:     details,
			})
			if err != nil {
				slog.Error("activity: failed to record assignee change",
					"issue_id", issue.ID, "error", err)
			} else {
				publishActivityEvent(bus, e, activity)
			}
		}

		if startDateChanged, _ := payload["start_date_changed"].(bool); startDateChanged {
			prevStartDate := ""
			if v, ok := payload["prev_start_date"].(*string); ok && v != nil {
				prevStartDate = *v
			}
			newStartDate := ""
			if issue.StartDate != nil {
				newStartDate = *issue.StartDate
			}
			details, _ := json.Marshal(map[string]string{
				"from": prevStartDate,
				"to":   newStartDate,
			})
			activity, err := queries.CreateActivity(ctx, db.CreateActivityParams{
				ID:          dbid.NewV7(),
				WorkspaceID: parseUUID(issue.WorkspaceID),
				IssueID:     parseUUID(issue.ID),
				ActorType:   util.StrToText(e.ActorType),
				ActorID:     optionalUUID(e.ActorID),
				Action:      "start_date_changed",
				Details:     details,
			})
			if err != nil {
				slog.Error("activity: failed to record start date change",
					"issue_id", issue.ID, "error", err)
			} else {
				publishActivityEvent(bus, e, activity)
			}
		}

		if dueDateChanged, _ := payload["due_date_changed"].(bool); dueDateChanged {
			prevDueDate := ""
			if v, ok := payload["prev_due_date"].(*string); ok && v != nil {
				prevDueDate = *v
			}
			newDueDate := ""
			if issue.DueDate != nil {
				newDueDate = *issue.DueDate
			}
			details, _ := json.Marshal(map[string]string{
				"from": prevDueDate,
				"to":   newDueDate,
			})
			activity, err := queries.CreateActivity(ctx, db.CreateActivityParams{
				ID:          dbid.NewV7(),
				WorkspaceID: parseUUID(issue.WorkspaceID),
				IssueID:     parseUUID(issue.ID),
				ActorType:   util.StrToText(e.ActorType),
				ActorID:     optionalUUID(e.ActorID),
				Action:      "due_date_changed",
				Details:     details,
			})
			if err != nil {
				slog.Error("activity: failed to record due date change",
					"issue_id", issue.ID, "error", err)
			} else {
				publishActivityEvent(bus, e, activity)
			}
		}

		if titleChanged, _ := payload["title_changed"].(bool); titleChanged {
			prevTitle, _ := payload["prev_title"].(string)
			details, _ := json.Marshal(map[string]string{
				"from": prevTitle,
				"to":   issue.Title,
			})
			activity, err := queries.CreateActivity(ctx, db.CreateActivityParams{
				ID:          dbid.NewV7(),
				WorkspaceID: parseUUID(issue.WorkspaceID),
				IssueID:     parseUUID(issue.ID),
				ActorType:   util.StrToText(e.ActorType),
				ActorID:     optionalUUID(e.ActorID),
				Action:      "title_changed",
				Details:     details,
			})
			if err != nil {
				slog.Error("activity: failed to record title change",
					"issue_id", issue.ID, "error", err)
			} else {
				publishActivityEvent(bus, e, activity)
			}
		}

		if descriptionChanged {
			details, _ := json.Marshal(map[string]string{
				"from": derefPayloadString(payload["prev_description"]),
				"to":   derefPayloadString(issue.Description),
			})
			activity, err := queries.CreateActivity(ctx, db.CreateActivityParams{
				ID:          dbid.NewV7(),
				WorkspaceID: parseUUID(issue.WorkspaceID),
				IssueID:     parseUUID(issue.ID),
				ActorType:   util.StrToText(e.ActorType),
				ActorID:     optionalUUID(e.ActorID),
				Action:      "description_updated",
				Details:     details,
			})
			if err != nil {
				slog.Error("activity: failed to record description change",
					"issue_id", issue.ID, "error", err)
			} else {
				publishActivityEvent(bus, e, activity)
			}
		}
		if changed, _ := payload["parent_changed"].(bool); changed {
			previous := derefPayloadString(payload["prev_parent_issue_id"])
			current := derefPayloadString(issue.ParentIssueID)
			if previous != current {
				if previous != "" {
					recordSubIssueActivity(ctx, bus, queries, e, previous, "sub_issue_removed", issue)
				}
				if current != "" {
					recordSubIssueActivity(ctx, bus, queries, e, current, "sub_issue_added", issue)
				}
			}
		}
	})

	bus.Subscribe(protocol.EventIssueLabelsChanged, func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		issueID, _ := payload["issue_id"].(string)
		label, ok := payload["label"].(map[string]any)
		change, _ := payload["change"].(string)
		if !ok || issueID == "" || (change != "added" && change != "removed") {
			return
		}
		id, _ := label["id"].(string)
		name, _ := label["name"].(string)
		recordIssueRelationActivity(ctx, bus, queries, e, issueID, "label_"+change, map[string]string{"label_id": id, "label_name": name})
	})

	bus.Subscribe(protocol.EventIssueAttachmentsChanged, func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		issueID, _ := payload["issue_id"].(string)
		id, _ := payload["attachment_id"].(string)
		filename, _ := payload["filename"].(string)
		change, _ := payload["change"].(string)
		// Snapshot-only events, and files attached to comments, are not
		// issue-file changes and intentionally carry no attachment id.
		if issueID == "" || id == "" || (change != "added" && change != "removed") {
			return
		}
		recordIssueRelationActivity(ctx, bus, queries, e, issueID, "attachment_"+change, map[string]string{"attachment_id": id, "filename": filename})
	})

	// task:completed — record "task_completed" activity
	bus.Subscribe(protocol.EventTaskCompleted, func(e events.Event) {
		handleTaskActivity(ctx, bus, queries, e, "task_completed")
	})

	// task:failed — record "task_failed" activity
	bus.Subscribe(protocol.EventTaskFailed, func(e events.Event) {
		handleTaskActivity(ctx, bus, queries, e, "task_failed")
	})
}

func recordSubIssueActivity(ctx context.Context, bus *events.Bus, queries *db.Queries, e events.Event, parentID, action string, child handler.IssueResponse) {
	recordIssueRelationActivity(ctx, bus, queries, e, parentID, action, map[string]string{
		"sub_issue_id": child.ID, "sub_issue_identifier": child.Identifier, "sub_issue_title": child.Title,
	})
}

func recordIssueRelationActivity(ctx context.Context, bus *events.Bus, queries *db.Queries, e events.Event, issueID, action string, details map[string]string) {
	body, err := json.Marshal(details)
	if err != nil {
		slog.Error("activity: could not encode relation change", "issue_id", issueID, "action", action, "error", err)
		return
	}
	activity, err := queries.CreateActivity(ctx, db.CreateActivityParams{
		ID: dbid.NewV7(), WorkspaceID: parseUUID(e.WorkspaceID), IssueID: parseUUID(issueID),
		ActorType: util.StrToText(e.ActorType), ActorID: optionalUUID(e.ActorID), Action: action, Details: body,
	})
	if err != nil {
		slog.Error("activity: failed to record relation change", "issue_id", issueID, "action", action, "error", err)
		return
	}
	publishActivityEvent(bus, e, activity)
}

// Activity publication happens after the insert commits. Delivery uses the
// persisted row's identity and details, so the notice and its marks refer to
// precisely the timeline entry viewers see.
func registerIssueChangeInputListener(bus *events.Bus, queries *db.Queries, runInputs *service.RunInputService) {
	bus.Subscribe(protocol.EventActivityCreated, func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		entry, ok := payload["entry"].(map[string]any)
		if !ok {
			return
		}
		action, _ := entry["action"].(string)
		field := ""
		switch action {
		case "status_changed", "duplicate_marked", "duplicate_unmarked":
			field = "status"
		case "priority_changed":
			field = "priority"
		case "assignee_changed":
			field = "assignee"
		case "start_date_changed":
			field = "start date"
		case "due_date_changed":
			field = "due date"
		case "title_changed":
			field = "title"
		case "description_updated":
			field = "description"
		case "label_added", "label_removed", "attachment_added", "attachment_removed", "sub_issue_added", "sub_issue_removed":
			// Relation changes name the added or removed item below.
		default:
			return
		}
		body, ok := entry["details"].(json.RawMessage)
		if !ok {
			return
		}
		var details map[string]string
		if err := json.Unmarshal(body, &details); err != nil {
			slog.Error("activity: invalid issue change details", "action", action, "error", err)
			return
		}
		if (action == "duplicate_marked" || action == "duplicate_unmarked") && details["to"] == "" {
			return
		}
		from, to := details["from"], details["to"]
		if field == "status" || field == "priority" {
			from, to = activityValueLabel(from), activityValueLabel(to)
			if field == "status" {
				_, oldName := issuestatus.CategoryAndName(context.Background(), queries, parseUUID(e.WorkspaceID), details["from"])
				_, newName := issuestatus.CategoryAndName(context.Background(), queries, parseUUID(e.WorkspaceID), details["to"])
				if oldName != "" {
					from = oldName
				}
				if newName != "" {
					to = newName
				}
			}
		}
		if field == "assignee" {
			from = activityIdentityName(context.Background(), queries, details["from_type"], details["from_id"])
			to = activityIdentityName(context.Background(), queries, details["to_type"], details["to_id"])
		}
		actor := activityIdentityName(context.Background(), queries, e.ActorType, e.ActorID)
		var content string
		switch action {
		case "label_added", "label_removed":
			verb := "added"
			if action == "label_removed" {
				verb = "removed"
			}
			content = fmt.Sprintf("%s %s the label %s", actor, verb, details["label_name"])
		case "attachment_added", "attachment_removed":
			verb := "attached"
			if action == "attachment_removed" {
				verb = "removed"
			}
			content = fmt.Sprintf("%s %s %s (attachment %s)", actor, verb, details["filename"], details["attachment_id"])
			if action == "attachment_added" {
				content += fmt.Sprintf("\nmultica attachment download %s", details["attachment_id"])
			}
		case "sub_issue_added", "sub_issue_removed":
			verb := "added"
			if action == "sub_issue_removed" {
				verb = "removed"
			}
			content = fmt.Sprintf("%s %s the sub-issue %s %s", actor, verb, details["sub_issue_identifier"], details["sub_issue_title"])
		default:
			content = fmt.Sprintf("%s changed the %s from %s to %s", actor, field, from, to)
		}
		issueID, _ := payload["issue_id"].(string)
		itemID, _ := entry["id"].(string)
		if _, err := runInputs.Deliver(context.Background(), []pgtype.UUID{parseUUID(issueID)},
			service.RunInputActor{Type: e.ActorType, ID: optionalUUID(e.ActorID)},
			service.RunInputItem{WorkspaceID: parseUUID(e.WorkspaceID), Kind: "issue_change", ID: parseUUID(itemID), Content: content}, nil); err != nil {
			slog.Error("activity: run input delivery failed", "issue_id", issueID, "activity_id", itemID, "error", err)
		}
	})
}

func activityValueLabel(value string) string {
	words := strings.Fields(strings.ReplaceAll(value, "_", " "))
	for i, word := range words {
		runes := []rune(word)
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

func activityIdentityName(ctx context.Context, queries *db.Queries, kind, id string) string {
	if id == "" {
		if kind == "system" {
			return "System"
		}
		return "Unassigned"
	}
	uuid, err := util.ParseUUID(id)
	if err == nil {
		switch kind {
		case "member":
			var user db.User
			user, err = queries.GetUser(ctx, uuid)
			if err == nil {
				return user.Name
			}
		case "agent":
			var agent db.Agent
			agent, err = queries.GetAgent(ctx, uuid)
			if err == nil {
				return agent.Name
			}
		case "squad":
			var squad db.Squad
			squad, err = queries.GetSquad(ctx, uuid)
			if err == nil {
				return squad.Name
			}
		}
	}
	// A removed assignee still has an identity in the persisted change.
	slog.Warn("activity: name unavailable", "actor_type", kind, "actor_id", id, "error", err)
	return id
}

// handleTaskActivity records an activity for task:completed or task:failed events.
func handleTaskActivity(ctx context.Context, bus *events.Bus, queries *db.Queries, e events.Event, action string) {
	payload, ok := e.Payload.(map[string]any)
	if !ok {
		return
	}
	agentID, _ := payload["agent_id"].(string)
	issueID, _ := payload["issue_id"].(string)
	if issueID == "" {
		return
	}

	// Look up issue to get workspace_id
	issue, err := queries.GetIssue(ctx, parseUUID(issueID))
	if err != nil {
		slog.Error("activity: failed to get issue for task event",
			"issue_id", issueID, "action", action, "error", err)
		return
	}

	activity, err := queries.CreateActivity(ctx, db.CreateActivityParams{
		ID:          dbid.NewV7(),
		WorkspaceID: issue.WorkspaceID,
		IssueID:     parseUUID(issueID),
		ActorType:   util.StrToText("agent"),
		ActorID:     parseUUID(agentID),
		Action:      action,
		Details:     []byte("{}"),
	})
	if err != nil {
		slog.Error("activity: failed to record task activity",
			"issue_id", issueID, "action", action, "error", err)
		return
	}

	publishActivityEvent(bus, e, activity)
}

// publishActivityEvent sends an activity:created event for WS broadcasting.
// Payload matches frontend ActivityCreatedPayload: { issue_id, entry: TimelineEntry }
func publishActivityEvent(bus *events.Bus, original events.Event, activity db.ActivityLog) {
	actorType := ""
	if activity.ActorType.Valid {
		actorType = activity.ActorType.String
	}
	action := activity.Action
	bus.Publish(events.Event{
		Type:        protocol.EventActivityCreated,
		WorkspaceID: original.WorkspaceID,
		ActorType:   original.ActorType,
		ActorID:     original.ActorID,
		Payload: map[string]any{
			"issue_id": util.UUIDToString(activity.IssueID),
			"entry": map[string]any{
				"type":       "activity",
				"id":         util.UUIDToString(activity.ID),
				"actor_type": actorType,
				"actor_id":   util.UUIDToString(activity.ActorID),
				"action":     action,
				"details":    json.RawMessage(activity.Details),
				"created_at": util.TimestampToString(activity.CreatedAt),
			},
		},
	})
}

// recordDuplicateMarkActivities logs a duplicate mark change on both issues
// (MUL-7349): duplicate_marked / duplicate_unmarked on the duplicate and
// duplicate_added / duplicate_removed on its original. Every publisher of a
// mark change carries both ends as duplicate_of_issue_id and
// prev_duplicate_of_issue_id. Details name the other issue by id, for
// linking, and by identifier, so the row still reads once that issue is gone.
// It reports whether it wrote a row on the duplicate itself.
func recordDuplicateMarkActivities(ctx context.Context, bus *events.Bus, queries *db.Queries, e events.Event, issue handler.IssueResponse, payload map[string]any) bool {
	if !duplicateMarkChanged(payload) {
		return false
	}
	next := derefPayloadString(payload["duplicate_of_issue_id"])
	prev := derefPayloadString(payload["prev_duplicate_of_issue_id"])
	statusChanged, _ := payload["status_changed"].(bool)
	record := func(issueID, action string, details map[string]string) bool {
		if issueID == issue.ID && statusChanged {
			details["from"], _ = payload["prev_status"].(string)
			details["to"] = issue.Status
		}
		body, _ := json.Marshal(details)
		activity, err := queries.CreateActivity(ctx, db.CreateActivityParams{
			ID:          dbid.NewV7(),
			WorkspaceID: parseUUID(issue.WorkspaceID),
			IssueID:     parseUUID(issueID),
			ActorType:   util.StrToText(e.ActorType),
			ActorID:     optionalUUID(e.ActorID),
			Action:      action,
			Details:     body,
		})
		if err != nil {
			slog.Error("activity: failed to record duplicate mark change",
				"issue_id", issueID, "action", action, "error", err)
			return false
		}
		publishActivityEvent(bus, e, activity)
		return true
	}
	self := map[string]string{"duplicate_id": issue.ID, "duplicate_identifier": issue.Identifier}
	wroteOwnRow := false

	if prev != "" {
		if identifier, ok := lookupIssueIdentifier(ctx, queries, issue.WorkspaceID, prev); ok {
			details := map[string]string{"original_id": prev, "original_identifier": identifier}
			if statusChanged {
				// Removing a mark by reopening moved the status too; this row
				// stands in for the status row, so it says where.
				details["to"] = issue.Status
			}
			wroteOwnRow = record(issue.ID, "duplicate_unmarked", details) || wroteOwnRow
			record(prev, "duplicate_removed", self)
		} else if identifier, _ := payload["prev_duplicate_of_identifier"].(string); identifier != "" {
			// The original was deleted and took its own log with it; the delete
			// path passes the identifier it can no longer be looked up by.
			wroteOwnRow = record(issue.ID, "duplicate_unmarked", map[string]string{
				"original_id": prev, "original_identifier": identifier, "reason": "original_deleted",
			}) || wroteOwnRow
		}
	}
	if next != "" {
		if identifier, ok := lookupIssueIdentifier(ctx, queries, issue.WorkspaceID, next); ok {
			wroteOwnRow = record(issue.ID, "duplicate_marked", map[string]string{"original_id": next, "original_identifier": identifier}) || wroteOwnRow
			record(next, "duplicate_added", self)
		}
	}
	return wroteOwnRow
}

// duplicateMarkChanged reports whether an issue:updated payload carries a
// duplicate mark change: both ends ride as *string fields.
func duplicateMarkChanged(payload map[string]any) bool {
	return derefPayloadString(payload["duplicate_of_issue_id"]) != derefPayloadString(payload["prev_duplicate_of_issue_id"])
}

// derefPayloadString reads a *string payload field; nil and absent read as "".
func derefPayloadString(v any) string {
	if s, ok := v.(*string); ok && s != nil {
		return *s
	}
	return ""
}

// lookupIssueIdentifier renders the identifier of an issue in the workspace;
// ok is false when the issue no longer exists.
func lookupIssueIdentifier(ctx context.Context, queries *db.Queries, workspaceID, issueID string) (string, bool) {
	row, err := queries.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{
		ID:          parseUUID(issueID),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		return "", false
	}
	ws, err := queries.GetWorkspace(ctx, row.WorkspaceID)
	if err != nil {
		return "", false
	}
	return service.IssueIdentifier(ws.IssuePrefix, row.Number), true
}
