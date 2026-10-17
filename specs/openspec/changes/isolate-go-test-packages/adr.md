# ADR Review Manifest

- Status: completed
- Review date: 2026-10-17

## At a glance

```mermaid
flowchart LR
  classDef new fill:#d9f2d9,stroke:#2e7d32,color:#172117
  Runner[Go suite runner] --> Rule[ADR-0022: one database per test package]:::new
```

| Kind | Name | Change | Path |
|---|---|---|---|
| ADR | Each Go test package runs on its own database | new | `adr/0022-each-go-test-package-runs-on-its-own-database.md` |
| Review | Test database applicability | new | `specs/openspec/changes/isolate-go-test-packages/adr.md` |

## Review Summary

The in-force ADRs in `adr/` (0001-0021) record product decisions: runtime files, questions, organisations, repositories and worklanes, worktrees, issue dates, client access, wakeup input and asks. None of them constrains how the Go test suite reaches its database, so none limits this change.

The change records one lasting decision, because every later DB-backed Go test relies on it: `adr/0022-each-go-test-package-runs-on-its-own-database.md`.
