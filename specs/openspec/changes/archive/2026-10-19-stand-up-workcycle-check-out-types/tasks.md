## At a glance

Server types and templates, the autopilot issue type, a migration with backfill, and the Issues page by type.

## 1. Types and templates

- [x] 1.1 `server/internal/issuetemplate/templates.go` - add the stand-up, workcycle and check-out templates.
- [x] 1.2 `packages/core/types/issue.ts` - add the three types to `ISSUE_TYPES`.
- [x] 1.3 `packages/views/issues/components/type-icon.tsx`, `packages/ui/styles/tokens.css` and `packages/views/locales/*/issues.json` - give each type its icon, colour, name and hint.

## 2. Autopilot issue type

- [x] 2.1 `server/migrations/803_issue_type_workcycle.up.sql` - widen `issue_type_known`, add `autopilot.issue_type` and type the existing issues and stand-up autopilots.
- [x] 2.2 `server/internal/handler/autopilot.go` and `server/internal/service/autopilot.go` - take, refuse and apply the issue type, with the template in the description.
- [x] 2.3 `server/cmd/multica/cmd_autopilot.go` and `cmd_issue.go` - add `--issue-type` and name the eleven types.

## 3. Issues by type

- [x] 3.1 `server/internal/handler/issue_table_*.go` and `issue.go` - type facet, group and sort.
- [x] 3.2 `packages/core/issues/stores/view-store.ts` and `packages/views/issues/` - Type filter, grouping, sorting and column.
