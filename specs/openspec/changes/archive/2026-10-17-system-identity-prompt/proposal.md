# The SystemIdentity prompt

## Why

An agent's selected files are its identity. The default prompt says so with one block, without a sentence around it.

## What changes

The first selection in the instructions field inserts `<SystemIdentity>`, `{selection}` and `</SystemIdentity>`, each on its own line. madspec-sync writes the same block as the instructions of every agent it syncs.
