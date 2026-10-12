import concurrent.futures
import json
import sys
import tempfile
import time
from copy import deepcopy
from pathlib import Path
from types import SimpleNamespace

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import multica_projects  # noqa: E402
from multica_projects import (  # noqa: E402
    COMPLETED,
    IN_PROGRESS,
    Create,
    LocalDirectory,
    ResourceUpdate,
    StatusMove,
    TakeOver,
    Unplaced,
)

DAEMON = "019fce17-f818-71b1-bc29-4d9229e2139a"
REPOS = Path("/Users/brian/Repos")


class Workspace:
    """Multica's projects and their resources as plain data, the way the reads return them."""

    def __init__(self) -> None:
        self.projects: list[dict] = []
        self.resources: dict[str, list[dict]] = {}

    def project(self, project_id: str, title: str, status: str = IN_PROGRESS, created_at: str = "2026-10-01T00:00:00Z") -> str:
        self.projects.append({"id": project_id, "title": title, "status": status, "created_at": created_at})
        self.resources[project_id] = []
        return project_id

    def directory(self, project_id: str, path: Path, label: str = "", daemon: str = DAEMON, mode: str = "worktree") -> None:
        ref = {"local_path": str(path), "daemon_id": daemon, "execution_mode": mode}
        if label:
            ref["label"] = label
        resource = {"id": f"{project_id}-r{len(self.resources[project_id])}", "resource_type": "local_directory", "resource_ref": ref, "label": label or None}
        self.resources[project_id].append(resource)

    def marked(self, project_id: str, key: str, path: Path, status: str = IN_PROGRESS, mode: str = "worktree") -> str:
        self.project(project_id, multica_projects.name_of(key), status)
        self.directory(project_id, path, f"madspec:{key}", mode=mode)
        return project_id

    def apply(self, plan: multica_projects.Plan) -> None:
        """What Multica holds after the plan's writes went through."""
        for create in plan.creates:
            project_id = self.project(f"new-{create.key}", create.title)
            self.directory(project_id, Path(create.resource.local_path), create.resource.label)
        for take_over in plan.take_overs:
            self.directory(take_over.project_id, Path(take_over.resource.local_path), take_over.resource.label)
        for update in plan.resource_updates:
            for resource in self.resources[update.project_id]:
                if resource["id"] == update.resource_id:
                    resource["resource_ref"] = update.resource.ref()
        for move in plan.status_moves:
            next(project for project in self.projects if project["id"] == move.project_id)["status"] = move.status

    def plan(self, repositories: dict[str, list[Path]], archived: dict[str, bool | None] | None = None) -> multica_projects.Plan:
        known = {key: False for key in repositories} if archived is None else archived
        return multica_projects.plan(repositories, known, self.projects, self.resources, DAEMON)


def wanted(key: str, path: Path) -> LocalDirectory:
    return LocalDirectory(str(path), DAEMON, f"madspec:{key}", "worktree")


def reasons(plan: multica_projects.Plan) -> dict[str, Unplaced]:
    return {report.key: report.reason for report in plan.reports}


# Every Repository object is a Multica project on its checkout


def test_the_first_run_makes_one_project_per_repository_with_a_checkout():
    repositories = {
        "appboypov/baba": [REPOS / "Clients/baba/baba"],
        "appboypov/floatnote": [REPOS / "Apps/fn/floatnote"],
        "appboypov/whitebird-brain": [REPOS / "Clients/paps/whitebird-brain"],
        "appboypov/work": [],
    }
    workspace = Workspace()

    plan = workspace.plan(repositories)

    assert plan.creates == tuple(
        Create(key, multica_projects.name_of(key), wanted(key, paths[0]))
        for key, paths in sorted(repositories.items())
        if paths
    )
    assert all(create.resource.execution_mode == "worktree" for create in plan.creates)
    assert "work" not in {create.title for create in plan.creates}
    assert plan.take_overs == plan.resource_updates == plan.status_moves == ()


