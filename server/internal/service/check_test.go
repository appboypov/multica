package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func checkFixture(t *testing.T) (*questionFixture, CheckInput) {
	t.Helper()
	f := newQuestionFixture(t)
	project := util.MustParseUUID(f.fx.Project(t, "Checks"))
	f.fx.Exec(t, "UPDATE issue SET project_id=$2 WHERE id=$1", f.issue, project)
	f.fx.Cleanup(t, "DELETE FROM todo WHERE workspace_id=$1", f.workspace)
	f.fx.Cleanup(t, "DELETE FROM project_check_trust WHERE workspace_id=$1", f.workspace)
	f.fx.Cleanup(t, "DELETE FROM check_settings WHERE workspace_id=$1", f.workspace)
	return f, CheckInput{IssueID: f.issue, ProjectID: project, MemberID: f.brian, Kind: "feature", Link: "https://staging.example.test/feature", Title: "Check invoice totals"}
}

func openCheck(t *testing.T, f *questionFixture, in CheckInput) pgtype.UUID {
	t.Helper()
	row, err := f.svc.OpenCheck(context.Background(), f.workspace, f.asker, in)
	if err != nil {
		t.Fatal(err)
	}
	return util.MustParseUUID(row["id"].(string))
}
func answerCheck(t *testing.T, f *questionFixture, id pgtype.UUID, answer string) {
	t.Helper()
	if _, err := f.svc.AnswerCheck(context.Background(), f.workspace, id, f.answerer, answer, "The total needs rounding"); err != nil {
		t.Fatal(err)
	}
}
func trustState(t *testing.T, f *questionFixture, in CheckInput) CheckTrust {
	t.Helper()
	rows, err := f.svc.ListCheckTrust(context.Background(), f.workspace, in.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Kind == in.Kind {
			return row
		}
	}
	t.Fatal("missing trust")
	return CheckTrust{}
}

func TestCheckManualSamplingAndHistory(t *testing.T) {
	ctx := context.Background()
	f, in := checkFixture(t)
	settings, err := f.svc.CheckSettings(ctx, f.workspace, f.answerer, nil)
	if err != nil || settings != (CheckSettings{5}) { t.Fatalf("defaults: %+v %v",settings,err) }
	for range 10 { answerCheck(t,f,openCheck(t,f,in),"pass") }
	if state:=trustState(t,f,in);state.Sampling || state.Passed!=10 {t.Fatalf("mode must stay checked: %+v",state)}
	if got:=f.fx.Count(t,"SELECT count(*) FROM todo WHERE workspace_id=$1",f.workspace);got!=10 {t.Fatalf("unexpected to-dos: %d",got)}
	if _,err:=f.svc.SetCheckSampling(ctx,f.workspace,in.ProjectID,in.Kind,f.other,true);err==nil {t.Fatal("other member changed mode")}
	if _,err:=f.svc.SetCheckSampling(ctx,f.workspace,in.ProjectID,in.Kind,f.answerer,true);err!=nil {t.Fatal(err)}
	for i:=1;i<=5;i++ {
		id:=openCheck(t,f,in)
		row,err:=f.svc.GetCheck(ctx,f.workspace,id,f.asker);if err!=nil{t.Fatal(err)}
		var details CheckDetails
		if err:=json.Unmarshal(row["details"].(json.RawMessage),&details);err!=nil{t.Fatal(err)}
		if i<5 {
			if details.Answer!="pass"||!details.Automatic||row["done_at"]==nil {t.Fatalf("automatic: %+v",row)}
		} else {
			if !details.Sampled||details.Automatic||row["done_at"]!=nil {t.Fatalf("sample: %+v",row)}
			answerCheck(t,f,id,"feedback")
		}
	}
	if state:=trustState(t,f,in);!state.Sampling||state.Feedback!=1||state.AutomaticPassed!=4 {t.Fatalf("feedback must not switch mode: %+v",state)}
	history,err:=f.svc.CheckHistory(ctx,f.workspace,in.ProjectID,in.Kind)
	if err!=nil||len(history)!=15{t.Fatalf("history: %+v %v",history,err)}
	if _,err:=f.svc.SetCheckSampling(ctx,f.workspace,in.ProjectID,in.Kind,f.answerer,false);err!=nil{t.Fatal(err)}
	row,err:=f.svc.GetCheck(ctx,f.workspace,openCheck(t,f,in),f.asker)
	if err!=nil||row["done_at"]!=nil{t.Fatalf("every piece: %+v %v",row,err)}
}

func TestCheckReleaseAlwaysNeedsMember(t *testing.T) {
	f,in:=checkFixture(t);in.Kind="release"
	for range 7 {answerCheck(t,f,openCheck(t,f,in),"pass")}
	if state:=trustState(t,f,in);state.Sampling||state.Passed!=7{t.Fatalf("release: %+v",state)}
	_,err:=f.svc.SetCheckSampling(context.Background(),f.workspace,in.ProjectID,in.Kind,f.answerer,true)
	wantQuestionRefusal(t,err,422)
}

