import contextlib
import io
import json
import subprocess
import sys
import tempfile
import unittest
import unittest.mock
from pathlib import Path

SCRIPTS = Path(__file__).resolve().parents[2] / "server/internal/service/builtin_skills/multica-platform/scripts"
sys.path.insert(0, str(SCRIPTS))
import team_state  # noqa: E402
import team_state_chats  # noqa: E402
import team_state_intake  # noqa: E402
import team_state_io  # noqa: E402
import team_state_multica  # noqa: E402
import team_state_repos  # noqa: E402
import team_state_review  # noqa: E402

SCRIPT_MODULES = (team_state, team_state_chats, team_state_intake, team_state_io, team_state_multica, team_state_repos,
                  team_state_review)


def stand_in(test, name, fake):
    """Replace `name` with `fake` in every script module that calls it."""
    for module in SCRIPT_MODULES:
        if hasattr(module, name):
            test.enterContext(unittest.mock.patch.object(module, name, side_effect=fake))


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
    found = team_state_multica.lane_head(rows, known, VERIFIER)
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
        self.assertEqual(team_state_multica.lane_head(rows, {"WORK-1": rows[0]}, None)["active"]["key"], "WORK-1")

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
        found = team_state_multica.lane_head(steps, {r["key"]: r for r in steps}, VERIFIER)
        self.assertEqual(found["next"]["key"], "WORK-12")
        self.assertEqual(found["next"]["waits_on"], [])
        self.assertEqual(
            team_state_multica.waits_on(steps[2], steps[:2], {r["key"]: r for r in steps}, VERIFIER),
            [{"kind": "step", "key": "WORK-12", "status": "todo"}],
        )

    def test_a_client_wait_ahead_in_the_same_release_holds_the_work_behind_it(self):
        nasir, sam = {"type": "member", "id": "user-nasir"}, {"type": "member", "id": "user-sam"}
        rows = [row("WORK-20"), row("WORK-21", "done"), row("WORK-22", release="0.2.0"), row("WORK-24"), row("WORK-23")]
        for r in rows[:3]:
            r["assignee"] = nasir
        rows[3]["assignee"] = sam
        clients = frozenset({"user-nasir"})
        waits = team_state_multica.waits_on(rows[4], rows[:4], {r["key"]: r for r in rows}, VERIFIER, clients)
        self.assertEqual([w["key"] for w in waits], ["WORK-20"])
        # And the client wait itself waits on no other client wait
        self.assertEqual(team_state_multica.waits_on(rows[2], rows[:2], {}, VERIFIER, clients), [])


class IssuesAreReadAsMulticaHoldsThem(unittest.TestCase):
    def test_labels_points_parent_release_and_blocked_by_are_read(self):
        issue = {
            "id": "i2", "identifier": "WORK-2", "title": "Step", "type": "story", "status": "todo",
            "properties": {"p1": 5}, "due_date": None, "release_id": "r1", "parent_issue_id": "i1", "stage": 2,
            "assignee_type": "agent", "assignee_id": "a1", "metadata": {"blocked_by": "work-7, WORK-8"},
        }
        found = team_state_multica.issue_row(issue, {"i2": ["hitl"]}, "p1", {"i1": "WORK-1"}, {"r1": "0.3.0"})
        self.assertEqual(
            {k: found[k] for k in ("labels", "points", "parent", "release", "assignee", "blocked_by")},
            {"labels": ["hitl"], "points": 5, "parent": "WORK-1", "release": "0.3.0",
             "assignee": {"type": "agent", "id": "a1"}, "blocked_by": ["WORK-7", "WORK-8"]},
        )

    def test_a_project_s_issues_are_read_page_after_page(self):
        pages = [{"issues": [{"identifier": "WORK-1"}], "has_more": True}, {"issues": [{"identifier": "WORK-2"}], "has_more": False}]
        with unittest.mock.patch.object(team_state_multica, "multica", side_effect=pages) as called:
            self.assertEqual([i["identifier"] for i in team_state_multica.project_issues("p")], ["WORK-1", "WORK-2"])
        self.assertEqual(called.call_args_list[1].args[-1], "1")

    def test_a_failed_multica_command_stops_with_its_error(self):
        with unittest.mock.patch.object(team_state_io.subprocess, "run",
                                        return_value=subprocess.CompletedProcess([], 1, "", "unauthorized")):
            with self.assertRaisesRegex(team_state_io.StateError, "unauthorized"):
                team_state_io.multica("repo", "list")


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
        return {c["name"]: (c["archived"], c["build"]) for c in team_state_repos.collect_changes(self.repo, "main")}

    def test_a_branch_reports_its_open_changes_and_what_it_archived_that_main_has_not(self):
        self.assertEqual(self.found(), {"coupon": (False, {"status": "in-progress", "assignee": "ann"}), "receipt": (True, {})})

    def test_a_change_whose_build_map_cannot_be_read_keeps_its_error_and_the_others_are_read(self):
        (self.repo / "openspec" / "changes" / "broken").mkdir()
        (self.repo / "openspec" / "changes" / "broken" / ".openspec.yaml").write_text("x-build: [\n", encoding="utf-8")
        found = {c["name"]: c for c in team_state_repos.collect_changes(self.repo, "main")}
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
        self.assertEqual(team_state_repos.repo_home(tree), team_state_repos.repo_home(self.repo))


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
        self.assertEqual(team_state_repos.local_checkouts(self.repos), expected)

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
        self.assertEqual(team_state_repos.local_checkouts(self.repos), expected)

    def test_a_github_checkout_hides_the_git_folders_inside_it(self):
        # Given a GitHub checkout that holds a vendored repository of its own
        self.checkout("Apps/shop", "git@github.com:acme/shop.git")
        self.checkout("Apps/shop/vendor/lib", "git@github.com:other/lib.git")
        # When the checkouts are looked up, then only the outer checkout is found
        self.assertEqual(team_state_repos.local_checkouts(self.repos), {"acme/shop": [self.repos / "Apps/shop"]})

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
        self.assertEqual(team_state_repos.change_names(paths), ["cart-copy", "web-cart"])

    def test_the_title_and_branch_name_their_issue_keys(self):
        self.assertEqual(team_state_repos.issue_keys("Shop discounts (WORK-697)", "work-697"), ["WORK-697"])
        self.assertEqual(team_state_repos.issue_keys("Fix build", "update-123x"), [])


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
        clients = team_state_chats.endeavour_clients(root / "rebl", ["rodi"])
        self.assertEqual(clients, ["cem", "rodi"])
        found = team_state_chats.client_chats(clients, root)
        self.assertEqual(found[0], ("cem", ["!group-cem:ba.local-whatsapp.localhost", "!dm-cem:ba.local-whatsapp.localhost"]))
        self.assertEqual(found[1]["client"], "rodi")