def test_a_project_carries_its_repository_name_and_checkout():
    checkout = REPOS / "Clients/sm/sm-design"

    plan = Workspace().plan({"Supermarty-B-V/sm-design": [checkout]})

    (create,) = plan.creates
    assert create.title == "sm-design"
    assert create.resource.ref() == {
        "local_path": str(checkout),
        "daemon_id": DAEMON,
        "label": "madspec:supermarty-b-v/sm-design",
        "execution_mode": "worktree",
    }


def test_a_mixed_case_key_keeps_its_name_in_the_title_and_finds_its_lowercase_marker():
    checkout = REPOS / "Clients/baba/baba"

    (create,) = Workspace().plan({"AppBoyPov/Baba": [checkout]}).creates
    assert (create.title, create.resource.label) == ("Baba", "madspec:appboypov/baba")

    workspace = Workspace()
    workspace.marked("p-baba", "appboypov/baba", checkout)
    assert workspace.plan({"AppBoyPov/Baba": [checkout]}).writes == ()


def test_a_new_repository_object_becomes_one_project_and_nothing_else_changes():
    workspace = Workspace()
    workspace.marked("p-baba", "appboypov/baba", REPOS / "Clients/baba/baba")
    workspace.marked("p-rebl", "appboypov/rebl", REPOS / "Clients/rebl")
    repositories = {
        "appboypov/baba": [REPOS / "Clients/baba/baba"],
        "appboypov/rebl": [REPOS / "Clients/rebl"],
        "appboypov/mia": [REPOS / "Clients/ivga/mia"],
    }

    plan = workspace.plan(repositories)

    assert plan.writes == (Create("appboypov/mia", "mia", wanted("appboypov/mia", REPOS / "Clients/ivga/mia")),)


# A repository that cannot be placed is reported and left alone


def test_a_repository_without_a_checkout_is_reported_and_gets_no_project():
    plan = Workspace().plan({"appboypov/work": []})

    assert reasons(plan) == {"appboypov/work": Unplaced.NO_CHECKOUT}
    assert plan.writes == ()


def test_a_repository_with_two_checkouts_is_reported_with_both_and_its_project_stays():
    first, second = REPOS / "Clients/rebl", REPOS / "Clients/rebl/rebl"
    workspace = Workspace()
    workspace.marked("p-rebl", "appboypov/rebl", first, status=COMPLETED)

    plan = workspace.plan({"appboypov/rebl": [second, first]})

    (report,) = plan.reports
    assert (report.key, report.reason) == ("appboypov/rebl", Unplaced.CHECKOUTS)
    assert set(report.checkouts) == {str(first), str(second)}
    assert plan.writes == ()


def test_two_repositories_with_one_name_are_both_reported_and_get_no_project():
    plan = Workspace().plan({
        "appboypov/site": [REPOS / "Ventures/site"],
        "supermarty-b-v/site": [REPOS / "Clients/sm/site"],
    })

    assert reasons(plan) == {"appboypov/site": Unplaced.SHARED_NAME, "supermarty-b-v/site": Unplaced.SHARED_NAME}
    assert {report.key: report.others for report in plan.reports} == {
        "appboypov/site": ("supermarty-b-v/site",),
        "supermarty-b-v/site": ("appboypov/site",),
    }
    assert plan.writes == ()


def test_a_repository_github_does_not_know_is_reported_and_its_project_stays():
    workspace = Workspace()
    workspace.marked("p-gone", "appboypov/gone", REPOS / "Ventures/gone")

    plan = workspace.plan({"appboypov/gone": [REPOS / "Ventures/gone"]}, {"appboypov/gone": None})

    assert reasons(plan) == {"appboypov/gone": Unplaced.UNKNOWN}
    assert plan.writes == ()


# A changed checkout is updated in place


def test_a_moved_checkout_updates_the_same_project_without_touching_its_status():
    workspace = Workspace()
    workspace.marked("p-rebl", "appboypov/rebl", REPOS / "Clients/rebl")
    moved = REPOS / "Clients/rebl/rebl"

    plan = workspace.plan({"appboypov/rebl": [moved]})

    assert plan.writes == (ResourceUpdate("appboypov/rebl", "p-rebl", "rebl", "p-rebl-r0", wanted("appboypov/rebl", moved)),)


