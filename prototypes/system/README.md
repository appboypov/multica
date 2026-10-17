# Multica system design

A zoom map of what Multica's code runs today around an issue's pull requests, in the MadSpec system design template. `system.json` is the one source: every zoom, node, line, table and export comes from it, and `src/lib/system.ts` parses it at load. Each node path is a real repo path; anything that is not code goes in `caption`.

Baseline: `7fc83d223`. Each change that touches these parts updates its zooms from the merged code and commits the exports.

```sh
bun install --cwd prototypes/system
bun run --cwd prototypes/system dev       # http://127.0.0.1:5173/
bun run --cwd prototypes/system build     # static build in build/
bun run --cwd prototypes/system export    # smoke + exports/zoom-NN-<title>.png
```

`export` needs a build and Playwright Chromium (`bunx playwright install chromium`). It serves `build/` in-process, opens every zoom, drills down, goes back, climbs the boundary, and fails on any mismatch, page error or table wider than its column.
