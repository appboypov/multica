import type { WorkspacePaths } from "@multica/core/paths";
import type { QuestionRow, QuestionRowLink } from "@multica/core/types";
import { useActorName } from "@multica/core/workspace/hooks";
import { useT } from "../i18n";

/**
 * How a row's link reads: its title, or "Chat with {member}" for a chat, whose
 * title nobody but its member may see (Decision 6).
 */
export function useLinkTitle(): (link: QuestionRowLink) => string {
  const { t } = useT("questions");
  const { getActorName } = useActorName();
  return (link) =>
    link.type === "chat"
      ? t(($) => $.link.chat_with, { name: getActorName("member", link.member_id ?? "") })
      : link.title;
}

/**
 * Where a row without an issue opens (Decision 6): its chat at the card for
 * the chat's member, Answer all at the question for every other member.
 */
export function chatRowHref(row: QuestionRow, viewerId: string | undefined, paths: WorkspacePaths): string {
  return !!viewerId && row.link.member_id === viewerId
    ? paths.chatSession(row.link.id, { questionId: row.id })
    : paths.answerAll({ questionId: row.id });
}
