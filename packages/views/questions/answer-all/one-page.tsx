"use client";

import { useEffect, useRef, type FocusEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { issueDetailOptions } from "@multica/core/issues/queries";
import type { Question } from "@multica/core/types";
import { StatusIcon } from "../../issues/components/status-icon";
import { QuestionCard } from "../question-card";
import { QuestionSetCard } from "../question-set-card";
import { placeKey, placeScope, useChatTitle, type QuestionPlace } from "../row-link";
import { LinkIcon } from "../type-icon";
import { WakeBars } from "../wake-bar";
import { isPageKey, type SessionEntry } from "./session";

export interface OnePageProps {
  session: readonly SessionEntry[];
  questions: ReadonlyMap<string, Question>;
  focusedId: string;
  onFocus: (id: string) => void;
}

interface PlaceGroup {
  key: string;
  place: QuestionPlace;
  /** Runs of one ask: a set's questions together, a single question alone. */
  asks: SessionEntry[][];
}

/** The session grouped by issue or chat in first-seen order, a set's questions kept together. */
function groupByPlace(session: readonly SessionEntry[]): PlaceGroup[] {
  const groups = new Map<string, PlaceGroup>();
  for (const entry of session) {
    const key = placeKey(entry.place);
    let group = groups.get(key);
    if (!group) {
      group = { key, place: entry.place, asks: [] };
      groups.set(key, group);
    }
    const last = group.asks.at(-1);
    if (last && last[0]!.askId === entry.askId) last.push(entry);
    else group.asks.push([entry]);
  }
  return [...groups.values()];
}

/**
 * Every question of the session on one scrolling page, grouped by issue or
 * chat, each card answerable where it stands; a set shows stacked. `J`/`K`
 * move the focus, the focused card owns `⌘↵`, and each group ends with its
 * agents' wake bars.
 */
export function OnePage({ session, questions, focusedId, onFocus }: OnePageProps) {
  const groups = groupByPlace(session);

  const move = (delta: number) => {
    const index = Math.max(session.findIndex((entry) => entry.id === focusedId), 0);
    const next = session[Math.min(Math.max(index + delta, 0), session.length - 1)]!.id;
    onFocus(next);
    document.querySelector(`[data-question-id="${next}"]`)?.scrollIntoView({ block: "nearest", behavior: "smooth" });
  };
  const moveRef = useRef(move);
  moveRef.current = move;
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (!isPageKey(event) || event.shiftKey) return;
      const key = event.key.toLowerCase();
      if (key === "j") moveRef.current(1);
      else if (key === "k") moveRef.current(-1);
      else return;
      event.preventDefault();
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  // The page opens at the focused question, such as the one `?question=` asked for.
  const openedAtRef = useRef(focusedId);
  useEffect(() => {
    document.querySelector(`[data-question-id="${openedAtRef.current}"]`)?.scrollIntoView({ block: "center" });
  }, []);

  // Working inside a card focuses it, so `⌘↵` sends what is being typed.
  const focusFrom = (event: FocusEvent<HTMLDivElement>) => {
    const id = (event.target as HTMLElement).closest<HTMLElement>("[data-question-id]")?.dataset.questionId;
    if (id && id !== focusedId) onFocus(id);
  };

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-6" onFocusCapture={focusFrom}>
      {groups.map((group) => {
        const groupQuestions = group.asks.flat().flatMap((entry) => questions.get(entry.id) ?? []);
        const scope = placeScope(group.place);
        return (
          <section key={group.key} className="flex flex-col gap-3">
            {"issueId" in group.place ? (
              <IssueHeading issueId={group.place.issueId} identifier={group.place.identifier} />
            ) : (
              <ChatHeading memberId={group.place.memberId} />
            )}
            {group.asks.map((ask) => {
              const askQuestions = ask.flatMap((entry) => questions.get(entry.id) ?? []);
              if (askQuestions.length > 1) {
                return (
                  <QuestionSetCard
                    key={ask[0]!.id}
                    questions={askQuestions}
                    scope={scope}
                    layout="stacked"
                    focusedQuestionId={focusedId}
                  />
                );
              }
              const question = askQuestions[0];
              return question ? (
                <div key={question.id} data-question-id={question.id}>
                  <QuestionCard question={question} scope={scope} hideWake askQuestions={askQuestions} focused={question.id === focusedId} />
                </div>
              ) : null;
            })}
            <WakeBars place={group.place} questions={groupQuestions} />
          </section>
        );
      })}
    </div>
  );
}

/** An issue group's heading: status, identifier and title. */
function IssueHeading({ issueId, identifier }: { issueId: string; identifier: string }) {
  const wsId = useWorkspaceId();
  const { data: issue } = useQuery(issueDetailOptions(wsId, issueId));
  return (
    <div className="flex items-center gap-2 border-b pb-2 text-caption">
      {issue && <StatusIcon status={issue.status} className="size-3.5" />}
      <span className="shrink-0 font-medium text-muted-foreground">{identifier}</span>
      <span className="min-w-0 truncate font-medium">{issue?.title}</span>
    </div>
  );
}

/** A chat group's heading: "Chat with {member}", never the chat's title. */
function ChatHeading({ memberId }: { memberId: string | null }) {
  const chatTitle = useChatTitle();
  return (
    <div className="flex items-center gap-2 border-b pb-2 text-caption text-muted-foreground">
      <LinkIcon type="chat" />
      <span className="min-w-0 truncate font-medium text-foreground">{chatTitle(memberId)}</span>
    </div>
  );
}
