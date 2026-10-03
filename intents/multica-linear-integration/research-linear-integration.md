# Linear integration research digest

Research date and access date for every source: **2026-10-03**. Scope: public web research for a personal Multica fork; no local code inspected or changed. This is evidence for a PRD, not a decision about which sync modes to implement.

## Evidence convention

Every factual finding links to a source identifier whose URL, publication/update date, and access date are recorded in the source register. **Undated** means the fetched source exposes no trustworthy publication date; it does not mean the feature is new. **[INFERENCE]** marks implications rather than vendor guarantees. **Gap** marks an unresolved question. Current developer documentation takes precedence over historical release announcements. Product documentation and public repository records were read directly; search snippets alone are explicitly identified where used.

## 1. Authentication and integration options

### Personal API keys

- Linear exposes a GraphQL API at `https://api.linear.app/graphql`, supports schema introspection, and supports personal API keys and OAuth2. API keys use `Authorization: <API_KEY>`; OAuth tokens use `Authorization: Bearer <ACCESS_TOKEN>`. Linear recommends API keys for personal scripts and OAuth for applications others use. [L1]
- A key can grant full access to the data its user can access, or restrict permissions to Read, Write, Admin, Create issues, and Create comments. Keys can also be restricted to specified teams. Workspace admins control whether members may create keys and may revoke existing keys. [L2]
- **Gap:** The reviewed API-key guidance does not publish a fixed key lifetime or a refresh-token mechanism. Do not claim that personal keys expire on a particular schedule or that they are guaranteed permanent. OAuth's 24-hour lifetime is not an API-key lifetime. [L1][L2][L3]

### OAuth2 apps and scopes

- The authorization-code flow uses a client ID, redirect URI, `response_type=code`, comma-separated scopes, and an optional but recommended anti-CSRF `state`. Linear also supports PKCE with `plain` or `S256` code challenges. An optional `prompt=consent` can force consent again, including when connecting multiple workspaces. [L3]
- Documented general scopes are `read` (always present), `write`, `issues:create` (new issues and attachments), `comments:create`, `timeSchedule:write`, and `admin`. Linear explicitly discourages requesting `admin` unless absolutely needed, and recommends targeted create scopes where possible. [L3]
- Authorization-code access tokens last **24 hours** and are paired with refresh tokens. Refreshing returns a new access token and a new refresh token. A consumed refresh token can be replayed for **30 minutes** to recover the new token after a network error. All OAuth apps were migrated to the refresh-token system on **2026-04-01**, according to current guidance. [L3]
- The historical announcement says new apps began issuing refresh tokens by default on **2025-10-01** and existing apps had until **2026-04-01** to migrate. This corroborates the current guidance; older claims of indefinitely valid authorization-code access tokens are outdated. The announcement page displays September 18, 2025 despite a September 19 date in its URL. [L4][L3]
- Revocation accepts either access or refresh tokens at `https://api.linear.app/oauth/revoke`. Client secrets and webhook signing secrets can be rotated separately; rotating the client secret immediately invalidates client-credentials tokens, but does not itself revoke workspace access/refresh tokens. [L3]
- **Gap:** The fetched OAuth guidance does not state an absolute or inactivity lifetime for refresh tokens. The documented 30-minute replay grace period is not their overall lifetime. [L3]

### `actor=app`, app users, and agent APIs

- The default actor is the authorizing user. `actor=app` instead installs an app in the workspace and attributes mutations—including issue creation, comments, and status changes—to the app. This is intended for agents and service accounts. App installation requires a workspace admin. Legacy `actor=application` is deprecated and is not equivalent to the new app-user model in all cases. [L5][L6][L7]
- An installed app has a distinct app-user ID in each workspace, discoverable through `viewer { id }`. Admins can change or revoke its team access. `actor=app` apps **cannot request the `admin` scope**. Consequently, do not assume that a workspace-installed app can call admin-only webhook creation APIs merely because an admin installed it. OAuth app-configured webhooks are a separately documented provisioning route. [L6][L8]
- Optional agent scopes are `app:assignable` (delegate on issues/project member) and `app:mentionable` (mentions in editor surfaces). Assigning an agent sets the issue's **`delegate`**, not its human **`assignee`**. Customer access uses `customer:read`/`customer:write`; initiative access uses `initiative:read`/`initiative:write`. Installed agents are not billable users. [L6]
- Linear's Agents API remains labeled **Developer Preview**. Mentions/delegation create Agent Sessions; agents emit activities and their session state is derived from those activities. Enable **Agent session events** in the OAuth app configuration. Linear expects a first `thought` activity within **10 seconds**; after 30 minutes without follow-up activity a session can become stale, recoverable by another activity. [L6][L7]
- Agent activities provide frozen-in-time user-input snapshots, whereas ordinary comments are editable. Linear recommends activities for reconstructing the agent conversation. Additional webhook categories include `AppUserNotification` and `PermissionChange` (`teamAccessChanged`), plus OAuth revocation. [L7]
- Linear's own distinction: use an **integration** for data reads or actions attributed to individual people; use an **agent** when the app should appear as a distinct workspace member. **[INFERENCE]** Connecting Linear and syncing issues does not, by itself, require turning Multica into a native Linear agent. That is a separate experience and permission choice. [L7]

