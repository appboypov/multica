## At a glance

```mermaid
sequenceDiagram
  participant C as Client (web, desktop or madspec-sync)
  participant S as Server
  participant D as Daemon on the runtime machine
  participant A as Agent
  C->>S: list a runtime folder
  S->>D: heartbeat: pending directory listing
  D-->>S: entries (name, path, file or folder, hidden)
  S-->>C: entries
  C->>S: update agent: instructions, instruction_selection
  Note over S,D: a task starts
  S->>D: claim: instructions and instruction_selection
  D->>D: read the paths from disk
  alt a path is missing, unreadable or a selected binary file
    D->>S: fail the task: instruction_selection_error, path and cause
  else every path read
    D->>A: start with {selection} replaced by trees and files
  end
```

```mermaid
flowchart LR
  classDef new fill:#d9f2d9,stroke:#2e7d32,color:#172117
  classDef changed fill:#fff1cc,stroke:#b7791f,color:#172117
  API[Agent API]:::changed --> Agent[(agent.instruction_selection)]:::new
  ListAPI[Runtime directory listing API]:::new --> Heartbeat[Daemon heartbeat]:::changed
  Heartbeat --> Lister[Directory lister in the daemon]:::new
  Agent --> Claim[Task claim]:::changed
  Claim --> Loader[Selection loader in the daemon]:::new
  Loader --> Brief[Brief and inline system prompt]:::changed
  Limits[Description and starter limits]:::changed
  Sync[madspec-sync personas]:::changed --> API
```

| Kind | Name | Change | Path |
|---|---|---|---|
| Database column | `agent.instruction_selection` | new | `server/migrations/564_agent_instruction_selection.up.sql` |
| Database constraint | `agent_description_length` | removed | `server/migrations/565_drop_agent_description_length.up.sql` |
| Queries | agent create and update | changed | `server/pkg/db/queries/agent.sql` |
| API | agent create, update and response carry the selection | changed | `server/internal/handler/agent.go` |
| API | runtime directory listing request and result | new | `server/internal/handler/runtime_directory_listing.go`, `server/cmd/server/router.go` |
| Protocol | pending directory listing on the heartbeat | new | `server/pkg/protocol/messages.go`, `server/internal/handler/daemon.go` |
| Task claim | agent data with the selection | changed | `server/internal/handler/agent.go`, `server/internal/handler/daemon.go`, `server/internal/daemon/types.go` |
| Daemon | directory lister | new | `server/internal/daemon/directory_listing.go` |
| Daemon | selection loader | new | `server/internal/daemon/instruction_selection.go` |
| Daemon | task start fills `{selection}` | changed | `server/internal/daemon/daemon.go` |
| Failure reason | `instruction_selection_error` | new | `server/internal/daemon/daemon.go` |
| Limits | description 255, starter label 80 and prompt 4000 leave the API | changed | `server/internal/handler/agent.go` |
| Docs | the built-in platform skill's agent reference | changed | `server/internal/service/builtin_skills/multica-platform/references/agents.md` |
| Sync | persona agents select their skill folder | changed | `madspec-sync/multica_sync.py`, `madspec-sync/specs/multica-agents.md`, `madspec-sync/README.md`, `madspec-sync/tests/test_multica_sync.py` |

## Why

An agent's instructions are typed text, so a persona agent that should run a MadSpec skill holds only a written link to the skill and depends on the agent reading it, and any other agent holds a copy that goes stale. Brian wants an agent to run with the files on its runtime machine as they are at task start, a whole skill folder included, without a sync or a copy. The length limits on an agent's description and conversation starters get in the way of the same: sizes are his own responsibility.

This change holds the server, daemon and madspec-sync side. The instructions field, the runtime browser and the limit warnings in web and desktop are the change `agent-instruction-selection-field`, which builds on this one.

Success signal: an omp agent whose instructions are `{selection}` with a selected skill folder starts a task, and its brief holds that folder's tree and every text file as they are on disk at that moment; an edit saved to one of those files shows in the next task without any sync.

## What Changes

- An agent gets an ordered list of absolute paths on its runtime machine, `instruction_selection`, beside `instructions`; `{selection}` in the instructions marks where the selection goes. Only the runtime's owner sets or changes it.
- At every task start the daemon reads the selected paths from disk and replaces each `{selection}` with a tree per selected folder and every file in an XML tag; the result is the agent's instructions for that run.
- A missing or unreadable path, or a selected binary file, fails the task before the agent starts with the new failure reason `instruction_selection_error`, never retried.
- A runtime's owner can list its machine's skill roots and any folder through the API; the daemon on that machine does the listing.
- The agent description limit (255) and the conversation starter label (80) and prompt (4000) limits leave the API and the database. At most 3 starters stays.
- madspec-sync gives each persona agent its whole skill folder as selection instead of a written link.

