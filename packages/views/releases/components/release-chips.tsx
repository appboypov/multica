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
import type { Repository } from "@multica/core/types";
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
 * `href` it is a link, as on worklane rows and the Releases page.
 */
export function ReleaseChip({
  version,
  state,
  href,
  className,
}: {
  /** Without the `v`. */
  version: string;
  state: ReleaseState;
  href?: string;
  className?: string;
}) {
  const { t } = useT("releases");
  const classes = cn(
    "inline-flex h-5 shrink-0 items-center rounded-full border px-2 font-mono text-caption font-medium whitespace-nowrap",
    STATE_CLASS[state],
    href && "transition-colors hover:bg-accent focus-visible:outline-2 focus-visible:outline-ring",
    className,
  );
  const label = formatVersion(version);
  if (href) {
    return (
      <AppLink
        href={href}
        className={classes}
        aria-label={t(($) => $.chips.open, { version: label })}
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
  repositoryId,
  worklaneId,
  className,
}: {
  repositoryId: string;
  worklaneId: string;
  className?: string;
}) {
  const { t } = useT("releases");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const { data: repositories = NO_REPOSITORIES } = useQuery(repositoryListOptions(wsId));
  const repository = repositories.find((r) => r.id === repositoryId);
  const { line, isLoading } = useReleaseLine(repositoryId, repository?.last_tag ?? null);

  if (!repository || isLoading) return null;

  const wrap = (children: React.ReactNode) => (
    <span className={cn("inline-flex min-w-0 items-center gap-1.5", className)}>{children}</span>
  );

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
  const hrefOf = (releaseId: string) =>
    `${paths.worklaneDetail(worklaneId)}?release=${encodeURIComponent(releaseId)}`;

  return wrap(
    <>
      {shipped ? (
        <ReleaseChip version={shipped.version} state="shipped" href={hrefOf(shipped.id)} />
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
            href={hrefOf(next.id)}
          />
        </>
      ) : null}
    </>,
  );
}
