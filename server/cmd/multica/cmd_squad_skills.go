package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

// ── Skills ──────────────────────────────────────────────────────────────────

var squadSkillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Work with a squad's assigned workspace skills and runtime skills",
}

var squadSkillsListCmd = &cobra.Command{
	Use:   "list <squad-id>",
	Short: "List the workspace skills assigned to a squad",
	Args:  exactArgs(1),
	RunE:  runSquadSkillsList,
}

var squadSkillsSetCmd = &cobra.Command{
	Use:   "set <squad-id>",
	Short: "Replace the workspace skills assigned to a squad",
	Args:  exactArgs(1),
	RunE:  runSquadSkillsSet,
}

var squadSkillsAddCmd = &cobra.Command{
	Use:   "add <squad-id>",
	Short: "Assign workspace skills to a squad next to the ones it has",
	Args:  exactArgs(1),
	RunE:  runSquadSkillsAdd,
}

var squadSkillsRemoveCmd = &cobra.Command{
	Use:   "remove <squad-id> <skill-id>",
	Short: "Remove an assigned workspace skill from a squad",
	Args:  exactArgs(2),
	RunE:  runSquadSkillsRemove,
}

var squadSkillsEnableCmd = &cobra.Command{
	Use:   "enable <squad-id> <skill-id>",
	Short: "Turn on an assigned workspace skill of a squad",
	Args:  exactArgs(2),
	RunE:  runSquadSkillsEnable,
}

var squadSkillsDisableCmd = &cobra.Command{
	Use:   "disable <squad-id> <skill-id>",
	Short: "Turn off an assigned workspace skill of a squad",
	Args:  exactArgs(2),
	RunE:  runSquadSkillsDisable,
}

func runSquadSkillsList(cmd *cobra.Command, args []string) error {
	return squadSkillsRequest(cmd, args[0], "list squad skills", func(ctx context.Context, client *cli.APIClient, out *json.RawMessage) error {
		return client.GetJSON(ctx, "/api/squads/"+args[0]+"/skills", out)
	})
}

func runSquadSkillsSet(cmd *cobra.Command, args []string) error {
	if !cmd.Flags().Changed("skill-ids") {
		return fmt.Errorf("--skill-ids is required (comma-separated skill IDs; use --skill-ids '' to clear all)")
	}
	body := map[string]any{"skill_ids": cleanSkillIDsFlag(cmd)}
	return squadSkillsRequest(cmd, args[0], "set squad skills", func(ctx context.Context, client *cli.APIClient, out *json.RawMessage) error {
		return client.PutJSON(ctx, "/api/squads/"+args[0]+"/skills", body, out)
	})
}

func runSquadSkillsAdd(cmd *cobra.Command, args []string) error {
	if !cmd.Flags().Changed("skill-ids") {
		return fmt.Errorf("--skill-ids is required (comma-separated skill IDs)")
	}
	skillIDs := cleanSkillIDsFlag(cmd)
	if len(skillIDs) == 0 {
		return fmt.Errorf("--skill-ids must include at least one skill ID")
	}
	body := map[string]any{"skill_ids": skillIDs}
	return squadSkillsRequest(cmd, args[0], "add squad skills", func(ctx context.Context, client *cli.APIClient, out *json.RawMessage) error {
		return client.PostJSON(ctx, "/api/squads/"+args[0]+"/skills/add", body, out)
	})
}

func runSquadSkillsRemove(cmd *cobra.Command, args []string) error {
	return squadSkillsRequest(cmd, args[0], "remove squad skill", func(ctx context.Context, client *cli.APIClient, out *json.RawMessage) error {
		return client.DeleteJSONResponse(ctx, "/api/squads/"+args[0]+"/skills/"+args[1], out)
	})
}

func runSquadSkillsEnable(cmd *cobra.Command, args []string) error {
	return setSquadSkillEnabled(cmd, args, true)
}

func runSquadSkillsDisable(cmd *cobra.Command, args []string) error {
	return setSquadSkillEnabled(cmd, args, false)
}

func setSquadSkillEnabled(cmd *cobra.Command, args []string, enabled bool) error {
	body := map[string]any{"enabled": enabled}
	return squadSkillsRequest(cmd, args[0], "update squad skill", func(ctx context.Context, client *cli.APIClient, out *json.RawMessage) error {
		return client.PutJSON(ctx, "/api/squads/"+args[0]+"/skills/"+args[1]+"/enabled", body, out)
	})
}

