"""team_state's repository part: the checkouts, OpenSpec changes and pull requests of each repository."""

from __future__ import annotations

import os
import re
from collections.abc import Iterator
from pathlib import Path

from team_state_io import StateError, graphql, run
from team_state_multica import GITHUB_REMOTE_RE


ISSUE_KEY_RE = re.compile(r"(?<![A-Za-z0-9])([A-Za-z][A-Za-z0-9]*)-(\d+)(?![A-Za-z0-9])")
ARCHIVED_RE = re.compile(r"^\d{4}-\d{2}-\d{2}-(.+)$")
CHANGE_PATH_RE = re.compile(r"(?:^|/)openspec/changes/(?:archive/\d{4}-\d{2}-\d{2}-)?([^/]+)/")
BUILD_KEY_RE = re.compile(r"^([\w-]+):(?:\s+(.*))?$")

GITHUB_QUERY = """
query($owner: String!, $name: String!, $after: String) {
  repository(owner: $owner, name: $name) {
    defaultBranchRef { name target { ... on Commit { oid statusCheckRollup { state } } } }
    pullRequests(states: OPEN, first: 25, after: $after) {
      pageInfo { hasNextPage endCursor }
      nodes {
        id number url title body baseRefName headRefName headRefOid
        assignees(first: 100) { nodes { login } }
        commits(last: 1) { nodes { commit { statusCheckRollup { state } } } }
        comments(first: 100) { pageInfo { hasNextPage endCursor } nodes { body url createdAt author { login } } }
        reviews(first: 100) { pageInfo { hasNextPage endCursor } nodes { state body url createdAt: submittedAt author { login } } }
        reviewThreads(first: 100) { pageInfo { hasNextPage endCursor }
          nodes { isResolved path comments(first: 100) { totalCount nodes { body url createdAt author { login } } } } }
        files(first: 100) { pageInfo { hasNextPage endCursor } nodes { path } }
      }
    }
  }
}
"""

MORE_QUERY = {
    "comments": "comments(first: 100, after: $after) { pageInfo { hasNextPage endCursor } nodes { body url createdAt author { login } } }",
    "reviews": "reviews(first: 100, after: $after) { pageInfo { hasNextPage endCursor } nodes { state body url createdAt: submittedAt author { login } } }",
    "reviewThreads": "reviewThreads(first: 100, after: $after) { pageInfo { hasNextPage endCursor } nodes { isResolved path comments(first: 100) { totalCount nodes { body url createdAt author { login } } } } }",
    "files": "files(first: 100, after: $after) { pageInfo { hasNextPage endCursor } nodes { path } }",
}


def origin_name(checkout: Path) -> str | None:
    """The lowercase GitHub name (`owner/name`) of the checkout's `origin` remote, read from its `.git/config`."""
    try:
        lines = (checkout / ".git" / "config").read_text(encoding="utf-8", errors="replace").splitlines()
    except OSError:
        return None
    section = None
    for line in lines:
        line = line.strip()
        if line.startswith("["):
            section = line
        elif section == '[remote "origin"]' and line.startswith("url") and "=" in line:
            m = GITHUB_REMOTE_RE.search(line.split("=", 1)[1].strip())
            return m.group(1).lower() if m else None
    return None


def local_checkouts(root: Path) -> dict[str, list[Path]]:
    """The main checkouts under `root`, at any depth, by the GitHub name of their `origin` remote. A checkout's own
    folders are not searched; a git folder without a GitHub `origin` is searched for the checkouts inside it."""
    found = {}
    for folder, subfolders, _ in os.walk(root):
        if ".git" in subfolders:
            subfolders.remove(".git")
            if name := origin_name(Path(folder)):
                subfolders.clear()
                found.setdefault(name, []).append(Path(folder))
        subfolders.sort()
    return found


def repo_home(path: Path) -> Path:
    """The repository's main checkout, so a repository and each of its worktrees count as one."""
    return Path(run(["git", "rev-parse", "--path-format=absolute", "--git-common-dir"], cwd=path).strip()).parent


