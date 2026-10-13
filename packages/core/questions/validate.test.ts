// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { Question } from "../types/question";
import { EMPTY_QUESTION } from "../api/schemas";
import { validateAnswer } from "./validate";

// The server registry's ValidateAnswer cases are the canonical sending rules.
const cases: [string, string, unknown, unknown, string, boolean][] = [
  ["text with attachment", "text", {"length": "long"}, {"text": "Hello @Brian\nDetails", "attachment_ids": ["019a0e55-4d50-7000-8000-000000000002"]}, "", false],
  ["short text", "text", {"length": "short"}, {"text": "Per line"}, "", false],
  ["blank text", "text", {"length": "short"}, {"text": " \n\t"}, "", true],
  ["attachment only is not text", "text", {"length": "long"}, {"attachment_ids": ["019a0e55-4d50-7000-8000-000000000002"]}, "", true],
  ["invalid text attachment", "text", {"length": "long"}, {"text": "Hello", "attachment_ids": ["bad"]}, "", true],
  ["yes needs no note", "yes_no", {"why_on_no": true}, {"choice": "yes"}, "", false],
  ["no with why", "yes_no", {"why_on_no": true}, {"choice": "no"}, "Customer rounds per invoice", false],
  ["no without required why", "yes_no", {"why_on_no": true}, {"choice": "no"}, " \n", true],
  ["no without optional why", "yes_no", {"why_on_no": false}, {"choice": "no"}, "", false],
  ["unknown choice", "yes_no", {"why_on_no": false}, {"choice": "maybe"}, "", true],
  ["single option", "single_choice", {"options": ["per line", "per total"], "allow_other": false}, {"option": "per line"}, "", false],
  ["Other with text", "single_choice", {"options": ["per line", "per total"], "allow_other": true}, {"other": "per invoice"}, "", false],
  ["Other prohibited", "single_choice", {"options": ["per line", "per total"], "allow_other": false}, {"other": "per invoice"}, "", true],
  ["empty Other", "single_choice", {"options": ["per line", "per total"], "allow_other": true}, {"other": " "}, "", true],
  ["option and Other", "single_choice", {"options": ["per line", "per total"], "allow_other": true}, {"option": "per line", "other": "per invoice"}, "", true],
  ["no selection", "single_choice", {"options": ["per line", "per total"], "allow_other": true}, {}, "", true],
  ["unknown option", "single_choice", {"options": ["per line", "per total"], "allow_other": true}, {"option": "per week"}, "", true],
  ["multiple minimum", "multiple_choice", {"options": ["A", "B", "C"], "min": 2, "max": 3}, {"options": ["A", "B"]}, "", false],
  ["multiple maximum", "multiple_choice", {"options": ["A", "B", "C"], "min": 2, "max": 3}, {"options": ["A", "B", "C"]}, "", false],
  ["below minimum", "multiple_choice", {"options": ["A", "B", "C"], "min": 2, "max": 3}, {"options": ["A"]}, "", true],
  ["above maximum", "multiple_choice", {"options": ["A", "B", "C"], "min": 1, "max": 2}, {"options": ["A", "B", "C"]}, "", true],
  ["duplicates are not picks", "multiple_choice", {"options": ["A", "B", "C"], "min": 2, "max": 3}, {"options": ["A", "A"]}, "", true],
  ["unknown multiple option", "multiple_choice", {"options": ["A", "B", "C"], "min": 1, "max": 3}, {"options": ["Other"]}, "", true],
  ["zero picks allowed", "multiple_choice", {"options": ["A", "B"], "min": 0, "max": 2}, {"options": []}, "", false],
  ["facts with correction", "confirm_facts", {"facts": ["A", "B", "C"]}, {"facts": [{"correct": true}, {"correct": false, "correction": "B is per invoice"}, {"correct": true}]}, "", false],
  ["incorrect without correction", "confirm_facts", {"facts": ["A", "B", "C"]}, {"facts": [{"correct": true}, {"correct": false}, {"correct": true}]}, "An overall note does not correct B", true],
  ["blank correction", "confirm_facts", {"facts": ["A"]}, {"facts": [{"correct": false, "correction": " "}]}, "", true],
  ["missing fact", "confirm_facts", {"facts": ["A", "B"]}, {"facts": [{"correct": true}]}, "", true],
  ["extra fact", "confirm_facts", {"facts": ["A"]}, {"facts": [{"correct": true}, {"correct": true}]}, "", true],
  ["missing correct flag", "confirm_facts", {"facts": ["A"]}, {"facts": [{}]}, "", true],
  ["null correct flag", "confirm_facts", {"facts": ["A"]}, {"facts": [{"correct": null}]}, "", true],
  ["correct with correction", "confirm_facts", {"facts": ["A"]}, {"facts": [{"correct": true, "correction": "different"}]}, "", true],
  ["ranking permutation", "ranking", {"items": ["A", "B", "C"]}, {"order": ["C", "A", "B"]}, "", false],
  ["missing ranking item", "ranking", {"items": ["A", "B", "C"]}, {"order": ["A", "B"]}, "", true],
  ["duplicate ranking item", "ranking", {"items": ["A", "B", "C"]}, {"order": ["A", "B", "B"]}, "", true],
  ["foreign ranking item", "ranking", {"items": ["A", "B", "C"]}, {"order": ["A", "B", "D"]}, "", true],
  ["scale lower bound", "scale", {"min": 0, "max": 10}, {"point": 0}, "", false],
  ["scale upper bound", "scale", {"min": 0, "max": 10}, {"point": 10}, "", false],
  ["scale below", "scale", {"min": 0, "max": 10}, {"point": -1}, "", true],
  ["scale above", "scale", {"min": 0, "max": 10}, {"point": 11}, "", true],
  ["fractional point", "scale", {"min": 0, "max": 10}, {"point": 1.5}, "", true],
  ["missing point is not zero", "scale", {"min": 0, "max": 10}, {}, "", true],
  ["number lower bound", "number", {"min": -1.5, "max": 3.25}, {"number": -1.5}, "", false],
  ["number upper bound", "number", {"min": -1.5, "max": 3.25}, {"number": 3.25}, "", false],
  ["number below", "number", {"min": -1.5, "max": 3.25}, {"number": -1.6}, "", true],
  ["number above", "number", {"min": -1.5, "max": 3.25}, {"number": 3.26}, "", true],
  ["unbounded negative fraction", "number", {}, {"number": -23.75}, "", false],
  ["missing number is not zero", "number", {}, {}, "", true],
  ["nonfinite number", "number", {}, {"number": Infinity}, "", true],
  ["date leap day", "date", {"kind": "date"}, {"date": "2028-02-29"}, "", false],
  ["impossible date", "date", {"kind": "date"}, {"date": "2026-02-29"}, "", true],
  ["date rejects timestamp", "date", {"kind": "date"}, {"date": "2026-10-13T12:00:00Z"}, "", true],
  ["datetime with offset", "date", {"kind": "datetime"}, {"datetime": "2026-10-13T14:00:00+02:00"}, "", false],
  ["datetime needs timezone", "date", {"kind": "datetime"}, {"datetime": "2026-10-13T14:00:00"}, "", true],
  ["datetime fractional seconds", "date", {"kind": "datetime"}, {"datetime": "2026-10-13T14:00:00.123Z"}, "", false],
  ["datetime offset hours", "date", {"kind": "datetime"}, {"datetime": "2026-10-13T14:00:00+24:00"}, "", true],
  ["datetime offset minutes", "date", {"kind": "datetime"}, {"datetime": "2026-10-13T14:00:00+00:60"}, "", true],
  ["datetime comma fraction", "date", {"kind": "datetime"}, {"datetime": "2026-10-13T14:00:00,123Z"}, "", true],
  ["datetime padded hour", "date", {"kind": "datetime"}, {"datetime": "2026-10-13T4:00:00Z"}, "", true],
  ["wrong date kind", "date", {"kind": "datetime"}, {"date": "2026-10-13"}, "", true],
  ["mixed date variants", "date", {"kind": "date"}, {"date": "2026-10-13", "datetime": "2026-10-13T12:00:00Z"}, "", true],
  ["range compared as instants", "date", {"kind": "range"}, {"start": "2026-10-13T14:00:00+02:00", "end": "2026-10-13T12:30:00Z"}, "", false],
  ["equal range ends", "date", {"kind": "range"}, {"start": "2026-10-13T12:00:00Z", "end": "2026-10-13T12:00:00Z"}, "", false],
  ["reversed range", "date", {"kind": "range"}, {"start": "2026-10-13T12:00:00Z", "end": "2026-10-13T13:00:00+02:00"}, "", true],
  ["range requires end", "date", {"kind": "range"}, {"start": "2026-10-13T12:00:00Z"}, "", true],
  ["range needs timestamps", "date", {"kind": "range"}, {"start": "2026-10-13", "end": "2026-10-14"}, "", true],
  ["member", "person", {"allow": "members"}, {"type": "member", "id": "019a0e55-4d50-7000-8000-000000000001"}, "", false],
  ["agent", "person", {"allow": "agents"}, {"type": "agent", "id": "019a0e55-4d50-7000-8000-000000000001"}, "", false],
  ["both permits agent", "person", {"allow": "both"}, {"type": "agent", "id": "019a0e55-4d50-7000-8000-000000000001"}, "", false],
  ["both permits member", "person", {"allow": "both"}, {"type": "member", "id": "019a0e55-4d50-7000-8000-000000000001"}, "", false],
  ["disallowed agent", "person", {"allow": "members"}, {"type": "agent", "id": "019a0e55-4d50-7000-8000-000000000001"}, "", true],
  ["disallowed member", "person", {"allow": "agents"}, {"type": "member", "id": "019a0e55-4d50-7000-8000-000000000001"}, "", true],
  ["unknown person type", "person", {"allow": "both"}, {"type": "user", "id": "019a0e55-4d50-7000-8000-000000000001"}, "", true],
  ["name is not ID", "person", {"allow": "members"}, {"type": "member", "id": "Brian"}, "", true],
  ["file", "file", {"accept": ["application/pdf"]}, {"attachment_ids": ["019a0e55-4d50-7000-8000-000000000002"]}, "", false],
  ["no file", "file", {"accept": []}, {"attachment_ids": []}, "", true],
  ["invalid file ID", "file", {"accept": []}, {"attachment_ids": ["file.pdf"]}, "", true],
  ["approve", "approval", {"subject": "Design"}, {"decision": "approve"}, "", false],
  ["decline", "approval", {"subject": "Design"}, {"decision": "decline"}, "", false],
  ["request changes with note", "approval", {"subject": "Design"}, {"decision": "request_changes"}, "Round per line", false],
  ["request changes without note", "approval", {"subject": "Design"}, {"decision": "request_changes"}, " \n", true],
  ["unknown decision", "approval", {"subject": "Design"}, {"decision": "accept"}, "", true],
  ["invalid settings also rejected", "scale", {"min": 0, "max": 11}, {"point": 1}, "", true],
  ["null answer", "number", {}, null, "", true],
  ["null number", "number", {}, {"number": null}, "", true],
  ["wrong value shape", "number", {}, {"number": "2"}, "", true],
  ["unknown answer field", "yes_no", {"why_on_no": false}, {"choice": "yes", "why": "hello"}, "", true],
  ["unknown answer type", "unknown", {}, {}, "", true],
];

