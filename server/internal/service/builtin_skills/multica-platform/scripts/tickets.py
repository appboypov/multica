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
and `blocked_reason` from the `build:` map of its `.openspec.yaml`; an archived change
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
                                 set a story's status in the `build:` map of its change's .openspec.yaml,
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
from functools import cache
from pathlib import Path

sys.dont_write_bytecode = True

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
LEAF_TYPES = ("story", "bug", "enhancement", "refactor", "chore", "spike")
RISKS = ("low", "medium", "high")
CHANGE_RE = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
QUOTED_COMMENT_RE = re.compile(r"""^("(?:[^"\\]|\\.)*"|'(?:[^']|'')*')\s+#.*$""")
TOP_KEY_RE = re.compile(r"^([A-Za-z_][\w-]*)\s*:(.*)$")
FRONTMATTER_RE = re.compile(r"\A---\n(.*?)\n?---(?:\n|\Z)", re.S)
FENCE_RE = re.compile(r"^```[\w-]*\n(.*?)^```", re.M | re.S)
GITHUB_RE = re.compile(r"github\.com[:/]([\w.-]+/[\w.-]+?)(?:\.git)?/?$")
BUILD_FIELDS = ("status", "assignee", "blocked_at", "blocked_reason")
GREEN_LIGHT = "green light: Brian"
HITL = "hitl"
LISTS = ("references", "notes", "covers")


class TicketError(Exception):
    pass


def label_names(issue: dict) -> list[str]:
    return [str(label.get("name") if isinstance(label, dict) else label) for label in issue.get("labels") or []]


def one_label(names: list[str], prefix: str, where: str) -> str:
    """The value of the one label that starts with `prefix`; "" without one."""
    values = [n[len(prefix) :] for n in names if n.startswith(prefix)]
    if len(values) > 1:
        raise TicketError(f"{where}: carries more than one {prefix} label: {', '.join(values)}")
    return values[0] if values else ""


def keys_of(value) -> list[str]:
    """The issue keys of a comma-separated metadata value, in their order, each once."""
    return list(dict.fromkeys(k.strip() for k in str(value or "").split(",") if k.strip()))


# ---------------------------------------------------------------- scalars


def _scalar(value: str):
    if value.startswith("[") and value.endswith("]"):
        inner = value[1:-1].strip()
        return [] if not inner else [_scalar(v.strip()) for v in inner.split(",")]
    if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
        if value[0] == '"':
            try:
                return str(json.loads(value))
            except ValueError:
                pass
        return value[1:-1] if value[0] == '"' else value[1:-1].replace("''", "'")
    if value in ("true", "false"):
        return value == "true"
    if value in ("null", "~"):
        return ""
    if re.fullmatch(r"-?\d+", value):
        return int(value)
    return value


