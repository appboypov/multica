---
type: intent
---

# 🎯 Linear integration in our Multica fork

## 🗣️ User requests

### 2026-10-03

> i want to add a linear integration to our version  - i have already made many lienar integrations and we can use the same linear app i made in ~/Repos/Ventures/skuddy/ - i want it integrated in our version of multica can we reuse that logic now that i think of it? to create various types of syncs and views and then from that create issues in mutlica with the linear issue linked to it

> kunnen we bij multica bepalen welke folder gebruikt wrodt voor worktrees want ik heb een conventie dat worktrees in ~/Worktrees/{team-key}/ aangemaakt worden

> nee dit is niet goed lusiter we hebben allemaal verschillende teams in linear. die hebben de juiste issue ids - de ids van multica wil ik niet gerbuiken als branch naam - issue branch naam va lienar is de juistenaam - worktrees moeten dus door de agent worden aangemaakt met de linear team key van de organisatie (alleen hoofd teams, geen sub teams)

## 🎯 End goal

Our Multica fork connects to Linear. Several kinds of syncs and views bring Linear work in, and from them Multica issues are created that link back to their Linear issue. Proven Linear integration work from Skuddy is reused where it fits. An agent that works on such an issue creates its own worktree under `~/Worktrees/{team-key}/`, where the team key is the Linear issue's top-level team key and never a sub-team key, on the Linear issue's branch name. It never uses a Multica id as the branch name.

## ❔ Open points

- [ ] What "the same Linear app" is: Skuddy connects with a personal API key and holds no Linear OAuth app.
- [ ] Which kinds of syncs and views are wanted.
- [ ] Sync direction: into Multica only, or also back to Linear.
- [ ] Where a sub-team issue's worktree goes inside the main team's folder: `madspec-git` now adds a `{subteam}` folder level.
- [ ] How the agent knows the Linear issue before the integration exists: Multica's task instructions tell it to use `multica repo checkout`, which makes `agent/<agent>/<task>` branches under `~/multica_workspaces_<profile>` (`server/internal/daemon/execenv/runtime_config_sections.go:615`).

## 🔗 Material

- PRD: [prd.md](prd.md)
- Research: [research-linear-integration.md](research-linear-integration.md)