func TestCheckAnswerIsOnceAndRecipientOnly(t *testing.T) {
	f, in := checkFixture(t)
	ctx := context.Background()
	id := openCheck(t, f, in)
	_, err := f.svc.AnswerCheck(ctx, f.workspace, id, f.other, "pass", "")
	wantQuestionRefusal(t, err, 404)
	_, err = f.svc.AnswerCheck(ctx, f.workspace, id, f.answerer, "feedback", "")
	wantQuestionRefusal(t, err, 422)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.svc.AnswerCheck(ctx, f.workspace, id, f.answerer, "pass", "")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if state := trustState(t, f, in); state.Passed != 1 {
		t.Fatalf("counted twice: %+v", state)
	}
	if got := f.fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND status='queued'", f.issue); got != 1 {
		t.Fatalf("wake count %d", got)
	}
	var handoff string
	f.fx.QueryRow(t, "SELECT handoff_note FROM agent_task_queue WHERE issue_id=$1 AND status='queued'", f.issue).Scan(&handoff)
	if handoff == "" {
		t.Fatal("wake must carry the answer")
	}
}

func TestCheckValidatesRelationshipsAndSettings(t *testing.T) {
	f, in := checkFixture(t)
	ctx := context.Background()
	for _, kind := range []string{"data_model", "data_flow", "design", "prototype", "feature", "improvement", "agent_tool", "release"} {
		if !validCheckKind(kind) {
			t.Fatal(kind)
		}
	}
	for name, change := range map[string]func(*CheckInput){"kind": func(i *CheckInput) { i.Kind = "fix" }, "link": func(i *CheckInput) { i.Link = "javascript:alert(1)" }, "title": func(i *CheckInput) { i.Title = " " }, "project": func(i *CheckInput) { i.ProjectID = util.MustParseUUID(f.fx.Project(t, "Other")) }} {
		t.Run(name, func(t *testing.T) {
			input := in
			change(&input)
			if _, err := f.svc.OpenCheck(ctx, f.workspace, f.asker, input); err == nil {
				t.Fatal("accepted invalid check")
			}
		})
	}
	_, err := f.svc.CheckSettings(ctx, f.workspace, f.other, &CheckSettings{2})
	wantQuestionRefusal(t, err, 403)
	_, err = f.svc.CheckSettings(ctx, f.workspace, f.answerer, &CheckSettings{0})
	wantQuestionRefusal(t, err, 422)
	openCheck(t, f, in)
	in.MemberID = f.anouk
	_, err = f.svc.OpenCheck(ctx, f.workspace, f.asker, in)
	wantQuestionRefusal(t, err, 422)
}

func TestCheckDeliversAnswerToOpeningAgentRun(t *testing.T) {
	f, in := checkFixture(t)
	f.fx.Exec(t, "INSERT INTO task_supplement_capability(task_id,workspace_id,issue_id,capability) VALUES($1,$2,$3,'task-supplement-v1')", f.task, f.workspace, f.issue)
	f.fx.Cleanup(t, "DELETE FROM task_supplement_capability WHERE task_id=$1", f.task)
	f.fx.Cleanup(t, "DELETE FROM task_supplement WHERE task_id=$1", f.task)
	id := openCheck(t, f, in)
	answerCheck(t, f, id, "feedback")
	if got := f.fx.Count(t, "SELECT count(*) FROM task_supplement WHERE task_id=$1 AND item_id=$2 AND item_kind='answer'", f.task, id); got != 1 {
		t.Fatalf("receipt count %d", got)
	}
	if got := f.fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE issue_id=$1", f.issue); got != 1 {
		t.Fatalf("active run should receive rather than wake: %d", got)
	}
}

func TestCheckSampleSchedule(t *testing.T) {
	for i := range 10 {
		if got, want := checkSample(true, i, 5), (i+1)%5 == 0; got != want {
			t.Fatal(fmt.Sprint(i, got, want))
		}
	}
	if checkSample(false, 4, 5) {
		t.Fatal("checked work is not a sample")
	}
}

func TestCheckDeleteParentData(t *testing.T) {
	for _, scope := range []string{"project", "workspace"} {
		t.Run(scope, func(t *testing.T) {
			f, in := checkFixture(t)
			ctx := context.Background()
			openCheck(t, f, in)
			if _, err := f.svc.CheckSettings(ctx, f.workspace, f.answerer, &CheckSettings{4}); err != nil {
				t.Fatal(err)
			}
			project := in.ProjectID
			if scope == "workspace" {
				project = pgtype.UUID{}
			}
			tx, err := f.fx.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if err := DeleteCheckData(ctx, tx, f.workspace, project); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if got := f.fx.Count(t, "SELECT count(*) FROM project_check_trust WHERE workspace_id=$1", f.workspace); got != 0 {
				t.Fatalf("trust rows: %d", got)
			}
			wantSettings := 1
			if scope == "workspace" {
				wantSettings = 0
			}
			if got := f.fx.Count(t, "SELECT count(*) FROM check_settings WHERE workspace_id=$1", f.workspace); got != wantSettings {
				t.Fatalf("settings rows: %d", got)
			}
			if scope == "project" {
				if got := f.fx.Count(t, "SELECT count(*) FROM todo WHERE workspace_id=$1 AND kind='check'", f.workspace); got != 0 {
					t.Fatalf("project check rows: %d", got)
				}
			}
		})
	}
}

