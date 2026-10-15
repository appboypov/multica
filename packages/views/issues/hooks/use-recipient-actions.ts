"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { issueTasksOptions } from "@multica/core/issues/queries";
import { useCommentComposerStore } from "@multica/core/issues/stores";
import {
  agentRunState,
  recipientActions,
  recipientRouting,
  resolveRecipientAction,
  type AgentRunState,
  type RecipientAction,
  type RecipientRouting,
} from "@multica/core/issues/run-steering";
import type { AgentTask, CommentTriggerPreviewAgent } from "@multica/core/types";

const NO_TASKS: AgentTask[] = [];
const ACTIVE_STATUSES = new Set<AgentTask["status"]>(["queued", "deferred", "dispatched", "waiting_local_directory", "running"]);
const NO_CHOICES: Record<string, RecipientAction> = {};

export interface RecipientEntry {
  agent: CommentTriggerPreviewAgent;
  state: AgentRunState;
  action: RecipientAction;
  actions: RecipientAction[];
}

/**
 * The composer's per-recipient choice. The preview decides WHO receives the
 * message (existing trigger rules); each recipient's live run on this issue
 * decides what can happen to it: steer a running turn, fold into a queued
 * run, or start a new one.
 */
export function useRecipientActions({
  issueId,
  agents,
  allowSteer,
  resetKey,
}: {
  issueId: string;
  agents: CommentTriggerPreviewAgent[];
  /** False for edits: delivery is automatic and an edit never stops a run. */
  allowSteer: boolean;
  /** Choices reset when the composer's context changes. */
  resetKey: string;
}) {
  // Every comment row mounts this hook for its edit composer, so it reads only
  // the active runs of its own recipients: with none (the usual idle row) it
  // neither fetches nor re-renders on unrelated task events.
  const agentKey = agents.map((agent) => agent.id).join(",");
  const selectActive = useCallback((all: AgentTask[]) => {
    const ids = new Set(agentKey.split(","));
    return all.filter((task) => ids.has(task.agent_id) && ACTIVE_STATUSES.has(task.status));
  }, [agentKey]);
  const { data: tasks = NO_TASKS } = useQuery({
    ...issueTasksOptions(issueId),
    enabled: !!issueId && agents.length > 0,
    select: selectActive,
  });
  // A running turn that takes input gets the message by default in every
  // thread; only the personal preference to start after the run turns that off.
  const steerByDefault = useCommentComposerStore((s) => s.runningAgentReply !== "after_run");
  const [chosen, setChosen] = useState<Record<string, RecipientAction>>(NO_CHOICES);

  useEffect(() => {
    setChosen(NO_CHOICES);
  }, [resetKey]);

  // Forget choices for agents that are no longer recipients (an @mention removed).
  useEffect(() => {
    const visible = new Set(agents.map((agent) => agent.id));
    setChosen((prev) => {
      const next = Object.fromEntries(Object.entries(prev).filter(([id]) => visible.has(id)));
      return Object.keys(next).length === Object.keys(prev).length ? prev : next;
    });
  }, [agents]);

  const recipients = useMemo<RecipientEntry[]>(() => agents.map((agent) => {
    const state = agentRunState(tasks, agent.id);
    const opts = { canSteer: allowSteer, canRestart: allowSteer, steerByDefault };
    return {
      agent,
      state,
      action: resolveRecipientAction(state, chosen[agent.id], opts),
      actions: recipientActions(state, opts),
    };
  }), [agents, tasks, chosen, allowSteer, steerByDefault]);

  const setAction = useCallback((agentId: string, action: RecipientAction) => {
    setChosen((prev) => (prev[agentId] === action ? prev : { ...prev, [agentId]: action }));
  }, []);

  const reset = useCallback(() => {
    setChosen(NO_CHOICES);
  }, []);

  const routing = useMemo<RecipientRouting>(() => recipientRouting(recipients.map((r) => ({
    agentId: r.agent.id, action: r.action, state: r.state,
  }))), [recipients]);

  return { recipients, routing, setAction, reset };
}
