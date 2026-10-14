"use client";

import { useQuery } from "@tanstack/react-query";
import { CircleMinus } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  formatVersion,
  lastShipped,
  lineStates,
  nextRelease,
  type ReleaseState,
} from "@multica/core/releases";
import { repositoryListOptions } from "@multica/core/repositories";
import { releaseProgressOptions } from "@multica/core/issues/placement";
import type { Repository, Worklane } from "@multica/core/types";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";
import { useReleaseLine } from "../use-release-line";

const NO_REPOSITORIES: Repository[] = [];

const STATE_CLASS: Record<ReleaseState, string> = {
  shipped: "border-info/30 bg-info/10 text-info",
  next: "border-brand/40 bg-brand/5 text-brand",
  planned: "border-border bg-background text-foreground",
};

/**
 * A release's version as a chip, coloured by its state on the line. With
 * `link` it opens that worklane's Timeline at the release, as on worklane rows.
 */
export function ReleaseChip({
  version,
  state,
  link,
  className,
}: {
  /** Without the `v`. */
  version: string;
  state: ReleaseState;
  /** The Timeline the chip opens, and the worklane's name, which tells the chips of several rows apart. */
  link?: { href: string; worklane: string };
  className?: string;
}) {
  const { t } = useT("releases");
  const classes = cn(
    "inline-flex h-5 shrink-0 items-center rounded-full border px-2 font-mono text-caption font-medium whitespace-nowrap",
    STATE_CLASS[state],
    link && "transition-colors hover:bg-accent focus-visible:outline-2 focus-visible:outline-ring",
    className,
  );
  const label = formatVersion(version);
  if (link) {
    return (
      <AppLink
        href={link.href}
        className={classes}
        aria-label={t(($) => $.chips.open_on_worklane, { version: label, worklane: link.worklane })}
        onClick={(e) => e.stopPropagation()}
      >
        {label}
      </AppLink>
    );
  }
  return <span className={classes}>{label}</span>;
}

function Muted({ children }: { children: React.ReactNode }) {
  return <span className="shrink-0 text-caption text-muted-foreground whitespace-nowrap">{children}</span>;
}

/**
 * A worklane row's releases (the `worklane-release-chips` spec delta): its
 * repository's last shipped release or "nothing shipped", then the next
 * release; "No releases planned" on an empty line and "Does not release" for
 * a repository that does not release. Each chip opens the worklane's
 * Timeline at that release.
 */
export function ReleaseChips({
  worklane,
  className,
}: {
  worklane: Pick<Worklane, "id" | "name" | "repository_id">;
  className?: string;
}) {
  const { t } = useT("releases");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const { data: repositories = NO_REPOSITORIES } = useQuery(repositoryListOptions(wsId));
  const repository = repositories.find((r) => r.id === worklane.repository_id);
  const { line, isLoading } = useReleaseLine(worklane.repository_id, repository?.last_tag ?? null);

  const { data: progress } = useQuery(releaseProgressOptions(wsId));
  // The worklane's done and total issues in a release, or null when it holds none.
  const progressOf = (releaseId: string) => {
    const row = progress?.rows.find((r) => r.release_id === releaseId && r.worklane_id === worklane.id);
    return row && row.total > 0 ? row : null;
  };
  const wrap = (children: React.ReactNode) => (
    <span className={cn("inline-flex min-w-0 items-center gap-1.5", className)}>{children}</span>
  );

  if (!repository || isLoading) return wrap(null);

  if (repository.version_policy === "none") {
    return wrap(
      <Muted>
        <span className="inline-flex items-center gap-1">
          <CircleMinus className="size-3" aria-hidden="true" />
          {t(($) => $.chips.does_not_release)}
        </span>
      </Muted>,
    );
  }
  if (line.releases.length === 0) {
    return wrap(<Muted>{t(($) => $.chips.none_planned)}</Muted>);
  }

  const states = lineStates(line);
  const shipped = lastShipped(line);
  const next = nextRelease(line);
  const linkOf = (releaseId: string) => ({
    href: `${paths.worklaneDetail(worklane.id)}?release=${encodeURIComponent(releaseId)}`,
    worklane: worklane.name,
  });

  return wrap(
    <>
      {shipped ? (
        <ReleaseChip version={shipped.version} state="shipped" link={linkOf(shipped.id)} />
      ) : (
        <Muted>{t(($) => $.chips.nothing_shipped)}</Muted>
      )}
      {next ? (
        <>
          <span className="text-caption text-muted-foreground" aria-hidden="true">
            ·
          </span>
          <ReleaseChip
            version={next.version}
            state={states.get(next.id) ?? "next"}
            link={linkOf(next.id)}
          />
        </>
      ) : null}
      {next && progressOf(next.id) ? (
        <span className="shrink-0 text-caption tabular-nums text-muted-foreground">
          {progressOf(next.id)?.done}/{progressOf(next.id)?.total}
        </span>
      ) : null}
      {line.releases
        .filter((release) => states.get(release.id) === "planned" && progressOf(release.id))
        .slice(0, 2)
        .map((release) => (
          <ReleaseChip key={release.id} version={release.version} state="planned" link={linkOf(release.id)} />
        ))}
    </>,
  );
}
