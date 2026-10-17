## ADDED Requirements

### Requirement: Leader chooses each next step by identity and capability

On every trigger, a squad leader SHALL read the issue and its Agent Identity, including its persona skill. It SHALL do steps that belong to its own role in the current turn within its identity limits. It SHALL delegate a next step that fits a squad member better through the existing roster mention path. It MAY do its own step and delegate the next step in the same turn.

#### Scenario: Next step belongs to the leader

- **GIVEN** the issue's next step belongs to the leader's persona role
- **WHEN** the leader evaluates the trigger
- **THEN** the protocol directs it to do that step in the current turn
- **AND** it assesses the following step using its role and the roster

#### Scenario: Member fits the next step better

- **GIVEN** a roster member fits the next step better than the leader
- **WHEN** the leader chooses who takes that step
- **THEN** it delegates with the exact roster mention markdown
- **AND** its comment stays short and does not repeat issue context

#### Scenario: Own work followed by delegation

- **GIVEN** the leader completes its own role step and a member fits the following step better
- **WHEN** the leader hands off that step
- **THEN** one comment carries its result and the delegation
- **AND** it records squad activity and ends the turn after delegation

### Requirement: Own work without delegation uses normal delivery

The delegation stop rule SHALL apply only after delegation. Without delegation, a leader SHALL deliver its own result as a comment and follow the issue workflow's status rules within its status authority. It SHALL NOT have to wait for a member trigger to apply the completion rule when its own work meets the overall goal.

#### Scenario: Own work completes an owned issue

- **GIVEN** the issue is assigned to the leader's squad
- **AND** its own work meets the overall goal without delegation
- **WHEN** it delivers the result
- **THEN** it uses `in_review` while awaiting acceptance or may use `done` when finished and awaiting nobody
- **AND** Agent Identity limits still apply

#### Scenario: Guest leader does its own work

- **GIVEN** the squad was mentioned on an issue assigned to someone else
- **WHEN** its leader delivers its own role step
- **THEN** the leader posts the result without changing the issue status

### Requirement: Leader activity and routing boundaries stay in force

The leader SHALL record squad activity on every turn. The protocol's `no_action` silent exit and activity error fallback SHALL remain in force. A dispatch turn SHALL keep an owned parent `in_progress`. Member updates, member completion, and mentions SHALL keep their existing trigger behavior. Member instructions SHALL remain unchanged.

#### Scenario: No response is needed

- **GIVEN** a member update needs no action
- **WHEN** the leader records `no_action` successfully
- **THEN** it exits without a comment

#### Scenario: Activity recording fails after delivery

- **GIVEN** the leader has posted a result or delegation comment
- **WHEN** recording squad activity fails
- **THEN** that comment remains the record and it does not post a second comment for the error

#### Scenario: Delegated work remains underway

- **GIVEN** the squad owns the issue and the leader delegates work
- **WHEN** the dispatch turn ends
- **THEN** the parent remains `in_progress`
- **AND** member updates, member completion, and mentions can re-trigger the leader under the existing rules
