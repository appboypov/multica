#!/usr/bin/env python3
# /// script
# requires-python = ">=3.11"
# ///
"""team_state - collect the open work of one endeavour from every source; it checks nothing and decides nothing.

  team_state.py <endeavour folder> [--repo <owner/name or path> ...] [--lane <lane name or id>] [--client <name> ...]

- Repositories: the GitHub repositories of the checkouts in the endeavour's folder, at any depth (the folder itself
  when it is one), plus each --repo, that Multica registers (`multica repo list`); one Multica does not register is
  named under `unregistered`.
- Multica (`multica --profile skuddy`), per repository: its worklanes in their order (`worklane list`), each with its
  state, branch, worktree path and Multica's own active and next issue, its issues in lane order (`worklane issues`)
  with their labels, Points and assignee, and from these `active`, the first issue in lane order that has started and
  is not finished (`in_progress`, `blocked` or `in_review`), and `next`, the first that has not started, each with
  `waits_on`; `verifier`, its open issues assigned to the verifier (the Multica agent `--verifier` names), whose pull request is
  open and which are finished in the lane; and its release line (`release list`) with milestones and each release's
  open and done issues. Beside them the projects those issues are in, each with its organisation, and each project's
  open issues in no lane.
- `waits_on` names what holds an issue, read from Multica as written: status `blocked`; each issue its metadata
  `blocked_by` names that is neither closed nor assigned to the verifier; for a journey step (a sub-issue with a
  stage), each earlier step of its journey that is neither closed nor assigned to the verifier; and each open
  client wait of its release that comes before it in its lane: an issue assigned to a member with role `client`
  (`workspace member list`).
- Repositories, from git and GitHub: per repository its checkouts, the default branch and that branch's `checks`;
  `changes`, the OpenSpec changes of every checkout and worktree with their `x-build:` map; `prs`, the open pull
  requests with their checks, comments, reviews, review threads, the changes their files touch and the Multica
  issues their title or branch names.
- Client messages: the latest WhatsApp messages, through Beeper Desktop, of every chat in the agent file of each
  client the endeavour's `AGENTS.md` names (`das-client-agents/clients/<client>/AGENTS.md`), plus each --client.

A source that cannot be read carries its `error` in its place, and everything else is still collected. Output is
one JSON object on stdout. Exit 1 only when Multica cannot be read.

  team_state.py --intake <owner/name or path> --verifier <agent name>

Repository intake: the verifier's view of one repository, whether Multica registers it or not, without lanes or
client messages. It holds:
- `repository`, `default_branch`, its `default_head` commit and that commit's `checks`.
- `prs`: every open pull request, paged, with its `number`, `base`, `branch`, `head`, `body`, `checks` (`unknown`
  without a rollup), comments, reviews, threads and changes; its `links`, each Multica issue that links it, read
  with `issue pull-requests` from every workspace issue including closed ones. Named claims from the title, branch
  or closing keywords (`Closes WORK-1`) remain linked even when their issue is missing, with an evidence blocker.
  Each link has Multica's `source` or `claimed`; `issues` holds those keys. `stack_note` is the first `Stacked on ...`
  clause naming a concrete target, up to the end of its sentence or a `which`, with the pull requests (`#N` or this repository's
  pull request link), the branches (in backticks, not a commit) and the other repositories' pull requests it names.
- `issues`: each linked issue by key, from any project, lane or assignee.
- `edges`: each pull request's edge to the pull request under it, named by its note or by a base that is another
  open pull request's branch, with its `status`: `verified` once GitHub's comparison shows the upper holds the lower's
  head (`ancestry` `contains`) or a commit of it that is not on the default branch (`forked`: the lower has commits
  since); `lower_merged`; `other_repository`; and, needing reconciliation, `ancestry_unproven`, `ambiguous` (more
  open pull requests named), `cycle`, `lower_closed` (closed unmerged) and `unknown_target`. `unresolved` repeats
  those last five. Every named target is reconciled; `candidates` retains multiple known targets and
  `unresolved_refs` retains references that could not be resolved.
- `issueless`: the numbers of the pull requests without a linked issue.
- `prs`, `ready` and `blocked` are ranked individual pull request records. Each includes its `position` (the first
  linked issue's board position), `downstream` (distinct other open issues waiting on it transitively),
  `downstream_count` and `blocked_by`. Ranking uses downstream count descending, board position, then PR number.
  An issue waits on its `blocked_by`, earlier journey steps, enabled issue-condition wakeups or open children
  selected by a `children_done` wakeup. A pull request waits on its verified open lower pull request (`stack`).
  Downstream traversal follows linked issues back into their carrying PRs before continuing into upper stacks.
  Links are read for every workspace issue; wakeups are read for open issues. Unlike `waits_on`,
  an open pull request releases nothing here. Other blockers are unresolved stack edges (`stack_edge`), linked
  open issues held by someone other than the verifier (`assignee`), open issue dependencies (`blocked_by`, `step`,
  `wakeup`) and open sub-issues such as Fixes (`child`). Unread issues, agents, links or wakeups (`evidence`) hold
  every PR. An unknown verifier also holds every PR. A missing claimed issue holds its PR and reports an error.
  Check rollups are informational.
Exit 1 only when GitHub cannot be read; a Multica part that cannot be read carries its error in `errors`.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request
from collections.abc import Iterator, Sequence
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

MULTICA = ["multica", "--profile", "skuddy"]
MULTICA_CONFIG = Path.home() / ".multica" / "profiles" / "skuddy" / "config.json"
CLOSED = {"done", "cancelled"}
STARTED = {"in_progress", "blocked", "in_review"}
POINTS = "Points"
PAGE = 100
ISSUE_READERS = 8  # per-issue `multica` reads at once
UNRESOLVED_EDGES = {"ancestry_unproven", "ambiguous", "cycle", "lower_closed", "unknown_target"}
ISSUE_FIELDS = "id,identifier,title,type,status,assignee_type,assignee_id,parent_issue_id,stage,metadata,position,project_id,release_id,due_date"
CLIENTS_ROOT = Path.home() / ".omp" / "agent" / "skills" / "das-client-agents" / "clients"
BEEPER = "http://localhost:23373"
MESSAGES_PER_CHAT = 20
CLIENT_FILE_RE = re.compile(r"das-client-agents/clients/([\w.-]+)/AGENTS\.md")
CHAT_ID_RE = re.compile(r"`(![^`\s]+:[^`\s]+)`")
GITHUB_REMOTE_RE = re.compile(
    r"^(?:git@github\.com:|ssh://git@github\.com/|https?://(?:[^@/]+@)?github\.com/)([\w.-]+/[\w.-]+?)(?:\.git)?/?$",
    re.IGNORECASE,
)
ISSUE_KEY_RE = re.compile(r"(?<![A-Za-z0-9])([A-Za-z][A-Za-z0-9]*)-(\d+)(?![A-Za-z0-9])")
CLOSING_KEY_RE = re.compile(r"(?i)\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)[:\s]+([a-z][a-z0-9]{0,9})-(\d+)\b")
STACK_NOTE_RE = re.compile(r"(?i)\bstacked\s+on\b([^\n]*)")
NOTE_END_RE = re.compile(r"(?i)\bwhich\b|;|\.(?:\s|$)")
PR_REF_RE = re.compile(r"(?<![\w/.#-])#(\d+)\b")
OTHER_PR_REF_RE = re.compile(r"\b([\w.-]+/[\w.-]+)#(\d+)\b")
PR_URL_RE = re.compile(r"github\.com/([\w.-]+/[\w.-]+)/pull/(\d+)", re.IGNORECASE)
CODE_RE = re.compile(r"`([^`\s]+)`")
SHA_RE = re.compile(r"[0-9a-f]{7,40}")
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


class StateError(Exception):
    pass


def run(cmd: list[str], cwd: Path | None = None) -> str:
    try:
        done = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True, check=False)
    except OSError as e:
        raise StateError(f"{' '.join(cmd)}: {e}") from e
    if done.returncode != 0:
        where = f" (in {cwd})" if cwd else ""
        raise StateError(f"{' '.join(cmd[:5])}{where} failed: {done.stderr.strip() or done.stdout.strip()}")
    return done.stdout


def run_json(cmd: list[str], cwd: Path | None = None) -> dict:
    data = json.loads(run(cmd, cwd))
    if data.get("errors"):
        raise StateError(f"{' '.join(cmd[:3])}: {data['errors']}")
    return data


# ---------------------------------------------------------------- Multica


def multica(*args: str):
    """One `multica` command's JSON output."""
    out = run([*MULTICA, *args, "--output", "json"])
    try:
        return json.loads(out)
    except ValueError as e:
        raise StateError(f"multica {' '.join(args[:2])}: no JSON: {out[:200]}") from e