func TestCheckAnswerWaitsBehindDispatchedRun(t *testing.T) {
	ctx := context.Background()
	f, in := checkFixture(t)
	f.fx.Exec(t, "UPDATE agent_task_queue SET status='dispatched', dispatched_at=now(), started_at=NULL WHERE id=$1", f.task)
	first := openCheck(t, f, in)
	answerCheck(t, f, first, "feedback")
	var claimed pgtype.Text
	f.fx.QueryRow(t, "SELECT handoff_note FROM agent_task_queue WHERE id=$1", f.task).Scan(&claimed)
	if claimed.Valid {
		t.Fatalf("the dispatched run's prompt is already built: %q", claimed.String)
	}
	second := openCheck(t, f, in)
	answerCheck(t, f, second, "pass")
	if got := f.fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND id<>$2", f.issue, f.task); got != 1 {
		t.Fatalf("both answers belong in one successor run, got %d runs", got)
	}
	var successor pgtype.UUID
	var status, handoff string
	f.fx.QueryRow(t, "SELECT id, status, handoff_note FROM agent_task_queue WHERE issue_id=$1 AND id<>$2", f.issue, f.task).Scan(&successor, &status, &handoff)
	if status != "deferred" || !strings.Contains(handoff, util.UUIDToString(first)) || !strings.Contains(handoff, "multica todo instruction-note") || !strings.Contains(handoff, util.UUIDToString(second)) {
		t.Fatalf("successor %s must carry both answers: %q", status, handoff)
	}
	promoted, err := f.svc.Queries.PromoteDueDeferredTasksForRuntime(ctx, db.PromoteDueDeferredTasksForRuntimeParams{RuntimeID: f.runtime, RuntimeStaleSecs: 300})
	if err != nil || len(promoted) != 0 {
		t.Fatalf("the successor waits while the dispatched run holds the slot: %+v %v", promoted, err)
	}
	f.fx.Exec(t, "UPDATE agent_task_queue SET status='running', started_at=now() WHERE id=$1", f.task)
	promoted, err = f.svc.Queries.PromoteDueDeferredTasksForRuntime(ctx, db.PromoteDueDeferredTasksForRuntimeParams{RuntimeID: f.runtime, RuntimeStaleSecs: 300})
	if err != nil || len(promoted) != 1 || promoted[0].ID != successor || promoted[0].Status != "queued" {
		t.Fatalf("the successor queues once the slot frees: %+v %v", promoted, err)
	}
}

func TestCheckPathsHoldOneConnection(t *testing.T) {
	f, in := checkFixture(t)
	check := openCheck(t, f, in)
	answerCheck(t, f, check, "feedback")
	cfg := f.fx.Pool.Config()
	cfg.MaxConns, cfg.MinConns = 1, 0
	one, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(one.Close)
	svc := *f.svc
	svc.Queries, svc.TxStarter = db.New(one), one
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	must := func(step string) func(map[string]any, error) map[string]any {
		return func(row map[string]any, err error) map[string]any {
			t.Helper()
			if err != nil {
				t.Fatalf("%s: %v", step, err)
			}
			return row
		}
	}
	must("read")(svc.GetCheck(ctx, f.workspace, check, f.asker))
	must("answer retry")(svc.AnswerCheck(ctx, f.workspace, check, f.answerer, "pass", ""))
	input := InstructionNoteInput{File: "skills/totals.md", Link: "https://example.test/commit", Note: "Round totals before display"}
	note := util.MustParseUUID(must("note")(svc.OpenInstructionNote(ctx, f.workspace, check, f.asker, input))["id"].(string))
	must("note retry")(svc.OpenInstructionNote(ctx, f.workspace, check, f.asker, input))
	must("undo")(f.svc.AnswerCheck(ctx, f.workspace, note, f.answerer, "undo", ""))
	must("undo retry")(svc.AnswerCheck(ctx, f.workspace, note, f.answerer, "undo", ""))
	report := InstructionNoteInput{Link: "https://example.test/revert", Note: "Reverted"}
	must("undo result")(svc.ReportInstructionUndo(ctx, f.workspace, note, f.asker, report))
	must("undo result retry")(svc.ReportInstructionUndo(ctx, f.workspace, note, f.asker, report))
	must("keep")(svc.AnswerCheck(ctx, f.workspace, note, f.answerer, "keep", ""))
	must("keep retry")(svc.AnswerCheck(ctx, f.workspace, note, f.answerer, "keep", ""))
	if _, err := svc.CheckSettings(ctx, f.workspace, f.answerer, &CheckSettings{4}); err != nil {
		t.Fatalf("settings: %v", err)
	}
}
