package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/spf13/cobra"
)

var worklaneCmd = &cobra.Command{Use: "worklane", Short: "Work with repository worklanes", Long: "Work with repository worklanes and their fixed issue order.\nLane order follows the release line; move issues within the same release."}

func init() {
	for _, spec := range []struct {
		use, short string
		args       int
		run        func(*cobra.Command, []string) error
	}{
		{"list", "List worklanes by repository or project", 0, runWorklaneList},
		{"get <lane>", "Show a worklane and its next issue", 1, runWorklaneGet},
		{"issues <lane>", "List a worklane's issues in lane order", 1, runWorklaneIssues},
		{"rename <lane> <name>", "Rename a worklane without moving its worktree", 2, runWorklaneRename},
		{"remove <lane>", "Remove a worklane through its machine", 1, runWorklaneRemove},
		{"create <name>", "Create a worklane through its machine", 1, runWorklaneCreate},
		{"move <issue>", "Move an issue before or after another issue in its lane and release", 1, runWorklaneMove},
	} {
		cmd := &cobra.Command{Use: spec.use, Short: spec.short, Args: exactArgs(spec.args), RunE: spec.run}
		cmd.Flags().String("output", "table", "Output format: table or json")
		if !strings.HasPrefix(spec.use, "move") {
			cmd.Flags().String("repo", "", "Repository UUID or owner/name (required when a lane name is ambiguous)")
		}
		switch cmd.Name() {
		case "list":
			cmd.Flags().String("project", "", "Project UUID or name")
		case "create":
			cmd.Flags().String("folder", "", "Worktree folder name")
			cmd.Flags().String("daemon-id", "", "Machine daemon ID")
			cmd.MarkFlagRequired("repo")
			cmd.MarkFlagRequired("folder")
			cmd.MarkFlagRequired("daemon-id")
		case "move":
			cmd.Flags().String("before", "", "Issue key or UUID to move before")
			cmd.Flags().String("after", "", "Issue key or UUID to move after")
			cmd.MarkFlagsMutuallyExclusive("before", "after")
			cmd.MarkFlagsOneRequired("before", "after")
		}
		worklaneCmd.AddCommand(cmd)
	}
}

type cliWorklane struct {
	ID           string  `json:"id"`
	RepositoryID string  `json:"repository_id"`
	Name         string  `json:"name"`
	State        string  `json:"state"`
	Branch       *string `json:"branch"`
	Path         *string `json:"path"`
	IsMain       bool    `json:"is_main"`
	NextIssue    *struct {
		Identifier string `json:"identifier"`
	} `json:"next_issue"`
	ActiveIssue *struct {
		Identifier string `json:"identifier"`
	} `json:"active_issue"`
}

func laneRepositories(ctx context.Context, client *cli.APIClient) ([]issuePlacementRepository, error) {
	var response struct {
		Repositories []issuePlacementRepository `json:"repositories"`
	}
	if err := client.GetJSON(ctx, "/api/repositories", &response); err != nil {
		return nil, err
	}
	return response.Repositories, nil
}

func laneRepositoryID(repositories []issuePlacementRepository, input string) (string, error) {
	for _, repo := range repositories {
		if repo.ID == input || strings.EqualFold(repo.FullName, input) {
			return repo.ID, nil
		}
	}
	return "", fmt.Errorf("repository %q not found in this workspace", input)
}

func fetchWorklanes(ctx context.Context, client *cli.APIClient) ([]cliWorklane, error) {
	var response struct {
		Worklanes []cliWorklane `json:"worklanes"`
	}
	if err := client.GetJSON(ctx, "/api/worklanes", &response); err != nil {
		return nil, err
	}
	return response.Worklanes, nil
}

