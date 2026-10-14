## ADDED Requirements

### Requirement: Wake within Answer all
In Answer all, the bar under the card in One by one and the end of each link's cards in One page SHALL offer **Wake** for each agent that has answers on that link it has not seen, naming how many, such as "Pieter has 1 answer from you on WORK-412".

#### Scenario: One by one
- **GIVEN** Brian just answered Pieter's question on WORK-412 in One by one
- **WHEN** the answer is sent
- **THEN** the bar under the card reads "Pieter has 1 answer from you on WORK-412" and offers **Wake Pieter**
- **AND** no run is queued for Pieter

#### Scenario: One page
- **GIVEN** Brian answered both of Pieter's questions on WORK-398 on One page
- **WHEN** Brian reaches the end of WORK-398's cards
- **THEN** **Wake Pieter** is offered there
- **AND** pressing it starts Pieter's run with both answers
