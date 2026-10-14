package main

import (
	"context"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func registerIssueWorktreeListeners(bus *events.Bus, h *handler.Handler) {
	bus.Subscribe(protocol.EventIssueUpdated, func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		changed, _ := payload["status_changed"].(bool)
		issue, ok := payload["issue"].(handler.IssueResponse)
		if changed && ok {
			h.SyncIssueWorktreeStatus(context.Background(), issue)
		}
	})
}
