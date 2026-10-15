import type { Question, QuestionWakeTarget } from "../types";

// The client mirror of ADR-0004: a question set groups the questions of one
// ask, and "open" is decided per question and per answerer. The server's
// `internal/question/policy.go` is the authority; every count and list of the
// clients uses these rules.

/** One card of an issue's timeline: the questions of one ask, in ask order. */
export interface QuestionCardGroup {
  /** The id of the ask's first question. */
  askId: string;
  questions: Question[];
}

/** Open: status `open` or `partly_answered`. */
export function isOpen(question: Question): boolean {
  return question.status === "open" || question.status === "partly_answered";
}

/**
 * Open for a member: open, the member is a current answerer, and the member
 * has no answer to it. Under Any one an answered question is open for nobody.
 */
export function isOpenFor(question: Question, userId: string | null | undefined): boolean {
  if (!userId || !isOpen(question)) return false;
  if (!question.answerers.some((answerer) => answerer.user_id === userId)) return false;
  return !question.answers.some((answer) => answer.answerer_user_id === userId);
}

/**
 * The member's last open question of a list (an ask's questions): every other
 * question of the list is not open for the member, so answering this one
 * answers all the member had open.
 */
export function isLastOpenFor(questions: readonly Question[], questionId: string, userId: string | null | undefined): boolean {
  return questions.every((question) => question.id === questionId || !isOpenFor(question, userId));
}

/** The number of open questions; a set of three open questions counts 3. */
export function countOpen(questions: readonly Question[]): number {
  return questions.filter(isOpen).length;
}

/**
 * A set's "N of M answered": the questions `answered` or `closed` over those
 * not withdrawn.
 */
export function setProgress(questions: readonly Question[]): { answered: number; total: number } {
  const counted = questions.filter((question) => question.status !== "withdrawn");
  return {
    answered: counted.filter((question) => question.status === "answered" || question.status === "closed").length,
    total: counted.length,
  };
}

/**
 * An issue's questions as timeline cards: grouped by `ask_id`, each group in
 * `ask_position` order, the groups in the order their first questions were
 * asked.
 */
export function groupQuestionCards(questions: readonly Question[]): QuestionCardGroup[] {
  const byAsk = new Map<string, Question[]>();
  for (const question of questions) {
    const group = byAsk.get(question.ask_id);
    if (group) group.push(question);
    else byAsk.set(question.ask_id, [question]);
  }
  const groups = [...byAsk].map(([askId, group]) => ({
    askId,
    questions: group.sort((a, b) => a.ask_position - b.ask_position || a.id.localeCompare(b.id)),
  }));
  return groups.sort((a, b) => {
    const left = a.questions[0]!;
    const right = b.questions[0]!;
    return left.created_at.localeCompare(right.created_at) || left.id.localeCompare(right.id);
  });
}

/** An agent offered Wake on an issue, with a question that offers it. */
export interface WakeOffer {
  question: Question;
  target: QuestionWakeTarget;
}

/**
 * Each (agent, issue) wake target of the questions that can be woken now, once,
 * in question order and the asking agent first within a question (design.md
 * decision 6 of UJ-4): one Wake bar or button each.
 */
export function wakeOffers(questions: readonly Question[]): WakeOffer[] {
  const offers = new Map<string, WakeOffer>();
  for (const question of questions) {
    for (const target of question.wake_targets) {
      const key = `${target.agent_id}:${target.issue_id}`;
      if (target.wake_available && !offers.has(key)) offers.set(key, { question, target });
    }
  }
  return [...offers.values()];
}
