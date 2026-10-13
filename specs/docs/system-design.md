# Multica system design

Drawn from the code of the fork at `146d20e5c`.

## Context and containers

```mermaid
C4Container
  title Multica containers

  Person(member, "Workspace member", "Human member of a workspace")

  System_Boundary(product, "Multica") {
    Container(web, "Web app", "Next.js", "apps/web, app/layout.tsx, proxy.ts")
    Container(desktop, "Desktop app", "Electron, electron-vite, react-router-dom", "apps/desktop, src/main/index.ts")
    Container(mobile, "Mobile app", "Expo, expo-router", "apps/mobile, app/_layout.tsx")
    Container(docs, "Docs site", "Next.js, Fumadocs", "apps/docs")
    Container(server, "Go server", "Go, Chi, gorilla/websocket", "server/cmd/server/main.go, HTTP plus WebSocket")
    Container(migrate, "Migrator", "Go", "server/cmd/migrate, server/migrations")
    ContainerDb(pg, "PostgreSQL", "pgx, sqlc", "server/migrations, server/pkg/db")
    ContainerDb(redis, "Redis", "go-redis, optional", "realtime relay, runtime request stores")
    Container(madsync, "madspec-sync", "Python, uv", "madspec-sync/multica_sync.py, multica_projects.py")
  }

  Boundary(machine, "Runtime machine") {
    Container(daemon, "Daemon", "Go, multica CLI", "server/cmd/multica/cmd_daemon.go, server/internal/daemon")
    Container_Ext(clis, "Agent CLIs", "claude, codex, omp, pi, cursor-agent, opencode, copilot, hermes, kimi, qwen, ...", "launched through server/pkg/agent")
  }

  System_Ext(github, "GitHub", "GitHub App webhooks and REST API")
  System_Ext(vcs, "Forgejo, Gitea, GitLab", "VCS webhooks and API")
  System_Ext(chat, "Chat platforms", "Slack, Lark/Feishu, DingTalk, WeCom, Telegram")
  System_Ext(composio, "Composio", "App connections and MCP sessions")
  System_Ext(llmapi, "LLM API", "OpenAI-compatible, server/pkg/llm")
  System_Ext(mail, "Email", "Resend or SMTP")
  System_Ext(google, "Google OAuth", "Login")
  System_Ext(cloud, "Multica Cloud", "Cloud runtime and billing, Stripe webhook forwarded")
  System_Ext(posthog, "PostHog", "Product analytics")
  SystemDb_Ext(s3, "Object storage", "S3-compatible, or local disk")

  Rel(member, web, "Uses", "HTTPS")
  Rel(member, desktop, "Uses")
  Rel(member, mobile, "Uses")
  Rel(member, docs, "Reads", "HTTPS")
  Rel(member, chat, "Talks to agents in")
  Rel(web, server, "REST /api, WS /ws", "proxied by proxy.ts")
  Rel(desktop, server, "REST /api, WS /ws")
  Rel(mobile, server, "REST /api, WS /ws")
  Rel(desktop, daemon, "Spawns and supervises", "src/main/daemon-manager.ts")
  Rel(daemon, server, "REST /api/daemon, WS /api/daemon/ws")
  Rel(daemon, clis, "Launches", "Backend.Execute")
  Rel(clis, server, "Calls back", "multica CLI, MULTICA_TOKEN")
  Rel(madsync, server, "Syncs agents and projects", "REST /api")
  Rel(server, pg, "Reads and writes", "pgxpool")
  Rel(migrate, pg, "Applies migrations")
  Rel(server, redis, "Relays events, stores requests")
  Rel(server, s3, "Stores attachments", "server/internal/storage")
  Rel(github, server, "Webhooks", "/api/webhooks/github")
  Rel(vcs, server, "Webhooks", "/api/webhooks/vcs")
  Rel(server, chat, "Bot APIs, Socket Mode, streams", "server/internal/integrations")
  Rel(server, composio, "Connects apps", "server/pkg/composio")
  Rel(server, llmapi, "Titles, quick actions")
  Rel(server, mail, "Sends login codes, invitations")
  Rel(server, google, "Exchanges OAuth code")
  Rel(server, cloud, "Proxies runtime and billing", "server/internal/cloudruntime")
  Rel(server, posthog, "Captures events", "server/internal/analytics")
```

