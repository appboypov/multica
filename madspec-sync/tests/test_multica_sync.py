import json
import subprocess
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import multica_sync  # noqa: E402

TWO_MODELS = '["anthropic/claude-opus-5-5:high", "openai-codex/gpt-6.1-sol:high"]'
WORKSPACE = "ws"
PROFILES = f"/api/workspaces/{WORKSPACE}/runtime-profiles"
DAEMON = "d"
DAEMON_PROFILE = "desktop-localhost-8080"


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


def alias(command: str) -> str:
    return str(multica_sync.ALIASES / command)


def test_an_agent_runs_through_its_alias_with_its_fallbacks(repo, fallback_dir):
    personas, problems = multica_sync.read_personas(repo)
    mitchel = personas["mitchel"]

    assert problems == {}
    assert mitchel.fields() == {
        "name": "Mitchel",
        "description": "Product Manager",
        "instructions": "",
        "instruction_selection": [],
        "custom_args": [f"--config={fallback_dir / 'mitchel.yml'}"],
    }
    assert mitchel.seed() == {"model": "anthropic/claude-opus-5-5", "thinking_level": "high"}
    assert mitchel.marker == "MADSPEC_PERSONA"
    assert multica_sync.fallback_chain(mitchel.models) == {"anthropic/claude-opus-5-5": ["openai-codex/gpt-6.1-sol:high"]}


def test_an_agent_with_one_model_gets_no_arguments(repo):
    skuddy = multica_sync.read_personas(repo)[0]["skuddy"]

    assert (skuddy.model, skuddy.thinking_level, skuddy.custom_args) == ("x/y", "", [])


def test_a_worker_is_a_folder_named_after_its_level(repo):
    definition(repo, "medior-backend", "Medior Backend", models='["openai-codex/gpt-6.1-sol:high"]', title="Medior Backend")

    worker = multica_sync.read_personas(repo)[0]["medior-backend"]

    assert (worker.name, worker.model, worker.thinking_level, worker.custom_args, worker.marker) == (
        "Medior Backend", "openai-codex/gpt-6.1-sol", "high", [], "MADSPEC_WORKER")


def test_a_folder_without_agent_toml_is_no_agent(repo):
    write(repo / "tika" / "AGENTS.md", "# Tika\n")

    personas, problems = multica_sync.read_personas(repo)

    assert set(personas) == {"mitchel", "skuddy"}
    assert problems == {}


@pytest.mark.parametrize(
    ("models", "problem"),
    [(None, "sarkout/agent.toml has no models"), ("[]", "sarkout/agent.toml has no models")],
)
def test_an_agent_without_models_is_a_problem_and_no_agent(repo, models, problem):
    definition(repo, "sarkout", "Sarkout", models=models)

    personas, problems = multica_sync.read_personas(repo)

    assert "sarkout" not in personas
    assert problems == {"sarkout": problem}


def test_an_agent_without_agents_md_is_a_problem(repo):
    (repo / "mitchel" / "AGENTS.md").unlink()

    assert multica_sync.read_personas(repo)[1] == {"mitchel": "mitchel/AGENTS.md does not exist"}


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
    return multica_sync.read_personas(repo)[0]["mitchel"]


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


def test_agent_changes_never_include_the_model_or_thinking_level(mitchel):
    agent = {**mitchel.fields(), "model": "other/model", "thinking_level": "low", "instructions": None, "instruction_selection": None, "custom_args": None}

    assert multica_sync.agent_changes(agent, mitchel) == {"custom_args": mitchel.custom_args}
    assert multica_sync.agent_changes({**mitchel.fields(), "model": "other/model"}, mitchel) == {}


def test_a_profile_runtime_is_the_local_one_the_running_daemon_registered():
    runtimes = [
        runtime("omp", None),
        runtime("other-daemon", "p-mitchel", daemon_id="e"),
        {**runtime("cloud", "p-mitchel"), "runtime_mode": "cloud"},
        runtime("rt-mitchel", "p-mitchel"),
    ]

    assert multica_sync.profile_runtime(runtimes, "p-mitchel", DAEMON) == "rt-mitchel"
    assert multica_sync.profile_runtime(runtimes, "p-guus", DAEMON) is None




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


