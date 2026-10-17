import importlib.util
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
        self.archive("cart")
        git(self.repo, "checkout", "-qb", "work-9")
        self.archive("receipt")
        open_change = self.repo / "openspec" / "changes" / "coupon"
        open_change.mkdir()
        (open_change / ".openspec.yaml").write_text("build:\n  status: in-progress\n  assignee: ann\n", encoding="utf-8")

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
        (self.repo / "openspec" / "changes" / "broken" / ".openspec.yaml").write_text("build: [\n", encoding="utf-8")
        found = {c["name"]: c.get("error") for c in team_state.collect_changes(self.repo, "main")}
        self.assertEqual(found, {"broken": "build: is no map: [", "coupon": None, "receipt": None})

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


if __name__ == "__main__":
    unittest.main()
