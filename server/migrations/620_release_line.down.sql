DELETE FROM activity_log
WHERE action IN ('release_planned', 'release_edited', 'release_moved', 'release_removed');
DROP TABLE IF EXISTS release;
ALTER TABLE repository DROP COLUMN IF EXISTS last_tag;