def worktrees(checkout: Path) -> list[tuple[Path, str]]:
    """The checkout's worktrees with their branches."""
    found = []
    for block in run(["git", "worktree", "list", "--porcelain"], cwd=checkout).split("\n\n"):
        fields = dict(line.split(" ", 1) for line in block.splitlines() if " " in line)
        if "worktree" in fields and Path(fields["worktree"]).is_dir():
            found.append((Path(fields["worktree"]), fields.get("branch", "").removeprefix("refs/heads/")))
    return found


def changes_dir(checkout: Path) -> Path:
    """The OpenSpec changes folder: under `specs/` when the checkout holds `specs/openspec/`."""
    home = checkout / "specs" if (checkout / "specs" / "openspec").is_dir() else checkout
    return home / "openspec" / "changes"


def merged_archives(checkout: Path) -> set[str]:
    """The archive folder names on the default branch, the remote's first; none when git cannot tell."""
    folder = changes_dir(checkout) / "archive"
    for ref in ("refs/remotes/origin/HEAD", "main", "master"):
        try:
            run(["git", "rev-parse", "--verify", "--quiet", ref], cwd=checkout)
        except StateError:
            continue
        rel = folder.relative_to(checkout).as_posix()
        listing = run(["git", "ls-tree", "-d", "--name-only", ref, f"{rel}/"], cwd=checkout)
        return {Path(line).name for line in listing.splitlines()}
    return set()


def read_build(text: str) -> dict:
    """The scalar keys of the top-level `x-build:` map, as written."""
    out, inside, indent = {}, False, None
    for line in text.splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        if not line[0].isspace():
            if inside:
                break
            if line.startswith("x-build:"):
                rest = line[len("x-build:"):].split(" #")[0].strip()
                if rest not in ("", "{}"):
                    raise ValueError(f"x-build: is no map: {rest}")
                inside = True
            continue
        if inside:
            depth = len(line) - len(line.lstrip())
            indent = indent or depth
            if depth == indent and (m := BUILD_KEY_RE.match(line.strip())) and m.group(2):
                out[m.group(1)] = m.group(2).split(" #")[0].strip().strip("\"'")
    return out


def collect_changes(checkout: Path, default: str | None) -> list[dict]:
    """The OpenSpec changes of the checkout and its worktrees, each named once, a branch's before the default
    branch's: its open changes, and on another branch also each change it archived that the default branch has not."""
    places = sorted(worktrees(checkout), key=lambda w: w[1] == default)
    merged = {m.group(1) for n in merged_archives(checkout) if (m := ARCHIVED_RE.match(n))}
    seen, out = set(), []
    for path, branch in places:
        folder = changes_dir(path)
        if not folder.is_dir():
            continue
        found = [(p.name, p, False) for p in sorted(folder.iterdir()) if p.name != "archive"]
        if branch != default and (folder / "archive").is_dir():
            found += [
                (m.group(1), p, True)
                for p in sorted((folder / "archive").iterdir())
                if (m := ARCHIVED_RE.match(p.name)) and m.group(1) not in merged
            ]
        for name, p, is_archived in found:
            meta = p / ".openspec.yaml"
            if name in seen or not meta.is_file():
                continue
            seen.add(name)
            row = {"name": name, "checkout": str(path), "branch": branch, "archived": is_archived}
            try:
                out.append({**row, "build": read_build(meta.read_text(encoding="utf-8"))})
            except (ValueError, OSError, UnicodeError) as e:
                out.append({**row, "error": str(e)})
    return out


def all_nodes(pr: dict, field: str) -> Iterator[dict]:
    """Every node of the pull request's `field` connection, paging on."""
    page = pr[field]
    while True:
        yield from page["nodes"]
        if not page["pageInfo"]["hasNextPage"]:
            return
        query = f"query($id: ID!, $after: String) {{ node(id: $id) {{ ... on PullRequest {{ {MORE_QUERY[field]} }} }} }}"
        page = graphql(query, id=pr["id"], after=page["pageInfo"]["endCursor"])["node"][field]


