package handler

import (
 "net/http"
 "testing"

 "github.com/multica-ai/multica/server/internal/testutil"
)

func TestCheckAPIOpenAnswerAndTrust(t *testing.T){
 fx:=newQuestionAPIFixture(t)
 project:=dbfx.Project(t,"Check API")
 dbfx.Exec(t,"UPDATE issue SET project_id=$2 WHERE id=$1",fx.IssueID,project)
 dbfx.Cleanup(t,"DELETE FROM todo WHERE details->>'project_id'=$1",project)
 dbfx.Cleanup(t,"DELETE FROM project_check_trust WHERE project_id=$1",project)
 body:=map[string]string{"issue_id":fx.IssueID,"project_id":project,"member_id":testUserID,"kind":"design","link":"https://example.test/design","title":"Invoice design"}
 testutil.Call(t,testHandler.OpenCheck,newRequest("POST","/api/todos/checks",body)).Want(http.StatusForbidden)
 var row map[string]any
 testutil.Call(t,testHandler.OpenCheck,fx.agentRequest("POST","",body)).Want(http.StatusCreated).JSON(&row)
 id:=row["id"].(string)
 if row["kind"]!="check"{t.Fatalf("todo %+v",row)}
 testutil.Call(t,testHandler.MarkTodoDone,withURLParam(newRequest("POST","/",nil),"id",id)).Want(http.StatusUnprocessableEntity)
 testutil.Call(t,testHandler.AnswerCheck,fx.agentRequest("POST",id,map[string]string{"answer":"pass"})).Want(http.StatusForbidden)
 testutil.Call(t,testHandler.AnswerCheck,withURLParam(newRequest("POST","/",map[string]string{"answer":"feedback"}),"id",id)).Want(http.StatusUnprocessableEntity)
 testutil.Call(t,testHandler.AnswerCheck,withURLParam(newRequest("POST","/",map[string]string{"answer":"pass"}),"id",id)).Want(http.StatusOK).JSON(&row)
 if row["done_by"]!="member"{t.Fatalf("answer %+v",row)}
 var states []map[string]any
 testutil.Call(t,testHandler.ListCheckTrust,withURLParam(newRequest("GET","/",nil),"id",project)).Want(http.StatusOK).JSON(&states)
 if len(states)!=1||states[0]["passed"]!=float64(1)||states[0]["automatic"]!=false{t.Fatalf("trust %+v",states)}
 body["member_id"]="not-a-uuid"
 testutil.Call(t,testHandler.OpenCheck,fx.agentRequest("POST","",body)).Want(http.StatusBadRequest)
}
