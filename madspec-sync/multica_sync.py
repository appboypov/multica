#!/usr/bin/env python3
# /// script
# requires-python = ">=3.11"
# ///
"""Sync the MadSpec personas to agents in the local Multica workspace.

The sync runs only when started by hand. It reads the persona list, models and
arguments from `skills/` on `origin/main` in `~/Repos/Ventures/madspec`,
whichever branch or uncommitted edit the checkout holds. A persona agent's
skill content is read from that checkout's disk by its daemon at task start.

Every persona MadSpec's `roster.py --personas` reports is an agent. It runs the
persona as MadSpec's `tools/persona.sh` starts it: its name is the persona's
name, its description `Runs the <skill> skill.`, its instruction selection the
whole persona skill folder in the MadSpec checkout, and its instructions
`{skill-root}` bound to that folder with `{selection}` below. The daemon fills
the selection from disk at every task start, so uncommitted skill edits reach
the next task and Multica stores paths, not a copy of the skill. Its model and
thinking level are the first entry of the `model` list in the persona's
`customize.toml` `[agent]` and its `:level`, and its env
`MADSPEC_PERSONA=<command>`. Its omp
arguments are `--skills=` with the persona's skills, left out for a persona
with every skill, and, when `model` lists more than one entry, `--config=` with
its fallback overlay `~/.local/state/madspec/multica/<command>.yml`:
`retry.modelFallback: true` and a `retry.fallbackChains` entry from the first
model to the other entries in order, each with its `:level`. The persona
reaches its skills through their global links, `~/.agents/skills/<skill>/`.

The sync writes an agent's overlay before it updates the agent, since omp fails
on a missing overlay file; it is derived state, so it lives outside the
MadSpec checkout.

An agent is the sync's own when it carries `MADSPEC_PERSONA` in its env. An
unmarked agent with a persona's name is taken over, an active one before an
archived one. A full sync archives its agents whose persona is gone, restores
them when the persona returns and deletes the overlays no active agent names. A
sync writes only what differs from the source.

With a persona name or command it syncs only that persona, and archives and
deletes nothing.

It reads the server, workspace and token from `~/.multica/profiles/local/config.json`
and binds the agents to the one local omp runtime. It prints one line per write
and per failure, and exits 1 when anything failed or Multica is unreachable.

Usage, from the root of the Multica checkout:
  uv run madspec-sync/multica_sync.py
  uv run madspec-sync/multica_sync.py Mitchel
  uv run madspec-sync/multica_sync.py mitchel
"""

from __future__ import annotations

import argparse
import io
import json
import subprocess
import sys
import tarfile
import tempfile
import urllib.error
import urllib.request
from dataclasses import dataclass
from pathlib import Path

sys.dont_write_bytecode = True

# The MadSpec checkout the sync reads its personas, skills and roster from.
MADSPEC = Path.home() / "Repos" / "Ventures" / "madspec"
sys.path.insert(0, str(MADSPEC / "skills" / "madspec" / "scripts"))

from roster import personas  # noqa: E402

MAIN_REF = "refs/remotes/origin/main"
SOURCES = ("skills",)
CONFIG = Path.home() / ".multica" / "profiles" / "local" / "config.json"
# The fallback overlays the sync writes, one per persona with more than one model: derived state, outside the MadSpec checkout.
FALLBACK_DIR = Path.home() / ".local" / "state" / "madspec" / "multica"
MARKER = "MADSPEC_PERSONA"
# The env value Multica keeps a key's current value for: agent_env.go envSentinel.
KEEP_ENV = "****"
# omp's thinking levels.
THINKING_LEVELS = {"off", "minimal", "low", "medium", "high", "xhigh", "max"}
AGENT_FIELDS = ("name", "description", "instructions", "instruction_selection", "model", "thinking_level", "custom_args")
OPEN_TO_WORKSPACE = {"visibility": "workspace", "permission_mode": "public_to", "invocation_targets": [{"target_type": "workspace"}]}


@dataclass(frozen=True)
class Persona:
    """A Multica agent as its persona skill defines it."""

    command: str
    skill: str
    name: str
    description: str
    instructions: str
    instruction_selection: list[str]
    model: str
    thinking_level: str
    custom_args: list[str]
    models: list[str]

    def fields(self) -> dict[str, object]:
        return {key: getattr(self, key) for key in AGENT_FIELDS}


