# madspec-sync

Two manual tools fill the local Multica workspace from the agents repo and the brain. They run only when started by hand.

- `multica_sync.py` syncs the agents and workers of the agents repo, `~/Repos/Agents`, to local omp agents. Configuration comes from each folder's `agent.toml` on `origin/main`. Each agent selects its folder's `AGENTS.md`, whose disk content the daemon reads at task start. Source paths, model rules, ownership and dry-run behavior are defined in [specs/multica-agents.md](specs/multica-agents.md).
- `multica_projects.py` fills Multica organisations, one project per brain Product and each Product's repositories. It applies Add repository's checkout checks and picks the first version file found, or tags. The projects UJ-1 kept merge into the Product's project only once their repositories are added there, and are deleted only once empty. Spec: [specs/multica-projects.md](specs/multica-projects.md).

The linked specifications are the sync tools' contracts. When the Product sync
adds a repository, it sends the first install command found by its passed
checkout check, or `''` (None) when none was found. It never updates an existing
repository's install command, including an explicit None saved in Multica.

Both read the server, workspace and token from `~/.multica/profiles/local/config.json`.

The Product sync reads Organisation Linear keys and Product/Repository links from the brain without writing to it. Organisation, Product and added Repository identity is kept in `~/.local/state/madspec/multica/brain-sync.json`, separated by server and workspace and saved atomically after each create or take-over. Its `repository_projects` map identifies the projects UJ-1 retained; each entry is removed after its source merges or no longer exists. Existing names, organisation keys, project organisations and repository settings stay as Brian set them. Deleted brain objects remain in Multica and are reported.

An absent repository must be visible to the workspace's GitHub App and have exactly one checkout outside `~/Repos/Archive` that passes the local omp runtime's check. Missing or ambiguous checkouts, wrong origin or branch and inaccessible repositories are reported and left alone. A repository Brian put in another project stays there, with that project printed. No checkout resources are written. Retained projects merge only after their repository is in the Product's project: every issue page must move successfully, and a fresh issue read must count zero before deletion. Failed reads change no entities, and failed writes skip dependent work without stopping independent writes.

## Commands

Run from the root of this checkout:

```sh
uv run madspec-sync/multica_sync.py --dry-run  # preview agents and workers
uv run madspec-sync/multica_sync.py           # sync agents and workers
uv run madspec-sync/multica_sync.py mitchel   # one agent or worker by name or folder
uv run madspec-sync/multica_sync.py Lukas --adopt Lukas --dry-run
uv run madspec-sync/multica_sync.py Lukas --adopt Lukas
uv run madspec-sync/multica_projects.py        # organisations, Products, guarded repository merges
uv run --with pytest --with pyyaml pytest -p no:cacheprovider madspec-sync/tests
uv run --with pytest --with pyyaml pytest -p no:cacheprovider madspec-sync/tests/test_multica_projects.py
```

Use repeatable `--adopt NAME` only for an existing unmanaged agent that should come under the sync. Each name must have one match. Other unmanaged agents and all squads stay unchanged. Dry run makes no API or overlay writes.

The Product sync tests fake the brain reader and Multica HTTP transport,
including the GitHub App repository list and runtime checkout checks. They
follow [the Product sync contract](specs/multica-projects.md) without live calls.

## Agent instructions

For Guus on Brian's Mac, the sync writes these agent fields:

```json
{
  "instruction_selection": [
    "/Users/codaveto/Repos/Agents/guus/AGENTS.md"
  ],
  "instructions": "<SystemIdentity>\n{selection}\n</SystemIdentity>"
}
```

The path uses the sync user's home folder. At task start, the runtime's daemon replaces `{selection}` with the file as it is on disk. Multica stores the path, not the file's content. A missing or unreadable selection fails the task with `instruction_selection_error`, without a retry.

Run the sync as the local omp runtime's owner: only that owner may set or change an agent's selection. The name, description, model, thinking level and fallback `--config=` come from the folder's `agent.toml` on `origin/main`.