def pr(number, body="", base="main", issues=(), branch=None):
    """An open pull request as the intake reads it."""
    return {"number": number, "branch": branch or f"work-{number}", "head": f"h{number}", "base": base, "body": body,
            "issues": list(issues), "stack_note": team_state_intake.stack_note(body, "acme/app")}


def edges_of(prs, holds=(), forked=(), gone=None):
    """The edges with ancestry as given: `holds` and `forked` are the (lower, upper) pairs whose upper holds the lower's
    head or forked from its unmerged work; `gone` maps a pull request that is not open to its state."""
    gone = gone or {}

    def resolve(number=None, branch=None):
        return {"number": number, "state": gone[number]} if number in gone else None

    def ancestry(lower, upper):
        pair = (lower["number"], upper["number"])
        return "contains" if pair in holds else "forked" if pair in forked else None

    return team_state_intake.stack_edges(prs, "main", resolve, ancestry)


def issue(key, status="in_progress", position=0, parent=None, stage=None, blocked_by=(), assignee=VERIFIER):
    return {**row(key, status, parent=parent, stage=stage, blocked_by=blocked_by, assignee=assignee),
            "position": position}


def ranked(prs, issues, edges=(), wakeups=(), missing=()):
    edges = list(edges)
    issues = {i["key"]: i for i in issues}
    waits_of = team_state_intake.waits(issues, list(wakeups))
    found = team_state_intake.rank_prs(prs, issues, VERIFIER, waits_of, edges, missing)
    return [p for p in found if not p["blocked_by"]], [p for p in found if p["blocked_by"]]


class StackNotesNameThePullRequestUnderIt(unittest.TestCase):
    def test_the_first_stacked_on_phrase_names_the_lower_pull_request_and_its_branch(self):
        body = "## Summary\n- Stacked on #66 (`work-1087`, agent-active-sessions), which is stacked on #59. Merge those first."
        note = team_state_intake.stack_note(body, "acme/app")
        self.assertEqual((note["prs"], note["branches"], note["elsewhere"]), ([66], ["work-1087"], []))

    def test_a_link_counts_in_this_repository_and_a_commit_is_no_branch(self):
        note = team_state_intake.stack_note(
            "Stacked on [the data model, PR #59](https://github.com/Acme/App/pull/59), branch `work-1057` at `91085dd6b6`.",
            "acme/app",
        )
        self.assertEqual((note["prs"], note["branches"]), ([59], ["work-1057"]))

    def test_another_repository_s_pull_request_is_a_companion_not_a_lower(self):
        note = team_state_intake.stack_note("Stacked on acme/api#8 and https://github.com/acme/api/pull/7.", "acme/app")
        self.assertEqual((note["prs"], sorted(note["elsewhere"])), ([], ["acme/api#7", "acme/api#8"]))

    def test_a_lower_pull_request_saying_what_stacks_on_it_has_no_note(self):
        for body in ("The frontend stacks on this branch.", "Next: #70, stacked on this one.",
                     "PR #70 is stacked on this branch."):
            with self.subTest(body=body):
                self.assertIsNone(team_state_intake.stack_note(body, "acme/app"))

    def test_upper_prose_does_not_hide_a_later_own_stack_declaration(self):
        body = "Next: #70, stacked on this one.\nStacked on #68."
        self.assertEqual(team_state_intake.stack_note(body, "acme/app")["prs"], [68])


