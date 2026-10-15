"use client";

import { useQuery } from "@tanstack/react-query";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { issueDetailOptions } from "@multica/core/issues/queries";
import { useWorkspacePaths } from "@multica/core/paths";
import { questionAsker } from "@multica/core/questions";
import type { Question } from "@multica/core/types";
import { useActorName } from "@multica/core/workspace/hooks";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { ActorAvatar } from "../../common/actor-avatar";
import { useT, useTimeAgo } from "../../i18n";
import { StatusIcon } from "../../issues/components/status-icon";
import { AppLink } from "../../navigation";
import { questionCardHref } from "../questions-page";
import { chatQuestionHref, useChatTitle, usePlaceLabel } from "../row-link";
import { LinkIcon } from "../type-icon";
import type { SessionEntry } from "./session";

export interface AnswerAllContextProps {
  entry: SessionEntry;
  question: Question | undefined;
  /** The questions still waiting after the current one, in session order. */
  upNext: readonly { entry: SessionEntry; question: Question | undefined }[];
  onJump: (id: string) => void;
}

/** The first paragraph of an issue's Markdown description, as plain text. */
function firstParagraph(description: string | null | undefined): string {
  const paragraph = (description ?? "").split(/\n\s*\n/).find((part) => part.trim() !== "") ?? "";
  return paragraph.replace(/[#>*_`]/g, "").replace(/\s+/g, " ").trim();
}

/**
 * Beside One by one's card: the issue the question is on (it opens the issue)
 * and its description's first paragraph, or "Chat with {member}" for a chat,
 * then who asked and when, and **Up next**.
 */
export function AnswerAllContext({ entry, question, upNext, onJump }: AnswerAllContextProps) {
  const { t } = useT("questions");
  const timeAgo = useTimeAgo();
  const { getActorName } = useActorName();
  const placeLabel = usePlaceLabel();
  const { place } = entry;
  const asker = question ? questionAsker(question) : null;

  return (
    <aside className="flex min-w-0 flex-col gap-4 text-body">
      <section className="flex flex-col gap-2 rounded-xl border bg-card p-3.5">
        {"issueId" in place ? (
          <IssuePlace questionId={entry.id} issueId={place.issueId} identifier={place.identifier} />
        ) : (
          <ChatPlace questionId={entry.id} chatSessionId={place.chatSessionId} memberId={place.memberId} />
        )}
        {question && asker && (
          <div className="flex items-center gap-2 text-caption text-muted-foreground">
            <ActorAvatar actorType={asker.type} actorId={asker.id} size="xs" />
            <span className="truncate">
              {t(($) => $.answer_all.asked_by, {
                name: getActorName(asker.type, asker.id),
                time: timeAgo(question.created_at),
              })}
            </span>
          </div>
        )}
      </section>

      {upNext.length > 0 && (
        <section className="flex flex-col gap-1">
          <h2 className="px-1 text-micro font-semibold tracking-[0.08em] text-muted-foreground uppercase">
            {t(($) => $.answer_all.up_next)}
          </h2>
          <ul className="flex flex-col">
            {upNext.map(({ entry: next, question: nextQuestion }) => (
              <li key={next.id}>
                <button
                  type="button"
                  onClick={() => onJump(next.id)}
                  className="flex w-full min-w-0 items-center gap-2 rounded-md px-2 py-1.5 text-left hover:bg-accent"
                >
                  <span className="shrink-0 text-caption text-muted-foreground">{placeLabel(next.place)}</span>
                  <span className="min-w-0 truncate">{nextQuestion?.prompt}</span>
                </button>
              </li>
            ))}
          </ul>
        </section>
      )}
    </aside>
  );
}

/** The issue the question is on, opening its card there, and the description's first paragraph. */
function IssuePlace({ questionId, issueId, identifier }: { questionId: string; issueId: string; identifier: string }) {
  const { t } = useT("questions");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const { data: issue, isPending } = useQuery(issueDetailOptions(wsId, issueId));
  const summary = firstParagraph(issue?.description);

  return (
    <>
      {isPending ? (
        <Skeleton className="h-5 w-3/4" />
      ) : (
        <AppLink
          href={questionCardHref(paths.issueDetail(identifier), questionId)}
          className="flex min-w-0 items-center gap-2 hover:underline"
          title={t(($) => $.answer_all.open_issue, { identifier })}
        >
          {issue && <StatusIcon status={issue.status} className="size-3.5 shrink-0" />}
          <span className="shrink-0 text-caption font-medium text-muted-foreground">{identifier}</span>
          <span className="min-w-0 truncate font-medium">{issue?.title}</span>
        </AppLink>
      )}
      {summary && <p className="line-clamp-4 text-caption text-muted-foreground">{summary}</p>}
    </>
  );
}

/**
 * The chat the question was asked in, named by its member and never by its
 * title; only the member may open it, at the question's card (Decision 6).
 */
function ChatPlace({ questionId, chatSessionId, memberId }: { questionId: string; chatSessionId: string; memberId: string | null }) {
  const paths = useWorkspacePaths();
  const userId = useAuthStore((s) => s.user?.id);
  const chatTitle = useChatTitle();
  const label = (
    <>
      <LinkIcon type="chat" />
      <span className="min-w-0 truncate font-medium">{chatTitle(memberId)}</span>
    </>
  );

  return !!userId && memberId === userId ? (
    <AppLink
      href={chatQuestionHref(questionId, { chatSessionId, memberId }, userId, paths)}
      className="flex min-w-0 items-center gap-2 hover:underline"
    >
      {label}
    </AppLink>
  ) : (
    <div className="flex min-w-0 items-center gap-2">{label}</div>
  );
}
