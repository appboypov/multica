"""team_state's reads of the `multica` CLI, git and GitHub, and the error each failed read raises."""

from __future__ import annotations

import json
import subprocess
from pathlib import Path


MULTICA = ["multica", "--profile", "skuddy"]


class StateError(Exception):
    pass


def run(cmd: list[str], cwd: Path | None = None) -> str:
    try:
        done = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True, check=False)
    except OSError as e:
        raise StateError(f"{' '.join(cmd)}: {e}") from e
    if done.returncode != 0:
        where = f" (in {cwd})" if cwd else ""
        raise StateError(f"{' '.join(cmd[:5])}{where} failed: {done.stderr.strip() or done.stdout.strip()}")
    return done.stdout


def run_json(cmd: list[str], cwd: Path | None = None) -> dict:
    data = json.loads(run(cmd, cwd))
    if data.get("errors"):
        raise StateError(f"{' '.join(cmd[:3])}: {data['errors']}")
    return data


def multica(*args: str):
    """One `multica` command's JSON output."""
    out = run([*MULTICA, *args, "--output", "json"])
    try:
        return json.loads(out)
    except ValueError as e:
        raise StateError(f"multica {' '.join(args[:2])}: no JSON: {out[:200]}") from e


def graphql(query: str, **variables: str | None) -> dict:
    cmd = ["gh", "api", "graphql", "-f", f"query={query}"]
    for key, value in variables.items():
        cmd += ["-F", f"{key}=null"] if value is None else ["-f", f"{key}={value}"]
    return run_json(cmd)["data"]
