# MadSpec planning in Multica

MadSpec's `madspec-planning` skill names the planning concepts: organisations, projects, repositories, lanes, releases with milestones, and issues with their points, journey steps, tasks and client waits. This reference says how each of them is read and written in Multica, under the same headings.

## Connecting

Agents run `multica --profile skuddy <command>`: the account System, an admin of the workspace "De App Specialist", whose issue keys start with `WORK-`. Each command's `--help` is the source for its arguments and flags; read it before a command's first use. `--output json` gives the full record.

## Sessions

`multica agent sessions <alias>` lists an agent's active sessions. Before its first change to a repository or branch, a session runs it, and when another session of its agent works on that repository and branch, it coordinates with that session before the change. Reach a Herdr session through the omp messenger by its chat name, and a run through a comment on its issue. A session follows an issue with the omp tools `multica_issue_subscribe` and `multica_issue_listen`, and goes on once the issue is done.

## Scripts

MadSpec's `{planning-scripts}` is this skill's `scripts/` folder:

- `tickets.py` reads and writes a ticket tree through the `multica` CLI; its docstring names every verb and field. Run it with `uv run`.
- `labels.py` reports the ticket labels the workspace lacks, and creates them with `--create`.
- `team_state.py` collects the open work of one endeavour: Multica, its repositories, their pull requests and the client chats. `--intake <owner/name or path> --verifier <agent name>` instead ranks every open pull request of one repository with ready and blocked views. `--verifier` is the Multica agent name of the verifier. `--in-review` instead lists every In Review issue of the workspace with its project, release, lane, pull requests and place in its lane's stack, each stack bottom layer first, and each release whose issues are all done and whose release pull request is open, with its repository and pull request, and takes no `--verifier`.
  Intake retains closed issues' manual links. Missing claims block their PR; an unresolved verifier blocks selection.
- `words.ts` gives Brian's issue comments and question answers between `--start` and `--end`, through the `local` profile. Run it with `bun`.

## Questions

MadSpec's `madspec-inbox` reaches Brian with questions on his name on the issue of the work. In Multica:

- **Ask:** `multica question ask --on <issue> --answerer Brian`, all questions of one subject in one ask: `--prompt` for one question, or `--questions-file` with `--title` for several. Each question takes the answer type that fits it; `multica question ask --help` names the types and their flags. The output gives the ask's id and each question's id.
- **A pass:** `--type approval` with `--subject` naming the work and `--link-url` with `--link-title` linking it, such as its Review or pull request.
- **A to-do:** a question, or a sub-issue made with `issue create --parent <issue> --assignee Brian`.
- **Hear the answer:** a run ends once it has asked. Brian's answer starts a new run on the question's issue, which opens with an `[ANSWERS]` block. An omp session hears the answers to the questions it asked through the plugin `omp-multica-listen`, as the login its folder's `.multica-profile` names; each answer arrives as a message that starts a turn. `multica_question_subscribe` adds a question another session asked, and `multica_issue_subscribe` follows a sub-issue for Brian.
- **Read and close:** `question answers <id> --output json` reads his answer (`approve`, `request_changes` or `decline`), his note and the thread. `question close <id>` closes an answered question, and `question withdraw <id>` withdraws one that no longer needs his answer.

## Passes and gates

A pass is an approval question asked as "Questions" says, on the issue of the work it passes, such as the release's issue for a release. A release pass names the release in `--subject` by its version and repository. Gates in Anytype name the pass by its issue key and the question id the ask printed. A comment that links an Anytype object to its issue is made with `issue comment add`.

## Runs

- The daemon writes a block into a run's `AGENTS.md`, from `<!-- BEGIN MULTICA-RUNTIME` to `<!-- END MULTICA-RUNTIME -->`. It is no change of the work: it leaves the tree clean and stays uncommitted.
- An issue's runs and their messages: `multica issue runs` and `multica issue run-messages`.
- An agent is mentioned in a comment with `[@<name>](mention://agent/<id>)`, with its `name` and `id` from `agent list --output json`.

## Hierarchy

- The CLI has no organisation command; its API does it, on the profile's `server_url` with its `token` as Bearer token (both in `~/.multica/profiles/skuddy/config.json`): `GET /api/organisations` lists them. Organisations are made in the Multica app.
- A project's organisation is its `organisation_id` in `project list --output json`. `project create` makes a project; `PUT /api/projects/{id}` with `{"organisation_id": "<id>"}`, through the API as above, gives it its organisation.
- A repository is added with Add repository on the project in the Multica app, which checks its checkout on the machine that holds it. `repo add` only lists a URL in the workspace and puts no repository in a project. `worklane list --project` lists the lanes of every repository of a project.
- Lanes are Multica's worklanes.

