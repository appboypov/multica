#!/usr/bin/env python3
# /// script
# requires-python = ">=3.11"
# ///
"""tickets - read a ticket tree from Multica and answer what is next.

The tree is the epics of one Multica project and their stories, read and written through the
`multica` CLI as `references/madspec-planning.md` says. An epic is an issue of type epic; its stories are
its sub-issues. Epics and stories are in build order: the project's board order (`position`). A
story's fields are its issue's:

  ref, id           its key, such as WORK-12, and its UUID
  title             its title
  type              story, bug, enhancement, refactor, chore or spike; none is story
  green_lights      `brian` when it carries the label `green light: Brian`
  risk, hitl        its label `risk:<low|medium|high>`, and whether it carries the label `hitl`
  estimate          its property Points
  change            its metadata `change`: the name of its OpenSpec change
  after             its metadata `blocked_by`: the comma-separated keys of the stories it waits on,
                    or of an epic for that whole epic
  refine            status backlog: it needs refining before it starts
  dropped           status cancelled
  description, verify, unknown, references, notes, covers
                    its description's sections `## Description`, `## Verify`, `## Unknown`,
                    `## References`, `## Notes` and `## Covers`, in that order; the last three
                    are lists of `- ` lines. A `## ` line inside a ``` fence is no heading

An epic's `after` is its metadata `blocked_by`: the keys of the epics it waits on; what it needs of
each is the line `- <epic title>: <needs>` under `## After` in its description. A story needing
refining is refined once its change holds spec deltas.

One story is one OpenSpec change. A story with no change is `planned`. The OpenSpec root is
`specs/openspec/` when the project holds it, else `openspec/` (docs/contract.md). A change in
`<openspec-home>/openspec/changes/<change>/` gives the story its `status`, `assignee`, `blocked_at`,
and `blocked_reason` from the `x-build:` map of its `.openspec.yaml`; an archived change
(`<openspec-home>/openspec/changes/archive/<YYYY-MM-DD>-<change>/`) is done once its archive folder is
on the default branch (`origin/HEAD`, else `main`, else `master`): merged. A change is archived on its
pull request's branch before the merge, so an archive the default branch lacks is still in-review.
Outside a git repository, or without a default branch, an archived change is done. A spike produces
no change: its result is its issue's description, and the `yaml` block under `## Build` there holds
its `status`, `assignee`, `blocked_at`, `blocked_reason`, and `refined`; a spike without one is planned.

`status` is draft, ready-for-dev, in-progress, in-review, or built from the builds (built is their
last: the build finished and nobody has called it done), blocked from build-auto, done from the
user or an orchestrator through mark or from the archive, or dropped; absent means no build has
started. A story's `state` is `planned` with no change, else derived from `status`: absent, draft,
ready-for-dev -> backlog; in-progress, blocked -> in-progress; in-review, built -> review; done;
dropped. An epic is done when every story in it is done or dropped. A dropped prerequisite still
blocks.

  next   [--lights]              stories whose prerequisites are done or in review, grouped by state,
                                 in build order: a story builds on the branch of one prerequisite's
                                 open pull request, so at most one of them may still be in review; a
                                 whole-epic prerequisite waits for that epic to be done. With
                                 --lights, a story ready to start without Brian's green light moves
                                 to `held`
  status                         every story in build order, what it blocks, counts by state, longest
                                 chain; with --project also every epic
  find   <ref>                   the one story a reference names, with its description's sections, its
                                 `change`, the absolute path `change_dir` (null until pulled), and
                                 `spike_file`, the key of the issue holding a spike's result (null for
                                 others)
  pull   <ref> [--name <change>] run `openspec new change <change>` in the folder holding the OpenSpec root
                                 and set the story's metadata `change` to it; the name defaults to the
                                 title's kebab slug
  mark   <ref> <status> [--assignee <who>] [--blocked <reason>]
                                 set a story's status in the `x-build:` map of its change's .openspec.yaml,
                                 or a spike's under `## Build` in its issue's description; --blocked sets
                                 blocked_at and blocked_reason, else both are cleared

Every command takes a scope: `--epic <issue key>`, one epic, whose stories can name every epic of
its project, or `--project <project id>`, every epic of that project. `<ref>` is a story's key, its
UUID, a change name, or words from the title that match one story. `epic`, `after` and `blocks`
name stories and epics by their keys.

The project root is `--project-root`, else the first folder at or above the working directory that
holds `.git`. `--project-root` repeats for a tree whose changes live in several repositories. Each
pulled story reads from the first that holds its change: the project roots in order, then every
other worktree of their repositories (`git worktree list`). A story in a worklane reads its change
only from the folders of that worklane's repository, matched by their `origin`. `pull` creates the
change in the first project root, which must be a clone of the story's worklane's repository.

Output is one JSON object on stdout. Exit 0 on success, 1 on a malformed tree, a failed `multica`,
`openspec` or `git` call, 2 on a usage error.
"""

import argparse
import json
import re
import shutil
import subprocess
import sys
from datetime import date
from pathlib import Path

sys.dont_write_bytecode = True

