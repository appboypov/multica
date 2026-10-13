package execenv

import (
	"strings"
	"testing"
)

func TestQuestionsSectionForIssueRuns(t *testing.T) {
	const section = "## Questions\n\n" +
		"`multica question` asks people one or more questions at the same time and reads their answers.\n\n" +
		"`multica question --help` lists its commands, answer types and options.\n\n"
	for _, tc := range []struct {
		name string
		ctx  TaskContextForEnv
		want bool
	}{
		{"assignment", TaskContextForEnv{IssueID: "issue-1"}, true},
		{"comment", TaskContextForEnv{IssueID: "issue-1", TriggerCommentID: "comment-1"}, true},
		{"autopilot on issue", TaskContextForEnv{IssueID: "issue-1", AutopilotRunID: "run-1"}, true},
		{"autopilot without issue", TaskContextForEnv{AutopilotRunID: "run-1"}, false},
		{"chat", TaskContextForEnv{ChatSessionID: "chat-1"}, false},
		{"chat with linked issue", TaskContextForEnv{IssueID: "issue-1", ChatSessionID: "chat-1"}, false},
		{"quick create", TaskContextForEnv{QuickCreatePrompt: "Create an issue"}, false},
		{"no issue", TaskContextForEnv{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := buildMetaSkillContent("codex", tc.ctx)
			if got := strings.Contains(out, section); got != tc.want {
				t.Fatalf("Questions section present=%v want=%v", got, tc.want)
			}
			if !tc.want && strings.Contains(out, "## Questions\n") {
				t.Fatal("Questions heading leaked into a task without issue question tooling")
			}
			if tc.want && strings.Count(out, "## Questions\n") != 1 {
				t.Fatal("Questions section was duplicated")
			}
		})
	}
}
