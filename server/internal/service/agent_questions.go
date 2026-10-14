package service

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// AgentQuestionsSettingKey holds the workspace Questions default in
// workspace.settings. Absent means On.
const AgentQuestionsSettingKey = "agent_questions"

// AgentQuestionsDefault is the workspace Questions default. Only a boolean
// false turns it off: absent, null, malformed settings or any other value
// leave it On.
func AgentQuestionsDefault(settings []byte) bool {
	var values map[string]json.RawMessage
	if json.Unmarshal(settings, &values) != nil {
		return true
	}
	enabled := true
	if json.Unmarshal(values[AgentQuestionsSettingKey], &enabled) != nil {
		return true
	}
	return enabled
}

// ResolveAgentQuestions decides whether a run on an issue may ask questions:
// the issue's own setting when it has one, else the workspace default
// (ADR-0005).
func ResolveAgentQuestions(issue pgtype.Bool, settings []byte) bool {
	if issue.Valid {
		return issue.Bool
	}
	return AgentQuestionsDefault(settings)
}

// SetAgentQuestionsDefault stores the workspace Questions default with a
// server-side merge, so no other workspace setting is replaced.
func SetAgentQuestionsDefault(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID, enabled bool) error {
	raw, err := json.Marshal(map[string]bool{AgentQuestionsSettingKey: enabled})
	if err != nil {
		return err
	}
	return q.MergeWorkspaceSettings(ctx, db.MergeWorkspaceSettingsParams{ID: workspaceID, Patch: raw})
}
