## At a glance

```mermaid
flowchart LR
  classDef new fill:#d9f2d9,stroke:#2e7d32,color:#172117
  classDef changed fill:#fff1cc,stroke:#b7791f,color:#172117
  G1[1. Core types, client and query]:::new --> G2[2. Core helpers and drafts]:::changed
  G1 --> G4[4. Runtime browser]:::new
  G2 --> G5[5. Instructions field and surfaces]:::new
  G4 --> G5
  G3[3. Limit warnings]:::changed --> G7[7. Docs and checks]:::changed
  G5 --> G6[6. Failure label and copy]:::changed
  G6 --> G7
```

| Group | Builder | Proves it |
|---|---|---|
| 1. Core types, client and query | `madspec-api-builder` | `pnpm --filter @multica/core test -- schemas directory-listing` |
| 2. Core helpers and drafts | `madspec-service-builder` | `pnpm --filter @multica/core test -- instruction-selection draft stored-draft builder-protocol` |
| 3. Limit warnings | `madspec-view-builder` | `pnpm --filter @multica/views test -- char-counter agent-detail-inspector conversation-starters create-squad use-create-agent-form` |
| 4. Runtime browser | `madspec-view-builder` | `pnpm --filter @multica/views test -- runtime-file-browser` |
| 5. Instructions field and surfaces | `madspec-view-builder` | `pnpm --filter @multica/views test -- instructions-field instructions-tab agent-configuration-panel create-agent-dialog` |
| 6. Failure label and copy | `madspec-view-builder` | `pnpm --filter @multica/views test -- agents-i18n-parity task-failure` |
| 7. Docs and checks | `madspec-view-builder` | `pnpm typecheck`, `pnpm lint`, `pnpm test`, `openspec validate` |

## 1. Core types, client and query

- [x] 1.1 In `packages/core/types/agent.ts`, add `Agent.instruction_selection?: string[]`, `CreateAgentRequest.instruction_selection?: string[]`, `UpdateAgentRequest.instruction_selection?: string[]`, `RuntimeDirectoryEntry` and `RuntimeDirectoryListingRequest` per design Decision 1. Verify: `pnpm typecheck`.
- [x] 1.2 In `packages/core/api/schemas.ts`, add `RuntimeDirectoryListingRequestSchema` with the fallbacks of design Decision 1; in `packages/core/api/client.ts`, add `initiateDirectoryListing(runtimeId, path)` (`POST /api/runtimes/{runtimeId}/directory-listings`) and `getDirectoryListing(runtimeId, requestId)` (`GET .../{requestId}`), both through `parseWithFallback`. Verify: `schemas.test.ts` cases for an unknown status, missing entries and an entry without `kind` or `hidden`.
- [x] 1.3 Create `packages/core/runtimes/directory-listing.ts` (`resolveRuntimeDirectoryListing`, `runtimeDirectoryListingKeys`, `runtimeDirectoryListingOptions`) per design Decision 2 and export it from `packages/core/runtimes/index.ts`. Verify: a `// @vitest-environment node` test with `api` mocked at `@multica/core/api`: pending then completed returns the entries; failed throws the request's error; still pending after 30 s throws a timeout (fake timers).

## 2. Core helpers and drafts

- [x] 2.1 Create `packages/core/agents/instruction-selection.ts` (`SELECTION_PLACEHOLDER`, `appendSelection`, `insertSelectionPlaceholder`, `selectionWarning`) per design Decision 3 and export it from `packages/core/agents/index.ts`. Verify: `instruction-selection.test.ts`: appending keeps order and skips paths already selected; the placeholder goes in at the cursor, at the end after a blank line without a cursor, and alone in empty text; each warning case and `null` when both match.
- [x] 2.2 Add `instructionSelection: string[]` to `AgentDraft` and `EMPTY_AGENT_DRAFT` (`draft.ts`); `buildDuplicateDraft` copies `source.instruction_selection ?? []`; `buildCreateAgentRequest` sends `instruction_selection` when not empty. Verify: `draft.test.ts` cases for a duplicate with two paths and a create request with and without a selection.
- [x] 2.3 In `stored-draft.ts`, write and read `instruction_selection` and compare it in `storedAgentDraftsEqual`; in `schemas.ts`, add `instruction_selection: z.array(z.string()).catch([])` to `StoredAgentDraftSchema`. Verify: `stored-draft.test.ts` round trip with a selection and a stored draft without the field reading `[]`.
- [x] 2.4 In `builder-protocol.ts`, `mergeBuilderDraft` keeps `current.instructionSelection` and ignores `instruction_selection` in the payload. Verify: a test whose payload carries `instruction_selection` leaves the draft's selection as it was.

## 3. Limit warnings

