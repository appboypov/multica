SET LOCAL lock_timeout = '2s';

ALTER TABLE attachment
  DROP COLUMN IF EXISTS question_id;

DROP TABLE IF EXISTS question_event;
DROP TABLE IF EXISTS question_answer;
DROP TABLE IF EXISTS question_answerer;
DROP TABLE IF EXISTS question_link;
DROP TABLE IF EXISTS question;
