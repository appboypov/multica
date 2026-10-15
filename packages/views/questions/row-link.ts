import type { WorkspacePaths } from "@multica/core/paths";
import type { QuestionScope } from "@multica/core/questions";
import type { QuestionRow, QuestionRowLink } from "@multica/core/types";
import { useActorName } from "@multica/core/workspace/hooks";
import { useT } from "../i18n";

/**
 * Where a question's card shows, as lists name it: an issue by its
 * identifier, a chat by its member, never by its title (Decision 6).
 */
export type QuestionPlace =
  | { issueId: string; identifier: string }
  | { chatSessionId: string; memberId: string | null };

/** The place of a row's card: its `open_issue`, else the chat its first link is. */
export function rowPlace(row: QuestionRow): QuestionPlace {
  return row.open_issue
    ? { issueId: row.open_issue.id, identifier: row.open_issue.identifier }
    : { chatSessionId: row.link.id, memberId: row.link.member_id };
}

/** The scope a card in that place answers in and refreshes. */
export function placeScope(place: QuestionPlace): QuestionScope {
  return "issueId" in place ? { issueId: place.issueId } : { chatSessionId: place.chatSessionId };
}

/** One key per place, unique across issues and chats. */
export function placeKey(place: QuestionPlace): string {
  return "issueId" in place ? `issue:${place.issueId}` : `chat:${place.chatSessionId}`;
}

/** "Chat with {member}", the only way a chat is named outside it. */
export function useChatTitle(): (memberId: string | null) => string {
  const { t } = useT("questions");
  const { getActorName } = useActorName();
  return (memberId) => t(($) => $.link.chat_with, { name: getActorName("member", memberId ?? "") });
}

/** How a row's link reads: its title, or "Chat with {member}" for a chat. */
export function useLinkTitle(): (link: QuestionRowLink) => string {
  const chatTitle = useChatTitle();
  return (link) => (link.type === "chat" ? chatTitle(link.member_id) : link.title);
}

/** How a place reads: the issue's identifier, or "Chat with {member}". */
export function usePlaceLabel(): (place: QuestionPlace) => string {
  const chatTitle = useChatTitle();
  return (place) => ("issueId" in place ? place.identifier : chatTitle(place.memberId));
}

/**
 * Where a question in a chat opens (Decision 6): the chat at its card for the
 * chat's member, Answer all at the question for every other member.
 */
export function chatQuestionHref(
  questionId: string,
  chat: { chatSessionId: string; memberId: string | null },
  viewerId: string | undefined,
  paths: WorkspacePaths,
): string {
  return !!viewerId && chat.memberId === viewerId
    ? paths.chatSession(chat.chatSessionId, { questionId })
    : paths.answerAll({ questionId });
}

/** Where a row without an issue opens: see `chatQuestionHref`. */
export function chatRowHref(row: QuestionRow, viewerId: string | undefined, paths: WorkspacePaths): string {
  return chatQuestionHref(row.id, { chatSessionId: row.link.id, memberId: row.link.member_id }, viewerId, paths);
}
