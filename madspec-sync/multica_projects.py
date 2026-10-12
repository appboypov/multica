#!/usr/bin/env python3
# /// script
# requires-python = ">=3.11"
# dependencies = ["grpcio"]
# ///
"""Sync the brain's Repository objects to projects in the local Multica workspace.

A repository counts when it has a Repository object in the brain, GitHub knows
it and has not archived it, it has exactly one checkout under `~/Repos` outside
`~/Repos/Archive` (found by its `origin` remote) and no other counting
repository has its name. Each counting repository is one project titled with
the name part of its GitHub key, status `in_progress` on create, with one
`local_directory` resource on the checkout: the local omp daemon, execution
mode `worktree` and label `madspec:<owner/name>` in lowercase.

A project is the sync's own when it has a `local_directory` resource with that
label. An unmarked project titled with a counting repository's name and without
a `local_directory` resource on the local daemon is taken over by adding the
marked resource. Every other unmarked project is left alone.

A marked resource whose checkout, daemon or execution mode differs is updated
in place. A repository leaves when its Repository object is gone or GitHub
archived it: its project moves from `in_progress` to `completed`, and back to
`in_progress` when the repository counts again. No other status is set, and no
project or resource is deleted. A repository with no checkout, two checkouts,
a shared name or unknown to GitHub gets one printed line and its project stays
as it is.

The sync runs only when started by hand. It reads the brain, the checkouts,
GitHub and Multica before it writes, and writes nothing when a read fails or
the brain holds no Repository object. It writes only what differs, prints one
line per write and per reported repository, and exits 1 when anything failed.
Runs of one profile share a file lock across the read, plan and write phases,
so two runs at once never make the same project twice.

It reads the brain and the checkouts with MadSpec's `team_state.py`, from the
MadSpec checkout `multica_sync.py` names, and Multica with `multica_sync.py`
from this folder.

Usage, from the root of the Multica checkout:
  uv run madspec-sync/multica_projects.py
"""

from __future__ import annotations

import fcntl
import importlib.util
import json
import re
import subprocess
import sys
from dataclasses import dataclass
from enum import Enum
from pathlib import Path
from types import ModuleType

sys.dont_write_bytecode = True

MARKER_PREFIX = "madspec:"
LOCAL_DIRECTORY = "local_directory"
WORKTREE = "worktree"
IN_PROGRESS = "in_progress"
COMPLETED = "completed"
GITHUB_READ_TIMEOUT = 60


# --- The plan ---


@dataclass(frozen=True)
class LocalDirectory:
    """The `local_directory` resource a counting repository's project should have."""

    local_path: str
    daemon_id: str
    label: str
    execution_mode: str = WORKTREE

    def ref(self) -> dict[str, str]:
        return {
            "local_path": self.local_path,
            "daemon_id": self.daemon_id,
            "label": self.label,
            "execution_mode": self.execution_mode,
        }


@dataclass(frozen=True)
class Create:
    """A new project with status `in_progress`, then its resource."""

    key: str
    title: str
    resource: LocalDirectory


@dataclass(frozen=True)
class TakeOver:
    """An unmarked project that becomes the sync's own by getting its resource."""

    key: str
    project_id: str
    title: str
    resource: LocalDirectory


@dataclass(frozen=True)
class ResourceUpdate:
    """A marked resource set back to what the repository wants."""

    key: str
    project_id: str
    title: str
    resource_id: str
    resource: LocalDirectory


@dataclass(frozen=True)
class StatusMove:
    """A marked project moved between `in_progress` and `completed`."""

    key: str
    project_id: str
    title: str
    status: str


class Unplaced(Enum):
    """Why a repository with a Repository object gets no project."""

    NO_CHECKOUT = "has no checkout"
    CHECKOUTS = "has more than one checkout"
    SHARED_NAME = "shares its name"
    UNKNOWN = "is unknown to GitHub"


