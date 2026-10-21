"""team_state's `--in-review`: every In Review issue of the workspace with its place in its lane's stack."""

from __future__ import annotations

from team_state_intake import key_order, per_issue
from team_state_io import multica
from team_state_multica import issue_pages


ISSUE_FIELDS = "identifier,title,status,project_id,release_id,worklane_id"


def stacks(lanes: set[str], in_review: set[str]) -> dict[str, list[str]]:
    """The layers of each lane's stack: its In Review issues in lane order, the bottom layer first."""
    return {lane: [i["identifier"] for i in multica("worklane", "issues", lane) if i["identifier"] in in_review]
            for lane in sorted(lanes)}


def pull_requests(out: dict) -> list[dict]:
    """The pull requests Multica links to the issue, in any state."""
    return [{"url": p["html_url"], "number": p["number"], "branch": p["branch"], "state": p["state"]}
            for p in out.get("pull_requests") or []]


def merge_order(row: dict) -> tuple:
    """By project, then lane by lane with each stack's bottom layer first, then the issues in no lane."""
    lane, stack = row["lane"] or {}, row["stack"] or {}
    return (row["project"] is None, row["project"] or "", not lane, lane.get("name") or "", lane.get("id") or "",
            not stack, stack.get("position", 0), key_order(row["key"]))


def collect_in_review() -> dict:
    """Every In Review issue of the workspace; see the docstring of `team_state.py`."""
    issues = issue_pages("--status", "in_review", "--fields", ISSUE_FIELDS)
    projects = {p["id"]: p["title"] for p in multica("project", "list")}
    releases = {r["id"]: r["version"] for r in multica("release", "list")}
    lanes = {w["id"]: w["name"] for w in multica("worklane", "list")}
    keys = [i["identifier"] for i in issues]
    layers = stacks({i["worklane_id"] for i in issues if i.get("worklane_id")}, set(keys))
    linked, failed = per_issue(keys, "issue", "pull-requests")
    rows = []
    for i in issues:
        key, lane = i["identifier"], i.get("worklane_id")
        stack = layers.get(lane, [])
        rows.append({
            "key": key,
            "title": i["title"],
            "project": projects.get(i.get("project_id"), i.get("project_id")),
            "release": releases.get(i.get("release_id"), i.get("release_id")),
            "lane": {"id": lane, "name": lanes.get(lane)} if lane else None,
            "stack": {"position": stack.index(key) + 1, "layers": len(stack)} if key in stack else None,
            "pull_requests": pull_requests(linked[key]) if key in linked else None,
        })
    out = {"issues": sorted(rows, key=merge_order)}
    if failed:
        out["errors"] = [f"Multica links: {'; '.join(failed)}"]
    return out
