ALTER TABLE issue ADD COLUMN lane_position double precision;

WITH ranked AS (
    SELECT id, row_number() OVER (PARTITION BY workspace_id, worklane_id ORDER BY created_at, id)::double precision AS position
    FROM issue WHERE worklane_id IS NOT NULL
)
UPDATE issue SET lane_position = ranked.position FROM ranked WHERE issue.id = ranked.id;

