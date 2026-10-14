package main

import (
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "sort"
 "strings"

 "github.com/spf13/cobra"
 "github.com/multica-ai/multica/server/internal/cli"
 "github.com/multica-ai/multica/server/internal/issuetemplate"
)

type issuePlacementWorklane struct {
 ID string `json:"id"`
 RepositoryID string `json:"repository_id"`
 Name string `json:"name"`
}
type issuePlacementRepository struct { ID string `json:"id"`; FullName string `json:"full_name"`; ProjectID string `json:"project_id"` }
type issuePlacementRelease struct { ID string `json:"id"`; RepositoryID string `json:"repository_id"`; Version string `json:"version"` }
type issuePlacementCatalogue struct {
 worklanes []issuePlacementWorklane
 repositories []issuePlacementRepository
 releases []issuePlacementRelease
}
func loadIssuePlacementCatalogue(ctx context.Context,client *cli.APIClient) (issuePlacementCatalogue,error) {
 var wl struct { Worklanes []issuePlacementWorklane `json:"worklanes"` }
 var repos struct { Repositories []issuePlacementRepository `json:"repositories"` }
 var releases struct { Releases []issuePlacementRelease `json:"releases"` }
 if err:=client.GetJSON(ctx,"/api/worklanes",&wl); err!=nil { return issuePlacementCatalogue{},err }
 if err:=client.GetJSON(ctx,"/api/repositories",&repos); err!=nil { return issuePlacementCatalogue{},err }
 if err:=client.GetJSON(ctx,"/api/releases",&releases); err!=nil { return issuePlacementCatalogue{},err }
 return issuePlacementCatalogue{wl.Worklanes,repos.Repositories,releases.Releases},nil
}
func (c issuePlacementCatalogue) repositoryName(id string) string {
 for _,repo:=range c.repositories { if repo.ID==id { return repo.FullName } }; return id
}
func resolveWorklane(value string,c issuePlacementCatalogue) (*issuePlacementWorklane,error) {
 if value=="none" { return nil,nil }
 matches:=[]issuePlacementWorklane{}
 for _,wl:=range c.worklanes {
  if value==wl.ID { copy:=wl; return &copy,nil }
  if value==wl.Name || value==c.repositoryName(wl.RepositoryID)+"/"+wl.Name { matches=append(matches,wl) }
 }
 if len(matches)==1 { return &matches[0],nil }
 if len(matches)>1 { names:=make([]string,len(matches)); for i,wl:=range matches { names[i]=c.repositoryName(wl.RepositoryID)+"/"+wl.Name }; sort.Strings(names); return nil,fmt.Errorf("worklane %q is ambiguous: %s",value,strings.Join(names,", ")) }
 return nil,fmt.Errorf("worklane %q not found",value)
}
func resolveRelease(value string,wl *issuePlacementWorklane,c issuePlacementCatalogue) (*issuePlacementRelease,error) {
 if value=="none" { return nil,nil }
 if wl==nil { return nil,errors.New("a release needs a worklane") }
 for _,rel:=range c.releases {
  if rel.RepositoryID==wl.RepositoryID && (value==rel.ID || strings.TrimPrefix(value,"v")==strings.TrimPrefix(rel.Version,"v")) { copy:=rel; return &copy,nil }
 }
 return nil,fmt.Errorf("%s is not a release of %s",value,c.repositoryName(wl.RepositoryID))
}
func issuePlacementBody(cmd *cobra.Command,ctx context.Context,client *cli.APIClient,current,updates map[string]any) (map[string]any,error) {
 c,err:=loadIssuePlacementCatalogue(ctx,client); if err!=nil { return nil,err }
 worklaneID,_:=current["worklane_id"].(string); releaseID,_:=current["release_id"].(string)
 var wl *issuePlacementWorklane
 if worklaneID!="" { wl,err=resolveWorklane(worklaneID,c); if err!=nil { return nil,err } }
 if cmd.Flags().Changed("worklane") { value,_:=cmd.Flags().GetString("worklane"); wl,err=resolveWorklane(value,c); if err!=nil { return nil,err }; worklaneID=""; if wl!=nil { worklaneID=wl.ID } }
 if cmd.Flags().Changed("release") {
  value,_:=cmd.Flags().GetString("release"); rel,err:=resolveRelease(value,wl,c); if err!=nil { return nil,err }; releaseID=""; if rel!=nil { releaseID=rel.ID }
 } else if releaseID!="" && wl==nil { return nil,errors.New("a release needs a worklane; use --release none to clear it") }
 if project,touched:=updates["project_id"]; touched && wl!=nil {
  for _,repo:=range c.repositories {
   if repo.ID==wl.RepositoryID && project!=repo.ProjectID { return nil,errors.New("the issue's project follows its worklane") }
  }
 }
 body:=map[string]any{"worklane_id":nil,"release_id":nil}; if worklaneID!="" { body["worklane_id"]=worklaneID }; if releaseID!="" { body["release_id"]=releaseID }; return body,nil
}
func validateIssueTypeFlag(cmd *cobra.Command) error {
 if !cmd.Flags().Changed("type") { return nil }
 value,_:=cmd.Flags().GetString("type"); if !issuetemplate.IsType(value) { return fmt.Errorf("--type must be one of %s",strings.Join(issuetemplate.Types(),", ")) }; return nil
}
func issuePlacementCommandError(err error) error {
 var httpErr *cli.HTTPError
 if !errors.As(err,&httpErr) { return err }
 var refusal struct { Code string `json:"code"`; Error string `json:"error"`; Issues []struct { Identifier string `json:"identifier"` } `json:"issues"` }
 if json.Unmarshal([]byte(httpErr.Body),&refusal)!=nil { return err }
 switch refusal.Code {
 case "type_invalid","type_fixed","release_needs_worklane","release_not_of_repository","release_shipped","issue_not_in_repository","issue_in_release","project_follows_worklane","worklane_has_open_issues":
  message:=refusal.Error
  if len(refusal.Issues)>0 { identifiers:=make([]string,len(refusal.Issues)); for i,issue:=range refusal.Issues { identifiers[i]=issue.Identifier }; message += ": " + strings.Join(identifiers,", ") }
  return errors.New(message)
 default: return err
 }
}