def test_a_resource_switched_to_in_place_goes_back_to_worktree():
    checkout = REPOS / "Clients/baba/baba"
    workspace = Workspace()
    workspace.marked("p-baba", "appboypov/baba", checkout, mode="in_place")

    plan = workspace.plan({"appboypov/baba": [checkout]})

    (update,) = plan.resource_updates
    assert (update.project_id, update.resource.execution_mode) == ("p-baba", "worktree")
    assert plan.creates == plan.take_overs == plan.status_moves == ()


# A project follows its repository out and back


def test_a_deleted_repository_object_completes_its_project():
    workspace = Workspace()
    workspace.marked("p-photoprank", "appboypov/photoprank", REPOS / "Apps/photoprank")

    plan = workspace.plan({"appboypov/baba": [REPOS / "Clients/baba/baba"]})

    assert StatusMove("appboypov/photoprank", "p-photoprank", "photoprank", COMPLETED) in plan.status_moves
    assert not any(write.key == "appboypov/photoprank" for write in plan.writes if not isinstance(write, StatusMove))


def test_a_repository_github_archived_completes_its_project():
    checkout = REPOS / "Clients/ivga/mia"
    workspace = Workspace()
    workspace.marked("p-mia", "appboypov/mia", checkout)

    plan = workspace.plan({"appboypov/mia": [checkout]}, {"appboypov/mia": True})

    assert plan.writes == (StatusMove("appboypov/mia", "p-mia", "mia", COMPLETED),)
    assert plan.reports == ()


def test_a_returning_repository_puts_the_same_project_back_in_progress():
    checkout = REPOS / "Apps/photoprank"
    workspace = Workspace()
    workspace.marked("p-photoprank", "appboypov/photoprank", checkout, status=COMPLETED)

    plan = workspace.plan({"appboypov/photoprank": [checkout]})

    assert plan.writes == (StatusMove("appboypov/photoprank", "p-photoprank", "photoprank", IN_PROGRESS),)


def test_a_status_brian_set_stays_for_a_counting_and_a_leaving_repository():
    checkout = REPOS / "Clients/baba/baba"
    workspace = Workspace()
    workspace.marked("p-baba", "appboypov/baba", checkout, status="paused")
    workspace.marked("p-old", "appboypov/old", REPOS / "Ventures/old", status="paused")

    plan = workspace.plan({"appboypov/baba": [checkout]})

    assert plan.writes == ()


# The sync manages only its own projects


def test_a_hand_made_project_with_the_repository_name_is_taken_over():
    checkout = REPOS / "Clients/baba/baba"
    workspace = Workspace()
    workspace.project("p-baba", "baba")

    plan = workspace.plan({"appboypov/baba": [checkout]})

    assert plan.writes == (TakeOver("appboypov/baba", "p-baba", "baba", wanted("appboypov/baba", checkout)),)


def test_a_same_named_project_already_on_the_local_daemon_is_not_taken_over():
    checkout = REPOS / "Clients/baba/baba"
    workspace = Workspace()
    workspace.project("p-baba", "baba")
    workspace.directory("p-baba", REPOS / "Elsewhere/baba")

    plan = workspace.plan({"appboypov/baba": [checkout]})

    assert [type(write) for write in plan.writes] == [Create]


def test_an_unmarked_project_matching_no_repository_is_left_alone():
    workspace = Workspace()
    workspace.project("p-experiments", "Experiments", status="paused")
    workspace.directory("p-experiments", REPOS / "Experiments")

    plan = workspace.plan({"appboypov/baba": [REPOS / "Clients/baba/baba"]})

    assert all(getattr(write, "project_id", None) != "p-experiments" for write in plan.writes)


# A sync writes only what differs


