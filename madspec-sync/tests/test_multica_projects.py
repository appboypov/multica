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


class Workspace:
    """Plain server data used as the pure plan's inputs."""

    def __init__(self):
        self.organisations = []
        self.projects = []
        self.resources = {}
        self.issues = {}
        self.marks = {"organisations": {}, "products": {}, "repository_projects": {}}

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
                    "resource_ref": {"local_path": str(path or REPOS / "Ventures" / sync.name_of(key)),
                                     "daemon_id": daemon, "execution_mode": mode, "label": "madspec:" + key},
                    "label": "madspec:" + key}
        self.resources[project_id].append(resource)
        return resource

    def placed(self):
        self.organisation(brain_id=DAS.id)
        self.project("project-madspec", MADSPEC.name, "org-das", MADSPEC.id)
        return self

    def plan(self, source=None, archived=None):
        source = source or brain()
        states = {repo.key.lower(): False for repo in source.repositories} if archived is None else archived
        return sync.plan(source, states, self.organisations, self.projects, self.resources,
                         {key: tuple(ids) for key, ids in self.issues.items()}, DAEMON, self.marks)


def test_first_run_creates_only_the_fourteen_linked_organisations_and_thirty_three_products():
    organisations = tuple(Organisation(f"org-{index}", f"Organisation {index}", f"K{index}") for index in range(48))
    products = tuple(Product(f"product-{index}", f"Product {index}", (organisations[index % 14].id,)) for index in range(33))

    desired = Workspace().plan(brain(products, organisations=organisations))

    assert len(desired.organisations) == 14
    assert len(desired.projects) == 33
    assert {value.key for value in desired.organisations} == {f"k{index}" for index in range(14)}
    assert {value.title for value in desired.projects} == {value.name for value in products}
    assert desired.resources == desired.merges == desired.assignments == ()


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
    desired = Workspace().plan(brain((MADSPEC, HQ), repositories))
    assert {value.title for value in desired.projects} == {"MadSpec", "DAS HQ"}
    assert desired.resources == desired.merges == ()
    assert desired.repository_projects == {}


def test_new_product_changes_nothing_else():
    workspace = Workspace().placed()
    desired = workspace.plan(brain((MADSPEC, HQ)))
    assert desired.projects == (sync.PlaceProject(HQ.id, HQ.name, DAS.id, None),)
    assert desired.organisations == desired.assignments == desired.resources == desired.merges == ()


def test_product_take_over_requires_exact_title_and_no_checkout_mark():
    workspace = Workspace()
    workspace.project("manual", "MadSpec")
    workspace.directory("manual", "unmarked")
    workspace.resources["manual"][0].update(label=None, resource_ref={"daemon_id": DAEMON})
    assert workspace.plan().projects[0].project_id == "manual"
    workspace.projects[0]["title"] = "madspec"
    assert workspace.plan().projects[0].project_id is None
    workspace.projects[0]["title"] = "MadSpec"
    workspace.directory("manual", "appboypov/madspec")
    assert workspace.plan().projects[0].project_id is None


def test_madspec_repository_projects_keep_ten_two_and_one_issues_and_are_recorded():
    workspace = Workspace().placed()
    repositories = tuple(repository(name) for name in ("madspec", "plannotator", "omp-anytype-inbox"))
    for repo, count in zip(repositories, (10, 2, 1)):
        project_id = workspace.project(sync.name_of(repo.key), sync.name_of(repo.key), issue_count=count)
        workspace.directory(project_id, repo.key)
    before = deepcopy((workspace.projects, workspace.resources, workspace.issues))
    for _ in range(2):
        desired = workspace.plan(brain(repositories=repositories))
        assert desired.resources == desired.merges == ()
        assert desired.repository_projects == {repo.id: sync.name_of(repo.key) for repo in repositories}
        assert set(desired.reports) == {
            f"{sync.name_of(repo.key)} stays for MadSpec, which has 3 repositories" for repo in repositories
        }
    assert (workspace.projects, workspace.resources, workspace.issues) == before


