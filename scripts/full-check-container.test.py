#!/usr/bin/env python3
"""Exercise full-check startup failures with real HTTP servers and PostgreSQL.

Run inside the full-check image: python3 scripts/full-check-container.test.py
Only Go/Node/Python workload commands are replaced with small fixtures.
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
            "uv": '''#!/usr/bin/env bash
touch "$FIXTURE_DIR/madspec-sync"
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
            ["bash", "scripts/full-check-container.sh", str(spec), "madspec-sync/README.md"],
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
            assert "  madspec-sync=0 (" in output, output
            assert "full-check summary (" in output, output
            assert (fixture / "madspec-sync").exists(), output
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


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "serve":
        serve(sys.argv[2])
    else:
        run_case("exit", "healthy", 1, 20)
        # Detect a frontend exit even while the backend is not ready.
        run_case("error", "exit", 1, 20)
        run_case("hang", "healthy", 1, 135)
        run_case("healthy", "error", 1, 135)
        run_case("delayed", "delayed", 0, 20)
