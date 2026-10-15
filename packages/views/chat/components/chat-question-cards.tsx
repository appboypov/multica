"use client";

import type { QuestionCardGroup } from "@multica/core/questions";
import { cn } from "@multica/ui/lib/utils";
import { QuestionCard, QuestionSetCard } from "../../questions";

interface ChatQuestionCardsProps {
  /** The cards that follow one row, in the order they were asked. */
  cards: readonly QuestionCardGroup[];
  /** The chat they were asked in, whose questions refresh after each action. */
  chatSessionId: string;
  /** The question a `?question=` link landed on: its card is ringed and a set opens at it. */
  highlightedQuestionId: string | null;
}

/**
 * The question cards a chat turn ends with: the same card and set card as on
 * an issue, scoped to the chat, so the chat's member answers, cancels and
 * wakes in place and sees every other answerer's sent answer.
 */
export function ChatQuestionCards({ cards, chatSessionId, highlightedQuestionId }: ChatQuestionCardsProps) {
  const scope = { chatSessionId };
  return (
    <div className="mt-3 flex flex-col gap-3">
      {cards.map((card) => {
        const highlighted = !!highlightedQuestionId && card.questions.some((q) => q.id === highlightedQuestionId);
        return (
          <div
            key={card.askId}
            className={cn("rounded-xl transition-shadow duration-700", highlighted && "ring-2 ring-brand/40")}
            data-highlighted={highlighted || undefined}
          >
            {card.questions.length === 1 ? (
              <QuestionCard question={card.questions[0]!} scope={scope} />
            ) : (
              <QuestionSetCard
                questions={card.questions}
                scope={scope}
                activeQuestionId={highlighted ? highlightedQuestionId! : undefined}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}