def yaml_value(value: str):
    value = value.strip()
    quoted = QUOTED_COMMENT_RE.match(value)
    if quoted:
        return _scalar(quoted.group(1))
    if not (len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'"):
        value = value.split(" #")[0].strip()
    return _scalar(value)


def quoted(value: str) -> str:
    """A double-quoted YAML scalar that `read_build` reads back exactly."""
    # str.splitlines splits on these characters, so they go in as escapes.
    text = json.dumps(value, ensure_ascii=False)
    return text.translate({c: f"\\u{c:04x}" for c in (0x85, 0x2028, 0x2029)})


def listed(value) -> list:
    """A property value as a list: a list as it is, any other value but empty in a list of one."""
    if isinstance(value, list):
        return value
    return [] if value in (None, "") else [value]


def _flag(value) -> bool:
    return str(value).lower() == "true"


def _one_of(value, allowed: tuple, where: str, field: str):
    """`value` when it is absent (`""`) or one of `allowed`; else the error naming them."""
    if value not in ("", *allowed):
        raise TicketError(f"{where}: {field} {value!r} is not one of {', '.join(allowed)}")
    return value


# ---------------------------------------------------------------- build map and frontmatter


def build_span(lines: list[str]):
    """(header index, end index, key indent) of the top-level `build:` map; None without one."""
    for i, line in enumerate(lines):
        m = TOP_KEY_RE.match(line)
        if not m or m.group(1) != "build":
            continue
        rest = m.group(2).split(" #")[0].strip()
        if rest not in ("", "{}"):
            raise TicketError("the build: map in .openspec.yaml must be a block map")
        end = i + 1
        while end < len(lines) and (not lines[end].strip() or lines[end][:1] in (" ", "\t")):
            end += 1
        while end > i + 1 and not lines[end - 1].strip():
            end -= 1
        body = [b for b in lines[i + 1 : end] if b.strip()]
        indent = min(len(b) - len(b.lstrip()) for b in body) if body else 2
        return i, end, indent
    return None


def read_keys(body: list[str], indent: int) -> dict:
    """The scalar keys at `indent`; nested values and list items are skipped."""
    data = {}
    for line in body:
        stripped = line.strip()
        if len(line) - len(line.lstrip()) != indent or stripped.startswith(("#", "- ")):
            continue
        key, sep, value = stripped.partition(":")
        if sep:
            data[key.strip()] = yaml_value(value)
    return data


def set_keys(body: list[str], indent: int, updates: dict) -> list[str]:
    """`body` with each key of `updates` set at `indent`, or removed when its value is None."""
    body = list(body)
    for key, value in updates.items():
        at = next(
            (
                j
                for j, b in enumerate(body)
                if len(b) - len(b.lstrip()) == indent and re.match(rf"{re.escape(key)}\s*:", b.strip())
            ),
            None,
        )
        new = [] if value is None else [" " * indent + f"{key}: {value}"]
        if at is None:
            body += new
            continue
        stop = at + 1
        while stop < len(body) and body[stop].strip():
            deeper = len(body[stop]) - len(body[stop].lstrip()) > indent
            if not (deeper or body[stop].lstrip().startswith("- ")):
                break
            stop += 1
        body[at:stop] = new
    return body


def read_build(text: str) -> dict:
    """The scalar keys of the `build:` map."""
    lines = text.splitlines()
    span = build_span(lines)
    if span is None:
        return {}
    i, end, indent = span
    return read_keys(lines[i + 1 : end], indent)


def write_build(text: str, updates: dict) -> str:
    """`text` with each key of `updates` set in the `build:` map, or removed when its value is None."""
    lines = text.splitlines()
    span = build_span(lines)
    if span is None:
        while lines and not lines[-1].strip():
            lines.pop()
        lines.append("build:")
        i, end, indent = len(lines) - 1, len(lines), 2
    else:
        i, end, indent = span
        if TOP_KEY_RE.match(lines[i]).group(2).split(" #")[0].strip() == "{}":
            lines[i] = "build:"
    lines[i + 1 : end] = set_keys(lines[i + 1 : end], indent, updates)
    return "\n".join(lines) + "\n"


def read_frontmatter(text: str) -> dict:
    m = FRONTMATTER_RE.match(text)
    return read_keys(m.group(1).splitlines(), 0) if m else {}

# ---------------------------------------------------------------- bodies


def headings(markdown: str) -> list[tuple[str, str | None]]:
    """Each line of a body with its `## ` heading text, None for a line that is no heading or lies in a ``` fence."""
    out, fence = [], ""
    for line in markdown.split("\n"):
        run = re.match(r"`{3,}", line)
        if fence:
            if run and len(run.group()) >= len(fence) and not line.strip().strip("`"):
                fence = ""
            out.append((line, None))
        elif run:
            fence = run.group()
            out.append((line, None))
        else:
            out.append((line, line[3:].strip() if line.startswith("## ") else None))
    return out


def sections(markdown: str) -> dict[str, list[str]]:
    """The lines under each `## ` heading of a body, by the heading in lowercase."""
    out, current = {}, None
    for line, heading in headings(markdown):
        if heading is not None:
            current = out.setdefault(heading.lower(), [])
        elif current is not None:
            current.append(line)
    return out


def text_of(lines: list[str]) -> str:
    return "\n".join(lines).strip()


def items_of(lines: list[str]) -> list[str]:
    return [line[2:].strip() for line in lines if line.startswith("- ")]


def with_section(markdown: str, heading: str, lines: list[str]) -> str:
    """`markdown` with its `## <heading>` section holding `lines`, added at the end when absent."""
    out, skipping, found = [], False, False
    for line, title in headings(markdown):
        if title is not None:
            skipping = title.lower() == heading.lower()
            if skipping:
                out += [f"## {heading}", "", *lines, ""]
                found = True
                continue
        if not skipping:
            out.append(line)
    if not found:
        out += ["", f"## {heading}", "", *lines]
    return "\n".join(out).strip("\n") + "\n"


def spike_build(markdown: str) -> dict:
    """The keys of the `yaml` block under `## Build`; {} without one."""
    m = FENCE_RE.search(text_of(sections(markdown).get("build", [])) + "\n")
    return read_keys(m.group(1).splitlines(), 0) if m else {}


def write_spike_build(markdown: str, updates: dict) -> str:
    """`markdown` with each key of `updates` set in the `## Build` block, or removed when its value is None."""
    m = FENCE_RE.search(text_of(sections(markdown).get("build", [])) + "\n")
    lines = set_keys(m.group(1).splitlines() if m else [], 0, updates)
    return with_section(markdown, "Build", ["```yaml", *lines, "```"])


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
        status = _one_of(build.get("status", ""), STATUSES, f"{name}/.openspec.yaml", "build.status")
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
    """Write status, assignee, and the blocked fields to the `build:` map of the story's change,
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
