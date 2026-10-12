## ADDED Requirements

### Requirement: Question sets
One ask SHALL hold one question or a question set of several questions, and each question in a set SHALL be answerable and sendable without answering the others.

#### Scenario: Three questions in one ask
- **GIVEN** Pieter runs on WORK-412
- **WHEN** Pieter asks three questions in one ask
- **THEN** WORK-412 holds one question set of three questions

#### Scenario: Answering one question of a set
- **GIVEN** a question set of three open questions for Brian
- **WHEN** Brian sends an answer to the second question only
- **THEN** the second question is answered and the first and third are still open

### Requirement: Open counts count questions
Every count of open questions SHALL count questions, not asks.

#### Scenario: A set of three
- **GIVEN** WORK-412 has one question set of three open questions and no other question
- **WHEN** Brian reads WORK-412's open question count
- **THEN** it is 3

### Requirement: Asking a squad
Asking a squad SHALL make each of its human members an answerer and MUST skip its agents.

#### Scenario: Squad of three members and two agents
- **GIVEN** squad Finance has the members Brian, Anouk and Roos and the agents Joeri and Thomas
- **WHEN** Pieter asks a question of squad Finance
- **THEN** its answerers are Brian, Anouk and Roos

### Requirement: Answer policy
A question with more than one answerer SHALL have the answer policy **Any one** or **Each**, picked by the asker. Under Any one, the first sent answer SHALL make the question answered and take it off the other answerers' open lists, and only the asker closes it. Under Each, the question SHALL be partly answered until every answerer has sent an answer, then answered.

#### Scenario: Any one
- **GIVEN** a question asked of Brian, Anouk and Roos under Any one
- **WHEN** Brian sends his answer
- **THEN** the question is answered and is no longer open for Anouk and Roos
- **AND** it is not closed, so Brian can still edit his answer

#### Scenario: Each
- **GIVEN** a question asked of Brian, Anouk and Roos under Each
- **WHEN** Brian and Anouk have sent their answers
- **THEN** the question is partly answered, its card shows "2 of 3", and it is still open for Roos

#### Scenario: Each, completed
- **GIVEN** a question under Each with two of three answers sent
- **WHEN** Roos sends her answer
- **THEN** the question is answered

### Requirement: Blocks work is on by default during a run
A question an agent asks during a run SHALL have Blocks work on unless the ask turns it off.

#### Scenario: Asked during a run
- **GIVEN** Pieter is in a run on WORK-412
- **WHEN** Pieter asks a question without saying anything about Blocks work
- **THEN** the question blocks work on WORK-412

#### Scenario: Turned off
- **GIVEN** Pieter is in a run on WORK-412
- **WHEN** Pieter asks a question with Blocks work off
- **THEN** the question does not block work on WORK-412
