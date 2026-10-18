import contextlib
import importlib.util
import io
import json
import subprocess
import sys
import tempfile
import unittest
import unittest.mock
from pathlib import Path

SCRIPT = Path(__file__).resolve().parents[2] / "server/internal/service/builtin_skills/multica-platform/scripts" / "team_state.py"
spec = importlib.util.spec_from_file_location("team_state", SCRIPT)
team_state = importlib.util.module_from_spec(spec)
sys.modules["team_state"] = team_state
spec.loader.exec_module(team_state)


VERIFIER = "agent-verifier"


def row(key, status="todo", labels=(), release="0.1.0", parent=None, stage=None, blocked_by=(), assignee=None):
    """An issue as `issue_row` gives it."""
    return {
        "key": key, "title": f"Title of {key}", "type": None, "status": status, "labels": list(labels),
        "points": None, "due_date": None, "release": release, "parent": parent, "stage": stage,
        "assignee": {"type": "agent", "id": assignee} if assignee else None,
        "blocked_by": list(blocked_by), "change": None,
    }


def head(rows, known=None):
    known = known or {r["key"]: r for r in rows}
    found = team_state.lane_head(rows, known, VERIFIER)
    return {slot: (found[slot]["key"], found[slot]["waits_on"]) if found[slot] else None for slot in ("active", "next")}


class ALaneHasOneActiveItem(unittest.TestCase):
    def test_the_first_started_issue_is_active_and_the_first_unstarted_one_is_next(self):
        rows = [row("WORK-1", "done"), row("WORK-2", "in_progress", assignee=VERIFIER), row("WORK-3"),
                row("WORK-4", "in_progress")]
        self.assertEqual(head(rows), {"active": ("WORK-4", []), "next": ("WORK-3", [])})

    def test_an_issue_sarkout_holds_is_finished_in_its_lane(self):
        rows = [row("WORK-1", "in_progress", assignee=VERIFIER), row("WORK-2", "cancelled")]
        self.assertEqual(head(rows), {"active": None, "next": None})

    def test_an_issue_back_with_whoever_fixes_it_is_active_again(self):
        rows = [row("WORK-1", "in_progress", assignee="agent-backend-lead"), row("WORK-2", "in_progress", assignee=VERIFIER)]
        self.assertEqual(head(rows), {"active": ("WORK-1", []), "next": None})

    def test_without_the_verifier_s_agent_no_assignee_finishes_an_issue(self):
        rows = [row("WORK-1", "in_progress", assignee=VERIFIER)]
        self.assertEqual(team_state.lane_head(rows, {"WORK-1": rows[0]}, None)["active"]["key"], "WORK-1")

    def test_a_blocked_issue_stays_active_and_waits_on_its_block(self):
        rows = [row("WORK-1", "blocked"), row("WORK-2")]
        self.assertEqual(head(rows), {"active": ("WORK-1", [{"kind": "blocked"}]), "next": ("WORK-2", [])})


class AnIssueWaitsOnWhatHoldsIt(unittest.TestCase):
    def test_only_an_issue_named_in_blocked_by_that_still_holds_work_counts(self):
        # Given WORK-9 is named by WORK-5 and still in progress, the verifier holds WORK-6, WORK-7 is done
        known = {r["key"]: r for r in (row("WORK-6", "in_progress", assignee=VERIFIER), row("WORK-7", "done"),
                                       row("WORK-9", "in_progress"))}
        lane = [row("WORK-5", blocked_by=("WORK-6", "WORK-7", "WORK-9", "WORK-99"))]
        self.assertEqual(
            head(lane, {**known, "WORK-5": lane[0]})["next"][1],
            [{"kind": "issue", "key": "WORK-9", "status": "in_progress"}, {"kind": "issue", "key": "WORK-99", "status": None}],
        )

    def test_a_journey_step_waits_on_its_earlier_steps_until_sarkout_holds_them(self):
        steps = [row("WORK-11", "in_progress", parent="WORK-10", stage=1, assignee=VERIFIER),
                 row("WORK-12", parent="WORK-10", stage=2), row("WORK-13", parent="WORK-10", stage=3)]
        found = team_state.lane_head(steps, {r["key"]: r for r in steps}, VERIFIER)
        self.assertEqual(found["next"]["key"], "WORK-12")
        self.assertEqual(found["next"]["waits_on"], [])
        self.assertEqual(
            team_state.waits_on(steps[2], steps[:2], {r["key"]: r for r in steps}, VERIFIER),
            [{"kind": "step", "key": "WORK-12", "status": "todo"}],
        )

    def test_a_client_wait_ahead_in_the_same_release_holds_the_work_behind_it(self):
        rows = [row("WORK-20", labels=["Client wait"]), row("WORK-21", "done", labels=["Client wait"]),
                row("WORK-22", labels=["Client wait"], release="0.2.0"), row("WORK-23")]
        waits = team_state.waits_on(rows[3], rows[:3], {r["key"]: r for r in rows}, VERIFIER)
        self.assertEqual([w["key"] for w in waits], ["WORK-20"])
        # And the client wait itself waits on no other client wait
        self.assertEqual(team_state.waits_on(rows[2], rows[:2], {}, VERIFIER), [])


class IssuesAreReadAsMulticaHoldsThem(unittest.TestCase):
    def test_labels_points_parent_release_and_blocked_by_are_read(self):
        issue = {
            "id": "i2", "identifier": "WORK-2", "title": "Step", "type": "story", "status": "todo",
            "properties": {"p1": 5}, "due_date": None, "release_id": "r1", "parent_issue_id": "i1", "stage": 2,
            "assignee_type": "agent", "assignee_id": "a1", "metadata": {"blocked_by": "work-7, WORK-8"},
        }
        found = team_state.issue_row(issue, {"i2": ["Client wait"]}, "p1", {"i1": "WORK-1"}, {"r1": "0.3.0"})
        self.assertEqual(
            {k: found[k] for k in ("labels", "points", "parent", "release", "assignee", "blocked_by")},
            {"labels": ["Client wait"], "points": 5, "parent": "WORK-1", "release": "0.3.0",
             "assignee": {"type": "agent", "id": "a1"}, "blocked_by": ["WORK-7", "WORK-8"]},
        )

    def test_a_project_s_issues_are_read_page_after_page(self):
        pages = [{"issues": [{"identifier": "WORK-1"}], "has_more": True}, {"issues": [{"identifier": "WORK-2"}], "has_more": False}]
        with unittest.mock.patch.object(team_state, "multica", side_effect=pages) as called:
            self.assertEqual([i["identifier"] for i in team_state.project_issues("p")], ["WORK-1", "WORK-2"])
        self.assertEqual(called.call_args_list[1].args[-1], "1")

    def test_a_failed_multica_command_stops_with_its_error(self):
        with unittest.mock.patch.object(team_state.subprocess, "run",
                                        return_value=subprocess.CompletedProcess([], 1, "", "unauthorized")):
            with self.assertRaisesRegex(team_state.StateError, "unauthorized"):
                team_state.multica("repo", "list")


