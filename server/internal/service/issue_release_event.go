package service

import (
 "github.com/jackc/pgx/v5/pgtype"
 "github.com/multica-ai/multica/server/internal/events"
 "github.com/multica-ai/multica/server/internal/util"
 db "github.com/multica-ai/multica/server/pkg/db/generated"
 "github.com/multica-ai/multica/server/pkg/protocol"
)

// PublishIssueReleaseJoined runs after the membership transaction commits.
func PublishIssueReleaseJoined(bus *events.Bus, issue db.Issue, previous pgtype.UUID, actorType, actorID string) {
 if bus==nil || !issue.ReleaseID.Valid || issue.ReleaseID==previous { return }
 bus.Publish(events.Event{Type:protocol.EventIssueReleaseJoined,WorkspaceID:util.UUIDToString(issue.WorkspaceID),ActorType:actorType,ActorID:actorID,Payload:protocol.IssueReleaseJoinedPayload{IssueID:util.UUIDToString(issue.ID),ReleaseID:util.UUIDToString(issue.ReleaseID)}})
}