from ticket_sources import (
    STATUSES,
    archived_dir,
    changes_dir,
    changes_dirs,
    github_repo,
    lane_repositories,
    multica,
    openspec_home,
    project_roots,
    repository_id,
)
from ticket_text import TicketError, quoted, read_build, spike_build, write_build, write_spike_build
from ticket_tree import (
    CHANGE_RE,
    classify,
    epic_done,
    in_scope,
    load_tree,
    longest_remaining_chain,
    public,
    ref,
    unpinned_after,
)

BUILD_FIELDS = ("status", "assignee", "blocked_at", "blocked_reason")


# ---------------------------------------------------------------- commands


def _tree(args) -> dict:
    return load_tree(args, changes_dirs(project_roots(args)))


def cmd_next(args) -> dict:
    tree = _tree(args)
    groups = {k: [public(t, tree) for t in v] for k, v in classify(tree).items()}
    if args.lights:
        ready = groups["ready_to_start"]
        groups.update(
            ready_to_start=[r for r in ready if "brian" in r["green_lights"]],
            held=[r for r in ready if "brian" not in r["green_lights"]],
        )
    return {"scope": tree["label"], **groups, "unpinned_after": unpinned_after(tree)}


def cmd_status(args) -> dict:
    tree = _tree(args)
    tickets = in_scope(tree)
    counts = {}
    for t in tickets:
        counts[t["state"]] = counts.get(t["state"], 0) + 1
    blocks = {}
    for t in tree["tickets"]:
        for b in t["after"]:
            blocks.setdefault(b, []).append(t["key"])
    out = {
        "scope": tree["label"],
        "tickets": [public(t, tree, blocks) for t in tickets],
        "counts": {"total": len(tickets), **counts},
        "longest_remaining_chain": longest_remaining_chain(tree),
        "unpinned_after": unpinned_after(tree),
    }
    if tree["scope"] is None:
        out["epics"] = [
            {
                "ref": e["ref"],
                "title": e["title"],
                "status": "done" if epic_done(tree, epic) else "",
                "after": [ref(needed, tree) for needed, _ in e["after"]],
                "blocks": [ref(b, tree) for b in blocks.get(epic, [])],
            }
            for epic, e in tree["epics"].items()
        ]
    return out


def title_slug(title: str) -> str:
    return re.sub(r"[^a-z0-9]+", "-", title.lower()).strip("-")[:60].rstrip("-") or "untitled"


def resolve_ticket(tree: dict, text: str) -> dict:
    """The one row a reference names; see `<ref>` above."""
    tickets = tree["tickets"]
    ref_text = text.strip()
    if not ref_text:
        raise TicketError("the story reference is empty")
    low = ref_text.lower()
    hits = [t for t in tickets if t["id"] == ref_text]
    if not hits:
        hits = [t for t in tickets if low == t["ref"].lower()]
    if not hits:
        hits = [t for t in tickets if t["change"] and low == t["change"]]
    if not hits:
        hits = [t for t in in_scope(tree) if low in t["title"].lower()]
    if not hits:
        raise TicketError(f"no story matches {ref_text!r}")
    if len(hits) > 1:
        names = ", ".join(t["ref"] for t in hits)
        raise TicketError(f"{ref_text!r} matches more than one story: {names}")
    return hits[0]


def cmd_find(args) -> dict:
    tree = _tree(args)
    t = resolve_ticket(tree, args.ref)
    return {
        **public(t, tree),
        "description": t["description"],
        "verify": t["verify"],
        "references": t["references"],
        "notes": t["notes"],
        "unknown": t["unknown"],
        "change_dir": t["change_dir"],
        "spike_file": t["spike_file"],
    }


def cmd_pull(args) -> dict:
    roots = project_roots(args)
    if not roots:
        raise TicketError("no project root found: no .git at or above the working directory; pass --project-root")
    root = roots[0]
    homes = changes_dirs(roots)
    tree = load_tree(args, homes)
    t = resolve_ticket(tree, args.ref)
    if t["type"] == "spike":
        raise TicketError(f"{t['ref']} is a spike: a spike produces no change, its result is its issue's description")
    if t["change"]:
        raise TicketError(f"{t['ref']} is already pulled: {t['change']}")
    name = args.name or title_slug(t["title"])
    if not CHANGE_RE.match(name):
        raise TicketError(f"change name {name!r} is not kebab-case")
    if any((c / name).exists() or archived_dir(c, name) for c in homes):
        raise TicketError(f"change {name} exists already; pass --name")
    lane = t["worklane"]
    if lane and lane_repositories(tree["project"]).get(lane) != repository_id(github_repo(root)):
        raise TicketError(f"{t['ref']} is in a worklane of another repository than {root}; "
                          "pass that repository's clone as the first --project-root")
    try:
        r = subprocess.run(
            ["openspec", "new", "change", name], cwd=openspec_home(root), capture_output=True, encoding="utf-8", check=False
        )
    except FileNotFoundError:
        raise TicketError("the openspec CLI is not on PATH") from None
    if r.returncode != 0:
        raise TicketError(f"openspec new change {name} failed: {(r.stderr or r.stdout).strip()}")
    try:
        multica("issue", "metadata", "set", t["ref"], "--key", "change", "--value", name, "--type", "string")
    except TicketError:
        # A change no issue names would be an orphan: the folder goes with the failed link.
        shutil.rmtree(changes_dir(root) / name, ignore_errors=True)
        raise
    return {"change": name, "change_dir": str(changes_dir(root) / name), "refine": t["refine"]}


