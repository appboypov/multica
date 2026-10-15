"use client";

import { AlarmClock } from "lucide-react";
import { toast } from "sonner";
import { useWorkspaceId } from "@multica/core/hooks";
import { createLogger } from "@multica/core/logger";
import {
  useWakeQuestionAgent, wakeOfferKey, wakeOffers, wakeTargetScope, type QuestionScope, type WakeOffer,
} from "@multica/core/questions";
import type { Question } from "@multica/core/types";
import { useActorName } from "@multica/core/workspace/hooks";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";

const logger = createLogger("questions.wake");

/**
 * One **Wake <agent>** per wake target of the questions that can be woken now,
 * the asking agent first: "Wake Pieter", "Wake Joeri". `scope` is the issue or
 * chat the card shows in, whose questions refresh after the wake.
 */
export function WakeButtons({ questions, scope }: { questions: readonly Question[]; scope: QuestionScope }) {
  return (
    <>
      {wakeOffers(questions).map((offer) => (
        <WakeButton key={wakeOfferKey(offer.target)} offer={offer} scope={scope} />
      ))}
    </>
  );
}

/** **Wake <agent>**: wakes the agent in the target's chat or issue (ADR-0018). */
function WakeButton({ offer: { question, target }, scope }: { offer: WakeOffer; scope: QuestionScope }) {
  const { t } = useT("questions");
  const wsId = useWorkspaceId();
  const { getActorName } = useActorName();
  const wake = useWakeQuestionAgent(wsId, scope);
  const name = getActorName("agent", target.agent_id);

  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      disabled={wake.isPending}
      onClick={() =>
        wake.mutate(
          { id: question.id, target: { agentId: target.agent_id, scope: wakeTargetScope(target) } },
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
