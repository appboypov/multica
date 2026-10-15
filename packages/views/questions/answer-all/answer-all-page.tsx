"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useInfiniteQuery, useQueries } from "@tanstack/react-query";
import { ArrowLeft, CircleCheck, CircleHelp, LogOut } from "lucide-react";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  EMPTY_QUESTION_ROW_FILTERS,
  issueQuestionsOptions,
  questionDetailOptions,
  questionRowsOptions,
  sessionOrder,
  useQuestionsViewStore,
  type AnswerAllView,
} from "@multica/core/questions";
import type { Question } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { CollectionPageState } from "../../layout/collection-page";
import { PageHeader } from "../../layout/page-header";
import { useNavigation } from "../../navigation";
import { OneByOne } from "./one-by-one";
import { OnePage } from "./one-page";
import { isAnsweredBy, isPageKey, isWaiting, toSession, type SessionEntry } from "./session";
import { SessionWakeBars } from "./wake-bars";

/**
 * Session questions by id, from issue lists or from single question details;
 * stable while none of them changes.
 */
function combineQuestions(results: readonly { data?: Question[] | Question; isSuccess: boolean }[]) {
  const questions = new Map<string, Question>();
  for (const result of results) for (const question of [result.data ?? []].flat()) questions.set(question.id, question);
  return { questions, questionsLoaded: results.every((result) => result.isSuccess) };
}

/**
 * Answer all: every question waiting for the viewer in one sitting. It loads
 * all of For me, then freezes the session, so answered questions stay in it,
 * folded, while their rows leave For me. Each answer is sent on its own;
 * leaving keeps the sent ones and drops unsent drafts.
 */