def organisations() -> dict:
    """The workspace's organisations by id; the CLI has none, so `GET /api/organisations` with the profile's token."""
    config = json.loads(MULTICA_CONFIG.read_text(encoding="utf-8"))
    request = urllib.request.Request(
        config["server_url"].rstrip("/") + "/api/organisations",
        headers={"Authorization": f"Bearer {config['token']}", "X-Workspace-ID": config["workspace_id"]},
    )
    with urllib.request.urlopen(request, timeout=20) as response:
        return {o["id"]: {"name": o["name"], "key": o["key"]} for o in json.load(response)["organisations"]}


def registered_repositories() -> set[str]:
    """The lowercase GitHub names of the repositories Multica registers."""
    return {m.group(1).lower() for r in multica("repo", "list") if (m := GITHUB_REMOTE_RE.search(r.get("url") or ""))}


def points_property() -> str | None:
    return next((p["id"] for p in multica("property", "list") if p.get("name") == POINTS), None)


def issue_pages(*filters: str) -> list[dict]:
    """Every issue `issue list` gives with the filters, paged."""
    issues, offset = [], 0
    while True:
        page = multica("issue", "list", *filters, "--limit", str(PAGE), "--offset", str(offset))
        issues += page["issues"]
        if not page.get("has_more") or not page["issues"]:
            return issues
        offset += len(page["issues"])


def project_issues(project: str) -> list[dict]:
    """Every issue of the project, with its labels, paged."""
    return issue_pages("--project", project)


def issue_row(issue: dict, labels: dict[str, list[str]], points: str | None, keys: dict[str, str],
              releases: dict[str, str]) -> dict:
    """An issue as the report shows it: its key, what it is and where it sits."""
    metadata = issue.get("metadata") or {}
    blocked_by = metadata.get("blocked_by")
    return {
        "key": issue["identifier"],
        "title": issue["title"],
        "type": issue.get("type"),
        "status": issue["status"],
        "labels": labels.get(issue["id"], []),
        "points": (issue.get("properties") or {}).get(points) if points else None,
        "due_date": issue.get("due_date"),
        "release": releases.get(issue.get("release_id"), issue.get("release_id")),
        "parent": keys.get(issue.get("parent_issue_id"), issue.get("parent_issue_id")),
        "stage": issue.get("stage"),
        "assignee": {"type": issue["assignee_type"], "id": issue["assignee_id"]} if issue.get("assignee_id") else None,
        "blocked_by": [k.strip().upper() for k in blocked_by.split(",") if k.strip()] if isinstance(blocked_by, str)
        else blocked_by or [],
        "change": metadata.get("change"),
    }


