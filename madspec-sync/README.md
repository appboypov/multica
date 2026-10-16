# madspec-sync

Two manual tools fill the local Multica workspace from MadSpec and its agent offices. They run only when started by hand.

- `multica_sync.py` syncs managed personas and roster workers to local omp agents. Configuration comes from `origin/main` in MadSpec and each git-backed office. Workers use their agent file body as instructions. Personas select their whole skill folder, whose disk content the daemon reads at task start. Source paths, model rules, ownership and dry-run behavior are defined in [specs/multica-agents.md](specs/multica-agents.md).
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
uv run madspec-sync/multica_sync.py --dry-run  # preview personas and workers
uv run madspec-sync/multica_sync.py           # sync personas and workers
uv run madspec-sync/multica_sync.py mitchel   # one persona or worker by name or ID
uv run madspec-sync/multica_sync.py Lukas --adopt Lukas --dry-run
uv run madspec-sync/multica_sync.py Lukas --adopt Lukas
uv run madspec-sync/multica_projects.py        # organisations, Products, guarded single-repo merges
uv run --with pytest --with pyyaml pytest -p no:cacheprovider madspec-sync/tests
uv run --with pytest pytest -p no:cacheprovider madspec-sync/tests/test_multica_projects.py
```

Use repeatable `--adopt NAME` only for an existing unmanaged agent that should come under the sync. Each name must have one match. Other unmanaged agents and all squads stay unchanged. Dry run makes no API or overlay writes.

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

Run the sync as the local omp runtime's owner: only that owner may set or change an agent's selection. The model, thinking level, `--skills=` and fallback `--config=` come from the office's `origin/main`. Office-only persona skills use their office path.
