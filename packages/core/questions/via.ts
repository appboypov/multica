import type { QuestionVia } from "../types/question";

/** The copy keys of the via line; each language's `questions.json` holds them under `via`. */
export type ViaLabelKey = "issue" | "sub_issue" | "comment" | "comment_on" | "task" | "task_on";

export type ViaTranslate = (key: ViaLabelKey, params: { identifier: string }) => string;

/**
 * The one label rule for how a question reached a page (design.md decision 3):
 * "via sub-issue WORK-415", "via issue WORK-416", "via comment" or "via run" on
 * the issue it sits on, "via comment on WORK-415" and "via run on WORK-415"
 * elsewhere. `pageIssueIdentifier` is the identifier of the issue page the
 * label shows on, null on a project page or a list.
 */
export function viaLabel(
  via: QuestionVia | null,
  pageIssueIdentifier: string | null,
  t: ViaTranslate,
): string | null {
  if (!via) return null;
  if (via.type === "issue" || via.type === "sub_issue") {
    return t(via.type, { identifier: via.identifier ?? via.issue_identifier ?? "" });
  }
  const otherIssue = via.issue_identifier && via.issue_identifier !== pageIssueIdentifier ? via.issue_identifier : null;
  if (via.type === "comment") {
    return otherIssue ? t("comment_on", { identifier: otherIssue }) : t("comment", { identifier: "" });
  }
  return otherIssue ? t("task_on", { identifier: otherIssue }) : t("task", { identifier: "" });
}
