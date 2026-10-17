package service

import (
 "context"
 "encoding/json"
 "strings"
 "testing"
 "github.com/multica-ai/multica/server/internal/util"
)

func TestCheckInstructionNoteUndoLifecycle(t *testing.T) {
 ctx:=context.Background();f,in:=checkFixture(t)
 check:=openCheck(t,f,in);answerCheck(t,f,check,"feedback")
 var handoff string
 f.fx.QueryRow(t,"SELECT handoff_note FROM agent_task_queue WHERE issue_id=$1 AND status='queued'",f.issue).Scan(&handoff)
 if !strings.Contains(handoff,"multica todo instruction-note"){t.Fatalf("feedback wake: %s",handoff)}
 input:=InstructionNoteInput{File:"skills/totals.md",Link:"https://example.test/commit",Note:"Round totals before display"}
 row,err:=f.svc.OpenInstructionNote(ctx,f.workspace,check,f.asker,input);if err!=nil{t.Fatal(err)}
 id:=util.MustParseUUID(row["id"].(string))
 again,err:=f.svc.OpenInstructionNote(ctx,f.workspace,check,f.asker,input)
 if err!=nil||again["id"]!=row["id"]{t.Fatalf("duplicate: %+v %v",again,err)}
 _,err=f.svc.AnswerCheck(ctx,f.workspace,id,f.other,"undo","");wantQuestionRefusal(t,err,404)
 _,err=f.svc.ReportInstructionUndo(ctx,f.workspace,id,f.asker,InstructionNoteInput{Link:input.Link,Note:"Reverted"});wantQuestionRefusal(t,err,422)
 row,err=f.svc.AnswerCheck(ctx,f.workspace,id,f.answerer,"undo","");if err!=nil{t.Fatal(err)}
 if row["done_at"]!=nil{t.Fatal("undo must remain open")}
 var details InstructionNoteDetails
 if err:=json.Unmarshal(row["details"].(json.RawMessage),&details);err!=nil{t.Fatal(err)}
 if !details.UndoRequested||details.UndoNote!=""{t.Fatalf("undo requested: %+v",details)}
 f.fx.QueryRow(t,"SELECT handoff_note FROM agent_task_queue WHERE issue_id=$1 AND status='queued'",f.issue).Scan(&handoff)
 if !strings.Contains(handoff,"multica todo undo-result"){t.Fatalf("undo wake: %s",handoff)}
 before:=handoff
 if _,err:=f.svc.AnswerCheck(ctx,f.workspace,id,f.answerer,"undo","");err!=nil{t.Fatal(err)}
 f.fx.QueryRow(t,"SELECT handoff_note FROM agent_task_queue WHERE issue_id=$1 AND status='queued'",f.issue).Scan(&handoff)
 if handoff!=before{t.Fatal("undo woke twice")}
 row,err=f.svc.ReportInstructionUndo(ctx,f.workspace,id,f.asker,InstructionNoteInput{Link:"https://example.test/revert",Note:"Reverted the rounding instruction"});if err!=nil{t.Fatal(err)}
 if row["done_at"]!=nil{t.Fatal("report must await Keep")}
 if err:=json.Unmarshal(row["details"].(json.RawMessage),&details);err!=nil{t.Fatal(err)}
 if details.UndoNote!="Reverted the rounding instruction"||details.UndoLink!="https://example.test/revert"{t.Fatalf("result: %+v",details)}
 row,err=f.svc.AnswerCheck(ctx,f.workspace,id,f.answerer,"keep","")
 if err!=nil||row["done_at"]==nil{t.Fatalf("keep: %+v %v",row,err)}
}

func TestCheckInstructionNoteValidation(t *testing.T) {
 ctx:=context.Background();f,in:=checkFixture(t)
 check:=openCheck(t,f,in);answerCheck(t,f,check,"pass")
 input:=InstructionNoteInput{File:"skills/totals.md",Link:"https://example.test/commit",Note:"Round totals"}
 _,err:=f.svc.OpenInstructionNote(ctx,f.workspace,check,f.asker,input);wantQuestionRefusal(t,err,422)
 check=openCheck(t,f,in);answerCheck(t,f,check,"feedback")
 input.Link="javascript:alert(1)"
 _,err=f.svc.OpenInstructionNote(ctx,f.workspace,check,f.asker,input);wantQuestionRefusal(t,err,422)
 input.Link="https://example.test/commit";input.File=" "
 _,err=f.svc.OpenInstructionNote(ctx,f.workspace,check,f.asker,input);wantQuestionRefusal(t,err,422)
}
