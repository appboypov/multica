"use client";

import type { Question } from "@multica/core/types";
import { placeKey, type QuestionPlace } from "../row-link";
import { WakeBars } from "../wake-bar";
import type { SessionEntry } from "./session";

export interface SessionWakeBarsProps {
  session: readonly SessionEntry[];
  questions: ReadonlyMap<string, Question>;
  /** The issue or chat whose bars come first: the current card's. */
  firstPlace: QuestionPlace;
}

/**
 * One by one's wake bars: per agent and issue or chat of the whole session,
 * the current card's place first, so a Wake stays in reach after Send moves
 * the flow elsewhere.
 */
export function SessionWakeBars({ session, questions, firstPlace }: SessionWakeBarsProps) {
  const places = new Map<string, { place: QuestionPlace; questions: Question[] }>();
  for (const entry of session) {
    const question = questions.get(entry.id);
    if (!question) continue;
    const key = placeKey(entry.place);
    const group = places.get(key);
    if (group) group.questions.push(question);
    else places.set(key, { place: entry.place, questions: [question] });
  }
  const firstKey = placeKey(firstPlace);
  const ordered = [...places.entries()].sort(([a], [b]) => Number(b === firstKey) - Number(a === firstKey));

  return (
    <div className="flex flex-col gap-2">
      {ordered.map(([key, group]) => (
        <WakeBars key={key} place={group.place} questions={group.questions} />
      ))}
    </div>
  );
}