// Lane names are unique per repository, not per workspace. Never choose an ambiguous name.
func resolveCommandWorklane(ctx context.Context, client *cli.APIClient, input, repository string) (cliWorklane, error) {
	lanes, err := fetchWorklanes(ctx, client)
	if err != nil {
		return cliWorklane{}, err
	}
	repositories, err := laneRepositories(ctx, client)
	if err != nil {
		return cliWorklane{}, err
	}
	repositoryID := ""
	if repository != "" {
		repositoryID, err = laneRepositoryID(repositories, repository)
		if err != nil {
			return cliWorklane{}, err
		}
	}
	catalogue := issuePlacementCatalogue{repositories: repositories}
	for _, lane := range lanes {
		if repositoryID == "" || lane.RepositoryID == repositoryID {
			catalogue.worklanes = append(catalogue.worklanes, issuePlacementWorklane{ID: lane.ID, RepositoryID: lane.RepositoryID, Name: lane.Name})
		}
	}
	resolved, err := resolveWorklane(input, catalogue)
	if err != nil {
		return cliWorklane{}, err
	}
	if resolved != nil {
		for _, lane := range lanes {
			if lane.ID == resolved.ID {
				return lane, nil
			}
		}
	}
	return cliWorklane{}, fmt.Errorf("worklane %q not found", input)
}

func laneOutput(cmd *cobra.Command, lanes []cliWorklane, single bool) error {
	format, _ := cmd.Flags().GetString("output")
	if format == "json" {
		if single {
			return cli.PrintJSON(os.Stdout, lanes[0])
		}
		return cli.PrintJSON(os.Stdout, lanes)
	}
	rows := make([][]string, 0, len(lanes))
	for _, lane := range lanes {
		next, active := "", ""
		if lane.NextIssue != nil {
			next = lane.NextIssue.Identifier
		}
		if lane.ActiveIssue != nil {
			active = lane.ActiveIssue.Identifier
		}
		rows = append(rows, []string{lane.ID, lane.RepositoryID, lane.Name, lane.State, active, next})
	}
	cli.PrintTable(os.Stdout, []string{"ID", "REPOSITORY", "NAME", "STATE", "ACTIVE", "NEXT"}, rows)
	return nil
}

func runWorklaneList(cmd *cobra.Command, _ []string) error {
	client, _, err := repoCommandClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()
	lanes, err := fetchWorklanes(ctx, client)
	if err != nil {
		return err
	}
	repoInput, _ := cmd.Flags().GetString("repo")
	projectInput, _ := cmd.Flags().GetString("project")
	if repoInput != "" || projectInput != "" {
		repos, err := laneRepositories(ctx, client)
		if err != nil {
			return err
		}
		repoID, projectID := "", ""
		if repoInput != "" {
			repoID, err = laneRepositoryID(repos, repoInput)
			if err != nil {
				return err
			}
		}
		if projectInput != "" {
			ref, err := resolveProjectID(ctx, client, projectInput)
			if err != nil {
				return err
			}
			projectID = ref.ID
		}
		allowed := map[string]bool{}
		for _, repo := range repos {
			if (repoID == "" || repo.ID == repoID) && (projectID == "" || repo.ProjectID == projectID) {
				allowed[repo.ID] = true
			}
		}
		filtered := []cliWorklane{}
		for _, lane := range lanes {
			if allowed[lane.RepositoryID] {
				filtered = append(filtered, lane)
			}
		}
		lanes = filtered
	}
	return laneOutput(cmd, lanes, false)
}

