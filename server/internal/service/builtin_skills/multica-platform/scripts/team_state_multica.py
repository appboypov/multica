"""team_state's Multica part: the worklanes, issues, releases and projects of each repository."""

from __future__ import annotations

import json
import re
import urllib.error
import urllib.request
from pathlib import Path

from team_state_io import StateError, multica


MULTICA_CONFIG = Path.home() / ".multica" / "profiles" / "skuddy" / "config.json"
CLOSED = {"done", "cancelled"}
STARTED = {"in_progress", "blocked", "in_review"}
POINTS = "Points"
PAGE = 100
GITHUB_REMOTE_RE = re.compile(
    r"^(?:git@github\.com:|ssh://git@github\.com/|https?://(?:[^@/]+@)?github\.com/)([\w.-]+/[\w.-]+?)(?:\.git)?/?$",
    re.IGNORECASE,
)


def multica_api(path: str) -> dict:
    """One `GET` of Multica's API with the profile's token, for what the CLI does not read."""
    config = json.loads(MULTICA_CONFIG.read_text(encoding="utf-8"))
    request = urllib.request.Request(
        config["server_url"].rstrip("/") + path,
        headers={"Authorization": f"Bearer {config['token']}", "X-Workspace-ID": config["workspace_id"]},
    )
    with urllib.request.urlopen(request, timeout=20) as response:
        return json.load(response)


def organisations() -> dict:
    """The workspace's organisations by id; the CLI has none, so `GET /api/organisations`."""
    return {o["id"]: {"name": o["name"], "key": o["key"]} for o in multica_api("/api/organisations")["organisations"]}


def repository_names() -> dict[str, str]:
    """The GitHub name (`owner/name`) of each project repository by id; the CLI has none, so `GET /api/repositories`."""
    return {r["id"]: r["full_name"] for r in multica_api("/api/repositories")["repositories"]}


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
