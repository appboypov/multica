-- An issue that reaches In Review opens no to-do for its creator. The
-- Decider's review to-dos keep their 'decider:' source and stay open.
UPDATE todo
SET done_at = now(), done_by = 'handled'
WHERE kind = 'review' AND done_at IS NULL AND source_key NOT LIKE 'decider:%';
