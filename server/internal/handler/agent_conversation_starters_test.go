package handler

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestNormaliseAgentConversationStarters(t *testing.T) {
	t.Run("trims complete prompts", func(t *testing.T) {
		got, err := normaliseAgentConversationStarters([]AgentConversationStarter{{
			Label:  "  Review a PR  ",
			Prompt: "  Review the open pull request.  ",
		}})
		if err != nil {
			t.Fatalf("normaliseAgentConversationStarters() error = %v", err)
		}
		want := []AgentConversationStarter{{Label: "Review a PR", Prompt: "Review the open pull request."}}
		if len(got) != 1 || got[0] != want[0] {
			t.Fatalf("normaliseAgentConversationStarters() = %#v, want %#v", got, want)
		}
	})

	for name, prompts := range map[string][]AgentConversationStarter{
		"too many": {
			{Label: "One", Prompt: "One"},
			{Label: "Two", Prompt: "Two"},
			{Label: "Three", Prompt: "Three"},
			{Label: "Four", Prompt: "Four"},
		},
		"blank label":  {{Label: " ", Prompt: "Prompt"}},
		"blank prompt": {{Label: "Label", Prompt: " "}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := normaliseAgentConversationStarters(prompts); err == nil {
				t.Fatal("normaliseAgentConversationStarters() error = nil, want validation error")
			}
		})
	}
}

func TestAgentConversationStartersRoundTrip(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	var created AgentResponse
	testutil.Call(t, testHandler.CreateAgent, newRequest(http.MethodPost, "/api/agents", map[string]any{
		"name":       fmt.Sprintf("conversation-starters-%d", time.Now().UnixNano()),
		"runtime_id": handlerTestRuntimeID(t),
		"conversation_starters": []map[string]string{{
			"label":  "  Review a PR  ",
			"prompt": "  Review the most relevant open pull request.  ",
		}},
	})).Want(http.StatusCreated).JSON(&created)
	dbfx.Cleanup(t, `DELETE FROM agent WHERE id = $1`, created.ID)
	if len(created.ConversationStarters) != 1 ||
		created.ConversationStarters[0].Label != "Review a PR" ||
		created.ConversationStarters[0].Prompt != "Review the most relevant open pull request." {
		t.Fatalf("created conversation_starters = %#v", created.ConversationStarters)
	}

	var preserved AgentResponse
	testutil.Call(t, testHandler.UpdateAgent, withURLParam(
		newRequest(http.MethodPut, "/api/agents/"+created.ID, map[string]any{
			"description": "conversation starters unchanged",
		}),
		"id",
		created.ID,
	)).Want(http.StatusOK).JSON(&preserved)
	if len(preserved.ConversationStarters) != 1 {
		t.Fatalf("omitted update conversation_starters = %#v, want preserved prompt", preserved.ConversationStarters)
	}

	var clearedAgent AgentResponse
	testutil.Call(t, testHandler.UpdateAgent, withURLParam(
		newRequest(http.MethodPut, "/api/agents/"+created.ID, map[string]any{
			"conversation_starters": []AgentConversationStarter{},
		}),
		"id",
		created.ID,
	)).Want(http.StatusOK).JSON(&clearedAgent)
	if len(clearedAgent.ConversationStarters) != 0 {
		t.Fatalf("cleared conversation_starters = %#v, want empty", clearedAgent.ConversationStarters)
	}
}

