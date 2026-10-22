# madspec-sync

One manual tool fills the local Multica workspace from the brain. It runs only when started by hand.

- `multica_projects.py` fills Multica organisations, one project per brain Product and each Product's repositories. It applies Add repository's checkout checks and picks the first version file found, or tags. The projects UJ-1 kept merge into the Product's project only once their repositories are added there, and are deleted only once empty. Spec: [specs/multica-projects.md](specs/multica-projects.md). It calls Multica through `multica_api.py`.

The linked specification is the sync tool's contract. When the Product sync
adds a repository, it sends the first install command found by its passed
checkout check, or `''` (None) when none was found. It never updates an existing
repository's install command, including an explicit None saved in Multica.

It reads the server, workspace and token from `~/.multica/profiles/local/config.json`.

The Product sync reads Organisation Linear keys and Product/Repository links from the brain without writing to it. Organisation, Product and added Repository identity is kept in `~/.local/state/madspec/multica/brain-sync.json`, separated by server and workspace and saved atomically after each create or take-over. Its `repository_projects` map identifies the projects UJ-1 retained; each entry is removed after its source merges or no longer exists. Existing names, organisation keys, project organisations and repository settings stay as Brian set them. Deleted brain objects remain in Multica and are reported.

An absent repository must be visible to the workspace's GitHub App and have exactly one main checkout at `~/hq/{organisation}/{project}/repos/{repo}` (ADR-0037), outside `~/hq/misc`, that passes the local omp runtime's check. Missing or ambiguous checkouts, wrong origin or branch and inaccessible repositories are reported and left alone. A repository Brian put in another project stays there, with that project printed. No checkout resources are written. A project the sync creates gets the kebab-case of its title as slug, its folder under its organisation. Retained projects merge only after their repository is in the Product's project: every issue page must move successfully, and a fresh issue read must count zero before deletion. Failed reads change no entities, and failed writes skip dependent work without stopping independent writes.

## Commands

Run from the root of this checkout:

```sh
uv run madspec-sync/multica_projects.py        # organisations, Products, guarded repository merges
uv run --with pytest --with pyyaml pytest -p no:cacheprovider madspec-sync/tests
```

The Product sync tests fake the brain reader and Multica HTTP transport,
including the GitHub App repository list and runtime checkout checks. They
follow [the Product sync contract](specs/multica-projects.md) without live calls.
