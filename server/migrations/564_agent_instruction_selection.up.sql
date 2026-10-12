ALTER TABLE agent ADD COLUMN instruction_selection JSONB NOT NULL DEFAULT '[]'::jsonb;
