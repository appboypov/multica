package scheduler

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/events"
	dbfx "github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestQuestionOverdueJobStampsOpenPastDueQuestionsOnce(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("question overdue regression requires an explicit test DATABASE_URL")
	}
	pool := integrationPool(t)
	ctx := context.Background()
	// The local calendar is October 15, but UTC is still October 14. The
	// due-today question must not be notified on the local calendar boundary.
	planTime := time.Date(2026, time.October, 15, 0, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60))
	today := planTime.UTC().Truncate(24 * time.Hour)
	yesterday := today.AddDate(0, 0, -1)
	previousStamp := today.Add(-time.Hour)

	type questionCase struct {
		name        string
		status      string
		due         *time.Time
		stamp       *time.Time
		wantNotice  bool
		workspaceID string
		issueID     string
		id          string
		askID       string
		group       string
		position    int
	}
	cases := []questionCase{
		{name: "first open question due yesterday", status: "open", due: &yesterday, wantNotice: true, group: "three overdue"},
		{name: "second open question due yesterday", status: "open", due: &yesterday, wantNotice: true, group: "three overdue", position: 1},
		{name: "third open question due yesterday", status: "open", due: &yesterday, wantNotice: true, group: "three overdue", position: 2},
		{name: "answered first question due yesterday", status: "answered", due: &yesterday, group: "partly overdue"},
		{name: "partly answered second question due yesterday", status: "partly_answered", due: &yesterday, wantNotice: true, group: "partly overdue", position: 1},
		{name: "withdrawn due yesterday", status: "withdrawn", due: &yesterday},
		{name: "closed due yesterday", status: "closed", due: &yesterday},
		{name: "open due today", status: "open", due: &today},
		{name: "open without due date", status: "open"},
		{name: "already notified", status: "open", due: &yesterday, stamp: &previousStamp},
		{name: "open due yesterday in another workspace", status: "open", due: &yesterday, wantNotice: true},
	}
	workspaceIDs := make(map[string]bool)
	for workspace := range 2 {
		fixture := dbfx.New(pool, "", "")
		fixture.UserID = fixture.User(t, "Question answerer", uuid.NewString()+"@example.test")
		fixture.WorkspaceID = fixture.Workspace(t, "Question overdue", "question-overdue-"+uuid.NewString())
		workspaceIDs[fixture.WorkspaceID] = true
		issueID := fixture.Issue(t, "An agent needs an answer")
		agentID := fixture.Agent(t, "Question asker", "")
		taskID := fixture.Task(t, agentID, dbfx.Cols{"issue_id": issueID, "status": "completed", "completed_at": dbfx.Raw("now()")})
		groupAskIDs := make(map[string]string)
		for i := range cases {
			if (workspace == 0 && i == len(cases)-1) || (workspace == 1 && i != len(cases)-1) {
				continue
			}
			c := &cases[i]
			c.workspaceID, c.issueID = fixture.WorkspaceID, issueID
			c.id = uuid.NewString()
			c.askID = c.id
			if c.group != "" {
				if groupAskIDs[c.group] == "" {
					groupAskIDs[c.group] = c.id
				}
				c.askID = groupAskIDs[c.group]
			}
			c.id = fixture.Insert(t, "question", dbfx.Cols{
				"id":                  c.id,
				"ask_id":              c.askID,
				"ask_position":        c.position,
				"workspace_id":        fixture.WorkspaceID,
				"asker_agent_id":      agentID,
				"asked_in_task_id":    taskID,
				"prompt":              c.name,
				"answer_type":         "yes_no",
				"settings":            dbfx.Raw(`'{"why_on_no":false}'::jsonb`),
				"status":              c.status,
				"due_date":            c.due,
				"overdue_notified_at": c.stamp,
			})
			for position, link := range []struct{ kind, id string }{{"issue", issueID}, {"task", taskID}} {
				fixture.InsertNoID(t, "question_link", dbfx.Cols{
					"question_id":  c.id,
					"workspace_id": fixture.WorkspaceID,
					"link_type":    link.kind,
					"link_id":      link.id,
					"position":     position,
				}, "question_id = $1 AND link_type = $2 AND link_id = $3", c.id, link.kind, link.id)
			}
			fixture.InsertNoID(t, "question_answerer", dbfx.Cols{
				"question_id":  c.id,
				"workspace_id": fixture.WorkspaceID,
				"user_id":      fixture.UserID,
			}, "question_id = $1 AND user_id = $2", c.id, fixture.UserID)
		}
	}

	// Roll back the sweep's global writes as well as the fixture stamps; it
	// must not alter questions belonging to other database-backed tests.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin overdue sweep: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	bus := events.New()
	var published []events.Event
	bus.SubscribeAll(func(event events.Event) {
		if workspaceIDs[event.WorkspaceID] {
			published = append(published, event)
		}
	})
	job := QuestionOverdueJob(db.New(tx), bus)
	firstStamps := make(map[string]pgtype.Timestamptz)
	for tick := range 2 {
		if _, err := job.Handler(ctx, HandlerInput{Job: &job, Scope: ScopeGlobal, PlanTime: planTime.Add(time.Duration(tick) * time.Hour)}); err != nil {
			t.Fatalf("overdue tick %d: %v", tick+1, err)
		}
		for _, c := range cases {
			var status string
			var stamp pgtype.Timestamptz
			if err := tx.QueryRow(ctx, `SELECT status, overdue_notified_at FROM question WHERE id = $1 AND workspace_id = $2`, c.id, c.workspaceID).Scan(&status, &stamp); err != nil {
				t.Fatalf("%s: read tick %d: %v", c.name, tick+1, err)
			}
			if status != c.status {
				t.Errorf("%s: status = %s, want %s", c.name, status, c.status)
			}
			if stamp.Valid != (c.wantNotice || c.stamp != nil) {
				t.Errorf("%s: stamped = %v, want %v", c.name, stamp.Valid, c.wantNotice || c.stamp != nil)
			}
			if c.stamp != nil && !stamp.Time.Equal(*c.stamp) {
				t.Errorf("%s: existing stamp changed to %v", c.name, stamp.Time)
			}
			if tick == 0 {
				firstStamps[c.id] = stamp
			} else if first := firstStamps[c.id]; first.Valid != stamp.Valid || !first.Time.Equal(stamp.Time) {
				t.Errorf("%s: stamp changed on second tick", c.name)
			}
		}
		if len(published) != 3 {
			t.Fatalf("tick %d published %d fixture events, want one per overdue ask (3)", tick+1, len(published))
		}
		for _, c := range cases {
			if !c.wantNotice {
				continue
			}
			count := 0
			for _, event := range published {
				payload, ok := event.Payload.(protocol.QuestionEventPayload)
				if !ok {
					t.Fatalf("event payload = %T, want QuestionEventPayload", event.Payload)
				}
				if !slices.Contains(payload.QuestionIDs, c.id) {
					continue
				}
				count++
				if event.Type != protocol.EventQuestionUpdated || event.WorkspaceID != c.workspaceID || event.ActorType != "system" || payload.IssueID != c.issueID || payload.Reason != protocol.QuestionReasonOverdue || payload.AskID != c.askID || payload.QuestionID != c.askID {
					t.Errorf("%s: unexpected overdue event %+v with payload %+v", c.name, event, payload)
				}
				wantIDs := []string{}
				for _, sibling := range cases {
					if sibling.askID == c.askID && sibling.wantNotice {
						wantIDs = append(wantIDs, sibling.id)
					}
				}
				if !slices.Equal(payload.QuestionIDs, wantIDs) {
					t.Errorf("%s: grouped ids = %v, want %v in ask order", c.name, payload.QuestionIDs, wantIDs)
				}
			}
			if count != 1 {
				t.Errorf("%s: notices = %d, want 1", c.name, count)
			}
		}
	}
}