- [ ] 3.1 In `packages/core/agents/constants.ts`, rename to `AGENT_DESCRIPTION_WARN_LENGTH`, `AGENT_CONVERSATION_STARTER_LABEL_WARN_LENGTH`, `AGENT_CONVERSATION_STARTER_WARN_LENGTH` and rewrite the header comment; create `packages/core/squads/constants.ts` with `SQUAD_DESCRIPTION_MAX_LENGTH = 255`, exported from `packages/core/squads/index.ts`; move every caller; remove `isDraftDescriptionWithinLimit` and its callers and tests. Verify: `pnpm typecheck`.
- [ ] 3.2 `CharCounter` gains `soft` per design Decision 8, with `char_counter.over_threshold` in `en/agents.json`. Verify: a test renders 300 of 255 soft (warning tone, the over-threshold text) and hard (destructive tone, today's text).
- [ ] 3.3 Description inputs in `agent-detail-inspector.tsx`, `create/agent-configuration-panel.tsx` and `create-agent-dialog.tsx` drop `maxLength` and use a soft counter; the inspector's save gate drops the length check. `create-squad.tsx` uses `SQUAD_DESCRIPTION_MAX_LENGTH`. Verify: an inspector test types 300 characters, sees all of them and the warning, and saves; a 255-character description shows no warning; `create-squad.test.tsx` still stops at 255.
- [ ] 3.4 `conversation-starters-editor.tsx` drops `maxLength` and shows a soft counter past 80 and 4000; `use-create-agent-form.ts` keeps only the empty label and prompt checks; `builder-protocol.ts` stops slicing label and prompt and keeps the cut to 3. Verify: an editor test with an 81-character label and a 4,001-character prompt keeps every character and warns at both; `builder-protocol.test.ts` with 4 starters and a 120-character label keeps 3 and the label whole; remove tests that assert truncation.

## 4. Runtime browser

- [ ] 4.1 Create `packages/views/agents/components/runtime-file-browser.tsx` per design Decision 5. Verify: `runtime-file-browser.test.tsx` with `api` mocked at `@multica/core/api`: the roots list; opening a folder lists it; a typed path lists it and a missing one shows its error; a hidden entry shows its badge; ticking a file and a folder in two folders then Add calls `onAdd` with both in tick order; an offline runtime shows the offline message, requests nothing and keeps Add disabled.

## 5. Instructions field and surfaces

- [ ] 5.1 Create `packages/views/agents/components/instructions-field.tsx` per design Decision 4. Verify: `instructions-field.test.tsx`: the owner sees Browse and removable chips; another member sees read-only chips and no Browse; the first add inserts `{selection}` at the cursor and a later add only appends; removing a chip calls `onSelectionChange` without it; each warning shows and neither disables anything.
- [ ] 5.2 `tabs/instructions-tab.tsx` takes `runtime` and `currentUserId` from `agent-overview-pane.tsx`, uses `InstructionsField`, includes the selection in `isDirty`, and sends `instruction_selection` only when it changed. Verify: a tab test where the owner adds a path and saves sends it; a member who edits only the text sends no `instruction_selection`.
- [ ] 5.3 `create/agent-configuration-panel.tsx` uses `InstructionsField` on `draft.instructions` and `draft.instructionSelection` with the draft's runtime and `currentUserId`. Verify: a panel test adds a path, sees `{selection}` in the text, and the create request carries the path.
- [ ] 5.4 `create-agent-dialog.tsx` uses `InstructionsField` with a `selection` state seeded from the template; remove `instructions-editor.tsx`. Verify: a dialog test for a duplicate of an agent with a selection sends that selection.

## 6. Failure label and copy

- [ ] 6.1 Map `instruction_selection_error` in `tabs/task-failure.ts` and add its label to each `agents.json`. Verify: a `task-failure` test resolves the key; `agents-i18n-parity.test.ts` passes.
- [ ] 6.2 Add the `instructions.selection`, `runtime_browser` and `char_counter.over_threshold` keys of design Decision 10 to `packages/views/locales/{en,fr,ja,ko,zh-Hans}/agents.json`, following `apps/docs/content/docs/developers/conventions{,.zh}.mdx`. Verify: `agents-i18n-parity.test.ts` passes.

## 7. Docs and checks

- [ ] 7.1 Update `apps/docs/content/docs/agents-create{,.zh,.ko,.ja,.fr}.mdx`: selecting files and folders, `{selection}`, the owner rule, and the warnings instead of the 255/80/4000 limits. Verify: the docs build (`pnpm --filter @multica/docs build`) passes.
- [ ] 7.2 Run `pnpm typecheck`, `pnpm lint` and `pnpm test` from the worktree root, and `openspec validate agent-instruction-selection-field --type change --strict` in `specs/`. Verify: all pass.
- [ ] 7.3 Smoke on the worktree stack in web and in the desktop app: as Brian, open an omp agent's instructions, browse to `~/Repos/Ventures/madspec/skills/as-ux-designer`, add it, see `{selection}` inserted and the chip, save, reload, and see both; as another member, see the chip read-only and no Browse; stop the daemon and see the offline message.

## Implementation Notes

- Starts once groups 1 to 4 of `agent-instruction-selection` are built: the agent API field and the directory listing endpoints must exist.
- Mobile ignores `instruction_selection`; nothing changes there.
- Mock stores with their Zustand callable shape plus `getState`; views tests do not mock `next/*` or `react-router-dom`.
- Group 1 scope: add only the selection wire fields, directory listing response parsing and client calls, and the dedicated polling query with regression tests; preserve other agent/runtime behaviour.
- 1.1: Added optional selection fields and directory entry/listing request types in `packages/core/types/agent.ts`, and exposed the directory types through the existing explicit `packages/core/types/index.ts` barrel. The explicit barrel was an additional necessary callsite. Existing callers need no changes for the optional selection fields. Typecheck is reserved for the main agent's final verification.
- 1.2: Added lenient listing/entry schemas and an explicit failed-response fallback in `packages/core/api/schemas.ts`; added both directory listing calls through `parseWithFallback` in `packages/core/api/client.ts`. `schemas.test.ts` covers unknown status, missing entries, missing/malformed entry markers, malformed strings and non-object responses; `client.test.ts` covers POST/GET paths, request bodies, boundary parsing and fallback request identity. Unknown statuses fail rather than pretending a directory is empty. No additional surprises; tests remain for the main agent to run.
- 1.3: Added `packages/core/runtimes/directory-listing.ts`, exported from `runtimes/index.ts`, with 500 ms polling, `{ path, entries }` results, server-error propagation, runtime/path query keys, `staleTime: 0` and `retry: false`. Kept polling separate from local skills as designed; use `>= 30_000` to stop at the specified boundary rather than the local-skills loop's extra interval. `directory-listing.test.ts` uses the node environment, API-boundary mocks and fake timers for pending/running/completed, immediate roots, failed/timeout, poll failure and the 30-second cutoff, plus query key/cache policy checks. Tests and typecheck remain for the main agent's final verification; no unresolved implementation blockers.
- 2.1: Added pure helpers in `packages/core/agents/instruction-selection.ts` and exported them from `agents/index.ts`. Paths retain exact spelling and selection order; added duplicates are skipped without mutating either input. Cursor insertion adds no spacing; unfocused insertion adds a blank line only for nonempty text. `instruction-selection.test.ts` covers ordered deduplication, cursor start/middle/end and empty text, unfocused insertion and all warning states. No logging is needed for these pure transformations. Tests await the main agent's final verification; no implementation surprises.
- 2.2: Added `AgentDraft.instructionSelection`, its empty value, an independent ordered copy for duplicates, and conditional create-request submission in `packages/core/agents/draft.ts`. `draft.test.ts` covers two-path duplication, missing source selection, populated/empty create payloads and retention on runtime changes. Updated required draft fixtures in `draft.test.ts`, `builder-protocol.test.ts`, `stored-draft.test.ts` and `packages/views/agents/create/agent-execution-overrides.test.tsx`; the views fixture was the additional type migration identified by references. Core draft tests use the required node environment. Existing runtime/model, access and length behaviour is unchanged. Tests await the main agent's final verification.
- 2.3: Added selection serialization, empty-on-absence restoration and ordered selection equality in `agents/stored-draft.ts`; added the specified catch-to-empty schema in `api/schemas.ts`. The existing `StoredAgentDraft` interface in `types/agent.ts` also needed optional `instruction_selection` to represent persisted drafts that omit the field. Equality normalizes absence to `[]`, so restoring a blank saved draft does not appear as an edit. `stored-draft.test.ts` covers ordered round trips, absent and malformed fields, changed/reordered paths, and absence-versus-empty equality. No other stored fields or autosave rules change. Tests await the main agent's final verification.
- 2.4: `agents/builder-protocol.ts` explicitly preserves `current.instructionSelection`; the builder payload contract exposes no selection field. `builder-protocol.test.ts` parses a real structured payload containing `instruction_selection` and verifies that both an empty and an existing two-path selection stay unchanged while permitted fields merge. Starter truncation and other builder behaviour are intentionally left to group 3. No logging or additional payload fields were introduced. Tests await the main agent's final verification; all group 2 behaviours and task commits are complete.

## Plan Change Log

- 2026-10-12: L5 split the work along the roster: this change holds the Frontend team's work from `agent-instruction-selection`, with the specs `runtime-file-browser`, `instruction-selection-field` and the new `agent-field-limit-warnings`.
