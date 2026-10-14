// Grouping and ordering of the Questions page's rows. The server decides which
// rows a tab holds (ADR-0006); the client only groups and orders loaded rows.

import { addDaysDateOnly, isPastDateOnly, todayDateOnly } from "../issues/date";
import type { QuestionRow, QuestionRowLink } from "../types/question";

export type QuestionGrouping = "link" | "due" | "asker";

export type QuestionDueBucket = "overdue" | "today" | "this_week" | "later" | "no_date";

const DUE_BUCKETS: readonly QuestionDueBucket[] = ["overdue", "today", "this_week", "later", "no_date"];

export type QuestionRowGroup =
  | { kind: "link"; key: string; link: QuestionRowLink; rows: QuestionRow[] }
  | { kind: "due"; key: QuestionDueBucket; rows: QuestionRow[] }
  | { kind: "asker"; key: string; agentId: string; rows: QuestionRow[] };

/** Where a row's due date falls; only an open question is overdue. */
export function dueBucket(row: QuestionRow): QuestionDueBucket {
  if (!row.due_date) return "no_date";
  const open = row.status === "open" || row.status === "partly_answered";
  if (open && isPastDateOnly(row.due_date)) return "overdue";
  const today = todayDateOnly();
  if (row.due_date <= today) return "today";
  return row.due_date <= addDaysDateOnly(7) ? "this_week" : "later";
}

/**
 * Groups loaded rows: by link and by asker in first-seen order, by due date in
 * the fixed order Overdue, Today, This week, Later, No date. Rows keep their
 * order inside a group; empty groups are left out.
 */
export function groupQuestionRows(rows: readonly QuestionRow[], grouping: QuestionGrouping): QuestionRowGroup[] {
  if (grouping === "due") {
    const byBucket = new Map<QuestionDueBucket, QuestionRow[]>();
    for (const row of rows) {
      const bucket = dueBucket(row);
      const bucketRows = byBucket.get(bucket);
      if (bucketRows) bucketRows.push(row);
      else byBucket.set(bucket, [row]);
    }
    return DUE_BUCKETS.flatMap((key) => {
      const bucketRows = byBucket.get(key);
      return bucketRows ? [{ kind: "due" as const, key, rows: bucketRows }] : [];
    });
  }

  const groups = new Map<string, QuestionRowGroup>();
  for (const row of rows) {
    const key = grouping === "link" ? row.link.id : row.asker_agent_id;
    const group = groups.get(key);
    if (group) group.rows.push(row);
    else if (grouping === "link") groups.set(key, { kind: "link", key, link: row.link, rows: [row] });
    else groups.set(key, { kind: "asker", key, agentId: row.asker_agent_id, rows: [row] });
  }
  return [...groups.values()];
}

/**
 * The order Answer all walks the rows in: link groups in first-seen order, and
 * inside a group a set's questions together by `ask_position`, at the place its
 * first loaded question was seen.
 */
export function sessionOrder(rows: readonly QuestionRow[]): QuestionRow[] {
  return groupQuestionRows(rows, "link").flatMap((group) => {
    const asks = new Map<string, QuestionRow[]>();
    for (const row of group.rows) {
      const ask = asks.get(row.ask_id);
      if (ask) ask.push(row);
      else asks.set(row.ask_id, [row]);
    }
    return [...asks.values()].flatMap((ask) => ask.sort((a, b) => a.ask_position - b.ask_position));
  });
}
