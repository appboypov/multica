package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

var agentSessionsCmd = &cobra.Command{
	Use:   "sessions <agent>",
	Short: "List an agent's active sessions: Herdr panes, terminals and runs",
	Args:  exactArgs(1),
	RunE:  runAgentSessions,
}

var agentRegisterSessionCmd = &cobra.Command{
	Use:   "register-session <agent>",
	Short: "Register a session started by hand with the local daemon",
	Args:  exactArgs(1),
	RunE:  runAgentRegisterSession,
}

// errDaemonNotRunning is what the persona command and office extension show
// when no daemon listens on this profile's health port.
var errDaemonNotRunning = errors.New("the Multica daemon is not running; start it with `multica daemon start`")

func init() {
	agentSessionsCmd.Flags().String("output", "table", "Output format: table or json")

	agentRegisterSessionCmd.Flags().Int("pid", 0, "Process ID of the session (required)")
	agentRegisterSessionCmd.Flags().String("cwd", "", "Folder the session works in (required)")
	agentRegisterSessionCmd.Flags().String("pane", "", "Herdr pane ID of the session")
	agentRegisterSessionCmd.Flags().String("output", "table", "Output format: table or json")
}

// agentSessionListing is the part of GET /api/agents/{id}/sessions the table
// reads; --output json prints the response untouched.
type agentSessionListing struct {
	Sessions []struct {
		Kind        string  `json:"kind"`
		HerdrPaneID *string `json:"herdr_pane_id"`
		Runtime     *struct {
			Name string `json:"name"`
		} `json:"runtime"`
		Issue *struct {
			Identifier string `json:"identifier"`
		} `json:"issue"`
		WorkDir    string  `json:"work_dir"`
		Repository *string `json:"repository"`
		Branch     *string `json:"branch"`
		Status     string  `json:"status"`
		StartedAt  string  `json:"started_at"`
	} `json:"sessions"`
}

