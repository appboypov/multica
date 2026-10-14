-- Whether the run of this task may ask questions, resolved once at claim from
-- the issue's Agent questions and the workspace default (ADR-0005). The
-- default false fails closed: a task claimed before this column existed
-- cannot ask.
SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '10s';

ALTER TABLE agent_task_queue ADD COLUMN IF NOT EXISTS questions_enabled boolean NOT NULL DEFAULT false;
