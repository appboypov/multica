// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { QuestionRow } from "../types/question";
import { groupQuestionRows } from "./rows";

function row(id: string, asker: { asker_agent_id: string; asker_user_id: string }): QuestionRow {
  return {
    id,
    ask_id: id,
    ask_position: 0,
    ask_title: null,
    prompt: "Pass the plan?",
    status: "open",
    answer_policy: "any_one",
    ...asker,
    answerer_ids: ["user-brian"],
    due_date: null,
    blocks_work: false,
    created_at: "2026-10-15T12:00:00Z",
    link: { type: "issue", id: "issue-442", identifier: "WORK-442", title: "Ask from outside a run", status: "in_progress", member_id: null },
    via: null,
    open_issue: { id: "issue-442", identifier: "WORK-442" },
    my_answer: null,
    answer_type: "yes_no",
    settings: { why_on_no: false },
  };
}

describe("groupQuestionRows by asker", () => {
  // Scenario: Questions page by asker
  it("groups Pieter's question asked in a run apart from Skuddy's asked outside one", () => {
    const pieters = row("q-1", { asker_agent_id: "agent-pieter", asker_user_id: "" });
    const skuddys = row("q-2", { asker_agent_id: "", asker_user_id: "user-skuddy" });
    const skuddysOther = row("q-3", { asker_agent_id: "", asker_user_id: "user-skuddy" });

    expect(groupQuestionRows([pieters, skuddys, skuddysOther], "asker")).toEqual([
      { kind: "asker", key: "agent-pieter", asker: { type: "agent", id: "agent-pieter" }, rows: [pieters] },
      { kind: "asker", key: "user-skuddy", asker: { type: "member", id: "user-skuddy" }, rows: [skuddys, skuddysOther] },
    ]);
  });
});
