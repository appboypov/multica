#!/usr/bin/env python3
# /// script
# requires-python = ">=3.11"
# dependencies = ["grpcio"]
# ///
"""Fill Multica's organisations and Product projects from the brain, by hand.

Read, plan, then write: organisations, projects, checkout resources, issue
merges and guarded project deletes. Identity lives in brain-sync.json; checkout
labels remain madspec:<owner/name>. See specs/multica-projects.md for the contract.

Usage from the Multica checkout: uv run madspec-sync/multica_projects.py
"""

from __future__ import annotations

import fcntl
import importlib.util
import json
import os
import re
import subprocess
import sys
import tempfile
from copy import deepcopy
from dataclasses import dataclass, field
from pathlib import Path
from types import ModuleType
from urllib.parse import urlencode

sys.dont_write_bytecode = True

MARKER_PREFIX = "madspec:"
LOCAL_DIRECTORY = "local_directory"
WORKTREE = "worktree"
GITHUB_READ_TIMEOUT = 60
PAGE_SIZE = 100
MARK_FILE = Path.home() / ".local/state/madspec/multica/brain-sync.json"
KEYS = {"Marvelous K": "mk", "Mens & Maatwerk": "mnm", "InclusiTalent": "incl", "Buttler": "btlr", "Whitebird": "wb"}


@dataclass(frozen=True)
class Organisation:
    id: str
    name: str
    linear_key: str = ""


@dataclass(frozen=True)
class Product:
    id: str
    name: str
    organisations: tuple[str, ...]


@dataclass(frozen=True)
class Repository:
    id: str
    key: str
    products: tuple[str, ...]
    checkouts: tuple[Path, ...]


@dataclass(frozen=True)
class Brain:
    organisations: tuple[Organisation, ...]
    products: tuple[Product, ...]
    repositories: tuple[Repository, ...]


@dataclass(frozen=True)
class LocalDirectory:
    local_path: str
    daemon_id: str
    label: str
    execution_mode: str = WORKTREE

    def ref(self) -> dict[str, str]:
        return {"local_path": self.local_path, "daemon_id": self.daemon_id,
                "label": self.label, "execution_mode": self.execution_mode}


@dataclass(frozen=True)
class PlaceOrganisation:
    brain_id: str
    name: str
    key: str
    organisation_id: str | None


@dataclass(frozen=True)
class PlaceProject:
    brain_id: str
    title: str
    organisation: str
    project_id: str | None


@dataclass(frozen=True)
class SetOrganisation:
    product: str
    organisation: str


@dataclass(frozen=True)
class ResourceWrite:
    key: str
    product: str
    resource: LocalDirectory
    resource_id: str | None = None


@dataclass(frozen=True)
class Merge:
    project_id: str
    title: str
    product: str
    issue_ids: tuple[str, ...]
    keys: tuple[str, ...]


@dataclass(frozen=True)
class Plan:
    organisations: tuple[PlaceOrganisation, ...] = ()
    projects: tuple[PlaceProject, ...] = ()
    assignments: tuple[SetOrganisation, ...] = ()
    resources: tuple[ResourceWrite, ...] = ()
    merges: tuple[Merge, ...] = ()
    reports: tuple[str, ...] = ()
    organisation_ids: dict[str, str] = field(default_factory=dict)
    project_ids: dict[str, str] = field(default_factory=dict)
    repository_projects: dict[str, str] = field(default_factory=dict)

    @property
    def writes(self) -> tuple:
        return (*self.organisations, *self.projects, *self.assignments, *self.resources, *self.merges)


