package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/todo"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

type InstructionNoteInput struct {
	File string `json:"file"`
	Link string `json:"link"`
	Note string `json:"note"`
}

type InstructionNoteDetails struct {
	CheckID string `json:"check_id"`
	ProjectID string `json:"project_id"`
	Kind string `json:"check_kind"`
	InstructionNoteInput
	UndoRequested bool `json:"undo_requested"`
	UndoNote string `json:"undo_note"`
	UndoLink string `json:"undo_link"`
}

func instructionReportValid(in InstructionNoteInput, fileRequired bool) bool {
	link, err := url.Parse(in.Link)
	return err == nil && link.Host != "" && (link.Scheme == "http" || link.Scheme == "https") && strings.TrimSpace(in.Note) != "" && (!fileRequired || strings.TrimSpace(in.File) != "")
}

func loadCheckTodo(ctx context.Context, tx pgx.Tx, q *db.Queries, ws, id pgtype.UUID) (db.GetTodoForMemberRow, error) {
	var member pgtype.UUID
	if err := tx.QueryRow(ctx, "SELECT member_id FROM todo WHERE workspace_id=$1 AND id=$2 FOR UPDATE", ws, id).Scan(&member); err != nil { return db.GetTodoForMemberRow{}, questionLoadError(err) }
	return q.GetTodoForMember(ctx, db.GetTodoForMemberParams{ID:id, WorkspaceID:ws, MemberID:member})
}

func (s *QuestionService) OpenInstructionNote(ctx context.Context, ws, checkID pgtype.UUID, actor QuestionActor, in InstructionNoteInput) (map[string]any, error) {
	if !instructionReportValid(in, true) { return nil, questionRefusal(422, "file, summary and HTTP commit link are required") }
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil { return nil, err }
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	task, err := s.requireAgentTask(ctx, q, ws, actor)
	if err != nil { return nil, err }
	var projectID string
	if err := tx.QueryRow(ctx, "SELECT details->>'project_id' FROM todo WHERE workspace_id=$1 AND id=$2", ws, checkID).Scan(&projectID); err != nil { return nil, questionLoadError(err) }
	var parent pgtype.UUID
	if err := tx.QueryRow(ctx, "SELECT id FROM workspace WHERE id=$1 FOR KEY SHARE", ws).Scan(&parent); err != nil { return nil, questionLoadError(err) }
	if err := tx.QueryRow(ctx, "SELECT id FROM project WHERE workspace_id=$1 AND id=$2::uuid FOR KEY SHARE", ws, projectID).Scan(&parent); err != nil { return nil, questionLoadError(err) }
	check, err := loadCheckTodo(ctx, tx, q, ws, checkID)
	if err != nil { return nil, err }
	var details CheckDetails
	if err := json.Unmarshal(check.Details, &details); err != nil { return nil, err }
	if check.Kind != "check" || check.ActorID != actor.ID || task.IssueID != check.IssueID { return nil, questionRefusal(403, "only the opening agent on this issue reports the instruction change") }
	if !check.DoneAt.Valid || details.Answer != "feedback" { return nil, questionRefusal(422, "an instruction note needs an answered feedback check") }
	var existing pgtype.UUID
	err = tx.QueryRow(ctx, "SELECT id FROM todo WHERE workspace_id=$1 AND kind='instruction_change' AND source_key=$2 ORDER BY created_at LIMIT 1", ws, util.UUIDToString(checkID)).Scan(&existing)
	if err == nil { return readCheck(ctx, tx, q, ws, existing, actor) }
	if !errors.Is(err, pgx.ErrNoRows) { return nil, err }
	data, err := json.Marshal(InstructionNoteDetails{CheckID:util.UUIDToString(checkID), ProjectID:details.ProjectID, Kind:details.Kind, InstructionNoteInput:in})
	if err != nil { return nil, err }
	var member pgtype.UUID
	if err := tx.QueryRow(ctx, "SELECT member_id FROM todo WHERE id=$1 AND workspace_id=$2", checkID, ws).Scan(&member); err != nil { return nil, err }
	row, err := q.OpenTodo(ctx, db.OpenTodoParams{ID:dbid.NewV7(), WorkspaceID:ws, MemberID:member, Kind:"instruction_change", SourceKey:util.UUIDToString(checkID), IssueID:check.IssueID, TaskID:actor.TaskID, ActorType:pgtype.Text{String:"agent",Valid:true}, ActorID:actor.ID, Title:"Instruction changed: "+in.File, Details:data})
	if err != nil { return nil, err }
	if err := tx.Commit(ctx); err != nil { return nil, err }
	if err := (todo.Service{Queries:s.Queries, Bus:s.Bus}).PublishOpened(ctx,row); err != nil { return nil, err }
	return s.GetCheck(ctx,ws,row.ID,actor)
}

