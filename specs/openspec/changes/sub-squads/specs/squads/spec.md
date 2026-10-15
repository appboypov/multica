## ADDED Requirements

### Requirement: A squad holds squads as members
A squad member SHALL be an agent, a workspace member or a squad of the same workspace. One squad MAY be a member of several squads, and nesting SHALL have no depth limit.

#### Scenario: Adding a squad as a member
- **GIVEN** squads Tech, led by Mitchel, and Backend, led by Joeri, in one workspace
- **WHEN** Brian adds Backend to Tech with member type `squad`
- **THEN** Tech's member list holds a row of type `squad` for Backend

#### Scenario: A squad in two squads
- **GIVEN** Backend is a member of Tech, and squad Platform is led by Pieter
- **WHEN** Brian adds Backend to Platform
- **THEN** Backend is a member of both Tech and Platform

#### Scenario: A squad of another workspace or an archived squad
- **GIVEN** squad Ops of another workspace, and squad Legacy that is archived
- **WHEN** Brian adds Ops or Legacy to Tech
- **THEN** the add is refused as a squad not found in this workspace

#### Scenario: A creator adds a squad whose leader they cannot run
- **GIVEN** Anouk, a regular member, created Tech, and squad Private is led by an agent Anouk cannot run
- **WHEN** Anouk adds Private to Tech
- **THEN** the add is refused, as adding that agent would be

### Requirement: Nesting never makes a loop
Adding a squad as a member SHALL be refused when the main squad is the added squad or sits in it at any depth, and the refusal SHALL name the loop from the main squad through each squad back to it.

#### Scenario: A direct loop
- **GIVEN** Backend is a member of Tech
- **WHEN** Brian adds Tech to Backend
- **THEN** the add is refused with "adding Tech would create a loop: Backend → Tech → Backend"

#### Scenario: A loop through several squads
- **GIVEN** Backend is a member of Tech and Data is a member of Backend
- **WHEN** Brian adds Tech to Data
- **THEN** the add is refused with "adding Tech would create a loop: Data → Tech → Backend → Data"

#### Scenario: A squad in itself
- **WHEN** Brian adds Tech to Tech
- **THEN** the add is refused with "adding Tech would create a loop: Tech → Tech"

#### Scenario: Two adds at the same time
- **GIVEN** Backend is a member of Tech, and Data is a member of Ops
- **WHEN** Brian adds Ops to Backend while Anouk adds Tech to Data
- **THEN** one add succeeds and the other is refused as a loop

### Requirement: A sub-squad never shares its leader with its main squad
A squad SHALL NOT be a member of a squad with the same leader agent. Adding such a squad SHALL be refused, and changing a squad's leader SHALL be refused when the new leader leads a non-archived squad it is a member of or a non-archived squad that is its member.

#### Scenario: Adding a squad with the same leader
- **GIVEN** squads Tech and Design are both led by Mitchel
- **WHEN** Brian adds Design to Tech
- **THEN** the add is refused because Design has the same leader as Tech

#### Scenario: Changing the sub-squad's leader to the main leader
- **GIVEN** Backend, led by Joeri, is a member of Tech, led by Mitchel
- **WHEN** Brian makes Mitchel the leader of Backend
- **THEN** the change is refused because Backend sits in Tech, which Mitchel leads

#### Scenario: Changing the main squad's leader to the sub-leader
- **GIVEN** Backend, led by Joeri, is a member of Tech, led by Mitchel
- **WHEN** Brian makes Joeri the leader of Tech
- **THEN** the change is refused because Backend, a member of Tech, is led by Joeri

### Requirement: The leader's roster lists sub-squads
The Squad Roster in a leader's briefing SHALL list each non-archived sub-squad with its name, the kind `squad`, its role when set and its mention `[@<name>](mention://squad/<id>)`. An archived sub-squad SHALL be left out.

#### Scenario: Roster row of a sub-squad
- **GIVEN** Backend is a member of Tech with role "server work"
- **WHEN** Mitchel claims a leader run of Tech
- **THEN** its roster holds one row for Backend with the kind `squad`, the role "server work" and the mention `[@Backend](mention://squad/<Backend id>)`, in the same row format as an agent member

#### Scenario: Archived sub-squad
- **GIVEN** Backend is a member of Tech and Backend is archived
- **WHEN** Mitchel claims a leader run of Tech
- **THEN** the roster has no row for Backend

### Requirement: A sub-squad's report wakes the leader that delegated to it
When a run of a squad's leader in its leader role posts a comment on an issue that @mentions nobody, the leader of each non-archived squad that has that squad as a member and whose current leader, in a leader run, posted a comment on the issue that @mentions that squad SHALL get a leader run of its squad, as a worker's result wakes its leader today. When that squad is the issue's assignee, today's assignee route SHALL wake its leader instead, once.

#### Scenario: Main squad is a guest on the issue
- **GIVEN** WORK-12 is assigned to Brian, Mitchel as Tech's leader @mentioned Backend on it, and Joeri ran as Backend's leader
- **WHEN** Joeri, in a Backend leader run, posts "Backend is done" without a mention
- **THEN** a leader run of Tech is queued for Mitchel

#### Scenario: Main squad is the assignee
- **GIVEN** WORK-12 is assigned to Tech and Mitchel @mentioned Backend on it
- **WHEN** Joeri, in a Backend leader run, posts "Backend is done" without a mention
- **THEN** exactly one leader run of Tech is queued for Mitchel

#### Scenario: No delegation from the main squad
- **GIVEN** Backend is a member of Tech, WORK-12 is assigned to Brian, and Brian @mentioned Backend himself
- **WHEN** Joeri, in a Backend leader run, posts "Backend is done" without a mention
- **THEN** no run of Mitchel is queued

#### Scenario: Report that names someone
- **GIVEN** Mitchel @mentioned Backend on WORK-12
- **WHEN** Joeri, in a Backend leader run, posts a comment that @mentions a Backend worker
- **THEN** only that worker gets a run, and Mitchel does not

### Requirement: Direct members only outside the roster
@squad inbox notices, squad questions and "involves me" filters SHALL reach the direct agent and member rows of the named squad only, and SHALL NOT look into its sub-squads.

#### Scenario: Notice for a squad with a sub-squad
- **GIVEN** Tech has member Brian and sub-squad Backend with member Anouk
- **WHEN** someone @mentions Tech on an issue
- **THEN** Brian gets the inbox notice and Anouk does not
