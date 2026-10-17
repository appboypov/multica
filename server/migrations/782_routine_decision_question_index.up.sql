CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS routine_decision_question
ON routine_decision (question_id)
WHERE question_id IS NOT NULL;
