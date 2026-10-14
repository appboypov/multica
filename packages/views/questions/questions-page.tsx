"use client";

import { useEffect, useMemo, useRef, type ReactNode } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { CircleAlert, CircleHelp, Filter, Inbox, Layers, Play, RotateCcw } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  EMPTY_QUESTION_ROW_FILTERS,
  groupQuestionRows,
  questionFilterCount,
  questionRowsOptions,
  useQuestionFilters,
  useQuestionsOpenCount,
  useQuestionsViewStore,
  type QuestionGrouping,
  type QuestionListFilter,
  type QuestionRowGroup,
} from "@multica/core/questions";
import { isEditableShortcutTarget, isPortalLayerShortcutTarget } from "@multica/core/shortcuts";
import type { AnswerType, QuestionRow, QuestionRowFilters, QuestionTab } from "@multica/core/types";
import { useActorName } from "@multica/core/workspace/hooks";
import { Button } from "@multica/ui/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import { cn } from "@multica/ui/lib/utils";
import { ActorAvatar } from "../common/actor-avatar";
import { useT } from "../i18n";
import { StatusIcon } from "../issues/components/status-icon";
import { CollectionPageHeader, CollectionPageState } from "../layout/collection-page";
import { useNavigation } from "../navigation";
import { QuestionListRow } from "./question-row";

/** The tabs of the page, in order; `for_me` is the default and leaves `?tab=` out. */
const TABS = ["for_me", "all_open", "answered"] as const satisfies readonly QuestionTab[];
type PageTab = (typeof TABS)[number];

const ANSWER_TYPES: readonly AnswerType[] = [
  "text", "yes_no", "single_choice", "multiple_choice", "confirm_facts", "ranking",
  "scale", "number", "date", "person", "file", "approval",
];

const GROUPINGS: readonly QuestionGrouping[] = ["link", "due", "asker"];

function readTab(value: string | null): PageTab {
  return TABS.find((tab) => tab === value) ?? "for_me";
}

/** The rows of every loaded page, in server order. */
function loadedRows(pages: { rows: QuestionRow[] }[] | undefined): QuestionRow[] {
  return pages?.flatMap((page) => page.rows) ?? [];
}

/** The URL of a question's card on its issue, highlighted on arrival. */
export function questionCardHref(issueHref: string, questionId: string): string {
  return `${issueHref}#question-${questionId}`;
}

/**
 * The Questions page: every question waiting on the viewer (For me), every
 * open one in the workspace (All open) and the viewer's answers (Answered),
 * grouped by link, due date or asker, with Answer all over For me.
 */
