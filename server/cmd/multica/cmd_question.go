package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
	questionrules "github.com/multica-ai/multica/server/internal/question"
)

var questionCmd = newQuestionCommand()

const questionTypesHelp = "Answer types: text, yes_no, single_choice, multiple_choice, confirm_facts, ranking, scale, number, date, person, file, approval."

const questionAskHelp = `Ask flags: --prompt or --prompt-file, --context, --type, repeatable --answerer (member name, email or ID), --due YYYY-MM-DD, --blocks-work, --output table|json.

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
and multiple_choice to min 1 and max the number of options.`

// questionAskRequest uses the core question model's wire names. Links come from
// the authenticated task, not from CLI input.
type questionAskRequest struct {
	Prompt          string          `json:"prompt"`
	ContextLine     string          `json:"context_line,omitempty"`
	AnswerType      string          `json:"answer_type"`
	Settings        json.RawMessage `json:"settings"`
	SuggestedAnswer json.RawMessage `json:"suggested_answer,omitempty"`
	AnswererIDs     []string        `json:"answerer_ids"`
	DueDate         string          `json:"due_date,omitempty"`
	BlocksWork      bool            `json:"blocks_work"`
}

// questionsAvailable reports whether this process may use questions: always in
// a person's own terminal, and in a daemon task only while its run may ask
// (the daemon sets cli.AgentQuestionsEnv then).
func questionsAvailable() bool {
	return !inDaemonTaskIdentityContext() || os.Getenv(cli.AgentQuestionsEnv) == "1"
}

func newQuestionCommand() *cobra.Command {
	q := &cobra.Command{Use: "question", Short: "Ask, read and end questions", Long: "Ask questions, read their answers and thread, close answered questions or withdraw questions.\n\n" + questionTypesHelp + "\n\n" + questionAskHelp}
	// While questions are off for the run, `multica --help` does not list the
	// command, so the agent learns nothing about it.
	q.Hidden = !questionsAvailable()
	ask := &cobra.Command{Use: "ask", Short: "Ask members a typed question", Long: questionTypesHelp + "\n\n" + questionAskHelp, Args: cobra.NoArgs, RunE: runQuestionAsk}
	f := ask.Flags()
	f.String("prompt", "", "Question prompt (mutually exclusive with --prompt-file)")
	f.String("prompt-file", "", "Read the prompt from a UTF-8 file inside the working directory")
	f.Bool("allow-external-file", false, "Allow --prompt-file outside the working directory")
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
	f.String("due", "", "Due date (YYYY-MM-DD)")
	f.Bool("blocks-work", false, "Mark the question as blocking work")
	f.String("output", "table", "Output format: table or json")
	q.AddCommand(ask)
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
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	directory, err := loadQuestionDirectory(ctx, client)
	if err != nil {
		return err
	}
	answerers, _ := cmd.Flags().GetStringArray("answerer")
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
	if request.AnswerType == "person" && cmd.Flags().Changed("suggest") {
		suggest, _ := cmd.Flags().GetStringArray("suggest")
		allow, _ := cmd.Flags().GetString("allow")
		kind, id, err := matchAssignee(suggest[0], assigneeKinds{member: allow != "agents", agent: allow != "members"}, questionCandidatesOfKinds(directory, allow))
		if err != nil {
			return fmt.Errorf("suggested answer %q: %w", suggest[0], err)
		}
		request.SuggestedAnswer, err = json.Marshal(map[string]string{"type": kind, "id": id})
		if err != nil {
			return err
		}
		if err := questionrules.ValidateSuggested(request.AnswerType, request.Settings, request.SuggestedAnswer); err != nil {
			return err
		}
	}
	var result map[string]any
	if err := client.PostJSON(ctx, "/api/questions", request, &result); err != nil {
		return err
	}
	if output == "json" {
		return cli.PrintJSON(cmd.OutOrStdout(), result)
	}
	fmt.Fprintln(cmd.OutOrStdout(), strVal(result, "id"))
	return nil
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

func questionRequestFromFlags(cmd *cobra.Command) (questionAskRequest, error) {
	var request questionAskRequest
	prompt, _, err := resolveTextFlag(cmd, "prompt")
	if err != nil {
		return request, err
	}
	if strings.TrimSpace(prompt) == "" {
		return request, fmt.Errorf("--prompt or --prompt-file is required")
	}
	answerers, _ := cmd.Flags().GetStringArray("answerer")
	if len(answerers) == 0 {
		return request, fmt.Errorf("at least one --answerer is required")
	}
	request.Prompt = prompt
	request.AnswerType, _ = cmd.Flags().GetString("type")
	request.ContextLine, _ = cmd.Flags().GetString("context")
	request.DueDate, _ = cmd.Flags().GetString("due")
	request.BlocksWork, _ = cmd.Flags().GetBool("blocks-work")
	if request.DueDate != "" {
		if _, err := time.Parse(time.DateOnly, request.DueDate); err != nil {
			return request, fmt.Errorf("--due must be YYYY-MM-DD")
		}
	}
	settings, err := questionSettingsFromFlags(cmd, request.AnswerType)
	if err != nil {
		return request, err
	}
	request.Settings, err = json.Marshal(settings)
	if err != nil {
		return request, err
	}
	if err := questionrules.ValidateSettings(request.AnswerType, request.Settings); err != nil {
		return request, err
	}
	suggestions, _ := cmd.Flags().GetStringArray("suggest")
	if len(suggestions) == 0 {
		return request, nil
	}
	if len(suggestions) != 1 && request.AnswerType != "multiple_choice" && request.AnswerType != "ranking" {
		return request, fmt.Errorf("--suggest can be repeated only for multiple_choice or ranking")
	}
	if request.AnswerType == "person" {
		return request, nil // resolved against the workspace directory before posting
	}
	value, err := questionSuggestion(request.AnswerType, settings, suggestions)
	if err != nil {
		return request, err
	}
	request.SuggestedAnswer, err = json.Marshal(value)
	if err != nil {
		return request, err
	}
	if err := questionrules.ValidateSuggested(request.AnswerType, request.Settings, request.SuggestedAnswer); err != nil {
		return request, err
	}
	return request, nil
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
