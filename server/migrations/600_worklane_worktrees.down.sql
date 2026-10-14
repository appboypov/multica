ALTER TABLE activity_log DROP COLUMN IF EXISTS repository_id;
ALTER TABLE activity_log DROP COLUMN IF EXISTS worklane_id;
ALTER TABLE worklane DROP COLUMN IF EXISTS path;
ALTER TABLE repository DROP COLUMN IF EXISTS install_command;
