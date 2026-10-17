package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func checkoutGit(ctx context.Context, path string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", path}, args...)...)
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}

var repositoryVersionTag = regexp.MustCompile(`^v([0-9]+)\.([0-9]+)\.([0-9]+)$`)

// checkRepositoryCheckout reads only local refs; it never fetches during a
// check. Version-file ordering is the repository policy ordering.
func checkRepositoryCheckout(ctx context.Context, request protocol.DaemonHeartbeatPendingRepositoryCheckout) protocol.RepositoryCheckoutResult {
	result := protocol.RepositoryCheckoutResult{Status: "completed", Failures: []protocol.RepositoryCheckoutFailure{}, VersionFiles: []string{}, InstallCommands: []protocol.RepositoryInstallCommand{}}
	fail := func(code, detail string) protocol.RepositoryCheckoutResult {
		result.Failures = append(result.Failures, protocol.RepositoryCheckoutFailure{Code: code, Detail: detail})
		return result
	}
	path, err := execenv.ExpandWorklanePath(request.Path)
	if err != nil {
		return fail("not_found", err.Error())
	}
	result.ResolvedPath = path
	_, err = os.Stat(path)
	if os.IsNotExist(err) && request.Clone {
		cmd := exec.CommandContext(ctx, "git", "clone", "git@github.com:"+request.FullName+".git", path)
		cmd.WaitDelay = 5 * time.Second
		out, cloneErr := cmd.CombinedOutput()
		if cloneErr != nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			return fail("clone_failed", lines[len(lines)-1])
		}
		err = nil
	}
	if err != nil {
		return fail("not_found", err.Error())
	}
	inside, err := checkoutGit(ctx, path, "rev-parse", "--is-inside-work-tree")
	if err != nil || inside != "true" {
		return fail("not_git", path)
	}
	origin, err := checkoutGit(ctx, path, "remote", "get-url", "origin")
	if err != nil || !strings.EqualFold(execenv.GitHubFullNameFromURL(origin), request.FullName) {
		result.Failures = append(result.Failures, protocol.RepositoryCheckoutFailure{Code: "origin_mismatch", Detail: origin})
	}
	branch, err := checkoutGit(ctx, path, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || branch != request.DefaultBranch {
		result.Failures = append(result.Failures, protocol.RepositoryCheckoutFailure{Code: "wrong_branch", Detail: branch})
	}
	if len(result.Failures) != 0 {
		return result
	}
	result.Passed = true
	for _, file := range []string{"pubspec.yaml", "Cargo.toml", "pyproject.toml", "package.json"} {
		if _, err := checkoutGit(ctx, path, "cat-file", "-e", "refs/heads/"+request.DefaultBranch+":"+file); err == nil {
			result.VersionFiles = append(result.VersionFiles, file)
		}
	}
	for _, rule := range []protocol.RepositoryInstallCommand{
		{File: "pnpm-lock.yaml", Command: "pnpm install"},
		{File: "package-lock.json", Command: "npm ci"},
		{File: "yarn.lock", Command: "yarn install"},
		{File: "pubspec.yaml", Command: "flutter pub get"},
		{File: "uv.lock", Command: "uv sync"},
	} {
		if _, err := checkoutGit(ctx, path, "cat-file", "-e", "refs/heads/"+request.DefaultBranch+":"+rule.File); err == nil {
			result.InstallCommands = append(result.InstallCommands, rule)
		}
	}
	tags, err := checkoutGit(ctx, path, "tag", "--list", "v*")
	if err == nil {
		var highest [3]uint64
		for _, tag := range strings.Split(tags, "\n") {
			parts := repositoryVersionTag.FindStringSubmatch(tag)
			if parts == nil {
				continue
			}
			var version [3]uint64
			valid := true
			for i := range version {
				version[i], err = strconv.ParseUint(parts[i+1], 10, 64)
				if err != nil {
					valid = false
				}
			}
			if !valid {
				continue
			}
			if result.LastTag == "" || version[0] > highest[0] || (version[0] == highest[0] && version[1] > highest[1]) || (version[0] == highest[0] && version[1] == highest[1] && version[2] > highest[2]) {
				highest, result.LastTag = version, tag
			}
		}
	}
	return result
}

func (d *Daemon) handleRepositoryCheckout(ctx context.Context, runtimeID string, request protocol.DaemonHeartbeatPendingRepositoryCheckout) {
	limit := 30 * time.Second
	if request.Clone {
		limit = 10 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	result := checkRepositoryCheckout(runCtx, request)
	if err := d.client.ReportRepositoryCheckoutResult(ctx, runtimeID, request.ID, result); err != nil {
		d.logger.Error("report repository checkout failed", "request_id", request.ID, "error", err)
	}
}
