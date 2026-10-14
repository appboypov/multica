#!/usr/bin/env python3
# /// script
# requires-python = ">=3.11"
# dependencies = ["grpcio"]
# ///
"""Fill Multica's organisations and Product projects from the brain, by hand.

Read, plan, then write: organisations, projects, repositories and guarded
merges of the repository projects UJ-1 kept. Identity lives in brain-sync.json.
See specs/multica-projects.md for the contract.

Usage from the Multica checkout: uv run madspec-sync/multica_projects.py
"""

from __future__ import annotations

import fcntl
import importlib.util
import json
import os
import re
import sys
import tempfile
import time
from copy import deepcopy
from dataclasses import dataclass, field
from pathlib import Path
from types import ModuleType
from urllib.parse import urlencode

sys.dont_write_bytecode = True

PAGE_SIZE = 100
CHECKOUT_WAIT_SECONDS = 120
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
class RepositoryWrite:
    brain_id: str
    product: str
    full_name: str
    checkout_check_id: str
    version_policy: str
    install_command: str


@dataclass(frozen=True)
class Merge:
    project_id: str
    title: str
    product: str
    issue_ids: tuple[str, ...]
    repositories: tuple[str, ...]


@dataclass(frozen=True)
class Plan:
    organisations: tuple[PlaceOrganisation, ...] = ()
    projects: tuple[PlaceProject, ...] = ()
    assignments: tuple[SetOrganisation, ...] = ()
    repositories: tuple[RepositoryWrite, ...] = ()
    merges: tuple[Merge, ...] = ()
    reports: tuple[str, ...] = ()
    organisation_ids: dict[str, str] = field(default_factory=dict)
    project_ids: dict[str, str] = field(default_factory=dict)
    forget_repository_projects: tuple[str, ...] = ()

    @property
    def writes(self) -> tuple:
        return (*self.organisations, *self.projects, *self.assignments, *self.repositories, *self.merges)


