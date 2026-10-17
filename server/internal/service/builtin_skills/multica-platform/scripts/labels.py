#!/usr/bin/env python3
# /// script
# requires-python = ">=3.11"
# ///
"""labels - the ticket labels of the Multica workspace.

The labels MadSpec reads on every ticket, as `madspec-issues` `references/operations.md` "fields" says:
`green light: Brian`, `risk:low`, `risk:medium`, `risk:high` and `hitl`. A label is matched by
name among the workspace's issue labels.

  labels.py            report which of these labels the workspace lacks
  labels.py --create   create the missing ones and report them; existing labels are left as they are

It talks to Multica through `multica --profile skuddy label`. Output is one JSON object on stdout:
`{"missing": [...], "created": [...]}`, each a label name. Exit 0 on success, 1 when Multica fails,
with `{"error": ...}` on stderr.
"""

import argparse
import json
import subprocess
import sys

sys.dont_write_bytecode = True

MULTICA = ("multica", "--profile", "skuddy")
LABELS = ("green light: Brian", "risk:low", "risk:medium", "risk:high", "hitl")
COLOR = {"green light": "#22c55e", "risk": "#ef4444", "hitl": "#f97316"}


class LabelError(Exception):
    pass


def multica(*args: str):
    try:
        r = subprocess.run([*MULTICA, *args, "--output", "json"], capture_output=True, encoding="utf-8", check=False)
    except FileNotFoundError:
        raise LabelError("the multica CLI is not on PATH") from None
    if r.returncode != 0:
        raise LabelError(f"multica {' '.join(args)} failed: {(r.stderr or r.stdout).strip()}")
    try:
        return json.loads(r.stdout)
    except json.JSONDecodeError as e:
        raise LabelError(f"multica {' '.join(args)} returned no JSON: {r.stdout.strip()[:200]}") from e


def color(name: str) -> str:
    return next(c for family, c in COLOR.items() if name.startswith(family))


def run(create: bool) -> dict:
    have = {label["name"] for label in multica("label", "list")}
    missing = [name for name in LABELS if name not in have]
    if not create:
        return {"missing": missing, "created": []}
    for name in missing:
        multica("label", "create", "--name", name, "--color", color(name))
    return {"missing": [], "created": missing}


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    p.add_argument("--create", action="store_true", help="create the missing labels")
    args = p.parse_args(argv)
    try:
        print(json.dumps(run(args.create), ensure_ascii=False, indent=2))
    except LabelError as e:
        print(json.dumps({"error": str(e)}), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
