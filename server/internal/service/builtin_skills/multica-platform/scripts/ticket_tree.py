"""The ticket tree `tickets.py` answers from: its epics and stories, their order, and what is next."""

import re
import sys
from pathlib import Path

from ticket_sources import lane_repositories, multica, points, project_issues, read_change, read_spike
from ticket_text import TicketError, _one_of, items_of, keys_of, label_names, one_label, sections, text_of

LEAF_TYPES = ("story", "bug", "enhancement", "refactor", "chore", "spike")
RISKS = ("low", "medium", "high")
CHANGE_RE = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
GREEN_LIGHT = "green light: Brian"
HITL = "hitl"
LISTS = ("references", "notes", "covers")


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
        "risk": _one_of(one_label(names, "risk:", where), RISKS, where, "risk"),
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
