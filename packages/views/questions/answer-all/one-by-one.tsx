"use client";

import { useEffect, useRef } from "react";
import { ChevronLeft, ChevronRight, SkipForward } from "lucide-react";
import type { Question } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";
import { QuestionCard } from "../question-card";
import { AnswerAllContext } from "./answer-all-context";
import { isPageKey, nextWaiting, type SessionEntry } from "./session";
import { SessionWakeBars } from "./wake-bars";

export interface OneByOneProps {
  session: readonly SessionEntry[];
  questions: ReadonlyMap<string, Question>;
  waiting: (id: string) => boolean;
  currentId: string;
  onCurrent: (id: string) => void;
  skipped: ReadonlySet<string>;
  onSkip: (id: string) => void;
}

/**
 * One question at a time as the focused card, with the issue's context and
 * **Up next** beside it. `J` next, `K` previous, `S` skip; after a send the
 * flow moves to the next question still waiting, wrapping to skipped ones.
 */
export function OneByOne({ session, questions, waiting, currentId, onCurrent, skipped, onSkip }: OneByOneProps) {
  const { t } = useT("questions");
  const index = Math.max(session.findIndex((entry) => entry.id === currentId), 0);
  const entry = session[index]!;
  const question = questions.get(entry.id);
  const ask = session.filter((other) => other.askId === entry.askId);

  const step = (delta: number) => onCurrent(session[(index + delta + session.length) % session.length]!.id);
  const skip = () => {
    onSkip(entry.id);
    const next = nextWaiting(session, index, (id) => id !== entry.id && waiting(id), new Set(skipped).add(entry.id));
    if (next >= 0) onCurrent(session[next]!.id);
  };

  // A send folds the current question: move on to the next one still waiting.
  // Moving to a question that was already answered moves nothing.
  const seen = useRef({ id: entry.id, waiting: waiting(entry.id) });
  useEffect(() => {
    const now = waiting(entry.id);
    if (seen.current.id === entry.id && seen.current.waiting && !now) {
      const next = nextWaiting(session, index, waiting, skipped);
      if (next >= 0) onCurrent(session[next]!.id);
    }
    seen.current = { id: entry.id, waiting: now };
  }, [entry.id, index, session, waiting, skipped, onCurrent]);

  const keys = useRef({ step, skip });
  keys.current = { step, skip };
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (!isPageKey(event) || event.shiftKey) return;
      const key = event.key.toLowerCase();
      if (key === "j") keys.current.step(1);
      else if (key === "k") keys.current.step(-1);
      else if (key === "s") keys.current.skip();
      else return;
      event.preventDefault();
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  const upNext = session
    .map((other, position) => ({ other, position }))
    .filter(({ other, position }) => position !== index && waiting(other.id))
    .sort((a, b) => ((a.position - index + session.length) % session.length) - ((b.position - index + session.length) % session.length))
    .map(({ other }) => ({ entry: other, question: questions.get(other.id) }));

  return (
    <div className="mx-auto grid w-full max-w-5xl gap-6 px-4 py-6 lg:grid-cols-[minmax(0,1fr)_18rem]">
      <div className="flex min-w-0 flex-col gap-3">
        {ask.length > 1 && (
          <div className="flex items-baseline gap-2">
            {entry.askTitle && <p className="min-w-0 truncate text-title-sm font-semibold">{entry.askTitle}</p>}
            <span className="shrink-0 text-caption text-muted-foreground">
              {t(($) => $.answer_all.set_position, { position: ask.indexOf(entry) + 1, total: ask.length })}
            </span>
          </div>
        )}
        {question && <QuestionCard key={question.id} question={question} issueId={entry.issueId} hideWake focused />}
        <div className="flex items-center gap-1">
          <Button type="button" variant="ghost" size="sm" onClick={() => step(-1)}>
            <ChevronLeft />
            {t(($) => $.answer_all.previous)}
          </Button>
          <Button type="button" variant="ghost" size="sm" onClick={() => step(1)}>
            {t(($) => $.answer_all.next)}
            <ChevronRight />
          </Button>
          {waiting(entry.id) && (
            <Button type="button" variant="ghost" size="sm" className="ml-auto" onClick={skip}>
              <SkipForward />
              {t(($) => $.answer_all.skip)}
            </Button>
          )}
        </div>
        <SessionWakeBars session={session} questions={questions} firstIssueId={entry.issueId} />
      </div>
      <AnswerAllContext entry={entry} question={question} upNext={upNext} onJump={onCurrent} />
    </div>
  );
}