def plan(brain: Brain, archived: dict[str, bool | None], organisations: list[dict],
         projects: list[dict], resources: dict[str, list[dict]],
         issues: dict[str, tuple[str, ...]], daemon: str, marks: dict) -> Plan:
    """Pure plan; only Products with one Repository place checkouts and merge."""
    reports = []
    org_by_id = {org.id: org for org in brain.organisations}
    product_by_id = {product.id: product for product in brain.products}
    current_orgs = {org["id"]: org for org in organisations}
    current_projects = {project["id"]: project for project in projects}
    org_marks, product_marks = marks.get("organisations", {}), marks.get("products", {})
    for brain_id, org_id in sorted(org_marks.items()):
        if brain_id not in org_by_id and org_id in current_orgs:
            reports.append(f"{current_orgs[org_id]['name']}: has no Organisation in the brain")
    for brain_id, project_id in sorted(product_marks.items()):
        if brain_id not in product_by_id and project_id in current_projects:
            reports.append(f"{current_projects[project_id]['title']}: has no Product in the brain")

    valid_products = {}
    for product in sorted(brain.products, key=lambda value: value.id):
        if len(product.organisations) != 1:
            reason = "has more than one Organisation" if product.organisations else "has no Organisation"
            reports.append(f"{product.name}: {reason}")
        elif product.organisations[0] not in org_by_id:
            reports.append(f"{product.name}: has no Organisation in the brain")
        else:
            valid_products[product.id] = product

    org_placements, org_ids = [], {}
    for brain_id in sorted({product.organisations[0] for product in valid_products.values()}):
        org = org_by_id[brain_id]
        current = current_orgs.get(org_marks.get(brain_id))
        if current is None:
            current = next((value for value in organisations if value["name"].casefold() == org.name.casefold()), None)
        key = org.linear_key.lower() or KEYS.get(org.name, "")
        if current is None and not key:
            reports.append(f"{org.name}: has no key")
            for product in valid_products.values():
                if product.organisations[0] == brain_id:
                    reports.append(f"{product.name}: Organisation {org.name} has no key")
            continue
        if current is None and not re.fullmatch(r"[a-z0-9][a-z0-9-]*", key):
            raise ValueError(f"{org.name}: invalid organisation key {key!r}")
        if current is not None:
            org_ids[brain_id] = current["id"]
        if current is None or org_marks.get(brain_id) != current["id"]:
            org_placements.append(PlaceOrganisation(brain_id, org.name, key, None if current is None else current["id"]))
    available_orgs = set(org_ids) | {value.brain_id for value in org_placements}

    ordered = sorted(projects, key=lambda value: (value.get("created_at") or "", value["id"]))
    project_placements, project_ids, assignments = [], {}, []
    counts = repository_counts(brain.repositories)
    # A merged Product project holds its only Repository's checkout mark, so a lost mark file still finds it.
    only_key = {repository.products[0]: repository.key.lower() for repository in brain.repositories
                if len(repository.products) == 1 and counts[repository.products[0]] == 1}
    # A marked identity belongs to its Product even when that Product has left.
    claimed = set(product_marks.values()) & set(current_projects)
    for product in valid_products.values():
        org_id = product.organisations[0]
        if org_id not in available_orgs:
            continue
        current = current_projects.get(product_marks.get(product.id))
        if current is None:
            current = next((value for value in ordered
                            if value["id"] not in claimed and value["title"] == product.name
                            and all(marker_key(resource) in (None, only_key.get(product.id))
                                    for resource in resources.get(value["id"], []))), None)
        if current is not None:
            claimed.add(current["id"])
            project_ids[product.id] = current["id"]
            if not current.get("organisation_id"):
                assignments.append(SetOrganisation(product.id, org_id))
        if current is None or product_marks.get(product.id) != current["id"]:
            project_placements.append(PlaceProject(product.id, product.name, org_id, None if current is None else current["id"]))
    available_products = set(project_ids) | {value.brain_id for value in project_placements}
    repositories_by_key = {}
    for repository in brain.repositories:
        repositories_by_key.setdefault(repository.key.lower(), []).append(repository)

    counting, repository_products = {}, {}
    for repository in sorted(brain.repositories, key=lambda value: value.key.lower()):
        key = repository.key.lower()
        if len(repository.products) != 1:
            reason = "has more than one Product" if repository.products else "has no Product"
            reports.append(f"{repository.key}: {reason}")
            continue
        product_id = repository.products[0]
        if any(other.products != repository.products for other in repositories_by_key[key]):
            reports.append(f"{repository.key}: checkout mark does not identify one Product")
            continue
        if product_id not in available_products:
            reports.append(f"{repository.key}: has no placed Product")
            continue
        repository_products[key] = product_id
        state = archived.get(key)
        if state is True:
            reports.append(f"{repository.key}: is archived on GitHub")
        elif not repository.checkouts:
            reports.append(f"{repository.key}: has no checkout")
        elif len(repository.checkouts) > 1:
            reports.append(f"{repository.key}: has more than one checkout: {', '.join(sorted(map(str, repository.checkouts)))}")
        elif state is None:
            reports.append(f"{repository.key}: is unknown to GitHub")
        else:
            counting[key] = repository.checkouts[0]
    by_name = {}
    for key in counting:
        by_name.setdefault(name_of(key).lower(), []).append(key)
    for key in list(counting):
        sharing = by_name[name_of(key).lower()]
        if len(sharing) > 1:
            reports.append(f"{key}: shares its name {name_of(key)} with {', '.join(other for other in sharing if other != key)}")
            del counting[key]

    resource_writes = []
    for key, checkout in sorted(counting.items()):
        product_id = repository_products[key]
        if counts[product_id] != 1:
            continue
        target_resources = resources.get(project_ids.get(product_id), [])
        current = next((resource for resource in target_resources if marker_key(resource) == key), None)
        wanted = LocalDirectory(str(checkout), daemon, MARKER_PREFIX + key)
        if current is None or any(resource_ref(current).get(field) != value for field, value in wanted.ref().items()):
            resource_writes.append(ResourceWrite(key, product_id, wanted, None if current is None else current["id"]))

    merges, kept_projects = [], {}
    present = {repository.key.lower() for repository in brain.repositories}
    for project in ordered:
        keys = sorted({key for resource in resources.get(project["id"], []) if (key := marker_key(resource))})
        for key in keys:
            if key not in present:
                reports.append(f"{key}: has no Repository in the brain")
        candidates = {repository_products[key] for key in keys if key in repository_products}
        if not candidates:
            continue
        if len(candidates) > 1 or any(key not in repository_products for key in keys):
            reports.append(f"{project['title']}: checkout marks do not identify one Product; project left alone")
            continue
        (product_id,) = candidates
        if project["id"] == project_ids.get(product_id):
            continue
        if project["id"] in claimed:
            reports.append(f"{project['title']}: is a marked Product project; project left alone")
            continue
        if counts[product_id] > 1:
            reports.append(f"{project['title']} stays for {product_by_id[product_id].name}, which has {counts[product_id]} repositories")
            for key in keys:
                for repository in repositories_by_key[key]:
                    kept_projects[repository.id] = project["id"]
            continue
        merges.append(Merge(project["id"], project["title"], product_id, issues[project["id"]], tuple(keys)))
    for key in sorted(counting):
        product_id = repository_products[key]
        if counts[product_id] > 1 and not any(repository.id in kept_projects for repository in repositories_by_key[key]):
            reports.append(f"{key}: gets no checkout, because {product_by_id[product_id].name} has {counts[product_id]} repositories")
    return Plan(tuple(org_placements), tuple(project_placements), tuple(assignments),
                tuple(resource_writes), tuple(merges), tuple(reports), org_ids, project_ids, kept_projects)


