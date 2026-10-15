/** The item kinds delivered to a running run through its receipts. */
export type RunInputItemKind = "comment" | "comment_edit" | "answer" | "issue_change";

/** One run's acknowledgements of an item, separate from people's reactions. */
export interface RunInputMark {
  task_id: string;
  agent_id: string;
  seen_at?: string;
  handled_at?: string;
}
