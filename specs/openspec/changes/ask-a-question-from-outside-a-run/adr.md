## At a glance

| Kind | Name | Change | Path |
|---|---|---|---|
| ADR | ADR-0021: A member asks a question on an issue, outside a run, and wakes no agent | new | `adr/0021-a-member-asks-on-an-issue-and-wakes-no-agent.md` |

# ADR Review Manifest

- Status: completed
- Review date: 2026-10-16

## Review Summary

ADR review completed for this change. ADR-0021 binds more than this change: who may ask a question (a run's agent or a member outside a run), that a member's question is asked on an issue, and that it never reaches or starts a run. Plan steps 3 to 6 on WORK-435 (the omp plugin hearing answers, the skills asking Brian in Multica, the Anytype inbox removal) build on it.

ADR-0021 extends ADR-0002's asker without replacing the aggregate, so ADR-0002 stays in force and is not superseded.

## In-Force ADRs Reviewed

- ADR-0002 Questions are their own aggregate: binding. The question stays one aggregate; ADR-0021 adds a member asker next to the agent asker and the run it was asked in.
- ADR-0004 Question sets group questions: binding. A member asks a set the same way; the ask id and policy are unchanged.
- ADR-0005 Whether a run may ask is decided at claim: binding for agents only. A member has no run, so the issue's Agent questions setting does not gate a member's ask.
- ADR-0006 Questions are listed per viewer: binding. Member-asked questions list the same way; the asker filter matches either asker.
- ADR-0007 A question reaches its parents at read time: binding. A member's question reaches the parents of its issue link; its Wake targets stay empty.
- ADR-0019 Input reaches a running run as a receipt: binding for agent questions. A member's answers are not run input.
- ADR-0020 Only Wake turns answers into a run: binding. A member's question offers no Wake, so its answers never become a run.
- ADR-0003 Only Wake turns answers into a run: superseded by ADR-0020, history only.
- ADR-0001, ADR-0004 Organisations hold projects (superseded by ADR-0006), ADR-0005 Project status and dates, ADR-0006 Repositories hold worklanes, and ADR-0007 to ADR-0017 on worktrees, worklanes, releases, templates, done dates and client accounts: not touched.
- ADR-0018 lives only on open branches (GitHub writes; a wake continues where the question was asked, on the questions-in-chat branch): not touched. Numbering continues after ADR-0020 on `main`.

## New Durable ADRs Created

- `adr/0021-a-member-asks-on-an-issue-and-wakes-no-agent.md`
