"use client";

import { GitBranch } from "lucide-react";
import { useWorkspacePaths } from "@multica/core/paths";
import type { Worklane } from "@multica/core/types";
import { cn } from "@multica/ui/lib/utils";
import { AppLink, rowLinkInteractiveProps, useRowLink } from "../../navigation";
import { ReleaseChips } from "../../releases/components/release-chips";
import { WorklaneRowWork } from "./worklane-row-work";

/**
 * The worklane row's own column template: the name with its now or next line,
 * then its releases. A literal class so Tailwind sees it; Worklanes heads its
 * columns with the same template.
 */
export const WORKLANE_TREE_COLS = "grid-cols-[minmax(0,1fr)_minmax(0,24rem)]";

/**
 * One worklane row, the same on Projects, an organisation's page and
 * Worklanes: the name with the branch icon and the now or next line under it,
 * the release chips beside it, and the row opening the worklane's Timeline.
 */
export function WorklaneTreeRow({
  worklane,
  indent,
}: {
  worklane: Worklane;
  /** The row's left padding class at its depth in the tree. */
  indent: string;
}) {
  const wsPaths = useWorkspacePaths();
  const rowLink = useRowLink();
  const href = wsPaths.worklaneDetail(worklane.id);

  return (
    <div
      role="row"
      className={cn(
        "col-span-full grid min-h-12 cursor-pointer items-center gap-x-3 border-b py-1.5 pr-3 transition-colors hover:bg-accent/40",
        WORKLANE_TREE_COLS,
        indent,
      )}
      {...rowLink(href, worklane.name)}
    >
      <div role="cell" className="flex min-w-0 flex-col gap-0.5">
        <AppLink
          href={href}
          {...rowLinkInteractiveProps}
          className="flex min-w-0 items-center gap-1.5 text-caption font-medium hover:underline"
        >
          <GitBranch aria-hidden="true" className="size-3.5 shrink-0 text-muted-foreground" />
          <span className="min-w-0 truncate">{worklane.name}</span>
        </AppLink>
        <WorklaneRowWork worklane={worklane} className="pl-5" />
      </div>
      <div role="cell" className="flex min-w-0 items-center" {...rowLinkInteractiveProps}>
        <ReleaseChips repositoryId={worklane.repository_id} worklaneId={worklane.id} />
      </div>
    </div>
  );
}
