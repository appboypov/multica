# ADR-0037: Repositories live under the work root, by organisation and project

- Status: accepted, supersedes ADR-0006 in part
- Date: 2026-10-22
- Supersedes: ADR-0006 (worktree path template and the project without an organisation)

## Context

ADR-0006 put every new worktree at `~/Worktrees/{organisation}/{repo}/{folder}`, or at `~/Worktrees/{repo}/{folder}` for a project without an organisation, and left main checkouts wherever Brian cloned them, mostly under `~/Repos/<Category>`. The move to HQ (plan `~/hq/plan-hq-verhuizing.md`) gathers all work under one folder in which an agent sees every organisation and project. Paths stay recorded absolutely in the database (ADR-0007), so a move needs a way to rewrite them, and projects need a folder name.

## Decision

- The workspace setting `work_root` in `workspace.settings` names the work root, an absolute path or one written with `~/`. Absent or blank means `~/hq`. The server sends it in every task claim as `work_root`, next to `organisation_key` and `project_slug`.
- A project has a `slug`: lowercase letters, digits and dashes, starting with a letter or digit, unique per organisation (`project_organisation_slug_unique`). Migration 809 gave existing projects the kebab-case of their title, deduplicated per organisation with `-2`, `-3` in creation order.
- A project that holds repositories needs an organisation and a slug. Adding a repository to a project without either is refused with 400 `project_organisation_required` or `project_slug_required`; so is removing either from a project that has repositories.
- Folders, with `{root}` the work root, `{repo}` the part of the GitHub full name after the slash:
  - main checkout suggestion: `{root}/{org}/{project}/repos/{repo}`
  - lane worktree: `{root}/{org}/{project}/worktrees/{repo}/{folder}`
  - issue worktree: `{root}/{org}/{project}/worktrees/{repo}/{issue-key}`
  - pull-request worktree: `{root}/{org}/{project}/worktrees/{repo}/.pull-requests/{branch}`
- The rule exists once, in `server/internal/workpath`, used by the server and the daemon. Paths are stored in the root's style, so `~/hq` gives `~/hq/...`.
- Relocate: `POST /api/repositories/{id}/relocate` and `multica repo relocate <repo> --checkout <path> [--worktree-prefix <old>=<new> ...]` move a repository's records after its folders moved on disk. They refuse while a task of the repository's project or release is dispatched or running. The new checkout passes the same runtime checkout check Add repository uses, which also reports the landing paths of the recorded worktrees that do not exist. The write sets `repository.checkout_path` and rewrites `worklane.path` and `issue_worktree.path` of the repository in every state, `kept` included: a path equal to the old checkout follows the new checkout, any other at or under an old prefix moves under its new prefix (longest prefix wins), compared after `~` expansion and kept in its stored style. Task history (`agent_task_queue.work_dir`, messages, comments) stays as it was.

## Consequences

- A project without an organisation no longer has a worktree folder; it cannot hold repositories.
- Changing `work_root`, an organisation key or a project slug moves nothing already recorded (ADR-0007); only new worktrees use the new folder. Moving existing folders is a disk move followed by relocate.
- The agent brief tells agents that main checkouts live at `~/hq/{organisation}/{project}/repos/{repo}`.
