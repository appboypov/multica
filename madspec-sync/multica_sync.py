#!/usr/bin/env python3
# /// script
# requires-python = ">=3.11"
# ///
"""Sync the agents and workers of the agents repo to local omp agents.

Source configuration comes from origin/main in the agents repo. The contract is
specs/multica-agents.md. Run by hand; --dry-run previews every API and overlay
write. --adopt NAME allows one named unmanaged agent to join the sync. Squads
are outside this tool.
"""

from __future__ import annotations

import argparse
import io
import json
import subprocess
import sys
import tarfile
import tempfile
import tomllib
import urllib.error
import urllib.request
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

sys.dont_write_bytecode = True

# The agents repo: one folder per agent or worker, each with its `agent.toml` and `AGENTS.md`.
OFFICES = Path.home() / "Repos" / "Agents"

MAIN_REF = "refs/remotes/origin/main"
SOURCES = (":(glob)*/agent.toml", ":(glob)*/AGENTS.md")
# The folder names of workers start with their level.
WORKER_LEVELS = ("junior-", "medior-", "senior-")
CONFIG = Path.home() / ".multica" / "profiles" / "local" / "config.json"
# The fallback overlays the sync writes, one per agent with more than one model: derived state, outside the agents repo.
FALLBACK_DIR = Path.home() / ".local" / "state" / "madspec" / "multica"
MARKER = "MADSPEC_PERSONA"
WORKER_MARKER = "MADSPEC_WORKER"
# The env value Multica keeps a key's current value for: agent_env.go envSentinel.
KEEP_ENV = "****"
# omp's thinking levels.
THINKING_LEVELS = {"off", "minimal", "low", "medium", "high", "xhigh", "max"}
AGENT_FIELDS = ("name", "description", "instructions", "instruction_selection", "model", "thinking_level", "custom_args")
OPEN_TO_WORKSPACE = {"visibility": "workspace", "permission_mode": "public_to", "invocation_targets": [{"target_type": "workspace"}]}


@dataclass(frozen=True)
class Persona:
    """A Multica agent as its folder in the agents repo defines it."""

    command: str
    name: str
    description: str
    instructions: str
    instruction_selection: list[str]
    model: str
    thinking_level: str
    custom_args: list[str]
    models: list[str]
    marker: str = MARKER
    source: str = ""

    def fields(self) -> dict[str, object]:
        return {key: getattr(self, key) for key in AGENT_FIELDS}


class ApiError(Exception):
    """A Multica request failed."""


class Unreachable(ApiError):
    """Multica did not answer."""


def main() -> int:
    parser = argparse.ArgumentParser(description="Sync the agents and workers of the agents repo to Multica.")
    parser.add_argument("persona", nargs="?", help="sync only this agent or worker, by name or folder")
    parser.add_argument("--dry-run", action="store_true", help="print proposed changes without writing")
    parser.add_argument("--adopt", action="append", default=[], metavar="NAME", help="manage one existing same-name agent; repeat for each name")
    args = parser.parse_args()
    try:
        api = Multica(load_config(CONFIG))
        with tempfile.TemporaryDirectory(prefix="multica-sync-") as tree:
            export_main(OFFICES, Path(tree))
            return Sync(api, Path(tree), OFFICES, dry_run=args.dry_run, adopt=args.adopt).run(args.persona)
    except (ApiError, ValueError, OSError) as error:
        print(f"failed: {error}", file=sys.stderr, flush=True)
        return 1


# --- Main ---


def export_main(repo: Path, into: Path) -> None:
    """Write each folder's `agent.toml` and `AGENTS.md` as `MAIN_REF` holds them into the folder `into`.

    Raises ValueError when git cannot read `MAIN_REF` or the archive cannot be extracted.
    """
    try:
        archive = git(repo, "archive", "--format=tar", MAIN_REF, "--", *SOURCES)
    except ValueError as error:
        raise ValueError(f"cannot read {MAIN_REF}: {error}") from error
    try:
        with tarfile.open(fileobj=io.BytesIO(archive)) as tar:
            tar.extractall(into, filter="data")
    except tarfile.TarError as error:
        raise ValueError(f"cannot extract {MAIN_REF}: {error}") from error


def git(repo: Path, *args: str) -> bytes:
    """The output of a git command run in `repo`; raises ValueError with git's reason when it fails."""
    try:
        return subprocess.run(["git", *args], cwd=repo, capture_output=True, check=True).stdout
    except subprocess.CalledProcessError as error:
        raise ValueError(error.stderr.decode("utf-8", errors="replace").strip()) from error
    except OSError as error:
        raise ValueError(str(error)) from error


# --- Agent definitions ---


