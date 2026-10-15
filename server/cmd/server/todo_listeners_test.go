package main

import (
 "testing"
 "github.com/multica-ai/multica/server/internal/events"
 "github.com/multica-ai/multica/server/pkg/protocol"
)

func TestTodoRealtimeIsMemberPrivate(t *testing.T) {
 for _, kind := range []string{protocol.EventTodoOpened, protocol.EventTodoClosed} {
  t.Run(kind, func(t *testing.T) {
   bus := events.New()
   broadcaster := &fakeBroadcaster{}
   registerListeners(bus, broadcaster)
   bus.Publish(events.Event{Type:kind, WorkspaceID:"workspace", Payload:map[string]any{"member_id":"member"}})
   if len(broadcaster.userCalls) != 1 || len(broadcaster.workspaceCalls) != 0 || broadcaster.broadcastCalled != 0 { t.Fatalf("wrong todo audience: %+v", broadcaster) }
  })
 }
}
