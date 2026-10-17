import concurrent.futures
import json
import sys
import time
from copy import deepcopy
from pathlib import Path
from types import SimpleNamespace
from urllib.parse import parse_qs, urlsplit

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import multica_projects as sync  # noqa: E402
from multica_projects import Brain, Organisation, Product, Repository  # noqa: E402

DAEMON = "019fce17-f818-71b1-bc29-4d9229e2139a"
REPOS = Path("/Users/brian/Repos")
DAS = Organisation("brain-das", "De App Specialist", "DAS")
MADSPEC = Product("brain-madspec", "MadSpec", (DAS.id,))
HQ = Product("brain-hq", "DAS HQ", (DAS.id,))


def repository(name, product=MADSPEC.id, checkouts=None):
    return Repository("brain-" + name, "appboypov/" + name, () if product is None else (product,),
                      (REPOS / "Ventures" / name,) if checkouts is None else tuple(checkouts))


def brain(products=(MADSPEC,), repositories=(), organisations=(DAS,)):
    return Brain(tuple(organisations), tuple(products), tuple(repositories))


def passed_check(key):
    return {"id": "check-" + key, "status": "completed", "passed": True,
            "version_files": [], "install_commands": []}


class Workspace:
    """Plain server data used as the pure plan's inputs."""

    def __init__(self):
        self.organisations = []
        self.projects = []
        self.resources = {}
        self.repositories = []
        self.available = []
        self.checks = {}
        self.issues = {}
        self.marks = {"organisations": {}, "products": {}, "repositories": {}, "repository_projects": {}}

    def organisation(self, organisation_id="org-das", name=DAS.name, key="das", brain_id=None):
        self.organisations.append({"id": organisation_id, "name": name, "key": key})
        if brain_id:
            self.marks["organisations"][brain_id] = organisation_id
        return organisation_id

    def project(self, project_id, title, organisation_id=None, product_id=None, issue_count=0):
        self.projects.append({"id": project_id, "title": title, "organisation_id": organisation_id,
                              "created_at": "2026-10-01T00:00:00Z", "status": "paused", "description": "Keep me"})
        self.resources[project_id] = []
        self.issues[project_id] = [f"{project_id}-issue-{index}" for index in range(issue_count)]
        if product_id:
            self.marks["products"][product_id] = project_id
        return project_id

    def directory(self, project_id, key, path=None, daemon=DAEMON, mode="worktree"):
        resource = {"id": f"{project_id}-r{len(self.resources[project_id])}", "resource_type": "local_directory",
                    "resource_ref": {"local_path": str(path or REPOS / "Ventures" / key.rsplit("/", 1)[-1]),
                                     "daemon_id": daemon, "execution_mode": mode, "label": "madspec:" + key},
                    "label": "madspec:" + key}
        self.resources[project_id].append(resource)
        return resource

    def placed(self):
        self.organisation(brain_id=DAS.id)
        self.project("project-madspec", MADSPEC.name, "org-das", MADSPEC.id)
        return self

    def visible(self, key, default_branch="main"):
        self.available.append({"full_name": key, "default_branch": default_branch})
        self.checks[key.lower()] = passed_check(key)

    def kept(self, project_id="repository", key="appboypov/madspec", brain_id="brain-repository", issue_count=3):
        self.project(project_id, key.rsplit("/", 1)[-1], issue_count=issue_count)
        self.directory(project_id, key)
        self.marks["repository_projects"][brain_id] = project_id

    def plan(self, source=None):
        source = source or brain()
        return sync.plan(source, self.organisations, self.projects, self.repositories, self.available,
                         self.checks, {key: tuple(ids) for key, ids in self.issues.items()}, self.marks)


def test_first_run_creates_only_the_fourteen_linked_organisations_and_thirty_three_products():
    organisations = tuple(Organisation(f"org-{index}", f"Organisation {index}", f"K{index}") for index in range(48))
    products = tuple(Product(f"product-{index}", f"Product {index}", (organisations[index % 14].id,)) for index in range(33))

    desired = Workspace().plan(brain(products, organisations=organisations))

    assert len(desired.organisations) == 14
    assert len(desired.projects) == 33
    assert {value.key for value in desired.organisations} == {f"k{index}" for index in range(14)}
    assert {value.title for value in desired.projects} == {value.name for value in products}
    assert desired.repositories == desired.merges == desired.assignments == ()


@pytest.mark.parametrize("name,key", list(sync.KEYS.items()))
def test_key_table_for_organisations_without_a_linear_key(name, key):
    organisation = Organisation("org", name)
    desired = Workspace().plan(brain((Product("product", "Product", ("org",)),), organisations=(organisation,)))
    assert desired.organisations[0].key == key


def test_hand_made_organisation_is_taken_over_ignoring_case_and_its_key_stays():
    workspace = Workspace()
    workspace.organisation("org-manual", "de app specialist", "custom-key")
    desired = workspace.plan()
    assert desired.organisations[0].organisation_id == "org-manual"
    assert desired.projects[0].organisation == DAS.id
    assert desired.organisation_ids == {DAS.id: "org-manual"}
    assert workspace.organisations[0]["key"] == "custom-key"


def test_marked_organisation_keeps_brians_key_and_name_even_when_brain_name_changes():
    workspace = Workspace().placed()
    workspace.organisations[0].update(name="Brian's name", key="rr")
    desired = workspace.plan(brain(organisations=(Organisation(DAS.id, "Renamed in brain", "OTHER"),)))
    assert desired.writes == ()
    assert workspace.organisations[0]["key"] == "rr"


