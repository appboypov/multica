package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/spf13/cobra"
)

var repoRelocateCmd = &cobra.Command{
	Use:   "relocate <repo-id-or-full-name>",
	Short: "Point a repository at its moved checkout and worktrees",
	Long: "Records a repository's new checkout and moves the recorded paths of its worklanes and issue worktrees, " +
		"after the folders were moved on disk (ADR-0037). It runs the checkout check on the new checkout first: the " +
		"checkout must be on the default branch with a matching origin. Each --worktree-prefix old=new moves every " +
		"recorded worktree at or under old to the same place under new; the longest matching old wins, and a lane whose " +
		"path is the old checkout follows the new checkout. The command refuses while a task of the repository's project " +
		"is dispatched or running. It prints the updated rows and every new path the daemon cannot find; task history " +
		"keeps its old paths.",
	Args: exactArgs(1),
	RunE: runRepoRelocate,
}

func init() {
	repoRelocateCmd.Flags().String("checkout", "", "New path of the repository's main checkout (required)")
	repoRelocateCmd.Flags().StringArray("worktree-prefix", nil, "old=new: move every recorded worktree under old to the same place under new (may be repeated)")
	repoRelocateCmd.Flags().String("runtime-id", "", "Runtime that checks the new checkout (default: an online runtime of the repository's checkout daemon)")
	repoRelocateCmd.Flags().String("output", "json", "Output format: table or json")
	repoCmd.AddCommand(repoRelocateCmd)
}

// relocateRepository is the part of a repository the relocate reads.
type relocateRepository struct {
	ID               string `json:"id"`
	FullName         string `json:"full_name"`
	DefaultBranch    string `json:"default_branch"`
	CheckoutDaemonID string `json:"checkout_daemon_id"`
}

type relocateRuntime struct {
	ID       string  `json:"id"`
	DaemonID *string `json:"daemon_id"`
	Status   string  `json:"status"`
}

type relocatePrefix struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// parseWorktreePrefixes reads repeated old=new flags; each side is an
// absolute path or starts with "~/".
func parseWorktreePrefixes(values []string) ([]relocatePrefix, error) {
	prefixes := make([]relocatePrefix, 0, len(values))
	for _, value := range values {
		from, to, ok := strings.Cut(value, "=")
		from, to = strings.TrimSpace(from), strings.TrimSpace(to)
		if !ok || from == "" || to == "" {
			return nil, fmt.Errorf("--worktree-prefix %q: want old=new", value)
		}
		prefixes = append(prefixes, relocatePrefix{From: from, To: to})
	}
	return prefixes, nil
}

// relocateCheckoutPath makes a relative checkout path absolute; "~" paths are
// left for the daemon to expand.
func relocateCheckoutPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("--checkout is required")
	}
	if path == "~" || strings.HasPrefix(path, "~/") || filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Abs(path)
}

// findRelocateRepository finds the repository by id or full name.
func findRelocateRepository(repositories []relocateRepository, input string) (relocateRepository, error) {
	for _, repo := range repositories {
		if repo.ID == input || strings.EqualFold(repo.FullName, input) {
			return repo, nil
		}
	}
	return relocateRepository{}, fmt.Errorf("no repository %q in this workspace", input)
}

// pickRelocateRuntime is the explicit runtime, or else an online runtime of
// the daemon that holds the repository's checkout.
func pickRelocateRuntime(runtimes []relocateRuntime, daemonID, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	for _, rt := range runtimes {
		if rt.DaemonID != nil && *rt.DaemonID == daemonID && rt.Status == "online" {
			return rt.ID, nil
		}
	}
	return "", fmt.Errorf("no online runtime of daemon %q, which holds the checkout; start the daemon or pass --runtime-id", daemonID)
}

func runRepoRelocate(cmd *cobra.Command, args []string) error {
	rawCheckout, _ := cmd.Flags().GetString("checkout")
	checkout, err := relocateCheckoutPath(rawCheckout)
	if err != nil {
		return err
	}
	rawPrefixes, _ := cmd.Flags().GetStringArray("worktree-prefix")
	prefixes, err := parseWorktreePrefixes(rawPrefixes)
	if err != nil {
		return err
	}
	client, _, err := repoCommandClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var listed struct {
		Repositories []relocateRepository `json:"repositories"`
	}
	if err := client.GetJSON(ctx, "/api/repositories", &listed); err != nil {
		return fmt.Errorf("list repositories: %w", err)
	}
	repo, err := findRelocateRepository(listed.Repositories, args[0])
	if err != nil {
		return err
	}
	explicitRuntime, _ := cmd.Flags().GetString("runtime-id")
	var runtimes []relocateRuntime
	if explicitRuntime == "" {
		if err := client.GetJSON(ctx, "/api/runtimes", &runtimes); err != nil {
			return fmt.Errorf("list runtimes: %w", err)
		}
	}
	runtimeID, err := pickRelocateRuntime(runtimes, repo.CheckoutDaemonID, explicitRuntime)
	if err != nil {
		return err
	}

	checksPath := "/api/runtimes/" + url.PathEscape(runtimeID) + "/repository-checkouts"
	var check map[string]any
	if err := client.PostJSON(ctx, checksPath, map[string]any{
		"full_name":         repo.FullName,
		"default_branch":    repo.DefaultBranch,
		"path":              checkout,
		"repository_id":     repo.ID,
		"worktree_prefixes": prefixes,
	}, &check); err != nil {
		return fmt.Errorf("start checkout check: %w", err)
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for status := strVal(check, "status"); status == "pending" || status == "running"; status = strVal(check, "status") {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		if err := client.GetJSON(ctx, checksPath+"/"+url.PathEscape(strVal(check, "id")), &check); err != nil {
			return fmt.Errorf("poll checkout check: %w", err)
		}
	}
	if strVal(check, "status") != "completed" {
		return fmt.Errorf("checkout check %s: %s", strVal(check, "status"), strVal(check, "error"))
	}
	if passed, _ := check["passed"].(bool); !passed {
		return fmt.Errorf("the new checkout did not pass its check: %v", check["failures"])
	}

	var result map[string]any
	if err := client.PostJSON(ctx, "/api/repositories/"+url.PathEscape(repo.ID)+"/relocate", map[string]any{
		"checkout_check_id": strVal(check, "id"),
		"worktree_prefixes": prefixes,
	}, &result); err != nil {
		return fmt.Errorf("relocate %s: %w", repo.FullName, err)
	}
	if format, _ := cmd.Flags().GetString("output"); format == "json" {
		return cli.PrintJSON(os.Stdout, result)
	}
	rows := [][]string{}
	if repository, ok := result["repository"].(map[string]any); ok {
		rows = append(rows, []string{"checkout", strVal(repository, "full_name"), strVal(repository, "checkout_path")})
	}
	for _, kind := range []string{"worklanes", "issue_worktrees"} {
		items, _ := result[kind].([]any)
		for _, item := range items {
			row, _ := item.(map[string]any)
			rows = append(rows, []string{strings.TrimSuffix(kind, "s"), strVal(row, "id"), strVal(row, "path")})
		}
	}
	missing, _ := result["missing_paths"].([]any)
	for _, path := range missing {
		rows = append(rows, []string{"missing", "", fmt.Sprint(path)})
	}
	cli.PrintTable(os.Stdout, []string{"KIND", "ID", "PATH"}, rows)
	return nil
}
