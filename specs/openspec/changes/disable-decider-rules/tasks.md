## At a glance

One decider service change. A database regression owns the disabled-rule behavior.

## 1. Disabled decider rules

- [x] 1.1 `server/internal/service/decider_squad_events_test.go` - cover disabled todo and release squad rules that make no decision.
- [x] 1.2 `server/internal/service/decider_judgment.go` - add the `disabled` flag to `DeciderRule`.
- [x] 1.3 `server/internal/service/decider.go` - skip disabled rules in `Handle`.
- [x] 1.4 `specs/openspec/changes/disable-decider-rules/` - validate the change with `openspec validate disable-decider-rules --type change --strict`.
- [x] 1.5 `server/migrations/802_disable_workspace_squad_rules.up.sql` - retain and disable the two squad rules in Brian's workspace.
- [x] 1.6 `server/internal/service/decider_workspace_migration_test.go` - exercise the delivered migration on both event paths. Preserve other workspaces and the post-merge bug rule.

## Implementation Notes

Red: `go test ./internal/service -run '^TestDeciderWorkspaceSquadCutover$' -count=1 -v` fails because the workspace squad rules remain enabled. This ran through admitted Crabbox on Linux amd64 before the activation SQL was added.

Green: `go test ./internal/service -run Decider -count=1 -v` passes with the delivered migration. The regression proves both squad event paths stop asking in the target workspace. It also proves other workspace settings stay intact and the post-merge bug rule still asks.

`go test ./internal/handler -run Decider -count=1 -v` passes with the existing settings authorization boundary. `go run ./cmd/migrate up`, `go vet ./internal/service`, `go build -o "$run/server-bin" ./cmd/server` and the `gofmt -l` check on the five affected Go files pass.

Target: isolated Docker container `work1779-joeri-unit` on Linux arm64 with Go 1.26.6 and temporary PostgreSQL 17. Image: `sha256:067a5f41a870e45a2f707913b0141a7cc67c71878bbeb7bd57d90c16c39e55fb`. Limits: 2 CPUs and 3 GiB. The container exits and is removed. Direct local execution follows Brian's instruction to skip further Crabbox runs. This evidence covers the activation fixup's pre-commit source tree.

The release applies `802_disable_workspace_squad_rules` to set `"disabled": true` on `todo-squad` and `release-squad` in Brian's workspace. No manual settings write is needed. The settings API keeps its existing authorization.
