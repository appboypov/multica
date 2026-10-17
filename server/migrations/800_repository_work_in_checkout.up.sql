-- repository.work_in_checkout (ADR-0031): with it on, every issue task of the
-- repository works in its checkout as a task of worklane main, never in an
-- issue or pull-request worktree. Off for every repository from before. A
-- constant default keeps the ALTER metadata-only. No index: the column is read
-- with its repository row.
ALTER TABLE repository ADD COLUMN IF NOT EXISTS work_in_checkout BOOLEAN NOT NULL DEFAULT false;
