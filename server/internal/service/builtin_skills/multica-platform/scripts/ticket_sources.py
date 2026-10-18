"""Where `tickets.py` reads a ticket tree from: OpenSpec changes in git checkouts, and issues in Multica."""

import json
import re
import subprocess
from functools import cache
from pathlib import Path

from ticket_text import TicketError, _flag, _one_of, read_build, spike_build

MULTICA = ("multica", "--profile", "skuddy")
STATUSES = ("draft", "ready-for-dev", "in-progress", "in-review", "built", "done", "blocked", "dropped")
STATE_OF = {
    "": "backlog",
    "draft": "backlog",
    "ready-for-dev": "backlog",
    "in-progress": "in-progress",
    "blocked": "in-progress",
    "in-review": "review",
    "built": "review",
    "done": "done",
    "dropped": "dropped",
}
GITHUB_RE = re.compile(r"github\.com[:/]([\w.-]+/[\w.-]+?)(?:\.git)?/?$")


# ---------------------------------------------------------------- changes


def archived_dir(changes: Path, change: str) -> Path | None:
    archive = changes / "archive"
    if not archive.is_dir():
        return None
    pattern = re.compile(r"\d{4}-\d{2}-\d{2}-" + re.escape(change))
    hits = sorted(p for p in archive.iterdir() if p.is_dir() and pattern.fullmatch(p.name))
    return hits[-1] if hits else None


def git(cwd: Path, *args: str) -> str | None:
    try:
        r = subprocess.run(["git", "-C", str(cwd), *args], capture_output=True, encoding="utf-8", check=False)
    except FileNotFoundError:
        return None
    return r.stdout.strip() if r.returncode == 0 else None


@cache
def github_repo(path: Path) -> str | None:
    """The GitHub repository `owner/repo` of the git repository holding path, from its origin; None without one."""
    m = GITHUB_RE.search(git(path, "remote", "get-url", "origin") or "")
    return m.group(1) if m else None