class StackedPullRequestsWaitForTheirLowerWork(unittest.TestCase):
    def test_each_upper_waits_even_when_it_lacks_a_later_lower_fix(self):
        prs = [pr(1), pr(2, "Stacked on #1."), pr(3, "Stacked on #2 (`work-2`).")]
        edges = edges_of(prs, holds={(1, 2)}, forked={(2, 3)})
        self.assertEqual([(e["upper"], e["lower"], e["status"], e.get("ancestry")) for e in edges],
                         [(2, 1, "verified", "contains"), (3, 2, "verified", "forked")])
        ready, blocked = ranked(prs, [], edges)
        self.assertEqual([p["number"] for p in ready], [1])
        self.assertEqual({p["number"]: p["blocked_by"] for p in blocked},
                         {2: [{"kind": "stack", "pr": 1}], 3: [{"kind": "stack", "pr": 2}]})

    def test_a_closed_target_is_not_hidden_by_an_open_lower(self):
        prs = [pr(7), pr(9, "Stacked on #7 and #8.")]
        ready, blocked = ranked(prs, [], edges_of(prs, holds={(7, 9)}, gone={8: "CLOSED"}))
        self.assertEqual([p["number"] for p in ready], [7])
        self.assertEqual(blocked[0]["blocked_by"],
                         [{"kind": "stack_edge", "pr": 9, "lower": 8, "status": "lower_closed"}])

    def test_a_base_on_an_open_branch_also_blocks_the_upper(self):
        prs = [pr(1), pr(2, base="work-1")]
        ready, blocked = ranked(prs, [], edges_of(prs, holds={(1, 2)}))
        self.assertEqual([p["number"] for p in ready], [1])
        self.assertEqual(blocked[0]["blocked_by"], [{"kind": "stack", "pr": 1}])

    def test_siblings_wait_separately_then_become_ready_after_the_lower_merges(self):
        prs = [pr(1), pr(2, "Stacked on #1."), pr(3, "Stacked on #1.")]
        ready, blocked = ranked(prs, [], edges_of(prs, holds={(1, 2), (1, 3)}))
        self.assertEqual([p["number"] for p in ready], [1])
        self.assertEqual([(p["number"], p["blocked_by"]) for p in blocked],
                         [(2, [{"kind": "stack", "pr": 1}]), (3, [{"kind": "stack", "pr": 1}])])
        remaining = prs[1:]
        ready, blocked = ranked(remaining, [], edges_of(remaining, gone={1: "MERGED"}))
        self.assertEqual(([p["number"] for p in ready], blocked), ([2, 3], []))

    def test_merged_lower_and_unproven_open_lower_have_different_readiness(self):
        prs = [pr(1), pr(4, "Stacked on #2."), pr(5, "Stacked on #1.")]
        ready, blocked = ranked(prs, [], edges_of(prs, gone={2: "MERGED"}))
        self.assertEqual([p["number"] for p in ready], [1, 4])
        self.assertEqual(blocked[0]["blocked_by"],
                         [{"kind": "stack_edge", "pr": 5, "lower": 1, "status": "ancestry_unproven"}])

    def test_ambiguous_and_cyclic_relationships_stay_blocked(self):
        prs = [pr(1, "Stacked on #2."), pr(2, "Stacked on #1."), pr(3), pr(4), pr(5, "Stacked on #3 and #4.")]
        ready, blocked = ranked(prs, [], edges_of(prs, holds={(2, 1), (1, 2)}))
        self.assertEqual([p["number"] for p in ready], [3, 4])
        self.assertEqual([(p["number"], p["blocked_by"][0]["status"]) for p in blocked],
                         [(1, "cycle"), (2, "cycle"), (5, "ambiguous")])

    def test_another_repository_is_a_companion_not_a_lower_pr(self):
        prs = [pr(1, "Stacked on acme/api#8.")]
        self.assertEqual([p["number"] for p in ranked(prs, [], edges_of(prs))[0]], [1])


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
        with unittest.mock.patch.object(team_state_intake, "compare", side_effect=compare):
            return team_state_intake.ancestry("acme/app", "m1", {"head": lower}, {"head": upper})

    def test_an_upper_holding_the_lower_head_contains_it(self):
        self.assertEqual(self.ancestry("c2", "c3"), "contains")

    def test_an_upper_cut_before_a_lower_fix_forked_from_its_unmerged_work(self):
        self.assertEqual(self.ancestry("c5", "c3"), "forked")

    def test_sharing_only_the_default_branch_proves_no_stack(self):
        self.assertIsNone(self.ancestry("c5", "x1"))