### Client-credentials option

- OAuth apps can opt into `client_credentials` for server-to-server communication. These tokens use an app actor, initially access all public teams, last **30 days**, and have **no refresh token**. Admins can modify team access after issuance. Linear recommends requesting a fresh token per CI/scheduled run, not copying an access token into configuration as a permanent API key. [L3]
- Up to **1,000** client-credentials tokens can coexist with the same scopes. Requesting different scopes revokes/replaces existing app-actor tokens. The guidance imposes additional coexistence restrictions when tokens originate through another grant. **[INFERENCE]** This is a developer-managed alternative, not evidence of a frictionless end-user workspace connection flow. [L3]

## 2. Webhooks and API limits

### Event coverage and delivery

- Data-change webhooks cover **Issues, Issue attachments, Issue comments, Issue labels, Comment reactions, Projects, Project updates, Documents, Initiatives, Initiative Updates, Cycles, Customers, Customer Requests, and Users**. Other documented events include **Issue SLA** and **OAuthApp revoked**. Agent-session, app-user notification, and permission-change categories are documented separately in agent guidance. [L8][L6][L7]
- Webhooks are organization-specific and can cover all public teams or one team. A configured OAuth application's webhook is automatically created when an organization authorizes the app. Only workspace admins or OAuth apps with the `admin` scope can create/read webhook resources through the ordinary webhook API. [L8]
- Payloads include `action` (`create`, `update`, `remove` for data changes), entity `type`, actor, current `data`, time, subject URL, and `updatedFrom` containing previous values of changed fields. Headers include `Linear-Delivery` (payload UUID), `Linear-Event`, `Linear-Signature`, and `Linear-Timestamp`. Actor can be a user, OAuth client, integration, or null. [L8]
- The receiving endpoint must be publicly accessible HTTPS, not localhost, and return **HTTP 200**. Failure includes a non-200 response, unavailability, or taking more than **5 seconds**. Linear retries up to **three times**, at **1 minute, 1 hour, and 6 hours**; a persistently unresponsive webhook might be disabled and require manual re-enablement. [L8]
- Verify a hex-encoded **HMAC-SHA256 of the raw request body** with the signing secret against `Linear-Signature`, preferably with constant-time comparison; do not reserialize parsed JSON. Linear recommends checking body `webhookTimestamp` within **one minute** to guard against replay. Source-IP checking is supplementary. [L8]
- **Gap:** Reviewed guidance does not promise strict ordering, exactly-once delivery, an unlimited recovery history, or a documented event-replay endpoint. **[INFERENCE]** Durable ingestion, deduplication using delivery/entity identity, quick acknowledgement, and reconciliation after a disabled webhook are requirements to consider—not guarantees supplied by Linear. [L8]

### Rate limits

| Authentication | Request quota | Complexity quota | Quota identity |
| --- | ---: | ---: | --- |
| Personal API key | 2,500/hour | 3,000,000 points/hour | User; multiple keys for that user share quota |
| OAuth app | 5,000/hour | 2,000,000 points/hour | User or app user |
| Unauthenticated | 600/hour | 100,000 points/hour | Originating IP |

All figures and identities above are from current rate-limit documentation. Workspace-level actor-authorized OAuth limits can increase dynamically based on paid workspace users; the multiplier/formula is not published. OAuth has a higher base request allowance but a lower base complexity allowance than personal keys. [L9]