def test_rebl_takes_its_single_repository_checkout_and_six_issues():
    organisation = Organisation("brain-rebl-records", "Rebl Records", "REBL")
    product = Product("brain-rebl", "Rebl", (organisation.id,))
    repo = repository("rebl", product.id)
    workspace = Workspace()
    workspace.project("repository-rebl", "rebl", issue_count=6)
    workspace.directory("repository-rebl", repo.key)
    desired = workspace.plan(brain((product,), (repo,), (organisation,)))
    assert desired.projects == (sync.PlaceProject(product.id, product.name, organisation.id, None),)
    assert len(desired.resources) == 1
    assert desired.resources[0].product == product.id
    assert desired.resources[0].resource.ref() == sync.LocalDirectory(str(repo.checkouts[0]), DAEMON, "madspec:" + repo.key).ref()
    assert desired.merges == (sync.Merge("repository-rebl", "rebl", product.id,
                                       tuple(workspace.issues["repository-rebl"]), (repo.key,)),)
    assert desired.repository_projects == {}


@pytest.mark.parametrize("checkouts,state", [
    ([], False),
    ([REPOS / "one"], True),
    ([REPOS / "one"], None),
    ([REPOS / "one", REPOS / "two"], False),
])
def test_second_repository_counts_even_when_its_checkout_cannot_be_placed(checkouts, state):
    workspace = Workspace().placed()
    first = repository("madspec")
    second = repository("plannotator", checkouts=checkouts)
    workspace.project("repository", "madspec", issue_count=10)
    workspace.directory("repository", first.key)
    desired = workspace.plan(brain(repositories=(first, second)), {first.key: False, second.key: state})
    assert desired.resources == desired.merges == ()
    assert desired.repository_projects == {first.id: "repository"}
    assert "madspec stays for MadSpec, which has 2 repositories" in desired.reports


def test_repository_without_a_product_keeps_its_project_and_issues():
    workspace = Workspace().placed()
    repo = repository("das-duckmd-brain", product=None)
    workspace.project("standalone", "das-duckmd-brain", issue_count=8)
    workspace.directory("standalone", repo.key)
    before = deepcopy((workspace.projects, workspace.resources, workspace.issues))
    desired = workspace.plan(brain(repositories=(repo,)))
    assert desired.writes == ()
    assert desired.reports == (f"{repo.key}: has no Product",)
    assert (workspace.projects, workspace.resources, workspace.issues) == before


@pytest.mark.parametrize("checkouts,state,reason", [
    ([], False, "has no checkout"),
    ([REPOS / "one", REPOS / "two"], False, "has more than one checkout"),
    ([REPOS / "one"], None, "is unknown to GitHub"),
    ([REPOS / "one"], True, "is archived on GitHub"),
])
def test_non_counting_repository_still_merges_but_gets_no_product_checkout(checkouts, state, reason):
    workspace = Workspace().placed()
    repo = repository("madspec", checkouts=checkouts)
    workspace.project("repository", "madspec", issue_count=3)
    workspace.directory("repository", repo.key)
    desired = workspace.plan(brain(repositories=(repo,)), {repo.key: state})
    assert desired.resources == ()
    assert len(desired.merges) == 1
    assert any(reason in report for report in desired.reports)


def test_shared_repository_names_skip_checkouts_but_merge_single_repository_products():
    workspace = Workspace().placed()
    repositories = (repository("site"), Repository("brain-other-site", "other/site", (HQ.id,), (REPOS / "Other/site",)))
    for index, repo in enumerate(repositories):
        workspace.project(str(index), "site")
        workspace.directory(str(index), repo.key)
    desired = workspace.plan(brain((MADSPEC, HQ), repositories))
    assert desired.resources == ()
    assert len(desired.merges) == 2
    assert len([report for report in desired.reports if "shares its name" in report]) == 2


def test_merged_product_project_is_taken_over_by_title_with_its_own_checkout_mark():
    workspace = Workspace().placed()
    workspace.marks["products"] = {}
    repo = repository("madspec")
    workspace.directory("project-madspec", repo.key)
    desired = workspace.plan(brain(repositories=(repo,)))
    assert desired.projects == (sync.PlaceProject(MADSPEC.id, MADSPEC.name, DAS.id, "project-madspec"),)
    assert desired.resources == desired.merges == ()


def test_multi_repository_product_reports_a_repository_without_a_project():
    workspace = Workspace().placed()
    repositories = (repository("madspec"), repository("plannotator"))
    workspace.project("repository", "madspec", issue_count=1)
    workspace.directory("repository", repositories[0].key)
    desired = workspace.plan(brain(repositories=repositories))
    assert "appboypov/plannotator: gets no checkout, because MadSpec has 2 repositories" in desired.reports
    assert not any(report.startswith("appboypov/madspec:") for report in desired.reports)


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
    workspace.project("repository", "gone", issue_count=8)
    workspace.directory("repository", "appboypov/gone")
    desired = workspace.plan()
    assert desired.writes == ()
    assert desired.reports == ("appboypov/gone: has no Repository in the brain",)