def plan(brain: Brain, organisations: list[dict], projects: list[dict],
         repositories: list[dict], available: list[dict], checks: dict[str, dict],
         issues: dict[str, tuple[str, ...]], marks: dict) -> Plan:
    """Pure plan from a complete read, with no changes to existing repositories."""
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
    # A marked identity belongs to its Product even when that Product has left.
    claimed = set(product_marks.values()) & set(current_projects)
    for product in valid_products.values():
        org_id = product.organisations[0]
        if org_id not in available_orgs:
            continue
        current = current_projects.get(product_marks.get(product.id))
        if current is None:
            current = next((value for value in ordered
                            if value["id"] not in claimed and value["title"] == product.name), None)
        if current is not None:
            claimed.add(current["id"])
            project_ids[product.id] = current["id"]
            if not current.get("organisation_id"):
                assignments.append(SetOrganisation(product.id, org_id))
        if current is None or product_marks.get(product.id) != current["id"]:
            project_placements.append(PlaceProject(product.id, product.name, org_id, None if current is None else current["id"]))
    available_products = set(project_ids) | {value.brain_id for value in project_placements}
    brain_repos = {repo.id: repo for repo in brain.repositories}
    current_repos = {repo["full_name"].lower(): repo for repo in repositories}
    visible = {repo["full_name"].lower(): repo for repo in available}
    by_key = {}
    for repo in brain.repositories:
        by_key.setdefault(repo.key.lower(), []).append(repo)
    for brain_id, repository_id in sorted(marks.get("repositories", {}).items()):
        if brain_id not in brain_repos:
            current = next((repo for repo in repositories if repo["id"] == repository_id), None)
            if current is not None:
                reports.append(f"{current['full_name']}: has no Repository in the brain")

    repository_writes, placed_repositories = [], {}
    for repo in brain.repositories:
        key = repo.key.lower()
        if len(repo.products) != 1:
            reason = "has more than one Product" if repo.products else "has no Product"
            reports.append(f"{repo.key}: {reason}")
            continue
        product_id = repo.products[0]
        if any(other.products != repo.products for other in by_key[key]):
            reports.append(f"{repo.key}: links more than one Product")
            continue
        if product_id not in available_products:
            reports.append(f"{repo.key}: has no placed Product")
            continue
        current = current_repos.get(key)
        if current is not None:
            if current["project_id"] != project_ids.get(product_id):
                project = current_projects.get(current["project_id"])
                reports.append(f"{repo.key}: in {project['title'] if project else current['project_id']}")
            else:
                placed_repositories[repo.id] = product_id
            continue
        if key not in visible:
            reports.append(f"{repo.key}: the GitHub App cannot see it")
            continue
        if not repo.checkouts:
            reports.append(f"{repo.key}: has no checkout")
            continue
        if len(repo.checkouts) != 1:
            reports.append(f"{repo.key}: has {len(repo.checkouts)} checkouts")
            continue
        check = checks[key]
        if check["status"] != "completed" or check.get("passed") is not True:
            failures = check.get("failures") or []
            detail = "; ".join(f"{failure['code']}: {failure.get('detail', '')}" for failure in failures)
            reports.append(f"{repo.key}: checkout failed: {detail or check.get('error') or check['status']}")
            continue
        files = check.get("version_files") or []
        commands = check.get("install_commands") or []
        repository_writes.append(RepositoryWrite(
            repo.id, product_id, visible[key]["full_name"], check["id"],
            files[0] if files else "tags", commands[0]["command"] if commands else "",
        ))
        placed_repositories[repo.id] = product_id

    # One retained project can have multiple marks. It is deleted only when
    # all of them identify repositories placed on the same Product project.
    kept = {}
    forgotten = []
    for brain_id, project_id in sorted(marks.get("repository_projects", {}).items()):
        if project_id not in current_projects:
            forgotten.append(brain_id)
        else:
            kept.setdefault(project_id, []).append(brain_id)
    merges = []
    for project_id, repository_brain_ids in kept.items():
        project = current_projects[project_id]
        products = {placed_repositories.get(brain_id) for brain_id in repository_brain_ids}
        if None in products or len(products) != 1:
            reports.append(f"{project['title']}: kept until its repositories are in their Product's project")
            continue
        (product_id,) = products
        if project_id == project_ids.get(product_id):
            forgotten.extend(repository_brain_ids)
            continue
        if project_id in claimed:
            reports.append(f"{project['title']}: is a marked Product project; project left alone")
            continue
        merges.append(Merge(project_id, project["title"], product_id,
                            issues[project_id], tuple(repository_brain_ids)))
    return Plan(
        organisations=tuple(org_placements), projects=tuple(project_placements),
        assignments=tuple(assignments), repositories=tuple(repository_writes),
        merges=tuple(merges), reports=tuple(reports), organisation_ids=org_ids,
        project_ids=project_ids,
        forget_repository_projects=tuple(forgotten),
    )


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


