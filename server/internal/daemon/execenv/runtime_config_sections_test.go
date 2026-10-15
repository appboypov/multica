package execenv

import (
	"strings"
	"testing"
)

// TestQuestionsSectionFollowsTheClaim pins where the brief writes the
// Questions section: an issue run or an in-app chat turn whose claim has
// questions on gets it word for word; a run with questions off and a
// quick-create run never do (ADR-0017).
func TestQuestionsSectionFollowsTheClaim(t *testing.T) {
	const section = "## Questions\n\n" +
		"`multica question` asks people one or more questions at the same time and reads their answers.\n\n" +
		"`multica question --help` lists its commands, answer types and options.\n\n"
	for _, tc := range []struct {
		name string
		ctx  TaskContextForEnv
		want bool
	}{
		{"assignment", TaskContextForEnv{IssueID: "issue-1", QuestionsEnabled: true}, true},
		{"comment", TaskContextForEnv{IssueID: "issue-1", TriggerCommentID: "comment-1", QuestionsEnabled: true}, true},
		{"autopilot on issue", TaskContextForEnv{IssueID: "issue-1", AutopilotRunID: "run-1", QuestionsEnabled: true}, true},
		{"issue run with questions off", TaskContextForEnv{IssueID: "issue-1"}, false},
		{"autopilot without issue", TaskContextForEnv{AutopilotRunID: "run-1"}, false},
		{"chat with questions on", TaskContextForEnv{ChatSessionID: "chat-1", QuestionsEnabled: true}, true},
		{"chat with questions off", TaskContextForEnv{ChatSessionID: "chat-1"}, false},
		{"chat with linked issue and questions on", TaskContextForEnv{IssueID: "issue-1", ChatSessionID: "chat-1", QuestionsEnabled: true}, true},
		{"chat with linked issue and questions off", TaskContextForEnv{IssueID: "issue-1", ChatSessionID: "chat-1"}, false},
		{"quick create", TaskContextForEnv{QuickCreatePrompt: "Create an issue"}, false},
		{"quick create claimed with questions on", TaskContextForEnv{QuickCreatePrompt: "Create an issue", QuestionsEnabled: true}, false},
		{"no issue", TaskContextForEnv{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := buildMetaSkillContent("codex", tc.ctx)
			if got := strings.Contains(out, section); got != tc.want {
				t.Fatalf("Questions section present=%v want=%v", got, tc.want)
			}
			if !tc.want && strings.Contains(out, "## Questions\n") {
				t.Fatal("Questions heading leaked into a run whose claim has questions off")
			}
			if tc.want && strings.Count(out, "## Questions\n") != 1 {
				t.Fatal("Questions section was duplicated")
			}
		})
	}
}