def test_a_second_run_without_a_source_change_writes_nothing():
    repositories = {
        "appboypov/baba": [REPOS / "Clients/baba/baba"],
        "appboypov/rebl": [REPOS / "Clients/rebl/rebl"],
        "appboypov/mia": [REPOS / "Clients/ivga/mia"],
        "appboypov/photoprank": [REPOS / "Apps/photoprank"],
        "appboypov/work": [],
    }
    workspace = Workspace()
    workspace.project("p-baba", "baba", status=COMPLETED)
    workspace.marked("p-rebl", "appboypov/rebl", REPOS / "Clients/rebl", mode="in_place")
    workspace.marked("p-photoprank", "appboypov/photoprank", REPOS / "Apps/photoprank", status=COMPLETED)
    workspace.marked("p-left", "appboypov/left", REPOS / "Ventures/left")
    first = workspace.plan(repositories)
    workspace.apply(first)

    second = workspace.plan(repositories)

    assert first.writes
    assert second.writes == ()
    assert reasons(second) == {"appboypov/work": Unplaced.NO_CHECKOUT}


# The shell: failed reads never write; failed writes do not stop other projects.


class FakeApiError(Exception):
    pass


class FakeMultica(Workspace):
    """An in-memory workspace with the real API shapes and observable mutations."""

    def __init__(self):
        super().__init__()
        self.writes = []
        self.fail_read = None
        self.fail_create = None
        self.fail_resource = False

    def call(self, method, path, body=None):
        if method == "GET":
            if path == self.fail_read:
                raise FakeApiError("Multica is unreachable")
            if path == "/api/runtimes":
                return [{"id": "runtime", "daemon_id": DAEMON, "provider": "omp", "runtime_mode": "local"}]
            if path == "/api/projects":
                return {"projects": [{**project, "resource_count": len(self.resources[project["id"]])} for project in self.projects]}
            return {"resources": self.resources[path.split("/")[3]]}
        self.writes.append((method, path, body))
        if method == "POST" and path == "/api/projects":
            if body["title"] == self.fail_create:
                raise FakeApiError("create refused")
            project_id = self.project("new-" + body["title"], body["title"], body["status"])
            return next(project for project in self.projects if project["id"] == project_id)
        project_id = path.split("/")[3]
        if method == "POST":
            if self.fail_resource:
                self.fail_resource = False
                raise FakeApiError("resource refused")
            resource = {"id": project_id + "-resource", **body}
            self.resources[project_id].append(resource)
            return resource
        if "/resources/" in path:
            resource = next(resource for resource in self.resources[project_id] if resource["id"] == path.split("/")[-1])
            resource.update(body)
            return resource
        project = next(project for project in self.projects if project["id"] == project_id)
        project.update(body)
        return project


class Shell:
    """Fake external reads around the actual reader, plan and write loop."""

    def __init__(self, monkeypatch):
        self.api = FakeMultica()
        self.folder = tempfile.TemporaryDirectory()
        self.objects = [{"id": "brain-baba", "properties": {"github_key": "AppBoyPov/Baba"}}]
        self.checkouts = {"appboypov/baba": [REPOS / "Clients/baba/baba"]}
        self.brain_error = False
        self.github_errors = []
        self.archived = {"appboypov/baba": False}
        team = SimpleNamespace(
            brain=SimpleNamespace(search=self.search),
            model=SimpleNamespace(api_key=lambda: "fake-key", ModelError=FakeApiError),
            REPOS_ROOT=REPOS,
            local_checkouts=lambda: self.checkouts,
        )
        sync = SimpleNamespace(
            CONFIG=Path(self.folder.name) / "config.json",
            load_config=lambda path: {},
            Multica=lambda config: self.api,
            ApiError=FakeApiError,
            omp_runtime=lambda runtimes: runtimes[0]["id"],
        )
        monkeypatch.setattr(multica_projects, "load_dependencies", lambda: (team, sync))
        monkeypatch.setattr(multica_projects.subprocess, "run", self.github)

    def search(self, *args, **kwargs):
        if self.brain_error:
            raise FakeApiError("Anytype is unreachable")
        return self.objects

    def github(self, command, **kwargs):
        data = {
            f"r{index}": None if self.archived.get(key.lower()) is None else {"isArchived": self.archived[key.lower()]}
            for index, key in enumerate(sorted(repository["properties"]["github_key"] for repository in self.objects))
        }
        return SimpleNamespace(
            stdout=json.dumps({"data": data, "errors": self.github_errors}),
            stderr="GraphQL error" if self.github_errors else "",
            returncode=1 if self.github_errors else 0,
        )

    def add_repository(self, key):
        self.objects.append({"id": "brain-" + key, "properties": {"github_key": key}})
        self.checkouts[key] = [REPOS / "Clients" / multica_projects.name_of(key)]
        self.archived[key] = False

    def state(self):
        return deepcopy((self.api.projects, self.api.resources))


