SET LOCAL lock_timeout = '2s';

ALTER TABLE project
    DROP COLUMN IF EXISTS slug;
