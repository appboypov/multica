-- Text confirmation labels come from the decision's frozen squad snapshot.
-- Only displayed choices and answer values change; squad IDs and tasks stay put.
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
      AND q.answer_type = 'text'
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
    SELECT q.id, q.workspace_id, q.prompt AS original_prompt,
           (SELECT string_agg(
                coalesce(l.replacements->>line, line), E'\n' ORDER BY position
            )
            FROM unnest(string_to_array(q.prompt, E'\n'))
                 WITH ORDINALITY AS lines(line, position)) AS prompt,
           CASE WHEN l.replacements ? (q.suggested_answer->>'text')
               THEN jsonb_set(q.suggested_answer, '{text}',
                             l.replacements->(q.suggested_answer->>'text'))
               ELSE q.suggested_answer
           END AS suggested_answer
    FROM question q
    JOIN labels l ON l.question_id = q.id AND l.workspace_id = q.workspace_id
), updated_questions AS (
    UPDATE question q
    SET prompt = c.prompt, suggested_answer = c.suggested_answer
    FROM converted c
    WHERE q.id = c.id AND q.workspace_id = c.workspace_id
      AND (q.prompt IS DISTINCT FROM c.prompt
           OR q.suggested_answer IS DISTINCT FROM c.suggested_answer)
    RETURNING q.id
), updated_todos AS (
    UPDATE todo t
    SET body = c.prompt
    FROM converted c
    WHERE t.question_id = c.id AND t.workspace_id = c.workspace_id
      AND t.body = c.original_prompt AND t.body IS DISTINCT FROM c.prompt
    RETURNING t.id
)
UPDATE question_answer a
SET value = jsonb_set(a.value, '{text}', l.replacements->(a.value->>'text'))
FROM labels l
WHERE a.question_id = l.question_id AND a.workspace_id = l.workspace_id
  AND l.replacements ? (a.value->>'text');
