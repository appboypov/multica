import type { QuestionCardGroup } from "@multica/core/questions";

/** A row of the chat list as a question card can follow it. */
export interface ChatRowAnchor {
  /** The row key; a turn's row is `task:<taskId>`, live or persisted. */
  key: string;
  /** When the row's message was sent; null for the live row of a streaming turn. */
  createdAt: string | null;
}

/**
 * The question cards each row of a chat ends with (Decision 7): a card
 * follows the turn that asked it, its assistant message or, while the turn
 * streams, its live row, which share one row key. A card whose turn has no
 * row yet follows the last message sent before it was asked. A card asked
 * before every loaded message waits for the older page holding its turn; with
 * no older page left it follows the first row.
 */
export function placeQuestionCards(
  rows: readonly ChatRowAnchor[],
  cards: readonly QuestionCardGroup[],
  hasOlderRows: boolean,
): Map<string, QuestionCardGroup[]> {
  const placed = new Map<string, QuestionCardGroup[]>();
  if (cards.length === 0 || rows.length === 0) return placed;
  const keys = new Set(rows.map((row) => row.key));
  for (const card of cards) {
    const first = card.questions[0]!;
    const turnKey = `task:${first.asked_in_task_id}`;
    const key = keys.has(turnKey)
      ? turnKey
      : (lastRowBefore(rows, first.created_at) ?? (hasOlderRows ? null : rows[0]!.key));
    if (key === null) continue;
    const onRow = placed.get(key);
    if (onRow) onRow.push(card);
    else placed.set(key, [card]);
  }
  return placed;
}

function lastRowBefore(rows: readonly ChatRowAnchor[], askedAt: string): string | null {
  const asked = Date.parse(askedAt);
  let key: string | null = null;
  for (const row of rows) {
    if (row.createdAt === null || Date.parse(row.createdAt) > asked) continue;
    key = row.key;
  }
  return key;
}
