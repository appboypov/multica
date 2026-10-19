DROP TABLE IF EXISTS skill_folder;
ALTER TABLE agent RENAME COLUMN runtime_skill_exceptions TO disabled_runtime_skills;
