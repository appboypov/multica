package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/spf13/cobra"
)

// folderLogins gives the test a home with a default config and the profiles
// tika and skuddy, each with its own login, and a folder client/app below a
// fresh temporary folder. It clears every MULTICA_* login and daemon variable
// and answers the two folders.
func folderLogins(t *testing.T) (client, app string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	for _, key := range []string{"MULTICA_SERVER_URL", "MULTICA_TOKEN", "MULTICA_WORKSPACE_ID", "MULTICA_AGENT_ID", "MULTICA_TASK_ID", "MULTICA_DAEMON_PORT", "MULTICA_HTTP_TIMEOUT"} {
		t.Setenv(key, "")
	}
	logins := map[string]cli.CLIConfig{
		"":       {ServerURL: "https://default.example", Token: "mul_default", WorkspaceID: "ws-default"},
		"tika":   {ServerURL: "http://127.0.0.1:8081", Token: "mul_tika", WorkspaceID: "ws-tika", HTTPTimeout: "75s"},
		"skuddy": {ServerURL: "http://127.0.0.1:8082", Token: "mul_skuddy", WorkspaceID: "ws-skuddy"},
	}
	for profile, cfg := range logins {
		if err := cli.SaveCLIConfigForProfile(cfg, profile); err != nil {
			t.Fatalf("seed profile %q: %v", profile, err)
		}
	}
	client = filepath.Join(t.TempDir(), "client")
	app = filepath.Join(client, "app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	return client, app
}

func writeProfileFile(t *testing.T, folder, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(folder, cli.ProfileFile), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// login is what the resolvers make of the command's login.
type login struct{ profile, serverURL, token, workspaceID string }

func resolvedLogin(t *testing.T, args ...string) login {
	t.Helper()
	cmd := testCmd()
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatal(err)
	}
	return login{resolveLoginProfile(cmd), tryResolveServerURL(cmd), resolveToken(cmd), resolveWorkspaceID(cmd)}
}

var tikaLogin = login{"tika", "http://127.0.0.1:8081", "mul_tika", "ws-tika"}

func TestLoginResolution(t *testing.T) {
	t.Run("given a .multica-profile in the working folder, its profile's login applies", func(t *testing.T) {
		_, app := folderLogins(t)
		writeProfileFile(t, app, " tika \n")
		t.Chdir(app)
		if got := resolvedLogin(t); got != tikaLogin {
			t.Fatalf("login = %+v, want %+v", got, tikaLogin)
		}
	})

	t.Run("given a .multica-profile only in a folder above, its profile's login applies", func(t *testing.T) {
		client, app := folderLogins(t)
		writeProfileFile(t, client, "tika")
		t.Chdir(app)
		if got := resolvedLogin(t); got != tikaLogin {
			t.Fatalf("login = %+v, want %+v", got, tikaLogin)
		}
	})

	t.Run("given a .multica-profile in the folder and above, the folder's own wins", func(t *testing.T) {
		client, app := folderLogins(t)
		writeProfileFile(t, client, "skuddy")
		writeProfileFile(t, app, "tika")
		t.Chdir(app)
		if got := resolvedLogin(t); got != tikaLogin {
			t.Fatalf("login = %+v, want %+v", got, tikaLogin)
		}
	})

	t.Run("given a blank .multica-profile, the one above applies", func(t *testing.T) {
		client, app := folderLogins(t)
		writeProfileFile(t, client, "tika")
		writeProfileFile(t, app, " \n")
		t.Chdir(app)
		if got := resolvedLogin(t); got != tikaLogin {
			t.Fatalf("login = %+v, want %+v", got, tikaLogin)
		}
	})

	t.Run("given no .multica-profile, the default profile's login applies", func(t *testing.T) {
		_, app := folderLogins(t)
		t.Chdir(app)
		want := login{"", "https://default.example", "mul_default", "ws-default"}
		if got := resolvedLogin(t); got != want {
			t.Fatalf("login = %+v, want %+v", got, want)
		}
	})

	t.Run("given MULTICA_* variables, they win over the folder's profile", func(t *testing.T) {
		_, app := folderLogins(t)
		writeProfileFile(t, app, "tika")
		t.Chdir(app)
		t.Setenv("MULTICA_SERVER_URL", "http://127.0.0.1:9000")
		t.Setenv("MULTICA_TOKEN", "mul_env")
		t.Setenv("MULTICA_WORKSPACE_ID", "ws-env")
		want := login{"tika", "http://127.0.0.1:9000", "mul_env", "ws-env"}
		if got := resolvedLogin(t); got != want {
			t.Fatalf("login = %+v, want %+v", got, want)
		}
	})

	t.Run("given --profile, it wins over the folder's profile", func(t *testing.T) {
		_, app := folderLogins(t)
		writeProfileFile(t, app, "tika")
		t.Chdir(app)
		want := login{"skuddy", "http://127.0.0.1:8082", "mul_skuddy", "ws-skuddy"}
		if got := resolvedLogin(t, "--profile", "skuddy"); got != want {
			t.Fatalf("login = %+v, want %+v", got, want)
		}
	})

	t.Run("given an empty --profile, the default profile wins over the folder's profile", func(t *testing.T) {
		_, app := folderLogins(t)
		writeProfileFile(t, app, "tika")
		t.Chdir(app)
		if got := resolvedLogin(t, "--profile", "").profile; got != "" {
			t.Fatalf("profile = %q, want the default profile", got)
		}
	})

	t.Run("given a daemon-managed run, the folder's profile is ignored", func(t *testing.T) {
		_, app := folderLogins(t)
		writeProfileFile(t, app, "tika")
		t.Chdir(app)
		t.Setenv("MULTICA_TASK_ID", "task-1")
		if got := resolveLoginProfile(testCmd()); got != "" {
			t.Fatalf("profile = %q, want the default profile in a run", got)
		}
	})

	t.Run("given a .multica-profile, commands that manage a profile keep the default profile", func(t *testing.T) {
		_, app := folderLogins(t)
		writeProfileFile(t, app, "tika")
		t.Chdir(app)
		cmd := testCmd()
		if got := resolveProfile(cmd); got != "" {
			t.Fatalf("managed profile = %q, want the default profile", got)
		}
		if got := tryResolveHumanServerURL(cmd); got != "https://default.example" {
			t.Fatalf("login's server URL = %q, want the default profile's", got)
		}
	})
}

func TestUseConfigHTTPTimeoutReadsTheFolderProfile(t *testing.T) {
	_, app := folderLogins(t)
	writeProfileFile(t, app, "tika")
	t.Chdir(app)
	t.Cleanup(func() { cli.UseConfigHTTPTimeout("") })

	useConfigHTTPTimeout(testCmd())
	if got, want := cli.NewAPIClient("http://127.0.0.1", "ws", "tok").HTTPClient.Timeout, 75*time.Second; got != want {
		t.Fatalf("client timeout = %v, want tika's http_timeout %v", got, want)
	}

	t.Setenv("MULTICA_HTTP_TIMEOUT", "90s")
	if got, want := cli.NewAPIClient("http://127.0.0.1", "ws", "tok").HTTPClient.Timeout, 90*time.Second; got != want {
		t.Fatalf("client timeout = %v, want MULTICA_HTTP_TIMEOUT %v", got, want)
	}
}

func TestApplyConfigSetHTTPTimeout(t *testing.T) {
	cfg := cli.CLIConfig{}
	if err := applyConfigSet(&cfg, "http_timeout", "90s"); err != nil || cfg.HTTPTimeout != "90s" {
		t.Fatalf("http_timeout 90s: err=%v, stored %q", err, cfg.HTTPTimeout)
	}
	for _, bad := range []string{"soon", "0s", "-1m"} {
		if err := applyConfigSet(&cfg, "http_timeout", bad); err == nil {
			t.Errorf("http_timeout %q: want an error", bad)
		}
	}
	if err := applyConfigSet(&cfg, "http_timeout", ""); err != nil || cfg.HTTPTimeout != "" {
		t.Fatalf("http_timeout \"\": err=%v, stored %q", err, cfg.HTTPTimeout)
	}
}

func TestExplicitProfileOutranksMulticaVariables(t *testing.T) {
	setEnvLogin := func(t *testing.T) {
		t.Setenv("MULTICA_SERVER_URL", "http://127.0.0.1:9000")
		t.Setenv("MULTICA_TOKEN", "mul_env")
		t.Setenv("MULTICA_WORKSPACE_ID", "ws-env")
	}

	t.Run("given --profile and all three MULTICA_* variables, the profile's login applies", func(t *testing.T) {
		_, app := folderLogins(t)
		t.Chdir(app)
		setEnvLogin(t)
		if got := resolvedLogin(t, "--profile", "tika"); got != tikaLogin {
			t.Fatalf("login = %+v, want %+v", got, tikaLogin)
		}
	})

	t.Run("given an empty --profile and all three MULTICA_* variables, the default profile's login applies", func(t *testing.T) {
		_, app := folderLogins(t)
		writeProfileFile(t, app, "tika")
		t.Chdir(app)
		setEnvLogin(t)
		want := login{"", "https://default.example", "mul_default", "ws-default"}
		if got := resolvedLogin(t, "--profile", ""); got != want {
			t.Fatalf("login = %+v, want %+v", got, want)
		}
	})

	t.Run("given --profile in a daemon-managed run, the injected MULTICA_* login stays", func(t *testing.T) {
		_, app := folderLogins(t)
		t.Chdir(app)
		setEnvLogin(t)
		t.Setenv("MULTICA_TASK_ID", "task-1")
		cmd := testCmd()
		if err := cmd.ParseFlags([]string{"--profile", "tika"}); err != nil {
			t.Fatal(err)
		}
		if got := resolveToken(cmd); got != "mul_env" {
			t.Fatalf("token = %q, want the injected one", got)
		}
		if got := resolveWorkspaceID(cmd); got != "ws-env" {
			t.Fatalf("workspace = %q, want the injected one", got)
		}
	})
}

func TestWorkspaceSwitchSavesTheProfileItLooksUpWith(t *testing.T) {
	_, app := folderLogins(t)
	writeProfileFile(t, app, "tika")
	t.Chdir(app)
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode([]map[string]string{{"id": "ws-new", "name": "New", "slug": "new"}})
	}))
	defer srv.Close()
	if err := cli.SaveCLIConfigForProfile(cli.CLIConfig{ServerURL: srv.URL, Token: "mul_tika", WorkspaceID: "ws-tika"}, "tika"); err != nil {
		t.Fatal(err)
	}

	if err := runWorkspaceSwitch(testCmd(), []string{"ws-new"}); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer mul_tika" {
		t.Fatalf("lookup authorized as %q, want tika's token", auth)
	}
	if cfg, _ := cli.LoadCLIConfigForProfile("tika"); cfg.WorkspaceID != "ws-new" {
		t.Fatalf("tika's workspace = %q, want ws-new", cfg.WorkspaceID)
	}
	if cfg, _ := cli.LoadCLIConfigForProfile(""); cfg.WorkspaceID != "ws-default" {
		t.Fatalf("default workspace = %q, want it unchanged", cfg.WorkspaceID)
	}
}

