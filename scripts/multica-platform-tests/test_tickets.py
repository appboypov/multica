import contextlib
import io
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from datetime import date
from pathlib import Path
from types import SimpleNamespace
from unittest import mock

SCRIPTS = Path(__file__).resolve().parents[2] / "server/internal/service/builtin_skills/multica-platform/scripts"
sys.path.insert(0, str(SCRIPTS))
import ticket_sources  # noqa: E402
import tickets  # noqa: E402

PROJECT = "project-shop"

FAKE_OPENSPEC = """#!/usr/bin/env python3
import sys
from pathlib import Path

args = sys.argv[1:]
if len(args) != 3 or args[:2] != ["new", "change"] or not Path("openspec").is_dir():
    print("fake openspec: unexpected call " + " ".join(args), file=sys.stderr)
    raise SystemExit(1)
if args[2] == "refused":
    print("Change name refused", file=sys.stderr)
    raise SystemExit(1)
path = Path("openspec/changes") / args[2]
path.mkdir(parents=True)
(path / ".openspec.yaml").write_text("schema: intent-driven\\ncreated: 2026-09-26\\n")
"""

# The multica CLI over a JSON file: the issues, worklanes and repositories a test sets up.
FAKE_MULTICA = """#!/usr/bin/env python3
import json, os, sys

path = os.environ["FAKE_MULTICA_STATE"]
state = json.load(open(path, encoding="utf-8"))
args = sys.argv[1:]
if args[:2] != ["--profile", "skuddy"] or args[-2:] != ["--output", "json"]:
    print("fake multica: unexpected call " + " ".join(args), file=sys.stderr)
    raise SystemExit(2)
args = args[2:-2]
state["calls"].append(args)


def opt(name):
    return args[args.index(name) + 1] if name in args else None


def issue(ref):
    for i in state["issues"]:
        if ref in (i["id"], i["identifier"]) or ref.lower() == i["identifier"].lower():
            return i
    print(f"issue {ref} not found", file=sys.stderr)
    raise SystemExit(1)


def done(value):
    json.dump(state, open(path, "w", encoding="utf-8"))
    print(json.dumps(value))
    raise SystemExit(0)


if args[:2] == ["issue", "get"]:
    done(issue(args[2]))
if args[:2] == ["issue", "list"]:
    mine = sorted((i for i in state["issues"] if i["project_id"] == opt("--project")), key=lambda i: i["position"])
    offset, limit = int(opt("--offset") or 0), int(opt("--limit") or 50)
    page = mine[offset : offset + limit]
    done({"issues": page, "has_more": offset + len(page) < len(mine), "total": len(mine)})
if args[:2] == ["worklane", "list"]:
    if opt("--repo"):
        repo = state["repos"].get(opt("--repo").lower())
        done([lane for lane in state["lanes"] if lane["repository_id"] == repo])
    done(state["lanes"])
if args[:3] == ["issue", "metadata", "set"]:
    if state.get("refuse"):
        print(state["refuse"], file=sys.stderr)
        raise SystemExit(1)
    issue(args[3])["metadata"][opt("--key")] = opt("--value")
    done({"key": opt("--key"), "value": opt("--value")})
if args[:2] == ["issue", "update"] and "--description-stdin" in args:
    target = issue(args[2])
    target["description"] = sys.stdin.read()
    done(target)
print("fake multica: unknown command " + " ".join(args), file=sys.stderr)
raise SystemExit(1)
"""


def body(description="", verify="", unknown="", references=(), notes=(), covers=()):
    """An issue description with its sections, in the order tickets.py reads them."""
    parts = []
    for heading, text in (("Description", description), ("Verify", verify), ("Unknown", unknown)):
        if text:
            parts.append(f"## {heading}\n\n{text}")
    for heading, items in (("References", references), ("Notes", notes), ("Covers", covers)):
        if items:
            parts.append(f"## {heading}\n\n" + "\n".join(f"- {item}" for item in items))
    return "\n\n".join(parts)


