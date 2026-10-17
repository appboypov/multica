import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().parents[2] / "server/internal/service/builtin_skills/multica-platform/scripts" / "labels.py"

# A stand-in for `multica label` that keeps the workspace's labels in labels.json beside it.
FAKE_MULTICA = """#!/usr/bin/env python3
import json, sys
from pathlib import Path
state = Path(__file__).with_name("labels.json")
labels = json.loads(state.read_text())
args = sys.argv[1:]
assert args[:2] == ["--profile", "skuddy"] and args[-2:] == ["--output", "json"], args
args = args[2:-2]
if state.with_name("down").exists():
    print("server unreachable", file=sys.stderr)
    raise SystemExit(1)
if args[:2] == ["label", "list"]:
    print(json.dumps(labels))
elif args[:2] == ["label", "create"]:
    label = {"id": f"id-{len(labels)}", "name": args[args.index("--name") + 1], "color": args[args.index("--color") + 1]}
    labels.append(label)
    state.write_text(json.dumps(labels))
    print(json.dumps(label))
else:
    raise SystemExit(f"unexpected call {args}")
"""


class LabelTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.bin = Path(self.tmp.name)
        fake = self.bin / "multica"
        fake.write_text(FAKE_MULTICA.replace("#!/usr/bin/env python3", f"#!{sys.executable}"), encoding="utf-8")
        fake.chmod(0o755)
        self.env = {**os.environ, "PATH": f"{self.bin}{os.pathsep}{os.environ['PATH']}"}

    def tearDown(self):
        self.tmp.cleanup()

    def labels(self, *names):
        labels = [{"id": f"have-{i}", "name": n, "color": "#000000"} for i, n in enumerate(names)]
        (self.bin / "labels.json").write_text(json.dumps(labels), encoding="utf-8")

    def stored(self):
        return [label["name"] for label in json.loads((self.bin / "labels.json").read_text(encoding="utf-8"))]

    def run_labels(self, *args):
        return subprocess.run([sys.executable, str(SCRIPT), *args], capture_output=True, encoding="utf-8",
                              env=self.env, check=False)

    @unittest.skipIf(sys.platform == "win32", "the fake multica is a script with a shebang")
    def test_create_makes_every_missing_label_once(self):
        self.labels("Client wait", "green light: Brian")
        r = self.run_labels("--create")
        self.assertEqual(r.returncode, 0, r.stderr)
        expected = ["risk:low", "risk:medium", "risk:high", "hitl"]
        self.assertEqual(json.loads(r.stdout), {"missing": [], "created": expected})
        self.assertEqual(self.stored(), ["Client wait", "green light: Brian", *expected])
        again = self.run_labels("--create")
        self.assertEqual(json.loads(again.stdout), {"missing": [], "created": []})

    @unittest.skipIf(sys.platform == "win32", "the fake multica is a script with a shebang")
    def test_without_create_it_reports_and_changes_nothing(self):
        self.labels()
        r = self.run_labels()
        self.assertEqual(r.returncode, 0, r.stderr)
        out = json.loads(r.stdout)
        self.assertEqual(out["missing"], ["green light: Brian", "risk:low", "risk:medium", "risk:high", "hitl"])
        self.assertEqual((out["created"], self.stored()), ([], []))

    @unittest.skipIf(sys.platform == "win32", "the fake multica is a script with a shebang")
    def test_a_failing_multica_fails_with_its_message(self):
        self.labels()
        (self.bin / "down").write_text("", encoding="utf-8")
        r = self.run_labels()
        self.assertEqual(r.returncode, 1)
        self.assertIn("server unreachable", json.loads(r.stderr)["error"])


if __name__ == "__main__":
    unittest.main()