# --- Syncing against a fake Multica and daemon ---


def runtime(runtime_id: str, profile_id: str | None, daemon_id: str = DAEMON) -> dict:
    return {"id": runtime_id, "provider": "omp", "runtime_mode": "local", "daemon_id": daemon_id, "profile_id": profile_id}


def registered(*commands: str) -> list[dict]:
    """The built-in omp runtime and, per command, the runtime the daemon registered for its profile."""
    return [runtime("omp", None), *(runtime(f"rt-{command}", f"p-{command}") for command in commands)]


def profile(command: str) -> dict:
    return {"id": f"p-{command}", "display_name": command.title(), "runtime_type": "omp", "command_name": command}


class FakeMultica:
    """A Multica workspace that keeps its agents, envs, runtime profiles and runtimes,
    records each write and which `--config=` files were missing when each agent was
    created. A created profile gets the id `p-<command>`, a created agent its lowercase name."""

    workspace_id = WORKSPACE

    def __init__(self, agents=(), envs=None, profiles=(), runtimes=None) -> None:
        self.agents = [dict(agent) for agent in agents]
        self.envs = {key: dict(value) for key, value in (envs or {}).items()}
        self.profiles = [dict(item) for item in profiles]
        self.runtimes = registered("mitchel", "skuddy", "guus") if runtimes is None else runtimes
        self.writes: list[tuple[str, str, object]] = []
        self.missing_at_create: dict[str, list[str]] = {}

    def agent(self, agent_id: str) -> dict:
        return next(agent for agent in self.agents if agent["id"] == agent_id)

    def call(self, method: str, path: str, body: dict | None = None) -> object:
        if method == "GET":
            if path == "/api/runtimes":
                return [dict(item) for item in self.runtimes]
            if path == PROFILES:
                return {"runtime_profiles": [dict(item) for item in self.profiles]}
            if path.startswith("/api/agents?"):
                return [dict(agent) for agent in self.agents]
            if path.endswith("/env"):
                return {"custom_env": dict(self.envs[path.split("/")[3]])}
            raise AssertionError(f"unexpected GET {path}")
        self.writes.append((method, path, body))
        if path == PROFILES:
            created = {"id": f"p-{body['command_name']}", **body}
            self.profiles.append(created)
            return dict(created)
        if path == "/api/agents":
            configs = [arg.removeprefix("--config=") for arg in body["custom_args"] if arg.startswith("--config=")]
            self.missing_at_create[body["name"]] = [config for config in configs if not Path(config).is_file()]
            self.agents.append({**body, "id": body["name"].lower(), "archived_at": None, "has_custom_env": False})
            return {"id": body["name"].lower()}
        agent_id = path.split("/")[3]
        if path.endswith("/env"):
            current = self.envs.get(agent_id, {})
            self.envs[agent_id] = {key: current[key] if value == multica_sync.KEEP_ENV else value for key, value in body["custom_env"].items()}
            self.agent(agent_id)["has_custom_env"] = True
        elif path.endswith("/restore"):
            self.agent(agent_id)["archived_at"] = None
        elif path.endswith("/archive"):
            self.agent(agent_id)["archived_at"] = "t"
        else:
            self.agent(agent_id).update(body)
        return {}

    def agent_writes(self) -> list[tuple[str, str, object]]:
        return [write for write in self.writes if write[1].startswith("/api/agents")]


class FakeMachine:
    """The daemon running under `DAEMON_PROFILE`, or none, or `error` when its discovery fails."""

    def __init__(self, running: bool = True, pins: dict | None = None, error: str | None = None) -> None:
        self.status = {"status": "running", "profile": DAEMON_PROFILE, "daemon_id": DAEMON} if running else None
        self.error = error
        self.pins = dict(pins or {})
        self.pin_writes: list[tuple[str, str, str]] = []

    def running_daemon(self) -> dict | None:
        if self.error is not None:
            raise ValueError(self.error)
        return self.status

    def pinned(self, profile: str, profile_id: str) -> str | None:
        return self.pins.get((profile, profile_id))

    def pin(self, profile: str, profile_id: str, path: Path) -> None:
        self.pins[(profile, profile_id)] = str(path)
        self.pin_writes.append((profile, profile_id, str(path)))


