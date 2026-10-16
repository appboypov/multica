# Design

The manual sync exports MadSpec and git-backed offices from origin/main. It calls roster.personas with the office snapshot root and MadSpec skills root. Persona instruction selections point to the real office skill folder. Roster team worker IDs select office agent files, excluding session helpers. Worker frontmatter supplies name, description, skills and ordered models; its body supplies instructions. Existing fallback overlays carry the remaining models.

Managed persona and worker environment markers identify owned agents. An unmanaged name conflict is reported without changing it. Dry run reads API state and source files, prints intended writes and leaves API and overlay files unchanged. No squad endpoint is used.
