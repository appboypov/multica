## ADDED Requirements

### Requirement: A worklane's page opens on its timeline with its state
A worklane's page SHALL show the breadcrumb Projects › {project} › {owner/name} › {worklane}, the repository's `owner/name`, the worklane's state, and its Timeline, which SHALL say the worklane has no releases while its repository has none.

#### Scenario: A new worklane
- **GIVEN** Brian just created the worklane Fan app on appboypov/rebl of Rebl, which has no releases
- **WHEN** its page opens
- **THEN** it shows Projects › Rebl › appboypov/rebl › Fan app, appboypov/rebl and Free
- **AND** its Timeline says it has no releases yet

### Requirement: A worklane's page shows its activity
A worklane's page SHALL show the worklane's activity in the order it happened, each entry with who did it, what happened and when. Creating a worklane SHALL add the entry "{member} created {worklane} on {runtime}", as the member's yes to the worklane, and renaming it SHALL add "{member} renamed {old name} to {new name}".

#### Scenario: Created by Brian
- **GIVEN** Brian created the worklane Fan app on Brian's Mac at 12:40 on 13 October 2026
- **WHEN** he opens Fan app's page
- **THEN** its activity shows "Brian created Fan app on Brian's Mac" at 12:40 on 13 October 2026

#### Scenario: Renamed
- **GIVEN** Fan app's activity shows that Brian created it
- **WHEN** Brian renames Fan app to "Fan accounts"
- **THEN** its activity shows "Brian created Fan app on Brian's Mac" and then "Brian renamed Fan app to Fan accounts"

#### Scenario: No entry for a failed creation
- **GIVEN** creating the worklane Fan app failed at Install dependencies and Brian cancelled
- **WHEN** he creates Fan app again and every step passes
- **THEN** Fan app's activity shows one entry, "Brian created Fan app on Brian's Mac"