@cache
def merged_archives(changes: Path) -> set[str] | None:
    """The archive folder names on the default branch, or None when git cannot tell."""
    top = git(changes, "rev-parse", "--show-toplevel")
    if top is None:
        return None
    head = git(changes, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD")
    ref = next(
        (r for r in (head, "main", "master") if r and git(changes, "rev-parse", "--verify", "--quiet", r) is not None),
        None,
    )
    if ref is None:
        return None
    rel = (changes / "archive").resolve().relative_to(Path(top).resolve()).as_posix()
    listing = git(Path(top), "ls-tree", "-d", "--name-only", ref, f"{rel}/")
    return {Path(line).name for line in (listing or "").splitlines()}


def read_change(row: dict, changes: list[Path], where: str, repo: str | None) -> None:
    """Set the row's status fields from its change folder, in the first changes folder of its repository that holds it.

    `repo` is the Multica repository id of the story's worklane; None for a story in no worklane, then any folder counts.
    """
    name = row["change"]
    if not changes:
        raise TicketError(f"{where}: change {name!r} needs the project root: no .git found; pass --project-root")
    if repo:
        changes = [c for c in changes if repository_id(github_repo(c)) == repo]
        if not changes:
            raise TicketError(f"{where}: change {name!r} lives in the repository of its worklane, and no project root "
                              "is a clone of it; pass --project-root")
    home = next((c for c in changes if (c / name).is_dir() or archived_dir(c, name) is not None), None)
    if home is None:
        where_to = ", ".join(str(c) for c in changes)
        raise TicketError(f"{where}: change {name!r} is neither in {where_to} nor archived")
    path = home / name
    if path.is_dir():
        meta = path / ".openspec.yaml"
        build = read_build(meta.read_text(encoding="utf-8")) if meta.is_file() else {}
        status = _one_of(build.get("status", ""), STATUSES, f"{name}/.openspec.yaml", "x-build.status")
        row.update(
            {
                "status": status,
                **{k: str(build.get(k, "") or "") for k in ("assignee", "blocked_at", "blocked_reason")},
            }
        )
    else:
        path = archived_dir(home, name)
        merged = merged_archives(home)
        # Archived on its branch but not yet on the default branch: the pull request still waits.
        row["status"] = "done" if merged is None or path.name in merged else "in-review"
    row["state"] = STATE_OF[row["status"]]
    row["change_dir"] = str(path)
    row["refined"] = any((path / "specs").rglob("spec.md"))


def read_spike(row: dict, markdown: str) -> None:
    """Set a spike's status fields from the `## Build` block of its issue's description."""
    fm = spike_build(markdown)
    if not fm:
        return
    status = _one_of(fm.get("status", ""), STATUSES, f"story {row['ref']} ## Build", "status")
    row.update(
        {
            "status": status,
            "state": STATE_OF[status],
            "refined": _flag(fm.get("refined", False)),
            **{k: str(fm.get(k, "") or "") for k in ("assignee", "blocked_at", "blocked_reason")},
        }
    )

# ---------------------------------------------------------------- multica


def multica(*args: str, stdin: str | None = None):
    """The JSON a `multica` command prints."""
    try:
        r = subprocess.run(
            [*MULTICA, *args, "--output", "json"], input=stdin, capture_output=True, encoding="utf-8", check=False
        )
    except FileNotFoundError:
        raise TicketError("the multica CLI is not on PATH") from None
    if r.returncode != 0:
        raise TicketError(f"multica {' '.join(args)} failed: {(r.stderr or r.stdout).strip()}")
    try:
        return json.loads(r.stdout) if r.stdout.strip() else {}
    except ValueError:
        raise TicketError(f"multica {' '.join(args)} printed no JSON: {r.stdout.strip()[:200]}") from None


def project_issues(project: str) -> list[dict]:
    """Every issue of the project, in board order, its properties by name."""
    out, offset = [], 0
    while True:
        page = multica(
            "issue", "list", "--project", project, "--limit", "100", "--offset", str(offset), "--resolve-properties"
        )
        issues = page.get("issues") or []
        out += issues
        if not page.get("has_more") or not issues:
            return out
        offset += len(issues)


@cache
def lane_repositories(project: str) -> dict[str, str]:
    """Each worklane of the project by its id, with its repository's id."""
    return {lane["id"]: lane["repository_id"] for lane in multica("worklane", "list", "--project", project)}


@cache
def repository_id(name: str | None) -> str | None:
    """The Multica id of the repository `owner/name`; None when Multica holds no worklane of it."""
    if not name:
        return None
    try:
        lanes = multica("worklane", "list", "--repo", name)
    except TicketError:
        return None
    return lanes[0]["repository_id"] if lanes else None


def points(issue: dict):
    """The issue's Points; "" when unset."""
    rows = issue.get("properties") or []
    value = next((r.get("value") for r in rows if isinstance(r, dict) and r.get("name") == "Points"), None)
    return "" if value is None else value


# ---------------------------------------------------------------- project


def find_project_root(start: Path) -> Path | None:
    for p in [start, *start.parents]:
        if (p / ".git").exists():
            return p
    return None


def project_roots(args) -> list[Path]:
    """Every --project-root given, else the nearest folder holding .git at or above the working directory."""
    if args.project_root:
        return [Path(r).resolve() for r in args.project_root]
    found = find_project_root(Path.cwd().resolve())
    return [found] if found else []


def openspec_home(root: Path) -> Path:
    """The folder OpenSpec commands run in: `specs/` when the project holds `specs/openspec/`, else the project root.

    The OpenSpec CLI finds its root only in the folder it runs in, so this folder, not the project root, is the
    working directory of every `openspec` call (docs/contract.md).
    """
    specs = root / "specs"
    return specs if (specs / "openspec").is_dir() else root


def changes_dir(root: Path | None) -> Path | None:
    return openspec_home(root) / "openspec" / "changes" if root else None


def worktrees(root: Path) -> list[Path]:
    """Every worktree of the repository holding root, as `git worktree list` gives them; none outside git."""
    listing = git(root, "worktree", "list", "--porcelain") or ""
    return [Path(line.removeprefix("worktree ")) for line in listing.splitlines() if line.startswith("worktree ")]


def changes_dirs(roots: list[Path]) -> list[Path]:
    """The changes folder of each root, then of every other worktree of their repositories that has one."""
    out = [changes_dir(r) for r in roots]
    seen = {c.resolve() for c in out}
    for w in (w for r in roots for w in worktrees(r)):
        c = changes_dir(w)
        if c.is_dir() and c.resolve() not in seen:
            seen.add(c.resolve())
            out.append(c)
    return out