def test_not_found_alias_keeps_its_project_and_syncs_the_known_repository(monkeypatch):
    shell = Shell(monkeypatch)
    shell.add_repository("appboypov/gone")
    shell.archived["appboypov/gone"] = None
    shell.github_errors = [{"type": "NOT_FOUND", "path": ["r1"], "message": "not found"}]
    shell.api.marked("p-gone", "appboypov/gone", REPOS / "Old/gone", status=COMPLETED)
    gone_before = deepcopy(shell.api.resources["p-gone"])

    assert multica_projects.main() == 0

    assert {project["title"]: project["status"] for project in shell.api.projects} == {"gone": COMPLETED, "Baba": IN_PROGRESS}
    assert shell.api.resources["p-gone"] == gone_before
    assert shell.api.resources["new-Baba"][0]["resource_ref"]["execution_mode"] == "worktree"


def test_other_graphql_error_fails_the_read_without_writes(monkeypatch):
    shell = Shell(monkeypatch)
    shell.github_errors = [{"type": "FORBIDDEN", "message": "permission denied"}]
    shell.api.marked("p-left", "appboypov/left", REPOS / "Old/left")
    before = shell.state()

    assert multica_projects.main() == 1

    assert shell.state() == before
    assert shell.api.writes == []


@pytest.mark.parametrize("source", ["brain", "empty-brain", "runtimes", "projects", "resources"])
def test_failed_read_never_creates_updates_or_completes_projects(monkeypatch, source):
    shell = Shell(monkeypatch)
    shell.api.marked("p-baba", "appboypov/baba", REPOS / "Old/baba", mode="in_place")
    shell.api.marked("p-left", "appboypov/left", REPOS / "Old/left")
    before = shell.state()
    if source == "brain":
        shell.brain_error = True
    elif source == "empty-brain":
        shell.objects = []
    else:
        shell.api.fail_read = {
            "runtimes": "/api/runtimes", "projects": "/api/projects",
            "resources": "/api/projects/p-left/resources",
        }[source]

    assert multica_projects.main() == 1

    assert shell.state() == before
    assert shell.api.writes == []


def test_failed_create_still_syncs_other_projects_and_exits_one(monkeypatch):
    shell = Shell(monkeypatch)
    shell.add_repository("appboypov/rebl")
    shell.api.fail_create = "Baba"

    assert multica_projects.main() == 1

    assert {project["title"] for project in shell.api.projects} == {"rebl"}
    assert shell.api.resources["new-rebl"][0]["resource_ref"] == wanted("appboypov/rebl", shell.checkouts["appboypov/rebl"][0]).ref()


def test_failed_resource_continues_and_next_run_heals_without_a_duplicate(monkeypatch):
    shell = Shell(monkeypatch)
    shell.add_repository("appboypov/rebl")
    shell.api.fail_resource = True

    assert multica_projects.main() == 1
    assert shell.api.resources["new-Baba"] == []
    assert shell.api.resources["new-rebl"][0]["resource_ref"]["label"] == "madspec:appboypov/rebl"

    assert multica_projects.main() == 0

    assert [project["title"] for project in shell.api.projects] == ["Baba", "rebl"]
    assert shell.api.resources["new-Baba"][0]["resource_ref"]["label"] == "madspec:appboypov/baba"
    previous_writes = deepcopy(shell.api.writes)
    assert multica_projects.main() == 0
    assert shell.api.writes == previous_writes


