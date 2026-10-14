"use client";

import type { Line, ReleaseState } from "@multica/core/releases";
import type { Release, Repository } from "@multica/core/types";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { ReleaseChip } from "./release-chips";
import { ReleaseMenu } from "./release-menu";
import { SetByHand } from "./set-by-hand";

const NODE_CLASS: Record<ReleaseState, string> = {
  shipped: "border-info bg-info",
  next: "border-brand bg-background",
  planned: "border-ring bg-background",
};

/**
 * One release on a worklane's Timeline: its node on the line, version and
 * state, title, description, Set by hand, the menu of an unshipped release,
 * and the worklane's issues in it as `children`. Without children it says
 * "No issues of this worklane".
 */
export function ReleaseColumn({
  repository,
  release,
  state,
  line,
  findingTag = false,
  highlighted = false,
  worklaneId,
  children,
}: {
  repository: Repository;
  release: Release;
  state: ReleaseState;
  line: Line;
  findingTag?: boolean;
  /** The release `?release={id}` points at. */
  highlighted?: boolean;
  /** The worklane whose Timeline shows the column: its menu adds issues. */
  worklaneId?: string;
  children?: React.ReactNode;
}) {
  const { t } = useT("releases");
  const next = state === "next";
  return (
    <div data-release={release.id} className={cn("shrink-0", children ? "w-[34rem]" : "w-72")}>
      <div className="relative mb-5 h-3" aria-hidden="true">
        <span className="absolute top-1/2 -right-4 left-3 h-px -translate-y-1/2 bg-border" />
        <span
          className={cn(
            "absolute top-1/2 left-0 size-3 -translate-y-1/2 rounded-full border-[1.5px]",
            NODE_CLASS[state],
          )}
        />
      </div>
      <section
        aria-label={`${release.version} ${release.title}`}
        className={cn(
          "rounded-lg border bg-card shadow-xs",
          next ? "border-brand/35" : "border-border",
          highlighted && "ring-2 ring-ring/40",
        )}
      >
        <div className="rounded-t-lg border-b border-border px-4 py-3">
          <div className="flex items-center gap-2">
            <ReleaseChip version={release.version} state={state} />
            <span className={cn("text-caption", next ? "text-brand" : "text-muted-foreground")}>
              {t(($) => $.column.state[state])}
            </span>
            {release.bump === null && state !== "shipped" ? <SetByHand /> : null}
            <div className="flex-1" />
            {state !== "shipped" ? (
              <ReleaseMenu
                repository={repository}
                release={release}
                line={line}
                findingTag={findingTag}
                worklaneId={worklaneId}
              />
            ) : null}
          </div>
          <h3 className="mt-2 text-body-lg font-medium">{release.title}</h3>
          {release.description ? (
            <p className="mt-1 text-label whitespace-pre-wrap text-muted-foreground">{release.description}</p>
          ) : null}
        </div>
        <div className="py-1">
          {children ?? (
            <p className="px-4 py-3 text-label text-muted-foreground">{t(($) => $.column.no_issues)}</p>
          )}
        </div>
      </section>
    </div>
  );
}
