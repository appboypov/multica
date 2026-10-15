## ADDED Requirements

### Requirement: The workspace default decides questions in a chat
Whether an agent may ask questions during a chat turn SHALL follow the workspace Questions default, decided when the turn is claimed; a chat MUST NOT have a Questions setting of its own.

#### Scenario: Default On
- **GIVEN** the workspace Questions default is On
- **WHEN** Pieter's turn in Brian's chat starts
- **THEN** the turn's brief holds the Questions section and `multica --help` lists `multica question`

#### Scenario: Default Off
- **GIVEN** the workspace Questions default is Off
- **WHEN** Pieter's turn in Brian's chat starts and Pieter runs `multica question ask`
- **THEN** the brief has no Questions section and the ask is refused with nothing posted

#### Scenario: A change applies from the next turn
- **GIVEN** Pieter's turn in Brian's chat is running with questions on
- **WHEN** Brian turns the workspace Questions default off
- **THEN** the running turn is unchanged and Pieter's next turn in that chat has questions off

### Requirement: A question asked in a chat links to the chat
A question asked during a chat turn SHALL link to that chat and to the turn's run, also when the ask names other links.

#### Scenario: Asked without links
- **GIVEN** Pieter is in a turn of Brian's chat
- **WHEN** Pieter asks Brian a question without `--on` or `--link`
- **THEN** the question's first link is Brian's chat and it also links to the run

#### Scenario: Asked on an issue from a chat
- **GIVEN** Pieter is in a turn of Brian's chat
- **WHEN** Pieter asks a question with `--on WORK-412`
- **THEN** the question links to WORK-412 first, then to Brian's chat and the run
- **AND** it shows on WORK-412 and in Brian's chat

### Requirement: An agent in a chat asks any member or squad
An agent in a chat turn SHALL ask the same answerers as on an issue: one or more members, a squad, or both, under the answer policy Any one or Each.

#### Scenario: Asking Anouk from Brian's chat
- **GIVEN** Pieter is in a turn of Brian's chat
- **WHEN** Pieter asks Anouk a question
- **THEN** Anouk is the question's answerer and the question shows in Brian's chat

### Requirement: Question cards in the chat
A chat SHALL show each question and question set asked in it as the same card as on an issue, at the place in the conversation where it was asked, with the same answer controls for its answerers and the "Question for you" overline when the chat's member is an answerer; the chat SHALL show every sent answer, also those of other members.

#### Scenario: A set of three in the chat
- **GIVEN** Pieter asks Brian a question set of three in Brian's chat
- **WHEN** Brian looks at the chat
- **THEN** after Pieter's message the chat shows one set card of three questions, each answerable on its own, with the overline "Question for you"

#### Scenario: Anouk's answer shows in Brian's chat
- **GIVEN** Pieter asked Anouk a question in Brian's chat
- **WHEN** Anouk sends "per line" from her Questions page
- **THEN** Brian's chat shows the card folded to "per line", Anouk and the time
- **AND** offers Brian no answer controls

### Requirement: Waking a chat question's agent continues the chat
Every way to wake the asking agent of a question asked in a chat (sending or cancelling with **Wake {agent}** checked, pressing **Wake**, asking back) SHALL start the agent's next turn in that chat, never a run elsewhere, with every answer, ask back and cancel of the chat's questions it has not seen; an answer, edit or cancel without Wake MUST NOT start a turn.

#### Scenario: Answering the last question with Wake checked
- **GIVEN** Pieter asked Brian a question set of three in Brian's chat, Brian sent two answers and Pieter's turn has ended
- **WHEN** Brian sends the answer to the third question with **Wake Pieter** checked
- **THEN** Pieter's next turn in Brian's chat starts with all three answers in its input

#### Scenario: Answering the first of three
- **GIVEN** Pieter asked Brian a question set of three in Brian's chat
- **WHEN** Brian sends the answer to the first question
- **THEN** no turn is queued for Pieter

#### Scenario: Wake unchecked
- **GIVEN** Pieter asked Brian one question in Brian's chat
- **WHEN** Brian unchecks **Wake Pieter** and sends his answer
- **THEN** no turn is queued and the folded card in the chat offers **Wake Pieter**
- **AND** pressing it starts Pieter's next turn in Brian's chat with the answer

