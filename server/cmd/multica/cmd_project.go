package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Work with projects",
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects in the workspace",
	RunE:  runProjectList,
}

var projectGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get project details",
	Args:  exactArgs(1),
	RunE:  runProjectGet,
}

var projectCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new project",
	RunE:  runProjectCreate,
}

var projectUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a project",
	Args:  exactArgs(1),
	RunE:  runProjectUpdate,
}

var projectDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a project",
	Args:  exactArgs(1),
	RunE:  runProjectDelete,
}

var projectStatusCmd = &cobra.Command{
	Use:   "status <id> <status>",
	Short: "Change project status",
	Args:  exactArgs(2),
	RunE:  runProjectStatus,
}

var validProjectStatuses = []string{
	"planned", "in_progress", "paused", "completed", "cancelled",
}

// validateProjectStatus rejects unknown statuses client-side so a typo fails
// fast with the valid list instead of a server round-trip and a 400. Shared by
// `project create`, `project update`, and `project status`.
func validateProjectStatus(status string) error {
	for _, s := range validProjectStatuses {
		if s == status {
			return nil
		}
	}
	return fmt.Errorf("invalid status %q; valid values: %s", status, strings.Join(validProjectStatuses, ", "))
}

func init() {
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectGetCmd)
	projectCmd.AddCommand(projectCreateCmd)
	projectCmd.AddCommand(projectUpdateCmd)
	projectCmd.AddCommand(projectDeleteCmd)
	projectCmd.AddCommand(projectStatusCmd)

	// project list
	projectListCmd.Flags().String("output", "table", "Output format: table or json")
	projectListCmd.Flags().Bool("full-id", false, "Show full UUIDs in table output")
	projectListCmd.Flags().String("status", "", "Filter by status")

	// project get
	projectGetCmd.Flags().String("output", "json", "Output format: table or json")

	// project create
	projectCreateCmd.Flags().String("title", "", "Project title (required)")
	projectCreateCmd.Flags().String("description", "", "Project description")
	projectCreateCmd.Flags().String("status", "", "Project status")
	projectCreateCmd.Flags().String("icon", "", "Project icon (emoji)")
	projectCreateCmd.Flags().String("lead", "", "Lead name (member or agent)")
	projectCreateCmd.Flags().String("start-date", "", "Start date (calendar day, YYYY-MM-DD)")
	projectCreateCmd.Flags().String("due-date", "", "Due date (calendar day, YYYY-MM-DD)")
	projectCreateCmd.Flags().String("output", "json", "Output format: table or json")

	// project update
	projectUpdateCmd.Flags().String("title", "", "New title")
	projectUpdateCmd.Flags().String("description", "", "New description")
	projectUpdateCmd.Flags().String("status", "", "New status")
	projectUpdateCmd.Flags().String("icon", "", "New icon (emoji)")
	projectUpdateCmd.Flags().String("lead", "", "New lead name (member or agent)")
	projectUpdateCmd.Flags().String("start-date", "", "New start date (calendar day, YYYY-MM-DD; pass empty string to clear)")
	projectUpdateCmd.Flags().String("due-date", "", "New due date (calendar day, YYYY-MM-DD; pass empty string to clear)")
	projectUpdateCmd.Flags().String("output", "json", "Output format: table or json")

	// project delete
	projectDeleteCmd.Flags().String("output", "json", "Output format: table or json")

	// project status
	projectStatusCmd.Flags().String("output", "table", "Output format: table or json")
}

// ---------------------------------------------------------------------------
// Project commands
// ---------------------------------------------------------------------------

func runProjectList(cmd *cobra.Command, _ []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	params := url.Values{}
	if client.WorkspaceID != "" {
		params.Set("workspace_id", client.WorkspaceID)
	}
	if v, _ := cmd.Flags().GetString("status"); v != "" {
		params.Set("status", v)
	}

	path := "/api/projects"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var result map[string]any
	if err := client.GetJSON(ctx, path, &result); err != nil {
		return fmt.Errorf("list projects: %w", err)
	}

	projectsRaw, _ := result["projects"].([]any)

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, projectsRaw)
	}

	fullID, _ := cmd.Flags().GetBool("full-id")
	actors := loadActorDisplayLookup(ctx, client)
	headers := []string{"ID", "TITLE", "STATUS", "LEAD", "CREATED"}
	rows := make([][]string, 0, len(projectsRaw))
	for _, raw := range projectsRaw {
		p, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		lead := formatLead(p, actors)
		created := strVal(p, "created_at")
		if len(created) >= 10 {
			created = created[:10]
		}
		rows = append(rows, []string{
			displayID(strVal(p, "id"), fullID),
			strVal(p, "title"),
			strVal(p, "status"),
			lead,
			created,
		})
	}
	cli.PrintTable(os.Stdout, headers, rows)
	return nil
}