## Capabilities

### New Capabilities

- `agent-instruction-selection`: lets the owner of an agent's runtime keep an ordered list of files and folders on that machine as part of the agent's instructions.
- `instruction-selection-loading`: lets an agent run each task with its selected files as they are on disk at task start, and fails the task when they cannot be read.
- `runtime-directory-listing`: lets the owner of a runtime list its machine's skill roots and folders through the API.
- `agent-field-limits`: lets an agent description and conversation starters of any length through the API.
- `persona-agent-selection`: lets each madspec-sync persona agent run its whole skill folder as it is on disk.

### Modified Capabilities

None: the OpenSpec root is new and holds no specs yet.

## Impact

- Database: one new column on `agent` and one dropped constraint; no index, no foreign key.
- HTTP API: agent create, update and response carry `instruction_selection`; a new runtime directory listing request and its result; description and starter lengths are no longer rejected.
- Daemon protocol: the task claim carries the selection; the heartbeat carries directory listing requests and the daemon posts their results.
- madspec-sync and its spec, README and tests.
- The built-in `multica-platform` skill's agent reference.

## Constraints

- The server never reads a runtime machine's disk and never stores file content; only the daemon on that machine reads the selection.
- Only the owner of the agent's runtime lists that machine and changes an agent's selection.
- No size limit and no size warning on a selection.
- The cap of 3 conversation starters stays.
- AGENTS.md: no foreign keys, no compatibility shims, every check local.

## Non-goals

- Web, desktop and docs: the change `agent-instruction-selection-field`.
- Mobile: it has no instructions editor and ignores the new field.
- The CLI.
- The AI agent builder setting a selection.
- Text limits outside agents.
- A size warning on selections.

## Sources

- Intent: [Pick a local skill as an agent's live system prompt](anytype://object?objectId=bafyreih4afdxypfryync3lzilucl3qwjuvprpnlyypmkrm7iejq52u2dgi) and its Requests.
- Approach, passed by Brian: [Select local files and folders as an agent's instructions](anytype://object?objectId=bafyreib2hcvw46rajqfcbw2f5lpntjsemt3s62lvgcep6zqzoaqudgs5hu); its "Per file or system" section holds the code map for the design.
- Decisions: [The picker is a runtime browser](anytype://object?objectId=bafyreidkzrr6hpp46ry6rvrk2jyympdb4gydn4rbzdfo34e3dzuvuodlvi), [Only the runtime's owner browses and changes a selection](anytype://object?objectId=bafyreicdmlkimhz2z6dgyjydsuxfuc7hvzouiy6ignfgqbn5pzj5z2falu), [A path selected by hand is always read; a selected binary file fails](anytype://object?objectId=bafyreiddodnsgdj66vvavcxfva2y4quzr2q5n4j6cla6r5troe32hylg64), [An empty selected folder runs with an empty tree](anytype://object?objectId=bafyreidge2jqeeqs523jcq2ls46sq2g3fzmtjetps7vrciiivbzpezaw5y), [A file brought in twice comes in once](anytype://object?objectId=bafyreifkvsdzv3ttsyajrrbrqxllmos3rmy4omi26n6i3dgbtlodkv4hci), [Every {selection} gets the full selection](anytype://object?objectId=bafyreifmmh5whedozbywayxyxvcx4yntaj7spgkk2olglp4gezpv4vcosu), [Agents don't change runtimes; no selection rule for it](anytype://object?objectId=bafyreidbtlba4b5j2xcot2lhex4xdkrbbau57kme2ypznppcvwbo2udeg4), [The picker adds nothing while the runtime is offline](anytype://object?objectId=bafyreihmoqyjk77xfrcws6k3ua3jslvfsfdp6s5ian6lcvn2mhlcq67vxq), [The browser lists hidden entries, marked as hidden](anytype://object?objectId=bafyreia5bsfx66xf63mwzz4t54lgoxbwsrsmppapvvvv2cv3e7mp2drcca), [The AI agent builder never sets a selection](anytype://object?objectId=bafyreibfexxb4lejzoku6o3nkl72f4cur2ig2sd7e56awpcr62sdzbb5cy).
- madspec-sync spec this change alters: `madspec-sync/specs/multica-agents.md`.
