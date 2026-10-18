-- Choice labels come from the decision's frozen squad snapshot. Only question
-- presentation and answer values change; decision IDs and owned tasks stay put.
SET LOCAL lock_timeout = '2s';

WITH squad_options AS (
    SELECT d.question_id, d.workspace_id, squad.key AS squad_id,
           squad.value->>'name' AS name,
           count(*) OVER (
               PARTITION BY d.question_id, d.workspace_id, squad.value->>'name'
           ) AS name_count
    FROM routine_decision d
    JOIN question q ON q.id = d.question_id AND q.workspace_id = d.workspace_id
    CROSS JOIN LATERAL jsonb_each(d.input->'rule'->'squads') squad
    WHERE d.input->'rule'->>'decision' = 'squad'
      AND q.answer_type = 'single_choice'
), labels AS (
    SELECT question_id, workspace_id,
           jsonb_object_agg(
               name || ' (' || squad_id || ')',
               CASE WHEN name_count > 1
                   THEN name || ' (' || left(squad_id, 8) || ')'
                   ELSE name
               END
           ) AS replacements
    FROM squad_options
    GROUP BY question_id, workspace_id
), converted AS (
    SELECT q.id, q.workspace_id,
           jsonb_set(q.settings, '{options}', (
               SELECT jsonb_agg(
                   coalesce(l.replacements->option, to_jsonb(option)) ORDER BY position
               )
               FROM jsonb_array_elements_text(q.settings->'options')
                    WITH ORDINALITY AS options(option, position)
           )) AS settings,
           CASE WHEN l.replacements ? (q.suggested_answer->>'option')
               THEN jsonb_set(q.suggested_answer, '{option}',
                             l.replacements->(q.suggested_answer->>'option'))
               ELSE q.suggested_answer
           END AS suggested_answer
    FROM question q
    JOIN labels l ON l.question_id = q.id AND l.workspace_id = q.workspace_id
), updated_questions AS (
    UPDATE question q
    SET settings = c.settings, suggested_answer = c.suggested_answer
    FROM converted c
    WHERE q.id = c.id AND q.workspace_id = c.workspace_id
      AND (q.settings IS DISTINCT FROM c.settings
           OR q.suggested_answer IS DISTINCT FROM c.suggested_answer)
    RETURNING q.id
)
UPDATE question_answer a
SET value = jsonb_set(a.value, '{option}', l.replacements->(a.value->>'option'))
FROM labels l
WHERE a.question_id = l.question_id AND a.workspace_id = l.workspace_id
  AND l.replacements ? (a.value->>'option');