@pytest.mark.parametrize("path,daemon,mode", [(REPOS / "Moved", DAEMON, "worktree"), (None, "previous-daemon", "worktree"), (None, DAEMON, "in_place")])
def test_checkout_changes_update_the_marked_resource_in_place(path, daemon, mode):
    workspace = Workspace().placed()
    repo = repository("madspec")
    current = workspace.directory("project-madspec", repo.key, path, daemon, mode)
    desired = workspace.plan(brain(repositories=(repo,)))
    assert len(desired.resources) == 1
    assert desired.resources[0].resource_id == current["id"]
    assert desired.resources[0].resource.ref() == sync.LocalDirectory(str(repo.checkouts[0]), DAEMON, "madspec:" + repo.key).ref()
    assert desired.merges == ()


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
        self.fail_resource = False
        self.batch_updated = None

    def call(self, method, path, body=None):
        self.calls.append((method, path, deepcopy(body)))
        parsed = urlsplit(path)
        if method == "GET":
            if parsed.path == self.fail_read:
                raise FakeApiError("Multica is unreachable")
            if path == "/api/runtimes":
                return [{"id": "runtime", "daemon_id": DAEMON}]
            if path == "/api/organisations":
                return {"organisations": deepcopy(self.organisations)}
            if path == "/api/projects":
                return {"projects": [{**project, "resource_count": len(self.resources[project['id']])} for project in self.projects]}
            if parsed.path == "/api/issues":
                query = parse_qs(parsed.query)
                ids = self.issues[query["project_id"][0]]
                offset, limit = int(query["offset"][0]), int(query["limit"][0])
                return {"issues": [{"id": value} for value in ids[offset:offset + limit]], "total": len(ids)}
            assert path.endswith("/resources"), path
            return {"resources": deepcopy(self.resources[parsed.path.split("/")[3]])}
        self.writes.append((method, path, deepcopy(body)))
        if method == "POST" and path == "/api/organisations":
            organisation_id = self.organisation("new-org-" + body["name"], body["name"], body["key"])
            return next(value for value in self.organisations if value["id"] == organisation_id)
        if method == "POST" and path == "/api/projects":
            if body["title"] == self.fail_create:
                raise FakeApiError("create refused")
            project_id = self.project("new-" + body["title"], body["title"], body.get("organisation_id"))
            return next(value for value in self.projects if value["id"] == project_id)
        if path == "/api/issues/batch-update":
            ids = body["issue_ids"]
            moved = ids if self.batch_updated is None else ids[:self.batch_updated]
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
        if method == "POST":
            if self.fail_resource:
                self.fail_resource = False
                raise FakeApiError("resource refused")
            if any(value["resource_type"] == "local_directory"
                   and value["resource_ref"].get("daemon_id") == body["resource_ref"].get("daemon_id")
                   for value in self.resources[project_id]):
                raise FakeApiError("this daemon already has a local_directory attached to the project")
            resource = {"id": f"{project_id}-resource-{len(self.resources[project_id])}", **deepcopy(body)}
            self.resources[project_id].append(resource)
            return resource
        if "/resources/" in path:
            resource = next(value for value in self.resources[project_id] if value["id"] == path.split("/")[-1])
            resource.update(deepcopy(body))
            return resource
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
        self.archived = {"appboypov/madspec": False}
        self.brain_error = None
        self.github_errors = []
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
        monkeypatch.setattr(sync.subprocess, "run", self.github)

    def search(self, key, kind, fields):
        self.searches.append((kind, fields))
        if self.brain_error == kind:
            raise FakeApiError("Anytype is unreachable")
        return deepcopy(self.objects[kind])

    def github(self, command, **kwargs):
        keys = sorted({value["properties"]["github_key"] for value in self.objects["repository"]})
        data = {f"r{index}": None if self.archived.get(key.lower()) is None else {"isArchived": self.archived[key.lower()]} for index, key in enumerate(keys)}
        return SimpleNamespace(stdout=json.dumps({"data": data, "errors": self.github_errors}), stderr="", returncode=bool(self.github_errors))

    def state(self):
        return deepcopy((self.api.organisations, self.api.projects, self.api.resources, self.api.issues))

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
    shell.api.project("repository", "madspec", issue_count=3)
    shell.api.directory("repository", "appboypov/madspec")
    assert sync.main() == 0
    before = shell.state()
    writes = deepcopy(shell.api.writes)
    shell.mark_path.unlink()
    assert sync.main() == 0
    assert shell.state() == before
    assert shell.api.writes == writes
    marks = sync.Marks(shell.mark_path, shell.config["server_url"], shell.config["workspace_id"])
    assert marks.scope["products"] == {MADSPEC.id: "new-MadSpec"}