class ReadyPullRequestsComeMostUnblockingFirst(unittest.TestCase):
    def test_issue_dependencies_continue_through_their_prs_into_upper_stacks(self):
        prs = [pr(1, issues=["WORK-1"]), pr(2, issues=["WORK-2"]),
               pr(3, "Stacked on #2.", issues=["WORK-3"]),
               pr(4, "Stacked on #3.", issues=["WORK-4"]), pr(9, issues=["WORK-9"])]
        issues = [issue(f"WORK-{n}") for n in (1, 3, 4, 9)]
        issues += [issue("WORK-2", blocked_by=["WORK-1"]),
                   issue("WORK-10", blocked_by=["WORK-9"]), issue("WORK-11", blocked_by=["WORK-9"])]
        ready, blocked = ranked(prs, issues, edges_of(prs, holds={(2, 3), (3, 4)}))
        self.assertEqual([p["number"] for p in ready], [1, 9])
        self.assertEqual(ready[0]["downstream"], ["WORK-2", "WORK-3", "WORK-4"])
        self.assertEqual({p["number"] for p in blocked}, {2, 3, 4})

    def test_distinct_open_dependents_count_once_transitively(self):
        prs = [pr(3, issues=["WORK-2"]), pr(7, issues=["WORK-1"])]
        issues = [issue("WORK-1"), issue("WORK-2"), issue("WORK-10", "todo", blocked_by=["WORK-1"]),
                  issue("WORK-11", "backlog", blocked_by=["WORK-1"]),
                  issue("WORK-12", "todo", blocked_by=["WORK-10", "WORK-11"]),
                  issue("WORK-13", "todo", blocked_by=["WORK-2"]), issue("WORK-14", "done", blocked_by=["WORK-2"])]
        ready, blocked = ranked(prs, issues)
        self.assertEqual([(p["number"], p["downstream"]) for p in ready],
                         [(7, ["WORK-10", "WORK-11", "WORK-12"]), (3, ["WORK-13"])])
        self.assertEqual((ready[0]["downstream_count"], blocked), (3, []))

    def test_steps_enabled_wakeups_and_upper_prs_contribute_downstream_issues(self):
        prs = [pr(1, issues=["WORK-21"]), pr(2, "Stacked on #1.", issues=["WORK-40"]),
               pr(3, "Stacked on #1.", issues=["WORK-41"])]
        issues = [issue("WORK-21", parent="WORK-20", stage=1), issue("WORK-22", "todo", parent="WORK-20", stage=2),
                  issue("WORK-30", "blocked"), issue("WORK-31", "todo"), issue("WORK-40"), issue("WORK-41")]
        wakeups = [
            {"issue_identifier": "WORK-30", "enabled": True, "condition": {"type": "other_issue", "identifier": "WORK-21", "state": "done"}},
            {"issue_identifier": "WORK-31", "enabled": False, "condition": {"type": "other_issue", "identifier": "WORK-21", "state": "done"}},
        ]
        ready, blocked = ranked(prs, issues, edges_of(prs, holds={(1, 2), (1, 3)}), wakeups)
        self.assertEqual([(p["number"], p["downstream"]) for p in ready],
                         [(1, ["WORK-22", "WORK-30", "WORK-40", "WORK-41"])])
        self.assertEqual([p["number"] for p in blocked], [2, 3])

    def test_ties_go_by_board_order_then_pull_request_number(self):
        prs = [pr(4), pr(5, issues=["WORK-5"]), pr(6, issues=["WORK-6"]), pr(8, issues=["WORK-8"])]
        issues = [issue("WORK-5", position=10), issue("WORK-6", position=-3), issue("WORK-8", position=-3)]
        self.assertEqual([p["number"] for p in ranked(prs, issues)[0]], [6, 8, 5, 4])

    def test_a_shared_dependent_counts_once_across_siblings(self):
        prs = [pr(1, issues=["WORK-1"]), pr(2, "Stacked on #1.", issues=["WORK-2"]),
               pr(3, "Stacked on #1.", issues=["WORK-3"])]
        issues = [issue("WORK-1"), issue("WORK-2"), issue("WORK-3"),
                  issue("WORK-4", "todo", blocked_by=["WORK-2", "WORK-3"])]
        ready, blocked = ranked(prs, issues, edges_of(prs, holds={(1, 2), (1, 3)}))
        self.assertEqual(ready[0]["downstream"], ["WORK-2", "WORK-3", "WORK-4"])
        self.assertEqual([(p["number"], p["downstream_count"]) for p in blocked], [(2, 1), (3, 1)])


class BlockedPullRequestsWaitApartFromReadyOnes(unittest.TestCase):
    def test_fixes_assignment_and_dependencies_hold_only_their_pr(self):
        prs = [pr(1, issues=["WORK-1"]), pr(2, issues=["WORK-2"]), pr(3, issues=["WORK-3"]), pr(4, issues=["WORK-4"])]
        issues = [issue("WORK-1"), issue("WORK-50", parent="WORK-1", assignee="agent-joeri"),
                  issue("WORK-51", "done", parent="WORK-1"), issue("WORK-2", assignee="agent-joeri"),
                  issue("WORK-3", blocked_by=["WORK-60"]), issue("WORK-60", "todo", assignee=None), issue("WORK-4")]
        ready, blocked = ranked(prs, issues)
        self.assertEqual([p["number"] for p in ready], [4])
        self.assertEqual({p["number"]: p["blocked_by"] for p in blocked}, {
            1: [{"kind": "child", "key": "WORK-50", "status": "in_progress"}],
            2: [{"kind": "assignee", "key": "WORK-2", "assignee": {"type": "agent", "id": "agent-joeri"}}],
            3: [{"kind": "blocked_by", "key": "WORK-60", "status": "todo"}],
        })

    def test_an_open_lower_step_still_blocks_the_upper_step(self):
        prs = [pr(1, issues=["WORK-41"]), pr(2, "Stacked on #1.", issues=["WORK-42"])]
        issues = [issue("WORK-41", parent="WORK-40", stage=1), issue("WORK-42", parent="WORK-40", stage=2)]
        ready, blocked = ranked(prs, issues, edges_of(prs, holds={(1, 2)}))
        self.assertEqual([p["number"] for p in ready], [1])
        self.assertEqual(blocked[0]["blocked_by"],
                         [{"kind": "stack", "pr": 1}, {"kind": "step", "key": "WORK-41", "status": "in_progress"}])


