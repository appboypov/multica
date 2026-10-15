import { isOpenFor } from "@multica/core/questions";
import { isEditableShortcutTarget, isPortalLayerShortcutTarget } from "@multica/core/shortcuts";
import type { Question, QuestionRow } from "@multica/core/types";
import { rowPlace, type QuestionPlace } from "../row-link";

/** One question of the frozen Answer all session. */
export interface SessionEntry {
  id: string;
  askId: string;
  askTitle: string | null;
  /** The issue or chat the question's card shows in. */
  place: QuestionPlace;
}

/** Freezes For me's rows, already in `sessionOrder`, into the session's entries. */
export function toSession(rows: readonly QuestionRow[]): SessionEntry[] {
  return rows.map((row) => ({ id: row.id, askId: row.ask_id, askTitle: row.ask_title, place: rowPlace(row) }));
}

/** Still waiting on the viewer: open for them and not yet answered by them. */
export function isWaiting(question: Question | undefined, userId: string | undefined): boolean {
  return !!question && isOpenFor(question, userId);
}

/** The viewer has an answer on the question. */
export function isAnsweredBy(question: Question | undefined, userId: string | undefined): boolean {
  return !!question && !!userId && question.answers.some((a) => a.answerer_user_id === userId);
}

/**
 * The next question still waiting after `from`, wrapping around: questions not
 * skipped first, then the skipped ones. -1 when nothing waits.
 */
export function nextWaiting(
  session: readonly SessionEntry[],
  from: number,
  waiting: (id: string) => boolean,
  skipped: ReadonlySet<string>,
): number {
  const order = session.map((_, offset) => (from + 1 + offset) % session.length);
  const fresh = order.find((index) => waiting(session[index]!.id) && !skipped.has(session[index]!.id));
  if (fresh !== undefined) return fresh;
  return order.find((index) => waiting(session[index]!.id)) ?? -1;
}

/** A plain key press meant for the page: no modifier, not typed into a field or a popup. */
export function isPageKey(event: KeyboardEvent): boolean {
  return !event.defaultPrevented && !event.metaKey && !event.ctrlKey && !event.altKey &&
    !isEditableShortcutTarget(event.target) && !isPortalLayerShortcutTarget(event.target);
}
