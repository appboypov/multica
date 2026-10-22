"""team_state's `--in-review`: every In Review issue of the workspace with its place in its lane's stack, and each
finished release whose release pull request is open."""

from __future__ import annotations

import urllib.error

from team_state_intake import key_order, per_issue
from team_state_io import StateError, graphql, multica
from team_state_multica import CLOSED, issue_pages, repository_names


ISSUE_FIELDS = "identifier,title,status,status_category,project_id,release_id,worklane_id"
RELEASE_PR_QUERY = """
query($owner: String!, $name: String!, $after: String) {
  repository(owner: $owner, name: $name) {
    defaultBranchRef { name }
    pullRequests(states: OPEN, first: 100, after: $after) {
      pageInfo { hasNextPage endCursor }
      nodes { number url title baseRefName headRefName }
    }
  }
}
"""


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


def finished_releases(releases: list[dict], issues: list[dict]) -> list[dict]:
    """The unshipped releases that hold issues and whose issues are all closed, custom statuses by their category."""
    categories: dict[str, list[str]] = {}
    for i in issues:
        if i.get("release_id"):
            categories.setdefault(i["release_id"], []).append(i["status_category"])
    return [r for r in releases if not r.get("shipped_at") and r["id"] in categories
            and all(c in CLOSED for c in categories[r["id"]])]


def into_default(name: str) -> list[dict]:
    """The repository's open pull requests into its default branch, paged."""
    owner, repo = name.split("/", 1)
    found, after = [], None
    while True:
        data = graphql(RELEASE_PR_QUERY, owner=owner, name=repo, after=after)["repository"]
        default = (data["defaultBranchRef"] or {}).get("name")
        page = data["pullRequests"]
        found += [n for n in page["nodes"] if n["baseRefName"] == default]
        if not page["pageInfo"]["hasNextPage"]:
            return found
        after = page["pageInfo"]["endCursor"]


def release_pull_request(prs: list[dict], version: str) -> dict | None:
    """The release pull request: titled `Release <version>`, followed by its workcycle issue's key."""
    title = f"Release {version}"
    pr = next((p for p in prs if p["title"] == title or p["title"].startswith(f"{title} ")), None)
    return {"url": pr["url"], "number": pr["number"], "branch": pr["headRefName"]} if pr else None


def ready_releases(releases: list[dict], issues: list[dict], errors: list[str]) -> list[dict]:
    """Each finished release whose release pull request is open, with its repository; see `team_state.py`."""
    finished = finished_releases(releases, issues)
    if not finished:
        return []
    try:
        names = repository_names()
    except (OSError, ValueError, KeyError, urllib.error.URLError) as e:
        errors.append(f"Multica repositories: {e}")
        return []
    prs: dict[str, list[dict]] = {}
    rows = []
    for r in sorted(finished, key=lambda r: (names.get(r["repository_id"]) or "", r.get("position", 0))):
        name = names.get(r["repository_id"])
        if not name:
            errors.append(f"Multica repositories: release {r['version']} is in no repository")
            continue
        if name not in prs:
            try:
                prs[name] = into_default(name)
            except StateError as e:
                errors.append(f"GitHub {name}: {e}")
                prs[name] = []
                continue
        if pr := release_pull_request(prs[name], r["version"]):
            rows.append({"repository": name, "version": r["version"], "pull_request": pr})
    return rows


def collect_in_review() -> dict:
    """Every In Review issue of the workspace and each release ready to merge; see the docstring of `team_state.py`."""
    board = issue_pages("--fields", ISSUE_FIELDS)
    issues = [i for i in board if i["status"] == "in_review"]
    projects = {p["id"]: p["title"] for p in multica("project", "list")}
    release_line = multica("release", "list")
    releases = {r["id"]: r["version"] for r in release_line}
    lanes = {w["id"]: w["name"] for w in multica("worklane", "list")}
    keys = [i["identifier"] for i in issues]
    layers = stacks({i["worklane_id"] for i in issues if i.get("worklane_id")}, set(keys))
    linked, failed = per_issue(keys, "issue", "pull-requests")
    errors = [f"Multica links: {'; '.join(failed)}"] if failed else []
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
    out = {"issues": sorted(rows, key=merge_order), "releases": ready_releases(release_line, board, errors)}
    if errors:
        out["errors"] = errors
    return out