def test_shell_updates_resources_and_statuses_without_changing_other_fields(monkeypatch):
    shell = Shell(monkeypatch)
    shell.api.marked("p-baba", "appboypov/baba", REPOS / "Old/baba", status=COMPLETED, mode="in_place")
    shell.api.marked("p-left", "appboypov/left", REPOS / "Old/left")
    shell.api.projects[0]["description"] = "Keep this"
    old_resource_id = shell.api.resources["p-baba"][0]["id"]

    assert multica_projects.main() == 0

    assert {project["id"]: project["status"] for project in shell.api.projects} == {"p-baba": IN_PROGRESS, "p-left": COMPLETED}
    assert shell.api.projects[0]["description"] == "Keep this"
    resource = shell.api.resources["p-baba"][0]
    assert resource["id"] == old_resource_id
    assert resource["resource_ref"] == wanted("appboypov/baba", shell.checkouts["appboypov/baba"][0]).ref()


def test_archive_checkout_is_excluded_and_mixed_case_key_uses_live_checkout(monkeypatch):
    shell = Shell(monkeypatch)
    shell.checkouts["appboypov/baba"].append(REPOS / "Archive/baba")

    assert multica_projects.main() == 0

    assert shell.api.resources["new-Baba"][0]["resource_ref"]["local_path"] == str(REPOS / "Clients/baba/baba")


def test_failed_take_over_keeps_the_unmarked_project_completed_and_syncs_other_projects(monkeypatch):
    shell = Shell(monkeypatch)
    shell.api.project("manual-baba", "Baba", status=COMPLETED)
    shell.api.marked("p-left", "appboypov/left", REPOS / "Old/left")
    shell.api.fail_resource = True

    assert multica_projects.main() == 1

    assert shell.api.resources["manual-baba"] == []
    assert {project["id"]: project["status"] for project in shell.api.projects} == {
        "manual-baba": COMPLETED, "p-left": COMPLETED,
    }


def test_overlapping_syncs_create_only_one_managed_project(monkeypatch):
    shell = Shell(monkeypatch)
    original_call = shell.api.call

    def delayed_snapshot(method, path, body=None):
        snapshot = original_call(method, path, body)
        if method == "GET" and path == "/api/projects":
            time.sleep(0.1)
        return snapshot

    monkeypatch.setattr(shell.api, "call", delayed_snapshot)
    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
        assert list(pool.map(lambda _: multica_projects.main(), range(2))) == [0, 0]

    assert [project["title"] for project in shell.api.projects] == ["Baba"]
    assert len(shell.api.resources["new-Baba"]) == 1


def test_github_timeout_writes_nothing(monkeypatch):
    shell = Shell(monkeypatch)
    shell.api.marked("p-left", "appboypov/left", REPOS / "Old/left")
    before = shell.state()

    def stalled_query(*args, **kwargs):
        raise multica_projects.subprocess.TimeoutExpired("gh", 60)

    monkeypatch.setattr(multica_projects.subprocess, "run", stalled_query)
    assert multica_projects.main() == 1
    assert shell.state() == before
    assert shell.api.writes == []


def test_a_marked_resource_on_another_daemon_is_updated_in_place():
    checkout = REPOS / "Clients/baba/baba"
    workspace = Workspace()
    workspace.project("p-baba", "baba")
    workspace.directory("p-baba", checkout, "madspec:appboypov/baba", daemon="previous-daemon")

    desired = workspace.plan({"appboypov/baba": [checkout]})

    assert desired.resource_updates == (
        ResourceUpdate("appboypov/baba", "p-baba", "baba", "p-baba-r0", wanted("appboypov/baba", checkout)),
    )
    assert desired.creates == desired.take_overs == desired.status_moves == ()


def test_repository_objects_with_case_aliases_create_one_project(monkeypatch):
    shell = Shell(monkeypatch)
    shell.objects.append({"id": "brain-case-alias", "properties": {"github_key": "appboypov/baba"}})

    assert multica_projects.main() == 0
    assert [project["title"] for project in shell.api.projects] == ["Baba"]
    assert shell.api.resources["new-Baba"][0]["resource_ref"]["label"] == "madspec:appboypov/baba"