def with_verifier(row: dict, verifier: str | None) -> bool:
    """Whether the verifier holds the issue: its pull request is open and its builder assigned it to him."""
    return verifier is not None and row["assignee"] == {"type": "agent", "id": verifier}


def finished(row: dict, verifier: str | None) -> bool:
    """Whether the issue is finished in its lane: closed, or held by the verifier."""
    return row["status"] in CLOSED or with_verifier(row, verifier)


def holds(row: dict | None, verifier: str | None) -> bool:
    """Whether the issue still holds work that needs it: it is not finished in its lane."""
    return row is not None and not finished(row, verifier)


def client_wait(row: dict, clients: frozenset[str]) -> bool:
    """Whether the issue waits on a client: it is assigned to a member with role `client`."""
    return row["assignee"] is not None and row["assignee"]["type"] == "member" and row["assignee"]["id"] in clients


def waits_on(row: dict, before: list[dict], rows: dict[str, dict], verifier: str | None,
             clients: frozenset[str] = frozenset()) -> list[dict]:
    """What holds the issue, as Multica has it; `before` are the issues ahead of it in its lane."""
    out = []
    if row["status"] == "blocked":
        out.append({"kind": "blocked"})
    for key in row["blocked_by"]:
        other = rows.get(key)
        if other is None or holds(other, verifier):
            out.append({"kind": "issue", "key": key, "status": other["status"] if other else None})
    if row["parent"] and row["stage"] is not None:
        out += [
            {"kind": "step", "key": other["key"], "status": other["status"]}
            for other in rows.values()
            if other["parent"] == row["parent"] and other["stage"] is not None and other["stage"] < row["stage"]
            and holds(other, verifier)
        ]
    if not client_wait(row, clients):
        out += [
            {"kind": "client_wait", "key": other["key"], "title": other["title"], "due_date": other["due_date"]}
            for other in before
            if client_wait(other, clients) and other["release"] == row["release"] and other["status"] not in CLOSED
        ]
    return out


def lane_head(rows: list[dict], known: dict[str, dict], verifier: str | None,
              clients: frozenset[str] = frozenset()) -> dict:
    """The lane's active issue, the first that has started and is not finished, and its next, the first that has not
    started, each with what it waits on."""
    head = {"active": None, "next": None}
    for i, row in enumerate(rows):
        if finished(row, verifier):
            continue
        slot = "active" if row["status"] in STARTED else "next"
        if head[slot] is None:
            head[slot] = {**row, "waits_on": waits_on(row, rows[:i], known, verifier, clients)}
        if head["active"] and head["next"]:
            break
    return head


def find_rows(keys: set[str], rows: dict[str, dict], labels, points, ids, releases) -> None:
    """Read each issue a `blocked_by` names that no lane gave, into `rows`."""
    for key in sorted(keys - rows.keys()):
        try:
            issue = multica("issue", "get", key)
        except StateError:
            continue
        labels[issue["id"]] = [label["name"] for label in issue.get("labels") or []]
        rows[key] = issue_row(issue, labels, points, ids, releases)


