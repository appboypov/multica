# madspec-sync

Two manual tools that fill the local Multica workspace from MadSpec, read from its checkout `~/Repos/Ventures/madspec`. They run only when started by hand.

- `multica_sync.py` syncs the MadSpec personas to Multica agents, from MadSpec's `skills/` as it is on `origin/main`. Spec: [specs/multica-agents.md](specs/multica-agents.md).
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