func TestUseDaemonProfilePinsTheDefaultDaemonOverTheFolder(t *testing.T) {
	_, app := folderLogins(t)
	writeProfileFile(t, app, "tika")
	t.Chdir(app)
	served := daemonServes
	t.Cleanup(func() { daemonServes = served })
	daemonServes = func(profile string) bool { return profile == "" }

	cmd := testCmd()
	if err := cmd.ParseFlags(nil); err != nil {
		t.Fatal(err)
	}
	if err := useDaemonProfile(cmd); err != nil {
		t.Fatal(err)
	}
	want := login{"", "https://default.example", "mul_default", "ws-default"}
	got := login{resolveLoginProfile(cmd), tryResolveServerURL(cmd), resolveToken(cmd), resolveWorkspaceID(cmd)}
	if got != want {
		t.Fatalf("API login = %+v, want the default daemon's %+v", got, want)
	}
	if port := healthPortForProfile(resolveProfile(cmd)); port != healthPortForProfile("") {
		t.Fatalf("daemon port = %d, want the default daemon's", port)
	}
}

func TestCommandHookTimeoutFollowsTheManagedProfile(t *testing.T) {
	_, app := folderLogins(t)
	writeProfileFile(t, app, "tika")
	t.Chdir(app)
	t.Cleanup(func() { cli.UseConfigHTTPTimeout("") })
	if err := cli.SaveCLIConfigForProfile(cli.CLIConfig{ServerURL: "https://default.example", Token: "mul_default", WorkspaceID: "ws-default", HTTPTimeout: "20s"}, ""); err != nil {
		t.Fatal(err)
	}
	clientTimeout := func() time.Duration { return cli.NewAPIClient("http://127.0.0.1", "ws", "tok").HTTPClient.Timeout }

	for _, cmd := range []*cobra.Command{loginCmd, setupCmd, configSetCmd, daemonCmd} {
		rootCmd.PersistentPreRun(cmd, nil)
		if got := clientTimeout(); got != 20*time.Second {
			t.Errorf("%s: client timeout = %v, want the default profile's 20s", cmd.CommandPath(), got)
		}
	}
	rootCmd.PersistentPreRun(issueCmd, nil)
	if got := clientTimeout(); got != 75*time.Second {
		t.Errorf("issue: client timeout = %v, want tika's 75s", got)
	}
}

