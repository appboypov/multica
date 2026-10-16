package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
	questionrules "github.com/multica-ai/multica/server/internal/question"
)

var questionCmd = newQuestionCommand()

const questionTypesHelp = "Answer types: text, yes_no, single_choice, multiple_choice, confirm_facts, ranking, scale, number, date, person, file, approval."

const questionAskHelp = `Ask flags: --prompt or --prompt-file, --context, --type, or --questions-file;
--title; repeatable --answerer (member name, email or ID); --squad (name or ID);
--policy any_one|each (default any_one); --on and repeatable --link; --due YYYY-MM-DD;
--blocks-work (default true; --blocks-work=false turns it off); --output table|json.
One ask needs at least one --answerer or a --squad; --squad combines with --answerer.

Settings and suggested values by --type:
  text             --length short|long --placeholder; --suggest text
  yes_no           --why-on-no; --suggest yes|no
  single_choice    repeatable --option, --allow-other; --suggest label
  multiple_choice  repeatable --option, --min, --max; repeatable --suggest labels
  confirm_facts    repeatable --fact; no --suggest
  ranking          repeatable --item; repeatable --suggest labels in order
  scale            --min, --max, --min-label, --max-label; --suggest integer point
  number           --unit, --min, --max; --suggest number
  date             --date-kind date|datetime|range; --suggest ISO value (range: start/end)
  person           --allow members|agents|both; --suggest name, email or ID
  file             repeatable --accept MIME type or .extension; no --suggest
  approval         --subject, --link-url, --link-title; no --suggest

Text defaults to short, date to date, person to members, scale to 1–5,
and multiple_choice to min 1 and max the number of options.

--questions-file <path> reads 1–20 questions from a UTF-8 JSON array inside the
working directory and excludes --prompt, --prompt-file, --context, --type, the
settings flags and --suggest. --title (1–200 characters) is required with more
than one question and refused with one. Each entry:
  {"prompt": "...", "context_line": "...", "answer_type": "single_choice",
   "settings": {"options": ["A", "B"], "allow_other": false},
   "suggested_answer": {"option": "A"}}
context_line and suggested_answer are optional; settings get no flag defaults.
Keys of settings, then of suggested_answer, by answer_type:
  text             {"length", "placeholder"}; {"text"}
  yes_no           {"why_on_no"}; {"choice"}
  single_choice    {"options", "allow_other"}; {"option"}
  multiple_choice  {"options", "min", "max"}; {"options"}
  confirm_facts    {"facts"}; none
  ranking          {"items"}; {"order"}
  scale            {"min", "max", "min_label", "max_label"}; {"point"}
  number           {"unit", "min", "max"}; {"number"}
  date             {"kind"}; {"date"}, {"datetime"} or {"start", "end"}
  person           {"allow"}; {"type": "member|agent", "id"}
  file             {"accept"}; none
  approval         {"subject", "link": {"url", "title"}}; none

Text output prints the ask id, then one line per question: its id and prompt.`

// questionSingleFlags describe one question; --questions-file replaces them.
var questionSingleFlags = []string{"prompt", "prompt-file", "context", "type", "option", "allow-other", "min", "max", "fact", "item", "min-label", "max-label", "unit", "date-kind", "allow", "accept", "length", "placeholder", "why-on-no", "subject", "link-url", "link-title", "suggest"}

// questionAskItem is one entry of the API's questions field and of a
// --questions-file.
type questionAskItem struct {
	Prompt          string          `json:"prompt"`
	ContextLine     string          `json:"context_line,omitempty"`
	AnswerType      string          `json:"answer_type"`
	Settings        json.RawMessage `json:"settings"`
	SuggestedAnswer json.RawMessage `json:"suggested_answer,omitempty"`
}

const questionLinkHelp = `Links: --on <link> is where the question is asked (position 0); repeatable
--link <link> adds more links in the given order, at most 10 together. In a
run, without --on the first --link takes position 0; without either the
question is asked on the run's issue. The asking run is always linked after
them. Outside a run (a member login), --on names the issue the question is
asked on and is required; no run is linked. Add links later with:
multica question link <question-id> <link>...

A link is a bare issue key or ID (WORK-415), or <type>:<ref>:
  issue:<key or ID>        project:<ID or prefix>    autopilot:<ID or prefix>
  agent:<name or ID>       squad:<ID or prefix>      skill:<ID or prefix>
  run:<ID>                 comment:<ID>
run: and comment: also take a prefix of a run or comment on the asking run's
issue. Every link resolves before any request; an unknown type or reference
fails without asking.`

