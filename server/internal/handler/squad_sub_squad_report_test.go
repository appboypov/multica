package handler

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

// A sub-squad leader's report reaches the main squad leader that delegated to
// it (ADR-0017): the main leader @mentions the sub-squad in a leader run, the
// sub-squad's leader runs in its own leader role and posts its report.
func TestCreateComment_SubSquadReportWakesDelegatingLeader(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	cases := []struct {
		name            string
		assignToMain    bool
		delegate        bool
		reportMentions  bool
		wantMainLeader  int
		wantMentionedID bool
	}{
		{name: "guest main squad", delegate: true, wantMainLeader: 1},
		{name: "assigned main squad wakes once", assignToMain: true, delegate: true, wantMainLeader: 1},
		{name: "no delegation wakes nobody", wantMainLeader: 0},
		{name: "report with mention wakes only the mentioned", delegate: true, reportMentions: true, wantMainLeader: 0, wantMentionedID: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mainLeaderID := dbfx.Agent(t, "Sub report main leader "+tc.name, testRuntimeID)
			subLeaderID := dbfx.Agent(t, "Sub report sub leader "+tc.name, testRuntimeID)
			workerID := dbfx.Agent(t, "Sub report worker "+tc.name, testRuntimeID)
			mainID := dbfx.Squad(t, "Sub report main "+tc.name, mainLeaderID)
			subID := dbfx.Squad(t, "Sub report sub "+tc.name, subLeaderID)
			dbfx.SquadMember(t, mainID, "squad", subID)
			dbfx.SquadMember(t, subID, "agent", workerID)

			assignee := testutil.Cols{"assignee_type": "member", "assignee_id": testUserID}
			if tc.assignToMain {
				assignee = testutil.Cols{"assignee_type": "squad", "assignee_id": mainID}
			}
			issueID := dbfx.Issue(t, "sub-squad report "+tc.name, assignee)
			dbfx.Cleanup(t, `DELETE FROM comment WHERE issue_id = $1`, issueID)
			dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE issue_id = $1`, issueID)

			mainTaskID := dbfx.Task(t, mainLeaderID, testutil.Cols{
				"runtime_id": testRuntimeID, "issue_id": issueID, "status": "completed",
				"is_leader_task": true, "squad_id": mainID,
				"originator_user_id": testUserID, "accountable_user_id": testUserID,
			})
			handoff := "please look at this"
			if tc.delegate {
				handoff = "[@Sub](mention://squad/" + subID + ") please build this"
			}
			handoffID := dbfx.Comment(t, issueID, handoff, testutil.Cols{
				"author_type": "agent", "author_id": mainLeaderID, "source_task_id": mainTaskID,
			})
			subTaskID := dbfx.Task(t, subLeaderID, testutil.Cols{
				"runtime_id": testRuntimeID, "issue_id": issueID, "status": "running",
				"is_leader_task": true, "squad_id": subID, "trigger_comment_id": handoffID,
				"originator_user_id": testUserID, "accountable_user_id": testUserID,
			})

			report := "the backend part is done"
			if tc.reportMentions {
				report = "[@Worker](mention://agent/" + workerID + ") please add the tests"
			}
			r := newRequest(http.MethodPost, "/api/issues/"+issueID+"/comments", map[string]any{"content": report, "parent_id": handoffID})
			r.Header.Set("X-Agent-ID", subLeaderID)
			r.Header.Set("X-Task-ID", subTaskID)
			r = withURLParam(r, "id", issueID)
			testutil.Call(t, testHandler.CreateComment, r).Want(http.StatusCreated)

			if got := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND agent_id = $2 AND status = 'queued' AND is_leader_task = TRUE AND squad_id = $3`, issueID, mainLeaderID, mainID); got != tc.wantMainLeader {
				t.Fatalf("main squad leader received %d leader task(s), want %d", got, tc.wantMainLeader)
			}
			wantWorker := 0
			if tc.wantMentionedID {
				wantWorker = 1
			}
			if got := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND agent_id = $2 AND status = 'queued'`, issueID, workerID); got != wantWorker {
				t.Fatalf("mentioned worker received %d task(s), want %d", got, wantWorker)
			}
		})
	}
}
