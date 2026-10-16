## At a glance

| Group | Builder | Outcome |
|---|---|---|
| 1 | madspec-api-builder | Migrated storage, atomic API flows and regression coverage |
| 2 | madspec-command-builder | Repeatable CLI invitation selection and command docs |

## 1. Backend APIs

- [ ] 1.1 `server/migrations/743_client_organisation_sets.up.sql`, matching down migration, `server/pkg/db/queries/member.sql`, `invitation.sql` and generated models - migrate singleton bindings to arrays, remove singular columns, regenerate sqlc and migrate every production caller of changed generated types.
- [ ] 1.2 `server/internal/handler/membership.go`, `workspace.go`, `invitation.go` - implement the published plural-field contract, transactional set writes, deduplication, same-workspace validation, survivor projection and all-deleted acceptance refusal; retain account admission, capacity, onboarding, role permission and revocation behavior.
- [ ] 1.3 `server/internal/handler/*_test.go`, `server/cmd/server/client_gate_test.go` and affected server tests - migrate fixtures and assertions, cover A/B accept, add C/remove A, last removal refusal, partial/all deletion, migration preservation, foreign-workspace rejection, default deny, no-upgrade and demotion. Keep one canonical behavior test per layer. Add a migration regression test where the existing migration tests live.

## 2. Invitation CLI

- [ ] 2.1 `server/cmd/multica/cmd_workspace.go` - make `--organisation` repeatable, resolve all keys/UUIDs once, deduplicate, send `organisation_ids`, and print the full result while retaining staff rejection.
- [ ] 2.2 `server/cmd/multica/cmd_workspace_test.go` - migrate singular invitation cases and cover multiple keys, mixed UUIDs, duplicates, missing/unknown organisations and staff rejection; use fake HTTP servers, not installed agent executables.
- [ ] 2.3 Existing CLI docs under `apps/docs/content/docs` - update the client invite command and JSON payload examples for the plural contract in supported translations; do not change UI copy or unrelated commands.

## Implementation notes

The lead owns integration and task checkboxes. Builders edit their group's source/tests only and report files and behavior; no build, lint, tests, formatter, commits or pushes mid-flight. The lead runs verification once after both groups finish, then records completion and commits. Existing runtime changes to `AGENTS.md` and `.omp/` remain untouched.

## Plan change log

- 2026-10-16: Brian's approved multi-organisation story is planned as `client-organisation-sets`; API contract is in `design.md` before dependent client-read/staff-screen work starts.
