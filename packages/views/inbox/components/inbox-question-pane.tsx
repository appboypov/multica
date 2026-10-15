"use client";

import { useMemo, type ReactNode } from "react";
import { ArrowUpRight } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { issueDetailOptions, issueTasksOptions } from "@multica/core/issues/queries";
import { groupQuestionCards, issueQuestionsOptions, useQuestion } from "@multica/core/questions";
import type { InboxItem, Question } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { cn } from "@multica/ui/lib/utils";
import { useT, useTimeAgo } from "../../i18n";
import { runNumbersByTask } from "../../issues/components/task-run-labels";
import { QuestionCard } from "../../questions";
import { useTypeLabels } from "./inbox-detail-label";

/** The answer types the inbox answers in place; every other type is answered on its issue. */
const ANSWERED_IN_PLACE: ReadonlySet<Question["answer_type"]> = new Set(["yes_no", "single_choice", "approval"]);

/**
 * The question a to-do or an inbox item is about: its card, answerable in
 * place for Yes / No, Single choice and Approval, and for any other type the
 * way to its card on the issue. A set shows the set's title and each of its
 * questions.
 */
export function InboxQuestionBody({
  issueId,
  questionId,
  fallbackIdentifier,
  onOpenIssue,
}: {
  issueId: string;
  questionId: string;
  /** Names the issue in "Open on …" until the issue has loaded. */
  fallbackIdentifier: string;
  /** Opens the issue at the question's card, with `questionId` active when given. */
  onOpenIssue: (questionId?: string) => void;
}) {
  const { t } = useT("inbox");
  const { t: tQuestions } = useT("questions");
  const wsId = useWorkspaceId();
  const { data: question, isPending } = useQuestion(wsId, questionId);
  const { data: issue } = useQuery(issueDetailOptions(wsId, issueId));
  const { data: tasks } = useQuery(issueTasksOptions(issueId));
  const runNumber = useMemo(
    () => (question && tasks ? runNumbersByTask(tasks).get(question.asked_in_task_id) : undefined),
    [question, tasks],
  );

  const shown = question && question.id !== "" ? question : null;
  const isSet = !!shown && shown.ask_title !== null;
  const { data: issueQuestions } = useQuery({ ...issueQuestionsOptions(wsId, issueId), enabled: isSet && !!wsId && !!issueId });
  const setQuestions = useMemo(
    () => (isSet && issueQuestions ? groupQuestionCards(issueQuestions).find((card) => card.askId === shown?.ask_id)?.questions ?? null : null),
    [isSet, issueQuestions, shown],
  );
  const inPlace = !!shown && ANSWERED_IN_PLACE.has(shown.answer_type);
  const openLabel = t(($) => $.question_pane.open_on, { identifier: issue?.identifier ?? fallbackIdentifier });
  const openButton = (activeQuestionId?: string) => (
    <Button type="button" size="sm" className="ml-auto shrink-0" onClick={() => onOpenIssue(activeQuestionId)}>
      <ArrowUpRight />
      {openLabel}
    </Button>
  );
  const answeredOnIssue = (q: Question) => (
    <div className="flex items-center gap-3">
      <p className="text-caption text-muted-foreground">
        {t(($) => $.question_pane.answered_on_issue, { type: tQuestions(($) => $.types[q.answer_type]) })}
      </p>
      {openButton(q.id)}
    </div>
  );

  if (isPending || (isSet && !setQuestions && !issueQuestions)) {
    return <Skeleton className="h-40 w-full rounded-xl" />;
  }
  if (setQuestions) {
    return (
      <div className="flex flex-col gap-3">
        <p className="text-title-sm font-semibold">{shown?.ask_title}</p>
        {setQuestions.map((q) => (
          <QuestionCard
            key={q.id}
            question={q}
            issueId={issueId}
            runNumber={runNumber}
            hideWake
            askQuestions={setQuestions}
            answerElsewhere={ANSWERED_IN_PLACE.has(q.answer_type) ? undefined : answeredOnIssue(q)}
          />
        ))}
      </div>
    );
  }
  if (shown) {
    return (
      <QuestionCard
        question={shown}
        issueId={issueId}
        runNumber={runNumber}
        askQuestions={[shown]}
        answerElsewhere={inPlace ? undefined : answeredOnIssue(shown)}
      />
    );
  }
  return (
    <div className="flex items-center gap-3 rounded-xl border bg-card p-3.5">
      <p className="text-caption text-muted-foreground">{t(($) => $.question_pane.unavailable)}</p>
      {openButton()}
    </div>
  );
}

/** The detail pane of a question's inbox notice: its head and the question. */
export function InboxQuestionPane({
  item,
  issueId,
  questionId,
  onOpenIssue,
  leadingAction,
  actions,
  className,
}: {
  item: InboxItem;
  issueId: string;
  questionId: string;
  /** Opens the issue at the question's card, with `questionId` active when given. */
  onOpenIssue: (questionId?: string) => void;
  /** The compact layout's way back to the list. */
  leadingAction?: ReactNode;
  /** The item's own actions, such as Archive. */
  actions?: ReactNode;
  className?: string;
}) {
  const { t } = useT("inbox");
  const timeAgo = useTimeAgo();
  const typeLabels = useTypeLabels();
  const wsId = useWorkspaceId();
  const { data: question } = useQuestion(wsId, questionId);
  const { data: issue } = useQuery(issueDetailOptions(wsId, issueId));
  const shown = question && question.id !== "" ? question : null;
  const showOpen = !!shown && (ANSWERED_IN_PLACE.has(shown.answer_type) || shown.ask_title !== null);

  return (
    <div className={cn("flex min-h-0 flex-1 flex-col", className)}>
      <div className="flex h-12 shrink-0 items-center gap-2 border-b px-4">
        {leadingAction}
        <span className="truncate text-body font-medium">{typeLabels[item.type]}</span>
        <span className="shrink-0 text-caption text-muted-foreground">· {timeAgo(item.created_at)}</span>
        <span className="ml-auto flex shrink-0 items-center gap-1">
          {showOpen && (
            <Button type="button" variant="ghost" size="sm" className="text-muted-foreground" onClick={() => onOpenIssue()}>
              <ArrowUpRight />
              {t(($) => $.question_pane.open_on, { identifier: issue?.identifier ?? item.title })}
            </Button>
          )}
          {actions}
        </span>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto p-6">
        <div className="mx-auto flex max-w-3xl flex-col gap-4">
          <div className="flex flex-col gap-0.5">
            {issue?.identifier && <span className="text-caption text-muted-foreground">{issue.identifier}</span>}
            <h2 className="text-title font-semibold">{issue?.title ?? item.title}</h2>
          </div>
          <InboxQuestionBody issueId={issueId} questionId={questionId} fallbackIdentifier={item.title} onOpenIssue={onOpenIssue} />
        </div>
      </div>
    </div>
  );
}
