## At a glance

The section switch's list becomes exceptions to it, a folder table feeds the library, the daemon reads folder skills at run start, and the Skills page and Skills tab get folders and search.

## 1. Section switch

- [x] 1.1 `server/migrations/804_skill_folder.up.sql` - rename `agent.disabled_runtime_skills` to `runtime_skill_exceptions` and create `skill_folder`.
- [x] 1.2 `server/internal/handler/agent_runtime_skills.go` and `server/pkg/db/queries/agent.sql` - a flip clears the exceptions, and a per-skill change stores an exception to the section state.
- [x] 1.3 `server/internal/daemon/daemon.go` - turn the section state and its exceptions into the skills a run keeps out.

## 2. Skill folders

- [x] 2.1 `server/migrations/805_skill_folder_index.up.sql` and `server/pkg/db/queries/skill_folder.sql` - folder index and queries.
- [x] 2.2 `server/internal/handler/skill_folder.go` - add, list, remove and scan folders, and sync the library from a scan.
- [x] 2.3 `server/internal/daemon/local_skills.go` and `server/internal/daemon/daemon.go` - list folder skills on request and read a folder skill from disk at run start.
- [x] 2.4 `server/internal/service/task.go` and `server/pkg/skillbundle/hash.go` - send a folder skill as a folder reference.

## 3. Web and desktop

- [x] 3.1 `packages/core` - skill folder types, API, query and scan.
- [x] 3.2 `packages/views/skills/components/skill-folders-section.tsx` - the Skill folders section with the folder browser.
- [x] 3.3 `packages/views/skills` - folder skills show their folder as source and are read-only.
- [x] 3.4 `packages/views/agents/components/tabs/skills-tab.tsx` - rows switch both ways and a search field filters both lists.
- [x] 3.5 `packages/views/agents/components/skill-picker-list.tsx` - search in the Add skill dialog.
- [x] 3.6 `packages/views/locales/*` - strings in five languages.
