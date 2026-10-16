# Releases

## Inspect and assign

Read `multica release get <release> --output json` and `multica release list --repository owner/repo --output json`. Owner fields (`owner_type`, `owner_id`, `owner_name`, `owner_avatar_url`) are nullable. `multica release assign <release> --to <agent-or-squad>` resolves an agent or squad in the current workspace; ambiguous names need a real UUID. `--unassign` clears ownership. Members cannot own releases. Shipped releases refuse owner changes. Assignment starts the runnable agent or current squad leader; lacking a runtime does not undo ownership.

## Coordinate a fixed release

A release-owned run receives the real release, its repository/project, every issue in canonical worklane order, current status categories, assignees, parents, pull requests, source actors and trigger reasons. Keep the supplied issues, order, assignees, parents, worklanes, version and line position. Carry the existing release workflow and all applicable human gates. Start or wake work on its existing issues through their assignees' workflow; the platform does not schedule those issues for you. Cancelled is terminal, not successful. Every issue done is a coordination fact, not approval to publish or ship.

## Read and report on this release

Scan `multica release comment list <release> --roots-only --summary --compact --output json`, then expand relevant threads with `--thread <comment-id> --tail 30`. `--recent N` bounds complete threads, not individual comments. `--since <RFC3339>` reads an incremental window; composite `--before` and `--before-id` cursors page older input. Command help defines combination rules.

Write your actual final result to a UTF-8 file inside your working directory using your file-write tool. Post `multica release comment add <release> --content-file ./result.md --output table`, adding `--parent <trigger-comment-id>` for a reply and repeatable `--attachment <local-path>` for delivered files. Clean up the file only after success. Other callers may use `--content` or `--content-stdin`; use one input source. Replies and attachments must stay within the release's workspace. The run's source-task attribution lets completion retain an explicit report without duplicating it; a nonempty final assistant result is recorded only when no explicit report exists. Report failures truthfully; an empty result is not a successful report.

Member comments and replies wake the current owner and coalesce before claim. Exact delivered comment IDs describe only the comments embedded in this turn; later facts are retained for a later turn. Agent reports do not wake the owner recursively.

## Set a wait and finish this turn

Use `multica release wakeup create <release> --until-issue WORK-501 --until-issue-state in_review --instruction-file ./wait.md --expires-in 72h --on-timeout wake`. The issue must be in this workspace. States are `done` (successful), `ended` (done or cancelled), and `in_review`. Schedules use `--kind at --after 10m` or `--at <RFC3339>`, `--kind every --every 1h`, or `--kind cron --cron "0 9 * * *" --timezone Europe/Amsterdam`. Instruction, expiry, timeout, mode and max-fires options follow the existing wakeup contract. Inspect with `release wakeup list <release>` and remove a user wait with `release wakeup delete <release> <wakeup-id>`.

Waits resolve the current owner when they fire; do not supply an agent target. Platform coordination rules are not user-deletable. Finish the current turn after recording the wait and the real result; a durable wakeup starts the later run. Release runs have no Multica questions; use the owner's existing workflow/inbox gates.

## Ship explicitly

Publish through the existing release workflow, obtain its required approval, then call `multica release ship <release>`. Existing issue/pull-request checks still apply. Completion alone never ships. Shipping clears release waits and cancels unstarted coordination, leaving a running shipper able to post its result. Removing the release deletes its coordination and discussion; a terminal callback cannot recreate it.
