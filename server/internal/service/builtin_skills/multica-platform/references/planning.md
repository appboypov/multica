# Planning in Multica

Multica is the task board: it holds the tasks of the work, their runs and squads, the questions to Brian and the client updates. This reference says how each of them is read and written, under the same headings.

## Connecting

Agents run `multica --profile skuddy <command>`: the account System, an admin of the workspace "De App Specialist", whose issue keys start with `WORK-`. Each command's `--help` is the source for its arguments and flags; read it before a command's first use. `--output json` gives the full record.

## Sessions

`multica agent sessions <alias>` lists an agent's active sessions. Before its first change to a repository or branch, a session runs it, and when another session of its agent works on that repository and branch, it coordinates with that session before the change. Reach a Herdr session through the omp messenger by its chat name, and a run through a comment on its issue. A session follows an issue with the omp tools `multica_issue_subscribe` and `multica_issue_listen`, and goes on once the issue is done.

## Questions

Questions reach Brian on his name on the issue of the work:

- **Ask:** `multica question ask --on <issue> --answerer Brian`, all questions of one subject in one ask: `--prompt` for one question, or `--questions-file` with `--title` for several. Each question takes the answer type that fits it; `multica question ask --help` names the types and their flags. The output gives the ask's id and each question's id.
- **A to-do:** a question, or a sub-issue made with `issue create --parent <issue> --assignee Brian`.
- **Hear the answer:** a run ends once it has asked. Brian's answer starts a new run on the question's issue, which opens with an `[ANSWERS]` block. An omp session hears the answers to the questions it asked through the plugin `omp-multica-listen`, as the login its folder's `.multica-profile` names; each answer arrives as a message that starts a turn. `multica_question_subscribe` adds a question another session asked, and `multica_issue_subscribe` follows a sub-issue for Brian.
- **Read and close:** `question answers <id> --output json` reads his answer (`approve`, `request_changes` or `decline`), his note and the thread. `question close <id>` closes an answered question, and `question withdraw <id>` withdraws one that no longer needs his answer.

## Runs

- The daemon writes a block into a run's `AGENTS.md`, from `<!-- BEGIN MULTICA-RUNTIME` to `<!-- END MULTICA-RUNTIME -->`. It is no change of the work: it leaves the tree clean and stays uncommitted.
- An issue's runs and their messages: `multica issue runs` and `multica issue run-messages`.
- An agent is mentioned in a comment with `[@<name>](mention://agent/<id>)`, with its `name` and `id` from `agent list --output json`.

## Hierarchy

- The CLI has no organisation command; its API does it, on the profile's `server_url` with its `token` as Bearer token (both in `~/.multica/profiles/skuddy/config.json`): `GET /api/organisations` lists them. Organisations are made in the Multica app.
- A project's organisation is its `organisation_id` in `project list --output json`. `project create` makes a project; `PUT /api/projects/{id}` with `{"organisation_id": "<id>"}`, through the API as above, gives it its organisation.
- A repository is added with Add repository on the project in the Multica app, which checks its checkout on the machine that holds it. `repo add` only lists a URL in the workspace and puts no repository in a project.

## Issues

- **Types:** set once with `--type` on `issue create`. An epic's stories are made with `--parent` on the epic.
- **Statuses:** changed with `issue status`. `GET /api/issue-statuses`, reached as "Hierarchy" reaches organisations, gives each status with its description.
- **Runs:** an agent's Multica run starts when its issue leaves `backlog`, when the agent is assigned an issue outside `backlog`, on each comment on an issue it holds, when it is mentioned, and when a sub-issue of an issue it holds is done. A status change between the other statuses, such as from `in_progress` to `todo`, starts no run.
- **Search before create.** `issue search` matches titles, descriptions and comments; with closed issues included it finds earlier work too. An issue is created with `issue create --project <project> --type <type> --title <title> --description-file <file>` and changed with `issue update`, which changes title, description, parent and priority. Descriptions go in with `--description-file` or `--description-stdin`, never inline. `issue list --project` lists a project's issues, filtered by `--status`.
- An issue goes in its project with `--project`.
- An issue is read with `issue get <key> --output json`: its parent is its `parent_issue_id`, and `issue children <key>` lists its sub-issues.

## Tasks

- An issue is read with `issue get` and `issue comment list`.
- **Assignee:** set with `issue assign <issue> --to <name>`. Assigning an agent starts its Multica run, as "Runs" in "Issues" says. `--no-start` assigns without a run. A worker's sub-issue is made with `--parent` the issue and `--assignee` the worker.
- **Squads:** a work phase's lead is the squad that fills that phase. Work for a phase, a fix included, goes to that squad with `issue assign <issue> --to <squad name>`; Multica sends it to the squad's leader, who hands it to a member. An issue with an open pull request goes to the squad that fills the Verify phase: its leader is the verifier, who reviews it first and hands each fix to a member of that squad. Only a sub-issue goes to one agent, its worker. `squad list` shows each squad with the phase it fills in its description.
- **Progress:** comments with `issue comment add`.

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
