"""team_state's `--intake`: every open pull request of one repository, ranked into ready and blocked."""

from __future__ import annotations

import json
import re
from collections.abc import Sequence
from concurrent.futures import ThreadPoolExecutor

from team_state_io import StateError, graphql, multica, run_json
from team_state_multica import CLOSED, finished, issue_pages, issue_row
from team_state_repos import ISSUE_KEY_RE, collect_github, issue_keys


ISSUE_READERS = 8  # per-issue `multica` reads at once
UNRESOLVED_EDGES = {"ancestry_unproven", "ambiguous", "cycle", "lower_closed", "unknown_target"}
ISSUE_FIELDS = "id,identifier,title,type,status,assignee_type,assignee_id,parent_issue_id,stage,metadata,position,project_id,release_id,due_date"
CLOSING_KEY_RE = re.compile(r"(?i)\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)[:\s]+([a-z][a-z0-9]{0,9})-(\d+)\b")
STACK_NOTE_RE = re.compile(r"(?i)\bstacked\s+on\b([^\n]*)")
NOTE_END_RE = re.compile(r"(?i)\bwhich\b|;|\.(?:\s|$)")
PR_REF_RE = re.compile(r"(?<![\w/.#-])#(\d+)\b")
OTHER_PR_REF_RE = re.compile(r"\b([\w.-]+/[\w.-]+)#(\d+)\b")
PR_URL_RE = re.compile(r"github\.com/([\w.-]+/[\w.-]+)/pull/(\d+)", re.IGNORECASE)
CODE_RE = re.compile(r"`([^`\s]+)`")
SHA_RE = re.compile(r"[0-9a-f]{7,40}")


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


def workcycle_links(keys: list[str], issues: dict[str, dict]) -> set[str]:
    """The linked workcycles whose release task the pull request also links. Review comes before the workcycle checks
    out, so such a workcycle is context and its release task alone decides readiness."""
    return {k for k in keys if k in issues and issues[k]["type"] == "workcycle"
            and any(issues.get(o, {}).get("parent") == k for o in keys)}


def blockers(pr: dict, keys: list[str], issues: dict[str, dict], verifier: str | None,
             waits_of: dict[str, list[dict]], edges: list[dict], children: dict[str, list[dict]],
             missing: Sequence[str] = ()) -> list[dict]:
    """Missing evidence, stack relationships, assignment and dependencies that hold this pull request."""
    reasons = [{"kind": "evidence", "missing": list(missing)}] if missing else []
    reasons += [{"kind": "stack_edge", "pr": e["upper"], "lower": e["lower"], "status": e["status"]}
                for e in edges if e["upper"] == pr["number"] and e["status"] in UNRESOLVED_EDGES]
    reasons += [{"kind": "stack", "pr": e["lower"]}
                for e in edges if e["upper"] == pr["number"] and e["status"] == "verified"]
    context = workcycle_links(keys, issues)
    for key in (k for k in keys if k not in context):
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
