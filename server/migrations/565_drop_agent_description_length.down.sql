ALTER TABLE agent
    ADD CONSTRAINT agent_description_length
    CHECK (char_length(description) <= 255) NOT VALID;