def test_product_with_three_repositories_and_product_without_repositories():
    repositories = tuple(repository(name) for name in ("madspec", "plannotator", "omp-anytype-inbox"))
    workspace = Workspace()
    for repo in repositories:
        workspace.visible(repo.key)
    desired = workspace.plan(brain((MADSPEC, HQ), repositories))
    assert {value.title for value in desired.projects} == {"MadSpec", "DAS HQ"}
    assert len(desired.repositories) == 3
    assert {value.product for value in desired.repositories} == {MADSPEC.id}
    assert desired.merges == ()


def test_new_product_changes_nothing_else():
    workspace = Workspace().placed()
    desired = workspace.plan(brain((MADSPEC, HQ)))
    assert desired.projects == (sync.PlaceProject(HQ.id, HQ.name, DAS.id, None),)
    assert desired.organisations == desired.assignments == desired.repositories == desired.merges == ()


def test_product_take_over_requires_exact_title_and_ignores_checkout_marks():
    workspace = Workspace()
    workspace.project("manual", "MadSpec")
    workspace.directory("manual", "appboypov/madspec")
    assert workspace.plan().projects[0].project_id == "manual"
    workspace.projects[0]["title"] = "madspec"
    assert workspace.plan().projects[0].project_id is None


def test_madspec_merges_its_three_marked_repository_projects():
    workspace = Workspace().placed()
    repositories = tuple(repository(name) for name in ("madspec", "plannotator", "omp-anytype-inbox"))
    for repo, count in zip(repositories, (10, 2, 1)):
        workspace.kept(repo.key.rsplit("/", 1)[-1], repo.key, repo.id, count)
        workspace.visible(repo.key)
    before = deepcopy((workspace.projects, workspace.resources, workspace.issues, workspace.marks))
    desired = workspace.plan(brain(repositories=repositories))
    assert len(desired.repositories) == len(desired.merges) == 3
    assert sum(len(merge.issue_ids) for merge in desired.merges) == 13
    assert {merge.product for merge in desired.merges} == {MADSPEC.id}
    assert desired.reports == ()
    assert (workspace.projects, workspace.resources, workspace.issues, workspace.marks) == before


def test_rebl_takes_its_repository_and_six_issues():
    organisation = Organisation("brain-rebl-records", "Rebl Records", "REBL")
    product = Product("brain-rebl", "Rebl", (organisation.id,))
    repo = repository("rebl", product.id)
    workspace = Workspace()
    workspace.kept("repository-rebl", repo.key, repo.id, 6)
    workspace.visible(repo.key)
    desired = workspace.plan(brain((product,), (repo,), (organisation,)))
    assert desired.projects == (sync.PlaceProject(product.id, product.name, organisation.id, None),)
    assert desired.repositories == (sync.RepositoryWrite(repo.id, product.id, repo.key,
                                                        "check-" + repo.key, "tags", ""),)
    assert desired.merges == (sync.Merge("repository-rebl", "rebl", product.id,
                                       tuple(workspace.issues["repository-rebl"]), (repo.id,)),)


def test_one_blocked_repository_does_not_block_an_independent_repository():
    workspace = Workspace().placed()
    first = repository("madspec")
    second = repository("plannotator", checkouts=[])
    workspace.kept("first", first.key, first.id, 10)
    workspace.kept("second", second.key, second.id, 2)
    workspace.visible(first.key)
    workspace.visible(second.key)
    desired = workspace.plan(brain(repositories=(first, second)))
    assert [value.brain_id for value in desired.repositories] == [first.id]
    assert [value.project_id for value in desired.merges] == ["first"]
    assert any("has no checkout" in report for report in desired.reports)


def test_repository_without_a_product_keeps_its_project_and_issues():
    workspace = Workspace().placed()
    repo = repository("das-duckmd-brain", product=None)
    workspace.kept("standalone", repo.key, repo.id, 8)
    before = deepcopy((workspace.projects, workspace.resources, workspace.issues))
    desired = workspace.plan(brain(repositories=(repo,)))
    assert desired.writes == ()
    assert f"{repo.key}: has no Product" in desired.reports
    assert "das-duckmd-brain: kept until its repositories are in their Product's project" in desired.reports
    assert (workspace.projects, workspace.resources, workspace.issues) == before


@pytest.mark.parametrize("checkouts,visible,passed,reason", [
    ([], True, True, "has no checkout"),
    ([REPOS / "one", REPOS / "two"], True, True, "has 2 checkouts"),
    ([REPOS / "one"], False, True, "the GitHub App cannot see it"),
    ([REPOS / "one"], True, False, "wrong_branch: feature/login"),
])
def test_repository_that_cannot_be_added_keeps_its_source(checkouts, visible, passed, reason):
    workspace = Workspace().placed()
    repo = repository("madspec", checkouts=checkouts)
    workspace.kept("repository", repo.key, repo.id)
    if visible:
        workspace.visible(repo.key)
        workspace.checks[repo.key].update(passed=passed, failures=[{"code": "wrong_branch", "detail": "feature/login"}])
    desired = workspace.plan(brain(repositories=(repo,)))
    assert desired.repositories == desired.merges == ()
    assert any(reason in report for report in desired.reports)