func (s *QuestionService) AnswerInstructionNote(ctx context.Context, ws, id pgtype.UUID, actor QuestionActor, answer string) (map[string]any, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil { return nil, err }
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	if err := s.requireMember(ctx,q,ws,actor); err != nil { return nil, err }
	row, err := loadCheckTodo(ctx,tx,q,ws,id)
	if err != nil { return nil, err }
	var member pgtype.UUID
	if err := tx.QueryRow(ctx,"SELECT member_id FROM todo WHERE id=$1 AND workspace_id=$2",id,ws).Scan(&member); err != nil { return nil, err }
	if member != actor.ID { return nil, questionRefusal(404,"instruction note not found") }
	if row.Kind != "instruction_change" { return nil, questionRefusal(422,"this to-do is not an instruction note") }
	if row.DoneAt.Valid { return readCheck(ctx,tx,q,ws,id,actor) }
	var details InstructionNoteDetails
	if err := json.Unmarshal(row.Details,&details); err != nil { return nil,err }
	var task db.AgentTaskQueue
	var notified []db.ListRunningSupplementTasksForIssuesRow
	switch answer {
	case "keep":
		_,err=tx.Exec(ctx,"UPDATE todo SET done_at=now(),done_by='member' WHERE id=$1 AND workspace_id=$2",id,ws)
	case "undo":
		if details.UndoRequested { return readCheck(ctx,tx,q,ws,id,actor) }
		details.UndoRequested=true
		data,marshalErr:=json.Marshal(details);if marshalErr!=nil{return nil,marshalErr}
		_,err=tx.Exec(ctx,"UPDATE todo SET details=$3 WHERE id=$1 AND workspace_id=$2",id,ws,data)
		if err!=nil{return nil,err}
		content:=fmt.Sprintf("Brian asks you to Undo the instruction change for check %s. Revert only the change to %s described here: %s\n%s\nReport the result on this note with: multica todo undo-result %s --link <revert-commit-URL> --note <result>. Do not claim success until you have reverted it.",details.CheckID,details.File,details.Note,details.Link,util.UUIDToString(id))
		task,notified,err=s.deliverCheckMessage(ctx,tx,q,ws,row.IssueID,row.ActorID,actor.ID,id,content)
	default:
		return nil,questionRefusal(422,"instruction note answer must be keep or undo")
	}
	if err!=nil{return nil,err}
	if err:=tx.Commit(ctx);err!=nil{return nil,err}
	publisher:=todo.Service{Queries:s.Queries,Bus:s.Bus}
	if answer=="keep" { if err:=publisher.PublishClosed(ctx,[]db.Todo{{ID:id}});err!=nil{return nil,err} } else {
		if err:=publisher.PublishOpened(ctx,db.Todo{ID:id,WorkspaceID:ws,MemberID:member});err!=nil{return nil,err}
	}
	s.publishCheckDelivery(ctx,task,notified)
	return s.GetCheck(ctx,ws,id,actor)
}

