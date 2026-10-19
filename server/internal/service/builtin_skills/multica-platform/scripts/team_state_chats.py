"""team_state's client part: the latest WhatsApp messages of each client chat, through Beeper Desktop."""

from __future__ import annotations

import json
import os
import re
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

from team_state_io import StateError, run


CLIENTS_ROOT = Path.home() / ".omp" / "agent" / "skills" / "das-client-agents" / "clients"
BEEPER = "http://localhost:23373"
MESSAGES_PER_CHAT = 20
CLIENT_FILE_RE = re.compile(r"das-client-agents/clients/([\w.-]+)/AGENTS\.md")
CHAT_ID_RE = re.compile(r"`(![^`\s]+:[^`\s]+)`")


def endeavour_clients(endeavour: Path, named: list[str]) -> list[str]:
    """The clients whose agent file the endeavour's `AGENTS.md` names, then each named one."""
    try:
        text = (endeavour / "AGENTS.md").read_text(encoding="utf-8", errors="replace")
    except OSError:
        text = ""
    return list(dict.fromkeys([*CLIENT_FILE_RE.findall(text), *named]))


def client_chats(clients: list[str], root: Path = CLIENTS_ROOT) -> list[tuple[str, list[str]] | dict]:
    """Each client with the chat IDs its agent file holds; an agent file that cannot be read, with its error."""
    out: list[tuple[str, list[str]] | dict] = []
    for client in clients:
        f = root / client / "AGENTS.md"
        try:
            text = f.read_text(encoding="utf-8")
        except (OSError, UnicodeError) as e:
            out.append({"client": client, "file": str(f), "error": str(e)})
            continue
        out.append((client, list(dict.fromkeys(CHAT_ID_RE.findall(text)))))
    return out


def beeper_get(path: str, token: str) -> dict:
    request = urllib.request.Request(BEEPER + path, headers={"Authorization": f"Bearer {token}"})
    try:
        with urllib.request.urlopen(request, timeout=20) as response:
            return json.load(response)
    except (urllib.error.URLError, OSError, ValueError) as e:
        raise StateError(f"Beeper {path.split('?')[0]}: {e}") from e


def beeper_token() -> str:
    if token := os.environ.get("BEEPER_ACCESS_TOKEN"):
        return token
    try:
        return run(["security", "find-generic-password", "-s", "beeper-access-token", "-w"]).strip()
    except StateError as e:
        raise StateError("no BEEPER_ACCESS_TOKEN and no Keychain item beeper-access-token") from e


def collect_messages(clients: list[str]) -> list[dict]:
    """The latest messages of each chat of the clients, oldest first; a chat that cannot be read, with its error."""
    found = client_chats(clients)
    out = [c for c in found if isinstance(c, dict)]
    chats = [(c[0], chat) for c in found if isinstance(c, tuple) for chat in c[1]]
    if not chats:
        return out
    try:
        token = beeper_token()
    except StateError as e:
        return out + [{"client": client, "chat": chat, "error": str(e)} for client, chat in chats]
    for client, chat in chats:
        path = "/v1/chats/" + urllib.parse.quote(chat, safe="")
        try:
            title = beeper_get(path, token).get("title")
            page = beeper_get(f"{path}/messages?limit={MESSAGES_PER_CHAT}", token)
        except StateError as e:
            out.append({"client": client, "chat": chat, "error": str(e)})
            continue
        messages = [
            {
                "from": m.get("senderName"),
                "ours": m.get("isSender"),
                "at": m.get("timestamp"),
                "type": m.get("type"),
                "text": m.get("text"),
                "attachments": [
                    {k: a.get(k) for k in ("type", "mimeType", "fileName", "isVoiceNote", "srcURL")}
                    for a in m.get("attachments") or []
                ],
            }
            for m in reversed(page.get("items", []))
        ]
        out.append({"client": client, "chat": chat, "title": title, "messages": messages})
    return out
