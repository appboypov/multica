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
  `wakeup`) and open sub-issues such as Fixes (`child`). A linked workcycle whose release task the pull request also
  links holds nothing: review comes before the workcycle checks out, so the release task alone decides readiness.
  Unread issues, agents, links or wakeups (`evidence`) hold every PR. An unknown verifier also holds every PR. A
  missing claimed issue holds its PR and reports an error.
  Check rollups are informational.
Exit 1 only when GitHub cannot be read; a Multica part that cannot be read carries its error in `errors`.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

sys.dont_write_bytecode = True

from team_state_chats import collect_messages, endeavour_clients
from team_state_intake import collect_intake
from team_state_io import StateError, run
from team_state_multica import collect_multica, registered_repositories
from team_state_repos import collect_repo, local_checkouts, origin_name, repo_home


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
