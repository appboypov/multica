// Warning thresholds for an agent's own text fields: the inputs warn past
// them but never block typing or saving, and the API accepts any length.
// Kept in core so both apps and the test suite read from one source.
export const AGENT_DESCRIPTION_WARN_LENGTH = 255;

// The starter count stays a hard cap: an agent holds at most three.
export const AGENT_CONVERSATION_STARTERS_MAX = 3;
export const AGENT_CONVERSATION_STARTER_LABEL_WARN_LENGTH = 80;
export const AGENT_CONVERSATION_STARTER_WARN_LENGTH = 4000;

// Valid range for the per-agent scheduler cap. Kept here so creation,
// duplication, and settings editing cannot silently drift.
export const AGENT_MAX_CONCURRENT_TASKS_MIN = 1;
export const AGENT_MAX_CONCURRENT_TASKS_MAX = 50;
