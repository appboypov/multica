## REMOVED Requirements

### Requirement: A persona agent selects its whole skill folder

**Reason**: A persona agent runs on its runtime profile, whose command is its alias, and the alias loads the office `AGENTS.md` and the persona's `SKILL.md` itself. A selection would load the skill a second time.

**Migration**: `persona-runtime-profiles` "A run starts the agent through its alias" covers what a run loads. The sync clears each persona agent's `instruction_selection`.

### Requirement: A persona's skill content comes from disk at task start

**Reason**: The alias reads the persona skill from the office at every start, so a run follows an edit without a sync; this no longer goes through the instruction selection.

**Migration**: `persona-runtime-profiles` "A run starts the agent through its alias".