def split_model(value: str) -> tuple[str, str]:
    """A `models` entry as the model and its thinking level, `""` without a level suffix."""
    model, colon, level = value.rpartition(":")
    if colon and level in THINKING_LEVELS:
        return model, level
    return value, ""


def fallback_chain(models: list[str]) -> dict[str, list[str]] | None:
    """omp's `retry.fallbackChains` of a `models` list: the first model without its level
    to the other entries in order, each with its level; None for one model."""
    if len(models) < 2:
        return None
    return {split_model(models[0])[0]: models[1:]}


def fallback_overlay(command: str) -> Path:
    """The fallback overlay file of the agent in the folder `command`."""
    return FALLBACK_DIR / f"{command}.yml"


def overlay_text(source: str, chain: dict[str, list[str]]) -> str:
    """The omp overlay of a fallback chain, with JSON scalars for model selectors."""
    lines = [
        f"# Written by multica_sync.py from {source} on origin/main.",
        "retry:",
        "  modelFallback: true",
        "  fallbackChains:",
    ]
    lines += [f"    {json.dumps(model)}: {json.dumps(selectors)}" for model, selectors in chain.items()]
    return "\n".join(lines) + "\n"


def read_persona(folder: Path, checkout: Path) -> Persona:
    """The agent the folder's `agent.toml` and `AGENTS.md` define.

    Raises ValueError when `agent.toml` is unreadable, has no name or no models, or `AGENTS.md` is missing.
    """
    command = folder.name
    source = f"{command}/agent.toml"
    try:
        definition = tomllib.loads((folder / "agent.toml").read_text(encoding="utf-8"))
    except (OSError, UnicodeError, tomllib.TOMLDecodeError) as error:
        raise ValueError(f"{source}: {error}") from error
    name, title, models = definition.get("name"), definition.get("title", ""), definition.get("models")
    if not isinstance(name, str) or not name.strip() or not isinstance(title, str):
        raise ValueError(f"{source} needs a name, and a title that is text")
    if not isinstance(models, list) or not models or not all(isinstance(model, str) and model for model in models):
        raise ValueError(f"{source} has no models")
    if not (folder / "AGENTS.md").is_file():
        raise ValueError(f"{command}/AGENTS.md does not exist")
    instructions = checkout / command / "AGENTS.md"
    model, thinking_level = split_model(models[0])
    return Persona(
        command=command,
        name=name,
        description=title,
        instructions=f"Your instructions are {instructions} below; follow them.\n\n{{selection}}",
        instruction_selection=[str(instructions)],
        model=model,
        thinking_level=thinking_level,
        custom_args=[f"--config={fallback_overlay(command)}"] if fallback_chain(models) else [],
        models=models,
        marker=WORKER_MARKER if command.startswith(WORKER_LEVELS) else MARKER,
        source=source,
    )


def read_personas(tree: Path, checkout: Path) -> tuple[dict[str, Persona], dict[str, str]]:
    """Every agent of the tree by folder name, and the folders that failed with why.

    A folder is an agent when it holds an `agent.toml`.
    """
    found: dict[str, Persona] = {}
    problems: dict[str, str] = {}
    for folder in sorted(tree.iterdir()):
        if not (folder / "agent.toml").is_file():
            continue
        try:
            found[folder.name] = read_persona(folder, checkout)
        except ValueError as error:
            problems[folder.name] = str(error)
    return found, problems


# --- Comparing with Multica ---


def omp_runtime(runtimes: list[dict]) -> str:
    """The id of the one runtime with provider omp in local mode."""
    found = [runtime for runtime in runtimes if runtime.get("provider") == "omp" and runtime.get("runtime_mode") == "local"]
    if len(found) == 1:
        return found[0]["id"]
    shown = found or runtimes
    listing = ", ".join(f"{runtime.get('name')} ({runtime.get('provider')}, {runtime.get('runtime_mode')})" for runtime in shown)
    raise ValueError(f"need one local omp runtime, found {len(found)}; runtimes: {listing or 'none'}")


def match_agent(persona: Persona, agents: list[dict], markers: dict[str, str], adopt: bool = False) -> tuple[dict | None, bool]:
    """Match a managed identity, or one explicitly adopted same-name agent."""

    def first(candidates: list[dict]) -> dict | None:
        return min(candidates, key=lambda agent: agent.get("archived_at") is not None, default=None)

    marked = first([agent for agent in agents if markers.get(agent["id"]) == persona.command])
    if marked is not None:
        return marked, False
    unmarked = [agent for agent in agents if agent["id"] not in markers and agent.get("name") == persona.name]
    if adopt and len(unmarked) == 1:
        return unmarked[0], True
    return None, False


def agent_changes(agent: dict, persona: Persona) -> dict[str, object]:
    """The fields the agent differs in, with the persona's values."""
    current = {**agent, "custom_args": agent.get("custom_args") or []}
    return {key: value for key, value in persona.fields().items() if current.get(key) != value}


