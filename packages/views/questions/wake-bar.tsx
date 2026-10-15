"use client";

import { useState } from "react";
import { AlarmClock, Check } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { useWorkspaceId } from "@multica/core/hooks";
import { issueDetailOptions } from "@multica/core/issues/queries";
import { createLogger } from "@multica/core/logger";
import { useWakeQuestionAgent, wakeOfferKey, wakeOffers, wakeTargetScope, type WakeOffer } from "@multica/core/questions";
import type { Question } from "@multica/core/types";
import { useActorName } from "@multica/core/workspace/hooks";
import { Button } from "@multica/ui/components/ui/button";
import { ActorAvatar } from "../common/actor-avatar";
import { useT } from "../i18n";
import { placeScope, type QuestionPlace } from "./row-link";

const logger = createLogger("questions.wake-bar");

export interface WakeBarsProps {
  /** The issue or chat the questions show in. */
  place: QuestionPlace;
  /** The place's questions the bars are about; one bar per wake target. */
  questions: readonly Question[];
}

/**
 * One bar per wake target that can be woken among the questions, the asking
 * agent first: "Pieter has 1 answer from you on WORK-412" (or "in the chat
 * with Brian") and **Wake Pieter**. After a Wake the bar reads "Pieter was
 * woken with your answers" until the agent has a new answer to wake on.
 */
export function WakeBars({ place, questions }: WakeBarsProps) {
  const [woken, setWoken] = useState<readonly string[]>([]);
  const offers = wakeOffers(questions);
  const offered = new Set(offers.map((offer) => offer.target.agent_id));

  return (
    <>
      {offers.map((offer) => (
        <WakeBar
          key={wakeOfferKey(offer.target)}
          offer={offer}
          place={place}
          onWoken={() => setWoken((current) => [...current, offer.target.agent_id])}
        />
      ))}
      {[...new Set(woken)].filter((agentId) => !offered.has(agentId)).map((agentId) => (
        <WokenBar key={agentId} agentId={agentId} />
      ))}
    </>
  );
}

function WakeBar({
  offer: { question, target },
  place,
  onWoken,
}: {
  offer: WakeOffer;
  place: QuestionPlace;
  onWoken: () => void;
}) {
  const { t } = useT("questions");
  const wsId = useWorkspaceId();
  const { getActorName } = useActorName();
  const wake = useWakeQuestionAgent(wsId, placeScope(place));
  const placeIssueId = "issueId" in place ? place.issueId : null;
  // A target on another issue (an assignee above, a project lead) names that issue.
  const otherIssueId = target.issue_id !== null && target.issue_id !== placeIssueId ? target.issue_id : "";
  const otherIssue = useQuery({ ...issueDetailOptions(wsId, otherIssueId), enabled: !!otherIssueId });
  const name = getActorName("agent", target.agent_id);
  const count = target.unseen_answer_count;
  // A chat target wakes in the chat the questions were asked in, named by its member.
  const unseen = target.chat_session_id !== null
    ? t(($) => $.wake_bar.unseen_chat, {
        name,
        count,
        member: getActorName("member", ("memberId" in place ? place.memberId : null) ?? ""),
      })
    : t(($) => $.wake_bar.unseen, {
        name,
        count,
        identifier: otherIssueId ? (otherIssue.data?.identifier ?? "") : "identifier" in place ? place.identifier : "",
      });

  return (
    <div className="flex min-h-11 items-center gap-2 rounded-lg border border-brand/40 bg-brand/5 px-3 py-1.5 text-body" data-wake-agent={target.agent_id}>
      <ActorAvatar actorType="agent" actorId={target.agent_id} size="sm" />
      <span className="min-w-0 truncate">
        {unseen}
      </span>
      <Button
        type="button"
        size="sm"
        className="ml-auto shrink-0"
        disabled={wake.isPending}
        onClick={() =>
          wake.mutate(
            { id: question.id, target: { agentId: target.agent_id, scope: wakeTargetScope(target) } },
            {
              onSuccess: onWoken,
              onError: (error) => {
                logger.error("wake failed", { questionId: question.id, agentId: target.agent_id, scope: wakeTargetScope(target), error });
                toast.error(t(($) => $.card.wake_failed, { name }), { description: error.message });
              },
            },
          )
        }
      >
        <AlarmClock />
        {t(($) => $.card.wake, { name })}
      </Button>
    </div>
  );
}

function WokenBar({ agentId }: { agentId: string }) {
  const { t } = useT("questions");
  const { getActorName } = useActorName();
  return (
    <div className="flex min-h-11 items-center gap-2 rounded-lg border border-border px-3 py-1.5 text-body text-muted-foreground" role="status">
      <Check aria-hidden className="size-4 shrink-0 text-success" />
      <span className="min-w-0 truncate">{t(($) => $.wake_bar.woken, { name: getActorName("agent", agentId) })}</span>
    </div>
  );
}
