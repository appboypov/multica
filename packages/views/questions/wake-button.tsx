"use client";

import { AlarmClock } from "lucide-react";
import { toast } from "sonner";
import { useWorkspaceId } from "@multica/core/hooks";
import { createLogger } from "@multica/core/logger";
import { useWakeQuestionAgent } from "@multica/core/questions";
import type { Question } from "@multica/core/types";
import { useActorName } from "@multica/core/workspace/hooks";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";

const logger = createLogger("questions.wake");

/**
 * **Wake <agent>**: the one way an answer turns into a run (ADR-0003). The
 * caller shows it only while the question's `wake_available` is true.
 */
export function WakeButton({ question, issueId }: { question: Question; issueId: string }) {
  const { t } = useT("questions");
  const wsId = useWorkspaceId();
  const { getActorName } = useActorName();
  const wake = useWakeQuestionAgent(wsId, issueId);
  const name = getActorName("agent", question.asker_agent_id);

  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      disabled={wake.isPending}
      onClick={() =>
        wake.mutate(
          { id: question.id },
          {
            onError: (error) => {
              logger.error("wake failed", { questionId: question.id, error });
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