def collect_multica(repos: list[str], verifier_name: str, lane: str | None = None) -> dict:
    """The lanes, releases and lane-less open issues of the repositories, from Multica."""
    points = points_property()
    verifier = next((a["id"] for a in multica("agent", "list") if a["name"] == verifier_name), None)
    clients = frozenset(m["user_id"] for m in multica("workspace", "member", "list") if m.get("role") == "client")
    repositories, lanes_by_repo, issues_by_lane, project_ids = [], {}, {}, set()
    releases: dict[str, str] = {}
    for name in repos:
        entry = {"name": name}
        try:
            entry["releases"] = multica("release", "list", "--repository", name)
            lanes_by_repo[name] = multica("worklane", "list", "--repo", name)
            for w in lanes_by_repo[name]:
                issues_by_lane[w["id"]] = multica("worklane", "issues", w["id"], "--repo", name)
                project_ids |= {i["project_id"] for i in issues_by_lane[w["id"]] if i.get("project_id")}
        except (StateError, KeyError, TypeError) as e:
            entry["error"] = str(e)
            lanes_by_repo.pop(name, None)
        for r in entry.get("releases", []):
            releases[r["id"]] = r["version"]
        repositories.append(entry)

    projects = {p["id"]: p for p in multica("project", "list") if p["id"] in project_ids}
    try:
        orgs = organisations()
    except (OSError, ValueError, KeyError, urllib.error.URLError) as e:
        orgs = {"error": f"organisations: {e}"}
    labels: dict[str, list[str]] = {}
    unlaned, out_projects = [], []
    ids: dict[str, str] = {}
    issues_of_project: dict[str, list[dict]] = {}
    for pid, p in sorted(projects.items(), key=lambda kv: kv[1]["title"]):
        row = {"id": pid, "title": p["title"],
               "organisation": orgs.get("error") and orgs or orgs.get(p.get("organisation_id"))}
        try:
            issues_of_project[pid] = project_issues(pid)
        except (StateError, KeyError, TypeError) as e:
            row["error"] = str(e)
        out_projects.append(row)
    for issues in [*issues_by_lane.values(), *issues_of_project.values()]:
        for i in issues:
            ids[i["id"]] = i["identifier"]
            if "labels" in i:
                labels[i["id"]] = [label["name"] for label in i["labels"] or []]

    rows: dict[str, dict] = {}
    lane_rows: dict[str, list[dict]] = {}
    for lane_id, issues in issues_by_lane.items():
        lane_rows[lane_id] = [issue_row(i, labels, points, ids, releases) for i in issues]
        rows.update({r["key"]: r for r in lane_rows[lane_id]})
    for issues in issues_of_project.values():
        for i in issues:
            if i["identifier"] not in rows:
                rows[i["identifier"]] = issue_row(i, labels, points, ids, releases)
            if not i.get("worklane_id") and i["status"] not in CLOSED:
                unlaned.append(rows[i["identifier"]])
    find_rows({k for r in rows.values() for k in r["blocked_by"]}, rows, labels, points, ids, releases)

    lanes = []
    for name in repos:
        for w in lanes_by_repo.get(name, []):
            if lane and lane not in (w["id"], w["name"]):
                continue
            mine = lane_rows[w["id"]]
            lanes.append({
                "repository": name,
                "id": w["id"],
                "name": w["name"],
                "is_main": w.get("is_main"),
                "state": w.get("state"),
                "branch": w.get("branch"),
                "path": w.get("path"),
                "multica_active": (w.get("active_issue") or {}).get("identifier"),
                "multica_next": (w.get("next_issue") or {}).get("identifier"),
                **lane_head(mine, rows, verifier, clients),
                "verifier": [r["key"] for r in mine if r["status"] not in CLOSED and with_verifier(r, verifier)],
                "issues": [r for r in mine if r["status"] not in CLOSED],
                "closed": sum(r["status"] in CLOSED for r in mine),
            })

    for entry in repositories:
        for r in entry.get("releases", []):
            mine = [row for row in rows.values() if row["release"] == r["version"]]
            r["open"] = [row["key"] for row in mine if row["status"] not in CLOSED]
            r["done"] = sum(row["status"] in CLOSED for row in mine)
            r["milestones"] = [{"name": m["name"], "date": m["date"]} for m in r.get("milestones") or []]
            for noise in ("repository_id", "created_at", "updated_at"):
                r.pop(noise, None)
    return {"projects": out_projects, "repositories": repositories, "lanes": lanes, "unlaned": unlaned,
            "keys": set(rows)}


# ---------------------------------------------------------------- repositories


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


def graphql(query: str, **variables: str | None) -> dict:
    cmd = ["gh", "api", "graphql", "-f", f"query={query}"]
    for key, value in variables.items():
        cmd += ["-F", f"{key}=null"] if value is None else ["-f", f"{key}={value}"]
    return run_json(cmd)["data"]


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


# ---------------------------------------------------------------- client messages


def endeavour_clients(endeavour: Path, named: list[str]) -> list[str]:
    """The clients whose agent file the endeavour's `AGENTS.md` names, then each named one."""
    try:
        text = (endeavour / "AGENTS.md").read_text(encoding="utf-8", errors="replace")
    except OSError:
        text = ""
    return list(dict.fromkeys([*CLIENT_FILE_RE.findall(text), *named]))


def client_chats(clients: list[str], root: Path = CLIENTS_ROOT) -> list[tuple[str, list[str]] | dict]:
    """Each client with the chat IDs its agent file holds; an agent file that cannot be read, with its error."""
    out: list[tuple[str, list[str]] | dict] = []
    for client in clients:
        f = root / client / "AGENTS.md"
        try:
            text = f.read_text(encoding="utf-8")
        except (OSError, UnicodeError) as e:
            out.append({"client": client, "file": str(f), "error": str(e)})
            continue
        out.append((client, list(dict.fromkeys(CHAT_ID_RE.findall(text)))))
    return out


def beeper_get(path: str, token: str) -> dict:
    request = urllib.request.Request(BEEPER + path, headers={"Authorization": f"Bearer {token}"})
    try:
        with urllib.request.urlopen(request, timeout=20) as response:
            return json.load(response)
    except (urllib.error.URLError, OSError, ValueError) as e:
        raise StateError(f"Beeper {path.split('?')[0]}: {e}") from e


def beeper_token() -> str:
    if token := os.environ.get("BEEPER_ACCESS_TOKEN"):
        return token
    try:
        return run(["security", "find-generic-password", "-s", "beeper-access-token", "-w"]).strip()
    except StateError as e:
        raise StateError("no BEEPER_ACCESS_TOKEN and no Keychain item beeper-access-token") from e


def collect_messages(clients: list[str]) -> list[dict]:
    """The latest messages of each chat of the clients, oldest first; a chat that cannot be read, with its error."""
    found = client_chats(clients)
    out = [c for c in found if isinstance(c, dict)]
    chats = [(c[0], chat) for c in found if isinstance(c, tuple) for chat in c[1]]
    if not chats:
        return out
    try:
        token = beeper_token()
    except StateError as e:
        return out + [{"client": client, "chat": chat, "error": str(e)} for client, chat in chats]
    for client, chat in chats:
        path = "/v1/chats/" + urllib.parse.quote(chat, safe="")
        try:
            title = beeper_get(path, token).get("title")
            page = beeper_get(f"{path}/messages?limit={MESSAGES_PER_CHAT}", token)
        except StateError as e:
            out.append({"client": client, "chat": chat, "error": str(e)})
            continue
        messages = [
            {
                "from": m.get("senderName"),
                "ours": m.get("isSender"),
                "at": m.get("timestamp"),
                "type": m.get("type"),
                "text": m.get("text"),
                "attachments": [
                    {k: a.get(k) for k in ("type", "mimeType", "fileName", "isVoiceNote", "srcURL")}
                    for a in m.get("attachments") or []
                ],
            }
            for m in reversed(page.get("items", []))
        ]
        out.append({"client": client, "chat": chat, "title": title, "messages": messages})
    return out