@dataclass(frozen=True)
class Report:
    """A repository that cannot be placed, left as it is."""

    key: str
    reason: Unplaced
    checkouts: tuple[str, ...] = ()
    others: tuple[str, ...] = ()

    @property
    def line(self) -> str:
        if self.reason is Unplaced.CHECKOUTS:
            return f"{self.key}: {self.reason.value}: {', '.join(self.checkouts)}"
        if self.reason is Unplaced.SHARED_NAME:
            return f"{self.key}: {self.reason.value} {name_of(self.key)} with {', '.join(self.others)}"
        return f"{self.key}: {self.reason.value}"


Write = Create | TakeOver | ResourceUpdate | StatusMove


@dataclass(frozen=True)
class Plan:
    """What one run sends to Multica and what it reports."""

    creates: tuple[Create, ...] = ()
    take_overs: tuple[TakeOver, ...] = ()
    resource_updates: tuple[ResourceUpdate, ...] = ()
    status_moves: tuple[StatusMove, ...] = ()
    reports: tuple[Report, ...] = ()

    @property
    def writes(self) -> tuple[Write, ...]:
        return (*self.creates, *self.take_overs, *self.resource_updates, *self.status_moves)


def plan(
    repositories: dict[str, list[Path]],
    archived: dict[str, bool | None],
    projects: list[dict],
    resources: dict[str, list[dict]],
    daemon: str,
) -> Plan:
    """The writes that bring Multica's projects in line with the brain.

    repositories: GitHub key of every Repository object, as the brain writes it ->
    its checkouts outside ~/Repos/Archive; archived: key -> isArchived, None (or
    absent) when GitHub does not know it; projects: every project of the
    workspace; resources: project id -> its resources; daemon: the local omp
    daemon's id.
    """
    reports: list[Report] = []
    placeable: dict[str, Path] = {}
    for key in sorted(repositories):
        checkouts = sorted(repositories[key], key=str)
        state = archived.get(key)
        if state is True:
            continue
        if not checkouts:
            reports.append(Report(key, Unplaced.NO_CHECKOUT))
        elif len(checkouts) > 1:
            reports.append(Report(key, Unplaced.CHECKOUTS, tuple(str(path) for path in checkouts)))
        elif state is None:
            reports.append(Report(key, Unplaced.UNKNOWN))
        else:
            placeable[key] = checkouts[0]

    by_name: dict[str, list[str]] = {}
    for key in placeable:
        by_name.setdefault(name_of(key).lower(), []).append(key)
    counting: dict[str, Path] = {}
    for key, checkout in placeable.items():
        sharing = by_name[name_of(key).lower()]
        if len(sharing) > 1:
            reports.append(Report(key, Unplaced.SHARED_NAME, others=tuple(other for other in sharing if other != key)))
        else:
            counting[key] = checkout
    reports.sort(key=lambda report: report.key)

    ordered = sorted(projects, key=lambda project: (project.get("created_at") or "", project["id"]))
    marked: dict[str, tuple[dict, dict]] = {}
    unmarked: list[dict] = []
    for project in ordered:
        markers = [
            (key, resource)
            for resource in resources.get(project["id"], [])
            if (key := marker_key(resource)) is not None
        ]
        if not markers:
            unmarked.append(project)
        for key, resource in markers:
            marked.setdefault(key, (project, resource))

    creates: list[Create] = []
    take_overs: list[TakeOver] = []
    updates: list[ResourceUpdate] = []
    moves: list[StatusMove] = []
    taken: set[str] = set()
    for key, checkout in sorted(counting.items()):
        wanted = LocalDirectory(str(checkout), daemon, MARKER_PREFIX + key.lower())
        if key.lower() in marked:
            project, resource = marked[key.lower()]
            ref = resource_ref(resource)
            current = (ref.get("local_path"), ref.get("daemon_id"), ref.get("execution_mode"))
            if current != (wanted.local_path, wanted.daemon_id, wanted.execution_mode):
                updates.append(ResourceUpdate(key, project["id"], project.get("title", ""), resource["id"], wanted))
        else:
            project = next(
                (
                    candidate
                    for candidate in unmarked
                    if candidate["id"] not in taken
                    and (candidate.get("title") or "").lower() == name_of(key).lower()
                    and not on_daemon(resources.get(candidate["id"], []), daemon)
                ),
                None,
            )
            if project is None:
                creates.append(Create(key, name_of(key), wanted))
                continue
            taken.add(project["id"])
            take_overs.append(TakeOver(key, project["id"], project.get("title", ""), wanted))
        if project.get("status") == COMPLETED:
            moves.append(StatusMove(key, project["id"], project.get("title", ""), IN_PROGRESS))

    present = {key.lower(): key for key in repositories}
    for marked_key, (project, _) in sorted(marked.items()):
        key = present.get(marked_key, marked_key)
        left = marked_key not in present or archived.get(key) is True
        if left and project.get("status") == IN_PROGRESS:
            moves.append(StatusMove(key, project["id"], project.get("title", ""), COMPLETED))

    return Plan(tuple(creates), tuple(take_overs), tuple(updates), tuple(moves), tuple(reports))


