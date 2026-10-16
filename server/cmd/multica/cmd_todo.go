package main

import (
 "fmt"
 "net/url"
 "github.com/spf13/cobra"
 "github.com/multica-ai/multica/server/internal/cli"
)

func newTodoCommand() *cobra.Command {
 command:=&cobra.Command{Use:"todo",Short:"Open checks, answer to-dos and read project trust",GroupID:groupCore}
 check:=&cobra.Command{Use:"check",Short:"Ask for a check of work on the run's issue",Long:"Kinds: data_model, data_flow, design, prototype (before dependent work); feature, improvement, agent_tool (on staging after merge); release (beta or production, before release). Releases always need a human pass.",Args:cobra.NoArgs,RunE:runTodoCheck}
 for _,flag:=range []string{"issue","project","answerer","kind","link","title"}{check.Flags().String(flag,"","Check "+flag);_ = check.MarkFlagRequired(flag)}
 check.Flags().String("output","table","Output format: table or json")
 command.AddCommand(check)
 for _,action:=range []string{"get","answer","trust","settings"}{
  action:=action
  use:=action+" <id>";args:=exactArgs(1)
  if action=="settings"{use="settings";args=cobra.NoArgs}
  sub:=&cobra.Command{Use:use,Short:map[string]string{"get":"Read a check and its answer","answer":"Answer pass, feedback, yes or no","trust":"Read project check trust","settings":"Read or set workspace check settings"}[action],Args:args,RunE:func(cmd *cobra.Command,args []string)error{
   output,err:=questionOutput(cmd);if err!=nil{return err}
   client,err:=newAPIClient(cmd);if err!=nil{return err};ctx,cancel:=cli.APIContext(cmd.Context());defer cancel()
   var result any
   switch action{
   case "get":err=client.GetJSON(ctx,"/api/todos/checks/"+url.PathEscape(args[0]),&result)
   case "trust":err=client.GetJSON(ctx,"/api/projects/"+url.PathEscape(args[0])+"/check-trust",&result)
   case "settings":
    if cmd.Flags().Changed("streak-length") || cmd.Flags().Changed("sample-every"){
     var settings map[string]int
     if err=client.GetJSON(ctx,"/api/todos/check-settings",&settings);err!=nil{return err}
     for flag,key:=range map[string]string{"streak-length":"streak_length","sample-every":"sample_every"}{if cmd.Flags().Changed(flag){settings[key],_=cmd.Flags().GetInt(flag)}}
     err=client.PutJSON(ctx,"/api/todos/check-settings",settings,&result)
    }else{err=client.GetJSON(ctx,"/api/todos/check-settings",&result)}
   case "answer":
    answer,_:=cmd.Flags().GetString("answer");note,_:=cmd.Flags().GetString("note")
    err=client.PostJSON(ctx,"/api/todos/"+url.PathEscape(args[0])+"/answer",map[string]string{"answer":answer,"note":note},&result)
   }
   if err!=nil{return err};if output=="json"{return cli.PrintJSON(cmd.OutOrStdout(),result)}
   switch value:=result.(type){
   case []any:
    rows:=make([][]string,0,len(value))
    for _,raw:=range value{if state,ok:=raw.(map[string]any);ok{rows=append(rows,[]string{strVal(state,"kind"),fmt.Sprint(state["automatic"]),fmt.Sprint(state["passed"]),fmt.Sprint(state["feedback"]),fmt.Sprint(state["automatic_passed"])})}}
    cli.PrintTable(cmd.OutOrStdout(),[]string{"KIND","AUTOMATIC","PASSED","FEEDBACK","AUTO PASSED"},rows)
   case map[string]any:
    if action=="settings"{cli.PrintTable(cmd.OutOrStdout(),[]string{"STREAK LENGTH","SAMPLE EVERY"},[][]string{{fmt.Sprint(value["streak_length"]),fmt.Sprint(value["sample_every"])}})}else{
     details,_:=value["details"].(map[string]any)
     cli.PrintTable(cmd.OutOrStdout(),[]string{"ID","KIND","ANSWER","NOTE"},[][]string{{strVal(value,"id"),strVal(details,"check_kind"),strVal(details,"answer"),strVal(details,"note")}})
    }
   }
   return nil
  }}
  sub.Flags().String("output","table","Output format: table or json")
  if action=="answer"{sub.Flags().String("answer","","pass, feedback, yes or no");sub.Flags().String("note","","Feedback note");_ = sub.MarkFlagRequired("answer")}
  if action=="settings"{sub.Flags().Int("streak-length",5,"Human passes before a trust proposal (1–100)");sub.Flags().Int("sample-every",5,"Sample every N automatic checks (1–100)")}
  command.AddCommand(sub)
 }
 return command
}

func runTodoCheck(cmd *cobra.Command,_ []string)error{
 output,err:=questionOutput(cmd);if err!=nil{return err}
 client,err:=newAPIClient(cmd);if err!=nil{return err}
 ctx,cancel:=cli.APIContext(cmd.Context());defer cancel()
 body:=map[string]string{}
 for flag,key:=range map[string]string{"issue":"issue_id","project":"project_id","kind":"kind","link":"link","title":"title"}{body[key],_=cmd.Flags().GetString(flag)}
 answerer,_:=cmd.Flags().GetString("answerer")
 _,body["member_id"],err=resolveAssignee(ctx,client,answerer,assigneeKinds{member:true});if err!=nil{return err}
 var result map[string]any
 if err:=client.PostJSON(ctx,"/api/todos/checks",body,&result);err!=nil{return err}
 if output=="json"{return cli.PrintJSON(cmd.OutOrStdout(),result)}
 details,_:=result["details"].(map[string]any)
 cli.PrintTable(cmd.OutOrStdout(),[]string{"ID","KIND","ANSWER","AUTOMATIC","SAMPLED"},[][]string{{strVal(result,"id"),strVal(details,"check_kind"),strVal(details,"answer"),fmt.Sprint(details["automatic"]),fmt.Sprint(details["sampled"])}})
 return nil
}
