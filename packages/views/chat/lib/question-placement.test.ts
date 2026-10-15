// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { QuestionCardGroup } from "@multica/core/questions";
import type { Question } from "@multica/core/types";
import { placeQuestionCards, type ChatRowAnchor } from "./question-placement";

function card(askId: string, taskId: string, createdAt: string): QuestionCardGroup {
  return { askId, questions: [{ id: askId, ask_id: askId, asked_in_task_id: taskId, created_at: createdAt } as Question] };
}

const ROWS: ChatRowAnchor[] = [
  { key: "m-1", createdAt: "2026-10-15T09:00:00Z" },
  { key: "task:task-3", createdAt: "2026-10-15T09:02:00Z" },
  { key: "m-3", createdAt: "2026-10-15T09:05:00Z" },
];

describe("placeQuestionCards", () => {
  it("puts a card after the row of the turn that asked it, in the order asked", () => {
    const first = card("q-1", "task-3", "2026-10-15T09:01:00Z");
    const second = card("q-2", "task-3", "2026-10-15T09:01:30Z");
    const placed = placeQuestionCards(ROWS, [first, second], false);
    expect([...placed.keys()]).toEqual(["task:task-3"]);
    expect(placed.get("task:task-3")).toEqual([first, second]);
  });

  it("keeps a card asked mid-stream on the live row, which shares the turn's key", () => {
    const live: ChatRowAnchor[] = [ROWS[0]!, { key: "task:task-4", createdAt: null }];
    const placed = placeQuestionCards(live, [card("q-1", "task-4", "2026-10-15T09:01:00Z")], false);
    expect([...placed.keys()]).toEqual(["task:task-4"]);
  });

  it("puts a card whose turn has no row after the last message sent before it was asked", () => {
    const placed = placeQuestionCards(ROWS, [card("q-1", "task-9", "2026-10-15T09:03:00Z")], false);
    expect([...placed.keys()]).toEqual(["task:task-3"]);
  });

  it("holds back a card asked before every loaded message while older pages remain, else puts it on the first row", () => {
    const early = [card("q-1", "task-1", "2026-10-15T08:00:00Z")];
    expect(placeQuestionCards(ROWS, early, true).size).toBe(0);
    expect([...placeQuestionCards(ROWS, early, false).keys()]).toEqual(["m-1"]);
  });

  it("places nothing without cards or rows", () => {
    expect(placeQuestionCards(ROWS, [], false).size).toBe(0);
    expect(placeQuestionCards([], [card("q-1", "task-3", "2026-10-15T09:01:00Z")], false).size).toBe(0);
  });
});