class ApiError(Exception):
    """A Multica request failed."""


class Unreachable(ApiError):
    """Multica did not answer."""


def main() -> int:
    parser = argparse.ArgumentParser(description="Sync the MadSpec personas to Multica agents.")
    parser.add_argument("persona", nargs="?", help="sync only this persona, by name or command")
    args = parser.parse_args()
    try:
        api = Multica(load_config(CONFIG))
        with tempfile.TemporaryDirectory(prefix="multica-sync-") as tree:
            export_main(MADSPEC, Path(tree))
            return Sync(api, Path(tree), MADSPEC).run(args.persona)
    except (ApiError, ValueError) as error:
        print(f"failed: {error}", file=sys.stderr, flush=True)
        return 1


# --- Main ---


def export_main(repo: Path, into: Path) -> None:
    """Write the sync's sources as `MAIN_REF` holds them into the folder `into`.

    Raises ValueError when git cannot read `MAIN_REF` or the archive cannot be extracted.
    """
    try:
        archive = git(repo, "archive", "--format=tar", MAIN_REF, *SOURCES)
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


# --- Persona skills ---


def split_model(value: str) -> tuple[str, str]:
    """A `model` entry as the model and its thinking level, `""` without a level suffix."""
    model, colon, level = value.rpartition(":")
    if colon and level in THINKING_LEVELS:
        return model, level
    return value, ""


def fallback_chain(models: list[str]) -> dict[str, list[str]] | None:
    """omp's `retry.fallbackChains` of a `model` list: the first model without its level
    to the other entries in order, each with its level; None for one model."""
    if len(models) < 2:
        return None
    return {split_model(models[0])[0]: models[1:]}


def fallback_overlay(command: str) -> Path:
    """The fallback overlay file of the persona started with `command`."""
    return FALLBACK_DIR / f"{command}.yml"


def overlay_text(skill: str, chain: dict[str, list[str]]) -> str:
    """The omp overlay of a fallback chain, as YAML; JSON scalars keep ids with `:` whole."""
    lines = [
        f"# Written by multica_sync.py from skills/{skill}/customize.toml on origin/main; change its `model` instead.",
        "retry:",
        "  modelFallback: true",
        "  fallbackChains:",
    ]
    lines += [f"    {json.dumps(model)}: {json.dumps(selectors)}" for model, selectors in chain.items()]
    return "\n".join(lines) + "\n"


def arguments(skills: list[str] | None, fallback: Path | None) -> list[str]:
    """The omp arguments of a persona's agent, in a fixed order."""
    args = []
    if skills is not None:
        args.append(f"--skills={','.join(skills)}")
    if fallback is not None:
        args.append(f"--config={fallback}")
    return args


def read_persona(found: dict, tree: Path, checkout: Path) -> Persona:
    """The agent of a persona `roster.personas` reports.

    Raises ValueError when the persona has no `model` or no `SKILL.md`.
    """
    skill = found["skill"]
    models = found["models"]
    if not models:
        raise ValueError(f"skills/{skill}/customize.toml has no [agent] model")
    if not (tree / "skills" / skill / "SKILL.md").is_file():
        raise ValueError(f"skills/{skill}/SKILL.md does not exist")
    skill_root = checkout / "skills" / skill
    model, thinking_level = split_model(models[0])
    fallback = fallback_overlay(found["command"]) if fallback_chain(models) else None
    return Persona(
        command=found["command"],
        skill=skill,
        name=found["name"],
        description=f"Runs the {skill} skill.",
        instructions=f"`{{skill-root}}` is {skill_root}. Your instructions are the {skill} skill below; follow it.\n\n"
        "{selection}",
        instruction_selection=[str(skill_root)],
        model=model,
        thinking_level=thinking_level,
        custom_args=arguments(found["skills"], fallback),
        models=models,
    )


def read_personas(tree: Path, checkout: Path) -> tuple[dict[str, Persona], dict[str, str]]:
    """Every persona of the tree's `skills/` by command, and the personas that failed with why."""
    found: dict[str, Persona] = {}
    problems: dict[str, str] = {}
    for persona in personas(tree / "skills"):
        try:
            found[persona["command"]] = read_persona(persona, tree, checkout)
        except ValueError as error:
            problems[persona["command"]] = str(error)
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


