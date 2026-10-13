import type { QueryClient } from "@tanstack/react-query";
import type { Question } from "../types/question";
import { issueKeys } from "../issues/queries";
import { questionKeys } from "./queries";

export function invalidateIssueQuestions(qc: QueryClient, wsId: string, issueId: string, questionId?: string) {
  return Promise.all([
    qc.invalidateQueries({ queryKey: questionKeys.byIssue(wsId, issueId) }),
    qc.invalidateQueries({
      queryKey: questionKeys.details(wsId),
      predicate: (query) => {
        if (questionId && query.queryKey[3] === questionId) return true;
        const question = query.state.data as Question | undefined;
        // An unresolved detail has no link information yet; refetching avoids
        // keeping an in-flight snapshot from before this event indefinitely.
        return !question || question.links.some((link) => link.link_type === "issue" && link.link_id === issueId);
      },
    }),
  ]);
}

/** Question and task events carry identities, not authoritative question data. */
export function handleQuestionEvent(qc: QueryClient, wsId: string, type: string, payload: unknown) {
  if (type !== "question:created" && type !== "question:updated" && !type.startsWith("task:")) return;
  if (!wsId || !payload || typeof payload !== "object") return;
  const fields = payload as Record<string, unknown>;
  if (typeof fields.issue_id !== "string" || !fields.issue_id) return;
  return Promise.all([
    invalidateIssueQuestions(qc, wsId, fields.issue_id,
      typeof fields.question_id === "string" ? fields.question_id : undefined),
    // Wake writes activity without publishing activity:created. Question
    // updates and issue task events are the live signal for other viewers.
    type !== "question:created"
      ? qc.invalidateQueries({ queryKey: issueKeys.timeline(fields.issue_id) })
      : undefined,
  ]);
}
