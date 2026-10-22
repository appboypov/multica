# ADR-0006: A repository belongs to a project and holds its worklanes

- Status: accepted, supersedes ADR-0004; superseded by ADR-0037
- Date: 2026-10-13
- Supersedes: ADR-0004

## Context

ADR-0004 made the organisation's key the first folder of every worktree path, and left a project without an organisation without a worktree folder. Brian's answers for UJ-2 (P4, 13 October 2026) give such a project worktrees at `~/Worktrees/{repo}/{folder}`, and the change `repositories-of-a-project` specifies it. This record keeps ADR-0004's model and adds the repository and worklane levels that UJ-2 builds and UJ-3 to UJ-8 extend.

## Decision

The chain is:

```
workspace → organisation → project → repository → worklane and release → issue
```

- An organisation is a workspace-scoped table `organisation(id, workspace_id, name, key, created_at, updated_at)`. `name` is unique per workspace ignoring case. `key` is unique per workspace and matches `^[a-z0-9][a-z0-9-]*$`. There is no rename or delete of an organisation; its key can change.
- `project.organisation_id` is a nullable soft reference, so a project has at most one organisation.
- A repository is a GitHub repository in one project: `repository(project_id, github_repo_id, full_name, private, default_branch, checkout_daemon_id, checkout_path, version_policy)`. `full_name` is unique per workspace ignoring case. The server reads the GitHub fields from GitHub itself, never from the client.
- A repository has one normal checkout: a machine (the daemon id) and an absolute path, accepted only after a check on that machine passed.
- `version_policy` is one of `pubspec.yaml`, `Cargo.toml`, `pyproject.toml`, `package.json`, `tags` or `none`. Tags are always `v{version}`.
- A worklane belongs to one repository. Every repository has exactly one worklane `main` from the moment it is created; `main` works in the repository's normal checkout and stores no folder or machine of its own. Every other worklane stores its folder and machine.
- A worktree path is `~/Worktrees/{organisation key}/{repo}/{folder}`, or `~/Worktrees/{repo}/{folder}` for a project without an organisation. `{repo}` is the name part of `full_name`. A path is computed when its worktree is made and stored with it; a later key change moves nothing.
- A worklane's state is computed from its issues, never stored.
- No level references a level above its parent: worklanes and releases reference a repository, never a project or an organisation.

## Consequences

- Moving a project to another organisation moves its repositories, worklanes and releases with it. Worktrees made before stay where they are.
- Counts per organisation and project (projects, repositories, worklanes) are computed from the lists the client loads or by a query joined through the chain. There is no denormalised counter.
- Deletes run in application code in one transaction, top down: workspace delete, project delete and repository removal each delete the levels under them. Removing a repository never touches a folder on disk.
- Brain concepts stay out of the schema. The brain sync keeps its own map from brain ids to Multica ids.
- Project resources are gone. A project's code is its repositories.