def synced(agent_id: str, name: str, **fields) -> dict:
    return {"id": agent_id, "name": name, "archived_at": None, "has_custom_env": True, "runtime_id": "omp", **multica_sync.OPEN_TO_WORKSPACE, **fields}


def test_mitchel_gets_his_runtime_profile_and_moves_onto_its_runtime(repo, mitchel):
    api = FakeMultica([synced("m", **mitchel.fields())], {"m": {"MADSPEC_PERSONA": "mitchel"}})
    machine = FakeMachine()

    assert multica_sync.Sync(api, repo, machine).run("mitchel") == 0

    assert api.writes[0] == ("POST", PROFILES, {"display_name": "Mitchel", "runtime_type": "omp", "command_name": "mitchel"})
    assert machine.pin_writes == [(DAEMON_PROFILE, "p-mitchel", alias("mitchel"))]
    assert api.agent_writes() == [("PUT", "/api/agents/m", {"runtime_id": "rt-mitchel"})]


@pytest.mark.parametrize("pinned", [True, False])
def test_an_existing_profile_is_reused_and_pinned_only_when_its_path_is_missing(repo, mitchel, pinned, capsys):
    api = FakeMultica([synced("m", **mitchel.fields())], {"m": {"MADSPEC_PERSONA": "mitchel"}}, profiles=[profile("mitchel")])
    machine = FakeMachine(pins={(DAEMON_PROFILE, "p-mitchel"): alias("mitchel")} if pinned else {})

    assert multica_sync.Sync(api, repo, machine).run("mitchel") == 0

    assert [write for write in api.writes if write[1] == PROFILES] == []
    assert api.profiles == [profile("mitchel")]
    assert machine.pin_writes == ([] if pinned else [(DAEMON_PROFILE, "p-mitchel", alias("mitchel"))])
    assert ("restart the daemon" in capsys.readouterr().out) is not pinned
    assert api.agent("m")["runtime_id"] == "rt-mitchel"


def test_a_deleted_profile_is_created_again(repo):
    definition(repo, "guus", "Guus", title="UX Designer")
    api = FakeMultica()
    machine = FakeMachine()
    assert multica_sync.Sync(api, repo, machine).run("guus") == 0
    api.profiles.clear()
    api.agent("guus")["runtime_id"] = "omp"
    api.writes.clear()

    assert multica_sync.Sync(api, repo, machine).run("guus") == 0

    assert [write for write in api.writes if write[1] == PROFILES] == [
        ("POST", PROFILES, {"display_name": "Guus", "runtime_type": "omp", "command_name": "guus"})
    ]
    assert api.agent("guus")["runtime_id"] == "rt-guus"


def test_a_new_agent_is_seeded_and_keeps_its_multica_edits_while_an_archived_one_stays(repo):
    roos = {"id": "roos", "name": "Roos", "archived_at": "2026-10-15", "has_custom_env": True}
    api = FakeMultica([roos], {"roos": {"MADSPEC_PERSONA": "roos"}})
    machine = FakeMachine()

    assert multica_sync.Sync(api, repo, machine).run(None) == 0

    mitchel = api.agent("mitchel")
    assert (mitchel["model"], mitchel["thinking_level"], mitchel["runtime_id"], mitchel["archived_at"]) == (
        "anthropic/claude-opus-5-5", "high", "rt-mitchel", None,
    )
    assert api.envs["mitchel"] == {"MADSPEC_PERSONA": "mitchel"}
    mitchel.update(model="openai-codex/gpt-6.1-sol", thinking_level="low")
    definition(repo, "mitchel", "Mitchel", models='["z/other:max"]')
    api.writes.clear()

    assert multica_sync.Sync(api, repo, machine).run(None) == 0

    assert api.agent_writes() == [("PUT", "/api/agents/mitchel", {"custom_args": []})]
    assert (mitchel["model"], mitchel["thinking_level"]) == ("openai-codex/gpt-6.1-sol", "low")
    assert api.agent("roos")["archived_at"] == "2026-10-15"