def workcycle(key, status="in_progress"):
    return {**issue(key, status, assignee="squad-development"), "type": "workcycle"}


class AReleaseIsReviewedBeforeItsWorkcycleChecksOut(unittest.TestCase):
    # Given workcycle WORK-70, owned by Development, holds release task WORK-71 and unrelated open releases WORK-72/73
    ISSUES = [workcycle("WORK-70"), issue("WORK-71", "blocked", parent="WORK-70"),
              issue("WORK-72", "backlog", parent="WORK-70", assignee=None),
              issue("WORK-73", "in_review", parent="WORK-70", assignee="agent-joeri")]

    def test_the_release_task_alone_decides_readiness_and_the_workcycle_stays_linked(self):
        ready, blocked = ranked([pr(11, issues=["WORK-71", "WORK-70"], branch="work-70")], self.ISSUES)
        self.assertEqual(([p["number"] for p in ready], blocked), ([11], []))
        self.assertEqual(ready[0]["issues"], ["WORK-71", "WORK-70"])

    def test_the_release_task_s_own_prerequisites_and_fixes_still_block(self):
        issues = [*self.ISSUES, issue("WORK-80", "todo", assignee=None),
                  issue("WORK-81", parent="WORK-71", assignee="agent-joeri")]
        issues[1] = {**issues[1], "blocked_by": ["WORK-80"]}
        _, blocked = ranked([pr(11, issues=["WORK-71", "WORK-70"])], issues)
        self.assertEqual(blocked[0]["blocked_by"], [{"kind": "blocked_by", "key": "WORK-80", "status": "todo"},
                                                    {"kind": "child", "key": "WORK-81", "status": "in_progress"}])

    def test_missing_evidence_still_blocks_the_release(self):
        _, blocked = ranked([pr(11, issues=["WORK-71", "WORK-70"])], self.ISSUES, missing=("Multica wakeups",))
        self.assertEqual(blocked[0]["blocked_by"], [{"kind": "evidence", "missing": ["Multica wakeups"]}])

    def test_a_workcycle_claimed_without_its_release_task_keeps_its_checks(self):
        _, blocked = ranked([pr(12, issues=["WORK-70"])], self.ISSUES)
        self.assertEqual(blocked[0]["blocked_by"], [
            {"kind": "assignee", "key": "WORK-70", "assignee": {"type": "agent", "id": "squad-development"}},
            {"kind": "child", "key": "WORK-71", "status": "blocked"},
            {"kind": "child", "key": "WORK-72", "status": "backlog"},
            {"kind": "child", "key": "WORK-73", "status": "in_review"},
        ])