def repository_counts(repositories: tuple[Repository, ...]) -> dict[str, int]:
    counts = {}
    for repository in repositories:
        for product_id in repository.products:
            counts[product_id] = counts.get(product_id, 0) + 1
    return counts


def name_of(key: str) -> str:
    return key.rsplit("/", 1)[-1]


def resource_ref(resource: dict) -> dict:
    ref = resource.get("resource_ref")
    return ref if isinstance(ref, dict) else {}


def marker_key(resource: dict) -> str | None:
    """Resource column or ref label marks the same lowercase GitHub key."""
    if resource.get("resource_type") != LOCAL_DIRECTORY:
        return None
    for label in (resource.get("label"), resource_ref(resource).get("label")):
        if isinstance(label, str) and label.startswith(MARKER_PREFIX) and len(label) > len(MARKER_PREFIX):
            return label[len(MARKER_PREFIX):].lower()
    return None


# --- Reads ---


def load_dependencies() -> tuple[ModuleType, ModuleType]:
    multica_sync = load_module("madspec_projects_multica_sync", Path(__file__).resolve().parent / "multica_sync.py")
    team_state = load_module("madspec_projects_team_state", multica_sync.MADSPEC / "skills/madspec-find-next-chunk/scripts/team_state.py")
    return team_state, multica_sync


