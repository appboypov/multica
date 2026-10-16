package main

import (
 "context"
 "log/slog"
 "time"

 "github.com/multica-ai/multica/server/internal/events"
 "github.com/multica-ai/multica/server/internal/handler"
 "github.com/multica-ai/multica/server/internal/service"
 "github.com/multica-ai/multica/server/internal/util"
 "github.com/multica-ai/multica/server/pkg/protocol"
)

func registerDeciderListeners(ctx context.Context, bus *events.Bus, svc *service.DeciderService) {
 handle:=func(e events.Event, kind, id string) {
  ws,err:=util.ParseUUID(e.WorkspaceID); if err!=nil { return }; issue,err:=util.ParseUUID(id); if err!=nil { return }
  // Model calls do not hold the HTTP request or the event bus.
  go func() {
   run,cancel:=context.WithTimeout(ctx,time.Minute); defer cancel()
   if err:=svc.Handle(run,ws,issue,kind); err!=nil { slog.Error("routine decision failed","workspace_id",e.WorkspaceID,"issue_id",id,"error",err) }
  }()
 }
 issueID:=func(payload map[string]any) string {
  switch issue:=payload["issue"].(type) { case handler.IssueResponse: return issue.ID; case map[string]any: id,_:=issue["id"].(string); return id }
  id,_:=payload["issue_id"].(string); return id
 }
 bus.Subscribe(protocol.EventIssueCreated,func(e events.Event) { if p,ok:=e.Payload.(map[string]any); ok { handle(e,"status_entered",issueID(p)) } })
 bus.Subscribe(protocol.EventIssueUpdated,func(e events.Event) {
  p,ok:=e.Payload.(map[string]any); if !ok || p["decider_applied"]==true || p["status_changed"]!=true { return }
  handle(e,"status_entered",issueID(p))
 })
 bus.Subscribe(protocol.EventIssueReleaseJoined,func(e events.Event) { if p,ok:=e.Payload.(protocol.IssueReleaseJoinedPayload); ok { handle(e,"release_joined",p.IssueID) } })
 complete:=func(e events.Event) {
  p,ok:=e.Payload.(map[string]any); if !ok { return }; id,_:=p["task_id"].(string); taskID,err:=util.ParseUUID(id); if err!=nil { return }
  if err:=svc.Complete(ctx,taskID); err!=nil { slog.Error("routine decision completion failed","task_id",id,"error",err) }
 }
 bus.Subscribe(protocol.EventTaskCompleted,complete)
 bus.Subscribe(protocol.EventTaskFailed,complete)
 bus.Subscribe(protocol.EventTaskCancelled,complete)
}
