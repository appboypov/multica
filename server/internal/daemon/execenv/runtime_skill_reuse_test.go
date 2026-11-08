package execenv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestReuseRuntimeSkillPolicyFailure(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		t.Run(provider, func(t *testing.T) {
			if provider == "codex" && os.Geteuid() == 0 {
				t.Skip("root bypasses the directory permissions needed to prevent skill refresh")
			}
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CODEX_HOME", t.TempDir())
			params := PrepareParams{
				WorkspacesRoot: t.TempDir(),
				WorkspaceID:    "ws-policy-failure",
				TaskID:         "a1234567-1234-5678-9012-123456789012",
				Provider:       provider,
				Task: TaskContextForEnv{
					DisabledRuntimeSkills: []RuntimeSkillRefForEnv{{Root: "provider", Key: "review"}},
				},
			}
			env, err := Prepare(params, testLogger())
			if err != nil {
				t.Fatal(err)
			}
			defer env.Cleanup(true)

			if provider == "claude" {
				if err := os.Remove(env.ClaudeSettingsPath); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(env.ClaudeSettingsPath, 0o700); err != nil {
					t.Fatal(err)
				}
			} else {
				skillsDir := filepath.Join(env.CodexHome, "skills")
				if err := os.MkdirAll(skillsDir, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(skillsDir, "blocked"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(skillsDir, 0o500); err != nil {
					t.Fatal(err)
				}
				defer os.Chmod(skillsDir, 0o700)
				// Prove the failure is skill hydration rather than home setup.
				if err := prepareCodexHomeWithOpts(env.CodexHome, CodexHomeOptions{}, testLogger()); err != nil {
					t.Fatalf("Codex home setup must still succeed: %v", err)
				}
			}

			if reused := Reuse(ReuseParams{WorkDir: env.WorkDir, Provider: provider, Task: params.Task}, testLogger()); reused != nil {
				t.Error("Reuse accepted an environment whose runtime-skill policy could not be refreshed")
			}

			// The daemon owns and resets a fresh root before calling Prepare.
			// Preserve the blocked policy target to exercise a persistent failure.
			params.EnvRootPreclaimed = true
			if fresh, err := Prepare(params, testLogger()); err == nil || fresh != nil {
				t.Fatalf("Prepare returned environment %v and error %v; want no environment and a policy error", fresh, err)
			}
		})
	}
}

func TestReuseRuntimeSkillPolicyRetainsExclusions(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		t.Run(provider, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CODEX_HOME", t.TempDir())
			params := PrepareParams{
				WorkspacesRoot: t.TempDir(),
				WorkspaceID:    "ws-policy-reuse",
				TaskID:         "b1234567-1234-5678-9012-123456789012",
				Provider:       provider,
				Task: TaskContextForEnv{
					DisabledRuntimeSkills: []RuntimeSkillRefForEnv{{Root: "provider", Key: "review"}},
				},
			}
			env, err := Prepare(params, testLogger())
			if err != nil {
				t.Fatal(err)
			}
			defer env.Cleanup(true)
			reused := Reuse(ReuseParams{WorkDir: env.WorkDir, Provider: provider, Task: params.Task}, testLogger())
			if reused == nil {
				t.Fatal("Reuse declined a valid environment")
			}
			if provider == "claude" {
				data, err := os.ReadFile(reused.ClaudeSettingsPath)
				if err != nil {
					t.Fatal(err)
				}
				var policy struct {
					SkillOverrides map[string]string `json:"skillOverrides"`
					Permissions    struct {
						Deny []string `json:"deny"`
					} `json:"permissions"`
				}
				if err := json.Unmarshal(data, &policy); err != nil {
					t.Fatal(err)
				}
				if policy.SkillOverrides["review"] != "off" || !slices.Contains(policy.Permissions.Deny, "Skill(review)") || !slices.Contains(policy.Permissions.Deny, "Skill(review *)") {
					t.Fatalf("Claude review exclusion missing: %+v", policy)
				}
			} else {
				data, err := os.ReadFile(filepath.Join(reused.CodexHome, "config.toml"))
				if err != nil {
					t.Fatal(err)
				}
				var policy struct {
					Skills struct {
						Config []struct {
							Path    string `toml:"path"`
							Enabled bool   `toml:"enabled"`
						} `toml:"config"`
					} `toml:"skills"`
				}
				if err := toml.Unmarshal(data, &policy); err != nil {
					t.Fatal(err)
				}
				want := filepath.ToSlash(filepath.Join(reused.CodexHome, "skills", "review", "SKILL.md"))
				if len(policy.Skills.Config) != 1 || policy.Skills.Config[0].Path != want || policy.Skills.Config[0].Enabled {
					t.Fatalf("Codex review exclusion missing: %+v", policy)
				}
			}
		})
	}
}