def test_madspec_keeps_repository_projects_and_records_their_brain_ids_on_every_run(shell, capsys):
    sources = ("madspec", "plannotator", "omp-anytype-inbox")
    for name, count in zip(sources, (10, 2, 1)):
        if name != "madspec":
            key = "appboypov/" + name
            shell.objects["repository"].append({"id": "brain-" + name, "properties": {"github_key": key, "product": [MADSPEC.id]}})
            shell.checkouts[key] = [REPOS / "Ventures" / name]
            shell.archived[key] = False
        shell.api.project(name, name, issue_count=count)
        shell.api.directory(name, "appboypov/" + name)
    before = deepcopy((shell.api.projects, shell.api.resources, shell.api.issues))
    for run in range(2):
        assert sync.main() == 0
        assert {value["title"] for value in shell.api.projects} == {*sources, "MadSpec"}
        for index, source in enumerate(sources):
            assert next(value for value in shell.api.projects if value["id"] == source) == before[0][index]
            assert shell.api.resources[source] == before[1][source]
            assert shell.api.issues[source] == before[2][source]
        assert shell.api.issues["new-MadSpec"] == shell.api.resources["new-MadSpec"] == []
        marks = sync.Marks(shell.mark_path, shell.config["server_url"], shell.config["workspace_id"])
        assert marks.scope["repository_projects"] == {
            "brain-repository": "madspec", "brain-plannotator": "plannotator", "brain-omp-anytype-inbox": "omp-anytype-inbox"
        }
        output = capsys.readouterr().out
        assert all(f"{source} stays for MadSpec, which has 3 repositories" in output for source in sources)
        if run == 0:
            saved_marks = shell.mark_path.read_bytes()
            saved_writes = deepcopy(shell.api.writes)
        else:
            assert shell.mark_path.read_bytes() == saved_marks
            assert shell.api.writes == saved_writes
    assert not any(path.startswith("/api/issues") or method == "DELETE" for method, path, _ in shell.api.calls)


def test_rebl_merge_moves_six_issues_and_deletes_only_the_emptied_repository_project(shell):
    shell.objects["organisation"] = [{"id": "brain-rebl-org", "name": "Rebl Records", "properties": {"linear_key": "REBL"}}]
    shell.objects["product"] = [{"id": "brain-rebl", "name": "Rebl", "properties": {"organisation": ["brain-rebl-org"]}}]
    shell.objects["repository"] = [{"id": "brain-rebl-repo", "properties": {"github_key": "appboypov/rebl", "product": ["brain-rebl"]}}]
    shell.checkouts = {"appboypov/rebl": [REPOS / "Ventures/rebl"]}
    shell.archived = {"appboypov/rebl": False}
    shell.api.project("repository-rebl", "rebl", issue_count=6)
    shell.api.directory("repository-rebl", "appboypov/rebl")
    shell.api.project("manual", "Keep this", issue_count=4)
    assert sync.main() == 0
    assert {value["title"] for value in shell.api.projects} == {"Rebl", "Keep this"}
    assert len(shell.api.issues["new-Rebl"]) == 6
    assert len(shell.api.resources["new-Rebl"]) == 1
    assert len(shell.api.issues["manual"]) == 4
    check = shell.api.calls.index(("GET", "/api/issues?project_id=repository-rebl&limit=1&offset=0", None))
    delete = shell.api.calls.index(("DELETE", "/api/projects/repository-rebl", None))
    assert check < delete