## Component maps

### Go server

```mermaid
flowchart LR
  main["cmd/server/main.go<br/>main"]
  router["cmd/server/router.go<br/>NewRouterWithOptions, Chi"]
  mw["internal/middleware<br/>Auth, DaemonAuth, workspace gates"]
  authpkg["internal/auth<br/>JWT, PAT, daemon tokens"]
  handlers["internal/handler<br/>Handler, one file per resource"]
  daemonapi["internal/handler<br/>daemon.go, daemon_ws.go<br/>/api/daemon/*"]
  runtimeh["internal/handler<br/>runtime_local_skills.go, runtime_models.go,<br/>runtime_update.go, runtime_directory_listing.go"]
  reqstores["Runtime request stores<br/>UpdateStore, ModelListStore,<br/>LocalSkillListStore, LocalSkillImportStore<br/>InMemory or Redis"]
  dirstore["DirectoryListingStore<br/>InMemory only"]
  liveness["LivenessStore<br/>runtime_liveness_store.go"]
  publicapi["pkg/publicapi/v1<br/>plugin API"]
  services["internal/service<br/>IssueService, TaskService,<br/>AutopilotService, PluginService"]
  sqlc["pkg/db/generated<br/>sqlc Queries"]
  sqlfiles["pkg/db/queries<br/>*.sql"]
  pg[("PostgreSQL")]
  redis[("Redis")]
  bus["internal/events<br/>Bus"]
  listeners["cmd/server<br/>listeners.go, *_listeners.go"]
  hub["internal/realtime<br/>Hub, sharded_stream_relay.go"]
  dhub["internal/daemonws<br/>Hub, notifier.go"]
  sched["internal/scheduler<br/>Manager, jobs_*.go"]
  sweepers["cmd/server<br/>runtime_sweeper.go, source_context_sweeper.go"]
  integ["internal/integrations<br/>channel, slack, lark, dingtalk,<br/>wecom, telegram, vcs, composio, ghsnapshot"]
  storage["internal/storage<br/>local.go, s3.go"]
  llm["pkg/llm<br/>client.go"]
  protocol["pkg/protocol<br/>event and daemon wire types"]
  clients["Web, desktop, mobile"]
  daemon["Daemon"]

  main --> router
  router --> mw --> authpkg
  mw --> handlers
  mw --> daemonapi
  mw --> publicapi
  clients -->|"REST /api"| router
  clients -->|"WS /ws"| hub
  daemon -->|"REST /api/daemon"| router
  daemon -->|"WS /api/daemon/ws"| dhub
  handlers --> services
  handlers --> sqlc
  runtimeh --> reqstores
  daemonapi --> reqstores
  runtimeh --> dirstore
  daemonapi --> dirstore
  daemonapi --> liveness
  daemonapi --> services
  publicapi --> services
  services --> sqlc
  sqlfiles -.->|"make sqlc"| sqlc
  sqlc --> pg
  reqstores --> redis
  handlers --> bus
  services --> bus
  bus --> listeners
  listeners --> hub
  services --> dhub
  hub --> redis
  sched --> services
  sched --> sqlc
  sweepers --> sqlc
  handlers --> integ
  handlers --> storage
  handlers --> llm
  hub --- protocol
  dhub --- protocol
```

### Daemon

