## At a glance

Stand-up, workcycle and check-out become issue types with their own template. An autopilot makes its issues with a type, and Issues filters, groups, sorts and shows issues by type.

## Why

Each organisation's work runs per release as a stand-up, a workcycle and a check-out. These issues were told apart by their title only. Success means Brian sees each as its own kind of issue, each starts from its own template, the 07:00 autopilots make stand-ups with the stand-up template, and Issues can be cut by type.

## What Changes

- Three types join the eight: `standup`, `workcycle` and `checkout`, each with an icon, a colour, a hint and a template.
- An autopilot takes an optional issue type. Its `create_issue` run makes a typed issue whose description holds the type's template.
- A migration types the existing stand-ups, workcycles and check-outs by their title, and the stand-up autopilots by their title template.
- Issues gets a Type filter, Type grouping, Type sorting and a Type column.

## Capabilities

### New Capabilities

- `issues-by-type`: Issues filters, groups, sorts and shows issues by type.
- `autopilot-issue-type`: an autopilot makes its issues with a type and its template.

### Modified Capabilities

- `issue-types`: an issue without a type offers the eleven types.
- `issue-templates`: New issue offers eleven templates, with the fields of the three new ones.
- `cli-issue-type-and-placement`: `--type` takes the eleven types.
- `worklane-views`: the worklane Type filter offers the eleven types.

## Impact

Server templates, the issue and autopilot handlers, the table query, the CLI, the web Issues page and its locales change. ADR-0032 records the decision.

## Constraints

- An issue's type never changes once set.