def test_same_repository_name_under_different_owners_is_not_ambiguous():
    workspace = Workspace().placed()
    repositories = (repository("site"), Repository("brain-other-site", "other/site", (HQ.id,), (REPOS / "Other/site",)))
    for index, repo in enumerate(repositories):
        workspace.kept(str(index), repo.key, repo.id)
        workspace.visible(repo.key)
    desired = workspace.plan(brain((MADSPEC, HQ), repositories))
    assert len(desired.repositories) == len(desired.merges) == 2
    assert desired.reports == ()


def test_merged_product_project_is_taken_over_by_title_with_its_own_checkout_mark():
    workspace = Workspace().placed()
    workspace.marks["products"] = {}
    repo = repository("madspec")
    workspace.directory("project-madspec", repo.key)
    workspace.repositories.append({"id": "repo", "full_name": repo.key, "project_id": "project-madspec"})
    desired = workspace.plan(brain(repositories=(repo,)))
    assert desired.projects == (sync.PlaceProject(MADSPEC.id, MADSPEC.name, DAS.id, "project-madspec"),)
    assert desired.repositories == desired.merges == ()


def test_repository_projects_are_not_inferred_from_checkout_labels():
    workspace = Workspace().placed()
    repo = repository("madspec")
    workspace.project("unmarked", "madspec", issue_count=1)
    workspace.directory("unmarked", repo.key)
    workspace.visible(repo.key)
    desired = workspace.plan(brain(repositories=(repo,)))
    assert len(desired.repositories) == 1
    assert desired.merges == desired.forget_repository_projects == ()
    assert workspace.marks["repository_projects"] == {}


def test_brians_project_organisation_is_kept_and_missing_organisation_is_assigned():
    workspace = Workspace().placed()
    workspace.projects[0]["organisation_id"] = "org-rebl"
    assert workspace.plan().writes == ()
    workspace.projects[0]["organisation_id"] = None
    assert workspace.plan().assignments == (sync.SetOrganisation(MADSPEC.id, DAS.id),)


def test_deleted_product_and_organisation_are_printed_on_every_run_and_left_alone():
    workspace = Workspace().placed()
    workspace.organisation("org-gone", "Profa", "prf", "brain-gone-org")
    workspace.project("project-gone", "Photo Prank", "org-gone", "brain-gone-product", 7)
    for _ in range(2):
        desired = workspace.plan()
        assert desired.writes == ()
        assert set(desired.reports) == {"Profa: has no Organisation in the brain", "Photo Prank: has no Product in the brain"}
    assert len(workspace.issues["project-gone"]) == 7


def test_renamed_product_follows_its_mark_without_renaming_or_recreating_project():
    workspace = Workspace().placed()
    source = brain((Product(MADSPEC.id, "Mad Spec", (DAS.id,)),))
    assert workspace.plan(source).writes == ()
    assert workspace.projects[0]["title"] == "MadSpec"


def test_missing_marked_rows_rebuild_by_name_and_exact_title():
    workspace = Workspace().placed()
    workspace.marks = {"organisations": {DAS.id: "deleted"}, "products": {MADSPEC.id: "deleted"}}
    desired = workspace.plan()
    assert desired.organisations[0].organisation_id == "org-das"
    assert desired.projects[0].project_id == "project-madspec"


def test_keyless_organisation_and_its_products_are_reported_without_writes():
    unknown = Organisation("unknown", "Unknown")
    desired = Workspace().plan(brain((Product("unknown-product", "Unknown Product", (unknown.id,)),), organisations=(unknown,)))
    assert desired.writes == ()
    assert set(desired.reports) == {"Unknown: has no key", "Unknown Product: Organisation Unknown has no key"}


def test_multiple_parent_links_are_reported_and_left_alone():
    source = brain((Product("multi", "Many", (DAS.id, "other")), MADSPEC),
                   (Repository("brain-multi", "appboypov/multi", (MADSPEC.id, "other"), (REPOS / "multi",)),))
    desired = Workspace().placed().plan(source)
    assert desired.writes == ()
    assert set(desired.reports) == {"Many: has more than one Organisation", "appboypov/multi: has more than one Product"}


def test_repository_gone_from_brain_keeps_its_marked_project_and_is_printed():
    workspace = Workspace().placed()
    workspace.repositories.append({"id": "repository-gone", "full_name": "appboypov/gone",
                                   "project_id": "project-madspec"})
    workspace.marks["repositories"]["brain-gone"] = "repository-gone"
    desired = workspace.plan()
    assert desired.writes == ()
    assert desired.reports == ("appboypov/gone: has no Repository in the brain",)


@pytest.mark.parametrize("install_command", ["", "npm ci"])
def test_existing_repository_keeps_its_checkout_policy_and_install_command(install_command):
    workspace = Workspace().placed()
    repo = repository("madspec")
    workspace.repositories.append({"id": "repo", "full_name": repo.key, "project_id": "project-madspec",
                                   "checkout_path": "/Code/madspec", "version_policy": "none",
                                   "install_command": install_command})
    before = deepcopy(workspace.repositories)
    desired = workspace.plan(brain(repositories=(repo,)))
    assert desired.writes == desired.reports == ()
    assert workspace.repositories == before


@pytest.mark.parametrize("files,commands,policy,command", [
    ([], [], "tags", ""),
    (["pubspec.yaml", "package.json"], [{"command": "flutter pub get"}, {"command": "npm ci"}],
     "pubspec.yaml", "flutter pub get"),
])
def test_repository_uses_the_first_version_file_and_install_command(files, commands, policy, command):
    workspace = Workspace().placed()
    repo = repository("madspec")
    workspace.visible(repo.key)
    workspace.checks[repo.key].update(version_files=files, install_commands=commands)
    desired = workspace.plan(brain(repositories=(repo,)))
    assert desired.repositories[0].version_policy == policy
    assert desired.repositories[0].install_command == command