export function AnswerAllPage() {
  const { t } = useT("questions");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const userId = useAuthStore((s) => s.user?.id);
  const view = useQuestionsViewStore((s) => (userId ? s.answerAllView[userId] : undefined) ?? "one");
  const setView = useQuestionsViewStore((s) => s.setAnswerAllView);

  const rows = useInfiniteQuery(questionRowsOptions(wsId, "for_me", EMPTY_QUESTION_ROW_FILTERS));
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = rows;
  useEffect(() => {
    if (hasNextPage && !isFetchingNextPage) void fetchNextPage();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  const [session, setSession] = useState<SessionEntry[] | null>(null);
  useEffect(() => {
    if (session || !rows.isSuccess || hasNextPage) return;
    setSession(toSession(sessionOrder(rows.data.pages.flatMap((page) => page.rows))));
  }, [session, rows.isSuccess, rows.data, hasNextPage]);

  // An issue's questions load as its list; a chat's list is its member's alone
  // (Decision 6), so a question asked in a chat loads on its own.
  const issueIds = useMemo(
    () => [...new Set((session ?? []).flatMap((entry) => ("issueId" in entry.place ? [entry.place.issueId] : [])))],
    [session],
  );
  const chatQuestionIds = useMemo(
    () => (session ?? []).flatMap((entry) => ("chatSessionId" in entry.place ? [entry.id] : [])),
    [session],
  );
  const issueQuestions = useQueries({
    queries: issueIds.map((issueId) => issueQuestionsOptions(wsId, issueId)),
    combine: combineQuestions,
  });
  const chatQuestions = useQueries({
    queries: chatQuestionIds.map((id) => questionDetailOptions(wsId, id)),
    combine: combineQuestions,
  });
  const questions = useMemo(
    () => new Map([...issueQuestions.questions, ...chatQuestions.questions]),
    [issueQuestions.questions, chatQuestions.questions],
  );
  const questionsLoaded = issueQuestions.questionsLoaded && chatQuestions.questionsLoaded;

  const waiting = useCallback((id: string) => isWaiting(questions.get(id), userId), [questions, userId]);
  const [currentId, setCurrentId] = useState<string | null>(null);
  const [skipped, setSkipped] = useState<ReadonlySet<string>>(() => new Set());
  // `?question=` opens the session at that question, as a row of another member's chat does.
  const requestedId = navigation.searchParams.get("question");
  const current = currentId ?? session?.find((entry) => entry.id === requestedId)?.id ?? session?.[0]?.id ?? "";

  const leave = useCallback(() => navigation.push(paths.questions()), [navigation, paths]);
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || !isPageKey(event)) return;
      event.preventDefault();
      leave();
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [leave]);

  const entries = session ?? [];
  const answeredCount = entries.filter((entry) => isAnsweredBy(questions.get(entry.id), userId)).length;
  const skippedCount = entries.filter((entry) => skipped.has(entry.id) && waiting(entry.id)).length;
  const done = entries.length > 0 && questionsLoaded && entries.every((entry) => !waiting(entry.id));
  const ready = session !== null && questionsLoaded;
  const position = Math.max(entries.findIndex((entry) => entry.id === current), 0) + 1;

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <PageHeader className="gap-3">
        <Button type="button" variant="ghost" size="sm" onClick={leave}>
          <ArrowLeft />
          {t(($) => $.answer_all.back)}
        </Button>
        <h1 className="text-body font-medium">{t(($) => $.answer_all.title)}</h1>
        {ready && entries.length > 0 && (
          <>
            <Tabs value={view} onValueChange={(next) => userId && setView(userId, next as AnswerAllView)}>
              <TabsList aria-label={t(($) => $.answer_all.view_label)}>
                <TabsTrigger value="one">{t(($) => $.answer_all.view_single)}</TabsTrigger>
                <TabsTrigger value="page">{t(($) => $.answer_all.view_page)}</TabsTrigger>
              </TabsList>
            </Tabs>
            <ProgressSegments entries={entries} current={current} questions={questions} userId={userId} skipped={skipped} waiting={waiting} />
            {view === "one" && !done && (
              <span className="shrink-0 text-caption tabular-nums text-muted-foreground">
                {t(($) => $.answer_all.progress, { current: position, total: entries.length })}
              </span>
            )}
            <span className="hidden shrink-0 text-caption tabular-nums text-muted-foreground md:inline">
              {t(($) => $.answer_all.summary, { answered: answeredCount, skipped: skippedCount })}
            </span>
          </>
        )}
        <Button type="button" variant="ghost" size="sm" className="ml-auto" onClick={leave}>
          <LogOut />
          {t(($) => $.answer_all.leave)}
          {/* eslint-disable-next-line i18next/no-literal-string -- key name, not copy */}
          <kbd className="text-caption text-muted-foreground">Esc</kbd>
        </Button>
      </PageHeader>

      <div className="min-h-0 flex-1 overflow-y-auto">
        {rows.isError ? (
          <CollectionPageState
            icon={CircleHelp}
            title={t(($) => $.answer_all.load_failed)}
            tone="destructive"
            role="alert"
            actions={<Button type="button" variant="outline" size="sm" onClick={() => void rows.refetch()}>{t(($) => $.page.retry)}</Button>}
          />
        ) : !ready ? (
          <div className="mx-auto flex w-full max-w-3xl flex-col gap-3 px-4 py-6" role="status" aria-label={t(($) => $.answer_all.loading)}>
            <Skeleton className="h-6 w-1/3" />
            <Skeleton className="h-40 w-full rounded-xl" />
          </div>
        ) : entries.length === 0 ? (
          <CollectionPageState
            icon={CircleHelp}
            title={t(($) => $.answer_all.nothing)}
            actions={<Button type="button" variant="outline" size="sm" onClick={leave}>{t(($) => $.answer_all.back_to_questions)}</Button>}
          />
        ) : done ? (
          <div className="mx-auto flex w-full max-w-3xl flex-col items-center gap-4 px-4 py-10">
            <CircleCheck aria-hidden className="size-8 text-success" />
            <p className="text-title-sm font-semibold">{t(($) => $.answer_all.done, { count: entries.length })}</p>
            <Button type="button" size="sm" onClick={leave}>{t(($) => $.answer_all.back_to_questions)}</Button>
            <div className="w-full">
              <SessionWakeBars session={entries} questions={questions} firstPlace={entries[0]!.place} />
            </div>
          </div>
        ) : view === "one" ? (
          <OneByOne
            session={entries}
            questions={questions}
            waiting={waiting}
            currentId={current}
            onCurrent={setCurrentId}
            skipped={skipped}
            onSkip={(id) => setSkipped((all) => new Set(all).add(id))}
          />
        ) : (
          <OnePage session={entries} questions={questions} focusedId={current} onFocus={setCurrentId} />
        )}
      </div>
    </div>
  );
}

/** One segment per question: answered, current, skipped or still open. */
function ProgressSegments({
  entries,
  current,
  questions,
  userId,
  skipped,
  waiting,
}: {
  entries: readonly SessionEntry[];
  current: string;
  questions: ReadonlyMap<string, Question>;
  userId: string | undefined;
  skipped: ReadonlySet<string>;
  waiting: (id: string) => boolean;
}) {
  return (
    <div className="flex min-w-16 max-w-64 flex-1 items-center gap-0.5" aria-hidden>
      {entries.map((entry) => (
        <span
          key={entry.id}
          className={cn(
            "h-1.5 min-w-1 flex-1 rounded-full",
            entry.id === current
              ? "bg-brand"
              : isAnsweredBy(questions.get(entry.id), userId)
                ? "bg-success"
                : skipped.has(entry.id) && waiting(entry.id)
                  ? "bg-warning"
                  : "bg-muted",
          )}
        />
      ))}
    </div>
  );
}