def load_module(name: str, path: Path) -> ModuleType:
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


def links(properties: dict, name: str) -> tuple[str, ...]:
    values = properties.get(name, [])
    if not isinstance(values, list) or any(not isinstance(value, str) or not value for value in values):
        raise ValueError(f"invalid {name} relationship in the brain")
    return tuple(dict.fromkeys(values))


def read_brain(team_state: ModuleType) -> Brain:
    try:
        key = team_state.model.api_key()
        organisations = team_state.brain.search(key, "organisation", fields=["linear_key"])
        products = team_state.brain.search(key, "product", fields=["organisation"])
        repositories = team_state.brain.search(key, "repository", fields=["github_key", "product"])
    except (team_state.model.ModelError, OSError, KeyError) as error:
        raise ValueError(f"cannot read the brain: {error}") from error
    if not products:
        raise ValueError("the brain holds no Product objects")
    checkouts = team_state.local_checkouts()
    archive = team_state.REPOS_ROOT / "Archive"
    found = []
    for repository in repositories:
        properties = repository["properties"]
        github_key = properties.get("github_key") or ""
        if not re.fullmatch(r"[\w.-]+/[\w.-]+", github_key):
            raise ValueError(f"Repository {repository['id']} has no valid GitHub key")
        normalized = github_key.lower()
        found.append(Repository(repository["id"], github_key, links(properties, "product"),
                                tuple(path for path in checkouts.get(normalized, []) if not path.is_relative_to(archive))))
    return Brain(tuple(Organisation(value["id"], value["name"], value["properties"].get("linear_key") or "") for value in organisations),
                 tuple(Product(value["id"], value["name"], links(value["properties"], "organisation")) for value in products),
                 tuple(sorted(found, key=lambda value: (value.key.lower(), value.id))))


def read_archived(repositories: tuple[Repository, ...]) -> dict[str, bool | None]:
    """One aliased GitHub query; NOT_FOUND leaves only that key unknown."""
    if not repositories:
        return {}
    aliases = {f"r{index}": key for index, key in enumerate(sorted({repository.key.lower() for repository in repositories}))}
    fields = []
    for alias, key in aliases.items():
        owner, name = key.split("/", 1)
        fields.append(f"{alias}: repository(owner: {json.dumps(owner)}, name: {json.dumps(name)}) {{ isArchived }}")
    result = subprocess.run(["gh", "api", "graphql", "-f", "query=query { " + " ".join(fields) + " }"],
                            capture_output=True, text=True, timeout=GITHUB_READ_TIMEOUT)
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
            archived[key.lower()] = value
        return archived
    except (ValueError, KeyError, TypeError, AttributeError) as error:
        detail = result.stderr.strip() if not result.stdout.strip() else str(error)
        raise ValueError(f"cannot read GitHub: {detail}") from error


def read_issue_ids(api, project_id: str) -> tuple[str, ...]:
    ids = []
    while True:
        query = urlencode({"project_id": project_id, "limit": PAGE_SIZE, "offset": len(ids), "sort": "created_at", "direction": "asc"})
        page = api.call("GET", "/api/issues?" + query)
        batch = [issue["id"] for issue in page["issues"]]
        ids.extend(batch)
        if len(ids) >= page["total"]:
            return tuple(ids)
        if not batch:
            raise ValueError(f"{project_id}: incomplete issue read")


