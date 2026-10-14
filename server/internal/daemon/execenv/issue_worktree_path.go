package execenv

import (
	"fmt"
	"path/filepath"
	"strings"
)

// IssueWorktreePath keeps an issue's default folder out of recorded worklane
// paths (ADR-0009). Callers pass a recorded issue path unchanged for later
// tasks; only a new issue worktree uses this selection rule.
func IssueWorktreePath(defaultPath, issueKey string, worklanePaths []string) (string, error) {
	path, err := ExpandWorklanePath(defaultPath)
	if err != nil {
		return "", err
	}
	key := strings.ToLower(issueKey)
	if key == "" || key == "." || key == ".." || filepath.Base(key) != key {
		return "", fmt.Errorf("invalid issue key %q", issueKey)
	}
	for _, worklanePath := range worklanePaths {
		reserved, err := ExpandWorklanePath(worklanePath)
		if err != nil {
			return "", err
		}
		if sameWorklanePath(path, reserved) {
			path = filepath.Join(filepath.Dir(path), key+"-issue")
			break
		}
	}
	for _, worklanePath := range worklanePaths {
		reserved, err := ExpandWorklanePath(worklanePath)
		if err != nil {
			return "", err
		}
		if sameWorklanePath(path, reserved) {
			return "", fmt.Errorf("%s is a worklane's worktree", path)
		}
	}
	return path, nil
}
