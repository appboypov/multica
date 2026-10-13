// @vitest-environment node
import { describe, expect, it } from "vitest";
import { StoredAgentDraftSchema } from "../api/schemas";
import type { AgentDraft } from "./draft";
import {
  fromStoredAgentDraft,
  storedAgentDraftsEqual,
  toStoredAgentDraft,
} from "./stored-draft";

const draft = (): AgentDraft => ({
  name: "Release manager",
  description: "Ships carefully",
  instructions: "# Role\nShip.",
  instructionSelection: [],
  conversationStarters: [
    { label: "Plan a release", prompt: "Plan the next release." },
  ],
  avatarUrl: "🚀",
  runtimeId: "runtime-1",
  model: "gpt-5.6-sol",
  thinkingLevel: "high",
  serviceTier: "priority",
  skillIds: new Set(["skill-1", "skill-2"]),
  permissionScope: "members",
  memberIds: new Set(["member-1"]),
  teamIds: new Set(["team-1"]),
});

describe("stored agent draft", () => {
  it("round-trips every editable field", () => {
    const original = draft();
    const restored = fromStoredAgentDraft(
      toStoredAgentDraft(original, "msg-1"),
      original.runtimeId,
    );
    expect(restored).toEqual(original);
  });

  it("writes and restores selected paths in their order", () => {
    const original = {
      ...draft(),
      instructionSelection: ["/skills/reviewer", "/notes/context.md"],
    };
    const stored = toStoredAgentDraft(original, "msg-1");

    expect(stored.instruction_selection).toEqual(original.instructionSelection);
    expect(fromStoredAgentDraft(StoredAgentDraftSchema.parse(stored), "runtime-1"))
      .toEqual(original);
  });

  it("restores an absent stored selection as empty with or without schema parsing", () => {
    const stored = toStoredAgentDraft(draft(), null);
    delete stored.instruction_selection;

    expect(StoredAgentDraftSchema.parse(stored).instruction_selection).toEqual([]);
    expect(fromStoredAgentDraft(stored, "runtime-1").instructionSelection).toEqual([]);
  });

  it.each([
    { selection: null },
    { selection: "/skills/reviewer" },
    { selection: ["/skills/reviewer", 3] },
  ])(
    "falls back to an empty selection for malformed stored paths $selection",
    ({ selection }) => {
      const parsed = StoredAgentDraftSchema.parse({
        ...toStoredAgentDraft(draft(), null),
        instruction_selection: selection,
      });

      expect(parsed.instruction_selection).toEqual([]);
      expect(fromStoredAgentDraft(parsed, "runtime-1").instructionSelection).toEqual([]);
    },
  );

  it("compares selected paths and their order for autosave", () => {
    const stored = toStoredAgentDraft(
      { ...draft(), instructionSelection: ["/skills/reviewer", "/notes/context.md"] },
      null,
    );

    expect(storedAgentDraftsEqual(stored, { ...stored })).toBe(true);
    expect(storedAgentDraftsEqual(stored, {
      ...stored,
      instruction_selection: ["/skills/reviewer"],
    })).toBe(false);
    expect(storedAgentDraftsEqual(stored, {
      ...stored,
      instruction_selection: ["/notes/context.md", "/skills/reviewer"],
    })).toBe(false);
  });

  it("treats an absent selection and an empty selection as unchanged", () => {
    const stored = toStoredAgentDraft(draft(), null);
    const withoutSelection = { ...stored };
    delete withoutSelection.instruction_selection;

    expect(storedAgentDraftsEqual(stored, withoutSelection)).toBe(true);
    expect(storedAgentDraftsEqual(withoutSelection, stored)).toBe(true);
  });

  // The runtime a conversation executes on is owned by its carrier agent. A
  // copy inside the draft could only go stale and put the picker on a runtime
  // that runs nothing (MUL-5163), so the restore takes it from the caller.
  it("takes the runtime from the caller, not the stored copy", () => {
    const stored = toStoredAgentDraft(draft(), null);
    expect(stored).not.toHaveProperty("runtimeId");
    expect(fromStoredAgentDraft(stored, "runtime-2").runtimeId).toBe(
      "runtime-2",
    );
  });

  // The marker is what stops a restore from re-applying the last reply over
  // edits made after it, so it has to survive the round trip.
  it("carries the applied message marker", () => {
    expect(toStoredAgentDraft(draft(), "msg-9").applied_message_id).toBe(
      "msg-9",
    );
    expect(toStoredAgentDraft(draft(), null).applied_message_id).toBeNull();
  });

  // Autosave compares on the stored form: a Set rebuilt with the same members
  // is a new reference every render and would otherwise write on every keystroke
  // elsewhere in the form.
  it("treats rebuilt sets with the same members as unchanged", () => {
    expect(
      storedAgentDraftsEqual(
        toStoredAgentDraft(draft(), "msg-1"),
        toStoredAgentDraft(draft(), "msg-1"),
      ),
    ).toBe(true);
    expect(
      storedAgentDraftsEqual(
        toStoredAgentDraft(draft(), "msg-1"),
        toStoredAgentDraft({ ...draft(), name: "Other" }, "msg-1"),
      ),
    ).toBe(false);
  });
});
