"""The Multica API of one workspace, as the local CLI profile names it."""

from __future__ import annotations

import json
import urllib.error
import urllib.request
from pathlib import Path

CONFIG = Path.home() / ".multica" / "profiles" / "local" / "config.json"


class ApiError(Exception):
    """A Multica request failed."""


class Unreachable(ApiError):
    """Multica did not answer."""


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
        self.workspace_id = config["workspace_id"]
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
