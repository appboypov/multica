# madspec-sync

Two manual tools that fill the local Multica workspace from MadSpec, read from its checkout `~/Repos/Ventures/madspec`. They run only when started by hand.

- `multica_sync.py` syncs the MadSpec personas to Multica agents. The persona list, models and arguments come from `skills/` on `origin/main`; each agent selects its whole skill folder in the checkout, whose content the daemon reads from disk at every task start. Uncommitted skill edits reach the next task without another sync; uncommitted model or argument changes do not change the synced configuration. Spec: [specs/multica-agents.md](specs/multica-agents.md).
- `multica_projects.py` syncs the brain's Repository objects to Multica projects, each with a `local_directory` resource on its checkout. Spec: [specs/multica-projects.md](specs/multica-projects.md).

Both read the server, workspace and token from `~/.multica/profiles/local/config.json`.

## Commands

Run from the root of this checkout:

```sh
uv run madspec-sync/multica_sync.py            # every persona
uv run madspec-sync/multica_sync.py mitchel    # one persona, by name or command
uv run madspec-sync/multica_projects.py        # every Repository object
uv run --with pytest pytest -p no:cacheprovider madspec-sync/tests
```

## Persona instructions

For Guus on Brian's Mac, the sync writes these agent fields:

```json
{
  "instruction_selection": [
    "/Users/codaveto/Repos/Ventures/madspec/skills/as-ux-designer"
  ],
  "instructions": "`{skill-root}` is /Users/codaveto/Repos/Ventures/madspec/skills/as-ux-designer. Your instructions are the as-ux-designer skill below; follow it.\n\n{selection}"
}
```

The path uses the sync user's home folder. At task start, the runtime's daemon replaces `{selection}` with the selected folder's tree and text files as they are on disk, including supporting files below the skill folder. Multica stores the paths, not the skill content. A missing or unreadable selection fails the task with `instruction_selection_error`, without a retry.

Run the sync as the local omp runtime's owner: only the runtime's owner may set or change an agent's selection. The model, thinking level, `--skills=` and fallback `--config=` still come from `origin/main`.