#### Scenario: Two answers and a cancel
- **GIVEN** Pieter asked Brian three questions in Brian's chat and Brian answered two
- **WHEN** Brian cancels the third with **Wake Pieter** checked
- **THEN** Pieter's next turn in Brian's chat starts with the two answers and the cancel

#### Scenario: Cancel with Wake unchecked
- **GIVEN** Pieter asked Brian three questions in Brian's chat and Brian answered two
- **WHEN** Brian unchecks **Wake Pieter** and cancels the third
- **THEN** no turn is queued until Brian presses **Wake Pieter** or sends a chat message

#### Scenario: Ask back in a chat
- **GIVEN** Pieter's question in Brian's chat is open and Pieter's turn has ended
- **WHEN** Brian asks back "Which customer is this for?"
- **THEN** Pieter's next turn in Brian's chat starts with the ask back in its input

#### Scenario: Anouk wakes Pieter from Answer all
- **GIVEN** Pieter asked Anouk a question in Brian's chat and Anouk answered it in Answer all
- **WHEN** Anouk presses **Wake Pieter**
- **THEN** Pieter's next turn in Brian's chat starts with her answer

#### Scenario: The agent is still in a turn
- **GIVEN** Pieter is running a turn in Brian's chat and asked one question in it
- **WHEN** Brian answers it with **Wake Pieter** checked before the turn ends
- **THEN** no extra turn is queued and the answer rides along in Pieter's next woken turn in the chat

### Requirement: A chat message leaves open questions open
A chat message the member sends while questions of the chat are open SHALL start the agent's turn as any chat message does and MUST leave those questions open; the agent SHALL still read, close and withdraw them through `multica question`.

#### Scenario: Brian types instead of answering
- **GIVEN** Pieter's question in Brian's chat is open
- **WHEN** Brian sends the chat message "Use per line for now"
- **THEN** Pieter's next turn starts with that message and the question is still open
- **AND** Pieter can read and withdraw it through `multica question`

### Requirement: Chat questions outside the chat
Questions asked in a chat SHALL count and list on the Questions page, in the sidebar count, in Answer all and in the inbox as questions asked on issues do; opening one SHALL open the chat at its card for the chat's member, and the question in Answer all for every other member.

#### Scenario: Brian opens his chat question from the Questions page
- **GIVEN** Pieter asked Brian a question in Brian's chat
- **WHEN** Brian opens its row on the Questions page under For me
- **THEN** Brian's chat opens at the question's card

#### Scenario: Sidebar count
- **GIVEN** Brian has 2 open questions on issues and 1 open question in his chat
- **WHEN** Brian looks at the sidebar
- **THEN** the Questions item shows 3

#### Scenario: Inbox item
- **GIVEN** Pieter asks Brian a question in Brian's chat
- **WHEN** the question is posted
- **THEN** Brian's inbox gets an item for it that opens his chat at the card

#### Scenario: Anouk answers a question from Brian's chat
- **GIVEN** Pieter asked Anouk a question in Brian's chat
- **WHEN** Anouk opens its row on her Questions page
- **THEN** the question opens in Answer all, where she can answer it

#### Scenario: Overdue in a chat
- **GIVEN** Brian's open question in his chat has due date 2026-10-14
- **WHEN** it is 2026-10-15 and the question is still open
- **THEN** Brian's inbox gets an overdue item for it

### Requirement: The chat stays private
Multica MUST NOT show a chat's title or messages to anyone but the chat's member, also when another member answers or follows a question asked in it.

#### Scenario: Anouk sees the question, not the chat
- **GIVEN** Pieter asked Anouk a question in Brian's chat
- **WHEN** Anouk reads the question on her Questions page or in her inbox
- **THEN** she sees the prompt, its context line, Pieter as asker and that it was asked in a chat with Brian
- **AND** she sees neither the chat's title nor its messages

### Requirement: Deleting a chat withdraws its questions
Deleting a chat SHALL withdraw each open or answered question asked in it that links to no other remaining object.

#### Scenario: Brian deletes the chat
- **GIVEN** Pieter's open question in Brian's chat links only to that chat and its run
- **WHEN** Brian deletes the chat
- **THEN** the question is withdrawn and leaves every open list

#### Scenario: Also on an issue
- **GIVEN** Pieter asked a question from Brian's chat with `--on WORK-412`
- **WHEN** Brian deletes the chat
- **THEN** the question stays on WORK-412
