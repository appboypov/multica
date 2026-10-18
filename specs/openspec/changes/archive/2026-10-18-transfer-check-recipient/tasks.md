## At a glance

One backend service change. Database regressions own the handover behavior.

## 1. Check recipient handover

- [x] 1.1 `server/internal/service/check_recipient_test.go` - reproduce locked ownership and cover record preservation with sampling authority and pending check ownership.
- [x] 1.2 `server/internal/service/check.go` - update only the existing record's recipient during check opening.
- [x] 1.3 `server/internal/service/check_test.go` - remove the superseded different-recipient rejection assertion.
- [x] 1.4 `specs/openspec/changes/transfer-check-recipient/` - validate the completed change with `openspec validate transfer-check-recipient --type change --strict`.

## Implementation Notes

Red: three transfer regressions fail with the existing different-recipient refusal. Nonmember rejection passes. Run `go test ./internal/service -run '^TestCheckRecipientTransfer' -count=1 -v` against an isolated migrated PostgreSQL database.

Green: all four recipient regressions pass. The existing check and instruction service tests pass. Both existing check API tests pass. A throwaway HTTP server smoke opens and answers checks for two members and verifies both answer IDs in history. The colleague can read the new check and the previous recipient receives 404.

Checks pass: `go vet ./internal/service ./internal/handler`; `go build ./cmd/server` with a temporary output path; `gofmt -l internal/service/check_recipient_test.go`; strict OpenSpec validation.

Target: Crabbox `local-container` on Linux ARM64 with Go 1.26.6 and isolated PostgreSQL 17. VPS admission returned busy before starting a payload. Image: `sha256:067a5f41a870e45a2f707913b0141a7cc67c71878bbeb7bd57d90c16c39e55fb`. Resources: 2 CPUs and 3 GiB. Container removal was confirmed after each run. Full merge checks belong to the verifier.