```mermaid
flowchart TB
  cmd["cmd/multica/cmd_daemon.go<br/>multica daemon"]
  config["config.go<br/>Config"]
  run["daemon.go<br/>Daemon.Run"]
  probe["agents_probe.go<br/>runtime probe on PATH"]
  client["client.go<br/>Client"]
  hb["daemon.go<br/>heartbeatLoop, runHeartbeatTick"]
  ws["wsrpc.go, wakeup.go<br/>WS heartbeat, pending-work hints"]
  actions["daemon.go<br/>handleHeartbeatActions"]
  models["handleModelList"]
  localskills["local_skills.go<br/>handleLocalSkillList, handleLocalSkillImport"]
  dirlist["directory_listing.go<br/>handleDirectoryListing, listRuntimeDirectory"]
  selection["instruction_selection.go<br/>expandInstructionSelection"]
  update["handleUpdate"]
  poll["daemon.go<br/>pollLoop"]
  task["daemon.go<br/>handleTask, runTask"]
  execenv["execenv<br/>Prepare, Reuse, InjectRuntimeConfig"]
  prompt["prompt.go<br/>BuildPrompt"]
  repocache["repocache<br/>Cache, CreateWorktree"]
  skillcache["skill_cache.go<br/>skill bundles"]
  mcp["runtime_mcp.go, remote_mcp_broker.go"]
  agentpkg["pkg/agent<br/>New, NewRuntime, Backend.Execute"]
  builtin["pkg/agent/builtin_runtimes.go<br/>omp on the pi protocol"]
  backends["pkg/agent<br/>claude.go, codex.go, pi.go, cursor.go,<br/>opencode.go, copilot.go, hermes.go, ..."]
  procs["processtree"]
  gc["gc.go<br/>gcLoop"]
  server["Go server<br/>/api/daemon/*"]

  cmd --> config --> run
  run --> probe
  run --> hb
  run --> ws
  run --> poll
  run --> gc
  hb --> client
  ws --> actions
  hb --> actions
  actions --> models
  actions --> localskills
  actions --> dirlist
  actions --> update
  poll -->|"ClaimTasks"| client
  poll --> task
  task --> selection
  task --> execenv
  task --> prompt
  task --> repocache
  task --> skillcache
  task --> mcp
  task --> agentpkg
  agentpkg --> builtin
  agentpkg --> backends
  backends --> procs
  task -->|"StartTask, ReportTaskMessages, CompleteTask"| client
  models --> client
  localskills --> client
  dirlist --> client
  client --> server
```

### Web and desktop

```mermaid
flowchart TB
  subgraph web["apps/web"]
    webroutes["app/[workspaceSlug]<br/>Next.js App Router"]
    webproviders["components/web-providers.tsx"]
    webnav["platform/navigation.tsx<br/>WebNavigationProvider"]
    proxy["proxy.ts"]
  end
  subgraph desktop["apps/desktop"]
    dmain["src/main/index.ts"]
    dmanager["src/main/daemon-manager.ts"]
    preload["src/preload/index.ts"]
    droutes["src/renderer/src/routes.tsx<br/>react-router"]
    dnav["src/renderer/src/platform/navigation.tsx<br/>DesktopNavigationProvider"]
    dstores["src/renderer/src/stores<br/>tab-store.ts"]
  end
  subgraph views["packages/views"]
    layout["layout/dashboard-layout.tsx"]
    pages["issues, agents, runtimes, chat, skills,<br/>squads, projects, autopilots, inbox, settings"]
    navigation["navigation<br/>NavigationAdapter, app-link.tsx"]
    instrfield["agents/components<br/>instructions-field.tsx InstructionsField,<br/>runtime-file-browser.tsx RuntimeFileBrowser"]
  end
  subgraph core["packages/core"]
    coreprovider["platform/core-provider.tsx<br/>CoreProvider"]
    api["api/client.ts ApiClient<br/>api/schema.ts parseWithFallback"]
    wsclient["api/ws-client.ts<br/>WSClient"]
    features["feature dirs<br/>queries.ts, mutations.ts, ws-updaters.ts"]
    dirlisting["runtimes/directory-listing.ts<br/>runtimeDirectoryListingOptions"]
    realtime["realtime/use-realtime-sync.ts<br/>useRealtimeSync"]
    platform["platform<br/>workspace-storage.ts, persist-storage.ts"]
    qc["query-client.ts<br/>TanStack Query"]
  end
  ui["packages/ui<br/>components, markdown, styles"]
  sdk["packages/plugin-sdk"]
  server["Go server"]
  daemon["Daemon"]

  webroutes --> layout
  webproviders --> coreprovider
  webnav --> navigation
  droutes --> layout
  dnav --> navigation
  dmain --> dmanager --> daemon
  dmain --> preload
  layout --> pages
  pages --> navigation
  pages --> features
  pages --> instrfield
  instrfield --> dirlisting
  dirlisting --> api
  pages --> ui
  features --> api
  features --> qc
  realtime --> wsclient
  realtime --> features
  coreprovider --> api
  coreprovider --> realtime
  coreprovider --> platform
  api --> server
  wsclient --> server
  proxy --> server
  sdk -.-> server
```