def change_names(paths: list[str]) -> list[str]:
    """The OpenSpec changes the paths touch, open or archived, in any OpenSpec home."""
    return sorted({m.group(1) for p in paths if (m := CHANGE_PATH_RE.search(p)) and m.group(1) != "archive"})


def issue_keys(*texts: str) -> list[str]:
    """The issue keys a pull request's title and branch name, as Multica links them."""
    return list(dict.fromkeys(f"{m.group(1).upper()}-{m.group(2)}" for t in texts for m in ISSUE_KEY_RE.finditer(t or "")))


def message(m: dict) -> dict:
    return {"author": (m.get("author") or {}).get("login"), "at": m.get("createdAt"), "url": m.get("url"), "body": m.get("body")}


def collect_github(name: str, intake: bool = False) -> dict:
    """The repository's default branch and every open pull request, paged; for intake also each pull request's number,
    base and body, the default branch's head commit, and `unknown` for a check rollup GitHub does not have."""
    owner, repo = name.split("/", 1)
    nodes, after, data = [], None, None
    while True:
        data = graphql(GITHUB_QUERY, owner=owner, name=repo, after=after)
        page = data["repository"]["pullRequests"]
        nodes += page["nodes"]
        if not page["pageInfo"]["hasNextPage"]:
            break
        after = page["pageInfo"]["endCursor"]
    branch = data["repository"]["defaultBranchRef"] or {}
    target = branch.get("target") or {}
    prs = []
    for n in nodes:
        commit = n["commits"]["nodes"][0]["commit"] if n["commits"]["nodes"] else {}
        reviews = [{**message(r), "state": r["state"]} for r in all_nodes(n, "reviews")]
        checks = (commit.get("statusCheckRollup") or {}).get("state")
        extra = {"number": n["number"], "base": n["baseRefName"], "body": n["body"]} if intake else {}
        prs.append(
            {
                **extra,
                "url": n["url"],
                "title": n["title"],
                "repository": name,
                "branch": n["headRefName"],
                "head": n["headRefOid"],
                "issues": issue_keys(n["title"], n["headRefName"]),
                "assignees": sorted(a["login"] for a in n["assignees"]["nodes"]),
                "checks": (checks or "unknown") if intake else checks,
                "comments": sorted([message(c) for c in all_nodes(n, "comments")] + reviews, key=lambda c: c["at"] or ""),
                "threads": [
                    {
                        "resolved": t["isResolved"],
                        "path": t["path"],
                        "comment_count": t["comments"]["totalCount"],
                        "comments": [message(c) for c in t["comments"]["nodes"]],
                    }
                    for t in all_nodes(n, "reviewThreads")
                ],
                "changes": change_names([f["path"] for f in all_nodes(n, "files")]),
            }
        )
    checks = (target.get("statusCheckRollup") or {}).get("state")
    if intake:
        return {"default_branch": branch.get("name"), "default_head": target.get("oid"), "checks": checks or "unknown",
                "prs": prs}
    return {"default_branch": branch.get("name"), "checks": checks, "prs": prs}


def collect_repo(name: str, checkouts: list[Path]) -> tuple[dict, list[dict], list[dict]]:
    """One repository's summary, changes and pull requests; a part that cannot be read leaves its error."""
    summary = {"name": name, "checkouts": [str(c) for c in checkouts], "default_branch": None, "checks": None}
    errors, prs, changes = [], [], []
    try:
        github = collect_github(name)
        prs = github.pop("prs")
        summary.update(github)
    except (StateError, KeyError, TypeError) as e:
        errors.append(f"GitHub: {e}")
    for checkout in checkouts:
        try:
            changes += [{**c, "repository": name} for c in collect_changes(checkout, summary["default_branch"])]
        except (StateError, OSError, UnicodeError) as e:
            errors.append(f"{checkout}: {e}")
    if errors:
        summary["error"] = "; ".join(errors)
    return summary, changes, prs
