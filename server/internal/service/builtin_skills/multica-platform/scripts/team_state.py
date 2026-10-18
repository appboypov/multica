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
  `Client wait` issue of its release that comes before it in its lane.
- Repositories, from git and GitHub: per repository its checkouts, the default branch and that branch's `checks`;
  `changes`, the OpenSpec changes of every checkout and worktree with their `x-build:` map; `prs`, the open pull
  requests with their checks, comments, reviews, review threads, the changes their files touch and the Multica
  issues their title or branch names.
- Client messages: the latest WhatsApp messages, through Beeper Desktop, of every chat in the agent file of each
  client the endeavour's `AGENTS.md` names (`das-client-agents/clients/<client>/AGENTS.md`), plus each --client.

A source that cannot be read carries its `error` in its place, and everything else is still collected. Output is
one JSON object on stdout. Exit 1 only when Multica cannot be read.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

sys.dont_write_bytecode = True

from team_state_repositories import GITHUB_REMOTE_RE, collect_repo, local_checkouts, origin_name, repo_home
from team_state_run import StateError, run

MULTICA = ["multica", "--profile", "skuddy"]
MULTICA_CONFIG = Path.home() / ".multica" / "profiles" / "skuddy" / "config.json"
CLOSED = {"done", "cancelled"}
STARTED = {"in_progress", "blocked", "in_review"}
CLIENT_WAIT = "Client wait"
POINTS = "Points"
PAGE = 100
CLIENTS_ROOT = Path.home() / ".omp" / "agent" / "skills" / "das-client-agents" / "clients"
BEEPER = "http://localhost:23373"
MESSAGES_PER_CHAT = 20
CLIENT_FILE_RE = re.compile(r"das-client-agents/clients/([\w.-]+)/AGENTS\.md")
CHAT_ID_RE = re.compile(r"`(![^`\s]+:[^`\s]+)`")


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


def project_issues(project: str) -> list[dict]:
    """Every issue of the project, with its labels, paged."""
    issues, offset = [], 0
    while True:
        page = multica("issue", "list", "--project", project, "--limit", str(PAGE), "--offset", str(offset))
        issues += page["issues"]
        if not page.get("has_more") or not page["issues"]:
            return issues
        offset += len(page["issues"])


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


def waits_on(row: dict, before: list[dict], rows: dict[str, dict], verifier: str | None) -> list[dict]:
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
    if CLIENT_WAIT not in row["labels"]:
        out += [
            {"kind": "client_wait", "key": other["key"], "title": other["title"], "due_date": other["due_date"]}
            for other in before
            if CLIENT_WAIT in other["labels"] and other["release"] == row["release"] and other["status"] not in CLOSED
        ]
    return out


def lane_head(rows: list[dict], known: dict[str, dict], verifier: str | None) -> dict:
    """The lane's active issue, the first that has started and is not finished, and its next, the first that has not
    started, each with what it waits on."""
    head = {"active": None, "next": None}
    for i, row in enumerate(rows):
        if finished(row, verifier):
            continue
        slot = "active" if row["status"] in STARTED else "next"
        if head[slot] is None:
            head[slot] = {**row, "waits_on": waits_on(row, rows[:i], known, verifier)}
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
                **lane_head(mine, rows, verifier),
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
        path = Path(item).expanduser()
        if path.is_dir():
            home = repo_home(path.resolve())
            name = origin_name(home)
            if not name:
                raise StateError(f"{home} has no GitHub origin")
            if home not in repos.setdefault(name, []):
                repos[name].append(home)
        elif re.fullmatch(r"[\w.-]+/[\w.-]+", item):
            repos.setdefault(item.lower(), [])
        else:
            raise StateError(f"no folder and no owner/name: {item}")
    return repos


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


def main() -> int:
    parser = argparse.ArgumentParser(description="Collect the open work of one endeavour.")
    parser.add_argument("endeavour", help="the endeavour's folder, such as ~/Repos/Clients/rebl")
    parser.add_argument("--repo", action="append", default=[], help="a repository beside the endeavour's own: owner/name or a checkout")
    parser.add_argument("--verifier", required=True, help="the name of the Multica agent in the verifier's role, which holds an issue once its pull request is open")
    parser.add_argument("--lane", help="the worklane to run on, by name or id: only its lane is reported")
    parser.add_argument("--client", action="append", default=[], help="a client whose chats to read beside those the endeavour names")
    args = parser.parse_args()
    try:
        endeavour = Path(args.endeavour).expanduser().resolve()
        if not endeavour.is_dir():
            raise StateError(f"not a folder: {endeavour}")
        print(json.dumps(collect(endeavour, args.repo, args.verifier, args.lane, args.client), ensure_ascii=False, default=str))
        return 0
    except (StateError, OSError, ValueError, KeyError) as e:
        print(json.dumps({"error": str(e)}), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
