import subprocess
import sys
import tomllib
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import multica_sync  # noqa: E402

ROSTER_PERSONAS = multica_sync.personas

CHECKOUT = Path("/main-checkout")
TWO_MODELS = '["anthropic/claude-opus-5-5:high", "openai-codex/gpt-6.1-sol:high"]'


def write(path: Path, text: str) -> Path:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")
    return path


def persona_skill(root: Path, skill: str, name: str, models: str | None = TWO_MODELS, body: str = "Body.\n") -> None:
    model = f"model = {models}\n" if models is not None else ""
    write(root / "skills" / skill / "customize.toml", f'[agent]\nname = "{name}"\n{model}\n[[agent.menu]]\ncode = "X"\ndescription = "x"\nskill = "demo-skill"\n')
    write(root / "skills" / skill / "SKILL.md", f'---\nname: {skill}\ndescription: "{name}, a persona."\n---\n# {name}\n\n{body}')
    office = root / "offices" / name.lower()
    (office / ".git").mkdir(parents=True, exist_ok=True)
    (office / "skills").mkdir(exist_ok=True)
    linked = office / "skills" / skill
    if not linked.exists():
        linked.symlink_to(root / "skills" / skill, target_is_directory=True)


@pytest.fixture
def repo(tmp_path: Path) -> Path:
    write(tmp_path / "skills" / "demo-skill" / "SKILL.md", "---\nname: demo-skill\n---\n")
    persona_skill(tmp_path, "as-product-manager", "Mitchel")
    persona_skill(tmp_path, "as-chief-of-staff", "Skuddy", models='["x/y"]')
    write(tmp_path / "skills" / "madspec" / "roster.toml", "")
    return tmp_path


