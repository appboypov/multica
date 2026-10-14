import type { IssueTemplate, IssueTemplateSection } from "../../types";

/**
 * The TypeScript port of `server/internal/issuetemplate/markdown.go`. Both
 * run every case of `server/internal/issuetemplate/testdata/cases.json`, so
 * New issue's Markdown shows exactly what the server would write.
 */

type Section = Pick<IssueTemplateSection, "heading" | "question">;

/** The per-scenario sections the scenarios format adds under each scenario line. */
const SCENARIO_SECTIONS: readonly Section[] = [
  { heading: "#### 👤 User Story", question: "Who benefits and what do they want to achieve?" },
  {
    heading: "#### ✨ Scenario",
    question:
      "What do the sequences of { 👤 User, 🧠 System, 🎨 Screen } [ verb ] steps look like when this exists?",
  },
  { heading: "#### 📍 Currently", question: "What is the current state?" },
  { heading: "#### 🎯 Should", question: "What should the state be?" },
  { heading: "#### ⚠️ Constraints", question: "What limitations or constraints exist?" },
  { heading: "#### ✅ Acceptance Criteria", question: "How do we measure successful achievement of this scenario?" },
];

/** Go's `\s` is ASCII whitespace; match it exactly rather than JS's Unicode `\s`. */
const WS = "[\\t\\n\\f\\r ]";
/** A list mark typed at the start of a line: `- `, `* `, `+ ` with an optional `[ ] ` or `[x] `, or `1. ` and `1) `. */
const LIST_MARK = new RegExp(`^${WS}*(?:[-*+]${WS}+(?:\\[[ xX]\\]${WS}+)?|\\d+[.)]${WS}+)`);

export type TemplateFields = Record<string, string>;

/**
 * The template's Markdown with the entered fields converted into the sections
 * they fill. A section without a field, or with an empty one, keeps its
 * heading and question. Sections are joined by a blank line.
 */
export function templateMarkdown(t: IssueTemplate, fields: TemplateFields): string {
  return t.sections.map((s) => sectionMarkdown(s, fields)).join("\n\n");
}

/** Whether the title and every required field hold more than whitespace. */
export function requiredFilled(t: IssueTemplate, title: string, fields: TemplateFields): boolean {
  if (title.trim() === "") return false;
  return t.sections.every((s) => !s.field?.required || (fields[s.field.key] ?? "").trim() !== "");
}

function sectionMarkdown(s: IssueTemplateSection, fields: TemplateFields): string {
  const field = s.field;
  if (!field) return section(s, "");
  const text = (fields[field.key] ?? "").trim();
  if (text === "") return section(s, "");
  switch (field.format) {
    case "bullets":
      return section(s, lines(text).map((item) => `- ${item}`).join("\n"));
    case "checks":
      return section(s, lines(text).map((item) => `- [ ] ${item}`).join("\n"));
    case "scenarios": {
      const parts = [section(s, "")];
      for (const line of lines(text)) {
        const scenario = [`### 👤 ${line}`, ...SCENARIO_SECTIONS.map((sub) => section(sub, ""))];
        parts.push(scenario.join("\n\n"));
      }
      return parts.join("\n\n");
    }
    default:
      return section(s, text);
  }
}

/** A heading, its question as a quote, and the body after a blank line. */
function section(s: Section, body: string): string {
  const parts = [s.heading];
  if (s.question !== "") parts.push(`> ${s.question}`);
  if (body !== "") parts.push(`\n${body}`);
  return parts.join("\n");
}

/** The non-empty lines of the text with typed list marks stripped. */
function lines(text: string): string[] {
  return text
    .split("\n")
    .map((line) => line.replace(LIST_MARK, "").trim())
    .filter((line) => line !== "");
}

