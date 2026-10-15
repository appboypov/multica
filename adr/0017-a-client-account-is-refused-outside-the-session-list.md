# ADR-0017: A client account is refused outside the session list

- Status: accepted
- Date: 2026-10-15
- Supersedes: none

## Context

WORK-1059 gives each client contact a read-only Multica account bound to one organisation. The first story opens the account before any client data is shown, and the later stories open narrow reads for it. Upstream keeps adding routes, and the fork merges every upstream release. A client must reach none of them unless a story opens it on purpose. Many routes are user-scoped and name no workspace before their handler runs, such as attachment downloads, share-link joins and the inbox, so a per-workspace membership check cannot refuse them all.

## Decision

- A user is a client account when they hold a membership with the role `client`. A client account holds exactly one membership; every write that creates or changes a membership refuses to break that.
- `middleware.Auth` and `middleware.DaemonAuth` wrap their next handler in `clientGate`, so it runs for every credential that resolves to a user and for every route group that mounts them. For a client account the gate serves only the routes in `clientSessionRoutes` (`server/internal/middleware/client_gate.go`) and refuses every other request with 403 before the route's handler runs.
- The workspace WebSocket refuses a client account, and the hub drops a member's connections when a `member:updated` event makes them a client.
- A story opens a route to clients only by adding it to `clientSessionRoutes`; the route then does its own client rules, such as the organisation filter.

## Consequences

- A route added later, by the fork or by upstream, is closed to clients without any change; a router test walks every route to keep it so.
- Each authenticated request runs one indexed `EXISTS` on `member`.
- A person who is staff in one workspace needs a separate account to be a client.
- The client's reads in later stories each need a row in `clientSessionRoutes` and their own organisation check.
