"""tickets' ticket tree: the epics and stories of one Multica project, their changes and spikes, and what is next."""

import json
import re
import subprocess
import sys
from functools import cache
from pathlib import Path

from tickets_text import (
    CHANGE_RE, GITHUB_RE, GREEN_LIGHT, HITL, LEAF_TYPES, LISTS, RISKS, STATE_OF, STATUSES, TicketError, flag, one_of,
    items_of, keys_of, label_names, one_label, read_build, sections, spike_build, text_of,
)


MULTICA = ("multica", "--profile", "skuddy")


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
        status = one_of(build.get("status", ""), STATUSES, f"{name}/.openspec.yaml", "x-build.status")
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
    status = one_of(fm.get("status", ""), STATUSES, f"story {row['ref']} ## Build", "status")
    row.update(
        {
            "status": status,
            "state": STATE_OF[status],
            "refined": flag(fm.get("refined", False)),
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


# ---------------------------------------------------------------- loading


def scope_of(args) -> tuple[str | None, str, str]:
    """(the epic's id or None for a project, the scope as given, the project id)."""
    if args.project:
        return None, args.project, args.project
    epic = multica("issue", "get", args.epic)
    if epic.get("type") != "epic":
        raise TicketError(f"{args.epic} is a {epic.get('type') or 'story'}, not an epic")
    if not epic.get("project_id"):
        raise TicketError(f"epic {epic['identifier']} is in no project")
    return epic["id"], epic["identifier"], epic["project_id"]


def story_row(s: dict, epic: str, project: str, changes: list[Path]) -> dict:
    """The row of one story issue."""
    ref = s["identifier"]
    where = f"story {ref}"
    kind = s.get("type") or "story"
    if kind not in LEAF_TYPES:
        raise TicketError(f"{where}: type {kind!r} is not one of {', '.join(LEAF_TYPES)}")
    names = label_names(s)
    meta = s.get("metadata") or {}
    change = str(meta.get("change") or "")
    markdown = s.get("description") or ""
    found = sections(markdown)
    row = {
        "key": s["id"],
        "epic": epic,
        "id": s["id"],
        "ref": ref,
        "type": kind,
        "green_lights": ["brian"] if GREEN_LIGHT in names else [],
        "title": str(s.get("title") or ""),
        "change": change,
        "change_dir": None,
        "spike_file": None,
        "status": "",
        "state": "planned",
        "assignee": "",
        "refined": False,
        "refine": s.get("status") == "backlog",
        "risk": one_of(one_label(names, "risk:", where), RISKS, where, "risk"),
        "hitl": HITL in names,
        "estimate": points(s),
        "blocked_at": "",
        "blocked_reason": "",
        "raw_after": keys_of(meta.get("blocked_by")),
        "worklane": s.get("worklane_id") or "",
        "markdown": markdown,
        **{name: text_of(found.get(name, [])) for name in ("description", "verify", "unknown")},
        **{name: items_of(found.get(name, [])) for name in LISTS},
    }
    if kind == "spike":
        if change:
            raise TicketError(f"{where} is a spike, and a spike produces no change")
        row["spike_file"] = ref
        if s.get("status") != "cancelled":
            read_spike(row, markdown)
    elif change:
        if not CHANGE_RE.match(change):
            raise TicketError(f"{where}: change {change!r} is not a kebab-case change name")
        lane = row["worklane"]
        read_change(row, changes, where, lane_repositories(project).get(lane) if lane else None)
    if s.get("status") == "cancelled":
        row.update({"status": "dropped", "state": "dropped"})
    return row


def epic_after(e: dict, epics: dict, by_ref: dict) -> list[tuple[str, str]]:
    """(epic id, what it needs of it) for each epic this epic waits on."""
    blockers = keys_of((e.get("metadata") or {}).get("blocked_by"))
    if not blockers:
        return []
    lines = items_of(sections(e.get("description") or "").get("after", []))
    after = []
    for needed in blockers:
        key = by_ref.get(needed.lower(), needed)
        if key not in epics:
            raise TicketError(f"epic {e['identifier']} is after {needed!r}, which is no epic of its project")
        title = epics[key]["title"]
        needs = next((line[len(title) + 1 :].strip() for line in lines if line.startswith(f"{title}:")), "")
        after.append((key, needs))
    return after


def load_tree(args, changes: list[Path]) -> dict:
    """The epic or project asked about, with every epic of the project and their stories."""
    scope, label, project = scope_of(args)
    issues = project_issues(project)
    found = [i for i in issues if i.get("type") == "epic"]
    epics = {e["id"]: {"title": e.get("title") or "", "ref": e["identifier"], "after": []} for e in found}
    tickets = [story_row(s, s["parent_issue_id"], project, changes)
               for s in issues if s.get("parent_issue_id") in epics and s.get("type") != "epic"]
    by_ref = {i["identifier"].lower(): i["id"] for i in issues}
    for e in found:
        epics[e["id"]]["after"] = epic_after(e, epics, by_ref)
    tree = {"scope": scope, "label": label, "project": project, "epics": epics, "tickets": tickets}
    _resolve(tree, by_ref)
    _check_cycles(tickets)
    return tree


def _resolve(tree: dict, by_ref: dict) -> None:
    by_key = {t["key"]: t for t in tree["tickets"]}
    for t in tree["tickets"]:
        keys = []
        for b in t.pop("raw_after"):
            key = by_ref.get(b.lower(), b)
            if key not in by_key and key not in tree["epics"]:
                raise TicketError(f"story {t['ref']}: after {b!r} matches no story or epic of the tree")
            if key not in keys:
                keys.append(key)
        t["after"] = keys


def _check_cycles(tickets: list[dict]) -> None:
    sys.setrecursionlimit(max(1000, 3 * len(tickets) + 100))
    graph = {t["key"]: t["after"] for t in tickets}
    members = {}
    for t in tickets:
        members.setdefault(t["epic"], []).append(t["key"])
    names = {t["key"]: t["ref"] for t in tickets}
    state = {}

    def visit(node, path):
        if state.get(node) == "done":
            return
        if state.get(node) == "active":
            raise TicketError("cycle through " + " -> ".join(names.get(n, n) for n in path + [node]))
        state[node] = "active"
        for b in graph.get(node, members.get(node, [])):
            visit(b, path + [node])
        state[node] = "done"

    for node in graph:
        visit(node, [])


# ---------------------------------------------------------------- views


def epic_done(tree: dict, epic: str) -> bool:
    mine = [t for t in tree["tickets"] if t["epic"] == epic]
    return bool(mine) and all(t["state"] in ("done", "dropped") for t in mine)


def done_keys(tree: dict) -> set:
    done = {t["key"] for t in tree["tickets"] if t["state"] == "done"}
    return done | {epic for epic in tree["epics"] if epic_done(tree, epic)}


def in_scope(tree: dict) -> list[dict]:
    return [t for t in tree["tickets"] if tree["scope"] in (None, t["epic"])]


def buildable(after: list, done: set, review: set) -> bool:
    """Whether a story's prerequisites let its build start: each is done or in review, and at most
    one is in review, since the story's branch is cut from that one open pull request's branch."""
    return all(b in done or b in review for b in after) and sum(b in review for b in after) <= 1


def classify(tree: dict) -> dict:
    done = done_keys(tree)
    review = {t["key"] for t in tree["tickets"] if t["state"] == "review"}
    groups = {"ready_to_refine": [], "ready_to_start": [], "in_progress": [], "blocked": []}
    for t in in_scope(tree):
        s = t["state"]
        if s in ("done", "dropped"):
            continue
        if t["status"] == "blocked" or t["blocked_at"]:
            groups["blocked"].append(t)
        elif s in ("in-progress", "review"):
            groups["in_progress"].append(t)
        elif not buildable(t["after"], done, review):
            groups["blocked"].append(t)
        elif t["refine"] and not t["refined"]:
            groups["ready_to_refine"].append(t)
        else:
            groups["ready_to_start"].append(t)
    return groups


def longest_remaining_chain(tree: dict) -> list:
    remaining = {t["key"]: t for t in tree["tickets"] if t["state"] not in ("done", "dropped")}
    memo = {}

    def chain(k):
        if k in memo:
            return memo[k]
        best = []
        for b in remaining[k]["after"]:
            if b in remaining:
                c = chain(b)
                if len(c) > len(best):
                    best = c
        memo[k] = best + [k]
        return memo[k]

    longest = []
    for t in in_scope(tree):
        if t["key"] in remaining:
            c = chain(t["key"])
            if len(c) > len(longest):
                longest = c
    return [ref(k, tree) for k in longest]


def ref(key: str, tree: dict) -> str:
    """The key of a story or an epic."""
    if key in tree["epics"]:
        return tree["epics"][key]["ref"]
    return next(t["ref"] for t in tree["tickets"] if t["key"] == key)


def unpinned_after(tree: dict) -> list[dict]:
    """`after` of an epic that has stories but none waiting on the epic it names."""
    by_key = {t["key"]: t for t in tree["tickets"]}
    out = []
    for epic, e in tree["epics"].items():
        mine = [t for t in tree["tickets"] if t["epic"] == epic]
        for needed, needs in e["after"]:
            pinned = any(b == needed or by_key.get(b, {}).get("epic") == needed for t in mine for b in t["after"])
            if mine and not pinned and tree["scope"] in (None, epic):
                out.append({"epic": e["ref"], "after": ref(needed, tree), "needs": needs})
    return out

def public(t: dict, tree: dict, blocks: dict | None = None) -> dict:
    row = {
        k: t[k]
        for k in (
            "epic",
            "id",
            "type",
            "green_lights",
            "title",
            "change",
            "status",
            "state",
            "assignee",
            "risk",
            "hitl",
            "covers",
            "estimate",
            "refine",
            "refined",
            "blocked_at",
            "blocked_reason",
        )
    }
    row["epic"] = ref(t["epic"], tree)
    row["ref"] = t["ref"]
    row["after"] = [ref(b, tree) for b in t["after"]]
    if blocks is not None:
        row["blocks"] = [ref(b, tree) for b in blocks.get(t["key"], [])]
    return row
