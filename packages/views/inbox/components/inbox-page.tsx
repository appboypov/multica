"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useDefaultLayout } from "react-resizable-panels";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { ArrowLeft, Bell, Inbox } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { useModalStore } from "@multica/core/modals";
import { INBOX_TODO_PARAM, useWorkspacePaths } from "@multica/core/paths";
import {
  getShortcut,
  isEditableShortcutTarget,
  isPortalLayerShortcutTarget,
  shortcutMatchesEvent,
} from "@multica/core/shortcuts";
import { todoListOptions, useMarkTodoDone } from "@multica/core/todos";
import { canMarkTodoDone, type Todo } from "@multica/core/types";
import { isImeComposing } from "@multica/core/utils";
import { ErrorBoundary } from "@multica/ui/components/common/error-boundary";
import { Button } from "@multica/ui/components/ui/button";
import { NumberFlow } from "@multica/ui/components/ui/number-flow";
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from "@multica/ui/components/ui/resizable";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useIsCompact } from "@multica/ui/hooks/use-mobile";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { IssueDetail, issueHighlightMementoKey } from "../../issues/components/issue-detail";
import { PAGE_GUTTER, PageHeader } from "../../layout/page-header";
import { useNavigation } from "../../navigation";
import { useViewStateWriter } from "../../platform";
import { TodoDetailPane, type TodoIssueTarget } from "./todo-detail-pane";
import { TodoListItem } from "./todo-list-item";

const INBOX_LIST_DEFAULT_SIZE = 260;
const INBOX_LIST_MIN_SIZE = 240;
const INBOX_LIST_MAX_SIZE = 400;

/**
 * The row to select once `goneId` leaves `previous`: the next (older) row
 * still listed, else the nearest earlier one, else none.
 */
export function nextTodoAfter(goneId: string, previous: readonly Todo[], current: readonly Todo[]): string {
  const open = new Set(current.map((todo) => todo.id));
  const index = previous.findIndex((todo) => todo.id === goneId);
  if (index < 0) return current[0]?.id ?? "";
  const after = previous.slice(index + 1).find((todo) => open.has(todo.id));
  if (after) return after.id;
  const before = previous.slice(0, index).reverse().find((todo) => open.has(todo.id));
  return before?.id ?? "";
}

function ListSkeleton() {
  return (
    <div className="flex-1 min-h-0 overflow-y-auto space-y-1 p-2">
      {Array.from({ length: 5 }).map((_, i) => (
        <div key={i} className="flex items-center gap-3 px-2 py-2.5">
          <Skeleton className="h-5 w-5 shrink-0 rounded-full" />
          <div className="flex-1 space-y-2">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-3 w-1/2" />
          </div>
        </div>
      ))}
    </div>
  );
}

/**
 * The Inbox: the member's open to-dos, one row each, with the selected
 * to-do's detail on the right. A to-do leaves by itself once handled, or by
 * Mark done; either way the next row is selected.
 */