## Issues

- **Types:** set once with `--type` on `issue create`. An epic's stories are made with `--parent` on the epic.
- **Statuses:** changed with `issue status`. `GET /api/issue-statuses`, reached as "Hierarchy" reaches organisations, gives each status with its description.
- **Runs:** an agent's Multica run starts when its issue leaves `backlog`, when the agent is assigned an issue outside `backlog`, on each comment on an issue it holds, when it is mentioned, and when a sub-issue of an issue it holds is done. A status change between the other statuses, such as from `in_progress` to `todo`, starts no run.
- **Points:** the workspace property `Points`, set with `issue property set` or `--property Points=<n>` on create.
- **Severity:** a bug's `--priority`.
- **Labels:** `label list`, added with `issue label add`.
- **Prerequisite across lanes and OpenSpec change:** the metadata keys `blocked_by` and `change`, set with `issue metadata set`.
- **Search before create.** `issue search` matches titles, descriptions and comments; with closed issues included it finds earlier work too. An issue is created with `issue create --project <project> --type <type> --title <title> --description-file <file>` and changed with `issue update`, which changes title, description, parent, priority, lane and release. Descriptions go in with `--description-file` or `--description-stdin`, never inline. `issue list --project` lists a project's issues, filtered by `--status`.
- An issue goes in its project with `--project`.
- An issue is read with `issue get <key> --output json`: its parent is its `parent_issue_id`, and `issue children <key>` lists its sub-issues.

## Tasks

- An issue is read with `issue get` and `issue comment list`.
- **RepoPrompt export:** `issue comment add <issue> --content "RepoPrompt export" --attachment <export path>`. The export lives outside the working directory, so the command needs the flag its `--help` names for an external file. The next self takes the attachment's id from `issue comment list <issue> --output json` and fetches it with `attachment download`.
- **Assignee:** set with `issue assign <issue> --to <name>`. Assigning an agent starts its Multica run, as "Runs" in "Issues" says. `--no-start` assigns without a run. A worker's sub-issue is made with `--parent` the issue, `--assignee` the worker and `--stage` its place in the order of the work.
- **Squads:** a MadSpec phase's lead is the squad that fills that phase. Work for a phase, a fix included, goes to that squad with `issue assign <issue> --to <squad name>`; Multica sends it to the squad's leader, who hands it to a member. An issue with an open pull request goes to the squad that fills the QA phase: its leader is the verifier, who reviews it first and hands each fix to a member of that squad. Only a sub-issue goes to one agent, its worker. `squad list` shows each squad with the phase it fills in its description.
- **Progress:** comments with `issue comment add`.

## Journeys and steps

- Build steps are made with `--parent` on the journey and `--stage` in step order. `issue children` lists them by stage.
- A pass is placed right before the first build step of its journeys with `worklane move`.

## Lanes

- Every lane other than `main` has its worktree, which Multica makes on its machine (`worklane create`). `worklane get <lane> --repo <owner/name> --output json` gives its `path`; lane `main` has none.
- An issue joins a lane with `--worklane` on `issue create` or `issue update`.
- `worklane issues <lane>` lists a lane's issues in order. `worklane move <issue> --before <issue>` or `--after` changes it.
- `worklane list` and `worklane get <lane>` show each lane Busy or Free, with its active and next issue.

## Releases

- The release line: `release list --repository`, `release get`, and `release plan`, `edit`, `move` and `remove`.
- An issue joins a release with `--release` on `issue create` or `issue update`, or with `release add-issues`.
- Milestones: `release milestone add`, `edit`, `remove`.
- Mark a published release shipped with `release ship <release>`; `release ship --help` gives its flags.

## Client waits

A client wait is an ordinary issue assigned to the client's own member. The client reads every open issue assigned to them in a project of their organisations.

- **The client's member:** the member with role `client` whose `organisation_ids` holds the organisation of the issue's project, in `workspace member list --output json`. The organisation of the issue is its project's `organisation_id`, as "Client updates" says.
- **Assign:** `issue assign <issue> --to <client's name>`.
- The due date is set with `--due-date`.

## External waits

An external wait is work that cannot go on until something outside its own issue is done: another issue's pull request, a verification, a repair or a person. Its age runs from its original start, never from a renewal, and while it still waits it reaches Skuddy at its deadline, the original start plus 4 hours. A client wait is its own issue, which the loop does not wait on, as MadSpec's `madspec-planning` "Client waits" says.