def name_of(key: str) -> str:
    """The repository name of a GitHub key: `owner/name` -> `name`."""
    return key.rsplit("/", 1)[-1]


def resource_ref(resource: dict) -> dict:
    ref = resource.get("resource_ref")
    return ref if isinstance(ref, dict) else {}


def marker_key(resource: dict) -> str | None:
    """The lowercase GitHub key a `local_directory` resource marks, if any.

    Multica keeps a resource's label in its own column and in the ref; either marks.
    """
    if resource.get("resource_type") != LOCAL_DIRECTORY:
        return None
    for label in (resource.get("label"), resource_ref(resource).get("label")):
        if isinstance(label, str) and label.startswith(MARKER_PREFIX) and len(label) > len(MARKER_PREFIX):
            return label[len(MARKER_PREFIX):].lower()
    return None


def on_daemon(resources: list[dict], daemon: str) -> bool:
    """Whether a project has a `local_directory` resource on the daemon."""
    return any(
        resource.get("resource_type") == LOCAL_DIRECTORY and resource_ref(resource).get("daemon_id") == daemon
        for resource in resources
    )


# --- Reads ---


def load_dependencies() -> tuple[ModuleType, ModuleType]:
    """Load MadSpec's `team_state.py` and this folder's `multica_sync.py`."""
    multica_sync = load_module("madspec_projects_multica_sync", Path(__file__).resolve().parent / "multica_sync.py")
    team_state = load_module(
        "madspec_projects_team_state",
        multica_sync.MADSPEC / "skills" / "madspec-find-next-chunk" / "scripts" / "team_state.py",
    )
    return team_state, multica_sync


def load_module(name: str, path: Path) -> ModuleType:
    """The Python file at `path`, loaded as the module `name`."""
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


def read_repositories(team_state: ModuleType) -> dict[str, list[Path]]:
    """Repository keys from the brain, with their non-Archive checkouts."""
    try:
        objects = team_state.brain.search(team_state.model.api_key(), "repository", fields=["github_key"])
    except (team_state.model.ModelError, OSError, KeyError) as error:
        raise ValueError(f"cannot read the brain: {error}") from error
    if not objects:
        raise ValueError("the brain holds no Repository objects")
    keys: dict[str, str] = {}
    for repository in objects:
        key = repository["properties"].get("github_key") or ""
        if not re.fullmatch(r"[\w.-]+/[\w.-]+", key):
            raise ValueError(f"Repository {repository['id']} has no valid GitHub key")
        keys.setdefault(key.lower(), key)
    checkouts = team_state.local_checkouts()
    archive = team_state.REPOS_ROOT / "Archive"
    return {
        key: [path for path in checkouts.get(key.lower(), []) if not path.is_relative_to(archive)]
        for key in sorted(keys.values())
    }


