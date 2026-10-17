import subprocess
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import multica_sync  # noqa: E402

CHECKOUT = Path("/main-checkout")
TWO_MODELS = ["anthropic/claude-opus-5-5:high", "openai-codex/gpt-6.1-sol:high"]


def write(path: Path, text: str) -> Path:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")
    return path


def persona_skill(root: Path, skill: str, name: str, models: list[str] = TWO_MODELS, skills: list[str] | None = None) -> dict:
    """Write a persona's `SKILL.md` and return the entry `roster.personas` reports for it."""
    write(root / "skills" / skill / "SKILL.md", f'---\nname: {skill}\ndescription: "{name}, a persona."\n---\n# {name}\n\nBody.\n')
    return {"command": name.lower(), "name": name, "skill": skill, "models": models, "skills": skills}


@pytest.fixture
def repo(tmp_path: Path) -> Path:
    write(tmp_path / "skills" / "demo-skill" / "SKILL.md", "---\nname: demo-skill\n---\n")
    return tmp_path


@pytest.fixture
def roster(repo: Path) -> list[dict]:
    return [
        persona_skill(repo, "as-product-manager", "Mitchel", skills=["as-product-manager", "demo-skill"]),
        persona_skill(repo, "as-chief-of-staff", "Skuddy", models=["x/y"]),
    ]


@pytest.fixture(autouse=True)
def fallback_dir(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    folder = tmp_path / "fallback"
    monkeypatch.setattr(multica_sync, "FALLBACK_DIR", folder)
    return folder


def test_a_persona_runs_as_its_skill_with_its_skills_and_fallbacks(repo, roster, fallback_dir):
    personas, problems = multica_sync.read_personas(repo, CHECKOUT, roster)
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


def test_guus_selects_his_whole_skill_folder(repo, roster):
    roster.append(persona_skill(repo, "as-ux-designer", "Guus"))
    checkout = Path("/Users/codaveto/Repos/Ventures/madspec")

    personas, problems = multica_sync.read_personas(repo, checkout, roster)
    guus = personas["guus"]

    assert problems == {}
    assert guus.instruction_selection == ["/Users/codaveto/Repos/Ventures/madspec/skills/as-ux-designer"]
    assert guus.instructions == (
        "`{skill-root}` is /Users/codaveto/Repos/Ventures/madspec/skills/as-ux-designer. "
        "Your instructions are the as-ux-designer skill below; follow it.\n\n{selection}"
    )


def test_a_persona_with_every_skill_and_one_model_gets_no_arguments(repo, roster):
    skuddy = multica_sync.read_personas(repo, CHECKOUT, roster)[0]["skuddy"]

    assert (skuddy.model, skuddy.thinking_level, skuddy.custom_args) == ("x/y", "", [])


def test_a_persona_without_a_model_is_a_problem_and_no_agent(repo, roster):
    roster.append(persona_skill(repo, "as-verifier", "Sarkout", models=[]))

    personas, problems = multica_sync.read_personas(repo, CHECKOUT, roster)

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
def mitchel(repo, roster) -> multica_sync.Persona:
    return multica_sync.read_personas(repo, CHECKOUT, roster)[0]["mitchel"]


@pytest.mark.parametrize(
    ("agents", "markers", "expected", "take_over"),
    [
        ([{"id": "a", "name": "Other"}], {"a": "mitchel"}, "a", False),
        ([{"id": "a", "name": "Mitchel", "archived_at": "t"}, {"id": "b", "name": "Mitchel", "archived_at": None}], {}, "b", True),
        ([{"id": "a", "name": "Mitchel", "archived_at": "t"}], {}, "a", True),
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


def test_sync_restores_and_takes_over_an_archived_persona_agent(repo, roster, mitchel):
    api = FakeMultica([{"id": "m", "name": "Mitchel", "archived_at": "t", "has_custom_env": False, "runtime_id": "", **multica_sync.OPEN_TO_WORKSPACE}])

    assert multica_sync.Sync(api, repo, CHECKOUT, roster).run("Mitchel") == 0

    assert api.writes[0] == ("POST", "/api/agents/m/restore", None)
    assert api.writes[1] == ("PUT", "/api/agents/m/env", {"custom_env": {"MADSPEC_PERSONA": "mitchel"}})
    fields = {key: value for key, value in mitchel.fields().items() if key != "name"}
    assert api.writes[2] == ("PUT", "/api/agents/m", {**fields, "runtime_id": "omp"})
    assert len(api.writes) == 3


@pytest.mark.parametrize("selection", [None, [], ["/other/skill"]])
def test_sync_rewrites_a_stale_selection_and_leaves_equal_fields(repo, roster, selection, capsys):
    roster.append(persona_skill(repo, "as-ux-designer", "Guus", models=["x/y"]))
    guus = multica_sync.read_personas(repo, CHECKOUT, roster)[0]["guus"]
    agent = {
        **guus.fields(),
        **multica_sync.OPEN_TO_WORKSPACE,
        "id": "g",
        "runtime_id": "omp",
        "has_custom_env": True,
        "instruction_selection": selection,
    }
    api = FakeMultica([agent], {"g": {"MADSPEC_PERSONA": "guus"}})

    assert multica_sync.Sync(api, repo, CHECKOUT, roster).run("Guus") == 0

    assert api.writes == [("PUT", "/api/agents/g", {"instruction_selection": guus.instruction_selection})]
    assert capsys.readouterr().out == "update agent Guus (instruction_selection)\n"

    agent["instruction_selection"] = list(guus.instruction_selection)
    api.writes.clear()

    assert multica_sync.Sync(api, repo, CHECKOUT, roster).run("guus") == 0
    assert api.writes == []
    assert capsys.readouterr().out == ""


def test_full_sync_archives_only_its_agents_whose_persona_is_gone(repo, roster):
    api = FakeMultica(
        [
            {"id": "gone", "name": "Fallon", "archived_at": None, "has_custom_env": True},
            {"id": "foreign", "name": "Explorer", "archived_at": None, "has_custom_env": True},
        ],
        envs={"gone": {"MADSPEC_PERSONA": "fallon"}, "foreign": {"OTHER": "x"}},
    )

    assert multica_sync.Sync(api, repo, CHECKOUT, roster).run(None) == 0

    archived = [path for method, path, _ in api.writes if path.endswith("/archive")]
    assert archived == ["/api/agents/gone/archive"]


def test_sync_writes_overlays_before_agents_and_deletes_those_no_agent_names(repo, roster, fallback_dir, capsys):
    removed = write(fallback_dir / "removed.yml", "retry: {}\n")
    still_named = write(fallback_dir / "unreadable.yml", "retry: {}\n")
    api = FakeMultica([
        {"id": "removed", "name": "Removed", "archived_at": "t", "custom_args": [f"--config={removed}"]},
        {"id": "unreadable", "name": "Unreadable", "archived_at": None, "custom_args": [f"--config={still_named}"]},
    ])

    assert multica_sync.Sync(api, repo, CHECKOUT, roster).run(None) == 0

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
    assert multica_sync.Sync(api, repo, CHECKOUT, roster).run(None) == 0
    assert "write overlay" not in capsys.readouterr().out