def github_login() -> str | None:
    try:
        return run(["gh", "api", "user", "--jq", ".login"]).strip() or None
    except StateError:
        return None


# ---------------------------------------------------------------- endeavour


def endeavour_repositories(endeavour: Path, extra: list[str]) -> dict[str, list[Path]]:
    """The endeavour's repositories by GitHub name with their main checkouts: those in its folder, then each extra,
    an `owner/name` or a checkout's path."""
    repos = local_checkouts(endeavour)
    for item in extra:
        name, home = named_repository(item)
        checkouts = repos.setdefault(name, [])
        if home and home not in checkouts:
            checkouts.append(home)
    return repos


def named_repository(item: str) -> tuple[str, Path | None]:
    """A repository given as a checkout's path or as `owner/name`: its lowercase GitHub name and main checkout."""
    path = Path(item).expanduser()
    if path.is_dir():
        home = repo_home(path.resolve())
        name = origin_name(home)
        if not name:
            raise StateError(f"{home} has no GitHub origin")
        return name, home
    if re.fullmatch(r"[\w.-]+/[\w.-]+", item):
        return item.lower(), None
    raise StateError(f"no folder and no owner/name: {item}")


def collect(endeavour: Path, extra: list[str], verifier: str, lane: str | None = None, clients: list[str] = ()) -> dict:
    found = endeavour_repositories(endeavour, extra)
    registered = registered_repositories()
    names = [name for name in sorted(found) if name in registered]
    planning = collect_multica(names, verifier, lane)
    keys = planning.pop("keys")
    summaries, changes, prs = [], [], []
    for name in names:
        summary, c, p = collect_repo(name, found[name])
        summaries.append(summary)
        changes += c
        prs += [{**pr, "issues": [k for k in pr["issues"] if k in keys]} for pr in p]
    return {
        "endeavour": str(endeavour),
        "login": github_login(),
        **planning,
        "unregistered": sorted(set(found) - registered),
        "repos": summaries,
        "changes": changes,
        "prs": prs,
        "messages": collect_messages(endeavour_clients(endeavour, list(clients))),
    }


# ---------------------------------------------------------------- repository intake


def key_order(key: str) -> tuple[str, int]:
    m = ISSUE_KEY_RE.fullmatch(key)
    return (m.group(1).upper(), int(m.group(2))) if m else (key, 0)


def workspace_issues() -> dict[str, dict]:
    """Every issue of the workspace by key, in any project, lane or assignee, with its board `position`."""
    issues = issue_pages("--fields", ISSUE_FIELDS)
    ids = {i["id"]: i["identifier"] for i in issues}
    return {i["identifier"]: {**issue_row(i, {}, None, ids, {}), "position": i.get("position")} for i in issues}


def claimed_keys(pr: dict) -> list[str]:
    """The issue keys the pull request claims as Multica reads them: in its title or branch, or after a closing keyword
    (`Closes WORK-1`) in its title or body."""
    closing = [f"{m.group(1).upper()}-{m.group(2)}" for t in (pr["title"], pr["body"]) for m in CLOSING_KEY_RE.finditer(t or "")]
    return list(dict.fromkeys([*issue_keys(pr["title"], pr["branch"]), *closing]))


def per_issue(keys: list[str], *command: str) -> tuple[dict[str, object], list[str]]:
    """Each issue's `multica <command> KEY` output by key, read `ISSUE_READERS` at a time for what the CLI lists per
    issue only, and each key that could not be read, with its error."""
    def read(key: str) -> tuple[str, object, str | None]:
        try:
            return key, multica(*command, key), None
        except StateError as e:
            return key, None, str(e)

    found, failed = {}, []
    with ThreadPoolExecutor(ISSUE_READERS) as pool:
        for key, out, error in pool.map(read, keys):
            if error is None:
                found[key] = out
            else:
                failed.append(f"{key}: {error}")
    return found, failed


def pull_request_links(read: dict[str, object], repository: str) -> dict[int, list[dict]]:
    """The Multica links of the repository's pull requests by number, from each issue's `issue pull-requests` output,
    each `{key, source}` with Multica's link `source`."""
    found: dict[int, list[dict]] = {}
    for key, out in read.items():
        for record in (out.get("pull_requests") if isinstance(out, dict) else None) or []:
            m = PR_URL_RE.search(record.get("html_url") or "")
            if not m or m.group(1).lower() != repository:
                continue
            links = found.setdefault(int(m.group(2)), [])
            if all(link["key"] != key for link in links):
                links.append({"key": key, "source": record.get("link_source") or "auto"})
    return found


def stack_note(body: str | None, repository: str) -> dict | None:
    """The first `Stacked on` clause naming a concrete lower PR or branch, ending at its sentence or a `which`."""
    for m in STACK_NOTE_RE.finditer(body or ""):
        clause = NOTE_END_RE.split(m.group(1), 1)[0]
        elsewhere = [f"{repo.lower()}#{n}" for repo, n in OTHER_PR_REF_RE.findall(clause) if repo.lower() != repository]
        numbers = [int(n) for n in PR_REF_RE.findall(clause)]
        for repo, n in PR_URL_RE.findall(clause):
            if repo.lower() == repository:
                numbers.append(int(n))
            else:
                elsewhere.append(f"{repo.lower()}#{n}")
        numbers += [int(n) for repo, n in OTHER_PR_REF_RE.findall(clause) if repo.lower() == repository]
        branches = [b for b in CODE_RE.findall(clause) if not SHA_RE.fullmatch(b)]
        if numbers or branches or elsewhere:
            return {
                "text": (m.group(0)[: m.start(1) - m.start(0)] + clause).strip(),
                "prs": list(dict.fromkeys(numbers)),
                "branches": list(dict.fromkeys(branches)),
                "elsewhere": list(dict.fromkeys(elsewhere)),
            }
    return None