def test_a_folder_without_agent_toml_and_the_helper_agents_get_no_profile_and_keep_their_runtimes(repo):
    write(repo / "tika" / "AGENTS.md", "# Tika\n")
    helpers = [
        {"id": name.lower(), "name": name, "archived_at": None, "has_custom_env": False, "runtime_id": f"{name.lower()}-runtime"}
        for name in ("Herdr", "Astra", "Fable", "Haiku", "Tika")
    ]
    api = FakeMultica(helpers)

    assert multica_sync.Sync(api, repo, FakeMachine()).run(None) == 0

    assert sorted(body["command_name"] for method, path, body in api.writes if path == PROFILES) == ["mitchel", "skuddy"]
    helper_ids = {agent["id"] for agent in helpers}
    assert [path for _, path, _ in api.agent_writes() if path.removeprefix("/api/agents/").split("/")[0] in helper_ids] == []
    assert {agent["id"]: agent["runtime_id"] for agent in api.agents if agent["id"] in helper_ids} == {
        agent["id"]: agent["runtime_id"] for agent in helpers
    }


def test_a_profile_without_a_registered_runtime_fails_only_that_agent(repo, mitchel, capsys):
    definition(repo, "guus", "Guus", title="UX Designer")
    agents = [synced("m", **mitchel.fields()), synced("g", "Guus", instructions="old")]
    envs = {"m": {"MADSPEC_PERSONA": "mitchel"}, "g": {"MADSPEC_PERSONA": "guus"}}
    api = FakeMultica(agents, envs, runtimes=registered("mitchel", "skuddy"))

    assert multica_sync.Sync(api, repo, FakeMachine()).run(None) == 1

    assert [write for write in api.writes if write[1].startswith("/api/agents/g")] == []
    assert api.agent("m")["runtime_id"] == "rt-mitchel"
    assert "skuddy" in [agent["id"] for agent in api.agents]
    assert ("POST", PROFILES, {"display_name": "Guus", "runtime_type": "omp", "command_name": "guus"}) in api.writes
    assert capsys.readouterr().err == f"failed: persona Guus: {multica_sync.NO_RUNTIME}\n"


@pytest.mark.parametrize(
    ("machine", "message"),
    [
        (FakeMachine(running=False), "the Multica daemon is not running"),
        (FakeMachine(error="more than one Multica daemon serves"), "more than one Multica daemon serves"),
    ],
)
def test_without_one_running_daemon_the_sync_fails_before_any_write(repo, fallback_dir, machine, message, capsys):
    api = FakeMultica([synced("m", "Mitchel", instructions="old")], {"m": {"MADSPEC_PERSONA": "mitchel"}})

    assert multica_sync.Sync(api, repo, machine).run(None) == 1

    assert api.writes == []
    assert machine.pin_writes == []
    assert not fallback_dir.exists()
    assert message in capsys.readouterr().err


def test_the_selection_instructions_and_skills_go_and_the_fallback_overlay_stays(repo, fallback_dir, capsys):
    overlay = f"--config={fallback_dir / 'mitchel.yml'}"
    stale = {
        "description": "Product Manager",
        "instructions": "<SystemIdentity>\n{selection}\n</SystemIdentity>",
        "instruction_selection": [str(multica_sync.OFFICES / "mitchel" / "AGENTS.md")],
        "custom_args": ["--skills=as-product-manager,demo-skill", overlay],
        "runtime_id": "rt-mitchel",
    }
    api = FakeMultica([synced("m", "Mitchel", **stale)], {"m": {"MADSPEC_PERSONA": "mitchel"}})

    assert multica_sync.Sync(api, repo, FakeMachine()).run("mitchel") == 0

    assert api.agent_writes() == [("PUT", "/api/agents/m", {"instructions": "", "instruction_selection": [], "custom_args": [overlay]})]
    assert capsys.readouterr().out.splitlines()[-1] == "update agent Mitchel (instructions, instruction_selection, custom_args)"

    api.writes.clear()
    assert multica_sync.Sync(api, repo, FakeMachine(pins={(DAEMON_PROFILE, "p-mitchel"): alias("mitchel")})).run("mitchel") == 0
    assert api.writes == []