class TreeCase(unittest.TestCase):
    """A temp repository with `.git/` and `openspec/`, and a Multica project with its epic Cart."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name).resolve()
        (self.root / ".git").mkdir()
        (self.root / "openspec" / "changes" / "archive").mkdir(parents=True)
        bin_dir = self.root.parent / f"{self.root.name}-bin"
        bin_dir.mkdir()
        self.addCleanup(shutil.rmtree, bin_dir, True)
        for name, script in (("openspec", FAKE_OPENSPEC), ("multica", FAKE_MULTICA)):
            fake = bin_dir / name
            fake.write_text(script.replace("#!/usr/bin/env python3", f"#!{sys.executable}"), encoding="utf-8")
            fake.chmod(0o755)
        self.state_file = bin_dir / "multica.json"
        env = {"PATH": f"{bin_dir}{os.pathsep}{os.environ['PATH']}", "FAKE_MULTICA_STATE": str(self.state_file)}
        patcher = mock.patch.dict(os.environ, env)
        patcher.start()
        self.addCleanup(patcher.stop)
        self.state = {"issues": [], "lanes": [], "repos": {}, "calls": []}
        self.n = {}  # issue id: the number a test calls the story by
        self.epic = self.add_epic("Cart")

    def issue(self, number: int, title: str, kind: str, **fields) -> str:
        issue = {
            "id": f"id-{number}",
            "identifier": f"WORK-{number}",
            "title": title,
            "type": kind,
            "status": "todo",
            "parent_issue_id": None,
            "project_id": PROJECT,
            "position": len(self.state["issues"]),
            "labels": [],
            "metadata": {},
            "properties": [],
            "description": "",
            "worklane_id": None,
        }
        issue.update(fields)
        self.state["issues"].append(issue)
        return issue["identifier"]

    def get(self, ref: str) -> dict:
        return next(i for i in self.state["issues"] if ref in (i["id"], i["identifier"]))

    def add_epic(self, title, after=(), needs="", **fields):
        number = 100 + sum(i["type"] == "epic" for i in self.state["issues"])
        description = "## After\n\n" + "\n".join(f"- {self.get(a)['title']}: {needs}" for a in after) if after else ""
        metadata = {"blocked_by": ",".join(after)} if after else {}
        return self.issue(number, title, "epic", description=description, metadata=metadata, **fields)

    def story(self, n, title, epic=None, kind="story", after=(), change=None,
              status="todo", description="", labels=(), points=None, lane=None, **fields):
        """Story `n` of an epic (Cart by default); `after` holds story numbers or issue keys."""
        names = list(labels)
        metadata = {}
        if after:
            metadata["blocked_by"] = ",".join(self.key_of(a) for a in after)
        if change:
            metadata["change"] = change
        ref = self.issue(
            n,
            title,
            kind,
            parent_issue_id=self.get(epic or self.epic)["id"],
            status=status,
            description=description,
            labels=[{"name": name} for name in names],
            metadata=metadata,
            properties=[{"name": "Points", "value": points}] if points is not None else [],
            worklane_id=lane,
            **fields,
        )
        self.n[self.get(ref)["id"]] = n
        return ref

    def key_of(self, ref) -> str:
        return f"WORK-{ref}" if isinstance(ref, int) else ref

    def four(self, **first):
        """Scaffold; UI shell after 1; a spike after 1; Codes after 2 and 3."""
        self.story(1, "Scaffold", description=body(covers=["R1"]), **first)
        self.story(2, "UI shell", after=[1])
        self.story(3, "Tax engine?", kind="spike", after=[1], labels=["hitl"], description=body(covers=["R4"]))
        self.story(4, "Codes", after=[2, 3])

    def change(self, name, status=None, specs=False, archived=False, extra="", root=None):
        base = (root or self.root) / "openspec" / "changes"
        path = base / "archive" / f"2026-09-20-{name}" if archived else base / name
        path.mkdir(parents=True)
        text = "schema: intent-driven\ncreated: 2026-09-20\n"
        if status is not None:
            text += f"build:\n  status: {status}\n{extra}"
        (path / ".openspec.yaml").write_text(text, encoding="utf-8")
        if specs:
            (path / "specs" / "cart").mkdir(parents=True)
            (path / "specs" / "cart" / "spec.md").write_text("## ADDED Requirements\n", encoding="utf-8")
        return path

    def run_cli(self, *args, cwd=None):
        self.state_file.write_text(json.dumps(self.state), encoding="utf-8")
        out, err = io.StringIO(), io.StringIO()
        for cached in (ticket_sources.merged_archives, ticket_sources.github_repo, ticket_sources.lane_repositories,
                       ticket_sources.repository_id):
            cached.cache_clear()  # each command is a run of its own
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err), contextlib.chdir(cwd or self.root):
            code = tickets.main([str(a) for a in args])
        self.state = json.loads(self.state_file.read_text(encoding="utf-8"))
        return SimpleNamespace(returncode=code, stdout=out.getvalue(), stderr=err.getvalue())

    def scoped(self, command, *rest, scope=None):
        return self.run_cli("--project-root", self.root, command, *(scope or ["--epic", self.epic]), *rest)

    def ok(self, r):
        self.assertEqual(r.returncode, 0, r.stderr)
        return json.loads(r.stdout)

    def fails(self, r, message):
        self.assertEqual(r.returncode, 1, r.stdout)
        self.assertIn(message, json.loads(r.stderr)["error"])

    def next(self, scope=None):
        return self.ok(self.scoped("next", scope=scope))

    def status(self, scope=None):
        return self.ok(self.scoped("status", scope=scope))

    def find(self, ref, scope=None):
        return self.ok(self.scoped("find", ref, scope=scope))

    def ids(self, rows):
        return [self.n[r["id"]] for r in rows]

    def numbers(self, refs):
        return [int(r.removeprefix("WORK-")) for r in refs]

    def on_github(self, repo="acme/shop", repository="repo-shop"):
        """Make the root a git repository whose origin is `repo`, registered in Multica as `repository`."""
        shutil.rmtree(self.root / ".git")
        subprocess.run(["git", "-C", str(self.root), "init", "-q"], check=True)
        subprocess.run(["git", "-C", str(self.root), "remote", "add", "origin", f"git@github.com:{repo}.git"], check=True)
        return self.lane(repo, repository)

    def lane(self, repo, repository, name="main"):
        self.state["repos"][repo.lower()] = repository
        lane = f"lane-{repository}-{name}"
        self.state["lanes"].append({"id": lane, "repository_id": repository, "name": name})
        return lane


class TreeTests(TreeCase):
    def test_planned_stories_surface_when_unblocked_in_build_order(self):
        self.four()
        out = self.next()
        self.assertEqual(self.ids(out["ready_to_start"]), [1])
        self.assertEqual(self.ids(out["blocked"]), [2, 3, 4])
        self.assertEqual(out["ready_to_start"][0]["state"], "planned")
        self.assertEqual(out["ready_to_start"][0]["change"], "")
        status = self.status()
        self.assertEqual(status["counts"], {"total": 4, "planned": 4})
        self.assertEqual(self.numbers(status["tickets"][0]["blocks"]), [2, 3])
        self.assertEqual(self.numbers(status["longest_remaining_chain"]), [1, 2, 4])
        self.assertEqual((status["tickets"][0]["covers"], status["tickets"][2]["hitl"]), (["R1"], True))

    def test_build_order_is_the_board_order(self):
        self.story(1, "Second on the board", position=20)
        self.story(2, "First on the board", position=10)
        self.story(3, "Last on the board", position=30)
        self.assertEqual(self.ids(self.next()["ready_to_start"]), [2, 1, 3])
        self.assertEqual(self.ids(self.status()["tickets"]), [2, 1, 3])

    def test_issues_page_through_the_whole_project(self):
        for n in range(1, 131):
            self.story(n, f"Story {n}")
        self.assertEqual(self.status()["counts"]["total"], 130)
        lists = [c for c in self.state["calls"] if c[:2] == ["issue", "list"]]
        self.assertEqual([c[c.index("--offset") + 1] for c in lists], ["0", "100"])

    def test_labels_properties_and_metadata_give_the_fields(self):
        self.story(1, "Scaffold", labels=["green light: Brian", "risk:high"], points=5)
        self.story(2, "UI shell")
        rows = self.status()["tickets"]
        self.assertEqual(
            [(r["green_lights"], r["risk"], r["estimate"]) for r in rows],
            [(["brian"], "high", 5), ([], "", "")],
        )

    def test_status_comes_from_the_build_map_and_the_archive(self):
        self.four(change="scaffold")
        self.change("scaffold", "in-progress")
        out = self.next()
        self.assertEqual(self.ids(out["in_progress"]), [1])
        self.assertEqual(out["in_progress"][0]["status"], "in-progress")
        self.assertEqual(self.ids(out["blocked"]), [2, 3, 4])
        shutil.rmtree(self.root / "openspec" / "changes" / "scaffold")
        self.change("scaffold", archived=True)
        out = self.next()
        self.assertEqual(self.ids(out["ready_to_start"]), [2, 3])
        self.assertEqual(self.status()["tickets"][0]["status"], "done")

    def test_a_change_with_no_build_status_is_backlog(self):
        self.four(change="scaffold")
        self.change("scaffold")
        rows = self.status()["tickets"]
        self.assertEqual((rows[0]["status"], rows[0]["state"]), ("", "backlog"))
        self.assertEqual(self.ids(self.next()["ready_to_start"]), [1])

    def test_a_prerequisite_in_review_lets_what_waits_on_it_start_on_its_branch(self):
        self.four(change="scaffold")
        for status in ("in-review", "built"):
            shutil.rmtree(self.root / "openspec" / "changes" / "scaffold", ignore_errors=True)
            self.change("scaffold", status)
            out = self.next()
            self.assertEqual(self.ids(out["ready_to_start"]), [2, 3], status)
            self.assertEqual(self.ids(out["blocked"]), [4], status)
            self.assertEqual(self.ids(out["in_progress"]), [1], status)

    def test_a_story_builds_on_at_most_one_prerequisite_in_review(self):
        self.story(1, "Prices", change="prices")
        self.story(2, "Taxes", change="taxes")
        self.story(3, "Checkout", after=[1, 2])
        self.change("prices", "built")
        self.change("taxes", "built")
        self.assertEqual(self.ids(self.next()["blocked"]), [3])
        shutil.rmtree(self.root / "openspec" / "changes" / "taxes")
        self.change("taxes", archived=True)
        self.assertEqual(self.ids(self.next()["ready_to_start"]), [3])

    def test_a_backlog_story_waits_until_its_change_holds_spec_deltas(self):
        self.story(1, "Scaffold", status="backlog", change="scaffold")
        self.story(2, "Other", status="backlog")
        self.story(3, "Refined already")
        self.change("scaffold", "draft")
        out = self.next()
        self.assertEqual((self.ids(out["ready_to_refine"]), self.ids(out["ready_to_start"])), ([1, 2], [3]))
        shutil.rmtree(self.root / "openspec" / "changes" / "scaffold")
        self.change("scaffold", "draft", specs=True)
        out = self.next()
        self.assertEqual(self.ids(out["ready_to_start"]), [1, 3])
        self.assertTrue(out["ready_to_start"][0]["refined"])

    def test_blocked_status_and_blocked_at_land_in_blocked(self):
        self.story(1, "A", change="a")
        self.story(2, "B", change="b")
        self.story(3, "C", change="c")
        self.change("a", "blocked")
        self.change("b", "ready-for-dev", extra='  blocked_at: "2026-09-05"\n  blocked_reason: "legal: terms"\n')
        self.change("c", "ready-for-dev")
        out = self.next()
        self.assertEqual(self.ids(out["blocked"]), [1, 2])
        self.assertEqual(out["blocked"][1]["blocked_reason"], "legal: terms")
        self.assertEqual(self.ids(out["ready_to_start"]), [3])

    def test_a_cancelled_story_is_dropped_and_still_blocks(self):
        self.story(1, "Old", status="cancelled")
        self.story(2, "New", after=[1])
        status = self.status()
        self.assertEqual([t["state"] for t in status["tickets"]], ["dropped", "planned"])
        self.assertEqual(self.numbers(status["tickets"][0]["blocks"]), [2])
        self.assertEqual(self.ids(self.next()["blocked"]), [2])

    def test_stories_are_named_by_their_key(self):
        self.story(1, "A", change="a")
        self.story(2, "B", after=["work-1"])
        self.change("a", "done")
        row = self.status()["tickets"][1]
        self.assertEqual((row["after"], row["ref"]), (["WORK-1"], "WORK-2"))
        for ref in ("work-1", "WORK-1", "id-1", "a"):
            self.assertEqual(self.find(ref)["ref"], "WORK-1", ref)

    def test_text_outside_ascii_is_read_as_utf8(self):
        self.story(1, "Café v menu")
        r = self.scoped("next")
        self.assertIn("Café v menu", r.stdout)
        self.assertEqual(json.loads(r.stdout)["ready_to_start"][0]["title"], "Café v menu")


class ProjectTests(TreeCase):
    def project(self):
        """Epic Pricing with two stories, before epic Cart on the board: the scope of the project."""
        self.pricing = self.add_epic("Pricing", position=-1)
        self.story(11, "Pricing contract", epic=self.pricing, change="contract")
        self.story(12, "Pricing rules", epic=self.pricing, change="rules", after=[11])
        return ["--project", PROJECT]

    def test_rows_carry_refs_find_resolves_in_the_scope_asked(self):
        whole = self.project()
        self.change("contract")
        self.change("rules")
        self.story(1, "Cart", after=[12])
        self.story(2, "Codes", after=[1])
        for scope, numbers in ((whole, [11, 12, 1, 2]), (["--epic", self.epic], [1, 2])):
            rows = self.status(scope)["tickets"]
            self.assertEqual(self.ids(rows), numbers)
            for row in rows:
                found = self.find(row["ref"], scope)
                self.assertEqual((found["epic"], found["id"]), (row["epic"], row["id"]))
        self.assertEqual(self.numbers(self.status()["tickets"][0]["after"]), [12])

    def test_cross_epic_after_and_whole_epic_after(self):
        whole = self.project()
        self.change("contract", "in-review")
        self.change("rules", "in-review")
        self.story(1, "Cart", after=[12])
        self.story(2, "Codes", after=[self.pricing])
        out = self.next()
        # A story in review meets an `after` for building; a whole epic waits until it is done
        self.assertEqual(self.ids(out["ready_to_start"]), [1])
        self.assertEqual(self.ids(out["blocked"]), [2])
        for name in ("contract", "rules"):
            shutil.rmtree(self.root / "openspec" / "changes" / name)
            self.change(name, archived=True)
        self.assertEqual(self.ids(self.next()["ready_to_start"]), [1, 2])
        epics = self.status(whole)["epics"]
        self.assertEqual([(e["title"], e["ref"], e["status"]) for e in epics],
                         [("Pricing", self.pricing, "done"), ("Cart", self.epic, "")])
        self.assertEqual(self.numbers(epics[0]["blocks"]), [2])

    def test_an_epic_waits_on_another_epic_with_what_it_needs(self):
        whole = self.project()
        self.change("contract")
        self.change("rules")
        checkout = self.add_epic("Checkout", after=[self.pricing], needs="its contract")
        self.story(1, "Checkout", epic=checkout, after=[11])
        epics = {e["title"]: e for e in self.status(whole)["epics"]}
        self.assertEqual(epics["Checkout"]["after"], [self.pricing])
        self.assertEqual(self.status(whole)["unpinned_after"], [])

    def test_unpinned_after_reports_an_epic_with_no_story_waiting(self):
        whole = self.project()
        self.change("contract")
        self.change("rules")
        checkout = self.add_epic("Checkout", after=[self.pricing], needs="its contract")
        self.story(1, "Cart", epic=checkout)
        expected = [{"epic": checkout, "after": self.pricing, "needs": "its contract"}]
        self.assertEqual(self.status(whole)["unpinned_after"], expected)
        self.assertEqual(self.next(["--epic", checkout])["unpinned_after"], expected)

    def test_an_epic_without_stories_has_no_stories(self):
        whole = self.project()
        self.change("contract")
        self.change("rules")
        self.assertEqual(self.status(whole)["counts"], {"total": 2, "backlog": 2})
        self.assertEqual(self.status()["counts"], {"total": 0})

    def test_issues_outside_epics_are_no_tickets(self):
        self.issue(50, "Nasir delivers the buildings", "chore")
        self.issue(51, "Weekrooster per klant", "journey")
        self.story(1, "Cart")
        self.assertEqual(self.ids(self.status(["--project", PROJECT])["tickets"]), [1])


class FindTests(TreeCase):
    def test_find_returns_the_story_its_sections_its_change_and_its_path(self):
        self.story(1, "Scaffold", change="scaffold", description=body(
            description="It adds.", verify="It runs.", unknown="load", references=["adr/0001-x.md"], notes=["kept"],
            covers=["R1"]))
        self.story(2, "UI shell")
        path = self.change("scaffold", "draft")
        found = self.find("scaffold")
        self.assertEqual((self.n[found["id"]], found["change"], found["change_dir"]), (1, "scaffold", str(path)))
        self.assertEqual(
            (found["description"], found["verify"], found["references"], found["notes"], found["unknown"],
             found["covers"]),
            ("It adds.", "It runs.", ["adr/0001-x.md"], ["kept"], "load", ["R1"]),
        )
        planned = self.find("ui")
        self.assertEqual((self.n[planned["id"]], planned["change_dir"]), (2, None))

    def test_find_names_the_archived_folder(self):
        self.story(1, "Scaffold", change="scaffold")
        path = self.change("scaffold", archived=True)
        self.assertEqual(self.find("WORK-1")["change_dir"], str(path))

    def test_a_fenced_heading_does_not_end_a_section(self):
        description = "It shows:\n\n```md\n## Verify\nnot a heading\n```"
        self.story(1, "Scaffold", description=body(description=description, verify="It runs."))
        found = self.find("WORK-1")
        self.assertEqual((found["description"], found["verify"]), (description, "It runs."))

    def test_find_reads_a_story_in_another_epic(self):
        other = self.add_epic("Pricing")
        self.story(1, "Contract", epic=other, description=body(description="It adds.", verify="It runs."))
        found = self.find("WORK-1")
        self.assertEqual((found["description"], found["verify"], found["epic"]), ("It adds.", "It runs.", other))

    def test_ambiguous_and_unknown_refs_fail(self):
        self.story(1, "Cart add")
        self.story(2, "Cart remove")
        self.fails(self.scoped("find", "cart"), "more than one")
        self.fails(self.scoped("find", "WORK-9"), "no story matches")


class PullTests(TreeCase):
    def pull(self, *args):
        return self.scoped("pull", *args)

    def test_pull_creates_the_change_and_sets_it_on_the_story(self):
        self.story(1, "Scaffold", status="backlog")
        out = self.ok(self.pull("WORK-1"))
        path = self.root / "openspec" / "changes" / "scaffold"
        self.assertEqual(out, {"change": "scaffold", "change_dir": str(path), "refine": True})
        self.assertTrue((path / ".openspec.yaml").is_file())
        self.assertEqual(self.get("WORK-1")["metadata"], {"change": "scaffold"})
        self.assertEqual(self.status()["tickets"][0]["state"], "backlog")
        self.fails(self.pull("WORK-1"), "already pulled")

    def test_pull_takes_a_name_and_refuses_one_taken_or_bad(self):
        self.story(1, "Scaffold")
        self.story(2, "UI shell", change="scaffold")
        self.change("scaffold", "done")
        self.fails(self.pull("WORK-1"), "exists already")
        self.fails(self.pull("WORK-1", "--name", "Bad Name"), "kebab-case")
        self.ok(self.pull("WORK-1", "--name", "cart-scaffold"))
        self.assertEqual(self.get("WORK-1")["metadata"], {"change": "cart-scaffold"})
        self.fails(self.pull("WORK-7"), "no story matches")

    def test_pull_refuses_a_spike(self):
        self.story(1, "Tax engine?", kind="spike")
        self.fails(self.pull("WORK-1"), "a spike produces no change")
        self.assertFalse((self.root / "openspec" / "changes" / "tax-engine").exists())

    def test_a_failed_openspec_call_leaves_the_story_unchanged(self):
        self.story(1, "Refused")
        self.fails(self.pull("WORK-1"), "Change name refused")
        self.assertEqual(self.get("WORK-1")["metadata"], {})

    def test_a_refused_metadata_write_leaves_no_change_folder(self):
        self.story(1, "Scaffold")
        self.state["refuse"] = "metadata write refused"
        self.fails(self.pull("WORK-1"), "metadata write refused")
        self.assertFalse((self.root / "openspec" / "changes" / "scaffold").exists())

    def test_pull_uses_specs_openspec_when_the_project_has_it(self):
        (self.root / "openspec").rename(self.root / "moved")
        (self.root / "specs").mkdir()
        (self.root / "moved").rename(self.root / "specs" / "openspec")
        self.story(1, "Scaffold")
        out = self.ok(self.pull("WORK-1"))
        path = self.root / "specs" / "openspec" / "changes" / "scaffold"
        self.assertEqual(out["change_dir"], str(path))
        self.assertEqual(self.status()["tickets"][0]["change"], "scaffold")

    @unittest.skipIf(shutil.which("git") is None, "git is not installed")
    def test_pull_refuses_a_story_whose_worklane_is_another_repository(self):
        self.on_github()
        other = self.lane("acme/other", "repo-other")
        self.story(1, "Scaffold", lane=other)
        self.fails(self.pull("WORK-1"), "another repository")
        self.assertFalse((self.root / "openspec" / "changes" / "scaffold").exists())

    @unittest.skipIf(shutil.which("git") is None, "git is not installed")
    def test_pull_creates_the_change_in_the_worklane_repository(self):
        lane = self.on_github()
        self.story(1, "Scaffold", lane=lane)
        self.ok(self.pull("WORK-1"))
        self.assertEqual(self.status()["tickets"][0]["state"], "backlog")

    def test_pull_creates_the_change_in_the_first_project_root(self):
        self.story(1, "Scaffold")
        self.story(2, "UI shell", change="shell")
        other = tempfile.TemporaryDirectory()
        self.addCleanup(other.cleanup)
        second = Path(other.name).resolve()
        (second / "openspec" / "changes" / "shell").mkdir(parents=True)
        both = ["--project-root", self.root, "--project-root", second]
        out = self.ok(self.run_cli(*both, "pull", "--epic", self.epic, "WORK-1"))
        path = self.root / "openspec" / "changes" / "scaffold"
        self.assertEqual((out["change_dir"], path.is_dir()), (str(path), True))

    @unittest.skipIf(shutil.which("git") is None, "git is not installed")
    def test_an_archive_counts_as_merged_only_once_the_default_branch_has_it(self):
        shutil.rmtree(self.root / ".git")

        def git(*args):
            subprocess.run(["git", "-C", str(self.root), *args], check=True, capture_output=True)

        git("init", "-q", "-b", "main")
        git("config", "user.email", "t@example.com")
        git("config", "user.name", "t")
        self.story(1, "Scaffold", change="scaffold")
        self.story(2, "UI shell", after=[1])
        (self.root / "openspec" / "changes" / "archive" / ".keep").write_text("", encoding="utf-8")
        git("add", "-A")
        git("commit", "-q", "-m", "base")
        git("checkout", "-q", "-b", "scaffold")
        self.change("scaffold", archived=True)
        git("add", "-A")
        git("commit", "-q", "-m", "archive scaffold")
        out = self.next()
        self.assertEqual((self.ids(out["ready_to_start"]), self.status()["tickets"][0]["status"]), ([2], "in-review"))
        git("checkout", "-q", "main")
        git("merge", "-q", "scaffold")
        self.assertEqual((self.ids(self.next()["ready_to_start"]), self.status()["tickets"][0]["status"]), ([2], "done"))

    @unittest.skipIf(shutil.which("git") is None, "git is not installed")
    @unittest.skipIf(shutil.which("openspec") is None, "the openspec CLI is not installed")
    def test_real_cli_pull_then_mark_keeps_the_change_valid(self):
        self.story(1, "Scaffold")
        self.story(2, "UI shell", after=[1])
        real = shutil.which("openspec", path=os.environ["PATH"].split(os.pathsep, 1)[1])
        (Path(os.environ["PATH"].split(os.pathsep, 1)[0]) / "openspec").unlink()
        os.symlink(real, Path(os.environ["PATH"].split(os.pathsep, 1)[0]) / "openspec")
        self.ok(self.pull("WORK-1"))
        self.ok(self.scoped("mark", "WORK-1", "in-progress"))
        r = subprocess.run(["openspec", "status", "--change", "scaffold", "--json"], cwd=self.root, encoding="utf-8",
                           capture_output=True, check=False)
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertEqual(self.status()["tickets"][0]["status"], "in-progress")


class LightsTests(TreeCase):
    def test_starting_holds_stories_without_brians_green_light(self):
        self.story(1, "Lit", labels=["green light: Brian"])
        self.story(2, "Unlit")
        out = self.ok(self.scoped("next", "--lights"))
        self.assertEqual((self.ids(out["ready_to_start"]), self.ids(out["held"])), ([1], [2]))


class MarkTests(TreeCase):
    def mark(self, *args):
        return self.scoped("mark", *args)

    def test_mark_writes_the_build_map_and_keeps_the_rest(self):
        self.story(1, "Scaffold", change="scaffold")
        self.story(2, "UI shell", after=[1])
        path = self.change("scaffold", "in-progress", extra="  type: feature # feature | bugfix\ncontext:\n  - src/cart.ts\nlenses_ran: []\n")
        (path / ".openspec.yaml").write_text((path / ".openspec.yaml").read_text(encoding="utf-8") + "notes: kept\n",
                                             encoding="utf-8")
        out = self.ok(self.mark("WORK-1", "in-review", "--assignee", "ann"))
        self.assertEqual(out, {"change": "scaffold", "change_dir": str(path), "spike_file": None, "status": "in-review",
                               "assignee": "ann", "blocked_at": "", "blocked_reason": ""})
        text = (path / ".openspec.yaml").read_text(encoding="utf-8")
        self.assertIn('  status: in-review\n  type: feature # feature | bugfix\n  ticket: "WORK-1"\n  assignee: "ann"\ncontext:\n', text)
        self.assertTrue(text.endswith("notes: kept\n"))
        self.assertEqual(self.ids(self.next()["ready_to_start"]), [2])

    def test_mark_creates_the_build_map_on_a_fresh_change(self):
        self.story(1, "Scaffold", change="scaffold")
        path = self.change("scaffold")
        self.ok(self.mark("scaffold", "ready-for-dev"))
        text = (path / ".openspec.yaml").read_text(encoding="utf-8")
        self.assertTrue(text.startswith("schema: intent-driven\ncreated: 2026-09-20\nbuild:\n  status: ready-for-dev\n"))

    def test_blocked_sets_both_fields_and_a_later_mark_clears_them(self):
        self.story(1, "Scaffold", change="scaffold")
        path = self.change("scaffold", "in-progress")
        for reason in ("waiting on legal: terms", "it's odd", 'say "hi" \\ # not a comment', "Café ✓"):
            out = self.ok(self.mark("scaffold", "blocked", "--blocked", reason))
            self.assertEqual((out["blocked_at"], out["blocked_reason"]), (date.today().isoformat(), reason))
            row = self.next()["blocked"][0]
            self.assertEqual(row["blocked_reason"], reason)
        self.ok(self.mark("scaffold", "in-progress"))
        text = (path / ".openspec.yaml").read_text(encoding="utf-8")
        self.assertNotIn("blocked_", text)
        self.assertEqual(self.ids(self.next()["in_progress"]), [1])

    def test_mark_refuses_a_story_with_no_change_or_an_archived_one(self):
        self.story(1, "Scaffold")
        self.story(2, "Old", change="old")
        self.change("old", archived=True)
        self.fails(self.mark("WORK-1", "done"), "pull it first")
        self.fails(self.mark("old", "done"), "archived")

    def test_mark_done_makes_the_story_done(self):
        self.story(1, "Scaffold", change="scaffold")
        self.story(2, "UI shell", after=[1])
        self.change("scaffold", "built")
        self.ok(self.mark("scaffold", "done"))
        self.assertEqual(self.status()["counts"], {"total": 2, "done": 1, "planned": 1})


class MalformedTests(TreeCase):
    def test_malformed_trees_error_with_a_message(self):
        def elsewhere():
            self.issue(9, "Outside", "story", project_id="project-other")
            self.story(1, "A", after=["WORK-9"])

        def cycle():
            self.story(1, "A", after=["WORK-2"])
            self.story(2, "B", after=[1])

        def epic_after_a_story():
            self.story(1, "A")
            self.get(self.epic)["metadata"]["blocked_by"] = "WORK-1"

        cases = (
            (elsewhere, "matches no story or epic"),
            (cycle, "cycle through"),
            (lambda: self.story(1, "A", kind="task"), "is not one of"),
            (lambda: self.story(1, "A", change="Bad Name"), "kebab-case"),
            (lambda: self.story(1, "A", change="gone"), "neither in"),
            (lambda: self.story(1, "A", labels=["risk:low", "risk:high"]), "more than one risk: label"),
            (lambda: self.story(1, "A", labels=["risk:extreme"]), "risk 'extreme' is not one of"),
            (epic_after_a_story, "which is no epic"),
        )
        for build, message in cases:
            with self.subTest(message):
                self.state["issues"] = [i for i in self.state["issues"] if i["identifier"] == self.epic]
                self.get(self.epic)["metadata"] = {}
                self.n.clear()
                build()
                self.fails(self.scoped("next"), message)

    def test_a_bad_build_status_errors(self):
        self.story(1, "A", change="a")
        self.change("a", "todo")
        self.fails(self.scoped("next"), "build.status")

    def test_a_scope_that_is_no_epic_is_refused(self):
        story = self.story(1, "A")
        self.fails(self.scoped("next", scope=["--epic", story]), "is a story, not an epic")
        self.fails(self.scoped("next", scope=["--epic", "WORK-404"]), "not found")


class SpikeTests(TreeCase):
    def test_a_spike_reads_and_marks_its_build_block(self):
        spike = self.story(1, "Tax engine?", kind="spike", status="backlog")
        self.story(2, "Codes", after=[1])
        self.assertEqual(self.find(spike)["spike_file"], spike)
        self.assertEqual(self.status()["tickets"][0]["state"], "planned")
        out = self.ok(self.scoped("mark", spike, "in-progress", "--assignee", "ann"))
        self.assertEqual((out["spike_file"], out["status"], out["assignee"], out["change_dir"]),
                         (spike, "in-progress", "ann", None))
        self.assertEqual(self.ids(self.next()["in_progress"]), [1])
        result = "## Result\n\nNo tax engine: a table does."
        self.get(spike)["description"] = self.get(spike)["description"] + "\n\n" + result
        self.ok(self.scoped("mark", spike, "done"))
        markdown = self.get(spike)["description"]
        self.assertIn(result, markdown)
        self.assertIn('```yaml\nstatus: done\nassignee: "ann"\n```', markdown)
        self.assertEqual(self.ids(self.next()["ready_to_start"]), [2])
        updates = [c for c in self.state["calls"] if c[:2] == ["issue", "update"]]
        self.assertTrue(all("--no-start" in c for c in updates))

    def test_marking_a_spike_keeps_a_fenced_build_heading_in_its_result(self):
        spike = self.story(1, "Tax engine?", kind="spike")
        result = "## Result\n\nA build block reads:\n\n```md\n## Build\n\nstatus: done\n```\n\nThat is all."
        self.get(spike)["description"] = result
        self.ok(self.scoped("mark", spike, "done"))
        self.assertIn(result, self.get(spike)["description"])
        self.assertEqual(self.find(spike)["status"], "done")

    def test_a_spike_in_another_epic_still_meets_an_after(self):
        other = self.add_epic("Research")
        spike = self.story(1, "Tax engine?", kind="spike", epic=other)
        self.story(2, "Codes", after=[1])
        self.assertEqual(self.ids(self.next()["blocked"]), [2])
        self.ok(self.scoped("mark", spike, "done", scope=["--epic", other]))
        self.assertEqual(self.ids(self.next()["ready_to_start"]), [2])

    def test_a_spike_is_never_pulled_and_never_has_a_change(self):
        self.story(1, "Tax engine 2?", kind="spike", change="tax")
        self.fails(self.scoped("next"), "a spike produces no change")


class ProjectRootTests(TreeCase):
    """The project root is `--project-root`, else the nearest `.git` at or above the working directory."""

    def setUp(self):
        super().setUp()
        self.story(1, "Scaffold", change="scaffold")
        self.story(2, "UI shell", after=[1])
        self.change("scaffold", "in-progress")

    def elsewhere(self):
        other = tempfile.TemporaryDirectory()
        self.addCleanup(other.cleanup)
        return Path(other.name).resolve()

    def test_the_repository_holding_the_working_directory_is_the_root(self):
        (self.root / "deep" / "er").mkdir(parents=True)
        out = self.ok(self.run_cli("next", "--epic", self.epic, cwd=self.root / "deep" / "er"))
        self.assertEqual(out["in_progress"][0]["change"], "scaffold")

    def test_a_git_file_marks_a_worktree_root(self):
        (self.root / ".git").rmdir()
        (self.root / ".git").write_text("gitdir: /elsewhere\n", encoding="utf-8")
        out = self.ok(self.run_cli("next", "--epic", self.epic, cwd=self.root))
        self.assertEqual(out["in_progress"][0]["change"], "scaffold")

    def test_project_root_flag_names_the_project_from_anywhere(self):
        out = self.ok(self.run_cli("--project-root", self.root, "status", "--epic", self.epic, cwd=self.elsewhere()))
        self.assertEqual(out["counts"], {"total": 2, "in-progress": 1, "planned": 1})

    def test_no_project_root_found(self):
        cwd = self.elsewhere()
        self.fails(self.run_cli("status", "--epic", self.epic, cwd=cwd), "no .git found; pass --project-root")
        self.fails(self.run_cli("pull", "--epic", self.epic, "WORK-2", cwd=cwd), "no project root found")

    def test_a_tree_reads_each_change_from_the_repository_that_holds_it(self):
        # Given story 2 is pulled into a second repository while story 1 stays in the first
        self.get("WORK-2")["metadata"]["change"] = "shell"
        second = self.elsewhere()
        self.change("shell", "in-review", root=second)
        both = ["--project-root", self.root, "--project-root", second]
        out = self.ok(self.run_cli(*both, "status", "--epic", self.epic))
        self.assertEqual([(t["change"], t["status"]) for t in out["tickets"]],
                         [("scaffold", "in-progress"), ("shell", "in-review")])
        self.fails(self.run_cli("--project-root", self.root, "status", "--epic", self.epic), "neither in")

    @unittest.skipIf(shutil.which("git") is None, "git is not installed")
    def test_a_change_reads_only_from_its_worklane_repository(self):
        # Given both repositories hold a change `shell`, done in acme/shop and in progress in acme/other,
        # the repository of story 1's worklane, whose origin spells its name in other casing
        shutil.rmtree(self.root / ".git")
        second = self.elsewhere()
        for root, repo in ((self.root, "shop"), (second, "Other")):
            subprocess.run(["git", "init", "-q", str(root)], check=True)
            subprocess.run(["git", "-C", str(root), "remote", "add", "origin", f"git@github.com:acme/{repo}.git"],
                           check=True)
        self.lane("acme/shop", "repo-shop")
        lane = self.lane("acme/other", "repo-other", "lane-a")
        shutil.rmtree(self.root / "openspec" / "changes" / "scaffold")
        self.change("shell", "done")
        self.change("shell", "in-progress", root=second)
        story = self.get("WORK-1")
        story["metadata"]["change"], story["worklane_id"] = "shell", lane
        both = ["--project-root", self.root, "--project-root", second]
        out = self.ok(self.run_cli(*both, "next", "--epic", self.epic))
        self.assertEqual((self.ids(out["in_progress"]), self.ids(out["blocked"])), ([1], [2]))
        self.fails(self.run_cli("--project-root", self.root, "next", "--epic", self.epic), "no project root is a clone")

    @unittest.skipIf(shutil.which("git") is None, "git is not installed")
    def test_a_change_in_another_worktree_of_the_repository_is_found(self):
        # Given story 2 is pulled in its own worktree, and the main checkout does not hold its change
        shutil.rmtree(self.root / ".git")

        def git(*args):
            subprocess.run(["git", "-C", str(self.root), *args], check=True, capture_output=True)

        git("init", "-q", "-b", "main")
        git("config", "user.email", "t@example.com")
        git("config", "user.name", "t")
        git("add", "-A")
        git("commit", "-q", "-m", "base")
        worktree = self.elsewhere() / "ui-shell"
        git("worktree", "add", "-q", "-b", "ui-shell", str(worktree))
        self.change("shell", "in-progress", root=worktree)
        self.get("WORK-2")["metadata"]["change"] = "shell"
        rows = self.status()["tickets"]
        self.assertEqual([(t["change"], t["status"]) for t in rows], [("scaffold", "in-progress"), ("shell", "in-progress")])

    def test_every_command_needs_a_scope(self):
        for command in (["next"], ["status"], ["find", "scaffold"], ["pull", "scaffold"], ["mark", "scaffold", "done"]):
            err = io.StringIO()
            with contextlib.redirect_stderr(err), self.assertRaises(SystemExit) as raised:
                tickets.main(command)
            self.assertEqual(raised.exception.code, 2, command)
            self.assertIn("required", err.getvalue(), command)


if __name__ == "__main__":
    unittest.main()