class MergeWaitsLeaveLaneAdvanceAlone(unittest.TestCase):
    def test_an_open_pull_request_releases_the_lane_but_still_holds_the_merge(self):
        # Given WORK-5 is named by WORK-6, whose pull request is open and handed to Sarkout
        rows = [row("WORK-6", "in_progress", assignee=VERIFIER), row("WORK-5", blocked_by=("WORK-6",))]
        known = {r["key"]: r for r in rows}
        self.assertEqual(head(rows, known), {"active": None, "next": ("WORK-5", [])})
        self.assertEqual(team_state_intake.waits(known, [])["WORK-5"],
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
    """Six paginated PRs with a stack, a closed issue's manual link and cross-project assignments."""

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
        raise team_state_io.StateError(f"unexpected query: {query[:60]}")

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
            stand_in(self, name, fake)
        self.enterContext(unittest.mock.patch.object(team_state, "collect_messages",
                                                     side_effect=AssertionError("intake read chats")))

    def test_every_pull_request_is_listed_with_its_issues_whoever_holds_them(self):
        found = team_state_intake.collect_intake("acme/app", "Verifier")
        self.assertEqual([p["number"] for p in found["prs"]], [1, 5, 2, 3, 4, 6])
        self.assertEqual({p["number"]: p["links"] for p in found["prs"]}, {
            1: [{"key": "WORK-1057", "source": "claimed"}],
            2: [{"key": "WORK-1087", "source": "title"}],
            3: [{"key": "WORK-1001", "source": "manual"}],
            4: [{"key": "WORK-1200", "source": "auto"}],
            5: [{"key": "WORK-1173", "source": "claimed"}],
            6: [{"key": "WORK-1300", "source": "manual"}],
        })
        self.assertEqual(found["issueless"], [])
        self.assertEqual(found["issues"]["WORK-1001"]["status"], "done")
        self.assertEqual(found["issues"]["WORK-1300"]["assignee"], {"type": "member", "id": "brian"})
        self.assertEqual(found["issues"]["WORK-1200"]["assignee"], {"type": "member", "id": "brian"})
        self.assertEqual((found["prs"][1]["base"], found["default_head"], found["checks"], found["prs"][1]["checks"]),
                         ("main", "m1", "unknown", "unknown"))
        self.assertNotIn("errors", found)

    def test_closed_issue_links_are_read_while_wakeups_only_need_open_issues(self):
        team_state_intake.collect_intake("acme/app", "Verifier")
        open_keys = ["WORK-1057", "WORK-1087", "WORK-1090", "WORK-1091", "WORK-1173", "WORK-1200", "WORK-1300"]
        self.assertEqual(sorted(self.links_read, key=team_state_intake.key_order), ["WORK-1001", *open_keys])
        self.assertEqual(sorted(self.wakeups_read, key=team_state_intake.key_order), open_keys)

    def test_individual_prs_are_ranked_and_the_upper_waits_on_its_lower(self):
        found = team_state_intake.collect_intake("acme/app", "Verifier")
        self.assertEqual([(e["upper"], e["lower"], e["status"]) for e in found["edges"]], [(2, 1, "verified")])
        self.assertEqual([(p["number"], p["downstream"]) for p in found["ready"]],
                         [(1, ["WORK-1087", "WORK-1090"]), (5, ["WORK-1091"]), (3, [])])
        self.assertEqual({p["number"]: p["blocked_by"] for p in found["blocked"]}, {
            2: [{"kind": "stack", "pr": 1}],
            4: [{"kind": "assignee", "key": "WORK-1200", "assignee": {"type": "member", "id": "brian"}}],
            6: [{"kind": "assignee", "key": "WORK-1300", "assignee": {"type": "member", "id": "brian"}}],
        })
        self.assertEqual(found["unresolved"], [])
        self.assertEqual(found["ready"][0]["head"], "c2")

    def test_the_command_prints_the_intake_and_refuses_lane_options(self):
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            self.assertEqual(team_state.main(["--intake", "Acme/App", "--verifier", "Verifier"]), 0)
        self.assertEqual(json.loads(out.getvalue())["repository"], "acme/app")
        with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit):
            team_state.main(["--intake", "acme/app", "--verifier", "Verifier", "--lane", "main"])

    def test_the_endeavour_collection_keeps_its_pull_request_fields(self):
        prs = team_state_repos.collect_github("acme/app")["prs"]
        self.assertEqual(len(prs), 6)
        self.assertNotIn("body", prs[0])
        self.assertIsNone(prs[0]["checks"])

    def test_failed_check_rollups_do_not_block_review_eligibility(self):
        original = self.graphql

        def failing_checks(query, **variables):
            result = original(query, **variables)
            result["repository"]["defaultBranchRef"]["target"]["statusCheckRollup"] = {"state": "FAILURE"}
            return result

        stand_in(self, "graphql", failing_checks)
        found = team_state_intake.collect_intake("acme/app", "Verifier")
        self.assertEqual(found["checks"], "FAILURE")
        self.assertEqual([p["number"] for p in found["ready"]], [1, 5, 3])

    def test_missing_claims_stay_linked_and_block_only_their_pr(self):
        self.NODES = [pr_node(9, "Fix request (WORK-9999)", "work-9999", "x9"),
                      pr_node(3, "Independent maintenance", "maintenance", "x1")]
        self.LINKS = {}
        found = team_state_intake.collect_intake("acme/app", "Verifier")
        self.assertEqual([p["number"] for p in found["ready"]], [3])
        missing = found["blocked"][0]
        self.assertEqual((missing["number"], missing["issues"]), (9, ["WORK-9999"]))
        self.assertEqual(missing["links"], [{"key": "WORK-9999", "source": "claimed"}])
        self.assertEqual(missing["blocked_by"], [{"kind": "evidence", "missing": ["Multica issue WORK-9999"]}])
        self.assertEqual(found["errors"], ["Multica issues: missing WORK-9999"])
        self.assertEqual(found["issueless"], [3])

    def test_unknown_verifier_blocks_even_genuinely_issueless_prs(self):
        self.LINKS = {k: v for k, v in self.LINKS.items() if k != "WORK-1001"}
        found = team_state_intake.collect_intake("acme/app", "UnknownVerifier")
        self.assertEqual(found["ready"], [])
        self.assertIn(3, found["issueless"])
        self.assertEqual(found["errors"], ["Multica agents: verifier 'UnknownVerifier' not found"])
        self.assertTrue(all(p["blocked_by"][0] == {"kind": "evidence", "missing": ["Multica agents"]}
                            for p in found["blocked"]))

    def test_genuinely_issueless_prs_remain_ready_with_a_known_verifier(self):
        self.LINKS = {k: v for k, v in self.LINKS.items() if k != "WORK-1001"}
        found = team_state_intake.collect_intake("acme/app", "Verifier")
        self.assertIn(3, [p["number"] for p in found["ready"]])
        self.assertEqual(found["issueless"], [3])

    def test_upper_prose_keeps_the_lower_pr_ready(self):
        self.NODES = [pr_node(69, "Work (WORK-1057)", "work-1057", "c2",
                              "Next: #70, stacked on this one.")]
        found = team_state_intake.collect_intake("acme/app", "Verifier")
        self.assertEqual([p["number"] for p in found["ready"]], [69])
        self.assertEqual(found["edges"], [])

    def test_all_named_stack_targets_must_be_resolved(self):
        for states, expected in [({7: "MERGED", 8: "CLOSED"}, "lower_closed"),
                                 ({7: "MERGED"}, "unknown_target"),
                                 ({7: "MERGED", 8: "MERGED"}, "lower_merged")]:
            with self.subTest(states=states):
                self.NODES = [pr_node(9, "Fix stack", "stack", "x9", "Stacked on #7 and #8.")]
                def resolve(name, **ref):
                    n = ref["number"]
                    return {"number": n, "state": states[n]} if n in states else None
                with unittest.mock.patch.object(team_state_intake, "find_pull_request", side_effect=resolve):
                    found = team_state_intake.collect_intake("acme/app", "Verifier")
                self.assertEqual(found["edges"][0]["status"], expected)
                self.assertEqual([p["number"] for p in found["ready"]], [9] if expected == "lower_merged" else [])
                if expected == "lower_closed":
                    self.assertEqual(found["edges"][0]["lower"], 8)
                    self.assertEqual(found["edges"][0]["candidates"], [7, 8])

    def test_unread_links_hold_every_pr_and_show_the_error(self):
        def failing(*args):
            if args[:3] == ("issue", "pull-requests", "WORK-1300"):
                raise team_state_io.StateError("multica: timeout")
            return self.multica(*args)

        stand_in(self, "multica", failing)
        found = team_state_intake.collect_intake("acme/app", "Verifier")
        self.assertEqual(found["errors"], ["Multica links: WORK-1300: multica: timeout"])
        self.assertEqual(found["ready"], [])
        self.assertTrue(all(u["blocked_by"][0] == {"kind": "evidence", "missing": ["Multica links"]}
                            for u in found["blocked"]))
        self.assertEqual(found["issueless"], [6])

    def test_unread_wakeups_hold_every_pr_and_show_the_error(self):
        def failing(*args):
            if args[:4] == ("issue", "wakeup", "list", "WORK-1091"):
                raise team_state_io.StateError("multica: timeout")
            return self.multica(*args)

        stand_in(self, "multica", failing)
        found = team_state_intake.collect_intake("acme/app", "Verifier")
        self.assertEqual(found["errors"], ["Multica wakeups: WORK-1091: multica: timeout"])
        self.assertEqual(found["ready"], [])
        self.assertTrue(all(u["blocked_by"][0] == {"kind": "evidence", "missing": ["Multica wakeups"]}
                            for u in found["blocked"]))

    def test_unread_issues_keep_the_pull_requests_but_hold_every_pr(self):
        def failing(*args):
            if args[:2] == ("issue", "list"):
                raise team_state_io.StateError("multica: unauthorized")
            return self.multica(*args)

        stand_in(self, "multica", failing)
        found = team_state_intake.collect_intake("acme/app", "Verifier")
        self.assertEqual([p["number"] for p in found["prs"]], [1, 2, 3, 4, 5, 6])
        self.assertEqual(found["prs"][1]["links"], [{"key": "WORK-1087", "source": "claimed"}])
        self.assertEqual((self.links_read, self.wakeups_read), ([], []))
        self.assertEqual(found["ready"], [])
        self.assertEqual({json.dumps(u["blocked_by"][0]) for u in found["blocked"]},
                         {json.dumps({"kind": "evidence", "missing": ["Multica issues"]})})
        self.assertEqual(found["errors"], ["Multica issues: multica: unauthorized"])

    def test_unread_agents_hold_every_pr_and_show_the_error(self):
        def failing(*args):
            if args[:2] == ("agent", "list"):
                raise team_state_io.StateError("multica: timeout")
            return self.multica(*args)

        stand_in(self, "multica", failing)
        found = team_state_intake.collect_intake("acme/app", "Verifier")
        self.assertEqual(found["errors"], ["Multica agents: multica: timeout"])
        self.assertEqual(found["ready"], [])
        self.assertTrue(all(u["blocked_by"][0] == {"kind": "evidence", "missing": ["Multica agents"]}
                            for u in found["blocked"]))


