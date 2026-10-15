"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { DndContext, PointerSensor, KeyboardSensor, closestCenter, useSensor, useSensors, type DragEndEvent } from "@dnd-kit/core";
import { SortableContext, sortableKeyboardCoordinates, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { restrictToVerticalAxis } from "@dnd-kit/modifiers";
import { toast } from "sonner";
import { useWorkspaceId } from "@multica/core/hooks";
import { worklaneIssuesOptions } from "@multica/core/issues/placement";
import { getIssueSurfaceViewStore } from "@multica/core/issues/stores/surface-view-store";
import { ViewStoreProvider } from "@multica/core/issues/stores/view-store-context";
import { releaseListOptions, selectLine, toLine, type Line } from "@multica/core/releases";
import {
  listSections,
  useMoveWorklaneIssue,
  type LaneMoveMutation,
  typeFilter,
  type IssueTypePick,
  type NoReleaseListSection,
  type ReleaseListSection,
} from "@multica/core/worklanes";
import type { Issue, Repository, Worklane } from "@multica/core/types";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { IssueContextMenuProvider } from "../../issues/actions";
import { DraggableListRow, ListRow } from "../../issues/components/list-row";
import { ReleaseChip } from "../../releases/components/release-chips";
import { ReleaseMenu } from "../../releases/components/release-menu";
import { ReleaseShippedDate } from "../../releases/components/release-shipped-date";
import { useLastTag } from "../../releases/use-release-line";
import { useT } from "../../i18n";

/**
 * A worklane's List: a section per release of the repository's line, in line
 * order, with this worklane's issues in it, then "No release" while any issue
 * is in no release. Rows are the issue list's own, reading the card
 * properties from the worklane's surface store that its Board writes.
 */
export function WorklaneList({
  worklane,
  repository,
  issueTypes,
}: {
  worklane: Worklane;
  repository: Repository;
  issueTypes: readonly IssueTypePick[];
}) {
  const { t } = useT("worklanes");
  const { t: tReleases } = useT("releases");
  const wsId = useWorkspaceId();
  const issuesQuery = useQuery(worklaneIssuesOptions(wsId, worklane.id));
  const releasesQuery = useQuery(releaseListOptions(wsId));
  const store = useMemo(() => getIssueSurfaceViewStore(`worklane:${worklane.id}`), [worklane.id]);
  const move = useMoveWorklaneIssue(worklane.id);

  const { sections, line } = useMemo(() => {
    const releases = selectLine(releasesQuery.data ?? [], repository.id);
    const issues = (issuesQuery.data ?? []).filter(typeFilter(issueTypes));
    return {
      sections: listSections(releases, issues),
      line: toLine(releases, repository.last_tag),
    };
  }, [releasesQuery.data, issuesQuery.data, repository.id, repository.last_tag, issueTypes]);

  if (issuesQuery.isLoading || releasesQuery.isLoading) {
    return (
      <div className="flex flex-col gap-2 p-4" aria-busy="true">
        <Skeleton className="h-10 w-full rounded-lg" />
        <Skeleton className="h-9 w-full" />
        <Skeleton className="h-9 w-full" />
      </div>
    );
  }

  if (sections.length === 0) {
    return (
      <p className="px-6 py-10 text-center text-body text-muted-foreground">
        {repository.version_policy === "none"
          ? tReleases(($) => $.refusal.does_not_release, { repository: repository.full_name })
          : t(($) => $.list.no_releases)}
      </p>
    );
  }

  return (
    <ViewStoreProvider store={store}>
      <IssueContextMenuProvider>
        <div className="flex flex-col gap-4 overflow-y-auto p-4">
          {sections.map((section) =>
            section.kind === "release" ? (
              <ReleaseSection
                key={section.release.id}
                section={section}
                repository={repository}
                line={line}
                move={move}
              />
            ) : (
              <NoReleaseSection key="no_release" section={section} move={move} />
            ),
          )}
        </div>
      </IssueContextMenuProvider>
    </ViewStoreProvider>
  );
}

function ReleaseSection({
  section,
  repository,
  line,
  move,
}: {
  section: ReleaseListSection;
  repository: Repository;
  line: Line;
  move: LaneMoveMutation;
}) {
  const { t: tReleases } = useT("releases");
  const { release } = section;
  const lastTag = useLastTag(repository);
  return (
    // The release's own version and title: data, not copy.
    // eslint-disable-next-line no-restricted-syntax
    <section aria-label={`v${release.version} ${release.title}`}>
      <header className="flex min-h-10 flex-wrap items-center gap-2 rounded-lg bg-muted px-3 py-1.5">
        <ReleaseChip version={release.version} state={section.state} />
        <span className="text-caption text-muted-foreground">
          {tReleases(($) => $.column.state[section.state])}
        </span>
        <ReleaseShippedDate shippedAt={release.shipped_at} />
        <span className="min-w-0 truncate font-medium">{release.title}</span>
        {section.setByHand && (
          <span className="text-caption text-muted-foreground">
            {tReleases(($) => $.column.set_by_hand)}
          </span>
        )}
        <span className="ml-auto text-caption tabular-nums text-muted-foreground">
          {section.total > 0
            ? tReleases(($) => $.issues.progress, { done: section.done, total: section.total })
            : tReleases(($) => $.column.no_issues)}
        </span>
        <ReleaseMenu
          repository={repository}
          release={release}
          line={line}
          findingTag={lastTag === "finding"}
        />
      </header>
      {release.description && (
        <p className="whitespace-pre-wrap px-4 pt-2 text-caption text-muted-foreground">
          {release.description}
        </p>
      )}
      <WorklaneIssueRows issues={section.issues} move={move} />
    </section>
  );
}

function NoReleaseSection({ section, move }: { section: NoReleaseListSection; move: LaneMoveMutation }) {
  const { t } = useT("worklanes");
  const { t: tReleases } = useT("releases");
  return (
    <section aria-label={tReleases(($) => $.issues.no_release)}>
      <header className="flex h-10 items-center gap-2 rounded-lg bg-muted px-3">
        <span className="font-medium">{tReleases(($) => $.issues.no_release)}</span>
        <span className="ml-auto text-caption tabular-nums text-muted-foreground">
          {t(($) => $.list.issue_count, { count: section.issues.length })}
        </span>
      </header>
      <WorklaneIssueRows issues={section.issues} move={move} />
    </section>
  );
}

export function WorklaneIssueRows({ issues, move }: { issues: readonly Issue[]; move: Pick<LaneMoveMutation, "isPending" | "mutate"> }) {
  const { t } = useT("issues");
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );
  const onDragEnd = ({ active, over }: DragEndEvent) => {
    if (!over || active.id === over.id || move.isPending) return;
    const oldIndex = issues.findIndex((issue) => issue.id === active.id);
    const newIndex = issues.findIndex((issue) => issue.id === over.id);
    if (oldIndex < 0 || newIndex < 0) return;
    const issueId = String(active.id);
    const target = String(over.id);
    move.mutate(oldIndex < newIndex ? { issueId, after: target } : { issueId, before: target }, {
      onError: () => toast.error(t(($) => $.page.move_failed)),
    });
  };
  if (issues.length === 0) return null;
  return (
    <DndContext sensors={sensors} collisionDetection={closestCenter} modifiers={[restrictToVerticalAxis]} onDragEnd={onDragEnd}>
      <SortableContext items={issues.map((issue) => issue.id)} strategy={verticalListSortingStrategy}>
        <div className="pt-1" aria-busy={move.isPending}>
          {issues.map((issue) => move.isPending
            ? <ListRow key={issue.id} issue={issue} />
            : <DraggableListRow key={issue.id} issue={issue} />)}
        </div>
      </SortableContext>
    </DndContext>
  );
}
