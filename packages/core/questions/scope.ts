import type { QuestionWakeTarget } from "../types/question";

/**
 * Where a question's card shows and its caches live: the issue it is on, or
 * the chat it was asked in (ADR-0018).
 */
export type QuestionScope = { issueId: string } | { chatSessionId: string };

/** The scope a wake target's agent wakes in: its chat, else its issue. */
export function wakeTargetScope(target: QuestionWakeTarget): QuestionScope {
  return target.chat_session_id !== null ? { chatSessionId: target.chat_session_id } : { issueId: target.issue_id };
}