def stack_edges(prs: list[dict], default_branch: str | None, resolve, ancestry) -> list[dict]:
    """Each open pull request's edge to the pull request under it, named by its `stack_note` or by a base that is
    another open pull request's branch. `resolve(number=…)`/`resolve(branch=…)` gives a pull request that is not open
    as `{number, state}`, or None; `ancestry(lower, upper)` gives `contains`, `forked` or None. An edge is `verified`
    only by ancestry; verified edges that close a loop become `cycle`. Every named target is reconciled."""
    by_number = {p["number"]: p for p in prs}
    by_branch = {p["branch"]: p for p in prs}
    edges = []
    for pr in sorted(prs, key=lambda p: p["number"]):
        note = pr.get("stack_note")
        based = by_branch.get(pr.get("base")) if pr.get("base") and pr.get("base") != default_branch else None
        if not note and not based:
            continue
        named, gone = {based["number"]} if based else set(), {}
        unresolved = []
        refs = [{"number": n} for n in (note or {}).get("prs", [])] + [{"branch": b} for b in (note or {}).get("branches", [])]
        for ref in refs:
            found = by_number.get(ref["number"]) if "number" in ref else by_branch.get(ref["branch"])
            if found:
                named.add(found["number"])
                continue
            try:
                other = resolve(**ref)
            except StateError:
                other = None
            if other:
                gone[other["number"]] = other["state"]
            else:
                unresolved.append(ref)
        named.discard(pr["number"])
        gone.pop(pr["number"], None)
        edge = {"upper": pr["number"], "lower": None, "note": note["text"] if note else None,
                "base": pr.get("base") if based else None}
        candidates = sorted(named | gone.keys())
        if len(candidates) > 1:
            edge["candidates"] = candidates
        closed = sorted(n for n, state in gone.items() if state != "MERGED")
        if unresolved:
            edge.update(status="unknown_target", unresolved_refs=unresolved)
        elif closed:
            edge.update(status="lower_closed", lower=closed[0] if len(closed) == 1 else None)
        elif len(named) > 1:
            edge["status"] = "ambiguous"
        elif named:
            edge["lower"] = lower = named.pop()
            try:
                kind = ancestry(by_number[lower], pr)
            except StateError as e:
                kind, edge["error"] = None, str(e)
            edge["status"] = "verified" if kind else "ancestry_unproven"
            if kind:
                edge["ancestry"] = kind
        elif gone:
            merged = sorted(gone)
            edge.update(status="lower_merged", lower=merged[0] if len(merged) == 1 else None)
        elif note and note["elsewhere"]:
            edge.update(status="other_repository", candidates=note["elsewhere"])
        else:
            edge["status"] = "unknown_target"
        edges.append(edge)

    lower_of = {e["upper"]: e for e in edges if e["status"] == "verified"}
    for start in lower_of:
        path, n = [], start
        while n in lower_of and n not in path:
            path.append(n)
            n = lower_of[n]["lower"]
        if n in path:
            for member in path[path.index(n):]:
                lower_of[member]["status"] = "cycle"
    return edges


def dependents_of(prs: list[dict], waits_of: dict[str, list[dict]], edges: list[dict]) -> dict[object, set]:
    """Downstream links between issues and their carrying PRs, issue waits and verified upper-PR edges."""
    dependents: dict[object, set] = {}
    for key, ws in waits_of.items():
        for w in ws:
            dependents.setdefault(w["key"], set()).add(key)
    for p in prs:
        for key in p["issues"]:
            dependents.setdefault(("pr", p["number"]), set()).add(key)
            dependents.setdefault(key, set()).add(("pr", p["number"]))
    for e in edges:
        if e["status"] == "verified":
            dependents.setdefault(("pr", e["lower"]), set()).add(("pr", e["upper"]))
    return dependents


def standing(pr: dict, issues: dict[str, dict], dependents: dict[object, set]) -> dict:
    """The PR's linked issues, earliest board position and distinct transitive downstream open issues."""
    keys = list(dict.fromkeys(pr["issues"]))
    seen = set(keys) | {("pr", pr["number"])}
    todo, downstream = list(seen), set()
    while todo:
        for d in dependents.get(todo.pop(), ()):
            if d in seen:
                continue
            if isinstance(d, str):
                if d not in issues or issues[d]["status"] in CLOSED:
                    continue
                downstream.add(d)
            seen.add(d)
            todo.append(d)
    positions = [issues[k]["position"] for k in keys if k in issues and issues[k].get("position") is not None]
    return {
        "issues": keys,
        "position": min(positions) if positions else None,
        "downstream": sorted(downstream, key=key_order),
        "downstream_count": len(downstream),
    }


def pr_order(pr: dict) -> tuple:
    """Most unblocking first, then in board order, then by pull request number."""
    return -pr["downstream_count"], pr["position"] is None, pr["position"] or 0, pr["number"]