def blocked_updates(args) -> dict:
    if args.blocked is not None:
        return {"blocked_at": quoted(date.today().isoformat()), "blocked_reason": quoted(args.blocked)}
    return {"blocked_at": None, "blocked_reason": None}


def mark_spike(t: dict, args) -> dict:
    """Write status, assignee, and the blocked fields to the `## Build` block of the spike's issue description."""
    updates = {"status": args.status}
    if args.assignee is not None:
        updates["assignee"] = quoted(args.assignee)
    updates.update(blocked_updates(args))
    body = write_spike_build(t["markdown"], updates)
    multica("issue", "update", t["ref"], "--description-stdin", "--no-start", stdin=body)
    fm = spike_build(body)
    return {
        "change": "",
        "change_dir": None,
        "spike_file": t["ref"],
        **{k: str(fm.get(k, "") or "") for k in BUILD_FIELDS},
    }


def cmd_mark(args) -> dict:
    """Write status, assignee, and the blocked fields to the `x-build:` map of the story's change,
    or to a spike's issue description."""
    tree = _tree(args)
    t = resolve_ticket(tree, args.ref)
    if t["type"] == "spike":
        return mark_spike(t, args)
    if not t["change"]:
        raise TicketError(f"{t['ref']} has no change yet: pull it first")
    change = Path(t["change_dir"])
    if change.parent.name == "archive":
        raise TicketError(f"change {t['change']} is archived, so it is done")
    path = change / ".openspec.yaml"
    text = path.read_text(encoding="utf-8") if path.is_file() else ""
    current = read_build(text)
    updates = {"status": args.status}
    if current.get("ticket") in (None, ""):
        updates["ticket"] = quoted(t["ref"])
    if args.assignee is not None:
        updates["assignee"] = quoted(args.assignee)
    updates.update(blocked_updates(args))
    text = write_build(text, updates)
    data = text.encode("utf-8")  # before the file is opened, so a failure leaves it as it was
    path.write_bytes(data)
    build = read_build(text)
    return {
        "change": t["change"],
        "change_dir": str(change),
        "spike_file": None,
        **{k: str(build.get(k, "") or "") for k in BUILD_FIELDS},
    }


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description="Read a ticket tree from Multica and answer what is next.")
    root.add_argument(
        "--project-root",
        action="append",
        help="a code repository holding an OpenSpec root; repeat for a tree whose changes live in several repositories (pull creates the change in the first); every other worktree of their repositories is read too; default: the nearest folder holding .git at or above the working directory",
    )
    scope = argparse.ArgumentParser(add_help=False)
    group = scope.add_mutually_exclusive_group(required=True)
    group.add_argument("--epic", metavar="KEY", help="the epic issue; its stories can name every epic of its project")
    group.add_argument("--project", metavar="ID", help="the Multica project id: all its epics")
    sub = root.add_subparsers(dest="command", required=True)
    p = sub.add_parser("next", parents=[scope], help="stories whose prerequisites let their build start, by state")
    p.add_argument("--lights", action="store_true", help="hold stories ready to start without Brian's green light")
    p.set_defaults(func=cmd_next)
    p = sub.add_parser("status", parents=[scope], help="every story resolved")
    p.set_defaults(func=cmd_status)
    p = sub.add_parser("find", parents=[scope], help="the one story a reference names")
    p.add_argument("ref")
    p.set_defaults(func=cmd_find)
    p = sub.add_parser("pull", parents=[scope], help="create a story's OpenSpec change and set it on the story")
    p.add_argument("ref")
    p.add_argument("--name", help="the change name; default: the title's kebab slug")
    p.set_defaults(func=cmd_pull)
    p = sub.add_parser("mark", parents=[scope], help="set a story's status in its change's build map")
    p.add_argument("ref")
    p.add_argument("status", choices=STATUSES)
    p.add_argument("--assignee")
    p.add_argument(
        "--blocked", metavar="REASON", help="set blocked_at to today and blocked_reason; else both are cleared"
    )
    p.set_defaults(func=cmd_mark)
    return root


def main(argv: list[str] | None = None) -> int:
    args = parser().parse_args(argv)
    try:
        print(json.dumps(args.func(args), ensure_ascii=False, default=str))
        return 0
    except (TicketError, OSError, ValueError) as e:
        print(json.dumps({"error": str(e)}, ensure_ascii=False), file=sys.stderr)
        return 1


if __name__ == "__main__":
    if sys.platform == "win32":
        # Piped output on Windows defaults to a legacy code page, not UTF-8.
        sys.stdout.reconfigure(encoding="utf-8")
        sys.stderr.reconfigure(encoding="utf-8")
    raise SystemExit(main())