export function QuestionsPage() {
  const { t } = useT("questions");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const tab = readTab(navigation.searchParams.get("tab"));
  const filters = useQuestionFilters(wsId);
  const grouping = useQuestionsViewStore((s) => s.grouping);
  const openCount = useQuestionsOpenCount(wsId);
  const listRef = useRef<HTMLDivElement>(null);

  // For me and All open stay loaded for their tab counts; Answered loads when shown.
  const forMe = useInfiniteQuery(questionRowsOptions(wsId, "for_me", filters));
  const allOpen = useInfiniteQuery(questionRowsOptions(wsId, "all_open", filters));
  const answered = useInfiniteQuery({ ...questionRowsOptions(wsId, "answered", filters), enabled: !!wsId && tab === "answered" });
  const active = tab === "for_me" ? forMe : tab === "all_open" ? allOpen : answered;
  const rows = useMemo(() => loadedRows(active.data?.pages), [active.data]);
  const groups = useMemo(() => groupQuestionRows(rows, grouping), [rows, grouping]);

  // The proof the page works when nothing waits: the viewer's newest answer.
  const forMeEmpty = tab === "for_me" && active.isSuccess && rows.length === 0;
  const latestAnswered = useInfiniteQuery({
    ...questionRowsOptions(wsId, "answered", EMPTY_QUESTION_ROW_FILTERS),
    enabled: !!wsId && forMeEmpty,
  });
  const latestAnsweredRow = latestAnswered.data?.pages[0]?.rows[0];

  const openRow = (row: QuestionRow) => {
    navigation.push(questionCardHref(paths.issueDetail(row.open_issue.identifier), row.id));
  };

  const selectTab = (next: PageTab) => {
    navigation.replace(next === "for_me" ? paths.questions() : `${paths.questions()}?tab=${next}`);
  };

  // J / K and the arrows move the focus between rows, outside editable targets and open layers.
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey) return;
      if (isEditableShortcutTarget(event.target) || isPortalLayerShortcutTarget(event.target)) return;
      const step = event.key === "j" || event.key === "ArrowDown" ? 1 : event.key === "k" || event.key === "ArrowUp" ? -1 : 0;
      if (step === 0 || !listRef.current) return;
      const rowElements = [...listRef.current.querySelectorAll<HTMLElement>("[data-question-row]")];
      if (rowElements.length === 0) return;
      event.preventDefault();
      const current = rowElements.findIndex((element) => element === document.activeElement);
      const next = current === -1 ? (step === 1 ? 0 : rowElements.length - 1) : Math.min(Math.max(current + step, 0), rowElements.length - 1);
      rowElements[next]?.focus();
      rowElements[next]?.scrollIntoView({ block: "nearest" });
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  const tabCount = (value: PageTab): number | null => {
    if (value === "answered") return null;
    const query = value === "for_me" ? forMe : allOpen;
    return query.data ? loadedRows(query.data.pages).length : null;
  };

  let body: ReactNode;
  if (active.isPending) {
    body = (
      <div className="flex flex-col gap-1 p-4" role="status" aria-label={t(($) => $.page.loading)}>
        {[0, 1, 2, 3].map((index) => <Skeleton key={index} className="h-9 w-full" />)}
      </div>
    );
  } else if (active.isError) {
    body = (
      <CollectionPageState
        icon={CircleAlert}
        tone="destructive"
        role="alert"
        title={t(($) => $.page.load_failed)}
        actions={<Button type="button" variant="outline" size="sm" onClick={() => active.refetch()}>{t(($) => $.page.retry)}</Button>}
      />
    );
  } else if (rows.length === 0) {
    const filtered = questionFilterCount(filters) > 0;
    body = (
      <div className="flex flex-col">
        <CollectionPageState
          icon={Inbox}
          title={
            filtered
              ? t(($) => $.page.empty_filtered)
              : tab === "for_me"
                ? t(($) => $.page.empty_for_me)
                : tab === "all_open"
                  ? t(($) => $.page.empty_all_open)
                  : t(($) => $.page.empty_answered)
          }
        />
        {tab === "for_me" && latestAnsweredRow && (
          <div className="flex flex-col border-t">
            <p className="px-4 pt-3 pb-1 text-caption text-muted-foreground">{t(($) => $.page.latest_answered)}</p>
            <QuestionListRow row={latestAnsweredRow} onOpen={openRow} />
          </div>
        )}
      </div>
    );
  } else {
    body = (
      <>
        {groups.map((group) => (
          <section key={`${group.kind}:${group.key}`} aria-label={groupLabelText(group)}>
            <GroupHeading group={group} />
            <div role="list">
              {group.rows.map((row) => <QuestionListRow key={row.id} row={row} onOpen={openRow} />)}
            </div>
          </section>
        ))}
        {active.hasNextPage && (
          <div className="flex justify-center p-3">
            <Button type="button" variant="ghost" size="sm" disabled={active.isFetchingNextPage} onClick={() => active.fetchNextPage()}>
              {t(($) => $.page.load_more)}
            </Button>
          </div>
        )}
      </>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <CollectionPageHeader
        icon={CircleHelp}
        title={t(($) => $.page.title)}
        actions={
          <>
            <QuestionsFilterMenu wsId={wsId} rows={rows} filters={filters} />
            <GroupingMenu grouping={grouping} />
            <Button type="button" size="sm" disabled={openCount === 0} onClick={() => navigation.push(paths.answerAll())}>
              <Play />
              {t(($) => $.page.answer_all, { count: openCount })}
            </Button>
          </>
        }
      />
      <Tabs value={tab} onValueChange={(value) => selectTab(readTab(value as string))} className="border-b px-4">
        <TabsList variant="line">
          {TABS.map((value) => {
            const count = tabCount(value);
            return (
              <TabsTrigger key={value} value={value}>
                {t(($) => $.page.tabs[value])}
                {count !== null && count > 0 && <span className="text-caption tabular-nums text-muted-foreground">{count}</span>}
              </TabsTrigger>
            );
          })}
        </TabsList>
      </Tabs>
      <div ref={listRef} className="min-h-0 flex-1 overflow-y-auto">
        {body}
      </div>
    </div>
  );
}

function groupLabelText(group: QuestionRowGroup): string {
  if (group.kind === "link") return group.link.identifier ? `${group.link.identifier} ${group.link.title}` : group.link.title;
  return group.key;
}

/** A group's heading: the linked issue, the due bucket or the asking agent, and its row count. */
export function GroupHeading({ group }: { group: QuestionRowGroup }) {
  const { t } = useT("questions");
  const { getActorName } = useActorName();
  return (
    <div className="sticky top-0 z-10 flex h-8 items-center gap-2 border-b bg-muted/60 px-4 text-caption backdrop-blur">
      {group.kind === "link" && (
        <>
          {group.link.status && <StatusIcon status={group.link.status} className="size-3.5" />}
          {group.link.identifier && <span className="shrink-0 font-medium text-muted-foreground">{group.link.identifier}</span>}
          <span className="min-w-0 truncate font-medium">{group.link.title}</span>
        </>
      )}
      {group.kind === "due" && <span className="font-medium">{t(($) => $.page.due[group.key])}</span>}
      {group.kind === "asker" && (
        <>
          <ActorAvatar actorType="agent" actorId={group.agentId} size="xs" />
          <span className="min-w-0 truncate font-medium">{getActorName("agent", group.agentId)}</span>
        </>
      )}
      <span className="shrink-0 tabular-nums text-muted-foreground">{group.rows.length}</span>
    </div>
  );
}

