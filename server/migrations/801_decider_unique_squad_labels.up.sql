-- Rebuild presentation from frozen squad identities. Saved reviews identify
-- selections even when the question's labels are ambiguous.
SET LOCAL lock_timeout = '2s';

DO $$
DECLARE
    decision record;
    squad record;
    labels jsonb;
    names text[];
    label text;
    suffix text;
    width integer;
    options jsonb;
    prompt_value text;
    sentence_label text;
    sentence_prefix text;
    sentence_suffix text;
BEGIN
    FOR decision IN
        SELECT d.question_id, d.workspace_id, d.input->'rule'->'squads' AS squads,
               d.proposal->>'choice' AS proposed, d.reviews, d.review_mode,
               q.answer_type, q.prompt
        FROM routine_decision d
        JOIN question q ON q.id = d.question_id AND q.workspace_id = d.workspace_id
        WHERE d.input->'rule'->>'decision' = 'squad'
          AND q.answer_type IN ('single_choice', 'text')
    LOOP
        labels := '{}'::jsonb;
        SELECT array_agg(value->>'name') INTO names FROM jsonb_each(decision.squads);
        FOR squad IN SELECT key AS id, value->>'name' AS name FROM jsonb_each(decision.squads)
        LOOP
            label := squad.name;
            IF (SELECT count(*) FROM unnest(names) AS n(name) WHERE n.name = squad.name) > 1 THEN
                width := 8;
                WHILE EXISTS (
                    SELECT 1 FROM jsonb_each(decision.squads) AS other
                    WHERE other.key <> squad.id AND left(other.key, width) = left(squad.id, width)
                ) LOOP
                    width := width + 1;
                END LOOP;
                suffix := ' (' || left(squad.id, width) || ')';
                LOOP
                    label := label || suffix;
                    EXIT WHEN NOT label = ANY(names);
                END LOOP;
            END IF;
            labels := labels || jsonb_build_object(squad.id, label);
        END LOOP;

        SELECT jsonb_agg(value ORDER BY value COLLATE "C") INTO options FROM jsonb_each_text(labels);
        prompt_value := decision.prompt;
        sentence_prefix := CASE WHEN decision.review_mode = 'after' THEN 'The ' ELSE 'Should the ' END;
        sentence_suffix := CASE WHEN decision.review_mode = 'after' THEN ' squad took ' ELSE ' squad take ' END;
        -- Match only the proposed squad's sentence prefix, keeping issue and place text intact.
        FOREACH sentence_label IN ARRAY ARRAY[
            labels->>decision.proposed,
            (decision.squads->decision.proposed->>'name') || ' (' || decision.proposed || ')',
            (decision.squads->decision.proposed->>'name') || ' (' || left(decision.proposed, 8) || ')',
            decision.squads->decision.proposed->>'name'
        ] LOOP
            IF starts_with(prompt_value, sentence_prefix || sentence_label || sentence_suffix) THEN
                prompt_value := sentence_prefix || (labels->>decision.proposed) || sentence_suffix
                    || substr(prompt_value, length(sentence_prefix || sentence_label || sentence_suffix) + 1);
                EXIT;
            END IF;
        END LOOP;
        UPDATE question
        SET settings = CASE WHEN decision.answer_type = 'single_choice'
                THEN jsonb_set(settings, '{options}', options) ELSE settings END,
            suggested_answer = jsonb_set(suggested_answer,
                CASE WHEN decision.answer_type = 'single_choice' THEN '{option}'::text[] ELSE '{text}'::text[] END,
                labels->decision.proposed),
            prompt = CASE WHEN decision.answer_type = 'text' THEN
                split_part(prompt_value, E'\nSubmit a squad name from the list below.\n', 1)
                || E'\nSubmit a squad name from the list below.\n'
                || (SELECT string_agg(value, E'\n' ORDER BY value COLLATE "C") FROM jsonb_each_text(labels))
                ELSE prompt_value END
        WHERE id = decision.question_id AND workspace_id = decision.workspace_id;

        UPDATE todo t
        SET title = 'Confirm routine choice: ' || (labels->>decision.proposed),
            body = q.prompt
        FROM question q
        WHERE q.id = decision.question_id AND q.workspace_id = decision.workspace_id
          AND t.question_id = q.id AND t.workspace_id = q.workspace_id AND t.kind = 'question';

        UPDATE question_answer a
        SET value = jsonb_set(a.value,
            CASE WHEN decision.answer_type = 'single_choice' THEN '{option}'::text[] ELSE '{text}'::text[] END,
            labels->(
                SELECT review->>'choice'
                FROM jsonb_array_elements(decision.reviews) WITH ORDINALITY AS r(review, position)
                WHERE review->>'member_id' = a.answerer_user_id::text
                ORDER BY position DESC LIMIT 1
            ))
        WHERE a.question_id = decision.question_id AND a.workspace_id = decision.workspace_id;
    END LOOP;
END $$;