def test_existing_repository_in_another_project_is_not_moved_or_merged():
    workspace = Workspace().placed()
    repo = repository("madspec")
    workspace.kept("source", repo.key, repo.id)
    workspace.project("brians-project", "Brian's project")
    workspace.repositories.append({"id": "repo", "full_name": repo.key, "project_id": "brians-project"})
    desired = workspace.plan(brain(repositories=(repo,)))
    assert desired.repositories == desired.merges == ()
    assert "appboypov/madspec: in Brian's project" in desired.reports


class FakeApiError(Exception):
    pass


class FakeMultica(Workspace):
    """External transport fake using the actual HTTP request/response shapes."""

    def __init__(self):
        super().__init__()
        self.calls = []
        self.writes = []
        self.fail_read = None
        self.fail_create = None
        self.fail_repository = False
        self.batch_updated = None
        self.polls = {}

    def call(self, method, path, body=None):
        self.calls.append((method, path, deepcopy(body)))
        parsed = urlsplit(path)
        check_path = "/api/runtimes/runtime/repository-checkouts"
        if method == "GET":
            if parsed.path == self.fail_read:
                raise FakeApiError("Multica is unreachable")
            if path == "/api/runtimes":
                return [{"id": "runtime", "daemon_id": DAEMON}]
            if path == "/api/organisations":
                return {"organisations": deepcopy(self.organisations)}
            if path == "/api/projects":
                return {"projects": deepcopy(self.projects)}
            if path == "/api/repositories":
                return {"repositories": deepcopy(self.repositories)}
            if path == "/api/repositories/available":
                return {"repositories": deepcopy(self.available)}
            if path.startswith(check_path + "/"):
                return deepcopy(self.polls[path.rsplit("/", 1)[1]].pop(0))
            if parsed.path == "/api/issues":
                query = parse_qs(parsed.query)
                ids = self.issues[query["project_id"][0]]
                offset, limit = int(query["offset"][0]), int(query["limit"][0])
                return {"issues": [{"id": value} for value in ids[offset:offset + limit]], "total": len(ids)}
            raise AssertionError(path)
        if method == "POST" and path == check_path:
            return deepcopy(self.checks[body["full_name"].lower()])
        self.writes.append((method, path, deepcopy(body)))
        if method == "POST" and path == "/api/organisations":
            organisation_id = self.organisation("new-org-" + body["name"], body["name"], body["key"])
            return next(value for value in self.organisations if value["id"] == organisation_id)
        if method == "POST" and path == "/api/projects":
            if body["title"] == self.fail_create:
                raise FakeApiError("create refused")
            project_id = self.project("new-" + body["title"], body["title"], body.get("organisation_id"))
            return next(value for value in self.projects if value["id"] == project_id)
        if method == "POST" and path == "/api/repositories":
            if self.fail_repository:
                self.fail_repository = False
                raise FakeApiError("repository refused")
            assert not any(value["full_name"].lower() == body["full_name"].lower() for value in self.repositories)
            value = {"id": "repo-" + body["full_name"], **deepcopy(body)}
            self.repositories.append(value)
            return value
        if path == "/api/issues/batch-update":
            ids = body["issue_ids"]
            moved = ids if not self.batch_updated else ids[:self.batch_updated.pop(0)]
            for issue_id in moved:
                source = next(values for values in self.issues.values() if issue_id in values)
                source.remove(issue_id)
                self.issues[body["updates"]["project_id"]].append(issue_id)
            return {"updated": len(moved)}
        project_id = parsed.path.split("/")[3]
        if method == "DELETE":
            self.projects = [value for value in self.projects if value["id"] != project_id]
            del self.resources[project_id]
            del self.issues[project_id]
            return None
        assert method == "PUT" and path == f"/api/projects/{project_id}", path
        project = next(value for value in self.projects if value["id"] == project_id)
        project.update(body)
        return project


class Shell:
    def __init__(self, monkeypatch, tmp_path):
        self.api = FakeMultica()
        self.objects = {
            "organisation": [{"id": DAS.id, "name": DAS.name, "properties": {"linear_key": DAS.linear_key}}],
            "product": [{"id": MADSPEC.id, "name": MADSPEC.name, "properties": {"organisation": [DAS.id]}}],
            "repository": [{"id": "brain-repository", "name": "madspec", "properties": {"github_key": "appboypov/madspec", "product": [MADSPEC.id]}}],
        }
        self.checkouts = {"appboypov/madspec": [REPOS / "Ventures/madspec"]}
        self.api.visible("appboypov/madspec")
        self.brain_error = None
        self.searches = []
        self.config = {"server_url": "http://test-server/", "workspace_id": "workspace"}
        self.mark_path = tmp_path / "state/brain-sync.json"
        team = SimpleNamespace(brain=SimpleNamespace(search=self.search),
                               model=SimpleNamespace(api_key=lambda: "fake", ModelError=FakeApiError),
                               REPOS_ROOT=REPOS, local_checkouts=lambda: self.checkouts)
        external = SimpleNamespace(CONFIG=tmp_path / "config.json", load_config=lambda path: self.config,
                                   Multica=lambda config: self.api, ApiError=FakeApiError,
                                   omp_runtime=lambda runtimes: runtimes[0]["id"])
        monkeypatch.setattr(sync, "MARK_FILE", self.mark_path)
        monkeypatch.setattr(sync, "load_dependencies", lambda: (team, external))

    def search(self, key, kind, fields):
        self.searches.append((kind, fields))
        if self.brain_error == kind:
            raise FakeApiError("Anytype is unreachable")
        return deepcopy(self.objects[kind])

    def keep_source(self, project_id="repository", key="appboypov/madspec", brain_id="brain-repository", issue_count=3):
        self.api.kept(project_id, key, brain_id, issue_count)
        self.save_marks()

    def add_repository(self, name, product=MADSPEC.id):
        key = "appboypov/" + name
        self.objects["repository"].append({"id": "brain-" + name, "properties": {"github_key": key, "product": [product]}})
        self.checkouts[key] = [REPOS / "Ventures" / name]
        self.api.visible(key)

    def state(self):
        return deepcopy((self.api.organisations, self.api.projects, self.api.repositories,
                         self.api.resources, self.api.issues))

    def save_marks(self):
        marks = sync.Marks(self.mark_path, self.config["server_url"], self.config["workspace_id"])
        for kind, values in self.api.marks.items():
            for brain_id, multica_id in values.items():
                marks.record(kind, brain_id, multica_id)