// questionAskRequest uses the core question model's wire names. Links come
// from --on and --link; the asking run's task link comes from the
// authenticated task, not from CLI input.
type questionAskRequest struct {
	Title        string            `json:"title,omitempty"`
	Questions    []questionAskItem `json:"questions"`
	AnswererIDs  []string          `json:"answerer_ids"`
	SquadID      string            `json:"squad_id,omitempty"`
	AnswerPolicy string            `json:"answer_policy"`
	DueDate      string            `json:"due_date,omitempty"`
	BlocksWork   bool              `json:"blocks_work"`
	Links        []questionLink    `json:"links,omitempty"`
}

// questionLink is a resolved question link on the wire.
type questionLink struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// questionLinkInput keeps a parsed link next to what the user typed.
type questionLinkInput struct {
	raw string
	ref questionrules.LinkRef
}

// questionsAvailable reports whether this process may use questions: always in
// a person's own terminal, and in a daemon task only while its run may ask
// (the daemon sets cli.AgentQuestionsEnv then).
func questionsAvailable() bool {
	return !inDaemonTaskIdentityContext() || os.Getenv(cli.AgentQuestionsEnv) == "1"
}

func newQuestionCommand() *cobra.Command {
	q := &cobra.Command{Use: "question", Short: "Ask, read and end questions", Long: "Ask questions, link them to work, read their answers and thread, close answered questions or withdraw questions.\n\n" + questionTypesHelp + "\n\n" + questionAskHelp + "\n\n" + questionLinkHelp}
	// While questions are off for the run, `multica --help` does not list the
	// command, so the agent learns nothing about it.
	q.Hidden = !questionsAvailable()
	ask := &cobra.Command{Use: "ask", Short: "Ask members one typed question or a set of questions", Long: questionTypesHelp + "\n\n" + questionAskHelp + "\n\n" + questionLinkHelp, Args: cobra.NoArgs, RunE: runQuestionAsk}
	f := ask.Flags()
	f.String("prompt", "", "Question prompt (mutually exclusive with --prompt-file and --questions-file)")
	f.String("prompt-file", "", "Read the prompt from a UTF-8 file inside the working directory")
	f.String("questions-file", "", "Read the questions from a UTF-8 JSON array file inside the working directory; see the shape above")
	f.Bool("allow-external-file", false, "Allow --prompt-file or --questions-file outside the working directory")
	f.String("title", "", "Title of the set (1–200 characters; required with more than one question)")
	f.String("context", "", "Optional context line")
	f.String("type", "", "Answer type; see the twelve types above")
	f.StringArray("option", nil, "Choice label (repeatable; single_choice or multiple_choice)")
	f.Bool("allow-other", false, "Allow Other for single_choice")
	f.String("min", "", "Minimum selection count, scale point or number")
	f.String("max", "", "Maximum selection count, scale point or number")
	f.StringArray("fact", nil, "Statement for confirm_facts (repeatable)")
	f.StringArray("item", nil, "Item for ranking (repeatable)")
	f.String("min-label", "", "Label at the scale minimum")
	f.String("max-label", "", "Label at the scale maximum")
	f.String("unit", "", "Unit for number")
	f.String("date-kind", "date", "Date kind: date, datetime or range")
	f.String("allow", "members", "Person selection: members, agents or both")
	f.StringArray("accept", nil, "Accepted MIME type or .extension for file (repeatable)")
	f.String("length", "short", "Text length: short or long")
	f.String("placeholder", "", "Text placeholder")
	f.Bool("why-on-no", false, "Require an answer note for No on yes_no")
	f.String("subject", "", "Subject for approval")
	f.String("link-url", "", "Approval subject link URL (requires --link-title)")
	f.String("link-title", "", "Approval subject link title (requires --link-url)")
	f.StringArray("suggest", nil, "Suggested value; repeat for multiple_choice or ranking; unavailable for confirm_facts, file or approval")
	f.StringArray("answerer", nil, "Member name, email or ID (repeatable; agents cannot answer)")
	f.String("squad", "", "Squad name or ID; its human members answer (combinable with --answerer)")
	f.String("policy", questionrules.PolicyAnyOne, "Answer policy: any_one or each")
	f.String("on", "", "Link the question is asked on, at position 0: an issue key or ID, or <type>:<ref>")
	f.StringArray("link", nil, "Further link, kept in order (repeatable): an issue key or ID, or <type>:<ref>")
	f.String("due", "", "Due date (YYYY-MM-DD)")
	f.Bool("blocks-work", true, "Mark the questions as blocking work; --blocks-work=false turns it off")
	f.String("output", "table", "Output format: table or json")
	q.AddCommand(ask)
	link := &cobra.Command{Use: "link <question-id> <link>...", Short: "Add links to a question you asked", Long: questionLinkHelp, Args: cobra.MinimumNArgs(2), RunE: runQuestionLink}
	link.Flags().String("output", "table", "Output format: table or json")
	q.AddCommand(link)
	for _, action := range []string{"answers", "close", "withdraw"} {
		action := action
		c := &cobra.Command{Use: action + " <question-id>", Short: map[string]string{"answers": "Read status, answers and thread", "close": "Close an answered question", "withdraw": "Withdraw an open or answered question"}[action], Args: exactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return runQuestionAction(cmd, args[0], action) }}
		c.Flags().String("output", "table", "Output format: table or json")
		q.AddCommand(c)
	}
	return q
}

