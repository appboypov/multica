#!/usr/bin/env python3
"""Exercise selective unit tests and startup failures with real Vitest and PostgreSQL.

Run inside the full-check image: python3 scripts/full-check-container.test.py
Install workspace dependencies first. Pass "unit" for only file-input selection.
"""

import http.server
import os
from pathlib import Path
import socket
import subprocess
import sys
import tempfile
import time

ROOT = Path(__file__).resolve().parent.parent


def serve(role):
    mode = os.environ[f"{role.upper()}_MODE"]
    print(f"{role} fixture: {mode}", flush=True)
    if mode == "exit":
        sys.exit(23)
    if mode == "delayed":
        time.sleep(2)
    port = int(os.environ["PORT" if role == "backend" else "FRONTEND_PORT"])

    class Handler(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            if mode == "hang":
                time.sleep(300)
                return
            self.send_response(503 if mode == "error" else 200)
            self.end_headers()

        def log_message(self, *_args):
            pass

    server = http.server.ThreadingHTTPServer(("127.0.0.1", port), Handler)
    Path(os.environ["FIXTURE_DIR"], f"{role}.port").write_text(str(port))
    server.serve_forever()


def run_case(backend, frontend, expected, startup_limit):
    with tempfile.TemporaryDirectory(prefix="full-check-startup-test.") as tmp:
        fixture = Path(tmp)
        bin_dir = fixture / "bin"
        bin_dir.mkdir()
        commands = {
            "go": '''#!/usr/bin/env bash
case "$1" in
  env) echo go1.26.6 ;;
  build)
    printf '#!/usr/bin/env bash\nexec python3 "$TEST_FILE" serve backend\n' > "$3"
    chmod +x "$3"
    ;;
esac
''',
            "pnpm": '''#!/usr/bin/env bash
case "$*" in
  dev:web) exec python3 "$TEST_FILE" serve frontend ;;
  "exec playwright "*) touch "$FIXTURE_DIR/playwright" ;;
esac
exit 0
''',
        }
        for name, content in commands.items():
            path = bin_dir / name
            path.write_text(content)
            path.chmod(0o755)
        env = dict(
            os.environ,
            PATH=f"{bin_dir}:{os.environ['PATH']}",
            TEST_FILE=str(Path(__file__).resolve()),
            FIXTURE_DIR=tmp,
            BACKEND_MODE=backend,
            FRONTEND_MODE=frontend,
            TURBO_API="http://127.0.0.1:1",
            TURBO_TOKEN="fixture",
            TURBO_TEAM="fixture",
            BUDGET_SEC="1",
        )
        spec = next((ROOT / "e2e").glob("*.spec.ts")).relative_to(ROOT)
        start = time.monotonic()
        result = subprocess.run(
            ["bash", "scripts/full-check-container.sh", str(spec)],
            cwd=ROOT,
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            timeout=startup_limit + 30,
        )
        elapsed = time.monotonic() - start
        output = result.stdout
        try:
            assert result.returncode == expected, output
            assert elapsed < startup_limit, output
            assert f"  e2e={expected} (" in output, output
            assert "full-check summary (" in output, output
            assert (fixture / "playwright").exists() == (expected == 0), output
            if expected:
                assert "--- backend startup log" in output, output
                assert "--- frontend startup log" in output, output
                assert f"backend fixture: {backend}" in output, output
                assert f"frontend fixture: {frontend}" in output, output
            else:
                # A slow full check still succeeds when all stages succeed.
                assert "slow: over the 1s budget" in output, output
            for port_file in fixture.glob("*.port"):
                port = int(port_file.read_text())
                for _ in range(20):
                    try:
                        connection = socket.create_connection(("127.0.0.1", port), timeout=0.1)
                    except OSError:
                        break
                    connection.close()
                    time.sleep(0.1)
                else:
                    raise AssertionError(f"Server on {port} survived stage cleanup\n{output}")
        except AssertionError:
            print(output, file=sys.stderr)
            raise
        print(f"PASS backend={backend} frontend={frontend}: exit={expected} in {elapsed:.1f}s", flush=True)


def run_file_input_cases():
    """A broken file-read input must fail the selective unit stage."""
    vitest = ROOT / "apps/web/node_modules/vitest/vitest.mjs"
    cases = [
        ("packages/ui/styles/tokens.css", "apps/web/app/text-contrast.test.ts"),
        ("apps/desktop/electron-builder.yml", "apps/desktop/scripts/package.test.mjs"),
    ]
    failures = []
    with tempfile.TemporaryDirectory(prefix="full-check-file-input-test.") as tmp:
        fixture = Path(tmp)
        (fixture / "scripts").mkdir()
        (fixture / "scripts/full-check-container.sh").write_bytes(
            (ROOT / "scripts/full-check-container.sh").read_bytes()
        )
        # These are only hashed by the unrelated Go stage.
        (fixture / "server").mkdir()
        for name in [
            "test-go.sh", "test-go.test.sh", "go-test-with-agent-cli-guard.sh",
            "agent-cli-command-names.txt",
        ]:
            (fixture / "scripts" / name).touch()
        (fixture / "e2e").mkdir()
        for input_path, test_path in cases:
            source = fixture / input_path
            source.parent.mkdir(parents=True, exist_ok=True)
            source.write_text("valid")
            test = fixture / test_path
            test.parent.mkdir(parents=True, exist_ok=True)
            workspace = fixture.joinpath(*Path(test_path).parts[:2])
            (workspace / "package.json").write_text('{"scripts":{"test":"vitest run"}}')
            (workspace / "node_modules").symlink_to(ROOT / "apps/web/node_modules")
            relative_input = os.path.relpath(source, workspace)
            test.write_text(
                'import { test, expect } from "vitest";\n'
                'import { readFileSync } from "node:fs";\n'
                f'test("file input is valid", () => expect(readFileSync("{relative_input}", '
                '"utf8")).toBe("valid"));\n'
            )
        # Copy the real directory reader. The extra JSON has no import edge.
        workspace = fixture / "packages/views"
        locales = workspace / "locales"
        (locales / "en").mkdir(parents=True)
        (workspace / "package.json").write_text('{"scripts":{"test":"vitest run"}}')
        (workspace / "node_modules").symlink_to(ROOT / "apps/web/node_modules")
        (locales / "parity.test.ts").write_bytes(
            (ROOT / "packages/views/locales/parity.test.ts").read_bytes()
        )
        (locales / "index.ts").write_text("export const RESOURCES = {en: {registered: {}}};")
        (locales / "en/registered.json").write_text("{}")
        bin_dir = fixture / "bin"
        bin_dir.mkdir()
        for name, content in {
            "pnpm": '''#!/usr/bin/env bash
if [ "$1 $2 $3" = "exec vitest related" ]; then
  exec node "$VITEST" "${@:3}"
fi
exit 0
''',
            "go": '#!/usr/bin/env bash\necho go1.26.6\n',
        }.items():
            path = bin_dir / name
            path.write_text(content)
            path.chmod(0o755)
        env = dict(
            os.environ,
            PATH=f"{bin_dir}:{os.environ['PATH']}",
            VITEST=str(vitest),
            TURBO_API="http://127.0.0.1:1",
            TURBO_TOKEN="fixture",
            TURBO_TEAM="fixture",
        )
        (fixture / "notes.md").write_text("unrelated")
        scenarios = [
            (path, state, expected)
            for path, _ in cases
            for state, expected in [("valid", 0), ("broken", 1), (None, 1)]
        ] + [
            ("packages/views/locales/en/registered.json", "{}", 0),
            ("packages/views/locales/en/unregistered.json", "{}", 1),
            ("packages/views/locales/en/unregistered.json", None, 0),
            ("notes.md", "unrelated", 0),
        ]
        for changed, state, expected in scenarios:
            source = fixture / changed
            if state is None:
                source.unlink()
            else:
                source.write_text(state)
            result = subprocess.run(
                ["bash", "scripts/full-check-container.sh", changed],
                cwd=fixture, env=env, text=True,
                stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=90,
            )
            if result.returncode != expected or f"  unit={expected} (" not in result.stdout:
                failures.append(f"{changed} ({state}): expected unit exit={expected}\n{result.stdout}")
            else:
                print(f"PASS {changed} ({state}): unit exit={expected}", flush=True)
        assert not failures, "\n".join(failures)


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "serve":
        serve(sys.argv[2])
    elif len(sys.argv) > 1 and sys.argv[1] == "unit":
        run_file_input_cases()
    else:
        run_file_input_cases()
        run_case("exit", "healthy", 1, 20)
        # Detect a frontend exit even while the backend is not ready.
        run_case("error", "exit", 1, 20)
        run_case("hang", "healthy", 1, 135)
        run_case("healthy", "error", 1, 135)
        run_case("delayed", "delayed", 0, 20)
