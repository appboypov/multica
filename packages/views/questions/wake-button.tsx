"use client";

import { AlarmClock } from "lucide-react";
import { toast } from "sonner";
import { useWorkspaceId } from "@multica/core/hooks";
import { createLogger } from "@multica/core/logger";
import { useWakeQuestionAgent, wakeOffers, type WakeOffer } from "@multica/core/questions";
import type { Question } from "@multica/core/types";
import { useActorName } from "@multica/core/workspace/hooks";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";

const logger = createLogger("questions.wake");

/**
 * One **Wake <agent>** per wake target of the questions that can be woken now,
 * the asking agent first: "Wake Pieter", "Wake Joeri".
 */
export function WakeButtons({ questions, issueId }: { questions: readonly Question[]; issueId: string | null }) {
  return (
    <>
      {wakeOffers(questions).map((offer) => (
        <WakeButton key={`${offer.target.agent_id}:${offer.target.issue_id}`} offer={offer} issueId={issueId} />
      ))}
    </>
  );
}

/** **Wake <agent>**: the one way an answer turns into a run (ADR-0003), on the target's issue. */
function WakeButton({ offer: { question, target }, issueId }: { offer: WakeOffer; issueId: string | null }) {
  const { t } = useT("questions");
  const wsId = useWorkspaceId();
  const { getActorName } = useActorName();
  const wake = useWakeQuestionAgent(wsId, issueId);
  const name = getActorName("agent", target.agent_id);

  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      disabled={wake.isPending}
      onClick={() =>
        wake.mutate(
          { id: question.id, target: { agentId: target.agent_id, issueId: target.issue_id } },
          {
            onError: (error) => {
              logger.error("wake failed", { questionId: question.id, agentId: target.agent_id, error });
              toast.error(t(($) => $.card.wake_failed, { name }), { description: error.message });
            },
          },
        )
      }
    >
      <AlarmClock />
      {t(($) => $.card.wake, { name })}
    </Button>
  );
}