// squadSkillsRequest runs one squad skills call and prints the squad's
// assigned skills it answers with.
func squadSkillsRequest(cmd *cobra.Command, squadID, action string, call func(context.Context, *cli.APIClient, *json.RawMessage) error) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	var result json.RawMessage
	if err := call(ctx, client, &result); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}

	var skills []map[string]any
	if err := json.Unmarshal(result, &skills); err != nil {
		return fmt.Errorf("decode squad skills response: %w", err)
	}
	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, skills)
	}
	if len(skills) == 0 {
		fmt.Fprintf(os.Stderr, "No skills assigned to squad %s.\n", squadID)
		return nil
	}
	headers := []string{"ID", "NAME", "ENABLED", "DESCRIPTION"}
	rows := make([][]string, 0, len(skills))
	for _, s := range skills {
		enabled, _ := s["enabled"].(bool)
		rows = append(rows, []string{strVal(s, "id"), strVal(s, "name"), onOff(enabled), strVal(s, "description")})
	}
	cli.PrintTable(os.Stdout, headers, rows)
	return nil
}

func onOff(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

// ── Runtime Skills ──────────────────────────────────────────────────────────

var squadRuntimeSkillsCmd = &cobra.Command{
	Use:   "runtime",
	Short: "Work with the runtime skills a squad gives its runs",
	Long: "A squad gives the skills found on a runtime when its section switch is on, " +
		"minus the skills turned off. With the switch off it gives only the skills turned on. " +
		"Flipping the switch clears every per-skill choice.",
}

var squadRuntimeSkillsListCmd = &cobra.Command{
	Use:   "list <squad-id>",
	Short: "Show a squad's runtime skill section switch and per-skill choices",
	Args:  exactArgs(1),
	RunE:  runSquadRuntimeSkillsList,
}

var squadRuntimeSkillEnableCmd = &cobra.Command{
	Use:   "enable <squad-id>",
	Short: "Turn on one skill of a local runtime for a squad",
	Args:  exactArgs(1),
	RunE:  runSquadRuntimeSkillEnable,
}

var squadRuntimeSkillDisableCmd = &cobra.Command{
	Use:   "disable <squad-id>",
	Short: "Turn off one skill of a local runtime for a squad",
	Args:  exactArgs(1),
	RunE:  runSquadRuntimeSkillDisable,
}

var squadRuntimeSkillsSectionCmd = &cobra.Command{
	Use:   "section <squad-id> <on|off>",
	Short: "Turn every runtime skill of a squad on or off",
	Args:  exactArgs(2),
	RunE:  runSquadRuntimeSkillsSection,
}

func addSquadRuntimeSkillFlags(cmd *cobra.Command) {
	cmd.Flags().String("runtime-id", "", "Runtime the skill is found on (required)")
	cmd.Flags().String("root", "provider", "Skill root: provider, universal or plugin")
	cmd.Flags().String("key", "", "Skill key: its folder path below the root (required)")
	cmd.Flags().String("name", "", "Skill name to show")
	cmd.Flags().String("plugin", "", "Plugin that holds the skill (plugin root only)")
}

func runSquadRuntimeSkillsList(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	var squad map[string]any
	if err := client.GetJSON(ctx, "/api/squads/"+args[0], &squad); err != nil {
		return fmt.Errorf("get squad: %w", err)
	}
	inherit, _ := squad["inherit_runtime_skills"].(bool)
	exceptions, _ := squad["runtime_skill_exceptions"].([]any)
	if exceptions == nil {
		exceptions = []any{}
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, map[string]any{
			"squad_id":                 args[0],
			"inherit_runtime_skills":   inherit,
			"runtime_skill_exceptions": exceptions,
		})
	}
	fmt.Printf("Runtime skills section: %s\n", onOff(inherit))
	if len(exceptions) == 0 {
		fmt.Println("No per-skill choices.")
		return nil
	}
	headers := []string{"RUNTIME ID", "PROVIDER", "ROOT", "KEY", "PLUGIN", "NAME", "STATE"}
	rows := make([][]string, 0, len(exceptions))
	for _, raw := range exceptions {
		e, _ := raw.(map[string]any)
		rows = append(rows, []string{
			strVal(e, "runtime_id"), strVal(e, "provider"), strVal(e, "root"),
			strVal(e, "key"), strVal(e, "plugin"), strVal(e, "name"), onOff(!inherit),
		})
	}
	cli.PrintTable(os.Stdout, headers, rows)
	return nil
}