def match_agent(persona: Persona, agents: list[dict], markers: dict[str, str]) -> tuple[dict | None, bool]:
    """The Multica agent of a persona, and whether it is unmarked and to be taken over.

    The agent marked with the persona's command comes first, then an unmarked agent with
    the persona's name; an active agent before an archived one.
    """

    def first(candidates: list[dict]) -> dict | None:
        return min(candidates, key=lambda agent: agent.get("archived_at") is not None, default=None)

    marked = first([agent for agent in agents if markers.get(agent["id"]) == persona.command])
    if marked is not None:
        return marked, False
    unmarked = first([agent for agent in agents if agent["id"] not in markers and agent.get("name") == persona.name])
    return unmarked, unmarked is not None


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
    """The `--config=` files active agents still name, including an update that failed."""
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

    def __init__(self, api: Multica, tree: Path, checkout: Path) -> None:
        self.api = api
        self.tree = tree
        self.checkout = checkout
        self.failed = False

    def say(self, line: str) -> None:
        print(line, flush=True)

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
        runtime = omp_runtime(self.api.call("GET", "/api/runtimes"))
        agents = self.api.call("GET", "/api/agents?include_archived=true")
        envs = self.read_envs(agents)
        markers = {agent_id: env[MARKER] for agent_id, env in envs.items() if env.get(MARKER)}
        for persona in selected:
            self.sync_agent(persona, runtime, agents, markers, envs)
        if target is None:
            self.archive_gone(agents, markers, set(found) | set(problems))
            current_agents = self.api.call("GET", "/api/agents?include_archived=true")
            self.delete_overlays(found, named_overlays(current_agents))
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
        agent, take_over = match_agent(persona, agents, markers)
        if agent is not None and agent.get("has_custom_env") and agent["id"] not in envs:
            return
        if not self.write_overlay(persona):
            return
        try:
            if agent is None:
                if any(item.get("has_custom_env") and item["id"] not in envs for item in agents):
                    self.fail(f"persona {persona.name}: cannot identify an existing agent while an environment is unreadable")
                    return
                agent = self.api.call("POST", "/api/agents", {**persona.fields(), "runtime_id": runtime, **OPEN_TO_WORKSPACE})
                self.say(f"create agent {persona.name}")
                self.api.call("PUT", f"/api/agents/{agent['id']}/env", {"custom_env": {MARKER: persona.command}})
                self.say(f"update agent {persona.name} (marker)")
            else:
                self.update_agent(agent, persona, runtime, take_over, envs.get(agent["id"], {}))
        except ApiError as error:
            self.fail(f"persona {persona.name}: {error}")

    def update_agent(self, agent: dict, persona: Persona, runtime: str, take_over: bool, env: dict[str, str]) -> None:
        agent_id = agent["id"]
        if agent.get("archived_at") is not None:
            self.api.call("POST", f"/api/agents/{agent_id}/restore")
            self.say(f"restore agent {persona.name}")
        changes = []
        if take_over:
            marked = {key: KEEP_ENV for key in env} | {MARKER: persona.command}
            self.api.call("PUT", f"/api/agents/{agent_id}/env", {"custom_env": marked})
            changes.append("marker")
        fields = agent_changes(agent, persona)
        if agent.get("runtime_id") != runtime:
            fields["runtime_id"] = runtime
        if not is_open(agent):
            fields.update(OPEN_TO_WORKSPACE)
        if fields:
            self.api.call("PUT", f"/api/agents/{agent_id}", fields)
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
                self.api.call("POST", f"/api/agents/{agent['id']}/archive")
                self.say(f"archive agent {agent.get('name')}")
            except ApiError as error:
                self.fail(f"agent {agent.get('name')}: {error}")

    def write_overlay(self, persona: Persona) -> bool:
        """Write the persona's fallback overlay when it differs. False when it cannot be written."""
        chain = fallback_chain(persona.models)
        if chain is None:
            return True
        path = fallback_overlay(persona.command)
        text = overlay_text(persona.skill, chain)
        if path.is_file() and path.read_text(encoding="utf-8") == text:
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
            try:
                path.unlink()
                self.say(f"delete overlay {path}")
            except OSError as error:
                self.fail(f"overlay {path}: {error}")


if __name__ == "__main__":
    sys.exit(main())