def test_sync_adopts_one_explicit_archived_agent(repo, mitchel):
    api = FakeMultica([{"id": "m", "name": "Mitchel", "archived_at": "t", "has_custom_env": False, "runtime_id": "", **multica_sync.OPEN_TO_WORKSPACE}])

    assert multica_sync.Sync(api, repo, FakeMachine(), adopt=["Mitchel"]).run("Mitchel") == 0

    assert api.agent_writes() == [
        ("POST", "/api/agents/m/restore", None),
        ("PUT", "/api/agents/m/env", {"custom_env": {"MADSPEC_PERSONA": "mitchel"}}),
        ("PUT", "/api/agents/m", {"description": "Product Manager", "custom_args": mitchel.custom_args, "runtime_id": "rt-mitchel"}),
    ]


def test_full_sync_archives_only_its_agents_whose_folder_is_gone(repo):
    api = FakeMultica(
        [
            {"id": "gone", "name": "Fallon", "archived_at": None, "has_custom_env": True},
            {"id": "worker", "name": "First Reviewer GPT", "archived_at": None, "has_custom_env": True},
            {"id": "foreign", "name": "Explorer", "archived_at": None, "has_custom_env": True},
        ],
        envs={"gone": {"MADSPEC_PERSONA": "fallon"}, "worker": {"MADSPEC_WORKER": "first-reviewer-gpt"}, "foreign": {"OTHER": "x"}},
    )

    assert multica_sync.Sync(api, repo, FakeMachine()).run(None) == 0

    archived = [path for method, path, _ in api.writes if path.endswith("/archive")]
    assert archived == ["/api/agents/gone/archive", "/api/agents/worker/archive"]


def test_sync_writes_overlays_before_agents_and_deletes_those_no_agent_names(repo, fallback_dir, capsys):
    removed = write(fallback_dir / "removed.yml", "retry: {}\n")
    still_named = write(fallback_dir / "unreadable.yml", "retry: {}\n")
    api = FakeMultica([
        {"id": "removed", "name": "Removed", "archived_at": "t", "custom_args": [f"--config={removed}"]},
        {"id": "unreadable", "name": "Unreadable", "archived_at": None, "custom_args": [f"--config={still_named}"]},
    ])

    assert multica_sync.Sync(api, repo, FakeMachine()).run(None) == 0

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
    assert multica_sync.Sync(api, repo, FakeMachine()).run(None) == 0
    assert "write overlay" not in capsys.readouterr().out


def test_a_new_worker_gets_its_marker_and_profile(repo):
    definition(repo, "junior-creative", "Junior Creative", models='["anthropic/claude-haiku-5-5:high"]', title="Junior Creative")
    api = FakeMultica(runtimes=registered("junior-creative"))

    assert multica_sync.Sync(api, repo, FakeMachine()).run("Junior Creative") == 0

    created = api.agent_writes()[0][2]
    assert (created["runtime_id"], created["custom_args"], created["model"]) == ("rt-junior-creative", [], "anthropic/claude-haiku-5-5")
    assert api.agent_writes()[1] == ("PUT", "/api/agents/junior creative/env", {"custom_env": {"MADSPEC_WORKER": "junior-creative"}})


def test_unmanaged_agent_is_unchanged_without_adoption(repo, fallback_dir, capsys):
    api = FakeMultica([{"id": "m", "name": "Mitchel", "has_custom_env": False}])

    assert multica_sync.Sync(api, repo, FakeMachine()).run("Mitchel") == 1

    assert api.agent_writes() == []
    assert not fallback_dir.exists()
    assert "unmanaged name conflict" in capsys.readouterr().err


