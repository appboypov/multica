# Projects and repositories

A project groups issues and carries a durable description and GitHub repositories.
Project repositories replace workspace repositories in its task context. The
brief lists their identity and paths under `## Repositories`, and
`.multica/project/repositories.json` carries the same repository context.

- [Core model](#core-model)
- [CLI](#cli)
- [Issue worktrees](#issue-worktrees)
- [Referring to a project in a comment](#referring-to-a-project-in-a-comment)
- [Adding a repository](#adding-a-repository)
- [Debugging wrong context](#debugging-wrong-context)
- [Side effects](#side-effects)

## Core model

Each repository belongs to one project in a workspace. Its GitHub ID, full name,
visibility and default branch are read through a connected GitHub App
installation, never taken from client-supplied metadata. Its checkout is an
absolute path on one daemon, accepted only after a check on that machine.

Every repository has a worklane `main`, which uses the checkout. Other worklanes
have their own daemon and folder. An issue worktree is separate from the normal
checkout and is recorded by `(issue, repository)` with its daemon, path and branch.

A project's `description` is durable context: issue and quick-create tasks bound
to the project receive it under `## Project Context`. Chat tasks receive the
project selected on their chat session. Use the description for project-wide
rules; issue comments do not create project repositories or change the checkout.

## CLI

```bash
multica project list --output json
multica project get <project-id> --output json
multica project create --title "<title>" --output json
multica project create --title "<title>" --start-date 2026-03-01 --due-date 2026-03-31 --output json
multica project update <project-id> --title "<title>" --output json
multica project update <project-id> --due-date 2026-04-15 --output json
multica project update <project-id> --start-date "" --output json
multica project status <project-id> in_progress --output json
multica repo checkout owner/name
```

`--start-date` / `--due-date` are optional calendar days (`YYYY-MM-DD`, like
issue dates). On `project update`, an empty value clears a date; an unset flag
leaves it untouched. Project creation does not accept `--repo`; the old
`multica project resource` commands no longer exist.

## Issue worktrees

For an issue task, the daemon prepares each project repository on this machine
using its recorded worktree path when one exists. Otherwise the path is
`~/Worktrees/{organisation key}/{repo}/{issue key}`, or
`~/Worktrees/{repo}/{issue key}` without an organisation. A later issue key
change does not move the recorded worktree.

An existing worktree is reused without resetting its history. A missing worktree
is made from the fetched default branch, under the issue-key branch, or reuses
that branch when it already exists. A path that is not a worktree of the
checkout fails the task rather than overwriting it.

Tasks sharing a worktree wait for its path lock. With one repository on this
daemon, the agent starts in its worktree; with several it starts in the task
folder and uses the paths in the brief. `multica repo checkout owner/name`
returns the same issue worktree and records it. Work, commit and push there on
its branch, never in the normal checkout. Repositories on another runtime are
listed without a local path. Chat and other non-issue tasks receive checkout
paths without creating issue worktrees.

When an issue becomes Done or Cancelled, its recorded worktrees are queued for
removal. A missing path is removed from the records; a clean worktree with no
unpushed commits is removed along with its branch. Uncommitted changes or
unpushed commits keep the worktree with a reason and one system comment. A held
path lock skips removal. Reopening the issue makes its records active again.
Deleting a project or repository leaves recorded issue worktrees for issue
closure to clean up. Deleting an issue forgets its rows and leaves folders on disk.

## Referring to a project in a comment

A project has no `MUL-123`-style identifier, so writing its title as prose
produces dead text — there is nothing for the reader's client to autolink. Use
the mention-link form instead, with the project UUID from
`multica project list --output json`:

    [Roadmap](mention://project/<project-id>)

Every client makes it navigable, with different presentation: web and desktop
render a chip carrying the project's icon and current title, while mobile
renders an ordinary link that opens the project on tap. Unlike `@agent` /
`@squad`, it is a pure link: the mention parser does not recognize `project` at
all, so it enqueues nothing and notifies nobody — the same no-side-effect
contract as an `issue` mention.

Prefer this form over pasting the project's URL. Web and desktop do unfurl a
bare in-app project URL into that same chip, but mobile does not — there a
pasted URL is handed to the system browser and takes the reader out of the app.

## Adding a repository

Use the project's repository flow when the user asks for durable repository
context. `GET /api/repositories/available` lists repositories visible to the
connected GitHub App and an installation URL. The selected checkout is checked
through `POST /api/runtimes/{runtimeId}/repository-checkouts`
and the returned request is read through the corresponding
`GET .../repository-checkouts/{requestId}`.

Creating a repository through `POST /api/repositories` needs its project ID,
full name, a passed `checkout_check_id` for that repository's current default
branch, version policy and install command. The server takes the daemon and
resolved path from the check. A repository already in another project returns
409 naming that project. Updating through `PUT /api/repositories/{id}` keeps
its checkout without a new check, or moves it using another passed check; its
full name cannot change. Deleting uses `DELETE /api/repositories/{id}`.

## Debugging wrong context

1. Read `multica project get <project-id> --output json`.
2. Read `GET /api/repositories?project_id=<project-id>` in the selected workspace.
3. Check the full name, default branch, checkout daemon and checkout path.
4. Compare the task brief and `.multica/project/repositories.json`: a local
   issue task uses its recorded worktree, a chat uses the checkout, and a
   repository on another runtime has no usable local path.
5. A failed checkout check reports path, Git, origin or branch failures. Fix the
   chosen checkout and run a new check rather than bypassing it.

## Side effects

Project and repository writes mutate durable workspace state and affect future
tasks. Checkout checks read the runtime owner's disk; cloning additionally
creates a checkout. Only the runtime owner can start those operations, and an
offline runtime returns 503. Do not clone, move a checkout or remove a repository
unless the user requested that mutation.
