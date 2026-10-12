# ADR-0001: Runtime machine files are read by the daemon

- Status: accepted
- Date: 2026-10-12
- Supersedes: none

## Context

Agents run on runtime machines, each with a daemon that talks to the server over its heartbeat, claims and results. The change `agent-instruction-selection` lets an agent's instructions name files and folders on its runtime machine and lets the runtime's owner browse that machine. The server runs elsewhere (a Docker container on this Mac, or a remote host) and has no access to a runtime machine's disk; copying file content to the server would make a second copy that goes stale and would store a machine's private files in the database.

## Decision

The server stores and passes only absolute paths on a runtime machine. Every read of that machine's disk, for listing a folder or for loading files into a task, is done by the daemon on that machine: at task start for task input, and through a heartbeat-carried request (initiate, pending item on the heartbeat ack, daemon result, client poll) for anything a client asks interactively. Only the runtime's owner may ask for a read of that machine or change what an agent reads from it.

## Consequences

- A feature that needs a runtime machine's files adds a daemon step or a heartbeat-carried request, never a server-side read.
- File content never lands in the database or in a server response other than a listing's names and kinds.
- What an agent reads is the disk at task start; an edit shows in the next task without a sync, and a missing file fails the task on that machine.
- Reads depend on the runtime being online; an offline runtime can neither be browsed nor run tasks.
