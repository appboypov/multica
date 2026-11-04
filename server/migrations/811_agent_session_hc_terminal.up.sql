-- A session started in a Herdr app terminal stores that terminal's herdr-core
-- address (`t10`, or `machine/t5` on another machine) in hc_terminal. The
-- daemon follows it on the herdr-core socket as it follows herdr_pane_id on
-- the Herdr socket. It is null outside the Herdr app; an address never
-- changes while the terminal lives, so only a register writes it.
--
-- The runner sends this file as one implicit transaction. ALTER TABLE takes a
-- short ACCESS EXCLUSIVE lock on agent_session, so bound the wait and retry on
-- the next start rather than queue writers behind it.
SET LOCAL lock_timeout = '2s';

ALTER TABLE agent_session ADD COLUMN IF NOT EXISTS hc_terminal TEXT;