export function InboxPage() {
  const { t } = useT("inbox");
  const wsId = useWorkspaceId();
  const wsPaths = useWorkspacePaths();
  const { searchParams, replace, push } = useNavigation();
  const urlTodo = searchParams.get(INBOX_TODO_PARAM) ?? "";
  const [selectedId, setSelectedIdState] = useState(urlTodo);
  useEffect(() => {
    setSelectedIdState(urlTodo);
  }, [urlTodo]);

  const listQuery = useQuery(todoListOptions(wsId));
  const todos = listQuery.data;
  const loading = listQuery.isPending;
  const loadFailed = listQuery.isError && !todos;
  const selected = todos?.find((todo) => todo.id === selectedId) ?? null;

  const setSelectedId = useCallback((id: string) => {
    setSelectedIdState(id);
    replace(id ? wsPaths.inboxTodo(id) : wsPaths.inbox());
  }, [replace, wsPaths]);

  // The selected to-do's issue, opened in the pane at the question or comment
  // the to-do is about. Selecting a row, also the same one, returns to its detail.
  const [openedIssue, setOpenedIssue] = useState<{ todoId: string; target: TodoIssueTarget } | null>(null);
  const writeViewState = useViewStateWriter();
  const handleSelect = (todo: Todo) => {
    setOpenedIssue(null);
    setSelectedId(todo.id);
  };
  const openIssue = (todo: Todo, target: TodoIssueTarget) => {
    if (!todo.issue_id) return;
    // A fresh landing on every open, also on an issue opened before.
    writeViewState(issueHighlightMementoKey(todo.issue_id), undefined);
    setOpenedIssue({ todoId: todo.id, target });
  };

  const previousTodosRef = useRef<readonly Todo[] | undefined>(undefined);
  const listFetching = listQuery.isFetching;
  useEffect(() => {
    const previous = previousTodosRef.current;
    previousTodosRef.current = todos;
    if (!todos || !selectedId || todos.some((todo) => todo.id === selectedId)) return;
    const wasListed = previous?.some((todo) => todo.id === selectedId) ?? false;
    // A notification can link a to-do the list is still fetching.
    if (!wasListed && listFetching) return;
    setOpenedIssue(null);
    setSelectedId(wasListed && previous ? nextTodoAfter(selectedId, previous, todos) : "");
  }, [todos, selectedId, setSelectedId, listFetching]);

  const markDone = useMarkTodoDone();
  const handleMarkDone = ({ id, kind }: Todo) => {
    if (!canMarkTodoDone(kind)) return;
    if (id === selectedId && todos) {
      setOpenedIssue(null);
      setSelectedId(nextTodoAfter(id, todos, todos.filter((todo) => todo.id !== id)));
    }
    markDone.mutate(id, {
      onError: (err) =>
        toast.error(err instanceof Error && err.message ? err.message : t(($) => $.errors.mark_done_failed)),
    });
  };

  // The Inbox's archive key marks the selected to-do done. Kept stable while
  // calling the latest handler.
  const markSelectedDoneRef = useRef<(() => void) | null>(null);
  useEffect(() => {
    markSelectedDoneRef.current = selected ? () => handleMarkDone(selected) : null;
  });
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.repeat || isImeComposing(event)) return;
      if (isEditableShortcutTarget(event.target)) return;
      if (isPortalLayerShortcutTarget(event.target)) return;
      if (useModalStore.getState().modal) return;
      if (!shortcutMatchesEvent(getShortcut("archiveInboxItem"), event)) return;
      const run = markSelectedDoneRef.current;
      if (!run) return;
      event.preventDefault();
      run();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, []);

  const { defaultLayout, onLayoutChanged } = useDefaultLayout({ id: "multica_inbox_layout" });
  const isCompact = useIsCompact();
  const count = todos?.length ?? 0;

  const listHeader = (
    <PageHeader>
      <div className="flex min-w-0 items-center gap-2">
        <h1 className="truncate text-body font-semibold">{t(($) => $.page.title)}</h1>
        {count > 0 && (
          <NumberFlow
            value={count}
            animated={false}
            format={{ maximumFractionDigits: 0 }}
            aria-label={String(count)}
            className="shrink-0 text-caption text-muted-foreground"
          />
        )}
      </div>
    </PageHeader>
  );

  const emptyState = (
    <div className="flex flex-col items-center justify-center gap-1 px-6 py-16 text-center" data-testid="todo-empty">
      <Inbox className="mb-2 h-8 w-8 text-faint-foreground" />
      <p className="text-body font-medium">{t(($) => $.todos.empty_title)}</p>
      <p className="text-caption text-muted-foreground">{t(($) => $.todos.empty_body)}</p>
      <Button variant="outline" size="sm" className="mt-3" onClick={() => push(wsPaths.updates())}>
        <Bell />
        {t(($) => $.todos.open_updates)}
      </Button>
    </div>
  );

  const list = loading ? (
    <ListSkeleton />
  ) : loadFailed ? (
    <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
      <p className="text-body">{t(($) => $.todos.load_failed)}</p>
      <Button variant="outline" size="sm" className="mt-3" onClick={() => void listQuery.refetch()}>
        {t(($) => $.list.retry)}
      </Button>
    </div>
  ) : !todos?.length ? (
    emptyState
  ) : (
    <div className="flex-1 min-h-0 overflow-y-auto p-1.5" role="list" aria-label={t(($) => $.page.title)}>
      {todos.map((todo) => (
        <div key={todo.id} role="listitem">
          <TodoListItem
            todo={todo}
            isSelected={todo.id === selectedId}
            onClick={() => handleSelect(todo)}
            onMarkDone={() => handleMarkDone(todo)}
          />
        </div>
      ))}
    </div>
  );

  const listPanel = (
    <div className="flex flex-1 min-h-0 flex-col">
      {listHeader}
      {list}
    </div>
  );

  // Compact widths only: the detail fills the screen there, so the way back
  // to the list lives inside it.
  const compactBackAction = isCompact ? (
    <Button variant="ghost" size="sm" onClick={() => setSelectedId("")} className="-ml-2 shrink-0 gap-1.5 text-muted-foreground">
      <ArrowLeft className="h-4 w-4" />
      {t(($) => $.page.back)}
    </Button>
  ) : undefined;

  const issueOpen = selected?.issue_id && openedIssue?.todoId === selected.id ? openedIssue.target : null;
  const detailContent = selected ? (
    <ErrorBoundary
      resetKeys={[selected.id]}
      fallback={compactBackAction ? ({ error }) => (
        <div className="flex flex-1 min-h-0 flex-col">
          <div className={cn("flex h-12 shrink-0 items-center gap-2 border-b", PAGE_GUTTER)}>{compactBackAction}</div>
          <div className="flex flex-1 min-h-0 items-center justify-center px-4 text-center text-body text-muted-foreground">
            {error.message}
          </div>
        </div>
      ) : undefined}
    >
      {issueOpen && selected.issue_id ? (
        <IssueDetail
          key={selected.issue_id}
          issueId={selected.issue_id}
          defaultSidebarOpen={false}
          layoutId="multica_inbox_issue_detail_layout"
          highlightCommentId={issueOpen.commentId}
          highlightQuestionId={issueOpen.questionId}
          // The split layout already has a nav trigger in the list header.
          leadingAction={compactBackAction ?? false}
          onDelete={() => setSelectedId("")}
        />
      ) : (
        <TodoDetailPane
          key={selected.id}
          todo={selected}
          leadingAction={compactBackAction}
          onMarkDone={() => handleMarkDone(selected)}
          onOpenIssue={(target) => openIssue(selected, target)}
        />
      )}
    </ErrorBoundary>
  ) : null;

  if (isCompact) {
    if (detailContent) return <div className="flex flex-1 flex-col min-h-0">{detailContent}</div>;
    return <div className="flex flex-1 flex-col min-h-0">{listPanel}</div>;
  }

  return (
    <ResizablePanelGroup orientation="horizontal" className="flex-1 min-h-0" defaultLayout={defaultLayout} onLayoutChanged={onLayoutChanged}>
      <ResizablePanel
        id="list"
        defaultSize={INBOX_LIST_DEFAULT_SIZE}
        minSize={INBOX_LIST_MIN_SIZE}
        maxSize={INBOX_LIST_MAX_SIZE}
        groupResizeBehavior="preserve-pixel-size"
      >
        <div className="flex flex-col border-r h-full">{listPanel}</div>
      </ResizablePanel>
      <ResizableHandle />
      <ResizablePanel id="detail" minSize="40%">
        <div className="flex flex-col min-h-0 h-full">
          {detailContent ?? (todos?.length ? (
            <div className="flex h-full flex-col items-center justify-center text-muted-foreground">
              <Inbox className="mb-3 h-10 w-10 text-faint-foreground" />
              <p className="text-body">{t(($) => $.todos.select_prompt)}</p>
            </div>
          ) : null)}
        </div>
      </ResizablePanel>
    </ResizablePanelGroup>
  );
}
