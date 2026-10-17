# Design

The manual sync exports each top folder's `agent.toml` and `AGENTS.md` from the agents repo on origin/main. A folder with an `agent.toml` is an agent; its name, title and ordered models come from that file. Each agent selects its folder's `AGENTS.md` on disk as its instructions. A first model sets the agent's model and effort, and further models go to a fallback overlay. A worker is a folder named after its level, such as `medior-backend`.

Managed persona and worker environment markers identify owned agents. An unmanaged name conflict is reported without changing it. Repeatable `--adopt NAME` allows one unambiguous same-name unmanaged agent to become managed, preserving its other environment values. Dry run reads API state and source files, prints intended writes including adoptions, and leaves API and overlay files unchanged. No squad endpoint is used.