func runSquadRuntimeSkillEnable(cmd *cobra.Command, args []string) error {
	return setSquadRuntimeSkill(cmd, args[0], true)
}

func runSquadRuntimeSkillDisable(cmd *cobra.Command, args []string) error {
	return setSquadRuntimeSkill(cmd, args[0], false)
}

func setSquadRuntimeSkill(cmd *cobra.Command, squadID string, enabled bool) error {
	runtimeID, _ := cmd.Flags().GetString("runtime-id")
	root, _ := cmd.Flags().GetString("root")
	key, _ := cmd.Flags().GetString("key")
	name, _ := cmd.Flags().GetString("name")
	plugin, _ := cmd.Flags().GetString("plugin")
	if strings.TrimSpace(runtimeID) == "" {
		return fmt.Errorf("--runtime-id is required")
	}
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("--key is required")
	}
	body := map[string]any{
		"runtime_id": runtimeID,
		"root":       root,
		"key":        key,
		"name":       name,
		"plugin":     plugin,
		"enabled":    enabled,
	}
	return putSquadRuntimeSkills(cmd, squadID, "/runtime-skills/enabled", body,
		fmt.Sprintf("Runtime skill %s turned %s for squad %s.", key, onOff(enabled), squadID))
}

func runSquadRuntimeSkillsSection(cmd *cobra.Command, args []string) error {
	var enabled bool
	switch args[1] {
	case "on":
		enabled = true
	case "off":
	default:
		return fmt.Errorf("section must be 'on' or 'off'")
	}
	return putSquadRuntimeSkills(cmd, args[0], "/runtime-skills/inherit", map[string]any{"enabled": enabled},
		fmt.Sprintf("Runtime skills section turned %s for squad %s.", args[1], args[0]))
}

func putSquadRuntimeSkills(cmd *cobra.Command, squadID, path string, body map[string]any, done string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	if err := client.PutJSON(ctx, "/api/squads/"+squadID+path, body, nil); err != nil {
		return fmt.Errorf("update squad runtime skills: %w", err)
	}
	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		result := map[string]any{"squad_id": squadID}
		for k, v := range body {
			result[k] = v
		}
		return cli.PrintJSON(os.Stdout, result)
	}
	fmt.Fprintln(os.Stderr, done)
	return nil
}

func init() {
	for _, cmd := range []*cobra.Command{
		squadSkillsListCmd, squadSkillsSetCmd, squadSkillsAddCmd, squadSkillsRemoveCmd,
		squadSkillsEnableCmd, squadSkillsDisableCmd,
	} {
		cmd.Flags().String("output", "table", "Output format: table or json")
	}
	squadSkillsSetCmd.Flags().StringSlice("skill-ids", nil, "Comma-separated skill IDs")
	squadSkillsAddCmd.Flags().StringSlice("skill-ids", nil, "Comma-separated skill IDs")

	for _, cmd := range []*cobra.Command{
		squadRuntimeSkillsListCmd, squadRuntimeSkillEnableCmd, squadRuntimeSkillDisableCmd, squadRuntimeSkillsSectionCmd,
	} {
		cmd.Flags().String("output", "table", "Output format: table or json")
	}
	addSquadRuntimeSkillFlags(squadRuntimeSkillEnableCmd)
	addSquadRuntimeSkillFlags(squadRuntimeSkillDisableCmd)

	squadRuntimeSkillsCmd.AddCommand(squadRuntimeSkillsListCmd)
	squadRuntimeSkillsCmd.AddCommand(squadRuntimeSkillEnableCmd)
	squadRuntimeSkillsCmd.AddCommand(squadRuntimeSkillDisableCmd)
	squadRuntimeSkillsCmd.AddCommand(squadRuntimeSkillsSectionCmd)

	squadSkillsCmd.AddCommand(squadSkillsListCmd)
	squadSkillsCmd.AddCommand(squadSkillsSetCmd)
	squadSkillsCmd.AddCommand(squadSkillsAddCmd)
	squadSkillsCmd.AddCommand(squadSkillsRemoveCmd)
	squadSkillsCmd.AddCommand(squadSkillsEnableCmd)
	squadSkillsCmd.AddCommand(squadSkillsDisableCmd)
	squadSkillsCmd.AddCommand(squadRuntimeSkillsCmd)

	squadCmd.AddCommand(squadSkillsCmd)
}
