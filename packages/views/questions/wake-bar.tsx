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

const logger = createLogger("questions.wake-bar");

export interface WakeBarsProps {
  /** The issue the questions are on. */
  issueId: string;
  /** The issue's identifier, such as WORK-412. */
  identifier: string;
  /** The issue's questions the bars are about; one bar per wake target. */
  questions: readonly Question[];
}

/**
 * One bar per wake target that can be woken among the questions, the asking
 * agent first: "Pieter has 1 answer from you on WORK-412" and **Wake Pieter**.
 * After a Wake the bar reads "Pieter was woken with your answers" until the
 * agent has a new answer to wake on.
 */
export function WakeBars({ issueId, identifier, questions }: WakeBarsProps) {
  const [woken, setWoken] = useState<readonly string[]>([]);
  const offers = wakeOffers(questions);
  const offered = new Set(offers.map((offer) => offer.target.agent_id));

  return (
    <>
      {offers.map((offer) => (
        <WakeBar
          key={wakeOfferKey(offer.target)}
          offer={offer}
          issueId={issueId}
          identifier={identifier}
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
  issueId,
  identifier,
  onWoken,
}: {
  offer: WakeOffer;
  issueId: string;
  identifier: string;
  onWoken: () => void;
}) {
  const { t } = useT("questions");
  const wsId = useWorkspaceId();
  const { getActorName } = useActorName();
  const wake = useWakeQuestionAgent(wsId, { issueId });
  // A target on another issue (an assignee above, a project lead) names that issue.
  const otherIssueId = target.issue_id !== null && target.issue_id !== issueId ? target.issue_id : "";
  const otherIssue = useQuery({ ...issueDetailOptions(wsId, otherIssueId), enabled: !!otherIssueId });
  const targetIdentifier = target.issue_id === issueId ? identifier : (otherIssue.data?.identifier ?? "");
  const name = getActorName("agent", target.agent_id);

  return (
    <div className="flex min-h-11 items-center gap-2 rounded-lg border border-brand/40 bg-brand/5 px-3 py-1.5 text-body" data-wake-agent={target.agent_id}>
      <ActorAvatar actorType="agent" actorId={target.agent_id} size="sm" />
      <span className="min-w-0 truncate">
        {t(($) => $.wake_bar.unseen, { name, count: target.unseen_answer_count, identifier: targetIdentifier })}
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