function GroupingMenu({ grouping }: { grouping: QuestionGrouping }) {
  const { t } = useT("questions");
  const setGrouping = useQuestionsViewStore((s) => s.setGrouping);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={<Button type="button" variant="ghost" size="sm" className="text-muted-foreground" aria-label={t(($) => $.page.grouping.label)} />}
      >
        <Layers />
        {t(($) => $.page.grouping[grouping])}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-auto min-w-40">
        <DropdownMenuRadioGroup value={grouping} onValueChange={(value) => setGrouping(value as QuestionGrouping)}>
          {GROUPINGS.map((value) => (
            <DropdownMenuRadioItem key={value} value={value}>{t(($) => $.page.grouping[value])}</DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

interface FilterOption {
  value: string;
  label: string;
  icon?: ReactNode;
}

/** Each value once, in first-seen order, with the picked ones kept even when no loaded row has them. */
function options(seen: readonly string[], picked: readonly string[], label: (value: string) => FilterOption): FilterOption[] {
  return [...new Set([...seen, ...picked])].map(label);
}

function QuestionsFilterMenu({ wsId, rows, filters }: { wsId: string; rows: readonly QuestionRow[]; filters: QuestionRowFilters }) {
  const { t } = useT("questions");
  const { getActorName } = useActorName();
  const toggleFilter = useQuestionsViewStore((s) => s.toggleFilter);
  const toggleBlocking = useQuestionsViewStore((s) => s.toggleBlocking);
  const clearFilters = useQuestionsViewStore((s) => s.clearFilters);
  const activeCount = questionFilterCount(filters);
  const linkTitles = new Map(rows.map((row) => [row.link.id, row.link.identifier ?? row.link.title]));

  const dimensions: { key: QuestionListFilter; label: string; options: FilterOption[] }[] = [
    {
      key: "answer_types",
      label: t(($) => $.page.filters.answer_type),
      options: ANSWER_TYPES.map((value) => ({ value, label: t(($) => $.types[value]) })),
    },
    {
      key: "asker_ids",
      label: t(($) => $.page.filters.asker),
      options: options(rows.map((row) => row.asker_agent_id), filters.asker_ids, (value) => ({
        value,
        label: getActorName("agent", value),
        icon: <ActorAvatar actorType="agent" actorId={value} size="xs" />,
      })),
    },
    {
      key: "answerer_ids",
      label: t(($) => $.page.filters.answerer),
      options: options(rows.flatMap((row) => row.answerer_ids), filters.answerer_ids, (value) => ({
        value,
        label: getActorName("member", value),
        icon: <ActorAvatar actorType="member" actorId={value} size="xs" />,
      })),
    },
    {
      key: "link_ids",
      label: t(($) => $.page.filters.link),
      options: options(rows.map((row) => row.link.id), filters.link_ids, (value) => ({
        value,
        label: linkTitles.get(value) ?? value,
      })),
    },
  ];
  const label = t(($) => $.page.filters.label);

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            type="button"
            variant={activeCount > 0 ? "default" : "ghost"}
            size="icon-sm"
            aria-label={label}
            title={label}
            className={cn("text-muted-foreground", activeCount > 0 && "w-auto gap-1 bg-brand px-2 text-white hover:bg-brand/90")}
          />
        }
      >
        <Filter className="size-4" />
        {activeCount > 0 && <span className="text-caption tabular-nums">{activeCount}</span>}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-auto min-w-44">
        {dimensions.map((dimension) => (
          <DropdownMenuSub key={dimension.key}>
            <DropdownMenuSubTrigger>
              {dimension.label}
              {filters[dimension.key].length > 0 && (
                <span className="ml-auto text-caption tabular-nums text-muted-foreground">{filters[dimension.key].length}</span>
              )}
            </DropdownMenuSubTrigger>
            <DropdownMenuSubContent className="max-h-80 w-auto min-w-44 overflow-y-auto">
              {dimension.options.map((option) => (
                <DropdownMenuCheckboxItem
                  key={option.value}
                  checked={(filters[dimension.key] as readonly string[]).includes(option.value)}
                  onCheckedChange={() => toggleFilter(wsId, dimension.key, option.value)}
                >
                  {option.icon}
                  <span className="truncate">{option.label}</span>
                </DropdownMenuCheckboxItem>
              ))}
            </DropdownMenuSubContent>
          </DropdownMenuSub>
        ))}
        <DropdownMenuSeparator />
        <DropdownMenuCheckboxItem checked={filters.blocks_work} onCheckedChange={() => toggleBlocking(wsId)}>
          {t(($) => $.page.filters.blocking)}
        </DropdownMenuCheckboxItem>
        {activeCount > 0 && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => clearFilters(wsId)}>
              <RotateCcw className="size-3.5" />
              {t(($) => $.page.filters.clear)}
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