@pytest.fixture
def shell(monkeypatch, tmp_path):
    return Shell(monkeypatch, tmp_path)


def test_second_run_writes_nothing_and_preserves_status_description_and_names(shell):
    assert sync.main() == 0
    before = shell.state()
    writes = deepcopy(shell.api.writes)
    assert sync.main() == 0
    assert shell.state() == before
    assert shell.api.writes == writes
    assert all("status" not in (body or {}) for _, _, body in writes)
    assert shell.searches[:3] == [("organisation", ["linear_key"]), ("product", ["organisation"]), ("repository", ["github_key", "product"])]


def test_lost_mark_file_after_a_merge_keeps_the_product_project(shell):
    shell.keep_source()
    assert sync.main() == 0
    before = shell.state()
    writes = deepcopy(shell.api.writes)
    shell.mark_path.unlink()
    assert sync.main() == 0
    assert shell.state() == before
    assert shell.api.writes == writes
    marks = sync.Marks(shell.mark_path, shell.config["server_url"], shell.config["workspace_id"])
    assert marks.scope["products"] == {MADSPEC.id: "new-MadSpec"}


def test_madspec_adds_three_repositories_and_merges_their_marked_projects(shell):
    sources = ("madspec", "plannotator", "omp-anytype-inbox")
    for name, count in zip(sources, (10, 2, 1)):
        if name != "madspec":
            shell.add_repository(name)
        brain_id = "brain-repository" if name == "madspec" else "brain-" + name
        shell.keep_source(name, "appboypov/" + name, brain_id, count)
    assert sync.main() == 0
    assert [value["title"] for value in shell.api.projects] == ["MadSpec"]
    assert len(shell.api.issues["new-MadSpec"]) == 13
    assert {value["full_name"] for value in shell.api.repositories} == {"appboypov/" + name for name in sources}
    assert {value["project_id"] for value in shell.api.repositories} == {"new-MadSpec"}
    marks = sync.Marks(shell.mark_path, shell.config["server_url"], shell.config["workspace_id"])
    assert marks.scope["repository_projects"] == {}
    assert set(marks.scope["repositories"]) == {"brain-repository", "brain-plannotator", "brain-omp-anytype-inbox"}
    writes = deepcopy(shell.api.writes)
    assert sync.main() == 0
    assert shell.api.writes == writes


def test_unmarked_repository_projects_and_checkout_labels_stay_untouched(shell):
    shell.api.project("unmarked", "madspec", issue_count=2)
    shell.api.directory("unmarked", "appboypov/madspec")
    before = deepcopy((shell.api.projects[0], shell.api.resources["unmarked"], shell.api.issues["unmarked"]))
    assert sync.main() == 0
    assert (shell.api.projects[0], shell.api.resources["unmarked"], shell.api.issues["unmarked"]) == before
    marks = sync.Marks(shell.mark_path, shell.config["server_url"], shell.config["workspace_id"])
    assert marks.scope.get("repository_projects", {}) == {}
    assert not any(path.startswith("/api/issues") or method == "DELETE" for method, path, _ in shell.api.calls)


def test_rebl_merge_moves_six_issues_and_deletes_only_the_emptied_repository_project(shell):
    shell.objects["organisation"] = [{"id": "brain-rebl-org", "name": "Rebl Records", "properties": {"linear_key": "REBL"}}]
    shell.objects["product"] = [{"id": "brain-rebl", "name": "Rebl", "properties": {"organisation": ["brain-rebl-org"]}}]
    shell.objects["repository"] = [{"id": "brain-rebl-repo", "properties": {"github_key": "appboypov/rebl", "product": ["brain-rebl"]}}]
    shell.checkouts = {"appboypov/rebl": [REPOS / "Ventures/rebl"]}
    shell.api.visible("appboypov/rebl")
    shell.keep_source("repository-rebl", "appboypov/rebl", "brain-rebl-repo", 6)
    shell.api.project("manual", "Keep this", issue_count=4)
    assert sync.main() == 0
    assert {value["title"] for value in shell.api.projects} == {"Rebl", "Keep this"}
    assert len(shell.api.issues["new-Rebl"]) == 6
    assert shell.api.repositories[0]["project_id"] == "new-Rebl"
    assert shell.api.resources["new-Rebl"] == []
    assert len(shell.api.issues["manual"]) == 4
    check = shell.api.calls.index(("GET", "/api/issues?project_id=repository-rebl&limit=1&offset=0", None))
    delete = shell.api.calls.index(("DELETE", "/api/projects/repository-rebl", None))
    assert check < delete


