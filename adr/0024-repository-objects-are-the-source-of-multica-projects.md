# ADR-0024: Repository objects are the source of Multica projects

- Status: superseded by madspec ADR-0005
- Date: 2026-10-10
- Change: `client-repositories-become-multica-projects`

## Context

A Multica worker runs a Task inside a Multica project, and a project only gives the worker a repository when it has a resource that points at one. The local workspace De App Specialist had no projects. Brian decided on 2026-10-10 that every repository De App Specialist builds is one project, kept in line with the brain without hand work. Repositories already have one record each in the brain: the Repository object with its **GitHub key**. ADR-0023 makes the agent files the source of Multica workers in the same way.

## Decision

- Each Repository object in Brian's Anytype brain is the one source of one Multica project in the local workspace, titled with the GitHub repository name.
- The project's one resource is a `local_directory` on this Mac's daemon in `worktree` mode, on the repository's checkout under `~/Repos`, found by its `origin` remote. A project gets no `github_repo` resource.
- `skills/madspec/scripts/multica_projects.py` writes them; the linear-sync service, `make multica` and the git hooks run it.
- A managed project carries the resource label `madspec:<owner/name>`. The sync leaves every unmarked project alone, apart from taking over one with the repository's name and no local folder. It never deletes a project; a repository that leaves moves its project to `completed`, and one that returns moves it back to `in_progress`.

## Consequences

- A repository becomes a project by getting a Repository object, never by a step in Multica; a project changed by hand in Multica is overwritten in its resource at the next sync, and moves between `in_progress` and `completed` with its repository, while its title, issues and any other status stay Brian's.
- A repository gains a project only when it counts; an existing project stays when its repository cannot be placed.
- Multica, Anytype and GitHub must be reachable for a sync; a failed read writes nothing.
