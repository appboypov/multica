SET LOCAL lock_timeout = '2s';

ALTER TABLE agent_session
    DROP COLUMN IF EXISTS hc_terminal;
