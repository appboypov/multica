"use client";

import type { ReactNode } from "react";
import { CircleHelp } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  questionParentCountOptions, questionParentRowsOptions, viaLabel, type QuestionParent,
} from "@multica/core/questions";
import type { QuestionRow } from "@multica/core/types";
import { useT } from "../i18n";
import { useNavigation } from "../navigation";
import { questionAnchor } from "../issues/components/comment-runs";
import { TYPE_ICONS } from "./type-icon";

export interface QuestionsPropertyProps {
  /** The issue or project whose open questions, its own and those below it, the property counts. */
  parent: QuestionParent;
  /** The identifier of the issue page the property sits on; null on a project page. */
  pageIssueIdentifier?: string | null;
  /** Jumps to the first open card on this page; without it the count is plain text. */
  onCountClick?: () => void;
  /** Wraps the value in the page's own property row. Called only while a question is open. */
  children: (value: ReactNode) => ReactNode;
}

/**
 * The Questions property of a parent issue or project (design.md decision 4):
 * the open count ("2 open · blocks work") and, under it, the open questions
 * that reached this page through something below it, each marked with how.
 * On an issue page the questions linked to the issue itself are cards on the
 * page and are not listed; a project page has no cards, so it lists them all.
 */
export function QuestionsProperty({ parent, pageIssueIdentifier = null, onCountClick, children }: QuestionsPropertyProps) {
  const { t } = useT("issues");
  const wsId = useWorkspaceId();
  const { data: count } = useQuery(questionParentCountOptions(wsId, parent));
  const { data: page } = useQuery(questionParentRowsOptions(wsId, parent));

  if (!count || count.count === 0) return null;
  const rows = (page?.rows ?? []).filter((row) => parent.type === "project" || row.via !== null);

  const countText = (
    <>
      <CircleHelp aria-hidden className="size-3.5 shrink-0 text-muted-foreground" />
      <span className="text-brand">{t(($) => $.detail.questions_open, { count: count.count })}</span>
      {count.blocking_count > 0 && (
        <span className="text-destructive">· {t(($) => $.detail.questions_blocks_work)}</span>
      )}
    </>
  );

  return children(
    <div className="flex min-w-0 flex-1 flex-col items-start gap-0.5">
      {onCountClick ? (
        <button type="button" className="flex min-w-0 items-center gap-1.5 truncate" onClick={onCountClick}>
          {countText}
        </button>
      ) : (
        <span className="flex min-w-0 items-center gap-1.5 truncate">{countText}</span>
      )}
      {rows.map((row) => (
        <ViaRow key={row.id} row={row} pageIssueIdentifier={pageIssueIdentifier} />
      ))}
    </div>,
  );
}

/** One question below the page: its type glyph, prompt and how it reached the page; opens it on its issue. */
function ViaRow({ row, pageIssueIdentifier }: { row: QuestionRow; pageIssueIdentifier: string | null }) {
  const { t } = useT("questions");
  const { push } = useNavigation();
  const paths = useWorkspacePaths();
  const Icon = TYPE_ICONS[row.answer_type];
  const via = viaLabel(row.via, pageIssueIdentifier, (key, params) => t(($) => $.via[key], params));

  return (
    <button
      type="button"
      className="flex w-full min-w-0 items-center gap-1.5 text-left text-muted-foreground hover:text-foreground"
      onClick={() => push(`${paths.issueDetail(row.open_issue.id)}#${questionAnchor(row.id)}`)}
    >
      <Icon aria-hidden className="size-3.5 shrink-0" />
      <span className="min-w-0 truncate text-foreground">{row.prompt}</span>
      {via && <span className="shrink-0">{via}</span>}
    </button>
  );
}