1. **Record.** On the waiting issue, at the first wait, comment `External wait` with the original start in UTC, the deadline, the outcome that is blocked, what it waits on with that work's owner, and the keys of the downstream issues that wait with it. That comment is the wait's source; a resumed or replacement wait reads it back.
2. **Wake.** Make one wakeup on the waiting issue with `issue wakeup create`: the condition that ends the wait, the absolute `--expires-at <deadline>` with `--on-timeout wake`, and an `--instruction` that names the source comment and the deadline. When another issue owns the dependency, its condition is `--until-issue <key> --until-issue-state done`; `--until-pr merged` fits only when the waiting issue itself has that linked pull request. `--expires-in` restarts on each re-enable, so it never sets this deadline.
3. **Resume.** On every resume, whatever woke it, read the dependency first. Done: end the wakeup that still runs (`issue wakeup delete`) and go on; the wait reaches nobody. Still open before the deadline: wait on, as step 4 says. Still open past it: escalate now, as step 5 says.
4. **Renew.** A renewed, extended or replaced wakeup, a new dependency for the same blocked outcome included, keeps the source comment's start and deadline: `issue wakeup update` with the same `--expires-at`. A wait already past its deadline, an in-flight one found late included, escalates now instead.
5. **Escalate.** In a Multica run, comment on the waiting issue with `[@Skuddy](mention://agent/<his id>)`, his id the `id` of Skuddy in `multica agent list --output json`, and the source comment's fields with the wait's age now. In an interactive session, send him the same in a private omp messenger message. Skuddy handles it as `~/Repos/Agents/skuddy/AGENTS.md` "External waits" says. The wait then goes on with its condition alone, without a second deadline.

The deadline wake relies on Multica's wakeup dispatch, repaired under WORK-1183. A run that finds a wait past its deadline still unresolved escalates it, however that run started.

## Client updates

The update writer works inside its own Multica run, so every command is `multica <command>` without `--profile`. The run sets `MULTICA_SERVER_URL`, `MULTICA_TOKEN` and `MULTICA_AGENT_ID`; the API is `curl -s "$MULTICA_SERVER_URL/api/..." -H "Authorization: Bearer $MULTICA_TOKEN"`.

- **The client of an issue:** its `project_id` in `issue get <issue> --output json`, that project's `organisation_id` in `project get <project> --output json`, and that organisation's key in `GET /api/organisations`.
- **Open approval questions:** `GET /api/questions?tab=all_open&asker_id=$MULTICA_AGENT_ID&answer_type=approval`, the rows whose `settings.subject` is `Client update <client name>`. Each row holds the draft as `prompt`, the receptionist task as `link.identifier`, and the work issue keys in `context_line`.
- **A visual:** an attachment of the issue or a comment is in the `attachments` of their `--output json`, fetched with `attachment download <id> -o attachments`. `--attachment` reads only files inside the run's working directory.
- **The receptionist's task:**

  ```bash
  multica issue create --project <client's project id> \
    --title "Post update <client name> <date and time> (<work issue key>)" --description-stdin [--attachment attachments/<file>] <<'EOF'
  CLIENT=<contact id>
  Channel: <channel title>

  ~~~text
  <the update, word for word>
  ~~~
  EOF
  ```

  `--attachment` uploads the visual and ends the description with its image line. A rewrite is `issue update <task> --description-stdin`, which replaces the description whole.
- **The approval question:**

  ```bash
  multica question ask --type approval --on <task> --link <work issue> \
    --subject "Client update <client name>" \
    --context "<client name> · <channel title> · <issue keys of the work it covers>" \
    --prompt "<the update, word for word>" --answerer Brian
  ```

  One `--link` per work issue the draft covers. `question withdraw <id>` withdraws it, `question answers <id> --output json` reads Brian's `approve`, `request_changes` or `decline`, and `question close <id>` closes it.
- **Approve:** `issue assign <task> --to <receptionist>`. **Decline:** `issue status <task> cancelled`.
- **The receptionist** is a member, not an agent, and works from her own session: `multica_inbox_listen` brings her new tasks, `multica_issue_list` with `assignee` her name lists the open ones, and `multica issue get <task> --output json` reads one. She downloads the attachment whose `id` the description's image link names with `multica attachment download <id> -o attachments`, comments with `multica_comment_add` and sets the task done with `multica_issue_status`.

## Improvement report

The report task: `issue create --title "Improvement report <YYYY-MM-DD>" --assignee Brian --project <MadSpec project id> --status todo --description-stdin`, with the description on stdin.