// configProfileTestCmd is a config profile command, with --login when login.
func configProfileTestCmd(t *testing.T, login bool) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "profile"}
	cmd.Flags().Bool("login", false, "")
	if login {
		if err := cmd.Flags().Set("login", "true"); err != nil {
			t.Fatal(err)
		}
	}
	return cmd
}

func TestConfigProfileLoginRefusesInADaemonManagedRun(t *testing.T) {
	signals := map[string]func(t *testing.T, app string){
		"MULTICA_AGENT_ID":    func(t *testing.T, _ string) { t.Setenv("MULTICA_AGENT_ID", "agent-1") },
		"MULTICA_TASK_ID":     func(t *testing.T, _ string) { t.Setenv("MULTICA_TASK_ID", "task-1") },
		"MULTICA_DAEMON_PORT": func(t *testing.T, _ string) { t.Setenv("MULTICA_DAEMON_PORT", "19514") },
		cli.TaskConfigRootEnv: func(t *testing.T, _ string) { t.Setenv(cli.TaskConfigRootEnv, t.TempDir()) },
		"workdir marker only": func(t *testing.T, app string) {
			marker := filepath.Join(app, execenv.TaskContextMarkerRelPath)
			if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(map[string]string{"managed_by": execenv.TaskContextMarkerManagedBy})
			if err := os.WriteFile(marker, data, 0o644); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, signal := range signals {
		t.Run("given a run that lost MULTICA_TOKEN with "+name+", --login names no folder login", func(t *testing.T) {
			_, app := folderLogins(t)
			writeProfileFile(t, app, "tika")
			t.Chdir(app)
			signal(t, app)
			if out, err := captureStdout(t, func() error { return runConfigProfile(configProfileTestCmd(t, true), []string{app}) }); err == nil {
				t.Fatalf("config profile --login answered %q inside a daemon-managed run", out)
			}
			out, err := captureStdout(t, func() error { return runConfigProfile(configProfileTestCmd(t, false), []string{app}) })
			if err != nil || out != "tika\n" {
				t.Fatalf("config profile without --login = %q, %v; want the folder's profile", out, err)
			}
		})
	}

	t.Run("given no daemon signal, --login names the folder's profile", func(t *testing.T) {
		_, app := folderLogins(t)
		writeProfileFile(t, app, "tika")
		t.Chdir(app)
		out, err := captureStdout(t, func() error { return runConfigProfile(configProfileTestCmd(t, true), []string{app}) })
		if err != nil || out != "tika\n" {
			t.Fatalf("config profile --login = %q, %v; want tika", out, err)
		}
	})
}
