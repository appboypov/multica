import { cleanup, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { CommentSupplementReceipt } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { commentRunInputMarks, RunInputMarks } from "./run-input-marks";

vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({ getActorName: (_type: string, id: string) => (id === "orion" ? "Orion" : "Thomas") }),
}));

afterEach(cleanup);

function receipt(overrides: Partial<CommentSupplementReceipt>): CommentSupplementReceipt {
  return { task_id: "thomas-run", agent_id: "thomas", item_kind: "comment", item_id: "comment", status: "delivered", ...overrides };
}

const chips = () => screen.queryAllByRole("listitem").map((item) => item.textContent);

describe("RunInputMarks", () => {
  it("shows 👀 and ✅ of each receiving run with its agent's name", () => {
    renderWithI18n(<RunInputMarks marks={[
      { task_id: "thomas-run", agent_id: "thomas", seen_at: "2026-10-15T10:00:00Z", handled_at: "2026-10-15T10:05:00Z" },
      { task_id: "orion-run", agent_id: "orion", seen_at: "2026-10-15T10:01:00Z" },
    ]} />);

    const shown = chips();
    expect(shown).toHaveLength(3);
    expect(shown[0]).toContain("👀");
    expect(shown[0]).toContain("Thomas");
    expect(shown[1]).toContain("✅");
    expect(shown[1]).toContain("Thomas");
    expect(shown[2]).toContain("👀");
    expect(shown[2]).toContain("Orion");
  });

  it("shows nothing for runs that left the item unmarked", () => {
    renderWithI18n(<RunInputMarks marks={[{ task_id: "thomas-run", agent_id: "thomas" }]} />);

    expect(screen.queryByRole("list")).toBeNull();
  });

  it("keeps the first text's marks apart from the edit's, and each edit mark once per run", () => {
    const marks = commentRunInputMarks([
      receipt({ handled_at: "2026-10-15T10:05:00Z" }),
      receipt({ item_kind: "comment_edit", item_id: "edit-1", seen_at: "2026-10-15T10:06:00Z" }),
      receipt({ item_kind: "comment_edit", item_id: "edit-2", seen_at: "2026-10-15T10:07:00Z" }),
      receipt({ task_id: "legacy-run", agent_id: undefined, handled_at: "2026-10-15T10:08:00Z" }),
    ]);
    renderWithI18n(<RunInputMarks marks={marks} />);

    const shown = chips();
    expect(shown).toHaveLength(2);
    expect(shown[0]).toContain("✅");
    expect(shown[1]).toContain("👀");
    expect(shown[0]).not.toEqual(shown[1]?.replace("👀", "✅"));
  });
});
