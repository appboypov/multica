"use client";

import { useQuery } from "@tanstack/react-query";
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
 * Beside One by one's card: the issue the question is on (it opens the issue),
 * its description's first paragraph, who asked and when, and **Up next**.
 */
export function AnswerAllContext({ entry, question, upNext, onJump }: AnswerAllContextProps) {
  const { t } = useT("questions");
  const timeAgo = useTimeAgo();
  const { getActorName } = useActorName();
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const { data: issue, isPending } = useQuery(issueDetailOptions(wsId, entry.issueId));
  const summary = firstParagraph(issue?.description);
  const asker = question ? questionAsker(question) : null;

  return (
    <aside className="flex min-w-0 flex-col gap-4 text-body">
      <section className="flex flex-col gap-2 rounded-xl border bg-card p-3.5">
        {isPending ? (
          <Skeleton className="h-5 w-3/4" />
        ) : (
          <AppLink
            href={questionCardHref(paths.issueDetail(entry.identifier), entry.id)}
            className="flex min-w-0 items-center gap-2 hover:underline"
            title={t(($) => $.answer_all.open_issue, { identifier: entry.identifier })}
          >
            {issue && <StatusIcon status={issue.status} className="size-3.5 shrink-0" />}
            <span className="shrink-0 text-caption font-medium text-muted-foreground">{entry.identifier}</span>
            <span className="min-w-0 truncate font-medium">{issue?.title}</span>
          </AppLink>
        )}
        {summary && <p className="line-clamp-4 text-caption text-muted-foreground">{summary}</p>}
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
                  <span className="shrink-0 text-caption text-muted-foreground">{next.identifier}</span>
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