def test_dry_run_reports_create_update_archive_adopt_pin_and_overlays_without_writes(repo, mitchel, fallback_dir, capsys):
    definition(repo, "junior-backend", "Junior Backend", models='["openai-codex/gpt-6-luna:high"]', title="Junior Backend")
    removed = write(fallback_dir / "removed.yml", "keep this content")
    api = FakeMultica(
        [
            {"id": "m", "name": "Mitchel", "has_custom_env": True, "runtime_id": "other"},
            {"id": "gone", "name": "Gone", "has_custom_env": True},
        ],
        {"m": {"SECRET": "hidden"}, "gone": {"MADSPEC_PERSONA": "gone"}},
        profiles=[profile("mitchel"), profile("skuddy"), profile("junior-backend")],
        runtimes=registered("mitchel", "skuddy", "junior-backend"),
    )
    machine = FakeMachine()

    assert multica_sync.Sync(api, repo, machine, dry_run=True, adopt=["Mitchel"]).run(None) == 0

    assert api.writes == []
    assert machine.pin_writes == []
    assert list(fallback_dir.iterdir()) == [removed]
    assert removed.read_text() == "keep this content"
    output = capsys.readouterr().out
    for text in ("would pin runtime profile mitchel", "would create agent Junior Backend", "would update agent Mitchel",
                 "would adopt agent Mitchel", "would archive agent Gone", "would write overlay", "would delete overlay"):
        assert text in output
    assert "hidden" not in output


def test_dry_run_of_a_missing_profile_reports_its_create_and_the_restart(repo, capsys):
    api = FakeMultica(profiles=[profile("skuddy")])

    assert multica_sync.Sync(api, repo, FakeMachine(), dry_run=True).run("mitchel") == 1

    assert api.writes == []
    captured = capsys.readouterr()
    assert captured.out.splitlines()[:2] == ["would create runtime profile mitchel", f"would pin runtime profile mitchel to {alias('mitchel')}"]
    assert captured.err == f"failed: persona Mitchel: {multica_sync.NO_RUNTIME}\n"


def test_preview_lists_the_overlay_deletes_of_the_write_run(repo, fallback_dir, capsys):
    gone, skuddy = write(fallback_dir / "gone.yml", "retry: {}\n"), write(fallback_dir / "skuddy.yml", "retry: {}\n")

    def api() -> FakeMultica:
        return FakeMultica(
            [
                {"id": "gone", "name": "Gone", "archived_at": None, "has_custom_env": True, "custom_args": [f"--config={gone}"]},
                {"id": "s", "name": "Skuddy", "archived_at": None, "has_custom_env": True, "custom_args": [f"--config={skuddy}"]},
            ],
            {"gone": {"MADSPEC_PERSONA": "gone"}, "s": {"MADSPEC_PERSONA": "skuddy"}},
            profiles=[profile("mitchel"), profile("skuddy")],
        )

    deletes = []
    for dry_run in (True, False):
        assert multica_sync.Sync(api(), repo, FakeMachine(), dry_run=dry_run).run(None) == 0
        output = capsys.readouterr().out.splitlines()
        deletes.append([line.removeprefix("would ") for line in output if "delete overlay" in line])
    assert deletes[0] == deletes[1] == [f"delete overlay {gone}", f"delete overlay {skuddy}"]
    assert sorted(path.name for path in fallback_dir.iterdir()) == ["mitchel.yml"]


def test_adoption_preserves_other_environment_and_requires_unique_name(repo):
    agents = [{"id": "m", "name": "Mitchel", "has_custom_env": True}]
    api = FakeMultica(agents, {"m": {"SECRET": "hidden"}})
    assert multica_sync.Sync(api, repo, FakeMachine(), adopt=["Mitchel"]).run("Mitchel") == 0
    assert ("PUT", "/api/agents/m/env", {"custom_env": {"SECRET": "****", "MADSPEC_PERSONA": "mitchel"}}) in api.writes

    api = FakeMultica(agents + [{"id": "duplicate", "name": "Mitchel"}], {"m": {"SECRET": "hidden"}})
    assert multica_sync.Sync(api, repo, FakeMachine(), adopt=["Mitchel"]).run("Mitchel") == 1
    assert api.agent_writes() == []