def read_archived(repositories: dict[str, list[Path]]) -> dict[str, bool | None]:
    """One aliased GitHub query; NOT_FOUND leaves only that key unknown."""
    aliases = {f"r{index}": key for index, key in enumerate(sorted(repositories))}
    fields = []
    for alias, key in aliases.items():
        owner, name = key.split("/", 1)
        fields.append(f"{alias}: repository(owner: {json.dumps(owner)}, name: {json.dumps(name)}) {{ isArchived }}")
    result = subprocess.run(
        ["gh", "api", "graphql", "-f", "query=query { " + " ".join(fields) + " }"],
        capture_output=True, text=True, timeout=GITHUB_READ_TIMEOUT,
    )
    try:
        response = json.loads(result.stdout)
        errors = [error for error in response.get("errors", []) if error.get("type") != "NOT_FOUND"]
        if errors:
            raise ValueError(json.dumps(errors))
        data = response["data"]
        archived = {}
        for alias, key in aliases.items():
            repository = data[alias]
            value = None if repository is None else repository["isArchived"]
            if value is not None and not isinstance(value, bool):
                raise ValueError(f"{key}: invalid isArchived")
            archived[key] = value
        return archived
    except (ValueError, KeyError, TypeError, AttributeError) as error:
        detail = result.stderr.strip() if not result.stdout.strip() else str(error)
        raise ValueError(f"cannot read GitHub: {detail}") from error


def read_multica(api, multica_sync: ModuleType) -> tuple[list[dict], dict[str, list[dict]], str]:
    """Read the local omp daemon, every project and its nonempty resources."""
    runtimes = api.call("GET", "/api/runtimes")
    runtime_id = multica_sync.omp_runtime(runtimes)
    daemon = next(runtime["daemon_id"] for runtime in runtimes if runtime["id"] == runtime_id)
    if not daemon:
        raise ValueError("the local omp runtime has no daemon_id")
    projects = api.call("GET", "/api/projects")["projects"]
    resources = {
        project["id"]: api.call("GET", f"/api/projects/{project['id']}/resources")["resources"]
        if project["resource_count"] > 0 else []
        for project in projects
    }
    return projects, resources, daemon


# --- Writes and the run ---


def write_plan(api, desired: Plan, multica_sync: ModuleType) -> int:
    """Send each differing write, keeping other projects going on failure."""
    failed = False
    failed_take_overs: set[str] = set()
    for report in desired.reports:
        print(report.line, flush=True)
    for write in desired.writes:
        if isinstance(write, StatusMove) and write.project_id in failed_take_overs:
            continue
        try:
            if isinstance(write, Create):
                project = api.call("POST", "/api/projects", {"title": write.title, "status": IN_PROGRESS})
                project_id = project["id"]
                print(f"create project {write.key}: {project_id}", flush=True)
            else:
                project_id = write.project_id
            resource_path = f"/api/projects/{project_id}/resources"
            if isinstance(write, (Create, TakeOver)):
                api.call("POST", resource_path, {"resource_type": LOCAL_DIRECTORY, "resource_ref": write.resource.ref()})
                print(f"add resource {write.key}: {write.resource.local_path}", flush=True)
            elif isinstance(write, ResourceUpdate):
                api.call("PUT", f"{resource_path}/{write.resource_id}", {
                    "resource_ref": write.resource.ref(), "label": write.resource.label,
                })
                print(f"update resource {write.key}: {write.resource.local_path}", flush=True)
            else:
                api.call("PUT", f"/api/projects/{project_id}", {"status": write.status})
                print(f"move project {write.key}: {write.status}", flush=True)
        except (multica_sync.ApiError, OSError, ValueError, KeyError, TypeError) as error:
            failed = True
            if isinstance(write, TakeOver):
                failed_take_overs.add(write.project_id)
            print(f"failed: project {write.key}: {error}", file=sys.stderr, flush=True)
    return 1 if failed else 0


def main() -> int:
    try:
        team_state, multica_sync = load_dependencies()
    except (ImportError, OSError, ValueError) as error:
        print(f"failed: cannot load project sync readers: {error}", file=sys.stderr, flush=True)
        return 1
    try:
        with (multica_sync.CONFIG.parent / "projects.lock").open("a") as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            api = multica_sync.Multica(multica_sync.load_config(multica_sync.CONFIG))
            repositories = read_repositories(team_state)
            archived = read_archived(repositories)
            projects, resources, daemon = read_multica(api, multica_sync)
            desired = plan(repositories, archived, projects, resources, daemon)
            return write_plan(api, desired, multica_sync)
    except (multica_sync.ApiError, OSError, ValueError, KeyError, TypeError, subprocess.TimeoutExpired) as error:
        print(f"failed: project sync read: {error}", file=sys.stderr, flush=True)
        return 1


if __name__ == "__main__":
    sys.exit(main())
