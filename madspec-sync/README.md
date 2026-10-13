# madspec-sync

Two manual tools that fill the local Multica workspace from MadSpec, read from its checkout `~/Repos/Ventures/madspec`. They run only when started by hand.

- `multica_sync.py` syncs the MadSpec personas to Multica agents. The persona list, models and arguments come from `skills/` on `origin/main`; each agent selects its whole skill folder in the checkout, whose content the daemon reads from disk at every task start. Uncommitted skill edits reach the next task without another sync; uncommitted model or argument changes do not change the synced configuration. Spec: [specs/multica-agents.md](specs/multica-agents.md).
- `multica_projects.py` fills Multica organisations and one project per brain Product, places its counting repositories' checkout resources on it, and moves issues from marked repository projects into it before deleting only those emptied projects. Spec: [specs/multica-projects.md](specs/multica-projects.md).

Both read the server, workspace and token from `~/.multica/profiles/local/config.json`.

The Product sync reads Organisation Linear keys and Product/Repository links from the brain without writing to it. Organisation and Product identity is kept in `~/.local/state/madspec/multica/brain-sync.json`, separated by server and workspace and saved atomically after each create or take-over. Existing names, organisation keys and project organisations stay as Brian set them; deleted brain objects remain in Multica and are reported.

Counting controls only checkout placement: a Repository with a Product still merges its marked repository project even if its checkout cannot be placed or GitHub archived it. A Repository without a Product or gone from the brain keeps its project and issues. Each issue page must move successfully, and a fresh project read must count zero issues before deletion. The sync prints every write and report; read failures write nothing. Run it only against the intended profile's server after that server supports organisations. The issue moves are not undone if the organisation migrations are rolled back.

## Commands

Run from the root of this checkout:

```sh
uv run madspec-sync/multica_sync.py            # every persona
uv run madspec-sync/multica_sync.py mitchel    # one persona, by name or command
uv run madspec-sync/multica_projects.py        # organisations and every Product
uv run --with pytest pytest -p no:cacheprovider madspec-sync/tests
uv run --with pytest pytest -p no:cacheprovider madspec-sync/tests/test_multica_projects.py
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
