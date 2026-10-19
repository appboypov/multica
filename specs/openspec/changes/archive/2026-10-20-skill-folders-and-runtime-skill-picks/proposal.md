## At a glance

A folder on Brian's Mac feeds the Skills library, and each run reads a folder skill from that folder. The "Inherited from runtime" switch turns every skill on or off, and each skill stays switchable both ways. A search field filters an agent's skills and the Add skill dialog.

## Why

Brian keeps his skills in folders on his Mac and wants agents to use them without copying them into Multica. The workspace library was empty, so Add skill stayed greyed out. With inheritance off, every inherited skill was locked off, so one skill could not be turned back on. Success means Brian adds a folder once, gives agents its skills with Add skill, edits a skill in his own editor and the next run uses the edit, and turns off inheritance and then turns on the few skills he wants.

## What Changes

- The Skills page gets a "Skill folders" section. Each folder is saved per workspace and machine, and every direct subfolder with a `SKILL.md` becomes a library skill with the folder as its source.
- The library reads the folders each time the Skills page opens: a new subfolder comes in, a gone one leaves the library and its agents, and a name already in the library stays out, marked "Name taken".
- A run reads a folder skill's files from disk when it starts, for every provider. Multica keeps no copy to edit.
- The section switch turns every inherited skill on or off. Each flip clears the earlier per-skill choices, and each row stays switchable both ways.
- The agent's Skills tab and the Add skill dialog get a search field.

## Capabilities

### New Capabilities

- `skill-folders`: folders on a machine feed the workspace's Skills library, read live at run start.
- `agent-skill-search`: a search field filters an agent's skills and the Add skill dialog.

### Modified Capabilities

- `runtime-skill-inheritance`: the section switch sets every skill on or off, and each row switches both ways while the section is off.

## Impact

Migrations 804 and 805, the agent and skill handlers, the daemon's skill listing and run setup, the Skills page, the agent's Skills tab, the Add skill dialog and their locales change. ADR-0033 records the decision and supersedes ADR-0017 "Runtime skill switches exist only where the run enforces them".

## Constraints

- A folder skill reaches only runs on the machine that holds the folder.
- A folder skill is edited in its folder, never in Multica.
