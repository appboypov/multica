-- Worklanes get their own worktree (ADR-0006) and their activity (ADR-0008).
--
-- repository.install_command: NULL means not found yet, '' means None,
-- anything else is the command a new worklane runs in its worktree.
-- worklane.path: the absolute worktree path the daemon resolved. NULL for
-- `main`, which works in the repository's checkout; a path is stored when its
-- worktree is made, so a later key change moves nothing.
-- activity_log.worklane_id and activity_log.repository_id: worklane and
-- repository entries, with issue_id NULL.
--
-- All four columns are nullable and without defaults, so each ALTER is
-- metadata-only. No foreign keys (AGENTS.md). Their indexes are built
-- CONCURRENTLY in 601 to 603, one per file.
ALTER TABLE repository ADD COLUMN IF NOT EXISTS install_command TEXT;
ALTER TABLE worklane ADD COLUMN IF NOT EXISTS path TEXT;
ALTER TABLE activity_log ADD COLUMN IF NOT EXISTS worklane_id UUID;
ALTER TABLE activity_log ADD COLUMN IF NOT EXISTS repository_id UUID;
