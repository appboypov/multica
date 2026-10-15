package execenv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPrepareClaudeSkillSettings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path, err := prepareClaudeSkillSettings(root, []RuntimeSkillRefForEnv{
		{Root: "provider", Key: "review-dir", Name: "review"},
		{Root: "plugin", Key: "paper:design-to-code", Plugin: "paper@market"},
	}, nil)
	if err != nil {
		t.Fatalf("prepareClaudeSkillSettings: %v", err)
	}
	if path == "" {
		t.Fatal("expected task-local settings path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var got struct {
		SkillOverrides map[string]string `json:"skillOverrides"`
		Permissions    struct {
			Deny []string `json:"deny"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if got.SkillOverrides["review"] != "off" {
		t.Fatalf("ordinary skill override = %q, want off", got.SkillOverrides["review"])
	}
	if _, exists := got.SkillOverrides["paper:design-to-code"]; exists {
		t.Fatal("plugin skills must not use Claude's unsupported skillOverrides path")
	}
	for _, want := range []string{
		"Skill(review)",
		"Skill(review *)",
		"Skill(paper:design-to-code)",
		"Skill(paper:design-to-code *)",
	} {
		found := false
		for _, rule := range got.Permissions.Deny {
			found = found || rule == want
		}
		if !found {
			t.Errorf("missing deny rule %q in %v", want, got.Permissions.Deny)
		}
	}

	cleared, err := prepareClaudeSkillSettings(root, nil, nil)
	if err != nil {
		t.Fatalf("clear settings: %v", err)
	}
	if cleared != "" {
		t.Fatalf("cleared settings path = %q, want empty", cleared)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("stale settings file still exists: %v", err)
	}
}

func TestEnsureCodexDisabledSkillsConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	configPath := filepath.Join(root, "config.toml")
	if err := os.WriteFile(configPath, []byte("model = \"gpt-5\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureCodexDisabledSkillsConfig(configPath, root, []RuntimeSkillRefForEnv{
		{Root: "provider", Key: "review"},
		{Root: "universal", Key: "shared/release"},
		{Root: "provider", Key: "../escape"},
	}, nil); err != nil {
		t.Fatalf("ensureCodexDisabledSkillsConfig: %v", err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Count(content, "[[skills.config]]") != 2 {
		t.Fatalf("disabled entry count mismatch:\n%s", content)
	}
	wantProvider := filepath.ToSlash(filepath.Join(root, "skills", "review", "SKILL.md"))
	if !strings.Contains(content, wantProvider) {
		t.Fatalf("missing provider skill path %q:\n%s", wantProvider, content)
	}
	if strings.Contains(content, "escape") {
		t.Fatalf("unsafe key leaked into config:\n%s", content)
	}
}

func TestRuntimeSkillPoliciesYieldToWorkspaceSkills(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workspaceSkills := []SkillContextForEnv{{Name: "Review"}}
	settingsPath, err := prepareClaudeSkillSettings(root, []RuntimeSkillRefForEnv{
		{Root: "provider", Key: "review-dir", Name: "review"},
	}, workspaceSkills)
	if err != nil {
		t.Fatalf("prepareClaudeSkillSettings: %v", err)
	}
	if settingsPath != "" {
		t.Fatalf("workspace-owned Claude skill was disabled via %q", settingsPath)
	}

	configPath := filepath.Join(root, "config.toml")
	if err := ensureCodexDisabledSkillsConfig(configPath, root, []RuntimeSkillRefForEnv{
		{Root: "provider", Key: "review"},
	}, workspaceSkills); err != nil {
		t.Fatalf("ensureCodexDisabledSkillsConfig: %v", err)
	}
	if data, err := os.ReadFile(configPath); err == nil && strings.Contains(string(data), "[[skills.config]]") {
		t.Fatalf("workspace-owned Codex skill was disabled:\n%s", data)
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestPrepareOmpSkillSettings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path, err := prepareOmpSkillSettings(root, []RuntimeSkillRefForEnv{
		{Root: "provider", Key: "review-dir", Name: "review"},
		{Root: "universal", Key: "duplicate", Name: "review"},
		{Root: "provider", Key: "nested/../nested/fallback"},
		{Root: "provider", Key: "literal", Name: `my\*?[]{}!skill`},
		{Root: "provider", Key: "../escape", Name: "unsafe"},
		{Root: "provider", Key: filepath.Join(root, "absolute"), Name: "absolute"},
		{Root: "provider", Key: "", Name: "empty"},
		{Root: "provider", Key: "assigned-dir", Name: "Code Review"},
		{Root: "universal", Key: "nested/assigned"},
	}, []SkillContextForEnv{{Name: "code-review"}, {Name: "Assigned"}})
	if err != nil {
		t.Fatalf("prepareOmpSkillSettings: %v", err)
	}
	if want := filepath.Join(root, "omp-runtime-skill-settings.yml"); path != want {
		t.Fatalf("settings path = %q, want %q", path, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Skills struct {
			IgnoredSkills []string `json:"ignoredSkills"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decode omp overlay: %v", err)
	}
	want := []string{"review", "fallback", `my\\\*\?\[\]\{\}\!skill`}
	if !slices.Equal(got.Skills.IgnoredSkills, want) {
		t.Fatalf("ignoredSkills = %q, want %q", got.Skills.IgnoredSkills, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("settings mode = %o, want 600", mode)
	}
}

func TestPrepareOmpSkillSettingsRemovesUnusedOverlay(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		disabled []RuntimeSkillRefForEnv
		assigned []SkillContextForEnv
	}{
		{name: "no disabled skills"},
		{name: "invalid keys", disabled: []RuntimeSkillRefForEnv{{Key: "../escape"}, {Key: "."}}},
		{name: "assigned wins", disabled: []RuntimeSkillRefForEnv{{Key: "review", Name: "Review"}}, assigned: []SkillContextForEnv{{Name: "review"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "omp-runtime-skill-settings.yml")
			if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := prepareOmpSkillSettings(root, tc.disabled, tc.assigned)
			if err != nil || got != "" {
				t.Fatalf("settings path = %q, error = %v; want empty, nil", got, err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("stale overlay not removed: %v", err)
			}
			if got, err := prepareOmpSkillSettings(root, tc.disabled, tc.assigned); err != nil || got != "" {
				t.Fatalf("already absent overlay = %q, %v; want empty, nil", got, err)
			}
		})
	}
}

func TestPrepareOmpSkillSettingsReportsFileErrors(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "omp-runtime-skill-settings.yml")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "block-removal"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, disabled := range [][]RuntimeSkillRefForEnv{nil, {{Key: "review"}}} {
		if got, err := prepareOmpSkillSettings(root, disabled, nil); err == nil || got != "" {
			t.Fatalf("blocked overlay = %q, %v; want empty path and error", got, err)
		}
	}
}

func TestEscapeOmpSkillGlob(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		want string
	}{
		{name: "review", want: "review"},
		{name: "", want: ""},
		{name: `\`, want: `\\`},
		{name: "*", want: `\*`},
		{name: "?", want: `\?`},
		{name: "[]", want: `\[\]`},
		{name: "{}", want: `\{\}`},
		{name: "!", want: `\!`},
		{name: `my\*?[]{}!skill`, want: `my\\\*\?\[\]\{\}\!skill`},
	} {
		if got := escapeOmpSkillGlob(tc.name); got != tc.want {
			t.Errorf("escapeOmpSkillGlob(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}
