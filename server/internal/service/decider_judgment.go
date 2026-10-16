package service

import (
 "bytes"
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "math"
 "net/http"
 "strings"
 "time"

 "github.com/multica-ai/multica/server/internal/util"
)

const DeciderDefaultConfidence = 0.85

type DeciderSettings struct {
 AgentID string `json:"agent_id"`
 MemberID string `json:"member_id"`
 ConfidenceThreshold *float64 `json:"confidence_threshold,omitempty"`
 Rules []DeciderRule `json:"rules"`
}

type DeciderRule struct {
 ID string `json:"id"`
 Event string `json:"event"`
 Status string `json:"status,omitempty"`
 Decision string `json:"decision"`
 Question string `json:"question"`
 Reasons map[string]string `json:"reasons"`
 PostMergeProperty string `json:"post_merge_property,omitempty"`
 KindProperty string `json:"kind_property,omitempty"`
 LeadByKind map[string]string `json:"lead_by_kind,omitempty"`
}

func ParseDeciderSettings(settings []byte) (DeciderSettings, error) {
 var envelope struct { Decider DeciderSettings `json:"decider"` }
 if err := json.Unmarshal(settings, &envelope); err != nil { return DeciderSettings{}, err }
 s := envelope.Decider
 if len(s.Rules) == 0 { return s, nil }
 for _, id := range []string{s.AgentID, s.MemberID} { if _, err := util.ParseUUID(id); err != nil { return s, errors.New("decider agent_id and member_id must be UUIDs") } }
 if s.ConfidenceThreshold != nil && (!validDeciderProbability(*s.ConfidenceThreshold) || *s.ConfidenceThreshold == 0) { return s, errors.New("decider confidence_threshold must be greater than zero and at most one") }
 seen := map[string]bool{}
 for _, r := range s.Rules {
  if r.ID == "" || seen[r.ID] || strings.TrimSpace(r.Question) == "" || len(r.Reasons) < 2 { return s, errors.New("decider rules need unique ids, a question and at least two reasons") }
  seen[r.ID] = true
  if r.Event != "status_entered" && r.Event != "release_joined" { return s, errors.New("invalid decider event") }
  if r.Event == "status_entered" && r.Status == "" { return s, errors.New("status_entered needs a status") }
  if r.Decision != "bug" && r.Decision != "release" { return s, errors.New("invalid decider decision") }
  for id, reason := range r.Reasons { if id == "" || strings.TrimSpace(reason) == "" { return s, errors.New("decider reasons need ids and descriptions") } }
  if r.Decision == "bug" {
   for _, id := range []string{r.PostMergeProperty, r.KindProperty} { if _, err := util.ParseUUID(id); err != nil { return s, errors.New("bug rules need property UUIDs") } }
   if len(r.LeadByKind) == 0 { return s, errors.New("bug rules need kind leads") }
   for _, id := range r.LeadByKind { if _, err := util.ParseUUID(id); err != nil { return s, errors.New("kind leads must be agent UUIDs") } }
  }
 }
 return s, nil
}

func (s DeciderSettings) threshold() float64 { if s.ConfidenceThreshold != nil { return *s.ConfidenceThreshold }; return DeciderDefaultConfidence }
func (r DeciderRule) options() map[string]string {
 if r.Decision == "bug" { return map[string]string{"fix_now":"Fix this post-merge bug now through its kind's lead", "fix_later":"Keep this bug in backlog and ask Brian to plan it"} }
 return map[string]string{"ready":"Ready for Brian's release check", "not_yet":"Not ready for Brian's release check"}
}

type DeciderAnswer struct {
 Choice string `json:"choice"`
 Reason string `json:"reason"`
 Confidence float64 `json:"confidence"`
}
func validDeciderProbability(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 1 }
func (a DeciderAnswer) validate(r DeciderRule) error {
 if _, ok := r.options()[a.Choice]; !ok { return errors.New("invalid decider choice") }
 if _, ok := r.Reasons[a.Reason]; !ok { return errors.New("invalid decider reason") }
 if !validDeciderProbability(a.Confidence) { return errors.New("invalid decider confidence") }
 return nil
}

type DeciderJudge interface { Decide(context.Context, DeciderRule, json.RawMessage) (DeciderAnswer, error) }

type TypeSafeDecider struct { APIKey string; Client *http.Client }
func (j TypeSafeDecider) Decide(ctx context.Context, rule DeciderRule, state json.RawMessage) (DeciderAnswer, error) {
 if j.APIKey == "" { return DeciderAnswer{}, errors.New("TYPESAFE_API_KEY is not configured") }
 body, err := json.Marshal(map[string]any{"model":"jev-latest", "state":state, "questions":map[string]any{
  "action":map[string]any{"type":"choice", "instructions":rule.Question, "criteria":rule.options()},
  "reason":map[string]any{"type":"choice", "instructions":"Which reason best explains the action warranted by the state? "+rule.Question, "criteria":rule.Reasons},
 }})
 if err != nil { return DeciderAnswer{}, err }
 req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.typesafe.ai/v1/systemone", bytes.NewReader(body))
 if err != nil { return DeciderAnswer{}, errors.New("cannot create TypeSafe request") }
 req.Header.Set("Authorization", "Bearer "+j.APIKey)
 req.Header.Set("Content-Type", "application/json")
 client := j.Client; if client == nil { client = &http.Client{Timeout:20*time.Second} }
 resp, err := client.Do(req)
 if err != nil { return DeciderAnswer{}, errors.New("TypeSafe request failed") }
 defer resp.Body.Close()
 if resp.StatusCode != http.StatusOK { return DeciderAnswer{}, fmt.Errorf("TypeSafe status %d", resp.StatusCode) }
 var payload struct { Answers map[string]struct { Type string `json:"type"`; Choice string `json:"choice"`; Confidence *float64 `json:"confidence"`; Probabilities map[string]float64 `json:"probabilities"` } `json:"answers"` }
 if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil { return DeciderAnswer{}, errors.New("invalid TypeSafe response") }
 action, reason := payload.Answers["action"], payload.Answers["reason"]
 if action.Type != "choice" || reason.Type != "choice" || action.Confidence == nil || reason.Confidence == nil || !validDeciderProbability(*action.Confidence) || !validDeciderProbability(*reason.Confidence) { return DeciderAnswer{}, errors.New("missing typed TypeSafe choices") }
 answer := DeciderAnswer{Choice:action.Choice, Reason:reason.Choice, Confidence:math.Min(*action.Confidence,*reason.Confidence)}
 return answer, answer.validate(rule)
}