def is_open(agent: dict) -> bool:
    """Whether the whole workspace sees the agent and may start it."""
    return (
        agent.get("visibility") == OPEN_TO_WORKSPACE["visibility"]
        and agent.get("permission_mode") == OPEN_TO_WORKSPACE["permission_mode"]
        and any(target.get("target_type") == "workspace" for target in agent.get("invocation_targets") or [])
    )


def named_overlays(agents: list[dict]) -> set[Path]:
    """The `--config=` files active agents name. The sync records each write it makes or previews
    on `agents`, so a write run and its preview see the same agents, and a failed update keeps its files."""
    return {
        Path(arg.removeprefix("--config="))
        for agent in agents
        if agent.get("archived_at") is None
        for arg in agent.get("custom_args") or []
        if arg.startswith("--config=")
    }


# --- Multica ---


def load_config(path: Path) -> dict[str, str]:
    try:
        config = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as error:
        raise ValueError(f"cannot read the Multica profile {path}: {error}") from error
    missing = [key for key in ("server_url", "workspace_id", "token") if not config.get(key)]
    if missing:
        raise ValueError(f"the Multica profile {path} has no {', '.join(missing)}")
    return config


class Multica:
    """The Multica API of one workspace."""

    def __init__(self, config: dict[str, str]) -> None:
        self.url = config["server_url"].rstrip("/")
        self.headers = {
            "Authorization": f"Bearer {config['token']}",
            "X-Workspace-ID": config["workspace_id"],
            "Content-Type": "application/json",
        }

    def call(self, method: str, path: str, body: object = None) -> object:
        data = None if body is None else json.dumps(body).encode("utf-8")
        request = urllib.request.Request(self.url + path, data=data, method=method, headers=self.headers)
        try:
            with urllib.request.urlopen(request, timeout=60) as response:
                raw = response.read()
        except urllib.error.HTTPError as error:
            raise ApiError(f"{method} {path}: {error.code} {reason(error.read())}") from error
        except (urllib.error.URLError, OSError) as error:
            raise Unreachable(f"Multica is unreachable at {self.url}: {getattr(error, 'reason', error)}") from error
        return json.loads(raw) if raw else None


def reason(raw: bytes) -> str:
    text = raw.decode("utf-8", errors="replace").strip()
    try:
        return str(json.loads(text).get("error", text))
    except (ValueError, AttributeError):
        return text


