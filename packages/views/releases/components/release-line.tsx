"use client";

import { useEffect, useRef } from "react";
import { Plus } from "lucide-react";
import { useModalStore } from "@multica/core/modals";
import { lineStates } from "@multica/core/releases";
import type { Release, Repository, Worklane } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";
import { useNavigation } from "../../navigation";
import { useLastTag, useReleaseLine } from "../use-release-line";
import { ReleaseColumn } from "./release-column";

/** Plan release, or why a repository that does not release plans none. */
export function PlanReleaseButton({
  repository,
  worklane,
}: {
  repository: Repository;
  worklane: Worklane;
}) {
  const { t } = useT("releases");
  const open = useModalStore((s) => s.open);
  if (repository.version_policy === "none") {
    return (
      <p className="text-body text-muted-foreground">
        {t(($) => $.refusal.does_not_release, { repository: repository.full_name })}
      </p>
    );
  }
  return (
    <Button
      variant="outline"
      onClick={() => open("plan-release", { repositoryId: repository.id, worklaneId: worklane.id })}
    >
      <Plus className="size-4" aria-hidden="true" />
      {t(($) => $.column.plan)}
    </Button>
  );
}

/**
 * The release line on a worklane's Timeline (the `worklane-release-line`
 * spec delta): the repository's releases from left to right in line order,
 * each as a column, then Plan release. `?release={id}` scrolls that release
 * into view. `renderIssues` gives a release's issues of this worklane; without
 * them a column says "No issues of this worklane". `trailing` follows the
 * line, before Plan release: the worklane's "No release".
 */
export function ReleaseLine({
  repository,
  worklane,
  renderIssues,
  trailing,
}: {
  repository: Repository;
  worklane: Worklane;
  renderIssues?: (release: Release) => React.ReactNode;
  trailing?: React.ReactNode;
}) {
  const { releases, line } = useReleaseLine(repository.id, repository.last_tag);
  const lastTag = useLastTag(repository);
  const states = lineStates(line);
  const { searchParams } = useNavigation();
  const target = searchParams.get("release");
  const scroller = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!target) return;
    const node = scroller.current?.querySelector<HTMLElement>(`[data-release="${CSS.escape(target)}"]`);
    node?.scrollIntoView({ behavior: "smooth", block: "nearest", inline: "start" });
  }, [target, releases.length]);

  return (
    <div ref={scroller} className="overflow-x-auto pb-2">
      <div className="flex items-start gap-8">
        {releases.map((release) => (
          <ReleaseColumn
            key={release.id}
            repository={repository}
            release={release}
            state={states.get(release.id) ?? "planned"}
            line={line}
            findingTag={lastTag === "finding"}
            highlighted={release.id === target}
            worklaneId={worklane.id}
          >
            {renderIssues?.(release)}
          </ReleaseColumn>
        ))}
        {trailing}
        <div className="shrink-0 pt-8">
          <PlanReleaseButton repository={repository} worklane={worklane} />
        </div>
      </div>
    </div>
  );
}