def test_all_issue_ids_are_read_before_writes_and_moved_in_pages_of_100(shell):
    shell.api.project("repository", "madspec", issue_count=201)
    shell.api.directory("repository", "appboypov/madspec")
    assert sync.main() == 0
    batches = [body for method, path, body in shell.api.writes if path == "/api/issues/batch-update"]
    assert [len(value["issue_ids"]) for value in batches] == [100, 100, 1]
    reads = [index for index, (method, path, _) in enumerate(shell.api.calls) if path.startswith("/api/issues?") and f"limit={sync.PAGE_SIZE}&" in path]
    first_write = next(index for index, (method, _, _) in enumerate(shell.api.calls) if method != "GET")
    assert len(reads) == 3 and max(reads) < first_write
    assert len(shell.api.issues["new-MadSpec"]) == 201


@pytest.mark.parametrize("failure", ["partial", "late-issue", "fresh-read", "batch"])
def test_failed_moves_or_fresh_read_never_delete_the_repository_project(shell, monkeypatch, failure):
    shell.api.project("repository", "madspec", issue_count=3)
    shell.api.directory("repository", "appboypov/madspec")
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
        shell.api.batch_updated = 2
    monkeypatch.setattr(shell.api, "call", after_moves)
    assert sync.main() == 1
    assert "repository" in {value["id"] for value in shell.api.projects}
    assert not any(method == "DELETE" for method, _, _ in shell.api.writes)


@pytest.mark.parametrize("source", ["organisation", "product", "repository", "empty-products", "runtimes", "organisations", "projects", "resources", "issues", "github", "timeout", "marks"])
def test_failed_read_writes_nothing(shell, monkeypatch, source):
    shell.api.project("repository", "madspec", issue_count=3)
    shell.api.directory("repository", "appboypov/madspec")
    before = shell.state()
    if source in ("organisation", "product", "repository"):
        shell.brain_error = source
    elif source == "empty-products":
        shell.objects["product"] = []
    elif source == "github":
        shell.github_errors = [{"type": "FORBIDDEN"}]
    elif source == "timeout":
        def timeout(*args, **kwargs):
            raise sync.subprocess.TimeoutExpired("gh", 60)
        monkeypatch.setattr(sync.subprocess, "run", timeout)
    elif source == "marks":
        shell.mark_path.parent.mkdir()
        shell.mark_path.write_text("not JSON")
    else:
        shell.api.fail_read = {"runtimes": "/api/runtimes", "organisations": "/api/organisations", "projects": "/api/projects",
                               "resources": "/api/projects/repository/resources", "issues": "/api/issues"}[source]
    assert sync.main() == 1
    assert shell.state() == before
    assert shell.api.writes == []


def test_failed_resource_keeps_source_project_and_next_run_heals_without_duplicate(shell):
    shell.api.project("repository", "madspec", issue_count=2)
    shell.api.directory("repository", "appboypov/madspec")
    shell.api.fail_resource = True
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


def test_product_without_repositories_does_not_query_github(shell, monkeypatch):
    shell.objects["repository"] = []
    def unexpected_query(*args, **kwargs):
        pytest.fail("GitHub queried without repositories")
    monkeypatch.setattr(sync.subprocess, "run", unexpected_query)
    assert sync.main() == 0
    assert shell.api.resources["new-MadSpec"] == []


def test_archive_checkouts_are_excluded_and_mixed_case_keys_keep_lowercase_marks(shell):
    shell.checkouts["appboypov/madspec"].append(REPOS / "Archive/madspec")
    shell.objects["repository"][0]["properties"]["github_key"] = "AppBoyPov/MadSpec"
    assert sync.main() == 0
    resource = shell.api.resources["new-MadSpec"][0]
    assert resource["resource_ref"]["local_path"] == str(REPOS / "Ventures/madspec")
    assert resource["resource_ref"]["label"] == "madspec:appboypov/madspec"
    assert len(shell.api.resources["new-MadSpec"]) == 1


def test_distinct_repository_objects_with_same_github_key_count_separately(shell, capsys):
    shell.objects["repository"].append({"id": "brain-alias", "properties": {"github_key": "AppBoyPov/MadSpec", "product": [MADSPEC.id]}})
    shell.api.project("repository", "madspec", issue_count=2)
    shell.api.directory("repository", "appboypov/madspec")
    assert sync.main() == 0
    assert shell.api.resources["new-MadSpec"] == shell.api.issues["new-MadSpec"] == []
    assert len(shell.api.issues["repository"]) == 2
    assert "madspec stays for MadSpec, which has 2 repositories" in capsys.readouterr().out
    marks = sync.Marks(shell.mark_path, shell.config["server_url"], shell.config["workspace_id"])
    assert marks.scope["repository_projects"] == {"brain-repository": "repository", "brain-alias": "repository"}