- Linear uses leaky-bucket refill, publishes request/complexity remaining/reset headers, caps a single query at **10,000 complexity points**, and may impose lower endpoint-specific limits. Rate-limited GraphQL responses use HTTP **400** with `errors[].extensions.code = RATELIMITED`; do not rely only on HTTP 429. [L9]
- Linear discourages polling, recommends webhooks, server-side filters, explicit pagination, queries fetching only necessary fields, and updated-time ordering when fetching changes. Default connection page size is **50**, which multiplies nested query complexity. [L9][L1]

## 3. Linear's guidance and patterns for linked/two-way integrations

### Links and attachments: the concrete public contract

- An **issue attachment** represents an external resource and renders within a Linear issue similarly to a GitHub PR. `attachmentCreate` accepts `issueId`, external `url`, `title`, optional `subtitle`, `iconUrl`, and metadata. OAuth is recommended so the app icon is the default; API-key integrations can also provide an icon URL. [L10]
- **URL + same issue ID is idempotent:** recreating that attachment updates the original instead of creating another. `attachmentsForURL(url: ...)` returns attachments and their Linear issues, enabling reverse lookup. This guarantee is per issue; it does **not** enforce a single Linear issue for one external URL across a workspace. [L10]
- Attachments support string/number key-value metadata and rich metadata for messages/attributes. Attachments have create/update webhooks. **[INFERENCE]** A stable Multica issue URL is a direct fit for a Linear backlink attachment; Multica still needs an explicit rule for whether one Linear issue may link to several Multica issues. [L10][L8]
- **Important boundary:** An attachment provides identity/navigation/metadata, not automatic bidirectional synchronization of issue fields. The docs show separate issue create/update mutations and attachment create/update mutations. **[INFERENCE]** Field sync, conflict handling, task dispatch, and unlink semantics must be defined by the Multica integration. [L1][L10]

### What “linked” means in native integrations

- Front demonstrates the lightweight linked-resource pattern: create a Linear issue from a customer conversation or link an existing issue; show status/assignee in Front; show the conversation as a link attachment in Linear; reopen the conversation and optionally post an internal note when configured Linear events occur. Multiple conversations may link to one issue. Merging duplicate Linear issues moves Front attachments/comments to the canonical issue and updates the external link. [L11]
- GitLab demonstrates linking without duplicating issue records: match Linear identifiers in branches/MR titles/descriptions, show rich MR attachments in Linear, post a linkback comment in GitLab where write scope allows, and map MR events to team-specific Linear statuses. [L13]
- GitHub/Jira demonstrate actual mirrored issues: each product distinguishes record creation direction, ongoing field updates, linked/synced banners, comments, mappings, and error handling. This is materially different from a simple link attachment. [L12][L14]
- **Gap:** No reviewed developer source defines a universal capitalized **“Linked” integration protocol**, nor does it promise arbitrary third parties can reproduce native GitHub/Jira synced banners simply by creating an attachment. Use “linked” descriptively unless a specific API contract is identified. [L10][L11][L12][L14]

### Conflict and sync guidance: what is and is not supplied

- Linear explicitly recommends webhooks and filtering for near-real-time external displays, with recently updated records first if polling is necessary; it says not to poll every individual issue. [L1][L9]
- Native Jira docs expose real divergence cases: violating Jira workflow constraints changes Linear without changing Jira; unmapped Jira statuses do not update Linear; deleting a synced issue does not delete its counterpart; metadata refresh is **fix forward**, not a repair of already-diverged records. Errors may be surfaced as issue comments or sync banners. [L14]
- **Gap:** The reviewed developer docs do not prescribe a general external two-way conflict algorithm—no normative last-writer-wins rule, per-field version strategy, common-base merge contract, or API optimistic-concurrency recipe was established. The old `developers.linear.app/docs/graphql/syncing` route redirected to the developer landing page, not a usable current syncing specification. [L28][L1][L8][L10]
- **[INFERENCE] PRD decisions required:** per-field source of truth; record creation direction separately from update direction; status/user/label mapping; concurrent edits; echo suppression; import versus continued sync; missing/deleted/archived records; comments visible externally versus private comments; permission revocation; unlink/pause/reconnect behavior; and visible sync error/recovery state. These are implications of the documented native patterns, not extra Linear guarantees. [L10][L12][L14]

## 4. Comparable products

