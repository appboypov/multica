import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { TimelineEntry } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { QuestionWakeActivityText } from "./question-wake-activity";

vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({
    getActorName: (type: string, id: string) => (type === "agent" && id === "agent-pieter" ? "Pieter" : "Someone"),
  }),
}));

function wake(answerCount: number): TimelineEntry {
  return {
    type: "activity",
    id: "activity-1",
    action: "question_wake",
    actor_type: "member",
    actor_id: "user-brian",
    actor_name: "Brian",
    created_at: "2026-10-13T10:00:00Z",
    details: { agent_id: "agent-pieter", answer_count: answerCount, task_id: "task-4" },
  };
}

describe("QuestionWakeActivityText", () => {
  it("names the woken agent, the member who woke it and the answers it carries", () => {
    renderWithI18n(<QuestionWakeActivityText entry={wake(1)} />);
    expect(screen.getByText("Pieter was woken by Brian with 1 answer")).toBeInTheDocument();
  });

  it("counts several answers", () => {
    renderWithI18n(<QuestionWakeActivityText entry={wake(2)} />);
    expect(screen.getByText("Pieter was woken by Brian with 2 answers")).toBeInTheDocument();
  });
});
