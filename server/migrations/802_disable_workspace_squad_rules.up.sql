WITH policy AS (
    SELECT w.id,
           jsonb_agg(
               CASE
                   WHEN rule->>'id' IN ('todo-squad', 'release-squad')
                        AND rule->>'decision' = 'squad'
                   THEN rule || '{"disabled": true}'::jsonb
                   ELSE rule
               END ORDER BY position
           ) AS rules
    FROM workspace w
    CROSS JOIN LATERAL jsonb_array_elements(w.settings #> '{decider,rules}')
        WITH ORDINALITY AS r(rule, position)
    WHERE w.id = '30e8e92f-9a49-4167-95e1-195ab4ec38be'
    GROUP BY w.id
)
UPDATE workspace w
SET settings = jsonb_set(w.settings, '{decider,rules}', policy.rules),
    updated_at = now()
FROM policy
WHERE w.id = policy.id;