def test_unreadable_agent_is_reported_and_its_agent_stays(repo, capsys):
    write(repo / "mitchel" / "agent.toml", "name = \n")
    problem = multica_sync.read_personas(repo)[1]["mitchel"]
    assert problem.startswith("mitchel/agent.toml: ")
    api = FakeMultica([{"id": "m", "name": "Mitchel", "archived_at": None, "has_custom_env": True}], {"m": {"MADSPEC_PERSONA": "mitchel"}})

    assert multica_sync.Sync(api, repo, FakeMachine()).run(None) == 1

    assert not any(path.endswith("/archive") for _, path, _ in api.writes)
    assert f"persona mitchel: {problem}" in capsys.readouterr().err


# --- The daemon and CLI configs through a fake `multica` CLI ---

SERVER = "http://127.0.0.1:8080"


def daemon_status(profile: str, daemon_id: str, server: str = SERVER, workspace: str = WORKSPACE) -> dict:
    return {"status": "running", "profile": profile, "daemon_id": daemon_id, "server_url": server, "workspaces": [{"id": workspace, "runtimes": []}]}


def fake_cli(tmp_path: Path, home: Path, statuses: dict[str, dict]) -> tuple[str, Path]:
    """A `multica` that answers `daemon status` per `--profile` from `statuses`, stopped
    otherwise, saves `runtime profile set-path` into that profile's CLI config as the real
    CLI does, and logs its arguments."""
    log = tmp_path / "cli.log"
    script = tmp_path / "bin" / "multica"
    write(
        script,
        f"""#!{sys.executable}
import json, pathlib, sys
args = sys.argv[1:]
with open({str(log)!r}, "a", encoding="utf-8") as handle:
    handle.write(json.dumps(args) + "\\n")
profile = args[1] if args[:1] == ["--profile"] else ""
rest = args[2:] if profile else args
if rest[:2] == ["daemon", "status"]:
    print(json.dumps(json.loads({json.dumps(statuses)!r}).get(profile, {{"status": "stopped"}})))
elif rest[:3] == ["runtime", "profile", "set-path"]:
    home = pathlib.Path({str(home)!r})
    path = home / "profiles" / profile / "config.json" if profile else home / "config.json"
    config = json.loads(path.read_text()) if path.is_file() else {{}}
    config.setdefault("profile_command_overrides", {{}})[rest[3]] = rest[5]
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(config))
""",
    )
    script.chmod(0o755)
    return str(script), log


def cli_calls(log: Path) -> list[list[str]]:
    return [json.loads(line) for line in log.read_text(encoding="utf-8").splitlines()] if log.is_file() else []


@pytest.fixture
def home(tmp_path: Path) -> Path:
    for name in ("desktop-localhost-8080", "local", "other-server", "other-workspace"):
        (tmp_path / "multica" / "profiles" / name).mkdir(parents=True)
    return tmp_path / "multica"


def test_the_default_daemon_serving_the_sync_is_used_without_looking_further(tmp_path, home):
    cli, log = fake_cli(tmp_path, home, {"": daemon_status("", "default")})

    assert multica_sync.Machine(SERVER + "/", WORKSPACE, home, cli).running_daemon()["daemon_id"] == "default"
    assert cli_calls(log) == [["daemon", "status", "--output", "json"]]


def test_a_stopped_default_daemon_leads_to_the_one_profile_daemon_serving_the_sync(tmp_path, home):
    statuses = {
        "desktop-localhost-8080": daemon_status("desktop-localhost-8080", "desktop"),
        "other-server": daemon_status("other-server", "a", server="http://elsewhere"),
        "other-workspace": daemon_status("other-workspace", "b", workspace="other"),
    }
    cli, _ = fake_cli(tmp_path, home, statuses)

    assert multica_sync.Machine(SERVER, WORKSPACE, home, cli).running_daemon()["profile"] == "desktop-localhost-8080"