def test_failed_repository_identity_save_keeps_projects_and_next_run_recovers(shell, monkeypatch):
    shell.objects["repository"].append({"id": "brain-plannotator", "properties": {"github_key": "appboypov/plannotator", "product": [MADSPEC.id]}})
    shell.checkouts["appboypov/plannotator"] = [REPOS / "Ventures/plannotator"]
    shell.archived["appboypov/plannotator"] = False
    shell.api.project("repository", "madspec", issue_count=10)
    shell.api.directory("repository", "appboypov/madspec")
    original = sync.Marks.record
    def refused_mark(self, kind, brain_id, multica_id):
        if kind == "repository_projects":
            raise OSError("mark refused")
        return original(self, kind, brain_id, multica_id)
    monkeypatch.setattr(sync.Marks, "record", refused_mark)
    assert sync.main() == 1
    assert len(shell.api.issues["repository"]) == 10
    assert shell.api.resources["new-MadSpec"] == []
    monkeypatch.setattr(sync.Marks, "record", original)
    assert sync.main() == 0
    marks = sync.Marks(shell.mark_path, shell.config["server_url"], shell.config["workspace_id"])
    assert marks.scope["repository_projects"] == {"brain-repository": "repository"}
    assert len(shell.api.issues["repository"]) == 10
    assert not any(method == "DELETE" or path == "/api/issues/batch-update" for method, path, _ in shell.api.writes)


@pytest.mark.parametrize("entries", [[], {"brain-repository": 42}])
def test_invalid_repository_project_marks_fail_before_server_writes(shell, entries):
    shell.mark_path.parent.mkdir()
    shell.mark_path.write_text(json.dumps({"http://test-server": {"workspace": {"repository_projects": entries}}}))
    assert sync.main() == 1
    assert shell.api.calls == shell.api.writes == []


def test_not_found_github_alias_reports_and_still_merges(shell, capsys):
    shell.archived["appboypov/madspec"] = None
    shell.github_errors = [{"type": "NOT_FOUND", "path": ["r0"]}]
    shell.api.project("repository", "madspec", issue_count=1)
    shell.api.directory("repository", "appboypov/madspec")
    assert sync.main() == 0
    assert len(shell.api.issues["new-MadSpec"]) == 1
    assert shell.api.resources["new-MadSpec"] == []
    assert "unknown to GitHub" in capsys.readouterr().out


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
    assert len(shell.api.resources["new-MadSpec"]) == 1


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


def test_conflicting_checkout_marks_cannot_delete_a_project_or_choose_one_parent():
    workspace = Workspace().placed()
    workspace.project("mixed", "Mixed", issue_count=5)
    workspace.directory("mixed", "appboypov/madspec")
    workspace.directory("mixed", "appboypov/hq")
    desired = workspace.plan(brain((MADSPEC, HQ), (repository("madspec"), repository("hq", HQ.id))))
    assert desired.merges == ()
    assert any("do not identify one Product" in report for report in desired.reports)


def test_existing_product_project_cannot_be_deleted_by_a_changed_repository_link():
    workspace = Workspace().placed()
    workspace.directory("project-madspec", "appboypov/madspec")
    desired = workspace.plan(brain((MADSPEC, HQ), (repository("madspec", HQ.id),)))
    assert desired.merges == ()
    assert any("marked Product project" in report for report in desired.reports)


def test_failed_organisation_create_skips_dependent_projects_and_merges(shell, monkeypatch):
    shell.api.project("repository", "madspec", issue_count=1)
    shell.api.directory("repository", "appboypov/madspec")
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
    shell.api.project("repository", "madspec", issue_count=1)
    shell.api.directory("repository", "appboypov/madspec")
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


def test_partial_move_next_run_finishes_from_remaining_source_issues(shell):
    shell.api.project("repository", "madspec", issue_count=3)
    shell.api.directory("repository", "appboypov/madspec")
    shell.api.batch_updated = 2
    assert sync.main() == 1
    assert len(shell.api.issues["repository"]) == 1
    shell.api.batch_updated = None
    assert sync.main() == 0
    assert len(shell.api.issues["new-MadSpec"]) == 3
    assert "repository" not in shell.api.issues