| Product/pattern | Verified behavior | Boundaries / significance |
| --- | --- | --- |
| **GitHub native** | PR/commit linking and status automation; team-to-repository issue sync; one-way creation from GitHub or two-way creation; title, description, status, assignee, labels, sub-issues and selected comments sync; sync banners show status/errors. [L12] | Newly created issues sync; historical issues require import. Multiple repos can feed a team one-way; one repository at a time can be its two-way target. Only comments in the synced thread cross tools, permitting private Linear discussion. Removing the attachment stops syncing. GitHub Project custom statuses do not sync. GitHub Enterprise Server lacks GitHub Issues Sync. [L12] |
| **GitLab native** | MR linking, rich attachments, linkback comments, and configurable status automation, including branch-specific rules. GitLab itself offers an external-issue-tracker integration that recognizes `<TEAM>-<ID>` and links to a configured Linear workspace. [L13][L15] | Linear docs do not offer a built-in GitLab issue importer/sync; they suggest adapting the CLI importer or CSV migration. `read_api` permits reads but suppresses writeback link comments; `api` permits linkbacks. GitLab's URL/reference integration is not issue database synchronization. [L13][L15] |
| **Jira native** | Jira spaces map to Linear teams; epics map to projects. Current uni-directional mode controls **creation** Jira → Linear while updates to linked issues flow both ways; bi-directional mode creates counterparts from either side and updates both ways. [L14] | Legacy mappings from before March 2026 can be Jira → Linear for updates too; switching is irreversible. Existing Jira items can become synced when updated, or through historical import paired with enabled integration. Title, description, assignee, creator, priority, status, labels and due date are listed. Account links and status/label mapping matter. Deletion is not propagated. [L14] |
| **Plane importer** | Admin enters a Linear personal token, selects a Linear team and Plane project, maps states, and imports. Issues become work items; Linear projects become Plane modules; cycles become cycles; comments/attachments/users and backlinks are documented. [P1][P2] | **Rerunning the import pulls new/updated Linear issues**. This is a documented incremental pull model, not evidence of webhook-driven or two-way synchronization. Parent-child relations are imported; the guide does not promise all relation types. Source backlinks survive. [P1][P2] |
| **Notion previews** | Paste a Linear issue/project/view URL as preview or mention. Previews update on reload/manual refresh. Every user authenticates separately; private-team previews are visible only to authorized team members. [N1] | A live preview is not a duplicated editable issue database or two-way sync. The reviewed page documents display updates, not editing Linear issue fields through the preview. [N1] |
| **Notion AI connector** | Workspace integration reads projects, issues and comments, maps users by email, and respects Linear permissions. Requires Linear admin and Notion owner, and Notion Business/Enterprise. [N2] | Initial connection can take 36 hours; updates may take 3 hours to index; permissions sync hourly. Attachments/documents are not ingested. The page inconsistently describes some subsequent updates as immediate elsewhere; its explicit indexing-delay FAQ is safer evidence than a real-time guarantee. [N2] |
| **Height** | Height's official account announced closure on **2025-09-24**, corroborated by a March 28, 2025 independent report that quotes the website announcement. [H1][H2] | Not a current operational comparator. **Gap:** no reliable direct native Height↔Linear sync documentation was established. A search-indexed Zapier listing existed, but its live page returned 404; do not infer supported two-way sync or current availability. [H3] |

### Multica upstream: native integration versus existing custom workflows

