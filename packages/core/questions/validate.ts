import { z } from "zod";
import type { AnswerType, Question } from "../types/question";

export interface AnswerValidationError {
  key: `validation.${string}`;
  values?: Record<string, string | number>;
}

const uuidPattern = /^(?:[0-9a-f]{32}|(?:urn:uuid:)?[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}|\{[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}\})$/i;
const ids = z.array(z.string().regex(uuidPattern));
const labels = (min: number) => z.array(z.string().refine((label) => label.trim().length > 0))
  .min(min).max(20).refine((items) => new Set(items).size === items.length);

// Wire schemas are deliberately lenient for installed clients; sending uses
// strict shapes, just like the server's answer registry.
const settingsSchemas: Record<AnswerType, z.ZodType> = {
  text: z.object({ length: z.enum(["short", "long"]), placeholder: z.string().optional() }).strict(),
  yes_no: z.object({ why_on_no: z.boolean() }).strict(),
  single_choice: z.object({ options: labels(2), allow_other: z.boolean() }).strict(),
  multiple_choice: z.object({ options: labels(2), min: z.number().int().nonnegative(), max: z.number().int() })
    .strict().refine((s) => s.min <= s.max && s.max <= s.options.length),
  confirm_facts: z.object({ facts: labels(1) }).strict(),
  ranking: z.object({ items: labels(2) }).strict(),
  scale: z.object({ min: z.number().int(), max: z.number().int(), min_label: z.string().optional(), max_label: z.string().optional() })
    .strict().refine((s) => s.max > s.min && s.max - s.min <= 10),
  number: z.object({ unit: z.string().optional(), min: z.number().optional(), max: z.number().optional() })
    .strict().refine((s) => s.min === undefined || s.max === undefined || s.min <= s.max),
  date: z.object({ kind: z.enum(["date", "datetime", "range"]) }).strict(),
  person: z.object({ allow: z.enum(["members", "agents", "both"]) }).strict(),
  file: z.object({ accept: z.array(z.string().refine((kind) =>
    /^\.[^/\\\s]+$/.test(kind) || /^[!#$%&'*+.^_`|~\w-]+\/[!#$%&'*+.^_`|~\w-]+(?:\s*;\s*[\w-]+=(?:[\w-]+|"[^"]*"))*$/.test(kind))) }).strict(),
  approval: z.object({ subject: z.string().refine((s) => !!s.trim()), link: z.object({
    title: z.string().refine((s) => !!s.trim()), url: z.string().refine((s) => {
      try { const url = new URL(s); return (url.protocol === "http:" || url.protocol === "https:") && !!url.host; }
      catch { return false; }
    }),
  }).strict().optional() }).strict(),
};

const valueSchemas: Record<AnswerType, z.ZodType> = {
  text: z.object({ text: z.string(), attachment_ids: ids.optional() }).strict(),
  yes_no: z.object({ choice: z.enum(["yes", "no"]) }).strict(),
  single_choice: z.object({ option: z.string().optional(), other: z.string().optional() }).strict(),
  multiple_choice: z.object({ options: z.array(z.string()) }).strict(),
  confirm_facts: z.object({ facts: z.array(z.object({ correct: z.boolean(), correction: z.string().optional() }).strict()) }).strict(),
  ranking: z.object({ order: z.array(z.string()) }).strict(),
  scale: z.object({ point: z.number().int() }).strict(),
  number: z.object({ number: z.number() }).strict(),
  date: z.object({ date: z.string().optional(), datetime: z.string().optional(), start: z.string().optional(), end: z.string().optional() }).strict(),
  person: z.object({ type: z.enum(["member", "agent"]), id: z.string().regex(uuidPattern) }).strict(),
  file: z.object({ attachment_ids: ids }).strict(),
  approval: z.object({ decision: z.enum(["approve", "request_changes", "decline"]) }).strict(),
};

function validDate(value: string): boolean {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false;
  const year = Number(value.slice(0, 4));
  const month = Number(value.slice(5, 7));
  const day = Number(value.slice(8, 10));
  const leap = year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0);
  const days = month === 2 ? (leap ? 29 : 28) : ([4, 6, 9, 11].includes(month) ? 30 : 31);
  return month >= 1 && month <= 12 && day >= 1 && day <= days;
}

function timestamp(value: string): bigint | null {
  const match = /^(\d{4}-\d{2}-\d{2})T([01]\d|2[0-3]):([0-5]\d):([0-5]\d)(?:\.(\d+))?(Z|[+-](?:[01]\d|2[0-3]):[0-5]\d)$/.exec(value);
  if (!match || !validDate(match[1]!)) return null;
  const seconds = Date.parse(`${match[1]}T${match[2]}:${match[3]}:${match[4]}${match[6]}`);
  if (!Number.isFinite(seconds)) return null;
  return BigInt(seconds) * 1_000_000n + BigInt((match[5] ?? "").padEnd(9, "0").slice(0, 9));
}

/** Mirrors ValidateAnswer; membership and attachment ownership stay server-side. */
export function validateAnswer(question: Question, value: unknown, note = ""): AnswerValidationError | null {
  if (question.answer_type === "unknown") return { key: "validation.unknownType" };
  if (!settingsSchemas[question.answer_type].safeParse(question.settings).success) return { key: "validation.invalidSettings" };
  if (!valueSchemas[question.answer_type].safeParse(value).success) return { key: "validation.invalidAnswer" };
  const v = value as Record<string, unknown>;
  switch (question.answer_type) {
    case "text":
      return (v.text as string).trim() ? null : { key: "validation.textRequired" };
    case "yes_no":
      return v.choice === "no" && question.settings.why_on_no && !note.trim() ? { key: "validation.noNoteRequired" } : null;
    case "single_choice":
      if ((v.option === undefined) === (v.other === undefined)) return { key: "validation.singleChoiceRequired" };
      if (v.option !== undefined) return question.settings.options.includes(v.option as string) ? null : { key: "validation.optionNotAllowed", values: { option: v.option as string } };
      if (!question.settings.allow_other) return { key: "validation.otherNotAllowed" };
      return (v.other as string).trim() ? null : { key: "validation.otherRequired" };
    case "multiple_choice": {
      const options = v.options as string[];
      if (options.length < question.settings.min) return { key: "validation.minimumOptions", values: { count: question.settings.min } };
      if (options.length > question.settings.max) return { key: "validation.maximumOptions", values: { count: question.settings.max } };
      if (new Set(options).size !== options.length) return { key: "validation.distinctOptions" };
      const foreign = options.find((option) => !question.settings.options.includes(option));
      return foreign === undefined ? null : { key: "validation.optionNotAllowed", values: { option: foreign } };
    }
    case "confirm_facts": {
      const facts = v.facts as { correct: boolean; correction?: string }[];
      if (facts.length !== question.settings.facts.length) return { key: "validation.everyFactRequired" };
      for (let i = 0; i < facts.length; i++) {
        const fact = facts[i]!;
        if (!fact.correct && !fact.correction?.trim()) return { key: "validation.factCorrectionRequired", values: { number: i + 1 } };
        if (fact.correct && fact.correction) return { key: "validation.correctFactCorrection", values: { number: i + 1 } };
      }
      return null;
    }
    case "ranking": {
      const order = v.order as string[];
      return order.length === question.settings.items.length && new Set(order).size === order.length
        && order.every((item) => question.settings.items.includes(item)) ? null : { key: "validation.rankingRequired" };
    }
    case "scale":
      return (v.point as number) >= question.settings.min && (v.point as number) <= question.settings.max
        ? null : { key: "validation.scaleRange", values: { min: question.settings.min, max: question.settings.max } };
    case "number":
      if (question.settings.min !== undefined && (v.number as number) < question.settings.min) return { key: "validation.numberMinimum", values: { min: question.settings.min } };
      if (question.settings.max !== undefined && (v.number as number) > question.settings.max) return { key: "validation.numberMaximum", values: { max: question.settings.max } };
      return null;
    case "date": {
      const keys = Object.keys(v).filter((key) => v[key] !== undefined);
      if (question.settings.kind === "date") return keys.length === 1 && typeof v.date === "string" && validDate(v.date) ? null : { key: "validation.dateRequired" };
      if (question.settings.kind === "datetime") return keys.length === 1 && typeof v.datetime === "string" && timestamp(v.datetime) !== null ? null : { key: "validation.datetimeRequired" };
      const start = typeof v.start === "string" ? timestamp(v.start) : null;
      const end = typeof v.end === "string" ? timestamp(v.end) : null;
      if (keys.length !== 2 || start === null || end === null) return { key: "validation.rangeRequired" };
      return end >= start ? null : { key: "validation.rangeOrder" };
    }
    case "person":
      return question.settings.allow === "both" || (question.settings.allow === "members" ? v.type === "member" : v.type === "agent")
        ? null : { key: "validation.personNotAllowed", values: { allow: question.settings.allow } };
    case "file":
      return (v.attachment_ids as string[]).length > 0 ? null : { key: "validation.fileRequired" };
    case "approval":
      return v.decision === "request_changes" && !note.trim() ? { key: "validation.changesNoteRequired" } : null;
  }
}
