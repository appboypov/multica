package daemon

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestReleasePromptConsumesNativeScopeBeforeCommentWakeupAndQuickCreate(t *testing.T) {
	task := Task{ReleaseID: "release-1", WakeupID: "wait-1", QuickCreatePrompt: "must not create", TriggerCommentID: "comment-2", DeliveredCommentIDs: []string{"comment-1", "comment-2"}, HandoffNote: "WORK-502 reached review", Release: &protocol.ClaimRelease{ID: "release-1", RepositoryName: "owner/repo", Version: "1.1.0", Title: "Account", Issues: json.RawMessage(`[{"identifier":"WORK-501","status":"done"},{"identifier":"WORK-502","status":"in_review"}]`), RemainingOpen: 1}, ReleaseTriggers: json.RawMessage(`[{"event_type":"release.comment_created","actor_id":"member-1"}]`), CoalescedComments: []CoalescedCommentData{{ID: "comment-2", ThreadID: "comment-1", AuthorType: "member", AuthorName: "Brian", Content: "Wait for review"}}}
	prompt := BuildPrompt(task, "fake-runtime")
	for _, want := range []string{"owner/repo", "WORK-501", "WORK-502", "member-1", "Brian", "Wait for review", "comment-1, comment-2"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("missing consumed context %q: %s", want, prompt)
		}
	}
	if strings.Index(prompt, "WORK-501") > strings.Index(prompt, "WORK-502") {
		t.Fatal("release issue order changed")
	}
	for _, forbidden := range []string{"must not create"} {
		if strings.Contains(prompt, forbidden) {
			t.Errorf("release fell through to %q", forbidden)
		}
	}
}
