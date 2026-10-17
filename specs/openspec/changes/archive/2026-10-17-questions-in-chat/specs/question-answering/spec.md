## ADDED Requirements

### Requirement: Cancel a question
Every card that offers a current answerer answer controls SHALL also offer **Cancel question**, with an optional note as its reason; a cancel SHALL end the question for every answerer, take it off every open list and count, and MUST leave the answers already sent readable.

#### Scenario: Brian cancels a question asked of three
- **GIVEN** Pieter asked Brian, Anouk and Roos one question on WORK-412 under Each and Anouk already answered
- **WHEN** Brian cancels it with the note "already decided in the PRD"
- **THEN** the card reads "Cancelled by Brian" with the note and offers no answer controls to anyone
- **AND** the question leaves the For me lists and sidebar counts of Brian and Roos
- **AND** Anouk's answer is still shown on the card

#### Scenario: Cancelling without a note
- **GIVEN** Pieter's question on WORK-412 is open for Brian
- **WHEN** Brian cancels it without a note
- **THEN** the card reads "Cancelled by Brian"

#### Scenario: Pieter reads the cancel
- **GIVEN** Brian cancelled Pieter's question with the note "already decided in the PRD"
- **WHEN** Pieter reads the question through `multica question`
- **THEN** the output shows the question cancelled, by Brian, with that note and when

#### Scenario: A member who is not an answerer
- **GIVEN** Pieter asked Brian a question on WORK-412
- **WHEN** Anouk, who is not an answerer, opens WORK-412
- **THEN** the card offers her no **Cancel question**

#### Scenario: A cancelled question stays ended
- **GIVEN** Brian cancelled Pieter's question
- **WHEN** Brian opens its card
- **THEN** it offers no answer controls and no **Edit answer**
- **AND** Pieter can neither close nor withdraw it