func TestAgentLongFieldsRoundTrip(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	for _, field := range []string{"description", "conversation_starters"} {
		t.Run(field, func(t *testing.T) {
			description := strings.Repeat("界", 1000)
			starter := AgentConversationStarter{
				Label:  strings.Repeat("界", 120),
				Prompt: strings.Repeat("界", 5000),
			}
			payload := map[string]any{
				"name":       fmt.Sprintf("long-fields-%d", time.Now().UnixNano()),
				"runtime_id": handlerTestRuntimeID(t),
			}
			update := map[string]any{}
			if field == "description" {
				payload[field] = description
				update[field] = description + "updated"
			} else {
				payload[field] = []AgentConversationStarter{starter}
				updatedStarter := starter
				updatedStarter.Prompt += "updated"
				update[field] = []AgentConversationStarter{updatedStarter}
			}

			assertFields := func(response AgentResponse, updated bool) {
				t.Helper()
				suffix := ""
				if updated {
					suffix = "updated"
				}
				if field == "description" {
					if response.Description != description+suffix {
						t.Fatalf("description was not returned whole")
					}
				} else if len(response.ConversationStarters) != 1 ||
					response.ConversationStarters[0].Label != starter.Label ||
					response.ConversationStarters[0].Prompt != starter.Prompt+suffix {
					t.Fatalf("conversation starter was not returned whole")
				}
			}

			var created AgentResponse
			testutil.Call(t, testHandler.CreateAgent, newRequest(http.MethodPost, "/api/agents", payload)).
				Want(http.StatusCreated).JSON(&created)
			dbfx.Cleanup(t, `DELETE FROM agent WHERE id = $1`, created.ID)
			assertFields(created, false)

			var updated AgentResponse
			testutil.Call(t, testHandler.UpdateAgent, withURLParam(
				newRequest(http.MethodPut, "/api/agents/"+created.ID, update), "id", created.ID,
			)).Want(http.StatusOK).JSON(&updated)
			assertFields(updated, true)

			var fetched AgentResponse
			testutil.Call(t, testHandler.GetAgent, withURLParam(
				newRequest(http.MethodGet, "/api/agents/"+created.ID, nil), "id", created.ID,
			)).Want(http.StatusOK).JSON(&fetched)
			assertFields(fetched, true)
		})
	}
}

func TestAgentConversationStarterValidation(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	valid := []AgentConversationStarter{
		{Label: "One", Prompt: "One"},
		{Label: "Two", Prompt: "Two"},
		{Label: "Three", Prompt: "Three"},
	}
	var created AgentResponse
	testutil.Call(t, testHandler.CreateAgent, newRequest(http.MethodPost, "/api/agents", map[string]any{
		"name":                  fmt.Sprintf("starter-validation-%d", time.Now().UnixNano()),
		"runtime_id":            handlerTestRuntimeID(t),
		"conversation_starters": valid,
	})).Want(http.StatusCreated).JSON(&created)
	dbfx.Cleanup(t, `DELETE FROM agent WHERE id = $1`, created.ID)

	for name, starters := range map[string][]AgentConversationStarter{
		"four starters": append(append([]AgentConversationStarter{}, valid...), AgentConversationStarter{Label: "Four", Prompt: "Four"}),
		"empty label":   {{Label: "", Prompt: "Prompt"}},
		"blank label":   {{Label: " ", Prompt: "Prompt"}},
		"empty prompt":  {{Label: "Label", Prompt: ""}},
		"blank prompt":  {{Label: "Label", Prompt: " "}},
	} {
		t.Run(name, func(t *testing.T) {
			testutil.Call(t, testHandler.CreateAgent, newRequest(http.MethodPost, "/api/agents", map[string]any{
				"name":                  fmt.Sprintf("invalid-starters-%d", time.Now().UnixNano()),
				"runtime_id":            handlerTestRuntimeID(t),
				"conversation_starters": starters,
			})).Want(http.StatusBadRequest)
			testutil.Call(t, testHandler.UpdateAgent, withURLParam(
				newRequest(http.MethodPut, "/api/agents/"+created.ID, map[string]any{
					"conversation_starters": starters,
				}), "id", created.ID,
			)).Want(http.StatusBadRequest)

			var fetched AgentResponse
			testutil.Call(t, testHandler.GetAgent, withURLParam(
				newRequest(http.MethodGet, "/api/agents/"+created.ID, nil), "id", created.ID,
			)).Want(http.StatusOK).JSON(&fetched)
			if len(fetched.ConversationStarters) != len(valid) {
				t.Fatalf("invalid update changed stored starters: %#v", fetched.ConversationStarters)
			}
			for i := range valid {
				if fetched.ConversationStarters[i] != valid[i] {
					t.Fatalf("invalid update changed stored starter %d", i)
				}
			}
		})
	}
}