def git(cwd, *args):
    return subprocess.run(["git", *args], cwd=cwd, check=True, capture_output=True, text=True).stdout.strip()


class ChangesAreReadFromEveryCheckout(unittest.TestCase):
    def setUp(self):
        # Given "cart" was archived on main, and the main checkout has since moved to a branch archiving "receipt"
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.repo = Path(tmp.name).resolve()
        git(self.repo, "init", "-q", "-b", "main")
        git(self.repo, "config", "user.name", "Test")
        git(self.repo, "config", "user.email", "test@example.invalid")
        self.archive("cart")
        git(self.repo, "checkout", "-qb", "work-9")
        self.archive("receipt")
        open_change = self.repo / "openspec" / "changes" / "coupon"
        open_change.mkdir()
        (open_change / ".openspec.yaml").write_text("x-build:\n  status: in-progress\n  assignee: ann\n", encoding="utf-8")

    def archive(self, name):
        folder = self.repo / "openspec" / "changes" / "archive" / f"2026-09-29-{name}"
        folder.mkdir(parents=True)
        (folder / ".openspec.yaml").write_text("schema: intent-driven\n", encoding="utf-8")
        git(self.repo, "add", "-A")
        git(self.repo, "commit", "-qm", name)

    def found(self):
        return {c["name"]: (c["archived"], c["build"]) for c in team_state.collect_changes(self.repo, "main")}

    def test_a_branch_reports_its_open_changes_and_what_it_archived_that_main_has_not(self):
        self.assertEqual(self.found(), {"coupon": (False, {"status": "in-progress", "assignee": "ann"}), "receipt": (True, {})})

    def test_a_change_whose_build_map_cannot_be_read_keeps_its_error_and_the_others_are_read(self):
        (self.repo / "openspec" / "changes" / "broken").mkdir()
        (self.repo / "openspec" / "changes" / "broken" / ".openspec.yaml").write_text("x-build: [\n", encoding="utf-8")
        found = {c["name"]: c for c in team_state.collect_changes(self.repo, "main")}
        self.assertIn("error", found["broken"])
        self.assertNotIn("build", found["broken"])
        self.assertEqual(found["coupon"]["build"], {"status": "in-progress", "assignee": "ann"})
        self.assertNotIn("error", found["receipt"])

    def test_a_merge_on_the_remote_default_branch_counts_before_the_local_one_is_pulled(self):
        # Given the receipt branch was merged on GitHub and fetched, while the local main was not pulled
        git(self.repo, "update-ref", "refs/remotes/origin/main", "HEAD")
        git(self.repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
        self.assertNotIn("receipt", self.found())

    def test_a_worktree_and_its_repository_count_as_one(self):
        tree = self.repo.parent / f"{self.repo.name}-tree"
        git(self.repo, "worktree", "add", "-q", "-b", "feature", str(tree))
        self.addCleanup(lambda: subprocess.run(["rm", "-rf", str(tree)], check=False))
        self.assertEqual(team_state.repo_home(tree), team_state.repo_home(self.repo))


class CheckoutsAreFoundByTheirGitHubName(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.repos = Path(tmp.name).resolve()

    def checkout(self, folder, url=None):
        """A git folder with the remotes `upstream` and `origin` at `url`, or without a remote when `url` is None."""
        (self.repos / folder / ".git").mkdir(parents=True)
        config = f'[remote "upstream"]\n\turl = git@github.com:other/x.git\n[remote "origin"]\n\turl = {url}\n' if url else ""
        (self.repos / folder / ".git" / "config").write_text(config, encoding="utf-8")

    def test_any_depth_and_folder_name_count_and_a_look_alike_host_does_not(self):
        self.checkout("Clients/acme/apps/deep", "git@github.com:acme/deep.git")
        self.checkout("Forks/web", "https://github.com/Acme/Web")
        self.checkout("Forks/fake", "https://notgithub.com/acme/fake.git")
        expected = {"acme/deep": [self.repos / "Clients/acme/apps/deep"], "acme/web": [self.repos / "Forks/web"]}
        self.assertEqual(team_state.local_checkouts(self.repos), expected)

    def test_a_checkout_inside_a_git_folder_without_a_github_origin_is_found(self):
        # Given a git folder without a remote and one with a non-GitHub origin, each holding a GitHub checkout
        self.checkout("Clients/paps")
        self.checkout("Clients/paps/whitebird-brain", "git@github.com:appboypov/whitebird-brain.git")
        self.checkout("Clients/lab", "https://gitlab.com/lab/lab.git")
        self.checkout("Clients/lab/tools", "https://github.com/lab/tools.git")
        # When the checkouts are looked up, then both nested checkouts are found
        expected = {
            "appboypov/whitebird-brain": [self.repos / "Clients/paps/whitebird-brain"],
            "lab/tools": [self.repos / "Clients/lab/tools"],
        }
        self.assertEqual(team_state.local_checkouts(self.repos), expected)

    def test_a_github_checkout_hides_the_git_folders_inside_it(self):
        # Given a GitHub checkout that holds a vendored repository of its own
        self.checkout("Apps/shop", "git@github.com:acme/shop.git")
        self.checkout("Apps/shop/vendor/lib", "git@github.com:other/lib.git")
        # When the checkouts are looked up, then only the outer checkout is found
        self.assertEqual(team_state.local_checkouts(self.repos), {"acme/shop": [self.repos / "Apps/shop"]})

    def test_a_named_repository_joins_those_of_the_endeavour_folder(self):
        self.checkout("Clients/acme", "git@github.com:acme/app.git")
        found = team_state.endeavour_repositories(self.repos / "Clients/acme", ["Acme/Tools"])
        self.assertEqual(found, {"acme/app": [self.repos / "Clients/acme"], "acme/tools": []})


class PullRequestsNameTheirChangesAndIssues(unittest.TestCase):
    def test_open_and_archived_change_folders_are_found_in_any_openspec_home(self):
        paths = [
            "openspec/changes/cart-copy/proposal.md",
            "stores/web/openspec/changes/archive/2026-09-29-web-cart/tasks.md",
            "openspec/changes/archive/README.md",
            "skills/madspec/SKILL.md",
        ]
        self.assertEqual(team_state.change_names(paths), ["cart-copy", "web-cart"])

    def test_the_title_and_branch_name_their_issue_keys(self):
        self.assertEqual(team_state.issue_keys("Shop discounts (WORK-697)", "work-697"), ["WORK-697"])
        self.assertEqual(team_state.issue_keys("Fix build", "update-123x"), [])


class ClientsAreThoseTheEndeavourNames(unittest.TestCase):
    def test_the_agent_files_the_endeavour_names_are_read_with_their_chat_ids(self):
        root = Path(self.enterContext(tempfile.TemporaryDirectory()))
        (root / "rebl").mkdir()
        (root / "rebl" / "AGENTS.md").write_text(
            "**Client:** Cem. Read `~/.omp/agent/skills/das-client-agents/clients/cem/AGENTS.md`.\n", encoding="utf-8"
        )
        (root / "cem").mkdir()
        (root / "cem" / "AGENTS.md").write_text(
            "Updates group: `!group-cem:ba.local-whatsapp.localhost`.\n"
            "| +31600000000 | `!dm-cem:ba.local-whatsapp.localhost` | Default |\n",
            encoding="utf-8",
        )
        clients = team_state.endeavour_clients(root / "rebl", ["rodi"])
        self.assertEqual(clients, ["cem", "rodi"])
        found = team_state.client_chats(clients, root)
        self.assertEqual(found[0], ("cem", ["!group-cem:ba.local-whatsapp.localhost", "!dm-cem:ba.local-whatsapp.localhost"]))
        self.assertEqual(found[1]["client"], "rodi")


def pr(number, body="", base="main", issues=(), branch=None):
    """An open pull request as the intake reads it."""
    return {"number": number, "branch": branch or f"work-{number}", "head": f"h{number}", "base": base, "body": body,
            "issues": list(issues), "stack_note": team_state.stack_note(body, "acme/app")}


def edges_of(prs, holds=(), forked=(), gone=None):
    """The edges with ancestry as given: `holds` and `forked` are the (lower, upper) pairs whose upper holds the lower's
    head or forked from its unmerged work; `gone` maps a pull request that is not open to its state."""
    gone = gone or {}

    def resolve(number=None, branch=None):
        return {"number": number, "state": gone[number]} if number in gone else None

    def ancestry(lower, upper):
        pair = (lower["number"], upper["number"])
        return "contains" if pair in holds else "forked" if pair in forked else None

    return team_state.stack_edges(prs, "main", resolve, ancestry)


def units_of(prs, edges, issues=()):
    issues = {i["key"]: i for i in issues}
    return [(u["prs"], u["under"])
            for u in team_state.merge_units(prs, edges, issues, VERIFIER, team_state.waits(issues, []))]


def issue(key, status="in_progress", position=0, parent=None, stage=None, blocked_by=(), assignee=VERIFIER):
    return {**row(key, status, parent=parent, stage=stage, blocked_by=blocked_by, assignee=assignee),
            "position": position}


def ranked(prs, issues, edges=(), wakeups=()):
    edges = list(edges)
    issues = {i["key"]: i for i in issues}
    waits_of = team_state.waits(issues, list(wakeups))
    return team_state.rank_units(team_state.merge_units(prs, edges, issues, VERIFIER, waits_of), prs, issues, VERIFIER,
                                 waits_of, edges)


class StackNotesNameThePullRequestUnderIt(unittest.TestCase):
    def test_the_first_stacked_on_phrase_names_the_lower_pull_request_and_its_branch(self):
        body = "## Summary\n- Stacked on #66 (`work-1087`, agent-active-sessions), which is stacked on #59. Merge those first."
        note = team_state.stack_note(body, "acme/app")
        self.assertEqual((note["prs"], note["branches"], note["elsewhere"]), ([66], ["work-1087"], []))

    def test_a_link_counts_in_this_repository_and_a_commit_is_no_branch(self):
        note = team_state.stack_note(
            "Stacked on [the data model, PR #59](https://github.com/Acme/App/pull/59), branch `work-1057` at `91085dd6b6`.",
            "acme/app",
        )
        self.assertEqual((note["prs"], note["branches"]), ([59], ["work-1057"]))

    def test_another_repository_s_pull_request_is_a_companion_not_a_lower(self):
        note = team_state.stack_note("Stacked on acme/api#8 and https://github.com/acme/api/pull/7.", "acme/app")
        self.assertEqual((note["prs"], sorted(note["elsewhere"])), ([], ["acme/api#7", "acme/api#8"]))

    def test_a_lower_pull_request_saying_what_stacks_on_it_has_no_note(self):
        self.assertIsNone(team_state.stack_note("The frontend stacks on this branch.", "acme/app"))


class StacksBecomeMergeUnits(unittest.TestCase):
    def test_three_stacked_pull_requests_are_one_unit_even_when_the_top_lacks_a_fix_below(self):
        # Given B stacked on A and C on B, where B got a fix after C was cut
        prs = [pr(1), pr(2, "Stacked on #1."), pr(3, "Stacked on #2 (`work-2`).")]
        edges = edges_of(prs, holds={(1, 2)}, forked={(2, 3)})
        self.assertEqual([(e["upper"], e["lower"], e["status"], e.get("ancestry")) for e in edges],
                         [(2, 1, "verified", "contains"), (3, 2, "verified", "forked")])
        self.assertEqual(units_of(prs, edges), [([1, 2, 3], None)])

    def test_independent_pull_requests_stay_apart(self):
        prs = [pr(1), pr(2)]
        self.assertEqual(units_of(prs, edges_of(prs)), [([1], None), ([2], None)])

    def test_a_base_on_another_open_pull_request_s_branch_names_it_too(self):
        prs = [pr(1), pr(2, base="work-1")]
        self.assertEqual(units_of(prs, edges_of(prs, holds={(1, 2)})), [([1, 2], None)])

    def test_a_stale_note_is_shown_and_never_joins_the_pull_requests(self):
        # Given #4 names the merged #2, and #5 names the open #1 without holding any of its work
        prs = [pr(1), pr(4, "Stacked on #2."), pr(5, "Stacked on #1.")]
        edges = edges_of(prs, gone={2: "MERGED"})
        self.assertEqual([(e["upper"], e["lower"], e["status"]) for e in edges],
                         [(4, 2, "lower_merged"), (5, 1, "ancestry_unproven")])
        self.assertEqual(units_of(prs, edges), [([1], None), ([4], None), ([5], None)])
        ready, blocked = ranked(prs, [], edges)
        self.assertEqual([u["prs"] for u in ready], [[1], [4]])
        self.assertEqual(blocked[0]["blocked_by"], [{"kind": "stack_edge", "pr": 5, "lower": 1, "status": "ancestry_unproven"}])

    def test_sibling_stacks_share_their_lower_work_with_the_first_only(self):
        # Given A and B both stacked on L, and C stacked on A: L goes up with A and C, B waits on L apart
        prs = [pr(1), pr(2, "Stacked on #1."), pr(3, "Stacked on #1."), pr(4, "Stacked on #2.")]
        edges = edges_of(prs, holds={(1, 2), (1, 3), (2, 4)})
        self.assertEqual(units_of(prs, edges), [([1, 2, 4], None), ([3], 1)])
        ready, blocked = ranked(prs, [], edges)
        self.assertEqual([u["prs"] for u in ready], [[1, 2, 4]])
        self.assertEqual([(u["prs"], u["blocked_by"]) for u in blocked], [([3], [{"kind": "stack", "pr": 1}])])

    def test_ambiguous_and_cyclic_notes_stay_visible_and_block_their_units(self):
        prs = [pr(1, "Stacked on #2."), pr(2, "Stacked on #1."), pr(3), pr(4), pr(5, "Stacked on #3 and #4.")]
        edges = edges_of(prs, holds={(2, 1), (1, 2)})
        self.assertEqual([(e["upper"], e["status"]) for e in edges], [(1, "cycle"), (2, "cycle"), (5, "ambiguous")])
        self.assertEqual(edges[2]["candidates"], [3, 4])
        self.assertEqual(units_of(prs, edges), [([1], None), ([2], None), ([3], None), ([4], None), ([5], None)])
        ready, blocked = ranked(prs, [], edges)
        self.assertEqual(([u["prs"] for u in ready], [u["prs"] for u in blocked]), ([[3], [4]], [[1], [2], [5]]))

    def test_a_note_naming_only_another_repository_leaves_the_unit_ready(self):
        prs = [pr(1, "Stacked on acme/api#8.")]
        edges = edges_of(prs)
        self.assertEqual(edges[0]["status"], "other_repository")
        self.assertEqual([u["prs"] for u in ranked(prs, [], edges)[0]], [[1]])


class SharedLowerWorkGoesUpWithOneSiblingStack(unittest.TestCase):
    """L (#1) with sibling stacks A (#2) and B (#3) on it, each carrying its issue."""

    PRS = [pr(1, issues=["WORK-1"]), pr(2, "Stacked on #1.", issues=["WORK-2"]), pr(3, "Stacked on #1.", issues=["WORK-3"])]
    HOLDS = {(1, 2), (1, 3)}

    def test_the_sibling_with_more_downstream_of_its_own_takes_the_lower_work(self):
        # Given WORK-20 waits on A and WORK-30 and WORK-31 on B: counted from L, both [1, 2] and [1, 3] reach four
        issues = [issue("WORK-1"), issue("WORK-2"), issue("WORK-3"), issue("WORK-20", "todo", blocked_by=["WORK-2"]),
                  issue("WORK-30", "todo", blocked_by=["WORK-3"]), issue("WORK-31", "todo", blocked_by=["WORK-3"])]
        edges = edges_of(self.PRS, holds=self.HOLDS)
        self.assertEqual(units_of(self.PRS, edges, issues), [([1, 3], None), ([2], 1)])
        ready, blocked = ranked(self.PRS, issues, edges)
        self.assertEqual([(u["prs"], u["downstream"]) for u in ready],
                         [([1, 3], ["WORK-2", "WORK-20", "WORK-30", "WORK-31"])])
        self.assertEqual([(u["prs"], u["downstream"], u["blocked_by"]) for u in blocked],
                         [([2], ["WORK-20"], [{"kind": "stack", "pr": 1}])])

    def test_a_dependent_both_siblings_share_counts_once_for_each(self):
        # Given WORK-70 waits on A and B, WORK-50 on A, WORK-51 on A, WORK-50 and WORK-70, WORK-60 and WORK-61 on B
        issues = [issue("WORK-1"), issue("WORK-2"), issue("WORK-3"),
                  issue("WORK-70", "todo", blocked_by=["WORK-2", "WORK-3"]), issue("WORK-50", "todo", blocked_by=["WORK-2"]),
                  issue("WORK-51", "todo", blocked_by=["WORK-2", "WORK-50", "WORK-70"]),
                  issue("WORK-60", "todo", blocked_by=["WORK-3"]), issue("WORK-61", "todo", blocked_by=["WORK-3"])]
        ready, blocked = ranked(self.PRS, issues, edges_of(self.PRS, holds=self.HOLDS))
        self.assertEqual([(u["prs"], u["downstream_count"]) for u in ready], [([1, 3], 6)])
        self.assertEqual([(u["prs"], u["downstream"]) for u in blocked], [([2], ["WORK-50", "WORK-51", "WORK-70"])])

    def test_siblings_alike_go_by_board_order_then_pull_request_number(self):
        edges = edges_of(self.PRS, holds=self.HOLDS)
        on_board = [issue("WORK-1"), issue("WORK-2", position=7), issue("WORK-3", position=-1)]
        self.assertEqual(units_of(self.PRS, edges, on_board), [([1, 3], None), ([2], 1)])
        self.assertEqual(units_of(self.PRS, edges), [([1, 2], None), ([3], 1)])

    def test_a_fork_above_repeats_the_choice_and_its_other_branch_waits_on_its_own_lower(self):
        # Given D (#4) and E (#5) stacked on A, with WORK-50 and WORK-51 waiting on E and WORK-30 on B
        prs = [*self.PRS, pr(4, "Stacked on #2.", issues=["WORK-4"]), pr(5, "Stacked on #2.", issues=["WORK-5"])]
        issues = [issue("WORK-1"), issue("WORK-2"), issue("WORK-3"), issue("WORK-4"), issue("WORK-5"),
                  issue("WORK-50", "todo", blocked_by=["WORK-5"]), issue("WORK-51", "todo", blocked_by=["WORK-5"]),
                  issue("WORK-30", "todo", blocked_by=["WORK-3"])]
        edges = edges_of(prs, holds={*self.HOLDS, (2, 4), (2, 5)})
        self.assertEqual(units_of(prs, edges, issues), [([1, 2, 5], None), ([3], 1), ([4], 2)])
        ready, blocked = ranked(prs, issues, edges)
        self.assertEqual([u["prs"] for u in ready], [[1, 2, 5]])
        self.assertEqual([(u["prs"], u["blocked_by"]) for u in blocked],
                         [([3], [{"kind": "stack", "pr": 1}]), ([4], [{"kind": "stack", "pr": 2}])])

    def test_a_ready_sibling_takes_the_lower_work_over_one_more_unblocking_still_with_its_builder(self):
        # Given WORK-20 and WORK-21 wait on A, whose WORK-2 its builder still holds, and WORK-30 on B
        issues = [issue("WORK-1"), issue("WORK-2", assignee="agent-joeri"), issue("WORK-3"),
                  issue("WORK-20", "todo", blocked_by=["WORK-2"]), issue("WORK-21", "todo", blocked_by=["WORK-2"]),
                  issue("WORK-30", "todo", blocked_by=["WORK-3"])]
        edges = edges_of(self.PRS, holds=self.HOLDS)
        self.assertEqual(units_of(self.PRS, edges, issues), [([1, 3], None), ([2], 1)])
        ready, blocked = ranked(self.PRS, issues, edges)
        self.assertEqual([u["prs"] for u in ready], [[1, 3]])
        self.assertEqual([(u["prs"], u["blocked_by"]) for u in blocked], [([2], [
            {"kind": "stack", "pr": 1}, {"kind": "assignee", "key": "WORK-2", "assignee": {"type": "agent", "id": "agent-joeri"}}])])
        # When B is with its builder too, no candidate is ready and the more unblocking one goes first
        issues[2] = issue("WORK-3", assignee="agent-joeri")
        self.assertEqual(units_of(self.PRS, edges, issues), [([1, 2], None), ([3], 1)])

    def test_a_ready_sibling_takes_the_lower_work_over_one_with_an_open_fix(self):
        # Given WORK-20 waits on A, whose WORK-2 has the open Fix WORK-21
        issues = [issue("WORK-1"), issue("WORK-2"), issue("WORK-21", parent="WORK-2", assignee="agent-joeri"), issue("WORK-3"),
                  issue("WORK-20", "todo", blocked_by=["WORK-2"])]
        edges = edges_of(self.PRS, holds=self.HOLDS)
        self.assertEqual(units_of(self.PRS, edges, issues), [([1, 3], None), ([2], 1)])
        ready, blocked = ranked(self.PRS, issues, edges)
        self.assertEqual([u["prs"] for u in ready], [[1, 3]])
        self.assertEqual([(u["prs"], u["blocked_by"]) for u in blocked], [([2], [
            {"kind": "stack", "pr": 1}, {"kind": "child", "key": "WORK-21", "status": "in_progress"}])])

    def test_a_sibling_waiting_on_the_lower_work_is_ready_with_it(self):
        # Given A's WORK-2 is blocked by L's WORK-1, and WORK-20 waits on A
        issues = [issue("WORK-1"), issue("WORK-2", blocked_by=["WORK-1"]), issue("WORK-3"),
                  issue("WORK-20", "todo", blocked_by=["WORK-2"])]
        edges = edges_of(self.PRS, holds=self.HOLDS)
        self.assertEqual(units_of(self.PRS, edges, issues), [([1, 2], None), ([3], 1)])
        ready, blocked = ranked(self.PRS, issues, edges)
        self.assertEqual([(u["prs"], u["blocked_by"]) for u in ready + blocked],
                         [([1, 2], []), ([3], [{"kind": "stack", "pr": 1}])])

    def test_once_the_lower_work_merged_the_other_sibling_stack_stands_alone(self):
        # Given L landed with A: B and its upper G (#4) remain, B naming the merged L
        prs = [pr(3, "Stacked on #1.", issues=["WORK-3"]), pr(4, "Stacked on #3.", issues=["WORK-4"])]
        edges = edges_of(prs, holds={(3, 4)}, gone={1: "MERGED"})
        self.assertEqual([(e["upper"], e["lower"], e["status"]) for e in edges], [(3, 1, "lower_merged"), (4, 3, "verified")])
        self.assertEqual(units_of(prs, edges), [([3, 4], None)])
        ready, blocked = ranked(prs, [issue("WORK-3"), issue("WORK-4")], edges)
        self.assertEqual(([u["prs"] for u in ready], blocked), ([[3, 4]], []))

    def test_an_unconfirmed_sibling_joins_nothing_and_stays_visible(self):
        edges = edges_of(self.PRS, holds={(1, 2)})
        self.assertEqual(units_of(self.PRS, edges), [([1, 2], None), ([3], None)])
        ready, blocked = ranked(self.PRS, [issue("WORK-1"), issue("WORK-2"), issue("WORK-3")], edges)
        self.assertEqual([u["prs"] for u in ready], [[1, 2]])
        self.assertEqual([(u["prs"], u["blocked_by"]) for u in blocked],
                         [([3], [{"kind": "stack_edge", "pr": 3, "lower": 1, "status": "ancestry_unproven"}])])


# main: m0 <- m1; a lower branch m0 <- c1 <- c2, fixed later with c2 <- c5; an upper cut at c2 with c3; others on m1
PARENTS = {"m0": None, "m1": "m0", "c1": "m0", "c2": "c1", "c3": "c2", "c5": "c2", "x1": "m1", "x2": "m1", "x3": "m1"}


def lineage(commit):
    out = []
    while commit:
        out.append(commit)
        commit = PARENTS[commit]
    return out


def compare(name, base, head):
    """GitHub's comparison of two commits of `PARENTS`."""
    if base == head:
        return {"status": "identical", "merge_base": base}
    if base in lineage(head):
        return {"status": "ahead", "merge_base": base}
    if head in lineage(base):
        return {"status": "behind", "merge_base": head}
    return {"status": "diverged", "merge_base": next(c for c in lineage(head) if c in lineage(base))}


class AncestryIsReadFromGitHubsComparison(unittest.TestCase):
    def ancestry(self, lower, upper):
        with unittest.mock.patch.object(team_state, "compare", side_effect=compare):
            return team_state.ancestry("acme/app", "m1", {"head": lower}, {"head": upper})

    def test_an_upper_holding_the_lower_head_contains_it(self):
        self.assertEqual(self.ancestry("c2", "c3"), "contains")

    def test_an_upper_cut_before_a_lower_fix_forked_from_its_unmerged_work(self):
        self.assertEqual(self.ancestry("c5", "c3"), "forked")

    def test_sharing_only_the_default_branch_proves_no_stack(self):
        self.assertIsNone(self.ancestry("c5", "x1"))


class ReadyUnitsComeMostUnblockingFirst(unittest.TestCase):
    def test_distinct_open_issues_waiting_on_a_member_count_once_transitively(self):
        # Given WORK-10 and WORK-11 wait on WORK-1, WORK-12 on both, and WORK-13 on WORK-2; WORK-14 is done
        prs = [pr(3, issues=["WORK-2"]), pr(7, issues=["WORK-1"])]
        issues = [issue("WORK-1"), issue("WORK-2"), issue("WORK-10", "todo", blocked_by=["WORK-1"]),
                  issue("WORK-11", "backlog", blocked_by=["WORK-1"]),
                  issue("WORK-12", "todo", blocked_by=["WORK-10", "WORK-11"]),
                  issue("WORK-13", "todo", blocked_by=["WORK-2"]), issue("WORK-14", "done", blocked_by=["WORK-2"])]
        ready, blocked = ranked(prs, issues)
        self.assertEqual([(u["prs"], u["downstream"]) for u in ready],
                         [([7], ["WORK-10", "WORK-11", "WORK-12"]), ([3], ["WORK-13"])])
        self.assertEqual((ready[0]["downstream_count"], blocked), (3, []))

    def test_later_steps_wakeups_and_stacked_units_wait_on_their_lower_work(self):
        # Given step 2 of WORK-20's journey after the member step 1, an enabled wakeup of WORK-30 waiting for the
        # member to be done, a disabled one of WORK-31, and a sibling unit stacked on the member's pull request
        prs = [pr(1, issues=["WORK-21"]), pr(2, "Stacked on #1.", issues=["WORK-40"]),
               pr(3, "Stacked on #1.", issues=["WORK-41"])]
        issues = [issue("WORK-21", parent="WORK-20", stage=1), issue("WORK-22", "todo", parent="WORK-20", stage=2),
                  issue("WORK-30", "blocked"), issue("WORK-31", "todo"), issue("WORK-40"), issue("WORK-41")]
        wakeups = [
            {"issue_identifier": "WORK-30", "enabled": True, "condition": {"type": "other_issue", "identifier": "WORK-21", "state": "done"}},
            {"issue_identifier": "WORK-31", "enabled": False, "condition": {"type": "other_issue", "identifier": "WORK-21", "state": "done"}},
        ]
        ready, blocked = ranked(prs, issues, edges_of(prs, holds={(1, 2), (1, 3)}), wakeups)
        self.assertEqual([(u["prs"], u["downstream"]) for u in ready], [([1, 2], ["WORK-22", "WORK-30", "WORK-41"])])
        self.assertEqual([(u["prs"], u["downstream"]) for u in blocked], [([3], [])])

    def test_ties_go_by_board_order_then_pull_request_number(self):
        prs = [pr(4), pr(5, issues=["WORK-5"]), pr(6, issues=["WORK-6"]), pr(8, issues=["WORK-8"])]
        issues = [issue("WORK-5", position=10), issue("WORK-6", position=-3), issue("WORK-8", position=-3)]
        self.assertEqual([u["prs"] for u in ranked(prs, issues)[0]], [[6], [8], [5], [4]])


class BlockedUnitsWaitApartFromReadyOnes(unittest.TestCase):
    def test_a_unit_waiting_on_fixes_its_builder_or_other_work_is_blocked_while_another_is_ready(self):
        prs = [pr(1, issues=["WORK-1"]), pr(2, issues=["WORK-2"]), pr(3, issues=["WORK-3"]), pr(4, issues=["WORK-4"])]
        issues = [issue("WORK-1"), issue("WORK-50", parent="WORK-1", assignee="agent-joeri"),
                  issue("WORK-51", "done", parent="WORK-1"), issue("WORK-2", assignee="agent-joeri"),
                  issue("WORK-3", blocked_by=["WORK-60"]), issue("WORK-60", "todo", assignee=None),
                  issue("WORK-4")]
        ready, blocked = ranked(prs, issues)
        self.assertEqual([u["prs"] for u in ready], [[4]])
        self.assertEqual({u["prs"][0]: u["blocked_by"] for u in blocked}, {
            1: [{"kind": "child", "key": "WORK-50", "status": "in_progress"}],
            2: [{"kind": "assignee", "key": "WORK-2", "assignee": {"type": "agent", "id": "agent-joeri"}}],
            3: [{"kind": "blocked_by", "key": "WORK-60", "status": "todo"}],
        })

    def test_what_a_member_waits_on_inside_its_own_unit_blocks_nothing(self):
        prs = [pr(1, issues=["WORK-41"]), pr(2, "Stacked on #1.", issues=["WORK-42"])]
        issues = [issue("WORK-41", parent="WORK-40", stage=1), issue("WORK-42", parent="WORK-40", stage=2)]
        ready, blocked = ranked(prs, issues, edges_of(prs, holds={(1, 2)}))
        self.assertEqual(([u["prs"] for u in ready], blocked), ([[1, 2]], []))


class MergeWaitsLeaveLaneAdvanceAlone(unittest.TestCase):
    def test_an_open_pull_request_releases_the_lane_but_still_holds_the_merge(self):
        # Given WORK-5 is named by WORK-6, whose pull request is open and handed to Sarkout
        rows = [row("WORK-6", "in_progress", assignee=VERIFIER), row("WORK-5", blocked_by=("WORK-6",))]
        known = {r["key"]: r for r in rows}
        self.assertEqual(head(rows, known), {"active": None, "next": ("WORK-5", [])})
        self.assertEqual(team_state.waits(known, [])["WORK-5"],
                         [{"kind": "blocked_by", "key": "WORK-6", "status": "in_progress"}])


def pr_node(number, title, branch, head, body=""):
    empty = {"pageInfo": {"hasNextPage": False, "endCursor": None}, "nodes": []}
    return {"id": f"PR_{number}", "number": number, "url": f"https://github.com/acme/app/pull/{number}", "title": title,
            "body": body, "baseRefName": "main", "headRefName": branch, "headRefOid": head,
            "assignees": {"nodes": []}, "commits": {"nodes": []},
            "comments": empty, "reviews": empty, "reviewThreads": empty, "files": empty}


def multica_issue(key, status, position, assignee=("agent", "builder"), project="p1", blocked_by=None):
    return {"id": f"id-{key}", "identifier": key, "title": f"Title of {key}", "type": "story", "status": status,
            "assignee_type": assignee[0], "assignee_id": assignee[1], "parent_issue_id": None, "stage": None,
            "metadata": {"blocked_by": blocked_by} if blocked_by else {}, "position": position, "project_id": project}


class RepositoryIntakeReadsEveryOpenPullRequest(unittest.TestCase):
    """An unregistered repository with six open pull requests on three pages of two: a stack of two, an issue-less one,
    one closing an issue of another project held by a person, one only an issue's manual link names, and the named one."""

    NODES = [
        pr_node(1, "Session model (WORK-1057)", "work-1057", "c2"),
        pr_node(2, "Agent sessions (WORK-1087)", "work-1087", "c3", "Stacked on #1 (`work-1057`). Merge that first."),
        pr_node(3, "Fix build", "update-123x", "x1"),
        pr_node(4, "Fix typo", "typo", "x2", "Closes WORK-1200"),
        pr_node(5, "Later step (WORK-1173)", "work-1173", "x3"),
        pr_node(6, "Polish docs", "docs", "x4"),
    ]
    ISSUES = [
        multica_issue("WORK-1173", "in_progress", 2, assignee=("agent", VERIFIER)),
        multica_issue("WORK-1057", "in_progress", 5, assignee=("agent", VERIFIER)),
        multica_issue("WORK-1087", "in_progress", 6, assignee=("agent", VERIFIER), project="p2"),
        multica_issue("WORK-1200", "in_progress", 9, assignee=("member", "brian"), project="p3"),
        multica_issue("WORK-1090", "todo", 20, blocked_by="WORK-1087"),
        multica_issue("WORK-1091", "todo", 21),
        multica_issue("WORK-1300", "in_progress", 30, assignee=("member", "brian"), project="p4"),
        multica_issue("WORK-1001", "done", 1),
    ]
    LINKS = {"WORK-1087": [{"html_url": "https://github.com/acme/app/pull/2", "link_source": "title"}],
             "WORK-1200": [{"html_url": "https://github.com/acme/app/pull/4", "link_source": "auto"}],
             "WORK-1300": [{"html_url": "https://github.com/acme/api/pull/6", "link_source": "manual"},
                           {"html_url": "https://github.com/acme/app/pull/6", "link_source": "manual"}],
             "WORK-1001": [{"html_url": "https://github.com/acme/app/pull/3", "link_source": "manual"}]}
    WAKEUPS = {"WORK-1091": [
        {"enabled": True, "condition": {"type": "other_issue", "identifier": "WORK-1173", "state": "done"}},
        {"enabled": False, "condition": {"type": "other_issue", "identifier": "WORK-1300", "state": "done"}},
    ]}

    def graphql(self, query, **variables):
        if "pullRequests(states: OPEN" in query:
            start = int(variables["after"] or 0)
            return {"repository": {
                "defaultBranchRef": {"name": "main", "target": {"oid": "m1", "statusCheckRollup": None}},
                "pullRequests": {"pageInfo": {"hasNextPage": start + 2 < len(self.NODES), "endCursor": str(start + 2)},
                                 "nodes": self.NODES[start:start + 2]},
            }}
        raise team_state.StateError(f"unexpected query: {query[:60]}")

    def multica(self, *args):
        if args[:2] == ("issue", "list"):
            offset = int(args[-1])
            return {"issues": self.ISSUES[offset:offset + 4], "has_more": offset + 4 < len(self.ISSUES)}
        if args[:2] == ("issue", "pull-requests"):
            self.links_read.append(args[2])
            return {"pull_requests": self.LINKS.get(args[2], [])}
        if args[:3] == ("issue", "wakeup", "list"):
            self.wakeups_read.append(args[3])
            return self.WAKEUPS.get(args[3], [])
        if args[:2] == ("agent", "list"):
            return [{"id": "agent-joeri", "name": "Joeri"}, {"id": VERIFIER, "name": "Verifier"}]
        raise AssertionError(f"intake read multica {' '.join(args)}")

    def setUp(self):
        self.links_read, self.wakeups_read = [], []
        fakes = {"graphql": self.graphql, "multica": self.multica, "compare": compare}
        for name, fake in fakes.items():
            self.enterContext(unittest.mock.patch.object(team_state, name, side_effect=fake))
        self.enterContext(unittest.mock.patch.object(team_state, "collect_messages",
                                                     side_effect=AssertionError("intake read chats")))

    def test_every_pull_request_is_listed_with_its_issues_whoever_holds_them(self):
        found = team_state.collect_intake("acme/app", "Verifier")
        self.assertEqual([p["number"] for p in found["prs"]], [1, 2, 3, 4, 5, 6])
        self.assertEqual({p["number"]: p["links"] for p in found["prs"]}, {
            1: [{"key": "WORK-1057", "source": "claimed"}],
            2: [{"key": "WORK-1087", "source": "title"}],
            3: [],
            4: [{"key": "WORK-1200", "source": "auto"}],
            5: [{"key": "WORK-1173", "source": "claimed"}],
            6: [{"key": "WORK-1300", "source": "manual"}],
        })
        self.assertEqual(found["issueless"], [3])
        self.assertEqual(found["issues"]["WORK-1300"]["assignee"], {"type": "member", "id": "brian"})
        self.assertEqual(found["issues"]["WORK-1200"]["assignee"], {"type": "member", "id": "brian"})
        self.assertEqual((found["prs"][1]["base"], found["default_head"], found["checks"], found["prs"][1]["checks"]),
                         ("main", "m1", "unknown", "unknown"))
        self.assertNotIn("errors", found)

    def test_links_and_wakeups_are_read_per_open_issue_and_closed_ones_only_when_claimed(self):
        team_state.collect_intake("acme/app", "Verifier")
        open_keys = ["WORK-1057", "WORK-1087", "WORK-1090", "WORK-1091", "WORK-1173", "WORK-1200", "WORK-1300"]
        self.assertEqual(sorted(self.links_read, key=team_state.key_order), open_keys)
        self.assertEqual(sorted(self.wakeups_read, key=team_state.key_order), open_keys)

    def test_the_stack_is_one_unit_and_units_come_most_unblocking_first(self):
        found = team_state.collect_intake("acme/app", "Verifier")
        self.assertEqual([(e["upper"], e["lower"], e["status"]) for e in found["edges"]], [(2, 1, "verified")])
        self.assertEqual([(u["prs"], u["downstream"]) for u in found["ready"]],
                         [([5], ["WORK-1091"]), ([1, 2], ["WORK-1090"]), ([3], [])])
        self.assertEqual({u["prs"][0]: u["blocked_by"] for u in found["blocked"]}, {
            4: [{"kind": "assignee", "key": "WORK-1200", "assignee": {"type": "member", "id": "brian"}}],
            6: [{"kind": "assignee", "key": "WORK-1300", "assignee": {"type": "member", "id": "brian"}}],
        })
        self.assertEqual(found["unresolved"], [])
        self.assertEqual(found["ready"][1]["tip_head"], "c3")

    def test_the_command_prints_the_intake_and_refuses_lane_options(self):
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            self.assertEqual(team_state.main(["--intake", "Acme/App", "--verifier", "Verifier"]), 0)
        self.assertEqual(json.loads(out.getvalue())["repository"], "acme/app")
        with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit):
            team_state.main(["--intake", "acme/app", "--verifier", "Verifier", "--lane", "main"])

    def test_the_endeavour_collection_keeps_its_pull_request_fields(self):
        prs = team_state.collect_github("acme/app")["prs"]
        self.assertEqual(len(prs), 6)
        self.assertNotIn("body", prs[0])
        self.assertIsNone(prs[0]["checks"])

    def test_unread_links_hold_every_unit_and_show_the_error(self):
        def failing(*args):
            if args[:3] == ("issue", "pull-requests", "WORK-1300"):
                raise team_state.StateError("multica: timeout")
            return self.multica(*args)

        self.enterContext(unittest.mock.patch.object(team_state, "multica", side_effect=failing))
        found = team_state.collect_intake("acme/app", "Verifier")
        self.assertEqual(found["errors"], ["Multica links: WORK-1300: multica: timeout"])
        self.assertEqual(found["ready"], [])
        self.assertTrue(all(u["blocked_by"][0] == {"kind": "evidence", "missing": ["Multica links"]}
                            for u in found["blocked"]))
        self.assertEqual(found["issueless"], [3, 6])

    def test_unread_wakeups_hold_every_unit_and_show_the_error(self):
        def failing(*args):
            if args[:4] == ("issue", "wakeup", "list", "WORK-1091"):
                raise team_state.StateError("multica: timeout")
            return self.multica(*args)

        self.enterContext(unittest.mock.patch.object(team_state, "multica", side_effect=failing))
        found = team_state.collect_intake("acme/app", "Verifier")
        self.assertEqual(found["errors"], ["Multica wakeups: WORK-1091: multica: timeout"])
        self.assertEqual(found["ready"], [])
        self.assertTrue(all(u["blocked_by"][0] == {"kind": "evidence", "missing": ["Multica wakeups"]}
                            for u in found["blocked"]))

    def test_unread_issues_keep_the_pull_requests_but_hold_every_unit(self):
        def failing(*args):
            if args[:2] == ("issue", "list"):
                raise team_state.StateError("multica: unauthorized")
            return self.multica(*args)

        self.enterContext(unittest.mock.patch.object(team_state, "multica", side_effect=failing))
        found = team_state.collect_intake("acme/app", "Verifier")
        self.assertEqual([p["number"] for p in found["prs"]], [1, 2, 3, 4, 5, 6])
        self.assertEqual(found["prs"][1]["links"], [{"key": "WORK-1087", "source": "claimed"}])
        self.assertEqual((self.links_read, self.wakeups_read), ([], []))
        self.assertEqual(found["ready"], [])
        self.assertEqual({json.dumps(u["blocked_by"][0]) for u in found["blocked"]},
                         {json.dumps({"kind": "evidence", "missing": ["Multica issues"]})})
        self.assertEqual(found["errors"], ["Multica issues: multica: unauthorized"])

    def test_unread_agents_hold_every_unit_and_show_the_error(self):
        def failing(*args):
            if args[:2] == ("agent", "list"):
                raise team_state.StateError("multica: timeout")
            return self.multica(*args)

        self.enterContext(unittest.mock.patch.object(team_state, "multica", side_effect=failing))
        found = team_state.collect_intake("acme/app", "Verifier")
        self.assertEqual(found["errors"], ["Multica agents: multica: timeout"])
        self.assertEqual(found["ready"], [])
        self.assertTrue(all(u["blocked_by"][0] == {"kind": "evidence", "missing": ["Multica agents"]}
                            for u in found["blocked"]))


if __name__ == "__main__":
    unittest.main()