class Sync:
    """One run of the sync against Multica."""

    def __init__(self, api: Multica, tree: Path, checkout: Path, *, dry_run: bool = False, adopt: list[str] | None = None) -> None:
        self.api = api
        self.tree = tree
        self.checkout = checkout
        self.failed = False
        self.dry_run = dry_run
        self.adopt = set(adopt or [])

    def write(self, method: str, path: str, body: object = None) -> object:
        if self.dry_run:
            return None
        return self.api.call(method, path, body)

    def say(self, line: str) -> None:
        print(("would " if self.dry_run else "") + line, flush=True)

    def fail(self, line: str) -> None:
        self.failed = True
        print(f"failed: {line}", file=sys.stderr, flush=True)

    def run(self, target: str | None) -> int:
        found, problems = read_personas(self.tree, self.checkout)
        for command, problem in problems.items():
            if target is None or target.lower() == command:
                self.fail(f"persona {command}: {problem}")
        selected = sorted(found.values(), key=lambda persona: persona.command)
        if target is not None:
            selected = [persona for persona in selected if target in (persona.command, persona.name)]
            if not selected:
                self.fail(f"no persona is named {target}")
                return 1
        names = {persona.name for persona in selected}
        for name in self.adopt - names:
            self.fail(f"no selected persona or worker is named {name}")
        if self.failed and self.adopt - names:
            return 1
        runtime = omp_runtime(self.api.call("GET", "/api/runtimes"))
        agents = self.api.call("GET", "/api/agents?include_archived=true")
        envs = self.read_envs(agents)
        markers = {agent_id: env.get(MARKER) or env[WORKER_MARKER] for agent_id, env in envs.items() if env.get(MARKER) or env.get(WORKER_MARKER)}
        for persona in selected:
            self.sync_agent(persona, runtime, agents, markers, envs)
        if target is None:
            self.archive_gone(agents, markers, set(found) | set(problems))
            self.delete_overlays(found, named_overlays(agents))
        return 1 if self.failed else 0

    def read_envs(self, agents: list[dict]) -> dict[str, dict[str, str]]:
        """The env of every agent that has one, by agent id."""
        envs = {}
        for agent in agents:
            if not agent.get("has_custom_env"):
                continue
            try:
                envs[agent["id"]] = self.api.call("GET", f"/api/agents/{agent['id']}/env").get("custom_env") or {}
            except ApiError as error:
                self.fail(f"agent {agent.get('name')}: {error}")
        return envs

    def sync_agent(self, persona: Persona, runtime: str, agents: list[dict], markers: dict[str, str], envs: dict[str, dict[str, str]]) -> None:
        agent, take_over = match_agent(persona, agents, markers, persona.name in self.adopt)
        if agent is None and any(item.get("name") == persona.name and item["id"] not in markers for item in agents):
            self.fail(f"agent {persona.name}: unmanaged name conflict; use --adopt with one unambiguous name")
            return
        if agent is not None and agent.get("has_custom_env") and agent["id"] not in envs:
            return
        if not self.write_overlay(persona):
            return
        try:
            if agent is None:
                if any(item.get("has_custom_env") and item["id"] not in envs for item in agents):
                    self.fail(f"persona {persona.name}: cannot identify an existing agent while an environment is unreadable")
                    return
                agent = self.write("POST", "/api/agents", {**persona.fields(), "runtime_id": runtime, **OPEN_TO_WORKSPACE})
                self.say(f"create agent {persona.name}")
                if agent is not None:
                    self.write("PUT", f"/api/agents/{agent['id']}/env", {"custom_env": {persona.marker: persona.command}})
                self.say(f"update agent {persona.name} (marker)")
            else:
                self.update_agent(agent, persona, runtime, take_over, envs.get(agent["id"], {}))
        except ApiError as error:
            self.fail(f"persona {persona.name}: {error}")

    def update_agent(self, agent: dict, persona: Persona, runtime: str, take_over: bool, env: dict[str, str]) -> None:
        agent_id = agent["id"]
        if agent.get("archived_at") is not None:
            self.write("POST", f"/api/agents/{agent_id}/restore")
            agent["archived_at"] = None
            self.say(f"restore agent {persona.name}")
        changes = []
        if take_over:
            marked = {key: KEEP_ENV for key in env} | {persona.marker: persona.command}
            self.write("PUT", f"/api/agents/{agent_id}/env", {"custom_env": marked})
            self.say(f"adopt agent {persona.name}")
            changes.append("marker")
        fields = agent_changes(agent, persona)
        if agent.get("runtime_id") != runtime:
            fields["runtime_id"] = runtime
        if not is_open(agent):
            fields.update(OPEN_TO_WORKSPACE)
        if fields:
            self.write("PUT", f"/api/agents/{agent_id}", fields)
            agent.update(fields)
            changes.extend(fields)
        if changes:
            self.say(f"update agent {persona.name} ({', '.join(changes)})")

    def archive_gone(self, agents: list[dict], markers: dict[str, str], commands: set[str]) -> None:
        """Archive the active agents of the sync whose persona is gone."""
        for agent in agents:
            command = markers.get(agent["id"])
            if command is None or command in commands or agent.get("archived_at") is not None:
                continue
            try:
                self.write("POST", f"/api/agents/{agent['id']}/archive")
                agent["archived_at"] = datetime.now(timezone.utc).isoformat()
                self.say(f"archive agent {agent.get('name')}")
            except ApiError as error:
                self.fail(f"agent {agent.get('name')}: {error}")

    def write_overlay(self, persona: Persona) -> bool:
        """Write the persona's fallback overlay when it differs. False when it cannot be written."""
        chain = fallback_chain(persona.models)
        if chain is None:
            return True
        path = fallback_overlay(persona.command)
        text = overlay_text(persona.source, chain)
        if path.is_file() and path.read_text(encoding="utf-8") == text:
            return True
        if self.dry_run:
            self.say(f"write overlay {path}")
            return True
        try:
            path.parent.mkdir(parents=True, exist_ok=True)
            staged = path.with_name(f"{path.name}.tmp")
            staged.write_text(text, encoding="utf-8")
            staged.replace(path)
        except OSError as error:
            self.fail(f"persona {persona.name}: cannot write {path}: {error}")
            return False
        self.say(f"write overlay {path}")
        return True

    def delete_overlays(self, found: dict[str, Persona], named: set[Path]) -> None:
        """Delete the overlay files no persona has a chain for and no active agent names."""
        kept = named | {fallback_overlay(command) for command, persona in found.items() if fallback_chain(persona.models)}
        for path in sorted(FALLBACK_DIR.glob("*.yml")):
            if path in kept:
                continue
            if self.dry_run:
                self.say(f"delete overlay {path}")
                continue
            try:
                path.unlink()
                self.say(f"delete overlay {path}")
            except OSError as error:
                self.fail(f"overlay {path}: {error}")


if __name__ == "__main__":
    sys.exit(main())