def children_of(issues: dict[str, dict]) -> dict[str, list[dict]]:
    """The open sub-issues of each issue, such as its Fixes."""
    children: dict[str, list[dict]] = {}
    for row in issues.values():
        if row["parent"] and row["status"] not in CLOSED:
            children.setdefault(row["parent"], []).append(row)
    return children


def blockers(pr: dict, keys: list[str], issues: dict[str, dict], verifier: str | None,
             waits_of: dict[str, list[dict]], edges: list[dict], children: dict[str, list[dict]],
             missing: Sequence[str] = ()) -> list[dict]:
    """Missing evidence, stack relationships, assignment and dependencies that hold this pull request."""
    reasons = [{"kind": "evidence", "missing": list(missing)}] if missing else []
    reasons += [{"kind": "stack_edge", "pr": e["upper"], "lower": e["lower"], "status": e["status"]}
                for e in edges if e["upper"] == pr["number"] and e["status"] in UNRESOLVED_EDGES]
    reasons += [{"kind": "stack", "pr": e["lower"]}
                for e in edges if e["upper"] == pr["number"] and e["status"] == "verified"]
    for key in keys:
        row = issues.get(key)
        if row is None:
            reasons.append({"kind": "evidence", "missing": [f"Multica issue {key}"]})
        elif not finished(row, verifier):
            reasons.append({"kind": "assignee", "key": key, "assignee": row["assignee"]})
        reasons += [w for w in waits_of.get(key, []) if w["key"] not in keys]
        reasons += [{"kind": "child", "key": c["key"], "status": c["status"]}
                    for c in children.get(key, []) if c["key"] not in keys]
    return list({json.dumps(r, sort_keys=True): r for r in reasons}.values())


def waits(issues: dict[str, dict], wakeups: list[dict]) -> dict[str, list[dict]]:
    """What each open issue waits on before its work can land, from Multica as written: the issues its `blocked_by`
    names, the earlier steps of its journey and what an enabled wakeup of it waits for, an issue (`other_issue`) until
    it reaches the wakeup's state, or its children (`children_done`); each while open, or unknown. Unlike `waits_on`,
    an open pull request releases nothing: its work lands only once it merges."""
    children: dict[str, list[dict]] = {}
    for row in issues.values():
        if row["parent"]:
            children.setdefault(row["parent"], []).append(row)
    found: dict[str, list[tuple[str, str]]] = {}
    for key, row in issues.items():
        if row["status"] in CLOSED:
            continue
        found[key] = [(k, "blocked_by") for k in row["blocked_by"]]
        if row["parent"] and row["stage"] is not None:
            found[key] += [(o["key"], "step") for o in children.get(row["parent"], [])
                           if o["stage"] is not None and o["stage"] < row["stage"]]
    for w in wakeups:
        key, condition = w.get("issue_identifier"), w.get("condition") or {}
        if not w.get("enabled") or key not in found:
            continue
        if condition.get("type") == "other_issue" and condition.get("identifier"):
            other = issues.get(condition["identifier"])
            if other is None or other["status"] != condition.get("state"):
                found[key].append((condition["identifier"], "wakeup"))
        elif condition.get("type") == "children_done":
            found[key] += [(c["key"], "wakeup") for c in children.get(key, []) if condition.get("stage") is None
                           or (c["stage"] is not None and c["stage"] <= condition["stage"])]
    out = {}
    for key, pairs in found.items():
        seen, out[key] = set(), []
        for other, via in pairs:
            row = issues.get(other)
            if other in seen or other == key or (row and row["status"] in CLOSED):
                continue
            seen.add(other)
            out[key].append({"kind": via, "key": other, "status": row["status"] if row else None})
    return out


def rank_prs(prs: list[dict], issues: dict[str, dict], verifier: str | None,
             waits_of: dict[str, list[dict]], edges: list[dict],
             missing: Sequence[str] = ()) -> list[dict]:
    """Rank each open PR with its downstream impact and visible blockers."""
    dependents = dependents_of(prs, waits_of, edges)
    children = children_of(issues)
    ranked = []
    for pr in prs:
        held = standing(pr, issues, dependents)
        reasons = blockers(pr, held["issues"], issues, verifier, waits_of, edges, children, missing)
        ranked.append({**pr, **held, "blocked_by": reasons})
    return sorted(ranked, key=pr_order)


def find_pull_request(name: str, number: int | None = None, branch: str | None = None) -> dict | None:
    """The repository's pull request with that number, or the latest from that branch, as `{number, state}` with
    `state` `OPEN`, `MERGED` or `CLOSED`; None when there is none."""
    owner, repo = name.split("/", 1)
    if number is not None:
        query = ("query($owner: String!, $name: String!) { repository(owner: $owner, name: $name) { "
                 f"pullRequest(number: {int(number)}) {{ number state }} }} }}")
        return graphql(query, owner=owner, name=repo)["repository"]["pullRequest"]
    query = ("query($owner: String!, $name: String!, $branch: String!) { repository(owner: $owner, name: $name) { "
             "pullRequests(headRefName: $branch, first: 1, orderBy: {field: CREATED_AT, direction: DESC}) "
             "{ nodes { number state } } } }")
    nodes = graphql(query, owner=owner, name=repo, branch=branch)["repository"]["pullRequests"]["nodes"]
    return nodes[0] if nodes else None


def compare(name: str, base: str, head: str) -> dict:
    """GitHub's comparison of two commits: `status` (`ahead`, `behind`, `diverged` or `identical`) and `merge_base`."""
    return run_json(["gh", "api", f"repos/{name}/compare/{base}...{head}?per_page=1",
                     "--jq", "{status: .status, merge_base: .merge_base_commit.sha}"])