def read_multica(api, multica_sync: ModuleType, brain: Brain) -> tuple:
    runtimes = api.call("GET", "/api/runtimes")
    runtime_id = multica_sync.omp_runtime(runtimes)
    daemon = next(runtime["daemon_id"] for runtime in runtimes if runtime["id"] == runtime_id)
    if not daemon:
        raise ValueError("the local omp runtime has no daemon_id")
    organisations = api.call("GET", "/api/organisations")["organisations"]
    projects = api.call("GET", "/api/projects")["projects"]
    resources = {project["id"]: api.call("GET", f"/api/projects/{project['id']}/resources")["resources"]
                 if project["resource_count"] > 0 else [] for project in projects}
    counts = repository_counts(brain.repositories)
    linked = {repository.key.lower() for repository in brain.repositories
              if len(repository.products) == 1 and counts[repository.products[0]] == 1}
    issues = {project["id"]: read_issue_ids(api, project["id"]) for project in projects
              if any(marker_key(resource) in linked for resource in resources[project["id"]])}
    return organisations, projects, resources, issues, daemon


class Marks:
    """Identity per server and workspace, atomically persisted after each placement."""

    def __init__(self, path: Path, server: str, workspace: str):
        self.path, self.server, self.workspace = path, server.rstrip("/"), workspace
        self.data = json.loads(path.read_text()) if path.exists() else {}
        if not isinstance(self.data, dict):
            raise ValueError("invalid brain-sync mark file")
        for workspaces in self.data.values():
            if not isinstance(workspaces, dict):
                raise ValueError("invalid brain-sync server marks")
            for scope in workspaces.values():
                if not isinstance(scope, dict):
                    raise ValueError("invalid brain-sync workspace marks")
                for kind in ("organisations", "products", "repository_projects"):
                    entries = scope.get(kind, {})
                    if not isinstance(entries, dict) or any(not isinstance(key, str) or not isinstance(value, str) for key, value in entries.items()):
                        raise ValueError("invalid brain-sync identity map")

    @property
    def scope(self) -> dict:
        return self.data.get(self.server, {}).get(self.workspace, {})

    def record(self, kind: str, brain_id: str, multica_id: str) -> None:
        self.path.parent.mkdir(parents=True, exist_ok=True)
        # Other profiles share the file, not their profile's projects.lock.
        with self.path.with_suffix(".lock").open("a") as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            updated = deepcopy(Marks(self.path, self.server, self.workspace).data)
            updated.setdefault(self.server, {}).setdefault(self.workspace, {}).setdefault(kind, {})[brain_id] = multica_id
            temporary = None
            try:
                with tempfile.NamedTemporaryFile(mode="w", dir=self.path.parent, prefix=".brain-sync-", delete=False) as stream:
                    temporary = Path(stream.name)
                    json.dump(updated, stream, indent=2, sort_keys=True)
                    stream.write("\n")
                    stream.flush()
                    os.fsync(stream.fileno())
                os.replace(temporary, self.path)
            finally:
                if temporary is not None:
                    temporary.unlink(missing_ok=True)
            self.data = updated


# --- Writes and the run ---