func runAgentSessions(cmd *cobra.Command, args []string) error {
	if err := useDaemonProfile(cmd); err != nil {
		return err
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	output, _ := cmd.Flags().GetString("output")

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	agentID, err := resolveAgent(ctx, client, args[0])
	if err != nil {
		return err
	}
	var raw json.RawMessage
	if err := client.GetJSON(ctx, "/api/agents/"+url.PathEscape(agentID)+"/sessions", &raw); err != nil {
		return fmt.Errorf("list agent sessions: %w", err)
	}
	if output == "json" {
		return cli.PrintJSON(os.Stdout, raw)
	}

	var listing agentSessionListing
	if err := json.Unmarshal(raw, &listing); err != nil {
		return fmt.Errorf("parse agent sessions: %w", err)
	}
	if len(listing.Sessions) == 0 {
		fmt.Fprintln(os.Stdout, "No active sessions.")
		return nil
	}
	headers := []string{"WHERE", "FOLDER", "REPOSITORY", "BRANCH", "ISSUE", "STATUS", "STARTED"}
	rows := make([][]string, 0, len(listing.Sessions))
	for _, s := range listing.Sessions {
		where := s.Kind
		switch {
		case s.HerdrPaneID != nil && *s.HerdrPaneID != "":
			where = *s.HerdrPaneID
		case s.Runtime != nil && s.Runtime.Name != "":
			where = "run on " + s.Runtime.Name
		}
		var issue *string
		if s.Issue != nil {
			issue = &s.Issue.Identifier
		}
		rows = append(rows, []string{
			where, s.WorkDir, sessionCell(s.Repository), sessionCell(s.Branch),
			sessionCell(issue), s.Status, s.StartedAt,
		})
	}
	cli.PrintTable(os.Stdout, headers, rows)
	return nil
}

func sessionCell(value *string) string {
	if value == nil || *value == "" {
		return "-"
	}
	return *value
}

func runAgentRegisterSession(cmd *cobra.Command, args []string) error {
	// A run's session is its task; only a session started by hand registers.
	if err := requireHumanLocalCommand("agent register-session"); err != nil {
		return err
	}
	pid, _ := cmd.Flags().GetInt("pid")
	if pid <= 0 {
		return fmt.Errorf("--pid is required and must be a positive process ID")
	}
	cwd, _ := cmd.Flags().GetString("cwd")
	if strings.TrimSpace(cwd) == "" {
		return fmt.Errorf("--cwd is required")
	}
	// The daemon reads the folder's repository from its own working
	// directory, so a relative folder must not reach it.
	cwd, err := filepath.Abs(cwd)
	if err != nil {
		return fmt.Errorf("resolve --cwd: %w", err)
	}
	pane, _ := cmd.Flags().GetString("pane")
	output, _ := cmd.Flags().GetString("output")

	if err := useDaemonProfile(cmd); err != nil {
		return err
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	workspaceID, err := requireWorkspaceID(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	agentID, err := resolveAgent(ctx, client, args[0])
	if err != nil {
		return err
	}
	id, err := postDaemonSession(ctx, healthPortForProfile(resolveProfile(cmd)), map[string]any{
		"workspace_id":  workspaceID,
		"agent_id":      agentID,
		"pid":           pid,
		"cwd":           cwd,
		"herdr_pane_id": pane,
	})
	if err != nil {
		return err
	}
	if output == "json" {
		return cli.PrintJSON(os.Stdout, map[string]string{"id": id})
	}
	fmt.Fprintln(os.Stdout, id)
	return nil
}

// useDaemonProfile points a command run by hand without --profile at the CLI
// profile whose daemon runs on this machine for that profile's server and
// workspace: the default profile's, else the one named profile's. It pins
// that profile as --profile, the default one too, so the API client,
// workspace and daemon port all follow it, whatever a folder's
// .multica-profile or the MULTICA_* variables name. A persona alias reaches
// its own daemon whatever the default profile is. An explicit --profile and a
// daemon task keep their own routing, and with no such daemon nothing is
// pinned.
func useDaemonProfile(cmd *cobra.Command) error {
	if resolveProfile(cmd) != "" || inDaemonManagedExecutionContext() {
		return nil
	}
	if daemonServes("") {
		return cmd.Flags().Set("profile", "")
	}
	profiles, err := knownProfiles()
	if err != nil {
		return fmt.Errorf("list CLI profiles: %w", err)
	}
	var serving []string
	for _, profile := range profiles {
		if daemonServes(profile) {
			serving = append(serving, profile)
		}
	}
	switch len(serving) {
	case 0:
		return nil
	case 1:
		return cmd.Flags().Set("profile", serving[0])
	default:
		return fmt.Errorf("daemons of several CLI profiles run on this machine (%s); choose one with --profile", strings.Join(serving, ", "))
	}
}

// daemonServes is profileDaemonServes; tests replace it to stand in for a
// running daemon.
var daemonServes = profileDaemonServes

// profileDaemonServes reports whether the daemon on a profile's health port is
// that profile's, is running, and serves the server and workspace of the
// profile's CLI config.
func profileDaemonServes(profile string) bool {
	cfg, err := cli.LoadCLIConfigForProfile(profile)
	if err != nil || cfg.ServerURL == "" || cfg.WorkspaceID == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	port := healthPortForProfile(profile)
	health := checkDaemonHealthOnPort(ctx, port)
	if health["status"] != "running" || daemonIdentityMismatch(health, profile, port) != nil {
		return false
	}
	serverURL, _ := health["server_url"].(string)
	if normalizeAPIBaseURL(serverURL) != normalizeAPIBaseURL(cfg.ServerURL) {
		return false
	}
	workspaces, _ := health["workspaces"].([]any)
	for _, workspace := range workspaces {
		if entry, ok := workspace.(map[string]any); ok && entry["id"] == cfg.WorkspaceID {
			return true
		}
	}
	return false
}

// postDaemonSession registers a session through the daemon's local listener
// and returns the session id it reports.
func postDaemonSession(ctx context.Context, port int, registration map[string]any) (string, error) {
	data, err := json.Marshal(registration)
	if err != nil {
		return "", fmt.Errorf("encode session: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/sessions", port), bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create daemon session request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) && opErr.Op == "dial" {
			return "", errDaemonNotRunning
		}
		return "", fmt.Errorf("connect to daemon: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", fmt.Errorf("read daemon response: %w", err)
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		if resp.StatusCode == http.StatusBadRequest {
			return "", errors.New(message)
		}
		return "", fmt.Errorf("register session: daemon returned %d: %s", resp.StatusCode, message)
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.ID == "" {
		return "", fmt.Errorf("parse daemon response: %q", strings.TrimSpace(string(body)))
	}
	return result.ID, nil
}
