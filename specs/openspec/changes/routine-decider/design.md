# Routine decisions

## At a glance

The server listens to committed events. Its decider service calls TypeSafe, then either proposes a typed answer or queues a Multica task. Task completion sends the Decider answer through the same action path. Brian confirms each proposal during the start period. After that period, every fifth automatic choice comes to him after it is applied.

## Events

- `issue:created` counts as entering its initial status.
- `issue:updated` with `status_changed` matches a status-entry rule.
- `issue:release_joined` is published after commit for issue creation in a release, placement changes, batch release placement and release planning with issues.
- A status change also checks release rules for the issue's release. This catches the last issue becoming done.
- Decision writes do not start another decision. No timer or polling job is added.

## Settings

Workspace `settings.decider` holds `agent_id`, `member_id`, optional `confidence_threshold` (default `0.85`), optional `start_period` (default `true`) and `rules`. The API key is the server setting `TYPESAFE_API_KEY`, read from the environment and never stored in workspace JSON or logged. Rules are off when absent. `member_id` identifies Brian. Only that member can change this settings block, including its threshold, start period and member ID. First setup must be done by the named member with the start period on and threshold `0.85`. The server never changes either policy value.

Each rule has a unique `id`, `event` (`status_entered` or `release_joined`), optional `status`, `decision` (`bug` or `release`), `question`, and `reasons` (at least two named reason descriptions). Bug rules also have `post_merge_property`, `kind_property` and `lead_by_kind`. Property keys are workspace custom-property UUIDs. The first property must be boolean true; the second is the stored kind string or select option ID. Only issues of type `bug` match bug rules. Lead IDs and Decider IDs must belong to this workspace. `member_id` is Brian's workspace user ID.

The Decider agent must use Sonnet 5.5. The server validates its model before queuing. Rules choose event and status explicitly. Release rules also run on member status changes, and only run for a nonempty, unshipped release whose issues are all exactly `done`.

## Judgments

The TypeSafe `jev-latest` request asks two typed choices: the action and its reason. Bug actions are `fix_now` and `fix_later`; release actions are `ready` and `not_yet`. The reason is selected from the rule's descriptions, not invented as free text. Both answers must meet the threshold. The audit uses the lower confidence. API errors leave an explicit failure comment and do not apply an action.

Uncertain answers queue a fresh issue task for the configured agent. Its handoff contains the question, options, reasons and issue/release context. It returns only JSON with `choice` and `reason`. It must not change the issue itself. The result is checked against the allowed choices and reasons before application.

## Applying answers

A persisted receipt identifies a rule and an issue/release state. Policy settings do not change this identity. The receipt retains Jev's confidence even when Sonnet makes the proposal. It prevents the same state being decided twice. The queued task ID is stored before daemon notification. The apply transaction checks that the input state still matches; stale answers are recorded but do not change work. When `member_id` is no longer a workspace member, the decision is recorded as failed with that reason and changes nothing.

- `fix_now`: set `todo`, assign the agent mapped by the issue's kind and queue its task after commit.
- `fix_later`: keep or place the issue in `backlog` and open a `review` to-do for Brian.
- `ready`: open one release-check `review` to-do for Brian, linked to a release issue and carrying `release_id` in details.
- `not_yet`: leave the release unchanged and comment with the reason.

An action, its audit comment, the receipt and any to-do row commit together. Comments do not wake the assignee. Realtime events publish after commit. Release decisions are recorded on the issue that caused the check.

## Brian's answers

During the start period, a proposal opens a single-choice question to-do for Brian. Its text names the maker, Jev's confidence, the proposed action and reason, and the agreement record. Brian selects the proposed action to agree or selects the other action to correct it. No work action or lead task starts until he answers.

After the start period, the server applies proposals without Brian. A workspace row lock and a persisted automatic sequence select exactly numbers 5, 10, 15 and so on for a question to-do after application. Confirming leaves the applied outcome in place. Correcting applies Brian's selected action, closes the opposite action's to-do and cancels only the lead task created by that receipt when needed. An active release must still be unshipped and have all its issues done before a release-check to-do can open.

Question answers, the selected action, task changes, to-do changes and audit comments commit together. Only Brian can answer these questions. They cannot be reassigned, withdrawn or dismissed as done without an answer. Edits append another agreed/corrected entry instead of overwriting the record. Each entry retains the proposal, maker, Jev confidence, Brian's choice and note. The next confirmation shows agreement counts so Brian can judge the threshold. Neither agreement nor feedback changes the policy by itself.

## Dependencies and operations

Apply migrations after PR 49. Configure settings through the workspace API. Brian alone ends the start period and tunes the threshold from the agreement record. No live rules, agents, secrets or issues are created by this change.