def write_plan(api, desired: Plan, multica_sync: ModuleType, marks: Marks) -> int:
    failed = False
    org_ids, project_ids = dict(desired.organisation_ids), dict(desired.project_ids)
    failed_orgs, failed_products, failed_resources = set(), set(), set()
    errors = (multica_sync.ApiError, OSError, ValueError, KeyError, TypeError)
    for report in desired.reports:
        print(report, flush=True)
    for placement in desired.organisations:
        try:
            organisation_id = placement.organisation_id
            if organisation_id is None:
                organisation_id = api.call("POST", "/api/organisations", {"name": placement.name, "key": placement.key})["id"]
                print(f"create organisation {placement.name}: {organisation_id}", flush=True)
            marks.record("organisations", placement.brain_id, organisation_id)
            org_ids[placement.brain_id] = organisation_id
            print(f"mark organisation {placement.name}: {organisation_id}", flush=True)
        except errors as error:
            failed = True
            failed_orgs.add(placement.brain_id)
            print(f"failed: organisation {placement.name}: {error}", file=sys.stderr, flush=True)
    for placement in desired.projects:
        if placement.organisation in failed_orgs:
            failed_products.add(placement.brain_id)
            continue
        try:
            project_id = placement.project_id
            if project_id is None:
                project_id = api.call("POST", "/api/projects", {"title": placement.title, "organisation_id": org_ids[placement.organisation]})["id"]
                print(f"create project {placement.title}: {project_id}", flush=True)
            marks.record("products", placement.brain_id, project_id)
            project_ids[placement.brain_id] = project_id
            print(f"mark project {placement.title}: {project_id}", flush=True)
        except errors as error:
            failed = True
            failed_products.add(placement.brain_id)
            print(f"failed: project {placement.title}: {error}", file=sys.stderr, flush=True)
    for assignment in desired.assignments:
        if assignment.product in failed_products or assignment.organisation in failed_orgs:
            continue
        try:
            api.call("PUT", f"/api/projects/{project_ids[assignment.product]}", {"organisation_id": org_ids[assignment.organisation]})
            print(f"set project organisation {project_ids[assignment.product]}: {org_ids[assignment.organisation]}", flush=True)
        except errors as error:
            failed = True
            failed_products.add(assignment.product)
            print(f"failed: project {assignment.product}: {error}", file=sys.stderr, flush=True)
    for brain_id, project_id in sorted(desired.repository_projects.items()):
        if marks.scope.get("repository_projects", {}).get(brain_id) == project_id:
            continue
        try:
            marks.record("repository_projects", brain_id, project_id)
            print(f"mark repository project {brain_id}: {project_id}", flush=True)
        except errors as error:
            failed = True
            print(f"failed: repository project mark {brain_id}: {error}", file=sys.stderr, flush=True)
    for resource in desired.resources:
        if resource.product in failed_products or resource.product not in project_ids:
            continue
        try:
            path = f"/api/projects/{project_ids[resource.product]}/resources"
            if resource.resource_id is None:
                api.call("POST", path, {"resource_type": LOCAL_DIRECTORY, "resource_ref": resource.resource.ref()})
                print(f"add resource {resource.key}: {resource.resource.local_path}", flush=True)
            else:
                api.call("PUT", f"{path}/{resource.resource_id}", {"resource_ref": resource.resource.ref(), "label": resource.resource.label})
                print(f"update resource {resource.key}: {resource.resource.local_path}", flush=True)
        except errors as error:
            failed = True
            failed_resources.add(resource.key)
            print(f"failed: resource {resource.key}: {error}", file=sys.stderr, flush=True)
    for merge in desired.merges:
        if merge.product in failed_products or merge.product not in project_ids or any(key in failed_resources for key in merge.keys):
            continue
        try:
            for offset in range(0, len(merge.issue_ids), PAGE_SIZE):
                page = merge.issue_ids[offset:offset + PAGE_SIZE]
                response = api.call("POST", "/api/issues/batch-update", {"issue_ids": list(page), "updates": {"project_id": project_ids[merge.product]}})
                if response["updated"] != len(page):
                    raise ValueError(f"only {response['updated']} of {len(page)} issues moved; repository project kept")
                print(f"move {len(page)} issues {merge.title}: {project_ids[merge.product]}", flush=True)
            # The issue list's total fails loudly; a project's issue_count reads 0 when its count query fails.
            remaining = api.call("GET", "/api/issues?" + urlencode({"project_id": merge.project_id, "limit": 1, "offset": 0}))["total"]
            if remaining != 0:
                raise ValueError(f"{remaining} issues remain; repository project kept")
            api.call("DELETE", f"/api/projects/{merge.project_id}")
            print(f"delete emptied project {merge.title}: {merge.project_id}", flush=True)
        except errors as error:
            failed = True
            print(f"failed: merge {merge.title}: {error}", file=sys.stderr, flush=True)
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
            config = multica_sync.load_config(multica_sync.CONFIG)
            marks = Marks(MARK_FILE, config["server_url"], config["workspace_id"])
            api = multica_sync.Multica(config)
            brain = read_brain(team_state)
            archived = read_archived(brain.repositories)
            organisations, projects, resources, issues, daemon = read_multica(api, multica_sync, brain)
            desired = plan(brain, archived, organisations, projects, resources, issues, daemon, marks.scope)
            return write_plan(api, desired, multica_sync, marks)
    except (multica_sync.ApiError, OSError, ValueError, KeyError, TypeError, subprocess.TimeoutExpired) as error:
        print(f"failed: project sync read: {error}", file=sys.stderr, flush=True)
        return 1


if __name__ == "__main__":
    sys.exit(main())
