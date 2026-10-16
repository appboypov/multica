import { describe, expect, it } from "vitest";
import type { AnswerType, TypedQuestion } from "@multica/core/types";
import { controlShowsNote, draftError, toggleOption, valueToSend, type AnswerableQuestion } from "./draft";
import { draftForKey } from "./keys";

function question<T extends AnswerType>(
  answer_type: T,
  settings: TypedQuestion<T>["settings"],
): TypedQuestion<T> {
  return {
    id: "q-1",
    workspace_id: "ws-1",
    asker_agent_id: "agent-pieter",
    asked_in_task_id: "task-3",
    asker_user_id: "",
    ask_id: "q-1",
    ask_position: 0,
    ask_title: null,
    answer_policy: "any_one",
    prompt: "Prompt",
    context_line: null,
    status: "open",
    due_date: null,
    blocks_work: false,
    links: [],
    answerers: [],
    events: [],
    unseen_answer_count: 0,
    wake_available: false,
    wake_targets: [],
    created_at: "2026-10-13T10:00:00Z",
    updated_at: "2026-10-13T10:00:00Z",
    answer_type,
    settings,
    suggested_answer: null,
    answers: [],
  };
}

const ranking = question("ranking", { items: ["Invoices", "Credit notes", "Receipts"] });
const facts = question("confirm_facts", { facts: ["VAT is 21%", "Invoices are monthly"] });
const multiple = question("multiple_choice", { options: ["A", "B", "C"], min: 2, max: 3 });
const yesNo = question("yes_no", { why_on_no: true });
const single = question("single_choice", { options: ["Per line", "On total"], allow_other: true });

describe("valueToSend", () => {
  it("sends an untouched ranking in the order shown", () => {
    expect(valueToSend(ranking, null)).toEqual({ order: ["Invoices", "Credit notes", "Receipts"] });
  });

  it("sends nothing while a fact is unmarked", () => {
    expect(valueToSend(facts, { facts: [{ correct: true }, null] })).toBeNull();
    expect(valueToSend(facts, { facts: [{ correct: true }, { correct: true }] })).toEqual({
      facts: [{ correct: true }, { correct: true }],
    });
  });

  it("sends an emptied Multiple choice as an answer, which a minimum of 0 accepts", () => {
    const optional = question("multiple_choice", { options: ["A", "B"], min: 0, max: 2 });
    expect(valueToSend(optional, { options: [] })).toEqual({ options: [] });
    expect(draftError(optional, { options: [] }, "")).toBeNull();
    expect(draftError(multiple, { options: [] }, "")).toEqual({ key: "validation.minimumOptions", values: { count: 2 } });
  });

  it("sends nothing for a text, number or date emptied again", () => {
    expect(valueToSend(question("text", { length: "short" }), { text: " " })).toBeNull();
    expect(valueToSend(question("number", {}), { number: null })).toBeNull();
    expect(valueToSend(question("date", { kind: "date" }), {})).toBeNull();
  });
});

describe("draftError of a range", () => {
  const range = question("date", { kind: "range" });

  it("asks for the other end while one is picked", () => {
    expect(draftError(range, { start: "2026-10-20T00:00:00.000Z" }, "")).toEqual({ key: "validation.rangeRequired" });
    expect(draftError(range, { start: "2026-10-20T00:00:00.000Z", end: "2026-10-23T23:59:59.000Z" }, "")).toBeNull();
  });
});

describe("draftError", () => {
  it("holds back nothing while nothing is picked", () => {
    expect(draftError(multiple, null, "")).toBeNull();
  });

  it("asks for every fact, then for the correction of a Not correct one", () => {
    expect(draftError(facts, { facts: [{ correct: true }, null] }, "")).toEqual({ key: "validation.everyFactRequired" });
    expect(draftError(facts, { facts: [{ correct: true }, { correct: false, correction: " " }] }, "")).toEqual({
      key: "validation.factCorrectionRequired",
      values: { number: 2 },
    });
  });

  it("asks for the why of a No", () => {
    expect(draftError(yesNo, { choice: "no" }, "")).toEqual({ key: "validation.noNoteRequired" });
    expect(draftError(yesNo, { choice: "no" }, "Rounding differs")).toBeNull();
  });
});

describe("controlShowsNote", () => {
  it("is true only for a No whose why is asked", () => {
    expect(controlShowsNote(yesNo, { choice: "no" })).toBe(true);
    expect(controlShowsNote(yesNo, { choice: "yes" })).toBe(false);
    expect(controlShowsNote(question("yes_no", { why_on_no: false }), { choice: "no" })).toBe(false);
  });
});

describe("toggleOption", () => {
  it("keeps picks in the asker's order and empties to an empty answer", () => {
    const picked = toggleOption(multiple, toggleOption(multiple, null, "C"), "A");
    expect(picked).toEqual({ options: ["A", "C"] });
    expect(toggleOption(multiple, toggleOption(multiple, picked, "A"), "C")).toEqual({ options: [] });
  });
});

describe("draftForKey", () => {
  it("answers Yes / No with Y and N", () => {
    expect(draftForKey(yesNo, null, "y")).toEqual({ choice: "yes" });
    expect(draftForKey(yesNo, null, "N")).toEqual({ choice: "no" });
  });

  it("picks options by number, Other after them, and ignores other keys", () => {
    const q: AnswerableQuestion = single;
    expect(draftForKey(q, null, "2")).toEqual({ option: "On total" });
    expect(draftForKey(q, null, "3")).toEqual({ other: "" });
    expect(draftForKey(q, null, "4")).toBeUndefined();
    expect(draftForKey(q, null, "x")).toBeUndefined();
  });
});
