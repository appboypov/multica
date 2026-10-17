import subprocess
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import multica_sync  # noqa: E402

CHECKOUT = Path("/agents")
TWO_MODELS = '["anthropic/claude-opus-5-5:high", "openai-codex/gpt-6.1-sol:high"]'


def write(path: Path, text: str) -> Path:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")
    return path


def definition(root: Path, command: str, name: str, models: str | None = TWO_MODELS, title: str = "Product Manager") -> Path:
    """An agent folder with its `agent.toml` and `AGENTS.md`."""
    model = f"models = {models}\n" if models is not None else ""
    write(root / command / "AGENTS.md", f"# {name}\n\nBody.\n")
    return write(root / command / "agent.toml", f'name = "{name}"\ntitle = "{title}"\n{model}')


@pytest.fixture
def repo(tmp_path: Path) -> Path:
    tree = tmp_path / "tree"
    definition(tree, "mitchel", "Mitchel")
    definition(tree, "skuddy", "Skuddy", models='["x/y"]', title="Chief of Staff")
    return tree


@pytest.fixture(autouse=True)
def fallback_dir(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    folder = tmp_path / "fallback"
    monkeypatch.setattr(multica_sync, "FALLBACK_DIR", folder)
    return folder


def test_an_agent_runs_its_agents_md_with_its_fallbacks(repo, fallback_dir):
    personas, problems = multica_sync.read_personas(repo, CHECKOUT)
    mitchel = personas["mitchel"]

    assert problems == {}
    assert mitchel.fields() == {
        "name": "Mitchel",
        "description": "Product Manager",
        "instructions": "Your instructions are /agents/mitchel/AGENTS.md below; follow them.\n\n{selection}",
        "instruction_selection": ["/agents/mitchel/AGENTS.md"],
        "model": "anthropic/claude-opus-5-5",
        "thinking_level": "high",
        "custom_args": [f"--config={fallback_dir / 'mitchel.yml'}"],
    }
    assert mitchel.marker == "MADSPEC_PERSONA"
    assert multica_sync.fallback_chain(mitchel.models) == {"anthropic/claude-opus-5-5": ["openai-codex/gpt-6.1-sol:high"]}


def test_an_agent_with_one_model_gets_no_arguments(repo):
    skuddy = multica_sync.read_personas(repo, CHECKOUT)[0]["skuddy"]

    assert (skuddy.model, skuddy.thinking_level, skuddy.custom_args) == ("x/y", "", [])


def test_a_worker_is_a_folder_named_after_its_level(repo):
    definition(repo, "medior-backend", "Medior Backend", models='["openai-codex/gpt-6.1-sol:high"]', title="Medior Backend")

    worker = multica_sync.read_personas(repo, CHECKOUT)[0]["medior-backend"]

    assert (worker.name, worker.model, worker.thinking_level, worker.custom_args, worker.marker) == (
        "Medior Backend", "openai-codex/gpt-6.1-sol", "high", [], "MADSPEC_WORKER")


def test_a_folder_without_agent_toml_is_no_agent(repo):
    write(repo / "tika" / "AGENTS.md", "# Tika\n")

    personas, problems = multica_sync.read_personas(repo, CHECKOUT)

    assert set(personas) == {"mitchel", "skuddy"}
    assert problems == {}


@pytest.mark.parametrize(
    ("models", "problem"),
    [(None, "sarkout/agent.toml has no models"), ("[]", "sarkout/agent.toml has no models")],
)
def test_an_agent_without_models_is_a_problem_and_no_agent(repo, models, problem):
    definition(repo, "sarkout", "Sarkout", models=models)

    personas, problems = multica_sync.read_personas(repo, CHECKOUT)

    assert "sarkout" not in personas
    assert problems == {"sarkout": problem}


def test_an_agent_without_agents_md_is_a_problem(repo):
    (repo / "mitchel" / "AGENTS.md").unlink()

    assert multica_sync.read_personas(repo, CHECKOUT)[1] == {"mitchel": "mitchel/AGENTS.md does not exist"}


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
    """An agents repo whose `origin/main` holds merged text."""
    root = tmp_path / "clone"
    definition(root, "mitchel", "Mitchel")
    write(root / "mitchel" / "skills" / "notes.md", "Not a definition.\n")
    write(root / "tools" / "nested" / "agent.toml", 'name = "Nested"\n')
    git(root, "init", "-q", "-b", "main")
    git(root, "add", "-A")
    git(root, "commit", "-qm", "main")
    git(root, "update-ref", "refs/remotes/origin/main", "HEAD")
    return root


def test_export_holds_only_the_definitions_of_origin_main(clone, tmp_path):
    git(clone, "switch", "-qc", "feature")
    write(clone / "mitchel" / "AGENTS.md", "Unmerged.\n")
    git(clone, "commit", "-qam", "feature")
    write(clone / "mitchel" / "AGENTS.md", "Uncommitted.\n")
    tree = tmp_path / "tree"
    tree.mkdir()

    multica_sync.export_main(clone, tree)

    assert (tree / "mitchel" / "AGENTS.md").read_text(encoding="utf-8") == "# Mitchel\n\nBody.\n"
    assert sorted(str(path.relative_to(tree)) for path in tree.rglob("*") if path.is_file()) == ["mitchel/AGENTS.md", "mitchel/agent.toml"]


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


def test_sync_adopts_one_explicit_archived_agent(repo, mitchel):
    api = FakeMultica([{"id": "m", "name": "Mitchel", "archived_at": "t", "has_custom_env": False, "runtime_id": "", **multica_sync.OPEN_TO_WORKSPACE}])

    assert multica_sync.Sync(api, repo, CHECKOUT, adopt=["Mitchel"]).run("Mitchel") == 0

    assert api.writes[0] == ("POST", "/api/agents/m/restore", None)
    assert api.writes[1] == ("PUT", "/api/agents/m/env", {"custom_env": {"MADSPEC_PERSONA": "mitchel"}})
    fields = {key: value for key, value in mitchel.fields().items() if key != "name"}
    assert api.writes[2] == ("PUT", "/api/agents/m", {**fields, "runtime_id": "omp"})
    assert len(api.writes) == 3


@pytest.mark.parametrize("selection", [None, [], ["/other/skill"]])
def test_sync_rewrites_a_stale_selection_and_leaves_equal_fields(repo, selection, capsys):
    skuddy = multica_sync.read_personas(repo, CHECKOUT)[0]["skuddy"]
    agent = {
        **skuddy.fields(),
        **multica_sync.OPEN_TO_WORKSPACE,
        "id": "s",
        "runtime_id": "omp",
        "has_custom_env": True,
        "instruction_selection": selection,
    }
    api = FakeMultica([agent], {"s": {"MADSPEC_PERSONA": "skuddy"}})

    assert multica_sync.Sync(api, repo, CHECKOUT).run("Skuddy") == 0

    assert api.writes == [("PUT", "/api/agents/s", {"instruction_selection": skuddy.instruction_selection})]
    assert capsys.readouterr().out == "update agent Skuddy (instruction_selection)\n"

    agent["instruction_selection"] = list(skuddy.instruction_selection)
    api.writes.clear()

    assert multica_sync.Sync(api, repo, CHECKOUT).run("skuddy") == 0
    assert api.writes == []
    assert capsys.readouterr().out == ""


def test_full_sync_archives_only_its_agents_whose_folder_is_gone(repo):
    api = FakeMultica(
        [
            {"id": "gone", "name": "Fallon", "archived_at": None, "has_custom_env": True},
            {"id": "worker", "name": "First Reviewer GPT", "archived_at": None, "has_custom_env": True},
            {"id": "foreign", "name": "Explorer", "archived_at": None, "has_custom_env": True},
        ],
        envs={"gone": {"MADSPEC_PERSONA": "fallon"}, "worker": {"MADSPEC_WORKER": "first-reviewer-gpt"}, "foreign": {"OTHER": "x"}},
    )

    assert multica_sync.Sync(api, repo, CHECKOUT).run(None) == 0

    archived = [path for method, path, _ in api.writes if path.endswith("/archive")]
    assert archived == ["/api/agents/gone/archive", "/api/agents/worker/archive"]


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
    assert overlay == [
        "# Written by multica_sync.py from mitchel/agent.toml on origin/main.",
        "retry:",
        "  modelFallback: true",
        "  fallbackChains:",
        '    "anthropic/claude-opus-5-5": ["openai-codex/gpt-6.1-sol:high"]',
    ]
    assert sorted(path.name for path in fallback_dir.iterdir()) == ["mitchel.yml", "unreadable.yml"]

    capsys.readouterr()
    assert multica_sync.Sync(api, repo, CHECKOUT).run(None) == 0
    assert "write overlay" not in capsys.readouterr().out


def test_a_new_worker_gets_its_marker(repo):
    definition(repo, "junior-creative", "Junior Creative", models='["anthropic/claude-haiku-5-5:high"]', title="Junior Creative")
    api = FakeMultica([])

    assert multica_sync.Sync(api, repo, CHECKOUT).run("Junior Creative") == 0

    assert api.writes[0][2]["runtime_id"] == "omp"
    assert api.writes[0][2]["custom_args"] == []
    assert api.writes[1] == ("PUT", "/api/agents/Junior Creative/env", {"custom_env": {"MADSPEC_WORKER": "junior-creative"}})


def test_unmanaged_agent_is_unchanged_without_adoption(repo, fallback_dir, capsys):
    api = FakeMultica([{"id": "m", "name": "Mitchel", "has_custom_env": False}])
    assert multica_sync.Sync(api, repo, CHECKOUT).run("Mitchel") == 1
    assert api.writes == []
    assert not fallback_dir.exists()
    assert "unmanaged name conflict" in capsys.readouterr().err


def test_dry_run_reports_create_update_archive_adopt_and_overlays_without_writes(repo, mitchel, fallback_dir, capsys):
    definition(repo, "junior-backend", "Junior Backend", models='["openai-codex/gpt-6-luna:high"]', title="Junior Backend")
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
    for text in ("would create agent Junior Backend", "would update agent Mitchel", "would adopt agent Mitchel",
                 "would archive agent Gone", "would write overlay", "would delete overlay"):
        assert text in output
    assert "hidden" not in output


def test_preview_lists_the_overlay_deletes_of_the_write_run(repo, fallback_dir, capsys):
    gone, skuddy = write(fallback_dir / "gone.yml", "retry: {}\n"), write(fallback_dir / "skuddy.yml", "retry: {}\n")

    def api() -> FakeMultica:
        return FakeMultica([
            {"id": "gone", "name": "Gone", "archived_at": None, "has_custom_env": True, "custom_args": [f"--config={gone}"]},
            {"id": "s", "name": "Skuddy", "archived_at": None, "has_custom_env": True, "custom_args": [f"--config={skuddy}"]},
        ], {"gone": {"MADSPEC_PERSONA": "gone"}, "s": {"MADSPEC_PERSONA": "skuddy"}})

    deletes = []
    for dry_run in (True, False):
        assert multica_sync.Sync(api(), repo, CHECKOUT, dry_run=dry_run).run(None) == 0
        output = capsys.readouterr().out.splitlines()
        deletes.append([line.removeprefix("would ") for line in output if "delete overlay" in line])
    assert deletes[0] == deletes[1] == [f"delete overlay {gone}", f"delete overlay {skuddy}"]
    assert sorted(path.name for path in fallback_dir.iterdir()) == ["mitchel.yml"]


def test_adoption_preserves_other_environment_and_requires_unique_name(repo):
    agents = [{"id": "m", "name": "Mitchel", "has_custom_env": True}]
    api = FakeMultica(agents, {"m": {"SECRET": "hidden"}})
    assert multica_sync.Sync(api, repo, CHECKOUT, adopt=["Mitchel"]).run("Mitchel") == 0
    assert ("PUT", "/api/agents/m/env", {"custom_env": {"SECRET": "****", "MADSPEC_PERSONA": "mitchel"}}) in api.writes
    api = FakeMultica(agents + [{"id": "duplicate", "name": "Mitchel"}], {"m": {"SECRET": "hidden"}})
    assert multica_sync.Sync(api, repo, CHECKOUT, adopt=["Mitchel"]).run("Mitchel") == 1
    assert api.writes == []


def test_unreadable_agent_is_reported_and_its_agent_stays(repo, capsys):
    write(repo / "mitchel" / "agent.toml", "name = \n")
    problem = multica_sync.read_personas(repo, CHECKOUT)[1]["mitchel"]
    assert problem.startswith("mitchel/agent.toml: ")
    api = FakeMultica([{"id": "m", "name": "Mitchel", "archived_at": None, "has_custom_env": True}],
                     {"m": {"MADSPEC_PERSONA": "mitchel"}})
    assert multica_sync.Sync(api, repo, CHECKOUT).run(None) == 1
    assert not any(path.endswith("/archive") for _, path, _ in api.writes)
    assert f"persona mitchel: {problem}" in capsys.readouterr().err
