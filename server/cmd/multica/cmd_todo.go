package main

import (
	"fmt"
	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/spf13/cobra"
	"net/url"
)

func newTodoCommand() *cobra.Command {
	command := &cobra.Command{Use: "todo", Short: "Open checks, answer to-dos and read project trust", GroupID: groupCore}
	check := &cobra.Command{Use: "check", Short: "Ask for a check of work on the run's issue", Long: "Kinds: data_model, data_flow, design, prototype (before dependent work); feature, improvement, agent_tool (on staging after merge); release (beta or production, before release). Releases always need a human pass.", Args: cobra.NoArgs, RunE: runTodoCheck}
	for _, flag := range []string{"issue", "project", "answerer", "kind", "link", "title"} {
		check.Flags().String(flag, "", "Check "+flag)
		_ = check.MarkFlagRequired(flag)
	}
	check.Flags().String("output", "table", "Output format: table or json")
	command.AddCommand(check)
	for _, action := range []string{"get", "answer", "trust", "settings", "instruction-note", "undo-result"} {
		action := action
		use := action + " <id>"
		args := exactArgs(1)
		if action == "settings" {
			use = "settings"
			args = cobra.NoArgs
		}
		sub := &cobra.Command{Use: use, Short: map[string]string{"get": "Read a check or instruction note", "answer": "Answer pass, feedback, keep or undo", "trust": "Read project check records", "settings": "Read or set workspace check settings", "instruction-note": "Report an instruction change for a feedback check", "undo-result": "Report the result of an instruction undo"}[action], Args: args, RunE: func(cmd *cobra.Command, args []string) error {
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
			var result any
			switch action {
			case "get":
				err = client.GetJSON(ctx, "/api/todos/checks/"+url.PathEscape(args[0]), &result)
			case "trust":
				err = client.GetJSON(ctx, "/api/projects/"+url.PathEscape(args[0])+"/check-trust", &result)
			case "settings":
				if cmd.Flags().Changed("sample-every") {
					every, _ := cmd.Flags().GetInt("sample-every")
					err = client.PutJSON(ctx, "/api/todos/check-settings", map[string]int{"sample_every": every}, &result)
				} else {
					err = client.GetJSON(ctx, "/api/todos/check-settings", &result)
				}
			case "answer":
				answer, _ := cmd.Flags().GetString("answer")
				note, _ := cmd.Flags().GetString("note")
				err = client.PostJSON(ctx, "/api/todos/"+url.PathEscape(args[0])+"/answer", map[string]string{"answer": answer, "note": note}, &result)
			case "instruction-note", "undo-result":
				body := map[string]string{}
				for _, flag := range []string{"link", "note"} { body[flag], _ = cmd.Flags().GetString(flag) }
				path := "/api/todos/"+url.PathEscape(args[0])+"/undo-result"
				if action == "instruction-note" {
					body["file"], _ = cmd.Flags().GetString("file")
					path = "/api/todos/checks/"+url.PathEscape(args[0])+"/instruction-note"
				}
				err = client.PostJSON(ctx, path, body, &result)
			}
			if err != nil {
				return err
			}
			if output == "json" {
				return cli.PrintJSON(cmd.OutOrStdout(), result)
			}
			switch value := result.(type) {
			case []any:
				rows := make([][]string, 0, len(value))
				for _, raw := range value {
					if state, ok := raw.(map[string]any); ok {
						rows = append(rows, []string{strVal(state, "kind"), fmt.Sprint(state["sampling"]), fmt.Sprint(state["passed"]), fmt.Sprint(state["feedback"]), fmt.Sprint(state["automatic_passed"])})
					}
				}
				cli.PrintTable(cmd.OutOrStdout(), []string{"KIND", "SAMPLES", "PASSED", "FEEDBACK", "AUTO PASSED"}, rows)
			case map[string]any:
				if action == "settings" {
					cli.PrintTable(cmd.OutOrStdout(), []string{"SAMPLE EVERY"}, [][]string{{fmt.Sprint(value["sample_every"])}})
				} else {
					details, _ := value["details"].(map[string]any)
					cli.PrintTable(cmd.OutOrStdout(), []string{"ID", "KIND", "ANSWER", "NOTE"}, [][]string{{strVal(value, "id"), strVal(details, "check_kind"), strVal(details, "answer"), strVal(details, "note")}})
				}
			}
			return nil
		}}
		sub.Flags().String("output", "table", "Output format: table or json")
		if action == "answer" {
			sub.Flags().String("answer", "", "pass, feedback, keep or undo")
			sub.Flags().String("note", "", "Feedback note")
			_ = sub.MarkFlagRequired("answer")
		}
		if action == "settings" {
			sub.Flags().Int("sample-every", 5, "Sample every N checks in samples mode (1 to 100)")
		}
		if action == "instruction-note" || action == "undo-result" {
			for _, flag := range []string{"link", "note"} {
				sub.Flags().String(flag, "", "Instruction "+flag)
				_ = sub.MarkFlagRequired(flag)
			}
			if action == "instruction-note" {
				sub.Flags().String("file", "", "Instruction file path")
				_ = sub.MarkFlagRequired("file")
			}
		}
		command.AddCommand(sub)
	}
	return command
}

func runTodoCheck(cmd *cobra.Command, _ []string) error {
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
	body := map[string]string{}
	for flag, key := range map[string]string{"issue": "issue_id", "project": "project_id", "kind": "kind", "link": "link", "title": "title"} {
		body[key], _ = cmd.Flags().GetString(flag)
	}
	answerer, _ := cmd.Flags().GetString("answerer")
	_, body["member_id"], err = resolveAssignee(ctx, client, answerer, assigneeKinds{member: true})
	if err != nil {
		return err
	}
	var result map[string]any
	if err := client.PostJSON(ctx, "/api/todos/checks", body, &result); err != nil {
		return err
	}
	if output == "json" {
		return cli.PrintJSON(cmd.OutOrStdout(), result)
	}
	details, _ := result["details"].(map[string]any)
	cli.PrintTable(cmd.OutOrStdout(), []string{"ID", "KIND", "ANSWER", "AUTOMATIC", "SAMPLED"}, [][]string{{strVal(result, "id"), strVal(details, "check_kind"), strVal(details, "answer"), fmt.Sprint(details["automatic"]), fmt.Sprint(details["sampled"])}})
	return nil
}
