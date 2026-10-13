# ADR Review Manifest

- Status: completed
- Review date: 2026-10-13

## Review Summary

ADR review completed for this change.

## In-Force ADRs Reviewed

- [ADR-0001: Runtime machine files are read by the daemon](../../../../adr/0001-runtime-machine-files-are-read-by-the-daemon.md): the copy passes paths only, reads no runtime machine's disk, and keeps "only the runtime's owner changes what an agent reads" by checking ownership before the create.

## New Durable ADRs Created

- None - no major durable architectural decisions were introduced; the CLI copy follows the existing owner rule of web and desktop Duplicate.
