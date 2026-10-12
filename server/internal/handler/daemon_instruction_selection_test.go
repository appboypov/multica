package handler

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/multica-ai/multica/server/internal/daemon"
)

func TestClaimInstructionSelection(t *testing.T) {
	id, runtimeID, _ := startClaimFixture(t, "dispatched")
	task, err := testHandler.Queries.GetAgentTask(context.Background(), parseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := testHandler.Queries.GetAgentRuntime(context.Background(), parseUUID(runtimeID))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/notes/b.md", "/skills/persona", "/notes/b.md"}
	var original []byte
	dbfx.QueryRow(t, `SELECT instruction_selection FROM agent WHERE id=$1`, task.AgentID).Scan(&original)
	t.Cleanup(func() {
		dbfx.Exec(t, `UPDATE agent SET instruction_selection=$2 WHERE id=$1`, task.AgentID, original)
	})
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Exec(t, `UPDATE agent SET instruction_selection=$2 WHERE id=$1`, task.AgentID, encoded)
	req := newDaemonTokenRequest("POST", "/claim", nil, testWorkspaceID, "instruction-selection-claim")
	resp, _, _, _, _, failure := testHandler.buildClaimedTaskResponse(req, &task, runtime, runtimeID, testWorkspaceID)
	if failure != nil {
		t.Fatalf("build claim: %+v", failure)
	}
	if resp.Agent == nil || !reflect.DeepEqual(resp.Agent.InstructionSelection, want) {
		t.Fatalf("claim agent = %+v, want selection %v", resp.Agent, want)
	}
	wire, err := json.Marshal(resp.Agent)
	if err != nil {
		t.Fatal(err)
	}
	var received daemon.AgentData
	if err := json.Unmarshal(wire, &received); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(received.InstructionSelection, want) {
		t.Fatalf("daemon selection = %v, want %v", received.InstructionSelection, want)
	}
}