def ancestry(name: str, default_head: str | None, lower: dict, upper: dict) -> str | None:
    """How the upper pull request holds the lower one's work: `contains` its head, `forked` from a commit of it that
    is not on the default branch, the lower having commits since; None when it holds none of its unmerged work."""
    found = compare(name, lower["head"], upper["head"])
    if found["status"] in ("ahead", "identical"):
        return "contains"
    if found["status"] != "diverged" or not found.get("merge_base") or not default_head:
        return None
    return None if compare(name, found["merge_base"], default_head)["status"] in ("ahead", "identical") else "forked"


def collect_intake(name: str, verifier_name: str) -> dict:
    """One repository's ranked individual pull requests, ready and blocked; see the docstring."""
    try:
        github = collect_github(name, intake=True)
    except (KeyError, TypeError) as e:
        raise StateError(f"GitHub: {e}") from e
    prs = sorted(github.pop("prs"), key=lambda p: p["number"])
    errors, missing = [], []
    try:
        issues = workspace_issues()
    except (StateError, KeyError, TypeError) as e:
        issues = None
        errors.append(f"Multica issues: {e}")
        missing.append("Multica issues")
    try:
        verifier = next((a["id"] for a in multica("agent", "list") if a["name"] == verifier_name), None)
        if verifier is None:
            raise StateError(f"verifier {verifier_name!r} not found")
    except (StateError, KeyError, TypeError) as e:
        verifier = None
        errors.append(f"Multica agents: {e}")
        missing.append("Multica agents")
    claims = {p["number"]: claimed_keys(p) for p in prs}
    linked: dict[int, list[dict]] = {}
    wakeups: list[dict] = []
    if issues is not None:
        open_keys = sorted((k for k, row in issues.items() if row["status"] not in CLOSED), key=key_order)
        read, failed = per_issue(sorted(issues, key=key_order), "issue", "pull-requests")
        linked = pull_request_links(read, name)
        if failed:
            errors.append("Multica links: " + "; ".join(failed))
            missing.append("Multica links")
        read, failed = per_issue(open_keys, "issue", "wakeup", "list")
        failed += [f"{k}: not a list of wakeups" for k, out in read.items() if not isinstance(out, list)]
        wakeups = [{**w, "issue_identifier": k} for k, out in read.items() if isinstance(out, list) for w in out]
        if failed:
            errors.append("Multica wakeups: " + "; ".join(failed))
            missing.append("Multica wakeups")
    for pr in prs:
        links = linked.get(pr["number"], [])
        pr["links"] = links + [{"key": k, "source": "claimed"} for k in claims[pr["number"]]
                               if all(link["key"] != k for link in links)]
        pr["issues"] = [link["key"] for link in pr["links"]]
        pr["stack_note"] = stack_note(pr["body"], name)
    if issues is not None:
        unknown = sorted({k for p in prs for k in p["issues"] if k not in issues}, key=key_order)
        if unknown:
            errors.append("Multica issues: missing " + ", ".join(unknown))
    edges = stack_edges(prs, github["default_branch"], lambda **ref: find_pull_request(name, **ref),
                        lambda lower, upper: ancestry(name, github["default_head"], lower, upper))
    issues = issues or {}
    waits_of = waits(issues, wakeups)
    prs = rank_prs(prs, issues, verifier, waits_of, edges, missing)
    linked_keys = sorted({k for p in prs for k in p["issues"] if k in issues}, key=key_order)
    out = {
        "repository": name,
        **github,
        "prs": prs,
        "issues": {k: issues[k] for k in linked_keys},
        "edges": edges,
        "unresolved": [e for e in edges if e["status"] in UNRESOLVED_EDGES],
        "issueless": [p["number"] for p in prs if not p["issues"]],
        "ready": [p for p in prs if not p["blocked_by"]],
        "blocked": [p for p in prs if p["blocked_by"]],
    }
    if errors:
        out["errors"] = errors
    return out


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Collect the open work of one endeavour, or one repository's intake.")
    parser.add_argument("endeavour", nargs="?", help="the endeavour's folder, such as ~/Repos/Clients/rebl")
    parser.add_argument("--repo", action="append", default=[], help="a repository beside the endeavour's own: owner/name or a checkout")
    parser.add_argument("--verifier", required=True, help="the name of the Multica agent in the verifier's role, which holds an issue once its pull request is open")
    parser.add_argument("--lane", help="the worklane to run on, by name or id: only its lane is reported")
    parser.add_argument("--client", action="append", default=[], help="a client whose chats to read beside those the endeavour names")
    parser.add_argument("--intake", metavar="REPOSITORY",
                        help="repository intake instead: every open pull request of one repository, owner/name or a checkout, ranked individually")
    args = parser.parse_args(argv)
    if args.intake and (args.endeavour or args.repo or args.lane or args.client):
        parser.error("--intake takes no endeavour, --repo, --lane or --client")
    if not args.intake and not args.endeavour:
        parser.error("an endeavour folder or --intake is required")
    try:
        if args.intake:
            found = collect_intake(named_repository(args.intake)[0], args.verifier)
        else:
            endeavour = Path(args.endeavour).expanduser().resolve()
            if not endeavour.is_dir():
                raise StateError(f"not a folder: {endeavour}")
            found = collect(endeavour, args.repo, args.verifier, args.lane, args.client)
        print(json.dumps(found, ensure_ascii=False, default=str))
        return 0
    except (StateError, OSError, ValueError, KeyError) as e:
        print(json.dumps({"error": str(e)}), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
