"use client";

import type { Question } from "@multica/core/types";
import { WakeBars } from "../wake-bar";
import type { SessionEntry } from "./session";

export interface SessionWakeBarsProps {
  session: readonly SessionEntry[];
  questions: ReadonlyMap<string, Question>;
  /** The issue whose bars come first: the current card's. */
  firstIssueId: string;
}

/**
 * One by one's wake bars: per agent and issue of the whole session, the
 * current card's issue first, so a Wake stays in reach after Send moves the
 * flow to another issue.
 */
export function SessionWakeBars({ session, questions, firstIssueId }: SessionWakeBarsProps) {
  const issues = new Map<string, { identifier: string; questions: Question[] }>();
  for (const entry of session) {
    const question = questions.get(entry.id);
    if (!question) continue;
    const issue = issues.get(entry.issueId);
    if (issue) issue.questions.push(question);
    else issues.set(entry.issueId, { identifier: entry.identifier, questions: [question] });
  }
  const ordered = [...issues.entries()].sort(([a], [b]) => Number(b === firstIssueId) - Number(a === firstIssueId));

  return (
    <div className="flex flex-col gap-2">
      {ordered.map(([issueId, issue]) => (
        <WakeBars key={issueId} issueId={issueId} identifier={issue.identifier} questions={issue.questions} />
      ))}
    </div>
  );
}
