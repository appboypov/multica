package main

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestNotification_MalformedMemberMentionPreservesValidRecipients(t *testing.T) {
	for _, tc := range []struct {
		name       string
		mentionAll bool
		mapPayload bool
	}{
		{name: "explicit member"},
		{name: "explicit member map payload", mapPayload: true},
		{name: "all members", mentionAll: true},
		{name: "all members map payload", mentionAll: true, mapPayload: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := workspaceFixture(t)
			recipientID := fx.User(t, "Mention recipient", "mention-recipient@multica.ai")
			mutedID := fx.User(t, "Muted recipient", "mention-muted@multica.ai")
			fx.Member(t, testWorkspaceID, recipientID, "member")
			fx.Member(t, testWorkspaceID, mutedID, "member")
			fx.Insert(t, "notification_preference", testutil.Cols{
				"workspace_id": testWorkspaceID,
				"user_id":      mutedID,
				"preferences":  `{"mentions":"muted"}`,
			})
			issueID := fx.Issue(t, "Malformed mention")
			fx.Cleanup(t, `DELETE FROM inbox_item WHERE issue_id = $1`, issueID)
			fx.Cleanup(t, `DELETE FROM todo WHERE issue_id = $1`, issueID)
			fx.Cleanup(t, `DELETE FROM issue_subscriber WHERE issue_id = $1`, issueID)

			// These IDs match the markdown parser but are not UUIDs. Keep a
			// valid recipient on either side to exercise the whole batch.
			content := "[@Recipient](mention://member/" + recipientID + ") " +
				"[@Bad](mention://member/deadbeef) [@Bad all](mention://member/all) " +
				"[@Recipient again](mention://member/" + recipientID + ") " +
				"[@Muted](mention://member/" + mutedID + ") " +
				"[@Self](mention://member/" + testUserID + ")"
			if tc.mentionAll {
				content = "[@Bad](mention://member/deadbeef) [@All](mention://all/all)"
			}
			commentID := fx.Comment(t, issueID, content)
			comment := handler.CommentResponse{
				ID: commentID, IssueID: issueID, AuthorType: "member",
				AuthorID: testUserID, Content: content, Type: "comment",
			}
			var commentPayload any = comment
			if tc.mapPayload {
				commentPayload = map[string]any{
					"id": commentID, "issue_id": issueID, "author_type": "member",
					"author_id": testUserID, "content": content, "type": "comment",
				}
			}

			queries := db.New(testPool)
			bus := newNotificationBus(t, queries)
			registerTodoListeners(bus, queries, testPool)
			var inboxEvents []events.Event
			bus.Subscribe(protocol.EventInboxNew, func(e events.Event) {
				inboxEvents = append(inboxEvents, e)
			})
			bus.Publish(events.Event{
				Type: protocol.EventCommentCreated, WorkspaceID: testWorkspaceID,
				ActorType: "member", ActorID: testUserID,
				Payload: map[string]any{
					"comment": commentPayload, "issue_title": "Malformed mention", "issue_status": "todo",
				},
			})

			for _, member := range []string{recipientID, mutedID} {
				items, err := queries.ListOpenTodos(t.Context(), db.ListOpenTodosParams{
					WorkspaceID: parseUUID(testWorkspaceID), MemberID: parseUUID(member),
				})
				if err != nil || len(items) != 1 || items[0].Kind != "mention" || util.UUIDToString(items[0].CommentID) != commentID {
					t.Fatalf("member %s mention to-dos=%+v error=%v", member, items, err)
				}
			}
			if count := fx.Count(t, `SELECT count(*) FROM todo WHERE issue_id = $1 AND done_at IS NULL`, issueID); count != 2 {
				t.Fatalf("issue has %d to-dos, want both recipients regardless of notice preferences", count)
			}
			if count := fx.Count(t, `SELECT count(*) FROM inbox_item WHERE issue_id = $1 AND type='mentioned'`, issueID); count != 0 {
				t.Fatalf("issue has %d obsolete mention notices", count)
			}
			for _, e := range inboxEvents {
				if item := e.Payload.(map[string]any)["item"].(map[string]any); item["type"] == "mentioned" {
					t.Fatal("mention emitted an update notice")
				}
			}
		})
	}
}
