"use client";

import type { QuestionCardGroup } from "@multica/core/questions";
import { cn } from "@multica/ui/lib/utils";
import { QuestionCard, QuestionSetCard } from "../../questions";

interface ChatQuestionCardsProps {
  /** The cards that follow one row, in the order they were asked. */
  cards: readonly QuestionCardGroup[];
  /** The chat they were asked in, whose questions refresh after each action. */
  chatSessionId: string;
  /** The question a `?question=` link points at: a set card opens at it. */
  landingQuestionId: string | null;
  /** The question just landed on: its card is ringed until the ring fades. */
  highlightedQuestionId: string | null;
}

/**
 * Brings the landed card itself into view once it renders: the list scrolls
 * to the end of the card's row, and a row with several cards can leave an
 * earlier one offscreen.
 */
function revealCard(node: HTMLDivElement | null) {
  node?.scrollIntoView?.({ block: "nearest" });
}

/**
 * The question cards a chat turn ends with: the same card and set card as on
 * an issue, scoped to the chat, so the chat's member answers, cancels and
 * wakes in place and sees every other answerer's sent answer.
 */
export function ChatQuestionCards({ cards, chatSessionId, landingQuestionId, highlightedQuestionId }: ChatQuestionCardsProps) {
  const scope = { chatSessionId };
  return (
    <div className="mt-3 flex flex-col gap-3">
      {cards.map((card) => {
        const highlighted = card.questions.some((q) => q.id === highlightedQuestionId);
        const landing = card.questions.some((q) => q.id === landingQuestionId);
        return (
          <div
            key={card.askId}
            className={cn("rounded-xl transition-shadow duration-700", highlighted && "ring-2 ring-brand/40")}
            data-highlighted={highlighted || undefined}
            ref={highlighted ? revealCard : undefined}
          >
            {card.questions.length === 1 ? (
              <QuestionCard question={card.questions[0]!} scope={scope} askQuestions={card.questions} />
            ) : (
              <QuestionSetCard
                questions={card.questions}
                scope={scope}
                activeQuestionId={landing ? landingQuestionId! : undefined}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}