def test_all_issue_ids_are_read_before_writes_and_moved_in_pages_of_100(shell):
    shell.keep_source(issue_count=201)
    assert sync.main() == 0
    batches = [body for method, path, body in shell.api.writes if path == "/api/issues/batch-update"]
    assert [len(value["issue_ids"]) for value in batches] == [100, 100, 1]
    reads = [index for index, (method, path, _) in enumerate(shell.api.calls) if path.startswith("/api/issues?") and f"limit={sync.PAGE_SIZE}&" in path]
    first_write = next(index for index, (method, path, _) in enumerate(shell.api.calls)
                       if method != "GET" and "/repository-checkouts" not in path)
    assert len(reads) == 3 and max(reads) < first_write
    assert len(shell.api.issues["new-MadSpec"]) == 201


@pytest.mark.parametrize("failure", ["partial", "late-issue", "fresh-read", "batch"])
def test_failed_moves_or_fresh_read_never_delete_the_repository_project(shell, monkeypatch, failure):
    shell.keep_source()
    original = shell.api.call
    def after_moves(method, path, body=None):
        if failure == "batch" and path == "/api/issues/batch-update":
            raise FakeApiError("move refused")
        result = original(method, path, body)
        if path == "/api/issues/batch-update" and failure == "late-issue":
            shell.api.issues["repository"].append("late-issue")
        if path == "/api/issues/batch-update" and failure == "fresh-read":
            shell.api.fail_read = "/api/issues"
        return result
    if failure == "partial":
        shell.api.batch_updated = [2]
    monkeypatch.setattr(shell.api, "call", after_moves)
    assert sync.main() == 1
    assert "repository" in {value["id"] for value in shell.api.projects}
    assert not any(method == "DELETE" for method, _, _ in shell.api.writes)


@pytest.mark.parametrize("source", [
    "organisation", "product", "repository", "empty-products", "runtimes", "organisations",
    "projects", "repositories", "available", "issues", "checkout-request", "checkout-poll", "marks",
])
def test_failed_read_writes_nothing(shell, monkeypatch, source):
    shell.keep_source()
    before = shell.state()
    if source in ("organisation", "product", "repository"):
        shell.brain_error = source
    elif source == "empty-products":
        shell.objects["product"] = []
    elif source in ("checkout-request", "checkout-poll"):
        original = shell.api.call
        if source == "checkout-poll":
            shell.api.checks["appboypov/madspec"]["status"] = "running"
            monkeypatch.setattr(sync.time, "sleep", lambda _: None)
        def refused_check(method, path, body=None):
            if "/repository-checkouts" in path and method == ("POST" if source == "checkout-request" else "GET"):
                raise FakeApiError("checkout read refused")
            return original(method, path, body)
        monkeypatch.setattr(shell.api, "call", refused_check)
    elif source == "marks":
        shell.mark_path.write_text("not JSON")
    else:
        shell.api.fail_read = {"runtimes": "/api/runtimes", "organisations": "/api/organisations", "projects": "/api/projects",
                               "repositories": "/api/repositories", "available": "/api/repositories/available",
                               "issues": "/api/issues"}[source]
    assert sync.main() == 1
    assert shell.state() == before
    assert shell.api.writes == []


def test_failed_repository_add_keeps_source_and_next_run_heals_without_duplicate(shell):
    shell.keep_source(issue_count=2)
    shell.api.fail_repository = True
    assert sync.main() == 1
    assert "repository" in {value["id"] for value in shell.api.projects}
    assert sync.main() == 0
    assert [value["title"] for value in shell.api.projects] == ["MadSpec"]
    assert len(shell.api.issues["new-MadSpec"]) == 2
    writes = deepcopy(shell.api.writes)
    assert sync.main() == 0
    assert shell.api.writes == writes


def test_failed_project_creation_does_not_stop_independent_product(shell):
    shell.objects["product"].append({"id": HQ.id, "name": HQ.name, "properties": {"organisation": [DAS.id]}})
    shell.api.fail_create = "MadSpec"
    assert sync.main() == 1
    assert [value["title"] for value in shell.api.projects] == ["DAS HQ"]


def test_product_without_repositories_needs_no_checkout_check_or_runtime(shell):
    shell.objects["repository"] = []
    assert sync.main() == 0
    assert shell.api.repositories == []
    assert not any(path.startswith("/api/runtimes") for _, path, _ in shell.api.calls)


def test_archive_checkouts_are_excluded_and_mixed_case_keys_match_the_github_app(shell):
    shell.checkouts["appboypov/madspec"].append(REPOS / "Archive/madspec")
    shell.objects["repository"][0]["properties"]["github_key"] = "AppBoyPov/MadSpec"
    assert sync.main() == 0
    checks = [body for method, path, body in shell.api.calls if method == "POST" and path.endswith("/repository-checkouts")]
    assert checks == [{"full_name": "appboypov/madspec", "default_branch": "main",
                       "path": str(REPOS / "Ventures/madspec"), "clone": False}]
    assert len(shell.api.repositories) == 1