func laneCommand(cmd *cobra.Command, input string) (*cli.APIClient, cliWorklane, error) {
	client, _, err := repoCommandClient(cmd)
	if err != nil {
		return nil, cliWorklane{}, err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()
	repo, _ := cmd.Flags().GetString("repo")
	lane, err := resolveCommandWorklane(ctx, client, input, repo)
	return client, lane, err
}

func runWorklaneGet(cmd *cobra.Command, args []string) error {
	client, lane, err := laneCommand(cmd, args[0])
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()
	if err := client.GetJSON(ctx, "/api/worklanes/"+url.PathEscape(lane.ID), &lane); err != nil {
		return err
	}
	return laneOutput(cmd, []cliWorklane{lane}, true)
}

func laneIssuesOutput(cmd *cobra.Command, issues []map[string]any) error {
	format, _ := cmd.Flags().GetString("output")
	if format == "json" {
		return cli.PrintJSON(os.Stdout, issues)
	}
	rows := make([][]string, 0, len(issues))
	for _, issue := range issues {
		rows = append(rows, []string{issueDisplayKey(issue), strVal(issue, "title"), strVal(issue, "status")})
	}
	cli.PrintTable(os.Stdout, []string{"ISSUE", "TITLE", "STATUS"}, rows)
	return nil
}

func runWorklaneIssues(cmd *cobra.Command, args []string) error {
	client, lane, err := laneCommand(cmd, args[0])
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()
	var result struct {
		Issues []map[string]any `json:"issues"`
	}
	if err := client.GetJSON(ctx, "/api/worklanes/"+url.PathEscape(lane.ID)+"/issues", &result); err != nil {
		return err
	}
	return laneIssuesOutput(cmd, result.Issues)
}

func runWorklaneRename(cmd *cobra.Command, args []string) error {
	client, lane, err := laneCommand(cmd, args[0])
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()
	if err := client.PatchJSON(ctx, "/api/worklanes/"+url.PathEscape(lane.ID), map[string]string{"name": args[1]}, &lane); err != nil {
		return err
	}
	return laneOutput(cmd, []cliWorklane{lane}, true)
}

func awaitLaneRequest(ctx context.Context, client *cli.APIClient, request map[string]any) (map[string]any, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for strVal(request, "status") == "running" {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
		if err := client.GetJSON(ctx, "/api/worklane-requests/"+url.PathEscape(strVal(request, "id")), &request); err != nil {
			return nil, err
		}
	}
	if strVal(request, "status") != "completed" {
		return nil, fmt.Errorf("worklane request %s %s: %v", strVal(request, "id"), strVal(request, "status"), request["reason"])
	}
	return request, nil
}

func laneRequestOutput(cmd *cobra.Command, request map[string]any) error {
	format, _ := cmd.Flags().GetString("output")
	if format == "json" {
		return cli.PrintJSON(os.Stdout, request)
	}
	cli.PrintTable(os.Stdout, []string{"REQUEST", "STATUS", "WORKLANE", "PATH"}, [][]string{{strVal(request, "id"), strVal(request, "status"), strVal(request, "worklane_id"), strVal(request, "path")}})
	return nil
}

func runWorklaneRemove(cmd *cobra.Command, args []string) error {
	client, lane, err := laneCommand(cmd, args[0])
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	var request map[string]any
	if err := client.DeleteJSONResponse(ctx, "/api/worklanes/"+url.PathEscape(lane.ID), &request); err != nil {
		return err
	}
	request, err = awaitLaneRequest(ctx, client, request)
	if err != nil {
		return err
	}
	return laneRequestOutput(cmd, request)
}

func runWorklaneCreate(cmd *cobra.Command, args []string) error {
	client, _, err := repoCommandClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	repos, err := laneRepositories(ctx, client)
	if err != nil {
		return err
	}
	repo, _ := cmd.Flags().GetString("repo")
	repoID, err := laneRepositoryID(repos, repo)
	if err != nil {
		return err
	}
	folder, _ := cmd.Flags().GetString("folder")
	daemon, _ := cmd.Flags().GetString("daemon-id")
	var request map[string]any
	if err := client.PostJSON(ctx, "/api/worklane-requests", map[string]string{"repository_id": repoID, "name": args[0], "folder": folder, "daemon_id": daemon}, &request); err != nil {
		return err
	}
	request, err = awaitLaneRequest(ctx, client, request)
	if err != nil {
		return err
	}
	return laneRequestOutput(cmd, request)
}

func runWorklaneMove(cmd *cobra.Command, args []string) error {
	client, _, err := repoCommandClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()
	issue, err := resolveIssueRef(ctx, client, args[0])
	if err != nil {
		return err
	}
	current, err := fetchIssue(ctx, client, issue.ID)
	if err != nil {
		return err
	}
	lane := strVal(current, "worklane_id")
	if _, err := uuid.Parse(lane); err != nil {
		return fmt.Errorf("issue %s is not in a worklane", issue.Display)
	}
	before, _ := cmd.Flags().GetString("before")
	after, _ := cmd.Flags().GetString("after")
	if (before == "") == (after == "") {
		return fmt.Errorf("provide exactly one of --before or --after")
	}
	field, targetInput := "before", before
	if after != "" {
		field, targetInput = "after", after
	}
	target, err := resolveIssueRef(ctx, client, targetInput)
	if err != nil {
		return err
	}
	var result struct {
		Issues []map[string]any `json:"issues"`
	}
	if err := client.PostJSON(ctx, "/api/worklanes/"+url.PathEscape(lane)+"/issues/"+url.PathEscape(issue.ID)+"/move", map[string]string{field: target.ID}, &result); err != nil {
		return err
	}
	return laneIssuesOutput(cmd, result.Issues)
}