- Upstream issue **#7444**, opened **2026-08-23**, explicitly requests fetching issues from GitLab/GitHub/Linear and optionally publishing newly created Multica issues back to the external tool. At access it is **open**, labeled enhancement, **unassigned**, without milestone or comments. This is a user request, not a maintainer delivery commitment. [M1]
- Upstream discussion **#8043**, dated **2026-09-04**, supplies a valuable counterexample to “no Linear integration at all”: a user reports a working **custom** workflow transferring Linear issues—including assignee, description, and comments—to Multica based on criteria, then evolving it into scripts triggered through Multica webhooks/agents. Another workflow has a Multica agent investigate new Linear issues and post a report back to Linear. The request is to trigger bash directly rather than ask an agent to execute a script. No script implementation or native connector is supplied in the discussion. [M2]
- Current README describes agent assignment, source control and chat-platform integrations but does not list a native Linear workspace connector. `VISION.md` describes the future product and explicitly says it is not a feature list; it makes no specific Linear delivery commitment. **Bounded conclusion:** native shipped Linear sync and a committed timetable were **not established** from the reviewed public materials; custom user integrations and public demand are established. Do not translate this into a repository-wide proof that no Linear-related code exists. [M3][M4][M1][M2]
- Checks included issue/PR search for “Linear” (132 broad matches, first 100 returned), a title-restricted query (7 matches, predominantly Linear-style UI/performance work rather than an integration), and the Linear-filtered Discussions page (three returned entries, including #8043). The scoped request and discussion were then fetched directly. **Gap:** no exhaustive source-code audit or private/upcoming roadmap review was performed; public search results cannot prove absence. [M5][M6][M7]

### Agent platforms: Linear as task intake, not necessarily a mirrored tracker

| Platform | Intake and linkage | Scope/caveats |
| --- | --- | --- |
| **Devin** | Connect workspace, select team access; start a session by assigning Devin, adding a synced playbook label such as `!plan`, or mentioning it in comments. Agent activity, plan UI, follow-up messages, stop control, PR link, and Devin-session link are reflected in Linear. [A1] | Automation filters by teams, labels and statuses; triggers use **edge detection**, firing when an issue changes from nonmatching to matching, not for all pre-existing matches. Enterprise team-to-Devin-organization mapping is required. This is task execution and feedback, not documented general two-way tracker mirroring. [A1] |
| **Cursor** | Current terminology is **Cloud Agents** (rather than only “background agents”): delegate an issue or mention `@Cursor`; status appears in Linear and PRs are created when complete; follow-ups route to the running session. [A2] | Repository selection precedence is issue description/comments → issue labels → project labels → default repository. `repo`, `branch`, `model` configuration keys are documented. Connection requires a Cursor admin plus repository/Cloud Agent setup and usage-based pricing. [A2] |
| **Codegen** | Assign/mention agent; create/update issues, post comments, update statuses, and connect PRs/commits. Optional self-assign lets the parent create sub-issues and spawn child agents, with branch context in descriptions. [A3] | Child-agent feature is documented as Team Plan only. API access operates within Linear permissions. This is agent orchestration on issues; no generic database field-conflict sync contract is specified. [A3] |
| **Factory** | Current docs route to **Remote Delegations**, labeled **Private Preview**. Assign Factory or mention `@Factory`; Droid receives title, description, labels and comments, runs remotely, and posts progress/results. It attempts started-status transitions and attaches PR/MR links. [A4] | Organization-level shared execution uses a service-account identity/configuration, not the requester’s personal credentials. A personal Linear connector only gives tools inside sessions and **does not** install the delegate agent. An “Open in Factory” attachment starts a separate context-bearing session, distinct from native agent delegation. [A4] |

**[INFERENCE]** The common agent pattern is selected/filtered Linear issue → execution session → visible progress/results/PR/session backlink. It does not inherently require creating an editable mirrored issue in another project-management database. Multica's requested creation of linked local issues therefore needs its own local-record semantics, even if native Linear delegation is later supported. [A1][A2][A3][A4][M2]

## 5. What “views over Linear issues” means

### Saved Linear custom views

- Linear Custom Views are durable saved filtered collections of **issues, projects, or initiatives**, shareable/favoritable and available at workspace or contextual team/project/initiative scope. Initiative views require Enterprise. Saving a filtered board/list creates a custom view; an ephemeral filtered URL is not necessarily a saved view. Sharing a link does **not** grant access. [V1]
- Filters narrow by teams, assignees, statuses, labels, projects, cycles, dates, relationships and external links. Advanced filters allow nested AND/OR groups; filtering differs from display/grouping preferences. Main filters can appear in URLs, whereas view options, quick filters, and Insights filters are not included. [V2]
- Front gives a concrete external-link view: filter issues by Links → Front. **[INFERENCE]** A “linked to Multica” view and a “matches Linear filter but has no local issue yet” view are different questions from “which Linear team/project/cycle is this in?” [L11][V2]

### Public API `customViews`: schema evidence, not a guessed reimplementation

The official SDK schema was inspected at pinned commit `2a3ac4cfd28af84b98c439efa74b1e32b2ce985b`, published **2026-09-28**. Its generation configuration points to the production GraphQL API and public webhook schema. This independently corroborates the custom-view model beyond the product help page. [V3][V4]

- `Query.customViews(...)` lists custom views accessible to the caller, including personal and shared workspace views, but its description explicitly says it **excludes views scoped to a specific project or initiative**. It accepts connection pagination, `CustomViewFilter`, ordering and archive inclusion. `customView(id: String!)` fetches a single view. Do not promise that one `customViews` query lists every contextual view. [V3] (query lines 39276–39340)
- `CustomView` exposes ID, name, description, owner, creator, shared flag, team, `modelName`, `filterData`, and model-specific filter data. `filters` is legacy and deprecated in favor of `filterData`. Do not parse the legacy JSON format as the primary contract. [V3] (`CustomView` type)
- `CustomView.issues(...)` returns issues **matching the saved issue filter**, supports pagination plus an additional `IssueFilter`, and returns an **empty connection for non-Issue `modelName`**. Its issue `includeSubTeams` default is false. **[INFERENCE]** Fetching membership through this connection is less speculative than reproducing opaque saved-filter JSON locally. [V3]
- `IssueFilter` supports relationship filters for `team`, `project`, nullable `cycle`, labels, assignee, delegate and state, with nested `and` and `or`. Developer documentation shows label/assignee/project relationship filtering and relative dates. [V3][V5] (`IssueFilter` input)

Illustrative API query, **schema-backed but not executed against an authenticated workspace**:

```graphql
query LinearIssueView($id: String!, $after: String) {
  customView(id: $id) {
    id
    name
    modelName
    issues(first: 50, after: $after) {
      nodes { id identifier title url }
      pageInfo { hasNextPage endCursor }
    }
  }
}
```

**Gap:** Authentication-specific visibility of human-owned personal views under `actor=app`, and complete enumeration of project/initiative contextual views, need an authenticated implementation probe. No custom-view data-change webhook is listed in the reviewed webhook entity list, so do not assume a saved view's filter changes emit a standard `CustomView` webhook. [V3][L8]

### Views in comparable tools

- **Native Linear view:** the source-owned saved filter/membership, potentially personalized/shared. [V1][V3]
- **Projection/preview:** Notion displays a live reference to an issue, project or view; no local editable mirrored task is established. [N1]
- **Sync selection:** GitHub team↔repo mappings, Jira space↔team mappings/JQL webhook filters, or Plane team→project import scope define which records cross systems. They are not synonymous with a reusable Linear custom view. [L12][L14][P1]
- **Agent work queue:** Devin team/label/status rules, Cursor issue/project repository labels, and direct delegation select/reroute executable work. They are not a saved-view API implementation. [A1][A2]
- **[INFERENCE] Vocabulary for the PRD:** keep **connection**, **scope/filter**, **saved Linear view**, **read-only projection**, **import**, **linked local issue**, **continuous field sync**, and **agent delegation** distinct. A screen can combine them, but none implies the others. [L10][L12][L14][P1][N1][V1][A1]

## 6. Outstanding prerequisites and claim limits

1. Choose the human-token versus app-user connection semantics; they affect attribution, private-team access and personal-view visibility. The APIs support multiple models, but the product choice is not answered by documentation. [L3][L5][L6][V3]
2. Specify allowed creation/update directions independently; Jira demonstrates that “one-way” can mean one-way **creation** with two-way **updates**. [L14]
3. Define field ownership/conflict and local-issue cardinality; attachments only guarantee URL idempotency on one issue, not global one-to-one mapping. [L10]
4. Decide whether external execution should be a linked Multica issue only, a native Linear delegate app, or both; the Agents API is still preview and does not follow automatically from ordinary OAuth sync. [L6][L7][A4]
5. Live API behavior, production permissions, private-team/view visibility, field mappings and refresh revocation were **not runtime-tested with credentials** in this web-only research. The GraphQL example is not claimed as an exercised scenario. [L1][L3][V3]
6. Height native sync remains unverified; Multica has a proven public custom-workflow report and an open request, not proven native shipping or a committed roadmap date. [H1][H2][H3][M1][M2][M3][M4]

## Source register

All sources below were accessed **2026-10-03**. The publication column records an actual displayed date or repository update when available; otherwise **undated**. “Search only” is weaker evidence and is not used to assert a supported live feature.

| ID | Source URL | Publication / update date | Access date |
| --- | --- | --- | --- |
| L1 | [Linear: GraphQL getting started](https://linear.app/developers/graphql) | Undated current documentation | 2026-10-03 |
| L2 | [Linear: API and Webhooks](https://linear.app/docs/api-and-webhooks) | Undated current documentation | 2026-10-03 |
| L3 | [Linear: OAuth2 authentication](https://linear.app/developers/oauth-2-0-authentication) | Undated; explicitly records migration 2026-04-01 | 2026-10-03 |
| L4 | [Linear: OAuth improvements release](https://linear.app/changelog/2025-09-19-auto-apply-triage-suggestions#oauth-improvements) | Displayed 2025-09-18; URL dated 2025-09-19 | 2026-10-03 |
| L5 | [Linear: OAuth actor authorization](https://linear.app/developers/oauth-actor-authorization) | Undated | 2026-10-03 |
| L6 | [Linear: Agents getting started](https://linear.app/developers/agents) | Undated; Developer Preview label | 2026-10-03 |
| L7 | [Linear: Agent interaction best practices](https://linear.app/developers/agent-best-practices) | Undated | 2026-10-03 |
| L8 | [Linear: Webhooks](https://linear.app/developers/webhooks) | Undated | 2026-10-03 |
| L9 | [Linear: Rate limiting](https://linear.app/developers/rate-limiting) | Undated | 2026-10-03 |
| L10 | [Linear: Attachments](https://linear.app/developers/attachments) | Undated | 2026-10-03 |
| L11 | [Linear: Front integration](https://linear.app/docs/front) | Undated | 2026-10-03 |
| L12 | [Linear: GitHub integration](https://linear.app/docs/github) | Undated | 2026-10-03 |
| L13 | [Linear: GitLab integration](https://linear.app/docs/gitlab) | Undated | 2026-10-03 |
| L14 | [Linear: Jira integration](https://linear.app/docs/jira) | Undated; identifies pre-March-2026 legacy mappings | 2026-10-03 |
| L15 | [GitLab: Linear external issue tracker](https://docs.gitlab.com/user/project/integrations/linear/) | Undated; docs version 19.5; introduced in GitLab 18.3 | 2026-10-03 |
| L28 | [Historical Linear syncing route](https://developers.linear.app/docs/graphql/syncing) | Undated; fetch redirected to developer landing page | 2026-10-03 |
| P1 | [Plane: Linear importer guide](https://docs.plane.so/importers/linear) | Undated | 2026-10-03 |
| P2 | [Plane: Linear marketplace listing](https://plane.so/marketplace/linear) | Undated | 2026-10-03 |
| N1 | [Linear: Notion integration](https://linear.app/integrations/notion) | Undated | 2026-10-03 |
| N2 | [Notion: Linear AI connector](https://www.notion.com/help/notion-ai-connector-for-linear) | Undated | 2026-10-03 |
| H1 | [Height official closure announcement](https://x.com/height_app/status/1903820182557999555) | 2025-03-23; fetched through Nitter mirror by reader | 2026-10-03 |
| H2 | [Creativerly: Height closure report](https://www.creativerly.com/height-app-is-shutting-down/) | 2025-03-28 | 2026-10-03 |
| H3 | [Zapier Height↔Linear listing](https://zapier.com/apps/height/integrations/linear) | Undated search-only historical listing; direct fetch 404 | 2026-10-03 |
| M1 | [Multica issue #7444](https://github.com/multica-ai/multica/issues/7444) | Created/updated 2026-08-23 | 2026-10-03 |
| M2 | [Multica discussion #8043](https://github.com/multica-ai/multica/discussions/8043) | 2026-09-04 | 2026-10-03 |
| M3 | [Multica upstream README](https://github.com/multica-ai/multica) | Undated current default-branch snapshot | 2026-10-03 |
| M4 | [Multica VISION.md](https://github.com/multica-ai/multica/blob/main/VISION.md) | Last file update 2026-09-16, [commit metadata](https://api.github.com/repos/multica-ai/multica/commits?path=VISION.md&per_page=1) | 2026-10-03 |
| M5 | [Upstream broad issue/PR search](https://api.github.com/search/issues?q=repo%3Amultica-ai%2Fmultica%20Linear&per_page=100) | Live search, not publication | 2026-10-03 |
| M6 | [Upstream title-restricted issue/PR search](https://api.github.com/search/issues?q=repo%3Amultica-ai%2Fmultica%20Linear%20in%3Atitle&per_page=100) | Live search, not publication | 2026-10-03 |
| M7 | [Upstream Linear-filtered discussions](https://github.com/multica-ai/multica/discussions?discussions_q=Linear) | Live search, not publication | 2026-10-03 |
| A1 | [Devin: Linear integration](https://docs.devin.ai/integrations/linear) | Undated current documentation | 2026-10-03 |
| A2 | [Cursor: Linear integration](https://cursor.com/docs/integrations/linear) | Undated current documentation | 2026-10-03 |
| A3 | [Codegen: Linear integration](https://docs.codegen.com/integrations/linear) | Undated current documentation | 2026-10-03 |
| A4 | [Factory: Linear remote delegations](https://docs.factory.com/remote-delegations/linear) | Undated; Private Preview; old factory.ai software-factory route redirects here | 2026-10-03 |
| V1 | [Linear: Custom Views](https://linear.app/docs/custom-views) | Undated | 2026-10-03 |
| V2 | [Linear: Filters](https://linear.app/docs/filters) | Undated | 2026-10-03 |
| V3 | [Linear official pinned GraphQL schema](https://github.com/linear/linear/blob/2a3ac4cfd28af84b98c439efa74b1e32b2ce985b/packages/sdk/src/schema.graphql) | 2026-09-28; [commit metadata](https://api.github.com/repos/linear/linear/commits?path=packages/sdk/src/schema.graphql&per_page=1) | 2026-10-03 |
| V4 | [Linear schema generation configuration](https://github.com/linear/linear/blob/master/packages/sdk/codegen.schema.yml) | Undated current default-branch snapshot | 2026-10-03 |
| V5 | [Linear: API filtering](https://linear.app/developers/filtering) | Undated | 2026-10-03 |

### Retrieval limitations

- Height's website failed certificate/socket retrieval. Its official closure post was fetched via the reader's Nitter route and independently checked against a dated report. This verifies the announcement, not a currently working Height product. [H1][H2]
- Apollo Studio schema pages returned only shell metadata through static reading; a browser attempt timed out. The official pinned SDK schema was fetched directly instead. Repository generation configuration corroborates its origin, but this is not an authenticated live workspace API exercise. [V3][V4]
- Remote grep missed later definitions in the large schema; a direct full fetch located the pinned `customView`/`customViews` definitions. A tool issue was reported. Query claims above use that full pinned fetch, not absence from truncated search. [V3]

[L1]: https://linear.app/developers/graphql
[L2]: https://linear.app/docs/api-and-webhooks
[L3]: https://linear.app/developers/oauth-2-0-authentication
[L4]: https://linear.app/changelog/2025-09-19-auto-apply-triage-suggestions#oauth-improvements
[L5]: https://linear.app/developers/oauth-actor-authorization
[L6]: https://linear.app/developers/agents
[L7]: https://linear.app/developers/agent-best-practices
[L8]: https://linear.app/developers/webhooks
[L9]: https://linear.app/developers/rate-limiting
[L10]: https://linear.app/developers/attachments
[L11]: https://linear.app/docs/front
[L12]: https://linear.app/docs/github
[L13]: https://linear.app/docs/gitlab
[L14]: https://linear.app/docs/jira
[L15]: https://docs.gitlab.com/user/project/integrations/linear/
[L28]: https://developers.linear.app/docs/graphql/syncing
[P1]: https://docs.plane.so/importers/linear
[P2]: https://plane.so/marketplace/linear
[N1]: https://linear.app/integrations/notion
[N2]: https://www.notion.com/help/notion-ai-connector-for-linear
[H1]: https://x.com/height_app/status/1903820182557999555
[H2]: https://www.creativerly.com/height-app-is-shutting-down/
[H3]: https://zapier.com/apps/height/integrations/linear
[M1]: https://github.com/multica-ai/multica/issues/7444
[M2]: https://github.com/multica-ai/multica/discussions/8043
[M3]: https://github.com/multica-ai/multica
[M4]: https://github.com/multica-ai/multica/blob/main/VISION.md
[M5]: https://api.github.com/search/issues?q=repo%3Amultica-ai%2Fmultica%20Linear&per_page=100
[M6]: https://api.github.com/search/issues?q=repo%3Amultica-ai%2Fmultica%20Linear%20in%3Atitle&per_page=100
[M7]: https://github.com/multica-ai/multica/discussions?discussions_q=Linear
[A1]: https://docs.devin.ai/integrations/linear
[A2]: https://cursor.com/docs/integrations/linear
[A3]: https://docs.codegen.com/integrations/linear
[A4]: https://docs.factory.com/remote-delegations/linear
[V1]: https://linear.app/docs/custom-views
[V2]: https://linear.app/docs/filters
[V3]: https://github.com/linear/linear/blob/2a3ac4cfd28af84b98c439efa74b1e32b2ce985b/packages/sdk/src/schema.graphql
[V4]: https://github.com/linear/linear/blob/master/packages/sdk/codegen.schema.yml
[V5]: https://linear.app/developers/filtering