def test_no_daemon_serving_the_sync_is_none_and_two_are_an_error(tmp_path, home):
    cli, _ = fake_cli(tmp_path, home, {"other-server": daemon_status("other-server", "a", server="http://elsewhere")})
    assert multica_sync.Machine(SERVER, WORKSPACE, home, cli).running_daemon() is None

    two = {"local": daemon_status("local", "one"), "desktop-localhost-8080": daemon_status("desktop-localhost-8080", "two")}
    cli, _ = fake_cli(tmp_path / "two", home, two)
    with pytest.raises(ValueError, match="more than one Multica daemon serves .*profiles desktop-localhost-8080, local"):
        multica_sync.Machine(SERVER, WORKSPACE, home, cli).running_daemon()


def test_a_profile_path_is_pinned_in_the_daemon_profile_cli_config(tmp_path, home):
    cli, log = fake_cli(tmp_path, home, {})
    machine = multica_sync.Machine(SERVER, WORKSPACE, home, cli)

    machine.pin("desktop-localhost-8080", "p1", Path("/bin/mitchel"))

    assert cli_calls(log) == [["--profile", "desktop-localhost-8080", "runtime", "profile", "set-path", "p1", "--path", "/bin/mitchel"]]
    assert machine.pinned("desktop-localhost-8080", "p1") == "/bin/mitchel"
    assert machine.pinned("", "p1") is None


# --- End to end: the real API client and CLI calls against a fake server and CLI ---


@pytest.fixture
def server():
    """A local HTTP server answering the Multica API from a `FakeMultica`."""
    fake = FakeMultica()

    class Handler(BaseHTTPRequestHandler):
        def answer(self) -> None:
            length = int(self.headers.get("Content-Length") or 0)
            body = json.loads(self.rfile.read(length)) if length else None
            assert self.headers["X-Workspace-ID"] == WORKSPACE
            raw = json.dumps(fake.call(self.command, self.path, body)).encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)

        do_GET = do_POST = do_PUT = answer

        def log_message(self, *args) -> None:
            pass

    httpd = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=httpd.serve_forever, daemon=True)
    thread.start()
    yield fake, f"http://127.0.0.1:{httpd.server_address[1]}"
    httpd.shutdown()
    httpd.server_close()


def test_end_to_end_a_new_agent_is_seeded_and_keeps_its_multica_edits(repo, tmp_path, home, server):
    fake, url = server
    cli, log = fake_cli(tmp_path, home, {"desktop-localhost-8080": daemon_status("desktop-localhost-8080", DAEMON, server=url)})

    def run() -> int:
        api = multica_sync.Multica({"server_url": url, "workspace_id": WORKSPACE, "token": "t"})
        return multica_sync.Sync(api, repo, multica_sync.Machine(url, WORKSPACE, home, cli)).run(None)

    assert run() == 0

    mitchel = fake.agent("mitchel")
    assert (mitchel["model"], mitchel["thinking_level"], mitchel["runtime_id"]) == ("anthropic/claude-opus-5-5", "high", "rt-mitchel")
    assert sorted(call[5] for call in cli_calls(log) if "set-path" in call) == ["p-mitchel", "p-skuddy"]

    mitchel.update(model="openai-codex/gpt-6.1-sol", thinking_level="low")
    definition(repo, "mitchel", "Mitchel", models='["z/other:max"]')
    fake.writes.clear()
    log.unlink()

    assert run() == 0

    assert fake.writes == [("PUT", "/api/agents/mitchel", {"custom_args": []})]
    assert (fake.agent("mitchel")["model"], fake.agent("mitchel")["thinking_level"]) == ("openai-codex/gpt-6.1-sol", "low")
    assert [call for call in cli_calls(log) if "set-path" in call] == []
