# ADR-0016: The Worklanes and Releases pages are built from the workspace's lists

- Status: accepted
- Date: 2026-10-13
- Supersedes: none

## Context

UJ-7 adds two overview pages: Worklanes lists every worklane, and Releases every repository's unshipped releases, both under their repository, project and organisation, in the order Projects shows them. UJ-8 adds the shipped releases to Releases. The app already holds one workspace-wide list each of organisations, projects, repositories, worklanes, releases and release progress, with their realtime invalidation, and Projects already groups and orders projects in the client. A release's progress must count every issue in it, also a closed issue whose worklane was removed, which keeps its release.

## Decision

- The pages read the existing workspace-wide lists and build their tree with pure functions in `packages/core`: `worklaneOverview` and `releaseOverview`. There is no overview endpoint.
- The tree's order is Projects' order: organisations through `groupProjectsByOrganisation`, projects through `sortProjects` with the project view store's sort, repositories by `full_name`, worklanes by `sortWorklanes`, releases by line position.
- A group with nothing to list is left out, and the counts count the rows the overview keeps.
- A release's progress on Releases comes from the server's per-release totals in `GET /api/release-progress` (`releases`), which count every issue with that `release_id`. The per-worklane `rows` stay for the Timeline and the worklane rows.
- Now or next on a worklane row comes from the worklane response, with the issue's status, as ADR-0014 computes it.

## Consequences

- Grouping, order and counts have one implementation each, shared by Projects, Worklanes and Releases.
- The pages stay live through the invalidation the lists already have, with no new event.
- UJ-8 widens which release rows `releaseOverview` keeps and gives the release row a shipped state, and the counts follow.
- A page loads up to six lists on first open. They are cached across Projects, the worklane pages and the overviews.
