## ADDED Requirements

### Requirement: The selection is read from disk at every task start
When a task starts and the agent's instructions hold `{selection}`, the daemon on the agent's runtime machine SHALL read every selected path from disk at that moment and replace each `{selection}` with the full selection; the result SHALL be the agent's instructions for that run, both in the brief every runtime reads and in the inline system prompt of the runtimes that take one.

#### Scenario: An edit shows in the next task
- **GIVEN** an agent with instructions `{selection}` and the selected file `/Users/codaveto/notes/a.md` holding `one`
- **AND** a task of that agent that ran with `one`
- **WHEN** `a.md` is saved with `two`, uncommitted, and the next task starts
- **THEN** that task's instructions hold `two` and not `one`

#### Scenario: Text around the placeholder
- **GIVEN** the instructions `Read this first.`, a blank line, then `{selection}`
- **WHEN** a task starts
- **THEN** the task's instructions open with `Read this first.` followed by the selection

#### Scenario: The placeholder written twice
- **GIVEN** instructions that hold `{selection}` twice and one selected file
- **WHEN** a task starts
- **THEN** both places hold the full selection

#### Scenario: A runtime that takes an inline system prompt
- **GIVEN** an agent with a selected file on a runtime whose provider takes the instructions as an inline system prompt
- **WHEN** a task starts
- **THEN** the inline system prompt and the brief both hold the file's content in place of `{selection}`

### Requirement: The selection has one format
The daemon SHALL render `{selection}` as `<selection>`, then one `<tree root="ABSOLUTE PATH">` per selected folder in selection order holding that folder's file tree, then one `<file path="ABSOLUTE PATH">` per file holding its content, in selection order and within a folder by path, then `</selection>`; a selected file SHALL have no tree.

#### Scenario: A skill folder and a file
- **GIVEN** the selection `["/s/as-ux-designer", "/n/b.md"]`, where `/s/as-ux-designer` holds `SKILL.md` and `references/guide.md`
- **WHEN** a task starts
- **THEN** `{selection}` becomes `<selection>`, one `<tree root="/s/as-ux-designer">` listing `as-ux-designer/`, `SKILL.md`, `references/` and `guide.md`, then `<file path="/s/as-ux-designer/SKILL.md">`, `<file path="/s/as-ux-designer/references/guide.md">` and `<file path="/n/b.md">` in that order with their contents, then `</selection>`

#### Scenario: An empty selection
- **GIVEN** instructions that hold `{selection}` and an empty selection
- **WHEN** a task starts
- **THEN** `{selection}` is replaced with nothing and the task runs

#### Scenario: A selected folder with no text files
- **GIVEN** a selected folder that holds only a PNG and `.DS_Store`
- **WHEN** a task starts
- **THEN** the task runs, the folder's tree shows the folder alone, and no file of it follows

### Requirement: A selected folder brings only its text files
A selected folder SHALL bring every file in it, recursively, except hidden files and folders (a name starting with `.`), binary files (a NUL byte in the first 8 KB) and, inside a git repository, the files git ignores (those `git ls-files --cached --others --exclude-standard` does not list); a folder outside a git repository SHALL have no ignore rules.

#### Scenario: Hidden and binary files in a folder
- **GIVEN** a selected folder holding `SKILL.md`, `.env`, `.git/config` and `logo.png`
- **WHEN** a task starts
- **THEN** only `SKILL.md` appears, in the tree and as a file

#### Scenario: A git-ignored file in a repository
- **GIVEN** a selected folder inside a git repository whose `.gitignore` lists `out/`, holding `SKILL.md`, the untracked `notes.md` and `out/build.md`
- **WHEN** a task starts
- **THEN** `SKILL.md` and `notes.md` appear and `out/build.md` does not

#### Scenario: A folder outside any repository
- **GIVEN** a selected folder outside a git repository holding a `.gitignore` that lists `draft.md`, and the file `draft.md`
- **WHEN** a task starts
- **THEN** `draft.md` appears

### Requirement: A path selected by hand is always read
The skip rules SHALL apply only to what a selected folder contains: a path selected by hand SHALL be read even when it is hidden or git-ignored.

#### Scenario: A hidden skill root selected by hand
- **GIVEN** the selected folder `/Users/codaveto/.omp/agent/skills/as-ux-designer`
- **WHEN** a task starts
- **THEN** its text files appear, although `.omp` is a hidden folder

#### Scenario: A hidden file selected by hand
- **GIVEN** the selected file `/Users/codaveto/project/.notes.md`
- **WHEN** a task starts
- **THEN** its content appears

### Requirement: Symbolic links are followed once
The daemon SHALL read a selected link and a linked file inside a selected folder at their targets, and SHALL read a linked folder inside a selected folder only once, so a link loop cannot repeat.

#### Scenario: A selected link
- **GIVEN** the selected path `~/.agents/skills/as-ux-designer`, a link to `/Users/codaveto/Repos/Ventures/madspec/skills/as-ux-designer`
- **WHEN** a task starts
- **THEN** the files of the link's target appear

#### Scenario: A link loop
- **GIVEN** a selected folder `/f` holding `a.md` and the link `/f/loop` pointing to `/f`
- **WHEN** a task starts
- **THEN** the task runs and `a.md` appears once

### Requirement: A file comes in once
A file brought in by more than one selected path SHALL appear once, at its first place in selection order.

#### Scenario: A folder and a file inside it
- **GIVEN** the selection `["/s/skill", "/s/skill/SKILL.md"]`
- **WHEN** a task starts
- **THEN** `<file path="/s/skill/SKILL.md">` appears once, among the folder's files

### Requirement: A selection that cannot be read fails the task
When a selected path is missing or unreadable, or a selected file is binary, the task SHALL fail before the agent starts, with failure reason `instruction_selection_error`, never retried, and an error that names the path and the cause.

#### Scenario: A missing path
- **GIVEN** the selected file `/n/gone.md`, which does not exist
- **WHEN** a task starts
- **THEN** the agent does not start, the task fails with `instruction_selection_error`, its error names `/n/gone.md` and that it does not exist, and no retry is queued

#### Scenario: An unreadable file in a folder
- **GIVEN** a selected folder holding a text file the daemon's user may not read
- **WHEN** a task starts
- **THEN** the task fails with `instruction_selection_error` and its error names that file and the permission cause

#### Scenario: A selected binary file
- **GIVEN** the selected file `/n/logo.png`
- **WHEN** a task starts
- **THEN** the task fails with `instruction_selection_error` and its error names `/n/logo.png` and that it is not a text file

### Requirement: A selection without its placeholder is not read
When the agent's instructions hold no `{selection}`, the daemon MUST NOT read the selection and the task SHALL run with the instructions as written.

#### Scenario: A missing path that is not loaded
- **GIVEN** instructions without `{selection}` and the selected file `/n/gone.md`, which does not exist
- **WHEN** a task starts
- **THEN** the task runs with the instructions as written and does not fail