def test_same_github_key_is_added_once_and_each_brain_identity_is_recorded(shell):
    shell.objects["repository"].append({"id": "brain-alias", "properties": {"github_key": "AppBoyPov/MadSpec", "product": [MADSPEC.id]}})
    shell.keep_source(issue_count=2)
    shell.api.marks["repository_projects"]["brain-alias"] = "repository"
    shell.save_marks()
    assert sync.main() == 0
    assert len(shell.api.repositories) == 1
    assert len(shell.api.issues["new-MadSpec"]) == 2
    marks = sync.Marks(shell.mark_path, shell.config["server_url"], shell.config["workspace_id"])
    assert marks.scope["repositories"] == {"brain-repository": "repo-appboypov/madspec",
                                            "brain-alias": "repo-appboypov/madspec"}
    assert marks.scope["repository_projects"] == {}


def test_failed_repository_identity_save_keeps_projects_and_next_run_recovers(shell, monkeypatch):
    shell.keep_source(issue_count=10)
    original = sync.Marks.record
    def refused_mark(self, kind, brain_id, multica_id):
        if kind == "repositories":
            raise OSError("mark refused")
        return original(self, kind, brain_id, multica_id)
    monkeypatch.setattr(sync.Marks, "record", refused_mark)
    assert sync.main() == 1
    assert len(shell.api.issues["repository"]) == 10
    assert len(shell.api.repositories) == 1
    assert not any(path == "/api/issues/batch-update" or method == "DELETE" for method, path, _ in shell.api.writes)
    monkeypatch.setattr(sync.Marks, "record", original)
    assert sync.main() == 0
    marks = sync.Marks(shell.mark_path, shell.config["server_url"], shell.config["workspace_id"])
    assert marks.scope["repository_projects"] == {}
    assert len(shell.api.issues["new-MadSpec"]) == 10
    assert len(shell.api.repositories) == 1


@pytest.mark.parametrize("entries", [[], {"brain-repository": 42}])
def test_invalid_repository_project_marks_fail_before_server_writes(shell, entries):
    shell.mark_path.parent.mkdir()
    shell.mark_path.write_text(json.dumps({"http://test-server": {"workspace": {"repository_projects": entries}}}))
    assert sync.main() == 1
    assert shell.api.calls == shell.api.writes == []


def test_repository_hidden_from_github_app_keeps_source_until_a_later_run(shell, capsys):
    shell.api.available = []
    shell.keep_source(issue_count=1)
    assert sync.main() == 0
    assert len(shell.api.issues["repository"]) == 1
    assert shell.api.repositories == []
    assert "the GitHub App cannot see it" in capsys.readouterr().out
    shell.api.visible("appboypov/madspec")
    assert sync.main() == 0
    assert len(shell.api.issues["new-MadSpec"]) == 1


def test_overlapping_syncs_create_only_one_managed_project(shell, monkeypatch):
    original = shell.api.call
    def delayed_snapshot(method, path, body=None):
        snapshot = original(method, path, body)
        if method == "GET" and path == "/api/projects":
            time.sleep(0.1)
        return snapshot
    monkeypatch.setattr(shell.api, "call", delayed_snapshot)
    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
        assert list(pool.map(lambda _: sync.main(), range(2))) == [0, 0]
    assert [value["title"] for value in shell.api.projects] == ["MadSpec"]
    assert len(shell.api.organisations) == 1
    assert len(shell.api.repositories) == 1


def test_mark_file_separates_servers_and_workspaces_and_preserves_concurrent_profiles(tmp_path):
    path = tmp_path / "brain-sync.json"
    a = sync.Marks(path, "http://one/", "workspace-a")
    b = sync.Marks(path, "http://one", "workspace-b")
    c = sync.Marks(path, "http://two", "workspace-a")
    a.record("products", "product", "project-a")
    b.record("products", "product", "project-b")
    c.record("organisations", "organisation", "org-c")
    assert sync.Marks(path, "http://one", "workspace-a").scope == {"products": {"product": "project-a"}}
    assert sync.Marks(path, "http://one", "workspace-b").scope == {"products": {"product": "project-b"}}
    assert sync.Marks(path, "http://two", "workspace-a").scope == {"organisations": {"organisation": "org-c"}}
    assert list(tmp_path.glob(".brain-sync-*")) == []


def test_failed_atomic_replace_keeps_saved_marks_and_removes_temporary_file(tmp_path, monkeypatch):
    path = tmp_path / "brain-sync.json"
    marks = sync.Marks(path, "http://one", "workspace")
    marks.record("products", "product", "project")
    before = path.read_bytes()
    def refused_replace(*args):
        raise OSError("replace refused")
    monkeypatch.setattr(sync.os, "replace", refused_replace)
    with pytest.raises(OSError):
        marks.record("products", "other-product", "other-project")
    assert path.read_bytes() == before
    assert marks.scope == {"products": {"product": "project"}}
    assert list(tmp_path.glob(".brain-sync-*")) == []


def test_conflicting_retained_project_marks_cannot_choose_one_parent():
    workspace = Workspace().placed()
    workspace.kept("mixed", "appboypov/madspec", "brain-madspec", 5)
    workspace.marks["repository_projects"]["brain-hq"] = "mixed"
    workspace.visible("appboypov/madspec")
    workspace.visible("appboypov/hq")
    desired = workspace.plan(brain((MADSPEC, HQ), (repository("madspec"), repository("hq", HQ.id))))
    assert desired.merges == ()
    assert "madspec: kept until its repositories are in their Product's project" in desired.reports


def test_existing_product_project_cannot_be_deleted_by_a_changed_repository_link():
    workspace = Workspace().placed()
    workspace.marks["repository_projects"]["brain-madspec"] = "project-madspec"
    workspace.visible("appboypov/madspec")
    desired = workspace.plan(brain((MADSPEC, HQ), (repository("madspec", HQ.id),)))
    assert desired.merges == ()
    assert any("marked Product project" in report for report in desired.reports)


