# ADR Review Manifest

- Status: completed
- Review date: 2026-10-15

## Review Summary

ADR review completed for this change. The design fits every in-force ADR; one new durable decision is recorded.

## In-Force ADRs Reviewed

- ADR-0001 runtime machine files are read by the daemon: the section expansion lists skills in the daemon on the runtime machine. Fits.
- ADR-0002 questions are their own aggregate: not touched.
- ADR-0003 only Wake turns answers into a run: not touched.
- ADR-0004 question sets group questions, open is per question: not touched.
- ADR-0005 project status and dates stay stored but unseen: not touched.
- ADR-0005 whether a run may ask is decided at claim: the section setting also travels on the claim. Fits.
- ADR-0006 questions are listed per viewer through one row list: not touched.
- ADR-0006 repositories hold worklanes (supersedes ADR-0004 organisations hold projects): not touched.
- ADR-0007 a question reaches its parents at read time: not touched.
- ADR-0007 worktrees are made by the daemon and recorded by the server: not touched.
- ADR-0008 worklane activity is kept in the activity log: not touched.
- ADR-0009 the server keeps worklane and issue worktree folders apart: not touched.
- ADR-0010 the server keeps each repository's release line: not touched.
- ADR-0011 a worklane shows repository activity from its creation on: not touched.
- ADR-0012 the server holds the issue templates and a type is set once: not touched.
- ADR-0013 an issue is placed in a worklane and a release under the repository's lock: not touched.
- ADR-0014 a worklane's issues work in its worktree on their own branch: not touched.
- ADR-0015 the database keeps an issue's done date: not touched.
- ADR-0016 the worklanes and releases pages are built from the workspace's lists: not touched.

## New Durable ADRs Created

- [ADR-0017 runtime skill switches exist only where the run enforces them](../../../../adr/0017-runtime-skill-switches-exist-only-where-the-run-enforces-them.md)
