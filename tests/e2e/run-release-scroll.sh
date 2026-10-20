#!/usr/bin/env bash
set -euo pipefail
RUN=$(mktemp -d /tmp/release-scroll.XXXXXX)
PGBIN=/usr/lib/postgresql/17/bin
BACKEND_PID=
cleanup() {
  if [ -n "$BACKEND_PID" ]; then kill "$BACKEND_PID" 2>/dev/null || true; fi
  if [ -f "$RUN/pg/postmaster.pid" ]; then "$PGBIN/pg_ctl" -D "$RUN/pg" -m immediate -w stop >/dev/null; fi
  rm -rf "$RUN"
}
trap cleanup EXIT
read -r PGPORT BACKPORT < <(python3 -c 'import socket; ss=[socket.socket() for _ in range(2)]; [s.bind(("127.0.0.1",0)) for s in ss]; print(*[s.getsockname()[1] for s in ss])')
export DATABASE_URL="postgres://multica@127.0.0.1:$PGPORT/multica?sslmode=disable"
export PORT="$BACKPORT" APP_ENV=development FRONTEND_ORIGIN="http://localhost:5173"
export NEXT_PUBLIC_API_URL="http://127.0.0.1:$BACKPORT"
export VITE_API_URL="$NEXT_PUBLIC_API_URL" VITE_WS_URL="ws://127.0.0.1:$BACKPORT/ws" VITE_APP_URL="http://localhost:5173"
export PROOF_HOME="$RUN/home" TYPESAFE_API_KEY=
mkdir -p "$PROOF_HOME"
"$PGBIN/initdb" -D "$RUN/pg" -U multica --auth=trust >/dev/null
"$PGBIN/pg_ctl" -D "$RUN/pg" -o "-p $PGPORT -k $RUN -c listen_addresses=127.0.0.1 -c fsync=off" -l "$RUN/pg.log" -w start >/dev/null
"$PGBIN/createdb" -h "$RUN" -p "$PGPORT" -U multica multica
(cd server && go run ./cmd/migrate up && go build -o "$RUN/server" ./cmd/server)
(cd server && exec "$RUN/server") >"$RUN/backend.log" 2>&1 &
BACKEND_PID=$!
node --input-type=module -e 'for (let n=0;n<120;n++) { try { if ((await fetch(process.env.NEXT_PUBLIC_API_URL+"/health")).ok) process.exit(0); } catch {} await new Promise(r=>setTimeout(r,1000)); } process.exit(1);' || { cat "$RUN/backend.log"; exit 1; }
pnpm -C apps/desktop exec electron-vite build
xvfb-run -a -s '-screen 0 1280x800x24' pnpm exec playwright test --config tests/e2e/release-scroll.config.ts