func runProjectGet(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	projectRef, err := resolveProjectID(ctx, client, args[0])
	if err != nil {
		return fmt.Errorf("resolve project: %w", err)
	}

	var project map[string]any
	if err := client.GetJSON(ctx, "/api/projects/"+projectRef.ID, &project); err != nil {
		return fmt.Errorf("get project: %w", err)
	}

	// Breadcrumb to the resources sub-collection. Goes to stderr so JSON on
	// stdout stays parseable; the `resource_count` field on the response is
	// the programmatic equivalent. JSON numbers decode as float64.
	if n, _ := project["resource_count"].(float64); n > 0 {
		fmt.Fprintf(os.Stderr, "%d resource(s) attached — run `multica project resource list %s` to view.\n",
			int64(n), strVal(project, "id"))
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "table" {
		actors := loadActorDisplayLookup(ctx, client)
		lead := formatLead(project, actors)
		headers := []string{"ID", "TITLE", "STATUS", "LEAD", "DESCRIPTION"}
		rows := [][]string{{
			strVal(project, "id"),
			strVal(project, "title"),
			strVal(project, "status"),
			lead,
			strVal(project, "description"),
		}}
		cli.PrintTable(os.Stdout, headers, rows)
		return nil
	}

	return cli.PrintJSON(os.Stdout, project)
}

func runProjectCreate(cmd *cobra.Command, _ []string) error {
	title, _ := cmd.Flags().GetString("title")
	if title == "" {
		return fmt.Errorf("--title is required")
	}

	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	body := map[string]any{"title": title}
	if v, _ := cmd.Flags().GetString("description"); v != "" {
		body["description"] = v
	}
	if v, _ := cmd.Flags().GetString("status"); v != "" {
		if err := validateProjectStatus(v); err != nil {
			return err
		}
		body["status"] = v
	}
	if v, _ := cmd.Flags().GetString("icon"); v != "" {
		body["icon"] = v
	}
	if v, _ := cmd.Flags().GetString("lead"); v != "" {
		aType, aID, resolveErr := resolveAssignee(ctx, client, v, memberOrAgentKinds)
		if resolveErr != nil {
			return fmt.Errorf("resolve lead: %w", resolveErr)
		}
		body["lead_type"] = aType
		body["lead_id"] = aID
	}
	if v, _ := cmd.Flags().GetString("start-date"); v != "" {
		body["start_date"] = v
	}
	if v, _ := cmd.Flags().GetString("due-date"); v != "" {
		body["due_date"] = v
	}

	var result map[string]any
	if err := client.PostJSON(ctx, "/api/projects", body, &result); err != nil {
		return fmt.Errorf("create project: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "table" {
		headers := []string{"ID", "TITLE", "STATUS"}
		rows := [][]string{{
			strVal(result, "id"),
			strVal(result, "title"),
			strVal(result, "status"),
		}}
		cli.PrintTable(os.Stdout, headers, rows)
		return nil
	}

	return cli.PrintJSON(os.Stdout, result)
}

func runProjectUpdate(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	projectRef, err := resolveProjectID(ctx, client, args[0])
	if err != nil {
		return fmt.Errorf("resolve project: %w", err)
	}

	body := map[string]any{}
	if cmd.Flags().Changed("title") {
		v, _ := cmd.Flags().GetString("title")
		body["title"] = v
	}
	if cmd.Flags().Changed("description") {
		v, _ := cmd.Flags().GetString("description")
		body["description"] = v
	}
	if cmd.Flags().Changed("status") {
		v, _ := cmd.Flags().GetString("status")
		if err := validateProjectStatus(v); err != nil {
			return err
		}
		body["status"] = v
	}
	if cmd.Flags().Changed("icon") {
		v, _ := cmd.Flags().GetString("icon")
		body["icon"] = v
	}
	if cmd.Flags().Changed("lead") {
		v, _ := cmd.Flags().GetString("lead")
		aType, aID, resolveErr := resolveAssignee(ctx, client, v, memberOrAgentKinds)
		if resolveErr != nil {
			return fmt.Errorf("resolve lead: %w", resolveErr)
		}
		body["lead_type"] = aType
		body["lead_id"] = aID
	}
	// Changed() (not "") so an explicit --start-date "" reaches the server as a
	// clear, mirroring the issue update CLI.
	if cmd.Flags().Changed("start-date") {
		v, _ := cmd.Flags().GetString("start-date")
		body["start_date"] = v
	}
	if cmd.Flags().Changed("due-date") {
		v, _ := cmd.Flags().GetString("due-date")
		body["due_date"] = v
	}

	if len(body) == 0 {
		return fmt.Errorf("no fields to update; use flags like --title, --status, --description, --icon, --lead, --start-date, --due-date")
	}

	var result map[string]any
	if err := client.PutJSON(ctx, "/api/projects/"+projectRef.ID, body, &result); err != nil {
		return fmt.Errorf("update project: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "table" {
		headers := []string{"ID", "TITLE", "STATUS"}
		rows := [][]string{{
			strVal(result, "id"),
			strVal(result, "title"),
			strVal(result, "status"),
		}}
		cli.PrintTable(os.Stdout, headers, rows)
		return nil
	}

	return cli.PrintJSON(os.Stdout, result)
}

func runProjectDelete(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	projectRef, err := resolveProjectID(ctx, client, args[0])
	if err != nil {
		return fmt.Errorf("resolve project: %w", err)
	}

	if err := client.DeleteJSON(ctx, "/api/projects/"+projectRef.ID); err != nil {
		return fmt.Errorf("delete project: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Project %s deleted.\n", projectRef.Display)
	return nil
}

func runProjectStatus(cmd *cobra.Command, args []string) error {
	id := args[0]
	status := args[1]

	if err := validateProjectStatus(status); err != nil {
		return err
	}

	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	projectRef, err := resolveProjectID(ctx, client, id)
	if err != nil {
		return fmt.Errorf("resolve project: %w", err)
	}

	body := map[string]any{"status": status}
	var result map[string]any
	if err := client.PutJSON(ctx, "/api/projects/"+projectRef.ID, body, &result); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Project %s status changed to %s.\n", strVal(result, "title"), status)

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, result)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func formatLead(project map[string]any, actors actorDisplayLookup) string {
	lType := strVal(project, "lead_type")
	lID := strVal(project, "lead_id")
	if lType == "" || lID == "" {
		return ""
	}
	return actors.actor(lType, lID)
}
