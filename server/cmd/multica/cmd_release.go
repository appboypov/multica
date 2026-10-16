package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/spf13/cobra"
)

var releaseCmd = newReleaseCommand()

type releaseMilestoneDTO struct {
	ID        string `json:"id"`
	ReleaseID string `json:"release_id"`
	Name      string `json:"name"`
	Date      string `json:"date"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
type releaseDTO struct {
	ID             string                `json:"id"`
	RepositoryID   string                `json:"repository_id"`
	Position       int                   `json:"position"`
	Version        string                `json:"version"`
	Bump           *string               `json:"bump"`
	Title          string                `json:"title"`
	Description    string                `json:"description"`
	ShippedAt      *string               `json:"shipped_at"`
	CreatedAt      string                `json:"created_at"`
	UpdatedAt      string                `json:"updated_at"`
	Milestones     []releaseMilestoneDTO `json:"milestones"`
	OwnerType      *string               `json:"owner_type"`
	OwnerID        *string               `json:"owner_id"`
	OwnerName      *string               `json:"owner_name"`
	OwnerAvatarURL *string               `json:"owner_avatar_url"`
}
type releaseListDTO struct {
	Releases []releaseDTO `json:"releases"`
}

func newReleaseCommand() *cobra.Command {
	root := &cobra.Command{Use: "release", Short: "Plan and manage repository releases", Example: "  multica release list --repository owner/repo\n  multica release milestone add <release> --name \"UI prototype\" --date 2026-11-02"}
	for _, spec := range []struct {
		use, short string
		args       cobra.PositionalArgs
	}{
		{"list", "List releases, optionally by repository or project", cobra.NoArgs},
		{"get <release>", "Show a release and its milestones", exactArgs(1)},
		{"plan <repository>", "Plan a release on a repository", exactArgs(1)},
		{"edit <release>", "Edit a release", exactArgs(1)},
		{"move <release>", "Move a release earlier or later", exactArgs(1)},
		{"remove <release>", "Remove an unshipped release", exactArgs(1)},
		{"ship <release>", "Mark the next release shipped after publication", exactArgs(1)},
		{"assign <release>", "Assign an agent or squad as release owner", exactArgs(1)},
		{"add-issues <release> <issue>...", "Add issues to a release", cobra.MinimumNArgs(2)},
	} {
		cmd := &cobra.Command{Use: spec.use, Short: spec.short, Args: spec.args, RunE: runRelease}
		cmd.Flags().String("output", "table", "Output format: table or json")
		cmd.Flags().Bool("full-id", false, "Show full UUIDs in table output")
		switch cmd.Name() {
		case "list":
			cmd.Flags().String("repository", "", "Repository full name, UUID or UUID prefix")
			cmd.Flags().String("project", "", "Project UUID or UUID prefix")
		case "plan", "edit":
			cmd.Flags().String("version", "", "Explicit version (instead of --bump)")
			cmd.Flags().String("bump", "", "Version bump: patch, minor or major")
			cmd.Flags().String("title", "", "Release title")
			cmd.Flags().String("description", "", "Release description")
		case "assign":
			cmd.Flags().String("to", "", "Agent or squad name, UUID or UUID prefix")
			cmd.Flags().Bool("unassign", false, "Remove the release owner")
			cmd.MarkFlagsMutuallyExclusive("to", "unassign")
			cmd.MarkFlagsOneRequired("to", "unassign")
		case "move":
			cmd.Flags().String("direction", "", "earlier or later")
		}
		root.AddCommand(cmd)
	}
	milestone := &cobra.Command{Use: "milestone", Short: "Manage a release's named calendar-day milestones"}
	for _, action := range []string{"add", "edit", "remove"} {
		use := action + " <milestone>"
		if action == "add" {
			use = action + " <release>"
		}
		cmd := &cobra.Command{Use: use, Short: strings.ToUpper(action[:1]) + action[1:] + " a milestone", Args: exactArgs(1), RunE: runReleaseMilestone}
		cmd.Flags().String("output", "table", "Output format: table or json")
		cmd.Flags().Bool("full-id", false, "Show full UUIDs in table output")
		if action != "remove" {
			cmd.Flags().String("name", "", "Milestone name")
			cmd.Flags().String("date", "", "Calendar day in YYYY-MM-DD format")
		}
		milestone.AddCommand(cmd)
	}
	root.AddCommand(milestone)
	root.AddCommand(newReleaseCommentCommand(), newReleaseWakeupCommand())
	return root
}

func fetchReleases(ctx context.Context, client *cli.APIClient) (releaseListDTO, error) {
	var out releaseListDTO
	err := client.GetJSON(ctx, "/api/releases", &out)
	return out, err
}
func resolveReleaseID(ctx context.Context, client *cli.APIClient, input string) (resolvedID, error) {
	return resolveIDByPrefix(ctx, client, "release", input, func(ctx context.Context, client *cli.APIClient) ([]idCandidate, error) {
		out, err := fetchReleases(ctx, client)
		if err != nil {
			return nil, err
		}
		rows := make([]idCandidate, len(out.Releases))
		for i, r := range out.Releases {
			rows[i] = idCandidate{ID: r.ID, Display: "v" + r.Version, Detail: r.Title}
		}
		return rows, nil
	})
}
func resolveReleaseRepositoryID(ctx context.Context, client *cli.APIClient, input string) (resolvedID, error) {
	if uuidRegexp.MatchString(input) {
		return resolvedID{ID: input, Display: input}, nil
	}
	var out struct {
		Repositories []issuePlacementRepository `json:"repositories"`
	}
	if err := client.GetJSON(ctx, "/api/repositories", &out); err != nil {
		return resolvedID{}, err
	}
	for _, repo := range out.Repositories {
		if repo.FullName == input {
			return resolvedID{ID: repo.ID, Display: repo.FullName}, nil
		}
	}
	return resolveIDByPrefix(ctx, client, "repository", input, func(context.Context, *cli.APIClient) ([]idCandidate, error) {
		rows := make([]idCandidate, len(out.Repositories))
		for i, r := range out.Repositories {
			rows[i] = idCandidate{ID: r.ID, Display: r.FullName}
		}
		return rows, nil
	})
}

func runRelease(cmd *cobra.Command, args []string) error {
	output, _ := cmd.Flags().GetString("output")
	if output != "table" && output != "json" {
		return fmt.Errorf("--output must be table or json")
	}
	client, _, err := repoCommandClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()
	action := cmd.Name()
	if action == "list" {
		out, err := fetchReleases(ctx, client)
		if err != nil {
			return err
		}
		repository, _ := cmd.Flags().GetString("repository")
		project, _ := cmd.Flags().GetString("project")
		allowed := map[string]bool{}
		if project != "" {
			ref, err := resolveProjectID(ctx, client, project)
			if err != nil {
				return err
			}
			var repos struct {
				Repositories []issuePlacementRepository `json:"repositories"`
			}
			if err := client.GetJSON(ctx, "/api/repositories", &repos); err != nil {
				return err
			}
			for _, r := range repos.Repositories {
				if r.ProjectID == ref.ID {
					allowed[r.ID] = true
				}
			}
		}
		if repository != "" {
			ref, err := resolveReleaseRepositoryID(ctx, client, repository)
			if err != nil {
				return err
			}
			repository = ref.ID
		}
		rows := []releaseDTO{}
		for _, r := range out.Releases {
			if repository != "" && r.RepositoryID != repository || project != "" && !allowed[r.RepositoryID] {
				continue
			}
			rows = append(rows, r)
		}
		return printReleases(cmd, rows, false)
	}
	if action == "plan" {
		body, err := releaseWriteBody(cmd, nil)
		if err != nil {
			return err
		}
		repo, err := resolveReleaseRepositoryID(ctx, client, args[0])
		if err != nil {
			return err
		}
		var out releaseListDTO
		if err := client.PostJSON(ctx, "/api/repositories/"+repo.ID+"/releases", body, &out); err != nil {
			return err
		}
		return printReleases(cmd, out.Releases, false)
	}
	ref, err := resolveReleaseID(ctx, client, args[0])
	if err != nil {
		return err
	}
	path := "/api/releases/" + ref.ID
	if action == "assign" {
		body := map[string]any{"owner_type": nil, "owner_id": nil}
		if unassign, _ := cmd.Flags().GetBool("unassign"); !unassign {
			to, _ := cmd.Flags().GetString("to")
			if strings.TrimSpace(to) == "" {
				return fmt.Errorf("--to or --unassign is required")
			}
			kind, id, err := resolveAssignee(ctx, client, to, assigneeKinds{agent: true, squad: true})
			if err != nil {
				return err
			}
			body["owner_type"], body["owner_id"] = kind, id
		}
		var out releaseListDTO
		if err := client.PutJSON(ctx, path+"/owner", body, &out); err != nil {
			return err
		}
		return printReleases(cmd, out.Releases, false)
	}
	if action == "get" || action == "edit" {
		var rel releaseDTO
		if err := client.GetJSON(ctx, path, &rel); err != nil {
			return err
		}
		if action == "get" {
			return printReleases(cmd, []releaseDTO{rel}, true)
		}
		body, err := releaseWriteBody(cmd, &rel)
		if err != nil {
			return err
		}
		var out releaseListDTO
		if err := client.PatchJSON(ctx, path, body, &out); err != nil {
			return err
		}
		return printReleases(cmd, out.Releases, false)
	}
	if action == "add-issues" {
		ids := make([]string, len(args)-1)
		for i, input := range args[1:] {
			issue, err := resolveIssueRef(ctx, client, input)
			if err != nil {
				return err
			}
			ids[i] = issue.ID
		}
		var out struct {
			Issues []map[string]any `json:"issues"`
		}
		if err := client.PostJSON(ctx, path+"/issues", map[string]any{"issue_ids": ids}, &out); err != nil {
			return issuePlacementCommandError(err)
		}
		if output == "json" {
			return cli.PrintJSON(os.Stdout, out)
		}
		rows := make([][]string, len(out.Issues))
		for i, issue := range out.Issues {
			rows[i] = []string{strVal(issue, "identifier"), strVal(issue, "title")}
		}
		cli.PrintTable(os.Stdout, []string{"ISSUE", "TITLE"}, rows)
		return nil
	}
	var out releaseListDTO
	if action == "move" {
		direction, _ := cmd.Flags().GetString("direction")
		if direction != "earlier" && direction != "later" {
			return fmt.Errorf("--direction must be earlier or later")
		}
		err = client.PostJSON(ctx, path+"/move", map[string]string{"direction": direction}, &out)
	} else if action == "ship" {
		err = client.PostJSON(ctx, path+"/ship", nil, &out)
	} else {
		err = client.DeleteJSONResponse(ctx, path, &out)
	}
	if err != nil {
		return err
	}
	return printReleases(cmd, out.Releases, false)
}

func releaseWriteBody(cmd *cobra.Command, current *releaseDTO) (map[string]any, error) {
	body := map[string]any{"description": ""}
	if current != nil {
		body["title"], body["description"] = current.Title, current.Description
		if current.Bump != nil {
			body["bump"] = *current.Bump
		} else {
			body["version"] = current.Version
		}
	}
	for _, key := range []string{"title", "description", "version", "bump"} {
		if cmd.Flags().Changed(key) {
			value, _ := cmd.Flags().GetString(key)
			body[key] = value
		}
	}
	if cmd.Flags().Changed("version") && cmd.Flags().Changed("bump") {
		return nil, fmt.Errorf("send either --version or --bump")
	}
	if cmd.Flags().Changed("version") {
		delete(body, "bump")
	}
	if cmd.Flags().Changed("bump") {
		delete(body, "version")
	}
	title, _ := body["title"].(string)
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("--title is required")
	}
	if body["version"] == nil && body["bump"] == nil {
		return nil, fmt.Errorf("--version or --bump is required")
	}
	return body, nil
}

func runReleaseMilestone(cmd *cobra.Command, args []string) error {
	output, _ := cmd.Flags().GetString("output")
	if output != "table" && output != "json" {
		return fmt.Errorf("--output must be table or json")
	}
	action := cmd.Name()
	body := map[string]string{}
	if action != "remove" {
		for _, key := range []string{"name", "date"} {
			if cmd.Flags().Changed(key) {
				value, _ := cmd.Flags().GetString(key)
				body[key] = value
			}
		}
		if action == "add" && (strings.TrimSpace(body["name"]) == "" || body["date"] == "") {
			return fmt.Errorf("--name and --date are required")
		}
		if name, ok := body["name"]; ok && strings.TrimSpace(name) == "" {
			return fmt.Errorf("--name cannot be blank")
		}
		if date, ok := body["date"]; ok {
			day, err := time.Parse(time.DateOnly, date)
			if err != nil || day.Format(time.DateOnly) != date {
				return fmt.Errorf("--date must be a calendar day in YYYY-MM-DD format")
			}
		}
		if action == "edit" && len(body) == 0 {
			return fmt.Errorf("send --name or --date")
		}
	}
	client, _, err := repoCommandClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()
	var out releaseListDTO
	if action == "add" {
		ref, err := resolveReleaseID(ctx, client, args[0])
		if err != nil {
			return err
		}
		err = client.PostJSON(ctx, "/api/releases/"+ref.ID+"/milestones", body, &out)
		if err != nil {
			return err
		}
	} else {
		ref, err := resolveIDByPrefix(ctx, client, "milestone", args[0], func(ctx context.Context, client *cli.APIClient) ([]idCandidate, error) {
			out, err := fetchReleases(ctx, client)
			if err != nil {
				return nil, err
			}
			rows := []idCandidate{}
			for _, r := range out.Releases {
				for _, m := range r.Milestones {
					rows = append(rows, idCandidate{ID: m.ID, Display: m.Name, Detail: m.Date})
				}
			}
			return rows, nil
		})
		if err != nil {
			return err
		}
		path := "/api/release-milestones/" + ref.ID
		if action == "edit" {
			err = client.PatchJSON(ctx, path, body, &out)
		} else {
			err = client.DeleteJSONResponse(ctx, path, &out)
		}
		if err != nil {
			return err
		}
	}
	return printReleases(cmd, out.Releases, false)
}

func printReleases(cmd *cobra.Command, releases []releaseDTO, single bool) error {
	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		if single {
			return cli.PrintJSON(os.Stdout, releases[0])
		}
		return cli.PrintJSON(os.Stdout, releases)
	}
	full, _ := cmd.Flags().GetBool("full-id")
	rows := make([][]string, len(releases))
	for i, r := range releases {
		state := "planned"
		if r.ShippedAt != nil {
			state = "shipped"
		}
		owner := "—"
		if r.OwnerName != nil {
			owner = *r.OwnerName
		}
		rows[i] = []string{displayID(r.ID, full), displayID(r.RepositoryID, full), "v" + r.Version, r.Title, state, owner}
	}
	cli.PrintTable(os.Stdout, []string{"ID", "REPOSITORY", "VERSION", "TITLE", "STATE", "OWNER"}, rows)
	milestones := [][]string{}
	for _, r := range releases {
		for _, m := range r.Milestones {
			milestones = append(milestones, []string{displayID(m.ID, full), "v" + r.Version, m.Name, m.Date})
		}
	}
	if single && releases[0].Description != "" {
		fmt.Fprintln(os.Stdout, releases[0].Description)
	}
	if len(milestones) > 0 {
		cli.PrintTable(os.Stdout, []string{"MILESTONE ID", "RELEASE", "NAME", "DATE"}, milestones)
	}
	return nil
}