func questionOutput(cmd *cobra.Command) (string, error) {
	output, _ := cmd.Flags().GetString("output")
	if output != "json" && output != "table" {
		return "", fmt.Errorf("--output must be table or json")
	}
	return output, nil
}

func runQuestionAsk(cmd *cobra.Command, _ []string) error {
	if !questionsAvailable() {
		return fmt.Errorf("questions are off for this issue")
	}
	output, err := questionOutput(cmd)
	if err != nil {
		return err
	}
	request, err := questionRequestFromFlags(cmd)
	if err != nil {
		return err
	}
	linkInputs, err := questionLinkInputsFromFlags(cmd)
	if err != nil {
		return err
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	// Outside a run no run's issue stands in for the place of the question,
	// so --on must name the issue it is asked on; refused before any request.
	if client.TaskID == "" && (!cmd.Flags().Changed("on") || linkInputs[0].ref.Type != questionrules.LinkTypeIssue) {
		return fmt.Errorf("a question asked outside a run needs --on with an issue")
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	answerers, _ := cmd.Flags().GetStringArray("answerer")
	first := &request.Questions[0]
	suggestPerson := !cmd.Flags().Changed("questions-file") && first.AnswerType == "person" && cmd.Flags().Changed("suggest")
	if len(answerers) > 0 || suggestPerson {
		directory, err := loadQuestionDirectory(ctx, client)
		if err != nil {
			return err
		}
		seen := make(map[string]bool, len(answerers))
		for _, name := range answerers {
			kind, id, err := matchAssignee(name, assigneeKinds{member: true, agent: true}, directory)
			if err != nil {
				return err
			}
			if kind == "agent" {
				for _, entry := range directory {
					if entry.ID == id && entry.Type == kind {
						return fmt.Errorf("%s is an agent; only members answer questions", entry.Name)
					}
				}
			}
			if !seen[id] {
				request.AnswererIDs = append(request.AnswererIDs, id)
				seen[id] = true
			}
		}
		if suggestPerson {
			suggest, _ := cmd.Flags().GetStringArray("suggest")
			allow, _ := cmd.Flags().GetString("allow")
			kind, id, err := matchAssignee(suggest[0], assigneeKinds{member: allow != "agents", agent: allow != "members"}, questionCandidatesOfKinds(directory, allow))
			if err != nil {
				return fmt.Errorf("suggested answer %q: %w", suggest[0], err)
			}
			first.SuggestedAnswer, err = json.Marshal(map[string]string{"type": kind, "id": id})
			if err != nil {
				return err
			}
			if err := questionrules.ValidateSuggested(first.AnswerType, first.Settings, first.SuggestedAnswer); err != nil {
				return err
			}
		}
	}
	if squad, _ := cmd.Flags().GetString("squad"); strings.TrimSpace(squad) != "" {
		if _, request.SquadID, err = resolveAssignee(ctx, client, squad, assigneeKinds{squad: true}); err != nil {
			return err
		}
	}
	if request.Links, err = resolveQuestionLinks(ctx, client, linkInputs); err != nil {
		return err
	}
	var result map[string]any
	if err := client.PostJSON(ctx, "/api/questions", request, &result); err != nil {
		return err
	}
	if output == "json" {
		return cli.PrintJSON(cmd.OutOrStdout(), result)
	}
	fmt.Fprintln(cmd.OutOrStdout(), strVal(result, "ask_id"))
	if questions, ok := result["questions"].([]any); ok {
		for _, raw := range questions {
			if question, ok := raw.(map[string]any); ok {
				fmt.Fprintf(cmd.OutOrStdout(), "%s · %s\n", strVal(question, "id"), strings.Join(strings.Fields(strVal(question, "prompt")), " "))
			}
		}
	}
	return nil
}

func runQuestionLink(cmd *cobra.Command, args []string) error {
	output, err := questionOutput(cmd)
	if err != nil {
		return err
	}
	inputs, err := parseQuestionLinks(args[1:])
	if err != nil {
		return err
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	links, err := resolveQuestionLinks(ctx, client, inputs)
	if err != nil {
		return err
	}
	var result map[string]any
	if err := client.PostJSON(ctx, "/api/questions/"+url.PathEscape(args[0])+"/links", map[string]any{"links": links}, &result); err != nil {
		return err
	}
	if output == "json" {
		return cli.PrintJSON(cmd.OutOrStdout(), result)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s · %s\n%s\n", strVal(result, "id"), strVal(result, "status"), strVal(result, "prompt"))
	printQuestionLinks(cmd, result)
	return nil
}

// questionLinkInputsFromFlags parses --on, then every --link in order, so --on
// is position 0. It sends nothing, so a malformed link fails before any request.
func questionLinkInputsFromFlags(cmd *cobra.Command) ([]questionLinkInput, error) {
	on, _ := cmd.Flags().GetString("on")
	extra, _ := cmd.Flags().GetStringArray("link")
	raws := make([]string, 0, len(extra)+1)
	if cmd.Flags().Changed("on") {
		raws = append(raws, on)
	}
	raws = append(raws, extra...)
	if len(raws) > questionrules.MaxLinksPerAsk {
		return nil, fmt.Errorf("a question takes at most %d links from --on and --link, got %d", questionrules.MaxLinksPerAsk, len(raws))
	}
	return parseQuestionLinks(raws)
}

func parseQuestionLinks(raws []string) ([]questionLinkInput, error) {
	inputs := make([]questionLinkInput, 0, len(raws))
	for _, raw := range raws {
		ref, err := questionrules.ParseLinkRef(raw)
		if err != nil {
			return nil, fmt.Errorf("link %q: %w", raw, err)
		}
		inputs = append(inputs, questionLinkInput{raw: raw, ref: ref})
	}
	return inputs, nil
}

// resolveQuestionLinks turns every parsed link into its object's ID, in order,
// before the caller sends anything. The asking run's issue is looked up once,
// and only when a run: or comment: prefix needs it.
func resolveQuestionLinks(ctx context.Context, client *cli.APIClient, inputs []questionLinkInput) ([]questionLink, error) {
	var taskIssueID string
	taskIssue := func() (string, error) {
		if taskIssueID != "" {
			return taskIssueID, nil
		}
		id, err := questionTaskIssueID(ctx, client)
		if err != nil {
			return "", err
		}
		taskIssueID = id
		return id, nil
	}
	links := make([]questionLink, 0, len(inputs))
	for _, input := range inputs {
		id, err := resolveQuestionLink(ctx, client, input.ref, taskIssue)
		if err != nil {
			return nil, fmt.Errorf("link %q: %w", input.raw, err)
		}
		links = append(links, questionLink{Type: string(input.ref.Type), ID: id})
	}
	return links, nil
}

func resolveQuestionLink(ctx context.Context, client *cli.APIClient, ref questionrules.LinkRef, taskIssue func() (string, error)) (string, error) {
	var resolved resolvedID
	var err error
	switch ref.Type {
	case questionrules.LinkTypeIssue:
		resolved, err = resolveIssueRef(ctx, client, ref.Ref)
	case questionrules.LinkTypeProject:
		resolved, err = resolveProjectID(ctx, client, ref.Ref)
	case questionrules.LinkTypeAutopilot:
		resolved, err = resolveAutopilotID(ctx, client, ref.Ref)
	case questionrules.LinkTypeAgent:
		return resolveAgent(ctx, client, ref.Ref)
	case questionrules.LinkTypeTask:
		issueID := ""
		if !uuidRegexp.MatchString(ref.Ref) {
			if issueID, err = taskIssue(); err != nil {
				return "", fmt.Errorf("a run prefix resolves on the asking run's issue: %w", err)
			}
		}
		resolved, err = resolveTaskRunID(ctx, client, issueID, ref.Ref)
	case questionrules.LinkTypeComment:
		resolved, err = resolveIDByPrefix(ctx, client, "comment", ref.Ref, func(ctx context.Context, client *cli.APIClient) ([]idCandidate, error) {
			issueID, err := taskIssue()
			if err != nil {
				return nil, fmt.Errorf("a comment prefix resolves on the asking run's issue: %w", err)
			}
			return fetchCommentCandidatesForIssue(ctx, client, issueID)
		})
	case questionrules.LinkTypeSquad:
		resolved, err = resolveIDByPrefix(ctx, client, "squad", ref.Ref, fetchNamedCandidates("/api/squads"))
	case questionrules.LinkTypeSkill:
		resolved, err = resolveIDByPrefix(ctx, client, "skill", ref.Ref, fetchNamedCandidates("/api/skills"))
	default:
		return "", fmt.Errorf("unknown question link type %q", ref.Type)
	}
	if err != nil {
		return "", err
	}
	return resolved.ID, nil
}

// questionTaskIssueID finds the issue of the run this CLI runs in, from the
// agent's runs.
func questionTaskIssueID(ctx context.Context, client *cli.APIClient) (string, error) {
	if client.AgentID == "" || client.TaskID == "" {
		return "", fmt.Errorf("no agent run in context; pass the full UUID")
	}
	var tasks []map[string]any
	if err := client.GetJSON(ctx, "/api/agents/"+url.PathEscape(client.AgentID)+"/tasks", &tasks); err != nil {
		return "", fmt.Errorf("fetch the asking run: %w", err)
	}
	for _, task := range tasks {
		if strVal(task, "id") == client.TaskID {
			if issueID := strVal(task, "issue_id"); issueID != "" {
				return issueID, nil
			}
			break
		}
	}
	return "", fmt.Errorf("the asking run %s has no issue; pass the full UUID", truncateID(client.TaskID))
}

func fetchCommentCandidatesForIssue(ctx context.Context, client *cli.APIClient, issueID string) ([]idCandidate, error) {
	var comments []map[string]any
	if err := client.GetJSON(ctx, "/api/issues/"+url.PathEscape(issueID)+"/comments", &comments); err != nil {
		return nil, err
	}
	candidates := make([]idCandidate, 0, len(comments))
	for _, c := range comments {
		if id := strVal(c, "id"); id != "" {
			candidates = append(candidates, idCandidate{ID: id, Display: id})
		}
	}
	return candidates, nil
}

// fetchNamedCandidates lists the workspace's objects at path, each with an id
// and a name.
func fetchNamedCandidates(path string) func(context.Context, *cli.APIClient) ([]idCandidate, error) {
	return func(ctx context.Context, client *cli.APIClient) ([]idCandidate, error) {
		var objects []map[string]any
		if err := client.GetJSON(ctx, path, &objects); err != nil {
			return nil, err
		}
		candidates := make([]idCandidate, 0, len(objects))
		for _, o := range objects {
			if id := strVal(o, "id"); id != "" {
				candidates = append(candidates, idCandidate{ID: id, Display: strVal(o, "name")})
			}
		}
		return candidates, nil
	}
}

// printQuestionLinks lists a question's links with their positions and types.
func printQuestionLinks(cmd *cobra.Command, result map[string]any) {
	links, _ := result["links"].([]any)
	for _, raw := range links {
		if link, ok := raw.(map[string]any); ok {
			fmt.Fprintf(cmd.OutOrStdout(), "- link %s · %s · %s\n", strVal(link, "position"), strVal(link, "link_type"), strVal(link, "link_id"))
		}
	}
}

func loadQuestionDirectory(ctx context.Context, client *cli.APIClient) ([]assigneeCandidate, error) {
	if client.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required to resolve question members; use --workspace-id or MULTICA_WORKSPACE_ID")
	}
	var members, agents []map[string]any
	if err := client.GetJSON(ctx, "/api/workspaces/"+url.PathEscape(client.WorkspaceID)+"/members", &members); err != nil {
		return nil, fmt.Errorf("fetch question members: %w", err)
	}
	if err := client.GetJSON(ctx, "/api/agents?"+url.Values{"workspace_id": {client.WorkspaceID}}.Encode(), &agents); err != nil {
		return nil, fmt.Errorf("fetch question agents: %w", err)
	}
	candidates := memberCandidates(members)
	for _, agent := range agents {
		candidates = append(candidates, assigneeCandidate{assigneeMatch: assigneeMatch{Type: "agent", ID: strVal(agent, "id"), Name: strVal(agent, "name")}})
	}
	return candidates, nil
}

func questionCandidatesOfKinds(directory []assigneeCandidate, allow string) []assigneeCandidate {
	filtered := make([]assigneeCandidate, 0, len(directory))
	for _, entry := range directory {
		if allow == "both" || allow == "members" && entry.Type == "member" || allow == "agents" && entry.Type == "agent" {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// questionRequestFromFlags builds and checks the whole ask before any request:
// one question from the flags or several from --questions-file, then the
// fields the questions share.
func questionRequestFromFlags(cmd *cobra.Command) (questionAskRequest, error) {
	request := questionAskRequest{AnswererIDs: []string{}}
	var err error
	if cmd.Flags().Changed("questions-file") {
		request.Questions, err = questionsFromFile(cmd)
	} else {
		var question questionAskItem
		question, err = questionFromFlags(cmd)
		request.Questions = []questionAskItem{question}
	}
	if err != nil {
		return request, err
	}
	title, _ := cmd.Flags().GetString("title")
	request.Title = strings.TrimSpace(title)
	switch {
	case len(request.Questions) > 1 && request.Title == "":
		return request, fmt.Errorf("--title is required with more than one question")
	case len(request.Questions) == 1 && request.Title != "":
		return request, fmt.Errorf("--title is only for more than one question")
	case utf8.RuneCountInString(request.Title) > 200:
		return request, fmt.Errorf("--title must be at most 200 characters")
	}
	answerers, _ := cmd.Flags().GetStringArray("answerer")
	squad, _ := cmd.Flags().GetString("squad")
	if len(answerers) == 0 && strings.TrimSpace(squad) == "" {
		return request, fmt.Errorf("at least one --answerer or a --squad is required")
	}
	request.AnswerPolicy, _ = cmd.Flags().GetString("policy")
	if request.AnswerPolicy != questionrules.PolicyAnyOne && request.AnswerPolicy != questionrules.PolicyEach {
		return request, fmt.Errorf("--policy must be any_one or each")
	}
	request.DueDate, _ = cmd.Flags().GetString("due")
	request.BlocksWork, _ = cmd.Flags().GetBool("blocks-work")
	if request.DueDate != "" {
		if _, err := time.Parse(time.DateOnly, request.DueDate); err != nil {
			return request, fmt.Errorf("--due must be YYYY-MM-DD")
		}
	}
	return request, nil
}

// questionsFromFile reads the API's questions field from --questions-file and
// checks each entry, naming it by its 1-based place.
func questionsFromFile(cmd *cobra.Command) ([]questionAskItem, error) {
	for _, name := range questionSingleFlags {
		if cmd.Flags().Changed(name) {
			return nil, fmt.Errorf("--questions-file and --%s are mutually exclusive", name)
		}
	}
	path, _ := cmd.Flags().GetString("questions-file")
	if err := ensureFileFlagWithinWorkdir(cmd, "questions-file", "questions", path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file for --questions-file: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var questions []questionAskItem
	if err := decoder.Decode(&questions); err != nil {
		return nil, fmt.Errorf("--questions-file must hold a JSON array of questions: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("--questions-file must hold one JSON array of questions")
	}
	if len(questions) == 0 {
		return nil, fmt.Errorf("--questions-file holds no questions")
	}
	for i := range questions {
		question := &questions[i]
		if strings.TrimSpace(question.Prompt) == "" {
			return nil, fmt.Errorf("question %d: prompt is required", i+1)
		}
		if err := questionrules.ValidateSettings(question.AnswerType, question.Settings); err != nil {
			return nil, fmt.Errorf("question %d: %w", i+1, err)
		}
		if string(question.SuggestedAnswer) == "null" {
			question.SuggestedAnswer = nil
		}
		if question.SuggestedAnswer != nil {
			if err := questionrules.ValidateSuggested(question.AnswerType, question.Settings, question.SuggestedAnswer); err != nil {
				return nil, fmt.Errorf("question %d: %w", i+1, err)
			}
		}
	}
	return questions, nil
}

// questionFromFlags builds the one question --prompt, --type and the settings
// flags describe. A person suggestion is resolved against the workspace
// directory before posting.
func questionFromFlags(cmd *cobra.Command) (questionAskItem, error) {
	var question questionAskItem
	prompt, _, err := resolveTextFlag(cmd, "prompt")
	if err != nil {
		return question, err
	}
	if strings.TrimSpace(prompt) == "" {
		return question, fmt.Errorf("--prompt, --prompt-file or --questions-file is required")
	}
	question.Prompt = prompt
	question.AnswerType, _ = cmd.Flags().GetString("type")
	question.ContextLine, _ = cmd.Flags().GetString("context")
	settings, err := questionSettingsFromFlags(cmd, question.AnswerType)
	if err != nil {
		return question, err
	}
	question.Settings, err = json.Marshal(settings)
	if err != nil {
		return question, err
	}
	if err := questionrules.ValidateSettings(question.AnswerType, question.Settings); err != nil {
		return question, err
	}
	suggestions, _ := cmd.Flags().GetStringArray("suggest")
	if len(suggestions) == 0 {
		return question, nil
	}
	if len(suggestions) != 1 && question.AnswerType != "multiple_choice" && question.AnswerType != "ranking" {
		return question, fmt.Errorf("--suggest can be repeated only for multiple_choice or ranking")
	}
	if question.AnswerType == "person" {
		return question, nil
	}
	value, err := questionSuggestion(question.AnswerType, settings, suggestions)
	if err != nil {
		return question, err
	}
	question.SuggestedAnswer, err = json.Marshal(value)
	if err != nil {
		return question, err
	}
	if err := questionrules.ValidateSuggested(question.AnswerType, question.Settings, question.SuggestedAnswer); err != nil {
		return question, err
	}
	return question, nil
}

func questionSettingsFromFlags(cmd *cobra.Command, answerType string) (map[string]any, error) {
	settings := map[string]any{}
	stringFlag := func(flag, key string, required bool) {
		v, _ := cmd.Flags().GetString(flag)
		if required || v != "" {
			settings[key] = v
		}
	}
	arrayFlag := func(flag, key string) {
		v, _ := cmd.Flags().GetStringArray(flag)
		if v == nil {
			v = []string{}
		}
		settings[key] = v
	}
	boolFlag := func(flag, key string) { v, _ := cmd.Flags().GetBool(flag); settings[key] = v }
	bounds := func(integer bool, defaults map[string]int) error {
		for _, flag := range []string{"min", "max"} {
			v, _ := cmd.Flags().GetString(flag)
			if !cmd.Flags().Changed(flag) {
				if n, ok := defaults[flag]; ok {
					settings[flag] = n
				}
				continue
			}
			if integer {
				n, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("--%s must be an integer", flag)
				}
				settings[flag] = n
			} else {
				n, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return fmt.Errorf("--%s must be a number", flag)
				}
				settings[flag] = n
			}
		}
		return nil
	}
	switch answerType {
	case "text":
		stringFlag("length", "length", true)
		stringFlag("placeholder", "placeholder", false)
	case "yes_no":
		boolFlag("why-on-no", "why_on_no")
	case "single_choice":
		arrayFlag("option", "options")
		boolFlag("allow-other", "allow_other")
	case "multiple_choice":
		arrayFlag("option", "options")
		if err := bounds(true, map[string]int{"min": 1, "max": len(settings["options"].([]string))}); err != nil {
			return nil, err
		}
	case "confirm_facts":
		arrayFlag("fact", "facts")
	case "ranking":
		arrayFlag("item", "items")
	case "scale":
		if err := bounds(true, map[string]int{"min": 1, "max": 5}); err != nil {
			return nil, err
		}
		stringFlag("min-label", "min_label", false)
		stringFlag("max-label", "max_label", false)
	case "number":
		if err := bounds(false, nil); err != nil {
			return nil, err
		}
		stringFlag("unit", "unit", false)
	case "date":
		stringFlag("date-kind", "kind", true)
	case "person":
		stringFlag("allow", "allow", true)
	case "file":
		arrayFlag("accept", "accept")
	case "approval":
		stringFlag("subject", "subject", true)
		linkURL, _ := cmd.Flags().GetString("link-url")
		linkTitle, _ := cmd.Flags().GetString("link-title")
		if (linkURL == "") != (linkTitle == "") {
			return nil, fmt.Errorf("--link-url and --link-title must be given together")
		}
		if linkURL != "" {
			settings["link"] = map[string]string{"url": linkURL, "title": linkTitle}
		}
	default:
		return nil, fmt.Errorf("--type is required and must be one of: %s", strings.TrimPrefix(questionTypesHelp, "Answer types: "))
	}
	return settings, nil
}

func questionSuggestion(answerType string, settings map[string]any, suggestions []string) (any, error) {
	value := suggestions[0]
	switch answerType {
	case "text":
		return map[string]string{"text": value}, nil
	case "yes_no":
		return map[string]string{"choice": value}, nil
	case "single_choice":
		return map[string]string{"option": value}, nil
	case "multiple_choice":
		return map[string][]string{"options": suggestions}, nil
	case "ranking":
		return map[string][]string{"order": suggestions}, nil
	case "scale":
		n, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("suggested answer %q must be an integer point", value)
		}
		return map[string]int{"point": n}, nil
	case "number":
		n, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("suggested answer %q must be a number", value)
		}
		return map[string]float64{"number": n}, nil
	case "date":
		switch settings["kind"] {
		case "range":
			parts := strings.Split(value, "/")
			if len(parts) != 2 {
				return nil, fmt.Errorf("suggested answer %q must be start/end", value)
			}
			return map[string]string{"start": parts[0], "end": parts[1]}, nil
		case "datetime":
			return map[string]string{"datetime": value}, nil
		default:
			return map[string]string{"date": value}, nil
		}
	default:
		return nil, fmt.Errorf("suggested answer %q is not allowed for %s", value, answerType)
	}
}

func runQuestionAction(cmd *cobra.Command, id, action string) error {
	output, err := questionOutput(cmd)
	if err != nil {
		return err
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	path := "/api/questions/" + url.PathEscape(id)
	var result map[string]any
	if action == "answers" {
		err = client.GetJSON(ctx, path, &result)
	} else {
		err = client.PostJSON(ctx, path+"/"+action, map[string]any{}, &result)
	}
	if err != nil {
		return err
	}
	if output == "json" {
		return cli.PrintJSON(cmd.OutOrStdout(), result)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s · %s\n%s\n", strVal(result, "id"), strVal(result, "status"), strVal(result, "prompt"))
	if action != "answers" {
		return nil
	}
	printQuestionLinks(cmd, result)
	lookup := loadActorDisplayLookup(ctx, client)
	if answers, ok := result["answers"].([]any); ok {
		for _, raw := range answers {
			if answer, ok := raw.(map[string]any); ok {
				fmt.Fprintf(cmd.OutOrStdout(), "- %s · %s · %s", strVal(answer, "display_text"), lookup.actor("member", strVal(answer, "answerer_user_id")), strVal(answer, "updated_at"))
				if note := strVal(answer, "note"); note != "" {
					fmt.Fprintf(cmd.OutOrStdout(), " · note: %s", note)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
		}
	}
	if events, ok := result["events"].([]any); ok {
		for _, raw := range events {
			if event, ok := raw.(map[string]any); ok {
				fmt.Fprintf(cmd.OutOrStdout(), "- %s · %s · %s", strVal(event, "kind"), lookup.actor(strVal(event, "actor_type"), strVal(event, "actor_id")), strVal(event, "created_at"))
				if details, ok := event["details"].(map[string]any); ok {
					for _, key := range []string{"text", "note"} {
						if v := strVal(details, key); v != "" {
							fmt.Fprintf(cmd.OutOrStdout(), " · %s", v)
						}
					}
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
		}
	}
	return nil
}
