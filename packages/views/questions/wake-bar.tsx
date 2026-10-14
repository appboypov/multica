"use client";

import { useState } from "react";
import { AlarmClock, Check } from "lucide-react";
import { toast } from "sonner";
import { useWorkspaceId } from "@multica/core/hooks";
import { createLogger } from "@multica/core/logger";
import { useWakeQuestionAgent } from "@multica/core/questions";
import type { Question } from "@multica/core/types";
import { useActorName } from "@multica/core/workspace/hooks";
import { Button } from "@multica/ui/components/ui/button";
import { ActorAvatar } from "../common/actor-avatar";
import { useT } from "../i18n";

const logger = createLogger("questions.wake-bar");

export interface WakeBarsProps {
  /** The issue the questions are on; Wake starts the asking agent's run there. */
  issueId: string;
  /** The issue's identifier, such as WORK-412. */
  identifier: string;
  /** The issue's questions the bars are about; one bar per asking agent. */
  questions: readonly Question[];
}

/**
 * One bar per agent with `wake_available` among the questions: "Pieter has 1
 * answer from you on WORK-412" and **Wake Pieter**. After a Wake the bar reads
 * "Pieter was woken with your answers" until the agent has a new answer to wake on.
 */
export function WakeBars({ issueId, identifier, questions }: WakeBarsProps) {
  const [woken, setWoken] = useState<ReadonlySet<string>>(() => new Set());
  const agentIds = [...new Set(questions.map((q) => q.asker_agent_id))];

  return (
    <>
      {agentIds.map((agentId) => {
        const target = questions.find((q) => q.asker_agent_id === agentId && q.wake_available);
        if (target) {
          return (
            <WakeBar
              key={agentId}
              question={target}
              issueId={issueId}
              identifier={identifier}
              onWoken={() => setWoken((current) => new Set(current).add(agentId))}
            />
          );
        }
        return woken.has(agentId) ? <WokenBar key={agentId} agentId={agentId} /> : null;
      })}
    </>
  );
}

function WakeBar({
  question,
  issueId,
  identifier,
  onWoken,
}: {
  question: Question;
  issueId: string;
  identifier: string;
  onWoken: () => void;
}) {
  const { t } = useT("questions");
  const wsId = useWorkspaceId();
  const { getActorName } = useActorName();
  const wake = useWakeQuestionAgent(wsId, issueId);
  const name = getActorName("agent", question.asker_agent_id);

  return (
    <div className="flex min-h-11 items-center gap-2 rounded-lg border border-brand/40 bg-brand/5 px-3 py-1.5 text-body" data-wake-agent={question.asker_agent_id}>
      <ActorAvatar actorType="agent" actorId={question.asker_agent_id} size="sm" />
      <span className="min-w-0 truncate">
        {t(($) => $.wake_bar.unseen, { name, count: question.unseen_answer_count, identifier })}
      </span>
      <Button
        type="button"
        size="sm"
        className="ml-auto shrink-0"
        disabled={wake.isPending}
        onClick={() =>
          wake.mutate(
            { id: question.id },
            {
              onSuccess: onWoken,
              onError: (error) => {
                logger.error("wake failed", { questionId: question.id, issueId, error });
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
