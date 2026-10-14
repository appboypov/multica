## ADDED Requirements

### Requirement: Wake the agent assignees of links and parents
Wherever **Wake** is offered, it SHALL be offered for the asking agent and for each agent assignee of the question's links and parents that has answers on those links it has not seen; a woken agent assignee SHALL get every answer given since its last run on those links.

#### Scenario: The parent's agent
- **GIVEN** Pieter asked Anouk a question on WORK-415, whose parent WORK-410 is assigned to the agent Joeri, and Anouk answered
- **WHEN** Brian looks at the answered card
- **THEN** it offers **Wake Pieter** and **Wake Joeri**

#### Scenario: Waking the parent's agent
- **GIVEN** the answered card offers **Wake Joeri**
- **WHEN** Brian presses it
- **THEN** a run of Joeri starts with Anouk's answer
- **AND** no run of Pieter is queued
