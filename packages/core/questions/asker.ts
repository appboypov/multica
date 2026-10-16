// Who asked a question: an agent in a run, or a member outside one (ADR-0021).
// Every view names and draws the asker from this, never from `asker_agent_id`.

/** The asker of a question or row; `type` is the actor type views name and draw it with. */
export interface QuestionAsker {
  type: "agent" | "member";
  id: string;
}

/** The asking agent when the question has one, else the member who asked it outside a run. */
export function questionAsker(question: { asker_agent_id: string; asker_user_id: string }): QuestionAsker {
  return question.asker_agent_id
    ? { type: "agent", id: question.asker_agent_id }
    : { type: "member", id: question.asker_user_id };
}