def read_multica(api, multica_sync: ModuleType, brain: Brain, marks: dict) -> tuple:
    organisations = api.call("GET", "/api/organisations")["organisations"]
    projects = api.call("GET", "/api/projects")["projects"]
    repositories = api.call("GET", "/api/repositories")["repositories"]
    available = api.call("GET", "/api/repositories/available")["repositories"]
    kept_projects = set(marks.get("repository_projects", {}).values())
    issues = {project["id"]: read_issue_ids(api, project["id"]) for project in projects
              if project["id"] in kept_projects}
    current = {repo["full_name"].lower() for repo in repositories}
    visible = {repo["full_name"].lower(): repo for repo in available}
    candidates = {repo.key.lower(): repo for repo in brain.repositories
                  if repo.key.lower() not in current and repo.key.lower() in visible
                  and len(repo.products) == 1 and len(repo.checkouts) == 1}
    checks = {}
    if candidates:
        runtimes = api.call("GET", "/api/runtimes")
        runtime_id = multica_sync.omp_runtime(runtimes)
        runtime = next(runtime for runtime in runtimes if runtime["id"] == runtime_id)
        if not runtime.get("daemon_id"):
            raise ValueError("the local omp runtime has no daemon_id")
        path = f"/api/runtimes/{runtime_id}/repository-checkouts"
        for key, repo in candidates.items():
            check = api.call("POST", path, {
                "full_name": visible[key]["full_name"],
                "default_branch": visible[key]["default_branch"],
                "path": str(repo.checkouts[0]), "clone": False,
            })
            deadline = time.monotonic() + CHECKOUT_WAIT_SECONDS
            while check["status"] in ("pending", "running"):
                if time.monotonic() >= deadline:
                    check = {**check, "status": "timeout", "error": "checkout check timed out"}
                    break
                time.sleep(0.5)
                check = api.call("GET", f"{path}/{check['id']}")
            checks[key] = check
    return organisations, projects, repositories, available, checks, issues


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
                for kind in ("organisations", "products", "repository_projects", "repositories"):
                    entries = scope.get(kind, {})
                    if not isinstance(entries, dict) or any(not isinstance(key, str) or not isinstance(value, str) for key, value in entries.items()):
                        raise ValueError("invalid brain-sync identity map")

    @property
    def scope(self) -> dict:
        return self.data.get(self.server, {}).get(self.workspace, {})

    def record(self, kind: str, brain_id: str, multica_id: str | None) -> None:
        self.path.parent.mkdir(parents=True, exist_ok=True)
        # Other profiles share the file, not their profile's projects.lock.
        with self.path.with_suffix(".lock").open("a") as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            updated = deepcopy(Marks(self.path, self.server, self.workspace).data)
            entries = updated.setdefault(self.server, {}).setdefault(self.workspace, {}).setdefault(kind, {})
            if multica_id is None:
                entries.pop(brain_id, None)
            else:
                entries[brain_id] = multica_id
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
    failed_orgs, failed_products, failed_repositories = set(), set(), set()
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
    added_by_name = {}
    for repository in desired.repositories:
        if repository.product in failed_products or repository.product not in project_ids:
            failed_repositories.add(repository.brain_id)
            continue
        try:
            repository_id = added_by_name.get(repository.full_name.lower())
            if repository_id is None:
                repository_id = api.call("POST", "/api/repositories", {
                    "project_id": project_ids[repository.product],
                    "full_name": repository.full_name,
                    "checkout_check_id": repository.checkout_check_id,
                    "version_policy": repository.version_policy,
                    "install_command": repository.install_command,
                })["id"]
                added_by_name[repository.full_name.lower()] = repository_id
                print(f"add repository {repository.full_name}: {repository_id}", flush=True)
            marks.record("repositories", repository.brain_id, repository_id)
        except errors as error:
            failed = True
            failed_repositories.add(repository.brain_id)
            print(f"failed: repository {repository.full_name}: {error}", file=sys.stderr, flush=True)
    for brain_id in desired.forget_repository_projects:
        try:
            marks.record("repository_projects", brain_id, None)
        except errors as error:
            failed = True
            print(f"failed: remove repository project mark {brain_id}: {error}", file=sys.stderr, flush=True)
    for merge in desired.merges:
        if (merge.product in failed_products or merge.product not in project_ids
                or any(brain_id in failed_repositories for brain_id in merge.repositories)):
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
            for brain_id in merge.repositories:
                marks.record("repository_projects", brain_id, None)
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
            organisations, projects, repositories, available, checks, issues = read_multica(api, multica_sync, brain, marks.scope)
            desired = plan(brain, organisations, projects, repositories, available, checks, issues, marks.scope)
            return write_plan(api, desired, multica_sync, marks)
    except (multica_sync.ApiError, OSError, ValueError, KeyError, TypeError) as error:
        print(f"failed: project sync read: {error}", file=sys.stderr, flush=True)
        return 1


if __name__ == "__main__":
    sys.exit(main())
