import { validateAnswer, type AnswerValidationError } from "@multica/core/questions";
import type {
  AnswerType,
  AnswerValue,
  AnswerValueByType,
  FactCheck,
  TypedQuestion,
} from "@multica/core/types";

/** A question this client can answer: one of the twelve known answer types. */
export type AnswerableQuestion = { [T in AnswerType]: TypedQuestion<T> }[AnswerType];

/**
 * What an answerer is composing, per answer type. It is the answer value,
 * except where an edit can stand before it is an answer: Confirm facts keeps
 * `null` for a fact not marked yet, Number `null` while the typed text is no
 * number, and Date only the ends picked so far.
 */
export interface AnswerDraftByType extends Omit<AnswerValueByType, "confirm_facts" | "number" | "date"> {
  confirm_facts: { facts: (FactCheck | null)[] };
  number: { number: number | null };
  date: { date?: string; datetime?: string; start?: string; end?: string };
}

/**
 * `null` while the answerer has not picked or typed anything. Once they have,
 * the draft stays a value even when they empty it again, so an emptied edit
 * never turns back into the suggestion.
 */
export type AnswerDraft<T extends AnswerType = AnswerType> = AnswerDraftByType[T] | null;

/**
 * The value **Send** would send: the draft, or for Ranking the order as shown,
 * since an untouched list is an answer too. `null` while there is nothing to
 * send: nothing picked, or a text, number, date or file list emptied again.
 */
export function valueToSend(question: AnswerableQuestion, draft: AnswerDraft): AnswerValue | null {
  if (draft === null) {
    return question.answer_type === "ranking" ? { order: [...question.settings.items] } : null;
  }
  switch (question.answer_type) {
    case "confirm_facts": {
      const facts = (draft as AnswerDraftByType["confirm_facts"]).facts;
      return facts.every((fact): fact is FactCheck => fact !== null) ? { facts } : null;
    }
    case "text":
      return (draft as AnswerDraftByType["text"]).text.trim() ? (draft as AnswerValue) : null;
    case "number":
      return (draft as AnswerDraftByType["number"]).number === null ? null : (draft as AnswerValue);
    case "file":
      return (draft as AnswerDraftByType["file"]).attachment_ids.length > 0 ? (draft as AnswerValue) : null;
    case "date":
      // One end of a range is sent to `validateAnswer`, so the card says the other is missing.
      return Object.values(draft).some((value) => value !== undefined) ? (draft as AnswerValue) : null;
    default:
      return draft as AnswerValue;
  }
}

/**
 * Why the draft cannot be sent, in the keys of `validateAnswer`; `null` when it
 * can or when nothing is picked yet. The server stays the authority.
 */
export function draftError(question: AnswerableQuestion, draft: AnswerDraft, note: string): AnswerValidationError | null {
  if (question.answer_type === "confirm_facts" && draft !== null) {
    const facts = (draft as AnswerDraftByType["confirm_facts"]).facts;
    const unmarked = facts.findIndex((fact) => fact === null);
    if (unmarked !== -1) return { key: "validation.everyFactRequired" };
  }
  const value = valueToSend(question, draft);
  return value === null ? null : validateAnswer(question, value, note);
}

/**
 * True when the control itself shows the answer's note: the reason of a No
 * when the asker wants one, and the note of Request changes. The card then
 * leaves out its own note field.
 */
export function controlShowsNote(question: AnswerableQuestion, draft: AnswerDraft): boolean {
  if (question.answer_type === "yes_no") {
    return question.settings.why_on_no && (draft as AnswerDraftByType["yes_no"] | null)?.choice === "no";
  }
  if (question.answer_type === "approval") {
    return (draft as AnswerDraftByType["approval"] | null)?.decision === "request_changes";
  }
  return false;
}

/**
 * Picks or unpicks a Multiple choice option, keeping the picks in the order of
 * the asker's options; unpicking the last one leaves an empty answer, which a
 * minimum of 0 accepts.
 */
export function toggleOption(
  question: TypedQuestion<"multiple_choice">,
  draft: AnswerDraftByType["multiple_choice"] | null,
  option: string,
): AnswerDraftByType["multiple_choice"] {
  const picked = draft?.options ?? [];
  const options = picked.includes(option)
    ? picked.filter((label) => label !== option)
    : question.settings.options.filter((label) => label === option || picked.includes(label));
  return { options };
}
