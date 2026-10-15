// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { AgentTask } from "../types";
import { agentRunState, commentSupplementReceipts, recipientActions, recipientRouting, resolveRecipientAction, TASK_SUPPLEMENT_CAPABILITY } from "./run-steering";

function task(over: Partial<AgentTask>): AgentTask {
  return {
    id: "task", agent_id: "agent", issue_id: "issue", status: "running", priority: 0,
    dispatched_at: null, started_at: null, completed_at: null, result: null, error: null,
    created_at: "2026-09-23T00:00:00Z", ...over,
  } as AgentTask;
}

describe("agentRunState", () => {
  it("reports a steerable running turn only with the negotiated capability and permission", () => {
    const steerable = task({ supplement_capability: TASK_SUPPLEMENT_CAPABILITY, can_supplement: true });
    expect(agentRunState([steerable], "agent")).toEqual({ kind: "running", task: steerable, steerable: true });
    const unsupported = task({ can_supplement: true });
    expect(agentRunState([unsupported], "agent")).toMatchObject({ kind: "running", steerable: false });
    const forbidden = task({ supplement_capability: TASK_SUPPLEMENT_CAPABILITY, can_supplement: false });
    expect(agentRunState([forbidden], "agent")).toMatchObject({ kind: "running", steerable: false });
  });

  it("prefers the running turn over a queued follow-up for the same agent", () => {
    const queued = task({ id: "next", status: "queued" });
    const running = task({ id: "now" });
    expect(agentRunState([queued, running], "agent")).toMatchObject({ kind: "running", task: { id: "now" } });
  });

  it("separates a starting turn from a queued one and ignores other agents", () => {
    expect(agentRunState([task({ status: "dispatched" })], "agent").kind).toBe("starting");
    expect(agentRunState([task({ status: "waiting_local_directory" })], "agent").kind).toBe("starting");
    expect(agentRunState([task({ status: "deferred" })], "agent").kind).toBe("queued");
    expect(agentRunState([task({ status: "completed" }), task({ agent_id: "other" })], "agent").kind).toBe("idle");
  });
});

describe("commentSupplementReceipts", () => {
  it("returns the receipt list, or the single receipt an older server sent", () => {
    const receipts = [{ task_id: "a", agent_id: "x", status: "pending" as const }, { task_id: "b", status: "delivered" as const }];
    expect(commentSupplementReceipts({ supplements: receipts, supplement_task_id: "a", supplement_status: "pending" })).toBe(receipts);
    expect(commentSupplementReceipts({ supplement_task_id: "a", supplement_status: "failed", supplement_failure_reason: "turn_ended" }))
      .toEqual([{ task_id: "a", status: "failed", failure_reason: "turn_ended", delivered_at: undefined }]);
    expect(commentSupplementReceipts({})).toEqual([]);
  });
});

describe("recipient actions", () => {
  const steerable = { kind: "running", task: task({}), steerable: true } as const;
  const busy = { kind: "running", task: task({}), steerable: false } as const;
  const defaults = { canSteer: true, canRestart: true, steerByDefault: true };

  it("offers steering only to a steerable running turn", () => {
    expect(recipientActions(steerable, defaults)).toEqual(["steer", "after_run", "restart", "skip"]);
    expect(recipientActions(busy, defaults)).toEqual(["after_run", "restart", "skip"]);
    expect(recipientActions({ kind: "starting", task: task({ status: "dispatched" }) }, defaults)).toEqual(["after_run", "restart", "skip"]);
    expect(recipientActions({ kind: "idle" }, defaults)).toEqual(["start", "skip"]);
    expect(recipientActions({ kind: "queued", task: task({ status: "queued" }) }, defaults)).toEqual(["start", "skip"]);
  });

  it.each([undefined, "this-thread", "another-thread"])(
    "defaults a steerable run to steering regardless of its starting thread: %s",
    (trigger_comment_id) => {
      const state = { ...steerable, task: task({ trigger_comment_id }) };
      expect(resolveRecipientAction(state, undefined, defaults)).toBe("steer");
    },
  );

  it.each(["after_run", "restart", "skip", "steer"] as const)("preserves an explicit %s choice", (chosen) => {
    expect(resolveRecipientAction(steerable, chosen, defaults)).toBe(chosen);
  });

  it("respects the personal after-run default while allowing a per-message steering choice", () => {
    const afterRunDefault = { ...defaults, steerByDefault: false };
    expect(resolveRecipientAction(steerable, undefined, afterRunDefault)).toBe("after_run");
    expect(resolveRecipientAction(steerable, "steer", afterRunDefault)).toBe("steer");
  });

  it("uses normal handling until a run is running and can take input", () => {
    expect(resolveRecipientAction(busy, undefined, defaults)).toBe("after_run");
    expect(resolveRecipientAction({ kind: "starting", task: task({ status: "dispatched" }) }, undefined, defaults)).toBe("after_run");
    expect(resolveRecipientAction({ kind: "queued", task: task({ status: "queued" }) }, undefined, defaults)).toBe("start");
    expect(resolveRecipientAction({ kind: "idle" }, undefined, defaults)).toBe("start");
  });

  it("drops a choice that is no longer available", () => {
    // The run no longer takes input, or the composer is now editing.
    expect(resolveRecipientAction(busy, "steer", defaults)).toBe("after_run");
    expect(resolveRecipientAction(steerable, "steer", { ...defaults, canSteer: false })).toBe("after_run");
    // The turn ended while the message was being written.
    expect(resolveRecipientAction({ kind: "idle" }, "steer", defaults)).toBe("start");
    expect(resolveRecipientAction({ kind: "idle" }, "skip", defaults)).toBe("skip");
    // Editing never stops a run.
    expect(recipientActions(steerable, { canSteer: false, canRestart: false, steerByDefault: true })).toEqual(["after_run", "skip"]);
  });

  it("maps actions to request routing without steering explicitly excluded turns", () => {
    const routing = recipientRouting([
      { agentId: "a", action: "steer", state: steerable },
      { agentId: "b", action: "skip", state: { ...steerable, task: task({ id: "turn-b" }) } },
      { agentId: "c", action: "restart", state: { kind: "running", task: task({ id: "turn-c" }), steerable: false } },
      { agentId: "d", action: "after_run", state: { ...steerable, task: task({ id: "turn-d" }) } },
    ]);
    expect(routing).toEqual({ steerTaskIds: [steerable.task.id], suppressAgentIds: ["b"], restartTaskIds: ["turn-c"] });
    expect(recipientRouting([
      { agentId: "d", action: "after_run", state: steerable },
    ]).steerTaskIds).toEqual([]);
  });
});
