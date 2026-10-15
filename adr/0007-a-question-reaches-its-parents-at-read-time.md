# ADR-0007: A question reaches its parents at read time

- Status: accepted
- Date: 2026-10-13
- Supersedes: none

## Context

Brian's PRD shows a question to whoever owns the work above it (UJ-4). A question links to issues, comments, runs, projects and other workspace objects (ADR-0002's `question_link`). Every object above a link is a parent: a comment's issue, a run's issue, an issue's parent issues up the chain, and their projects. A parent issue and a project list and count the questions below them, the member assignees of links and parents see them under On my work, and the agent assignees are offered Wake.

What is above what changes all the time and outside the question: a sub-issue is moved to another parent, an issue changes project, a parent is deleted and its children are detached, an assignee changes. The tree is shallow, indexed on `parent_issue_id` and `project_id`, and the app only checks for cycles 10 levels deep on update.

## Decision

Nothing stores which parents a question has. The server reads them from the live issue tree, comment, run and project at the moment of each read:

- One rule, in one SQL file, in two forms. Upward, for given questions: the reached objects and their assignees, for Wake targets and "via". Downward, for given issues or projects: the questions whose links are in their subtree, for the parent lists, On my work and the counts.
- Direct unseen-answer and ask-back queries receive explicit question IDs; issue-wide reads use the question IDs from the agent's Wake scope.
- The recursive walks use `UNION`, so a cycle in the tree ends the walk.
- A test runs the same fixtures through both forms: a question is in X's downward list exactly when X is in its upward reach.
- Lists and counts stay in the one row list of ADR-0006; the client only formats "via".
- Only links are stored. A deleted object's links are removed in the deleting transaction, in application code; a moved object needs no write at all.

## Consequences

- Moving a sub-issue, changing an issue's project or its assignee moves its questions and their Wake targets at once, with no write and no stale copy.
- Every parent list, count and Wake target walks the tree at read time; one walk per page or count, seeded from one issue, one project or the viewer's work. No index is added.
- A change to what counts as a parent (a new link type, squads as an audience) changes one SQL file and its test, not stored rows.
- The client invalidates question queries when an issue's parent, project or assignee changes, because the server's answer moved.
