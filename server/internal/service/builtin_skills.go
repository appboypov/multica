package service

import (
	"embed"
	"io/fs"
	"path"
	"strings"

	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

//go:embed builtin_skills
var builtinSkillsFS embed.FS

// Redirect stubs for built-ins this server no longer ships, keyed by the name
// an older daemon's runtime brief still points at. See legacyRedirectSkills.
//
//go:embed builtin_skills_legacy
var legacyBuiltinSkillsFS embed.FS

const (
	builtinSkillsRoot       = "builtin_skills"
	legacyBuiltinSkillsRoot = "builtin_skills_legacy"
)

// PlatformSkillName is the built-in skill carrying Multica's platform
// contracts — issues, mentions, agents, squads, autopilots, projects, runtimes
// and skill import — behind one routing SKILL.md. Every agent receives it.
//
// The daemon's runtime brief names the same skill from its own constant: the
// daemon runs on the user's machine and must not import this package, so the
// two are pinned by the brief's rendered-output tests instead of a shared
// symbol.
const PlatformSkillName = "multica-platform"

// builtinSkillSystemKey restricts a built-in skill to one product-defined
// agent, keyed by that agent's system key. A skill absent from this map is
// universal and every agent receives it.
//
// The scoping exists because a built-in skill costs every agent that receives
// it: its description sits in the always-loaded skill listing, and its files
// are written into every task's workdir. Mika's onboarding walkthrough is one
// agent's procedure, not a platform contract, so shipping it workspace-wide
// spent that budget on nine agents out of ten that can never use it.
var builtinSkillSystemKey = map[string]string{
	"multica-onboarding": MikaSystemKey,
}

// BuiltinSkills returns the platform's built-in skills for an agent with the
// given system key (empty for an ordinary workspace agent), embedded at compile
// time. Every agent receives the universal ones on top of its workspace-bound
// skills, so they teach platform-wide "how to" workflows that the runtime brief
// intentionally leaves to skills.
//
// Layout: builtin_skills/<name>/SKILL.md plus optional supporting files. The
// <name> directory carries a "multica-" prefix: that is the platform
// namespace, and the brief names built-ins by their bare name on the
// assumption that no workspace skill shares one. Nothing server-side reserves
// the prefix today, so a user could author a skill that sanitizes to the same
// slug and take the bare directory. That is accepted, not handled — when it
// becomes real, the fix is to reject the prefix at skill create/import rather
// than to make every pointer defensive.
// legacyRedirects asks for redirect stubs under the names this server has
// stopped shipping. The runtime brief is assembled by the daemon, not the
// server, so a backend deploy cannot rewrite an installed daemon's copy of it:
// a daemon older than the multica-platform merge still tells its agent to read
// `multica-working-on-issues`. Passing true for such a daemon keeps that
// pointer resolvable; the stub carries no contracts of its own, only the new
// location. Callers decide by capability, never by version string.
//
// questions is whether the run may ask questions (ADR-0005): with false, every
// skill that holds a questions block comes as its variant without it.
func (s *TaskService) BuiltinSkills(agentSystemKey string, legacyRedirects, questions bool) []AgentSkillData {
	skills := loadBuiltinSkills(agentSystemKey, questions)
	if legacyRedirects {
		skills = append(skills, legacyRedirectSkills()...)
	}
	return skills
}

// legacyRedirectSkills loads the redirect stubs. They live outside
// builtin_skills/ so that nothing ships them by default — a stub is only ever
// correct for a daemon whose brief still names the skill it replaces.
func legacyRedirectSkills() []AgentSkillData {
	return loadSkillDirs(legacyBuiltinSkillsFS, legacyBuiltinSkillsRoot, func(string) bool { return true }, true)
}

// AllBuiltinSkills returns every built-in skill regardless of agent scope. Only
// the bundle-resolve path uses it: the claim already decided which built-ins an
// agent was told about, and a daemon can only ask to resolve a ref it was
// handed, so re-deriving the scope there would cost an agent read to re-answer
// a question the claim answered. A skill with a questions block comes in both
// variants, each under its own id.
func (s *TaskService) AllBuiltinSkills() []AgentSkillData {
	all := func(string) bool { return true }
	skills := loadBuiltinSkillDirs(all, true)
	for _, skill := range loadBuiltinSkillDirs(all, false) {
		if skill.ID != "" {
			skills = append(skills, skill)
		}
	}
	return append(skills, legacyRedirectSkills()...)
}

func loadBuiltinSkills(agentSystemKey string, questions bool) []AgentSkillData {
	return loadBuiltinSkillDirs(func(name string) bool {
		want, scoped := builtinSkillSystemKey[name]
		return !scoped || want == agentSystemKey
	}, questions)
}

func loadBuiltinSkillDirs(include func(name string) bool, questions bool) []AgentSkillData {
	return loadSkillDirs(builtinSkillsFS, builtinSkillsRoot, include, questions)
}

func loadSkillDirs(fsys embed.FS, root string, include func(name string) bool, questions bool) []AgentSkillData {
	entries, err := fs.ReadDir(fsys, root)
	if err != nil {
		return nil
	}
	var skills []AgentSkillData
	for _, entry := range entries {
		if !entry.IsDir() || !include(entry.Name()) {
			continue
		}
		if skill, ok := loadBuiltinSkill(fsys, root, entry.Name(), questions); ok {
			skills = append(skills, skill)
		}
	}
	return skills
}

// builtinSkillOffVariant suffixes the id of a skill's variant without its
// questions block, so the two variants hash and resolve apart.
const builtinSkillOffVariant = "@core"

// loadBuiltinSkill reads one skill directory. Every file may hold a questions
// block between the lines questionsBlockStart and questionsBlockEnd: with
// questions on only the marker lines go, with questions off the whole block
// goes and the skill gets the id of its off variant.
func loadBuiltinSkill(fsys embed.FS, root, name string, questions bool) (AgentSkillData, bool) {
	dir := path.Join(root, name)
	content, err := fs.ReadFile(fsys, path.Join(dir, "SKILL.md"))
	if err != nil {
		// A skill directory without a SKILL.md is malformed — skip it rather
		// than ship an empty skill.
		return AgentSkillData{}, false
	}
	text, removed := questionsBlock(string(content), questions)
	skill := AgentSkillData{Name: name, Content: text}
	// Any other file in the directory becomes a supporting file, preserving
	// its relative path so subdirectories (e.g. references/issues.md) survive.
	_ = fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		rel := strings.TrimPrefix(p, dir+"/")
		if rel == "SKILL.md" {
			return nil
		}
		data, readErr := fs.ReadFile(fsys, p)
		if readErr != nil {
			return nil
		}
		fileText, fileRemoved := questionsBlock(string(data), questions)
		removed = removed || fileRemoved
		skill.Files = append(skill.Files, AgentSkillFileData{Path: rel, Content: fileText})
		return nil
	})
	if removed {
		skill.ID = BuiltinSkillID(name) + builtinSkillOffVariant
		skill.Source = skillbundle.SourceBuiltin
	}
	return skill, true
}

const (
	questionsBlockStart = "<!-- questions -->"
	questionsBlockEnd   = "<!-- /questions -->"
)

// questionsBlock returns text with its questions blocks kept (marker lines
// dropped) when on, or removed whole when off, and whether a block was
// removed.
func questionsBlock(text string, on bool) (string, bool) {
	if !strings.Contains(text, questionsBlockStart) {
		return text, false
	}
	lines := strings.SplitAfter(text, "\n")
	out := make([]string, 0, len(lines))
	inBlock, removed := false, false
	for _, line := range lines {
		switch strings.TrimSpace(line) {
		case questionsBlockStart:
			inBlock = true
			continue
		case questionsBlockEnd:
			inBlock = false
			continue
		}
		if inBlock && !on {
			removed = true
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, ""), removed
}
