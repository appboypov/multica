"use client";

import { Check, Clock, Users } from "lucide-react";
import type { Question } from "@multica/core/types";
import { useActorName } from "@multica/core/workspace/hooks";
import { Badge } from "@multica/ui/components/ui/badge";
import { ActorAvatar } from "../common/actor-avatar";
import { useT } from "../i18n";

/** True for an Each question with more than one answerer: its card shows each answerer's progress. */
export function showsAnswererProgress(question: Question): boolean {
  return question.answer_policy === "each" && question.answerers.length > 1;
}

/** The current answerers' answers; answers of people reassigned away are not counted. */
function currentAnswers(question: Question) {
  return new Map(
    question.answers
      .filter((answer) => question.answerers.some((answerer) => answerer.user_id === answer.answerer_user_id))
      .map((answer) => [answer.answerer_user_id, answer]),
  );
}

/** The Each question's "2 of 3 answered" pill, in the card's header. */
export function AnswerCountPill({ question }: { question: Question }) {
  const { t } = useT("questions");
  return (
    <Badge variant="secondary">
      <Users />
      {t(($) => $.each.answered, { answered: currentAnswers(question).size, total: question.answerers.length })}
    </Badge>
  );
}

/**
 * An Each question's answerers: one chip per answerer with their answer or
 * "waiting", and the line that says the asker reads each answer as it arrives.
 */
export function AnswererProgress({ question }: { question: Question }) {
  const { t } = useT("questions");
  const { getActorName } = useActorName();
  const answers = currentAnswers(question);

  return (
    <div className="flex flex-col gap-2">
      <ul className="flex flex-wrap gap-1.5">
        {question.answerers.map((answerer) => {
          const answer = answers.get(answerer.user_id);
          return (
            <li key={answerer.user_id} className="flex h-8 min-w-0 items-center gap-1.5 rounded-lg border border-border px-2 text-body">
              <ActorAvatar actorType="member" actorId={answerer.user_id} size="sm" />
              <span className="shrink-0 font-medium">{getActorName("member", answerer.user_id)}</span>
              {answer ? (
                <>
                  <Check aria-hidden className="size-3.5 shrink-0 text-success" />
                  <span className="truncate text-caption text-muted-foreground" title={answer.display_text}>{answer.display_text}</span>
                </>
              ) : (
                <>
                  <Clock aria-hidden className="size-3.5 shrink-0 text-muted-foreground" />
                  <span className="text-caption text-muted-foreground">{t(($) => $.each.waiting)}</span>
                </>
              )}
            </li>
          );
        })}
      </ul>
      <p className="text-caption text-muted-foreground">
        {t(($) => $.each.line, { name: getActorName("agent", question.asker_agent_id) })}
      </p>
    </div>
  );
}