@pytest.fixture(autouse=True)
def fallback_dir(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    folder = tmp_path / "fallback"
    monkeypatch.setattr(multica_sync, "FALLBACK_DIR", folder)
    return folder


@pytest.fixture(autouse=True)
def roster_api(monkeypatch):
    """The two-root roster API, isolated from the host's MadSpec installation."""
    def report(offices_root, skills_root):
        assert offices_root.name == "offices"
        assert skills_root.name == "skills"
        found = []
        for office in sorted(offices_root.iterdir()):
            for path in sorted((office / "skills").glob("*/customize.toml")):
                agent = tomllib.loads(path.read_text())["agent"]
                found.append({"command": office.name, "skill": path.parent.name, "name": agent["name"],
                              "models": agent.get("model", []),
                              "skills": None if office.name == "skuddy" else [path.parent.name, "demo-skill"]})
        return found
    monkeypatch.setattr(multica_sync, "personas", report)


def test_a_persona_runs_as_its_skill_with_its_skills_and_fallbacks(repo, fallback_dir):
    personas, problems = multica_sync.read_personas(repo, CHECKOUT)
    mitchel = personas["mitchel"]

    assert problems == {}
    assert mitchel.fields() == {
        "name": "Mitchel",
        "description": "Runs the as-product-manager skill.",
        "instructions": f"`{{skill-root}}` is {CHECKOUT / 'skills' / 'as-product-manager'}. Your instructions are "
        "the as-product-manager skill below; follow it.\n\n{selection}",
        "instruction_selection": [str(CHECKOUT / "skills" / "as-product-manager")],
        "model": "anthropic/claude-opus-5-5",
        "thinking_level": "high",
        "custom_args": ["--skills=as-product-manager,demo-skill", f"--config={fallback_dir / 'mitchel.yml'}"],
    }
    assert multica_sync.fallback_chain(mitchel.models) == {"anthropic/claude-opus-5-5": ["openai-codex/gpt-6.1-sol:high"]}


def test_guus_selects_his_whole_skill_folder(repo):
    persona_skill(repo, "as-ux-designer", "Guus")
    checkout = Path("/Users/codaveto/Repos/Ventures/madspec")

    personas, problems = multica_sync.read_personas(repo, checkout)
    guus = personas["guus"]

    assert problems == {}
    assert guus.instruction_selection == ["/Users/codaveto/Repos/Ventures/madspec/skills/as-ux-designer"]
    assert guus.instructions == (
        "`{skill-root}` is /Users/codaveto/Repos/Ventures/madspec/skills/as-ux-designer. "
        "Your instructions are the as-ux-designer skill below; follow it.\n\n{selection}"
    )


def test_a_persona_with_every_skill_and_one_model_gets_no_arguments(repo):
    skuddy = multica_sync.read_personas(repo, CHECKOUT)[0]["skuddy"]

    assert (skuddy.model, skuddy.thinking_level, skuddy.custom_args) == ("x/y", "", [])


def test_a_persona_without_a_model_is_a_problem_and_no_agent(repo):
    persona_skill(repo, "as-verifier", "Sarkout", models=None)

    personas, problems = multica_sync.read_personas(repo, CHECKOUT)

    assert "sarkout" not in personas
    assert problems == {"sarkout": "skills/as-verifier/customize.toml has no [agent] model"}


@pytest.mark.parametrize(
    ("model", "expected"),
    [
        ("anthropic/claude-opus-5-5:high", ("anthropic/claude-opus-5-5", "high")),
        ("ollama/qwen:7b", ("ollama/qwen:7b", "")),
        ("x/y", ("x/y", "")),
    ],
)
def test_thinking_level_is_only_a_known_suffix(model, expected):
    assert multica_sync.split_model(model) == expected


@pytest.fixture
def mitchel(repo) -> multica_sync.Persona:
    return multica_sync.read_personas(repo, CHECKOUT)[0]["mitchel"]


@pytest.mark.parametrize(
    ("agents", "markers", "expected", "take_over"),
    [
        ([{"id": "a", "name": "Other"}], {"a": "mitchel"}, "a", False),
        ([{"id": "a", "name": "Mitchel", "archived_at": "t"}, {"id": "b", "name": "Mitchel", "archived_at": None}], {}, None, False),
        ([{"id": "a", "name": "Mitchel", "archived_at": "t"}], {}, None, False),
        ([{"id": "a", "name": "Mitchel"}], {"a": "pieter"}, None, False),
    ],
)
def test_agent_matching_respects_identity_and_active_state(mitchel, agents, markers, expected, take_over):
    agent, takeover = multica_sync.match_agent(mitchel, agents, markers)

    assert (agent["id"] if agent else None) == expected
    assert takeover is take_over


def test_agent_changes_restore_source_fields_without_rewriting_equal_fields(mitchel):
    agent = {**mitchel.fields(), "model": "", "custom_args": None}

    assert multica_sync.agent_changes(agent, mitchel) == {"model": mitchel.model, "custom_args": mitchel.custom_args}
    assert multica_sync.agent_changes(mitchel.fields(), mitchel) == {}


@pytest.mark.parametrize(
    "runtimes",
    [
        [],
        [{"id": "a", "provider": "omp", "runtime_mode": "local"}, {"id": "b", "provider": "omp", "runtime_mode": "local"}],
    ],
)
def test_runtime_selection_rejects_missing_or_ambiguous_local_omp(runtimes):
    with pytest.raises(ValueError):
        multica_sync.omp_runtime(runtimes)


def test_runtime_selection_ignores_other_providers_and_remote_omp():
    runtimes = [
        {"id": "other", "provider": "codex", "runtime_mode": "local"},
        {"id": "remote", "provider": "omp", "runtime_mode": "remote"},
        {"id": "local", "provider": "omp", "runtime_mode": "local"},
    ]

    assert multica_sync.omp_runtime(runtimes) == "local"


def git(cwd: Path, *args: str) -> None:
    subprocess.run(["git", "-c", "user.name=Test", "-c", "user.email=test@example.com", *args], cwd=cwd, check=True, capture_output=True)


@pytest.fixture
def clone(tmp_path: Path) -> Path:
    """A repository whose `origin/main` holds merged text."""
    root = tmp_path / "clone"
    write(root / "skills" / "demo-skill" / "SKILL.md", "Merged.\n")
    git(root, "init", "-q", "-b", "main")
    git(root, "add", "-A")
    git(root, "commit", "-qm", "main")
    git(root, "update-ref", "refs/remotes/origin/main", "HEAD")
    return root


def test_export_holds_origin_main_not_a_branch_or_working_tree(clone, tmp_path):
    git(clone, "switch", "-qc", "feature")
    write(clone / "skills" / "demo-skill" / "SKILL.md", "Unmerged.\n")
    git(clone, "commit", "-qam", "feature")
    write(clone / "skills" / "demo-skill" / "SKILL.md", "Uncommitted.\n")
    tree = tmp_path / "tree"
    tree.mkdir()

    multica_sync.export_main(clone, tree)

    assert (tree / "skills" / "demo-skill" / "SKILL.md").read_text(encoding="utf-8") == "Merged.\n"


def test_export_without_origin_main_fails(clone, tmp_path):
    git(clone, "update-ref", "-d", "refs/remotes/origin/main")

    with pytest.raises(ValueError, match="cannot read refs/remotes/origin/main"):
        multica_sync.export_main(clone, tmp_path)


class FakeMultica:
    """A Multica with one runtime and the given agents, recording each write and which
    `--config=` files were missing when each agent was created."""

    def __init__(self, agents: list[dict], envs: dict[str, dict[str, str]] | None = None) -> None:
        self.agents = agents
        self.envs = envs or {}
        self.writes: list[tuple[str, str, object]] = []
        self.missing_at_create: dict[str, list[str]] = {}

    def call(self, method: str, path: str, body: dict | None = None) -> object:
        if method == "GET" and path == "/api/runtimes":
            return [{"id": "omp", "provider": "omp", "runtime_mode": "local"}]
        if method == "GET" and path.startswith("/api/agents?"):
            return self.agents
        if method == "GET" and path.endswith("/env"):
            return {"custom_env": self.envs[path.split("/")[3]]}
        self.writes.append((method, path, body))
        if method == "POST" and path == "/api/agents":
            configs = [arg.removeprefix("--config=") for arg in body["custom_args"] if arg.startswith("--config=")]
            self.missing_at_create[body["name"]] = [config for config in configs if not Path(config).is_file()]
            return {"id": body["name"]}
        return {}


def test_sync_adopts_one_explicit_archived_persona_agent(repo, mitchel):
    api = FakeMultica([{"id": "m", "name": "Mitchel", "archived_at": "t", "has_custom_env": False, "runtime_id": "", **multica_sync.OPEN_TO_WORKSPACE}])

    assert multica_sync.Sync(api, repo, CHECKOUT, adopt=["Mitchel"]).run("Mitchel") == 0

    assert api.writes[0] == ("POST", "/api/agents/m/restore", None)
    assert api.writes[1] == ("PUT", "/api/agents/m/env", {"custom_env": {"MADSPEC_PERSONA": "mitchel"}})
    fields = {key: value for key, value in mitchel.fields().items() if key != "name"}
    assert api.writes[2] == ("PUT", "/api/agents/m", {**fields, "runtime_id": "omp"})
    assert len(api.writes) == 3


@pytest.mark.parametrize("selection", [None, [], ["/other/skill"]])
def test_sync_rewrites_a_stale_selection_and_leaves_equal_fields(repo, selection, capsys):
    persona_skill(repo, "as-ux-designer", "Guus", models='["x/y"]')
    guus = multica_sync.read_personas(repo, CHECKOUT)[0]["guus"]
    agent = {
        **guus.fields(),
        **multica_sync.OPEN_TO_WORKSPACE,
        "id": "g",
        "runtime_id": "omp",
        "has_custom_env": True,
        "instruction_selection": selection,
    }
    api = FakeMultica([agent], {"g": {"MADSPEC_PERSONA": "guus"}})

    assert multica_sync.Sync(api, repo, CHECKOUT).run("Guus") == 0

    assert api.writes == [("PUT", "/api/agents/g", {"instruction_selection": guus.instruction_selection})]
    assert capsys.readouterr().out == "update agent Guus (instruction_selection)\n"

    agent["instruction_selection"] = list(guus.instruction_selection)
    api.writes.clear()

    assert multica_sync.Sync(api, repo, CHECKOUT).run("guus") == 0
    assert api.writes == []
    assert capsys.readouterr().out == ""


def test_full_sync_archives_only_its_agents_whose_persona_is_gone(repo):
    api = FakeMultica(
        [
            {"id": "gone", "name": "Fallon", "archived_at": None, "has_custom_env": True},
            {"id": "foreign", "name": "Explorer", "archived_at": None, "has_custom_env": True},
        ],
        envs={"gone": {"MADSPEC_PERSONA": "fallon"}, "foreign": {"OTHER": "x"}},
    )

    assert multica_sync.Sync(api, repo, CHECKOUT).run(None) == 0

    archived = [path for method, path, _ in api.writes if path.endswith("/archive")]
    assert archived == ["/api/agents/gone/archive"]


def test_sync_writes_overlays_before_agents_and_deletes_those_no_agent_names(repo, fallback_dir, capsys):
    removed = write(fallback_dir / "removed.yml", "retry: {}\n")
    still_named = write(fallback_dir / "unreadable.yml", "retry: {}\n")
    api = FakeMultica([
        {"id": "removed", "name": "Removed", "archived_at": "t", "custom_args": [f"--config={removed}"]},
        {"id": "unreadable", "name": "Unreadable", "archived_at": None, "custom_args": [f"--config={still_named}"]},
    ])

    assert multica_sync.Sync(api, repo, CHECKOUT).run(None) == 0

    assert api.missing_at_create == {"Mitchel": [], "Skuddy": []}
    overlay = (fallback_dir / "mitchel.yml").read_text(encoding="utf-8").splitlines()
    assert overlay[1:] == [
        "retry:",
        "  modelFallback: true",
        "  fallbackChains:",
        '    "anthropic/claude-opus-5-5": ["openai-codex/gpt-6.1-sol:high"]',
    ]
    assert sorted(path.name for path in fallback_dir.iterdir()) == ["mitchel.yml", "unreadable.yml"]

    capsys.readouterr()
    assert multica_sync.Sync(api, repo, CHECKOUT).run(None) == 0
    assert "write overlay" not in capsys.readouterr().out


def worker_file(root, worker="backend-junior", body="# Backend Junior\n\nDo the assigned task.\n"):
    write(root / "skills" / "madspec" / "roster.toml",
          '[[teams]]\nid = "backend"\nlead = "as-product-manager"\nworkers = ["' + worker + '"]\n')
    return write(root / "offices" / "mitchel" / "agents" / f"{worker}.md",
                 '---\nname: Backend Junior\ndescription: "Junior worker."\nskills: madspec-function-builder, madspec-development\n'
                 'model: openai-codex/gpt-6-luna:high, anthropic/claude-haiku-5-5:high\n---\n\n' + body)


def test_worker_uses_file_body_models_skills_and_marker(repo, fallback_dir):
    worker_file(repo)
    write(repo / "offices" / "mitchel" / "agents" / "madspec-polisher.md", "Not a roster worker.")
    people, _ = multica_sync.read_personas(repo, CHECKOUT)
    workers, problems = multica_sync.read_workers(repo, people)
    worker = workers["backend-junior"]

    assert problems == {}
    assert set(workers) == {"backend-junior"}
    assert worker.instructions == "# Backend Junior\n\nDo the assigned task.\n"
    assert worker.instruction_selection == []
    assert (worker.name, worker.model, worker.thinking_level, worker.marker) == (
        "Backend Junior", "openai-codex/gpt-6-luna", "high", "MADSPEC_WORKER")
    assert worker.custom_args == ["--skills=madspec-function-builder,madspec-development",
                                  f"--config={fallback_dir / 'backend-junior.yml'}"]
    api = FakeMultica([])
    assert multica_sync.Sync(api, repo, CHECKOUT).run("Backend Junior") == 0
    assert api.writes[0][2]["runtime_id"] == "omp"
    assert api.writes[0][2]["instructions"] == worker.instructions
    assert api.writes[1] == ("PUT", "/api/agents/Backend Junior/env",
                             {"custom_env": {"MADSPEC_WORKER": "backend-junior"}})
    assert api.missing_at_create == {"Backend Junior": []}


def test_unmanaged_agent_is_unchanged_without_adoption(repo, fallback_dir, capsys):
    api = FakeMultica([{"id": "m", "name": "Mitchel", "has_custom_env": False}])
    assert multica_sync.Sync(api, repo, CHECKOUT).run("Mitchel") == 1
    assert api.writes == []
    assert not fallback_dir.exists()
    assert "unmanaged name conflict" in capsys.readouterr().err


def test_dry_run_reports_create_update_archive_adopt_and_overlays_without_writes(repo, mitchel, fallback_dir, capsys):
    worker_file(repo)
    removed = write(fallback_dir / "removed.yml", "keep this content")
    api = FakeMultica([
        {"id": "m", "name": "Mitchel", "has_custom_env": True, "runtime_id": "other"},
        {"id": "gone", "name": "Gone", "has_custom_env": True},
    ], {"m": {"SECRET": "hidden"}, "gone": {"MADSPEC_PERSONA": "gone"}})
    assert multica_sync.Sync(api, repo, CHECKOUT, dry_run=True, adopt=["Mitchel"]).run(None) == 0
    assert api.writes == []
    assert list(fallback_dir.iterdir()) == [removed]
    assert removed.read_text() == "keep this content"
    output = capsys.readouterr().out
    for text in ("would create agent Backend Junior", "would update agent Mitchel", "would adopt agent Mitchel",
                 "would archive agent Gone", "would write overlay", "would delete overlay"):
        assert text in output
    assert "hidden" not in output


def test_adoption_preserves_other_environment_and_requires_unique_name(repo):
    agents = [{"id": "m", "name": "Mitchel", "has_custom_env": True}]
    api = FakeMultica(agents, {"m": {"SECRET": "hidden"}})
    assert multica_sync.Sync(api, repo, CHECKOUT, adopt=["Mitchel"]).run("Mitchel") == 0
    assert ("PUT", "/api/agents/m/env", {"custom_env": {"SECRET": "****", "MADSPEC_PERSONA": "mitchel"}}) in api.writes
    api = FakeMultica(agents + [{"id": "duplicate", "name": "Mitchel"}], {"m": {"SECRET": "hidden"}})
    assert multica_sync.Sync(api, repo, CHECKOUT, adopt=["Mitchel"]).run("Mitchel") == 1
    assert api.writes == []


@pytest.mark.parametrize(("name", "skill"), [
    ("Luuk", "as-data-engineer"), ("Tortolero", "as-devops-engineer"),
    ("Paul", "as-ai-engineer"), ("Lukas", "as-handyman"),
])
def test_office_personas_are_discovered(repo, name, skill):
    persona_skill(repo, skill, name)
    people, problems = multica_sync.read_personas(repo, CHECKOUT)
    assert problems == {}
    assert people[name.lower()].name == name


def test_missing_worker_is_reported_without_archiving_its_agent(repo):
    worker_file(repo).unlink()
    api = FakeMultica([{"id": "worker", "name": "Backend Junior", "has_custom_env": True}],
                     {"worker": {"MADSPEC_WORKER": "backend-junior"}})
    assert multica_sync.Sync(api, repo, CHECKOUT).run(None) == 1
    assert not any(path.endswith("/archive") for _, path, _ in api.writes)


def test_worker_without_models_is_reported(repo):
    path = worker_file(repo)
    path.write_text('---\nname: Backend Junior\n---\n\nInstructions.\n')
    people, _ = multica_sync.read_personas(repo, CHECKOUT)
    workers, problems = multica_sync.read_workers(repo, people)
    assert workers == {}
    assert "worker needs a name, models and instructions" in problems["backend-junior"]


def test_export_offices_reads_main_and_keeps_worker_body(tmp_path, monkeypatch):
    offices = tmp_path / "offices-source"
    office = offices / "luuk"
    write(office / "skills" / "as-data-engineer" / "SKILL.md", "Persona.\n")
    worker = write(office / "agents" / "data-junior.md", "Merged worker.\n")
    git(office, "init", "-q", "-b", "main")
    git(office, "add", "-A")
    git(office, "commit", "-qm", "main")
    git(office, "update-ref", multica_sync.MAIN_REF, "HEAD")
    worker.write_text("Uncommitted worker.\n")
    destination = tmp_path / "snapshot" / "offices"
    monkeypatch.setattr(multica_sync, "OFFICES", offices)
    multica_sync.export_offices(offices, destination)
    assert (destination / "luuk" / ".git").is_dir()
    assert (destination / "luuk" / "agents" / "data-junior.md").read_text() == "Merged worker.\n"


def test_roster_personas_receives_both_roots(tmp_path, monkeypatch):
    from types import SimpleNamespace
    calls = []
    monkeypatch.setitem(sys.modules, "roster", SimpleNamespace(
        personas=lambda offices, skills: calls.append((offices, skills)) or []))
    assert ROSTER_PERSONAS(tmp_path / "offices", tmp_path / "skills") == []
    assert calls == [(tmp_path / "offices", tmp_path / "skills")]


def test_snapshot_skill_links_use_committed_office_files(tmp_path, monkeypatch):
    package, offices = tmp_path / "madspec", tmp_path / "source-offices"
    office = offices / "luuk"
    source = write(office / "skills" / "as-data-engineer" / "SKILL.md", "Merged.\n")
    for root in (office, package):
        root.mkdir(parents=True, exist_ok=True)
        git(root, "init", "-q", "-b", "main")
    link = package / "skills" / "as-data-engineer"
    link.parent.mkdir()
    link.symlink_to(source.parent)
    for root in (office, package):
        git(root, "add", "-A")
        git(root, "commit", "-qm", "main")
        git(root, "update-ref", multica_sync.MAIN_REF, "HEAD")
    source.write_text("Uncommitted.\n")
    monkeypatch.setattr(multica_sync, "OFFICES", offices)
    monkeypatch.setattr(multica_sync, "MADSPEC", package)
    snapshot = tmp_path / "snapshot"
    multica_sync.export_main(package, snapshot)
    multica_sync.export_offices(offices, snapshot / "offices")
    assert (snapshot / "skills" / "as-data-engineer" / "SKILL.md").read_text() == "Merged.\n"
    assert (snapshot / "skills" / "as-data-engineer").resolve().is_relative_to(snapshot)


def test_all_34_roster_workers_are_loaded_but_helpers_are_not(repo):
    levels = ("junior", "medior", "senior")
    ids = [f"{team}-{level}" for team in ("backend", "infra", "frontend", "design", "tools", "packages", "data", "product") for level in levels]
    ids += [f"{round}-reviewer-{family}" for round in ("first", "second") for family in ("gpt", "claude")]
    ids += [f"tester-{family}-{level}" for family in ("gpt", "claude") for level in levels]
    for worker in ids:
        worker_file(repo, worker)
    write(repo / "skills" / "madspec" / "roster.toml",
          '[[teams]]\nid = "workers"\nlead = "as-product-manager"\nworkers = ' + repr(ids).replace("'", '"') + "\n")
    write(repo / "offices" / "mitchel" / "agents" / "madspec-extractor.md", "Session helper.")
    people, _ = multica_sync.read_personas(repo, CHECKOUT)
    workers, problems = multica_sync.read_workers(repo, people)
    assert problems == {}
    assert len(workers) == 34
    assert set(workers) == set(ids)


def test_worker_yaml_model_list_preserves_fallback_order(repo):
    path = worker_file(repo)
    path.write_text('---\nname: Data Junior\nskills: [madspec-development]\nmodel:\n'
                    '  - google-antigravity/gemini-3.8-flash:high\n'
                    '  - github-copilot/gemini-3.8-flash:high\n'
                    '  - anthropic/claude-haiku-5-5:high\n---\n\nDo the task.\n')
    people, _ = multica_sync.read_personas(repo, CHECKOUT)
    workers, problems = multica_sync.read_workers(repo, people)
    assert problems == {}
    assert multica_sync.fallback_chain(workers["backend-junior"].models) == {
        "google-antigravity/gemini-3.8-flash": ["github-copilot/gemini-3.8-flash:high", "anthropic/claude-haiku-5-5:high"]}
