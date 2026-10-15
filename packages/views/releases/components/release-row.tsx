"use client";

import { ChevronRight, GitBranch } from "lucide-react";
import { useWorkspacePaths } from "@multica/core/paths";
import type { ReleaseOverviewRow } from "@multica/core/releases";
import { Progress } from "@multica/ui/components/ui/progress";
import { cn } from "@multica/ui/lib/utils";
import { AppLink } from "../../navigation";
import { useT } from "../../i18n";
import { ReleaseChip } from "./release-chips";
import { ReleaseShippedDate } from "./release-shipped-date";

/**
 * The release row's column template: name, issues, worklanes and the chevron.
 * A literal class so Tailwind sees it; Releases heads its columns with it.
 */
export const RELEASE_ROW_COLS =
  "grid-cols-[minmax(0,1fr)_minmax(6rem,10rem)_minmax(0,16rem)_1rem]";

/**
 * One release on Releases: its version chip, its state, title, shipped date,
 * progress over every issue in it and the worklanes that hold them. The whole
 * row opens the target worklane's Timeline with the release in view. It has
 * no menu: Releases does not plan, edit or move releases.
 */
export function ReleaseRow({ row, indent }: { row: ReleaseOverviewRow; indent: string }) {
  const { t } = useT("releases");
  const wsPaths = useWorkspacePaths();
  const { release, state, setByHand, done, total, worklanes, target } = row;
  const rowClass = cn(
    "col-span-full grid min-h-11 items-center gap-x-3 border-b py-1.5 pr-3 transition-colors",
    RELEASE_ROW_COLS,
    indent,
  );

  const cells = (
    <>
      <span className="flex min-w-0 items-center gap-2">
        <ReleaseChip version={release.version} state={state} />
        <span className="shrink-0 text-caption text-muted-foreground">
          {t(($) => $.column.state[state])}
        </span>
        <ReleaseShippedDate shippedAt={release.shipped_at} />
        <span className="min-w-0 truncate text-caption font-medium">{release.title}</span>
        {setByHand && (
          <span className="shrink-0 text-caption text-muted-foreground">
            {t(($) => $.overview.set_by_hand)}
          </span>
        )}
      </span>
      <span className="flex min-w-0 items-center gap-2">
        <Progress
          value={total > 0 ? (done / total) * 100 : 0}
          aria-label={t(($) => $.overview.progress_aria, { done, total })}
          className="w-16 shrink-0 flex-nowrap"
        />
        <span className="shrink-0 font-mono text-caption tabular-nums text-muted-foreground">
          {done}/{total}
        </span>
      </span>
      <span className="flex min-w-0 items-center gap-1.5 overflow-hidden">
        {total === 0 ? (
          <span className="truncate text-caption text-muted-foreground">
            {t(($) => $.overview.no_issues)}
          </span>
        ) : (
          worklanes.map((worklane) => (
            <span
              key={worklane.id}
              className="inline-flex min-w-0 max-w-40 shrink items-center gap-1 rounded-md border px-1.5 py-0.5 text-caption"
            >
              <GitBranch aria-hidden="true" className="size-3 shrink-0 text-muted-foreground" />
              <span className="min-w-0 truncate">{worklane.name}</span>
            </span>
          ))
        )}
      </span>
      <ChevronRight aria-hidden="true" className="size-4 text-muted-foreground" />
    </>
  );

  // A repository whose worklanes are not loaded has no Timeline to open yet.
  if (!target) {
    return (
      <div className={rowClass}>
        {cells}
      </div>
    );
  }

  return (
    <AppLink
      href={`${wsPaths.worklaneDetail(target.id)}?release=${encodeURIComponent(release.id)}`}
      className={cn(rowClass, "hover:bg-accent/40 focus-visible:bg-accent/40 focus-visible:outline-none")}
    >
      {cells}
    </AppLink>
  );
}