function question(answer_type: string, settings: unknown): Question {
  return { ...EMPTY_QUESTION, answer_type, settings } as Question;
}

describe("answer validation mirrors the server", () => {
  it.each(cases)("%s", (_name, type, settings, value, note, refused) => {
    const error = validateAnswer(question(type, settings), value, note);
    expect(error !== null).toBe(refused);
    if (error) expect(error.key).toMatch(/^validation\./);
  });

  it("returns the minimum count for the card's disabled Send explanation", () => {
    expect(validateAnswer(question("multiple_choice", { options: ["A", "B", "C"], min: 2, max: 3 }), { options: ["A"] }))
      .toEqual({ key: "validation.minimumOptions", values: { count: 2 } });
  });

  it("refuses a suggestion outside the allowed options", () => {
    expect(validateAnswer(question("single_choice", { options: ["per line", "per total"], allow_other: false }), { option: "per week" }))
      .toEqual({ key: "validation.optionNotAllowed", values: { option: "per week" } });
  });

  it("requires the request-changes note and each incorrect fact's own correction", () => {
    expect(validateAnswer(question("approval", { subject: "Invoice design" }), { decision: "request_changes" }))
      .toEqual({ key: "validation.changesNoteRequired" });
    expect(validateAnswer(question("confirm_facts", { facts: ["Rounding is per line"] }), { facts: [{ correct: false }] }, "An overall note"))
      .toEqual({ key: "validation.factCorrectionRequired", values: { number: 1 } });
  });

  it("does not normalize an impossible calendar day inside a timestamp", () => {
    expect(validateAnswer(question("date", { kind: "datetime" }), { datetime: "2026-02-29T12:00:00Z" }))
      .toEqual({ key: "validation.datetimeRequired" });
  });

  it("orders ranges even inside the same millisecond", () => {
    expect(validateAnswer(question("date", { kind: "range" }), {
      start: "2026-10-13T12:00:00.000000002Z", end: "2026-10-13T12:00:00.000000001Z",
    })).toEqual({ key: "validation.rangeOrder" });
  });

  it("blocks unknown future answer types without throwing", () => {
    expect(validateAnswer(EMPTY_QUESTION, {})).toEqual({ key: "validation.unknownType" });
  });
});
