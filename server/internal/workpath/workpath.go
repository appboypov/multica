// Package workpath holds the folder rules of the work root (ADR-0037): every
// repository of a project lives under {root}/{organisation}/{project}, its
// main checkout in repos/{repo} and its worktrees in worktrees/{repo}/{folder}.
// The server and the daemon both build paths here, so the rule exists once.
//
// Paths keep the root's style: a root stored as "~/hq" gives "~/hq/..." paths,
// which the daemon expands to its own home.
package workpath

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
)

// DefaultRoot is the work root of a workspace whose settings name none.
const DefaultRoot = "~/hq"

// SettingsKey is the key of the work root in a workspace's settings JSON.
const SettingsKey = "work_root"

// PullRequestsFolder holds a repository's pull-request worktrees, one per
// branch, beside its lane and issue worktrees.
const PullRequestsFolder = ".pull-requests"

// segmentPattern is the rule of an organisation key and a project slug: they
// each name one folder.
var segmentPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

var (
	// ErrNoOrganisation means the project has no organisation, so its
	// repositories have no folder.
	ErrNoOrganisation = errors.New("the project has no organisation")
	// ErrNoProjectSlug means the project has no slug, so its repositories
	// have no folder.
	ErrNoProjectSlug = errors.New("the project has no slug")
)

// ValidSlug reports whether s can be a project slug: lowercase letters,
// digits and dashes, starting with a letter or digit.
func ValidSlug(s string) bool {
	return segmentPattern.MatchString(s)
}

// ValidRoot reports whether root can be a work root: an absolute path or a
// path under the home folder written with "~/".
func ValidRoot(root string) bool {
	if root == "~" {
		return true
	}
	if !strings.HasPrefix(root, "/") && !strings.HasPrefix(root, "~/") {
		return false
	}
	for _, part := range strings.Split(root, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}

// RootFromSettings reads the work root from a workspace's settings JSON, or
// DefaultRoot when the settings name none or a value that is no valid root.
func RootFromSettings(settings []byte) string {
	if len(settings) == 0 {
		return DefaultRoot
	}
	var values map[string]any
	if err := json.Unmarshal(settings, &values); err != nil {
		return DefaultRoot
	}
	root, ok := values[SettingsKey].(string)
	root = strings.TrimSpace(root)
	if !ok || !ValidRoot(root) {
		return DefaultRoot
	}
	return path.Clean(root)
}

// RepoName is the folder name of a repository: the part of its GitHub
// full_name after the slash.
func RepoName(fullName string) (string, error) {
	_, name, ok := strings.Cut(fullName, "/")
	if !ok || !validFolder(name) {
		return "", fmt.Errorf("invalid repository %q", fullName)
	}
	return name, nil
}

// Project is where a project's repositories live: the work root, the key of
// the project's organisation and the project's slug.
type Project struct {
	Root         string
	Organisation string
	Slug         string
}

// Dir is the project's folder, {root}/{organisation}/{slug}.
func (p Project) Dir() (string, error) {
	if p.Organisation == "" {
		return "", ErrNoOrganisation
	}
	if p.Slug == "" {
		return "", ErrNoProjectSlug
	}
	if !segmentPattern.MatchString(p.Organisation) {
		return "", fmt.Errorf("invalid organisation key %q", p.Organisation)
	}
	if !segmentPattern.MatchString(p.Slug) {
		return "", fmt.Errorf("invalid project slug %q", p.Slug)
	}
	root := p.Root
	if root == "" {
		root = DefaultRoot
	}
	if !ValidRoot(root) {
		return "", fmt.Errorf("invalid work root %q", root)
	}
	return path.Join(root, p.Organisation, p.Slug), nil
}

// Checkout is the default place of a repository's main checkout,
// {project}/repos/{repo}.
func (p Project) Checkout(fullName string) (string, error) {
	dir, err := p.Dir()
	if err != nil {
		return "", err
	}
	repo, err := RepoName(fullName)
	if err != nil {
		return "", err
	}
	return path.Join(dir, "repos", repo), nil
}

// Worktrees is the folder of a repository's worktrees,
// {project}/worktrees/{repo}.
func (p Project) Worktrees(fullName string) (string, error) {
	dir, err := p.Dir()
	if err != nil {
		return "", err
	}
	repo, err := RepoName(fullName)
	if err != nil {
		return "", err
	}
	return path.Join(dir, "worktrees", repo), nil
}

// Worktree is a lane's or an issue's worktree, {project}/worktrees/{repo}/
// {folder}; an issue's folder is its lowercase issue key.
func (p Project) Worktree(fullName, folder string) (string, error) {
	if !validFolder(folder) {
		return "", fmt.Errorf("invalid worktree folder %q", folder)
	}
	dir, err := p.Worktrees(fullName)
	if err != nil {
		return "", err
	}
	return path.Join(dir, folder), nil
}

// PullRequests is the folder of a repository's pull-request worktrees,
// {project}/worktrees/{repo}/.pull-requests. Each one sits at its branch
// name below it.
func (p Project) PullRequests(fullName string) (string, error) {
	dir, err := p.Worktrees(fullName)
	if err != nil {
		return "", err
	}
	return path.Join(dir, PullRequestsFolder), nil
}

// validFolder accepts one path segment that is not "." or "..".
func validFolder(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\`)
}