func (s *QuestionService) ReportInstructionUndo(ctx context.Context, ws,id pgtype.UUID, actor QuestionActor,in InstructionNoteInput) (map[string]any,error) {
	if !instructionReportValid(in,false){return nil,questionRefusal(422,"summary and HTTP commit link are required")}
	tx,err:=s.TxStarter.Begin(ctx);if err!=nil{return nil,err};defer tx.Rollback(ctx)
	q:=s.Queries.WithTx(tx)
	task,err:=s.requireAgentTask(ctx,q,ws,actor);if err!=nil{return nil,err}
	row,err:=loadCheckTodo(ctx,tx,q,ws,id);if err!=nil{return nil,err}
	if row.Kind!="instruction_change"||row.ActorID!=actor.ID||task.IssueID!=row.IssueID{return nil,questionRefusal(403,"only the opening agent on this issue reports the undo")}
	var details InstructionNoteDetails
	if err:=json.Unmarshal(row.Details,&details);err!=nil{return nil,err}
	if !details.UndoRequested{return nil,questionRefusal(422,"undo was not requested")}
	if details.UndoNote!=""{return readCheck(ctx,tx,q,ws,id,actor)}
	if row.DoneAt.Valid{return nil,questionRefusal(422,"instruction note is closed")}
	details.UndoNote,details.UndoLink=in.Note,in.Link
	data,err:=json.Marshal(details);if err!=nil{return nil,err}
	if _,err:=tx.Exec(ctx,"UPDATE todo SET details=$3 WHERE id=$1 AND workspace_id=$2",id,ws,data);err!=nil{return nil,err}
	if err:=tx.Commit(ctx);err!=nil{return nil,err}
	if err:=(todo.Service{Queries:s.Queries,Bus:s.Bus}).PublishOpened(ctx,db.Todo{ID:id,WorkspaceID:ws});err!=nil{return nil,err}
	return s.GetCheck(ctx,ws,id,actor)
}

func (s *QuestionService) deliverCheckMessage(ctx context.Context, tx pgx.Tx,q *db.Queries,ws,issueID,agentID,authorID,itemID pgtype.UUID,content string) (db.AgentTaskQueue,[]db.ListRunningSupplementTasksForIssuesRow,error) {
	var task db.AgentTaskQueue
	if s.Tasks==nil{return task,nil,questionRefusal(503,"agent delivery is unavailable")}
	issue,err:=q.LockIssueForDescriptionUpdate(ctx,db.LockIssueForDescriptionUpdateParams{ID:issueID,WorkspaceID:ws});if err!=nil{return task,nil,err}
	active,err:=q.ListRunningSupplementTasksForIssues(ctx,db.ListRunningSupplementTasksForIssuesParams{WorkspaceID:ws,IssueIds:[]pgtype.UUID{issue.ID}});if err!=nil{return task,nil,err}
	var notified []db.ListRunningSupplementTasksForIssuesRow
	for _,run:=range active {
		if run.AgentID!=agentID{continue}
		_,err:=q.CreateRunInputSupplement(ctx,db.CreateRunInputSupplementParams{TaskID:run.ID,IssueID:issue.ID,WorkspaceID:ws,AuthorID:authorID,ItemKind:"answer",ItemID:itemID,Content:pgtype.Text{String:content,Valid:true}})
		if errors.Is(err,pgx.ErrNoRows){continue};if err!=nil{return task,nil,err}
		notified=append(notified,run)
	}
	if len(notified)>0{return task,notified,nil}
	var pending pgtype.UUID
	err=tx.QueryRow(ctx,`SELECT id FROM agent_task_queue WHERE issue_id=$1 AND agent_id=$2 AND status IN ('queued','dispatched') AND comment_thread_id IS NULL ORDER BY created_at LIMIT 1 FOR UPDATE`,issue.ID,agentID).Scan(&pending)
	if err==nil {
		_,err=tx.Exec(ctx,`UPDATE agent_task_queue SET handoff_note=concat_ws(E'\n\n',NULLIF(handoff_note,''),$2::text) WHERE id=$1`,pending,content)
	} else if errors.Is(err,pgx.ErrNoRows) {
		task,err=s.Tasks.enqueueMentionTaskWithQueries(ctx,q,issue,agentID,pgtype.UUID{},nil,false,pgtype.UUID{},false,content,authorID,pgtype.UUID{},OriginNamed,nil)
	}
	return task,nil,err
}

func (s *QuestionService) publishCheckDelivery(ctx context.Context,task db.AgentTaskQueue,notified []db.ListRunningSupplementTasksForIssuesRow) {
	if task.ID.Valid{s.Tasks.BroadcastTaskQueued(ctx,task);s.Tasks.NotifyTaskEnqueued(ctx,task)}
	if s.RunInputs!=nil&&s.RunInputs.Notifier!=nil { for _,run:=range notified{s.RunInputs.Notifier.NotifyTaskSupplementAvailable(util.UUIDToString(run.RuntimeID),util.UUIDToString(run.ID))} }
}
