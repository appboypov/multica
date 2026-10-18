"""The commands `team_state.py` reads its sources through, and the error a failed one raises."""

from __future__ import annotations

import json
import subprocess
from pathlib import Path


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