def test_failed_organisation_create_skips_dependent_projects_and_merges(shell, monkeypatch):
    shell.keep_source(issue_count=1)
    original = shell.api.call
    def refused_create(method, path, body=None):
        if method == "POST" and path == "/api/organisations":
            raise FakeApiError("organisation refused")
        return original(method, path, body)
    monkeypatch.setattr(shell.api, "call", refused_create)
    assert sync.main() == 1
    assert [value["title"] for value in shell.api.projects] == ["madspec"]
    assert shell.api.issues["repository"] == ["repository-issue-0"]
    assert not any(method == "DELETE" for method, _, _ in shell.api.writes)


def test_failed_project_mark_keeps_source_and_next_run_takes_over_unmarked_creation(shell, monkeypatch):
    shell.keep_source(issue_count=1)
    original = sync.Marks.record
    def refused_mark(self, kind, brain_id, multica_id):
        if kind == "products":
            raise OSError("mark refused")
        return original(self, kind, brain_id, multica_id)
    monkeypatch.setattr(sync.Marks, "record", refused_mark)
    assert sync.main() == 1
    assert {value["title"] for value in shell.api.projects} == {"madspec", "MadSpec"}
    assert not any(method == "DELETE" for method, _, _ in shell.api.writes)
    monkeypatch.setattr(sync.Marks, "record", original)
    assert sync.main() == 0
    assert [value["title"] for value in shell.api.projects] == ["MadSpec"]
    assert len(shell.api.issues["new-MadSpec"]) == 1


def test_partial_move_of_101_issues_keeps_source_and_next_run_moves_the_rest(shell):
    shell.keep_source(issue_count=101)
    shell.api.batch_updated = [100, 0]
    assert sync.main() == 1
    assert len(shell.api.issues["new-MadSpec"]) == 100
    assert shell.api.issues["repository"] == ["repository-issue-100"]
    assert not any(method == "DELETE" for method, _, _ in shell.api.writes)
    moves = len(shell.api.writes)
    assert sync.main() == 0
    batches = [body["issue_ids"] for _, path, body in shell.api.writes[moves:] if path == "/api/issues/batch-update"]
    assert batches == [["repository-issue-100"]]
    assert len(shell.api.issues["new-MadSpec"]) == 101
    assert "repository" not in shell.api.issues


def test_checkout_check_polls_flat_result_and_uses_github_app_branch(shell, monkeypatch):
    key = "appboypov/madspec"
    check = shell.api.checks[key]
    shell.api.available[0].update(full_name="AppBoyPov/MadSpec", default_branch="trunk")
    completed = {**check, "version_files": ["package.json"], "install_commands": [{"command": "npm ci"}]}
    shell.api.polls["checkout"] = [{**check, "id": "checkout", "status": "running"},
                                  {**completed, "id": "checkout"}]
    check.update(id="checkout", status="pending", passed=False)
    sleeps = []
    monkeypatch.setattr(sync.time, "sleep", sleeps.append)
    assert sync.main() == 0
    assert sleeps == [0.5, 0.5]
    assert shell.api.repositories == [{
        "id": "repo-AppBoyPov/MadSpec", "project_id": "new-MadSpec", "full_name": "AppBoyPov/MadSpec",
        "checkout_check_id": "checkout", "version_policy": "package.json", "install_command": "npm ci",
    }]
    request = next(body for method, path, body in shell.api.calls
                   if method == "POST" and path.endswith("/repository-checkouts"))
    assert request == {"full_name": "AppBoyPov/MadSpec", "default_branch": "trunk",
                       "path": str(REPOS / "Ventures/madspec"), "clone": False}


def test_checkout_timeout_is_bounded_and_keeps_its_source(shell, monkeypatch, capsys):
    shell.keep_source()
    shell.api.checks["appboypov/madspec"].update(status="running", passed=False)
    clock = iter((0, sync.CHECKOUT_WAIT_SECONDS))
    monkeypatch.setattr(sync.time, "monotonic", lambda: next(clock))
    assert sync.main() == 0
    assert "checkout check timed out" in capsys.readouterr().out
    assert shell.api.repositories == []
    assert len(shell.api.issues["repository"]) == 3
    assert not any(method == "DELETE" or path == "/api/issues/batch-update" for method, path, _ in shell.api.writes)


def test_existing_repository_skips_checkout_checks_and_readds_after_removal(shell):
    assert sync.main() == 0
    shell.api.repositories[0].update(install_command="", version_policy="none", checkout_path="/Code/madspec")
    before = deepcopy(shell.api.repositories)
    calls = len(shell.api.calls)
    assert sync.main() == 0
    assert shell.api.repositories == before
    assert not any(path.startswith("/api/runtimes") for _, path, _ in shell.api.calls[calls:])
    shell.api.repositories.clear()
    assert sync.main() == 0
    assert len(shell.api.repositories) == 1


def test_missing_retained_source_drops_only_its_mark(shell):
    shell.api.marks["repository_projects"]["brain-repository"] = "missing"
    shell.save_marks()
    assert sync.main() == 0
    marks = sync.Marks(shell.mark_path, shell.config["server_url"], shell.config["workspace_id"])
    assert marks.scope["repository_projects"] == {}
    assert not any(method == "DELETE" for method, _, _ in shell.api.writes)