def lane_issue(key, status, lane, project, release=None):
    return {"id": f"id-{key}", "identifier": key, "title": f"Title of {key}", "status": status,
            "project_id": project, "release_id": release, "worklane_id": lane}


def linked_pr(repository, number, state="open"):
    return {"html_url": f"https://github.com/{repository}/pull/{number}", "number": number,
            "branch": f"branch-{number}", "state": state, "link_source": "title"}


class SarkoutSeesEveryIssueInReview(unittest.TestCase):
    """Two projects: the app's lane holds a stack in lane order 12 under 11, the dashboard's lane a stack of 20 under
    21, and the dashboard holds one issue in review in no lane."""

    PROJECTS = [{"id": "p-dash", "title": "Dashboard"}, {"id": "p-app", "title": "App"}]
    RELEASES = [{"id": "r-1", "version": "0.1.0"}, {"id": "r-2", "version": "0.2.0"}]
    LANES = [{"id": "l-app", "name": "main", "repository_id": "repo-app"},
             {"id": "l-dash", "name": "Rooster", "repository_id": "repo-dash"}]
    LANE_ORDER = {
        "l-app": [lane_issue("WORK-10", "done", "l-app", "p-app", "r-1"),
                  lane_issue("WORK-12", "in_review", "l-app", "p-app", "r-1"),
                  lane_issue("WORK-11", "in_review", "l-app", "p-app", "r-2"),
                  lane_issue("WORK-13", "in_progress", "l-app", "p-app", "r-2"),
                  lane_issue("WORK-14", "todo", "l-app", "p-app", "r-2")],
        "l-dash": [lane_issue("WORK-20", "in_review", "l-dash", "p-dash", "r-1"),
                   lane_issue("WORK-21", "in_review", "l-dash", "p-dash", "r-1")],
    }
    UNLANED = [lane_issue("WORK-30", "in_review", None, "p-dash"), lane_issue("WORK-31", "todo", None, "p-dash")]
    LINKS = {"WORK-11": [linked_pr("acme/app", 8)], "WORK-12": [linked_pr("acme/app", 7)],
             "WORK-20": [linked_pr("acme/dash", 3)], "WORK-21": [linked_pr("acme/dash", 4)],
             "WORK-30": [linked_pr("acme/dash", 2, "closed"), linked_pr("acme/dash", 5)]}

    def multica(self, *args):
        if args[:2] == ("issue", "list"):
            status = args[args.index("--status") + 1] if "--status" in args else None
            board = sorted([*self.UNLANED, *(i for lane in self.LANE_ORDER.values() for i in lane)],
                           key=lambda i: i["identifier"], reverse=True)
            matching = [i for i in board if status in (None, i["status"])]
            offset = int(args[-1])
            return {"issues": matching[offset:offset + 2], "has_more": offset + 2 < len(matching)}
        if args[:2] == ("project", "list"):
            return self.PROJECTS
        if args[:2] == ("release", "list"):
            return self.RELEASES
        if args[:2] == ("worklane", "list"):
            return self.LANES
        if args[:2] == ("worklane", "issues"):
            return self.LANE_ORDER[args[2]]
        if args[:2] == ("issue", "pull-requests"):
            return {"pull_requests": self.LINKS.get(args[2], [])}
        raise AssertionError(f"in-review list read multica {' '.join(args)}")

    def setUp(self):
        stand_in(self, "multica", self.multica)

    def test_every_issue_in_review_names_its_project_release_lane_and_pull_request(self):
        found = {row["key"]: row for row in team_state_review.collect_in_review()["issues"]}
        self.assertEqual(found["WORK-11"], {
            "key": "WORK-11", "title": "Title of WORK-11", "project": "App", "release": "0.2.0",
            "lane": {"id": "l-app", "name": "main"}, "stack": {"position": 2, "layers": 2},
            "pull_requests": [{"url": "https://github.com/acme/app/pull/8", "number": 8, "branch": "branch-8",
                               "state": "open"}],
        })
        self.assertEqual({k: (r["project"], r["release"], r["lane"] and r["lane"]["name"]) for k, r in found.items()}, {
            "WORK-11": ("App", "0.2.0", "main"), "WORK-12": ("App", "0.1.0", "main"),
            "WORK-20": ("Dashboard", "0.1.0", "Rooster"), "WORK-21": ("Dashboard", "0.1.0", "Rooster"),
            "WORK-30": ("Dashboard", None, None),
        })
        self.assertEqual(found["WORK-30"]["stack"], None)
        self.assertEqual([p["number"] for p in found["WORK-30"]["pull_requests"]], [2, 5])

    def test_issues_in_other_statuses_stay_out_and_are_no_layer_of_a_stack(self):
        found = team_state_review.collect_in_review()["issues"]
        self.assertEqual(sorted(row["key"] for row in found), ["WORK-11", "WORK-12", "WORK-20", "WORK-21", "WORK-30"])
        self.assertEqual({row["key"]: row["stack"] for row in found if row["lane"]}, {
            "WORK-12": {"position": 1, "layers": 2}, "WORK-11": {"position": 2, "layers": 2},
            "WORK-20": {"position": 1, "layers": 2}, "WORK-21": {"position": 2, "layers": 2},
        })

    def test_each_stack_comes_bottom_layer_first_whatever_the_board_order(self):
        found = team_state_review.collect_in_review()["issues"]
        self.assertEqual([row["key"] for row in found], ["WORK-12", "WORK-11", "WORK-20", "WORK-21", "WORK-30"])

    def test_an_issue_whose_links_cannot_be_read_keeps_its_row_and_shows_the_error(self):
        def failing(*args):
            if args[:3] == ("issue", "pull-requests", "WORK-20"):
                raise team_state_io.StateError("multica: timeout")
            return self.multica(*args)

        stand_in(self, "multica", failing)
        found = team_state_review.collect_in_review()
        self.assertEqual({row["key"]: row["pull_requests"] for row in found["issues"]}["WORK-20"], None)
        self.assertEqual(found["errors"], ["Multica links: WORK-20: multica: timeout"])

    def test_the_command_prints_the_list_without_a_verifier_and_refuses_endeavour_options(self):
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            self.assertEqual(team_state.main(["--in-review"]), 0)
        self.assertEqual([row["key"] for row in json.loads(out.getvalue())["issues"]],
                         ["WORK-12", "WORK-11", "WORK-20", "WORK-21", "WORK-30"])
        for extra in (["~/Repos"], ["--intake", "acme/app"], ["--lane", "main"], ["--verifier", "Verifier"]):
            with self.subTest(extra=extra), contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit):
                team_state.main(["--in-review", *extra])


if __name__ == "__main__":
    unittest.main()