### Mobile

```mermaid
flowchart TB
  layout["app/_layout.tsx"]
  wslayout["app/(app)/[workspace]/_layout.tsx"]
  components["components<br/>issue, chat, inbox, project, composer"]
  queries["data/queries, data/mutations"]
  api["data/api.ts<br/>data/schemas.ts"]
  realtime["data/realtime<br/>ws-client.ts, *-ws-updaters.ts"]
  stores["data/stores, data/auth-store.ts,<br/>data/workspace-store.ts"]
  secure["data/secure-storage.ts"]
  coretypes["packages/core<br/>type-only imports"]
  server["Go server"]

  layout --> wslayout --> components
  components --> queries --> api
  components --> stores
  stores --> secure
  wslayout --> realtime --> queries
  api --> server
  realtime --> server
  api -.-> coretypes
```

## Models

```mermaid
classDiagram
  class workspace {
    +uuid id
    +text slug
    +text name
    +text issue_prefix
    +jsonb settings
  }
  class user {
    +uuid id
    +text email
    +text name
  }
  class member {
    +uuid id
    +uuid workspace_id
    +uuid user_id
    +text role
  }
  class agent {
    +uuid id
    +uuid workspace_id
    +uuid runtime_id
    +uuid owner_id
    +text name
    +text instructions
    +jsonb instruction_selection
    +text runtime_mode
    +text visibility
    +jsonb conversation_starters
  }
  class agent_runtime {
    +uuid id
    +uuid workspace_id
    +text daemon_id
    +uuid owner_id
    +uuid profile_id
    +text provider
    +text status
  }
  class runtime_profile {
    +uuid id
    +uuid workspace_id
    +text protocol_family
    +text command_name
  }
  class daemon_token {
    +uuid id
    +uuid workspace_id
    +text daemon_id
    +text token_hash
  }
  class agent_task_queue {
    +uuid id
    +uuid agent_id
    +uuid runtime_id
    +uuid issue_id
    +uuid chat_session_id
    +uuid autopilot_run_id
    +uuid trigger_comment_id
    +uuid squad_id
    +text status
    +text failure_reason
  }
  class task_message {
    +uuid id
    +uuid task_id
    +int seq
    +text type
  }
  class issue {
    +uuid id
    +uuid workspace_id
    +uuid project_id
    +uuid parent_issue_id
    +int number
    +text status
    +text assignee_type
    +uuid assignee_id
  }
  class comment {
    +uuid id
    +uuid issue_id
    +uuid parent_id
    +text author_type
    +uuid author_id
    +uuid source_task_id
  }
  class issue_label {
    +uuid id
    +uuid workspace_id
    +text name
  }
  class issue_to_label {
    +uuid issue_id
    +uuid label_id
  }
  class issue_dependency {
    +uuid id
    +uuid issue_id
    +uuid depends_on_issue_id
    +text type
  }
  class organisation {
    +uuid id
    +uuid workspace_id
    +text name
    +text key
  }
  class project {
    +uuid id
    +uuid workspace_id
    +uuid organisation_id
    +text title
    +text lead_type
    +uuid lead_id
  }
  class project_resource {
    +uuid id
    +uuid project_id
    +text resource_type
    +jsonb resource_ref
  }
  class skill {
    +uuid id
    +uuid workspace_id
    +uuid plugin_installation_id
    +text name
    +text content
  }
  class skill_file {
    +uuid id
    +uuid skill_id
    +text path
  }
  class agent_skill {
    +uuid agent_id
    +uuid skill_id
    +bool enabled
  }
  class squad {
    +uuid id
    +uuid workspace_id
    +uuid leader_id
    +text name
  }
  class squad_member {
    +uuid id
    +uuid squad_id
    +text member_type
    +uuid member_id
  }
  class chat_session {
    +uuid id
    +uuid workspace_id
    +uuid agent_id
    +uuid creator_id
    +uuid runtime_id
    +uuid project_id
  }
  class chat_message {
    +uuid id
    +uuid chat_session_id
    +uuid task_id
    +text role
  }
  class autopilot {
    +uuid id
    +uuid workspace_id
    +uuid project_id
    +text assignee_type
    +uuid assignee_id
    +text execution_mode
  }
  class autopilot_trigger {
    +uuid id
    +uuid autopilot_id
    +text kind
    +text cron_expression
  }
  class autopilot_run {
    +uuid id
    +uuid autopilot_id
    +uuid trigger_id
    +uuid issue_id
    +uuid task_id
    +text status
  }
  class inbox_item {
    +uuid id
    +uuid workspace_id
    +text recipient_type
    +uuid recipient_id
    +uuid issue_id
  }
  class attachment {
    +uuid id
    +uuid workspace_id
    +uuid issue_id
    +uuid comment_id
    +uuid chat_message_id
    +uuid question_id
  }
  class question {
    +uuid id
    +uuid workspace_id
    +uuid asker_agent_id
    +uuid asked_in_task_id
    +text answer_type
    +jsonb settings
    +jsonb suggested_answer
    +text status
    +date due_date
    +bool blocks_work
  }
  class question_link {
    +uuid question_id
    +text link_type
    +uuid link_id
    +int position
  }
  class question_answerer {
    +uuid question_id
    +uuid user_id
  }
  class question_answer {
    +uuid id
    +uuid question_id
    +uuid answerer_user_id
    +jsonb value
    +text note
  }
  class question_event {
    +uuid id
    +uuid question_id
    +text kind
    +text actor_type
    +uuid actor_id
    +jsonb details
  }
  class plugin_installation {
    +uuid id
    +uuid workspace_id
    +text plugin_key
    +bool enabled
  }
  class channel_installation {
    +uuid id
    +uuid workspace_id
    +uuid agent_id
    +text channel_type
  }
  class github_installation {
    +uuid id
    +uuid workspace_id
    +int installation_id
  }
  class vcs_connection {
    +uuid id
    +uuid workspace_id
    +text provider
  }

  workspace "1" --> "*" member : workspace_id
  user "1" --> "*" member : user_id
  workspace "1" --> "*" agent : workspace_id
  workspace "1" --> "*" issue : workspace_id
  workspace "1" --> "*" project : workspace_id
  workspace "1" --> "*" organisation : workspace_id
  organisation "1" --> "*" project : organisation_id
  workspace "1" --> "*" skill : workspace_id
  workspace "1" --> "*" squad : workspace_id
  workspace "1" --> "*" autopilot : workspace_id
  workspace "1" --> "*" agent_runtime : workspace_id
  workspace "1" --> "*" plugin_installation : workspace_id
  workspace "1" --> "*" channel_installation : workspace_id
  workspace "1" --> "*" github_installation : workspace_id
  workspace "1" --> "*" vcs_connection : workspace_id
  user "1" --> "*" agent : owner_id
  user "1" --> "*" agent_runtime : owner_id
  agent_runtime "1" --> "*" agent : runtime_id
  runtime_profile "1" --> "*" agent_runtime : profile_id
  daemon_token .. agent_runtime : daemon_id
  agent "1" --> "*" agent_task_queue : agent_id
  agent_runtime "1" --> "*" agent_task_queue : runtime_id
  issue "1" --> "*" agent_task_queue : issue_id
  chat_session "1" --> "*" agent_task_queue : chat_session_id
  autopilot_run "1" --> "*" agent_task_queue : autopilot_run_id
  comment "1" --> "*" agent_task_queue : trigger_comment_id
  squad "1" --> "*" agent_task_queue : squad_id
  agent_task_queue "1" --> "*" task_message : task_id
  project "1" --> "*" issue : project_id
  issue "1" --> "*" issue : parent_issue_id
  issue ..> user : assignee_id
  issue ..> agent : assignee_id
  issue ..> squad : assignee_id
  issue "1" --> "*" comment : issue_id
  comment "1" --> "*" comment : parent_id
  issue "1" --> "*" issue_to_label : issue_id
  issue_label "1" --> "*" issue_to_label : label_id
  issue "1" --> "*" issue_dependency : issue_id
  project "1" --> "*" project_resource : project_id
  skill "1" --> "*" skill_file : skill_id
  agent "1" --> "*" agent_skill : agent_id
  skill "1" --> "*" agent_skill : skill_id
  plugin_installation "1" --> "*" skill : plugin_installation_id
  agent "1" --> "*" squad : leader_id
  squad "1" --> "*" squad_member : squad_id
  squad_member ..> agent : member_id
  squad_member ..> user : member_id
  agent "1" --> "*" chat_session : agent_id
  user "1" --> "*" chat_session : creator_id
  chat_session "1" --> "*" chat_message : chat_session_id
  chat_message ..> agent_task_queue : task_id
  autopilot ..> agent : assignee_id
  autopilot ..> squad : assignee_id
  autopilot "1" --> "*" autopilot_trigger : autopilot_id
  autopilot "1" --> "*" autopilot_run : autopilot_id
  autopilot_trigger "1" --> "*" autopilot_run : trigger_id
  issue "1" --> "*" inbox_item : issue_id
  issue "1" --> "*" attachment : issue_id
  comment "1" --> "*" attachment : comment_id
  agent "1" --> "*" channel_installation : agent_id
  workspace "1" --> "*" question : workspace_id
  agent "1" --> "*" question : asker_agent_id
  agent_task_queue "1" --> "*" question : asked_in_task_id
  question "1" --> "*" question_link : question_id
  question_link ..> issue : link_id
  question_link ..> agent_task_queue : link_id
  question "1" --> "*" question_answerer : question_id
  user "1" --> "*" question_answerer : user_id
  question "1" --> "*" question_answer : question_id
  user "1" --> "*" question_answer : answerer_user_id
  question "1" --> "*" question_event : question_id
  question "1" --> "*" attachment : question_id
```

## Absent

- No remote CI: `.github/` holds only `ISSUE_TEMPLATE/` and `PULL_REQUEST_TEMPLATE.md`; verification is `make check` in `Makefile`.
- No release feed: `multica update` answers `LocalReleaseUpdateMessage` (`server/internal/cli/update.go`); releases come from `make upstream-sync` and `make local-release`.
- No Redis in the self-host stack (`docker-compose.selfhost.yml`): the realtime hub and runtime request stores run in memory on one node.
- No new foreign keys or cascades (`AGENTS.md`); tables since `server/migrations/173_remove_resource_label_foreign_keys.up.sql` relate by id columns only, while early tables such as those in `001_init.up.sql` keep theirs.
- No direct server access to a runtime machine: the server reaches it only through daemon heartbeat responses and `/api/daemon/ws`.
- No bundled agent CLIs: the daemon finds them on `PATH` or `MULTICA_<ID>_PATH` (`server/internal/daemon/agents_probe.go`, `server/pkg/agent/builtin_runtimes.go`).
- No shared UI on mobile: `apps/mobile` imports `packages/core` types only and never `packages/views` or `packages/ui`.
