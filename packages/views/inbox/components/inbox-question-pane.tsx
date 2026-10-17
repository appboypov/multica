"use client";

import { useMemo, type ReactNode } from "react";
import { ArrowUpRight } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { issueDetailOptions, issueTasksOptions } from "@multica/core/issues/queries";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  chatQuestionsOptions,
  groupQuestionCards,
  issueQuestionsOptions,
  useQuestion,
  type QuestionScope,
} from "@multica/core/questions";
import type { InboxItem, Question } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { cn } from "@multica/ui/lib/utils";
import { useT, useTimeAgo } from "../../i18n";
import { runNumbersByTask } from "../../issues/components/task-run-labels";
import { useNavigation } from "../../navigation";
import { QuestionCard } from "../../questions";
import { chatQuestionHref, useChatTitle } from "../../questions/row-link";
import { useTypeLabels } from "./inbox-detail-label";

/** The answer types the inbox answers in place; every other type is answered where its card is. */
const ANSWERED_IN_PLACE: ReadonlySet<Question["answer_type"]> = new Set(["yes_no", "single_choice", "approval"]);

/**
 * Where the question's card is: its issue, opened by the inbox, or the chat it
 * was asked in. `issueId` is null for a question without an issue or a chat.
 */
export type InboxQuestionPlace =
  | {
      issueId: string | null;
      /** Opens the issue at the question's card, with `questionId` active when given. */
      onOpenIssue: (questionId?: string) => void;
      chat?: never;
    }
  | {
      /** The chat the question was asked in and its member, from the item's details. */
      chat: { chatSessionId: string; memberId: string | null };
      issueId?: never;
      onOpenIssue?: never;
    };

/**
 * The question's place as the pane and its body use it: the scope its card
 * answers in, the issue it is on, and the way to its card. The chat's member
 * opens the chat and every other member Answer all (Decision 6).
 */
function useQuestionPlace(place: InboxQuestionPlace, questionId: string, fallbackIdentifier: string) {
  const { t } = useT("inbox");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const userId = useAuthStore((s) => s.user?.id);
  const chat = place.chat ?? null;
  const issueId = place.issueId ?? null;
  const scope: QuestionScope = chat ? { chatSessionId: chat.chatSessionId } : { issueId };
  const isChatMember = !!chat && !!userId && chat.memberId === userId;
  const { data: issue } = useQuery({ ...issueDetailOptions(wsId, issueId ?? ""), enabled: !!issueId });
  const openLabel = !chat
    ? t(($) => $.question_pane.open_on, { identifier: issue?.identifier ?? fallbackIdentifier })
    : isChatMember
      ? t(($) => $.question_pane.open_in_chat)
      : t(($) => $.question_pane.open_in_answer_all);
  const open = (activeQuestionId?: string) => {
    if (place.chat) navigation.push(chatQuestionHref(activeQuestionId ?? questionId, place.chat, userId, paths));
    else place.onOpenIssue(activeQuestionId);
  };
  return { chat, issueId, scope, isChatMember, issue, canOpen: !!chat || !!issueId, openLabel, open };
}

/**
 * The question a to-do or an inbox item is about: its card, answerable in
 * place for Yes / No, Single choice and Approval, and for any other type the
 * way to its card on its issue or in its chat. A question without an issue or
 * a chat is answered in place whatever its type. A set's item shows the set's
 * title and each of its questions, wherever the viewer may read the set.
 */
export function InboxQuestionBody({
  questionId,
  fallbackIdentifier,
  ...place
}: InboxQuestionPlace & {
  questionId: string;
  /** Names the issue in "Open on …" until the issue has loaded. */
  fallbackIdentifier: string;
}) {
  const { t } = useT("inbox");
  const { t: tQuestions } = useT("questions");
  const wsId = useWorkspaceId();
  const { chat, issueId, scope, isChatMember, canOpen, openLabel, open } = useQuestionPlace(place, questionId, fallbackIdentifier);

  const { data: question, isPending } = useQuestion(wsId, questionId);
  const { data: tasks } = useQuery(issueTasksOptions(issueId ?? ""));
  const runNumber = useMemo(
    () => (question && tasks ? runNumbersByTask(tasks).get(question.asked_in_task_id) : undefined),
    [question, tasks],
  );

  const shown = question && question.id !== "" ? question : null;
  // A set is listed per issue or per chat, and a chat's list is its member's
  // alone; anyone else, and a question without either, sees the one card.
  const isSet = !!shown && shown.ask_title !== null && (chat ? isChatMember : !!issueId);
  const { data: issueQuestions } = useQuery({ ...issueQuestionsOptions(wsId, issueId ?? ""), enabled: isSet && !chat && !!wsId });
  const { data: chatQuestions } = useQuery({
    ...chatQuestionsOptions(wsId, chat?.chatSessionId ?? ""),
    enabled: isSet && !!chat && !!wsId,
  });
  const placeQuestions = chat ? chatQuestions : issueQuestions;
  const setQuestions = useMemo(
    () => (isSet && placeQuestions ? groupQuestionCards(placeQuestions).find((card) => card.askId === shown?.ask_id)?.questions ?? null : null),
    [isSet, placeQuestions, shown],
  );
  const openButton = (activeQuestionId?: string) => (
    <Button type="button" size="sm" className="ml-auto shrink-0" onClick={() => open(activeQuestionId)}>
      <ArrowUpRight />
      {openLabel}
    </Button>
  );
  const answeredElsewhere = (q: Question) => {
    const type = tQuestions(($) => $.types[q.answer_type]);
    return (
      <div className="flex items-center gap-3">
        <p className="text-caption text-muted-foreground">
          {!chat
            ? t(($) => $.question_pane.answered_on_issue, { type })
            : isChatMember
              ? t(($) => $.question_pane.answered_in_chat, { type })
              : t(($) => $.question_pane.answered_in_answer_all, { type })}
        </p>
        {openButton(q.id)}
      </div>
    );
  };

  if (isPending || (isSet && !setQuestions && !placeQuestions)) {
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
            scope={scope}
            runNumber={runNumber}
            hideWake
            askQuestions={setQuestions}
            answerElsewhere={ANSWERED_IN_PLACE.has(q.answer_type) ? undefined : answeredElsewhere(q)}
          />
        ))}
      </div>
    );
  }
  if (shown) {
    return (
      <QuestionCard
        question={shown}
        scope={scope}
        runNumber={runNumber}
        askQuestions={[shown]}
        answerElsewhere={!canOpen || ANSWERED_IN_PLACE.has(shown.answer_type) ? undefined : answeredElsewhere(shown)}
      />
    );
  }
  return (
    <div className="flex items-center gap-3 rounded-xl border bg-card p-3.5">
      <p className="text-caption text-muted-foreground">{t(($) => $.question_pane.unavailable)}</p>
      {canOpen && openButton()}
    </div>
  );
}

/** The detail pane of a question's inbox item: its head and the question. */
export function InboxQuestionPane({
  item,
  questionId,
  leadingAction,
  actions,
  className,
  ...place
}: InboxQuestionPlace & {
  item: InboxItem;
  questionId: string;
  /** The compact layout's way back to the list. */
  leadingAction?: ReactNode;
  /** The item's own actions, such as Archive. */
  actions?: ReactNode;
  className?: string;
}) {
  const timeAgo = useTimeAgo();
  const typeLabels = useTypeLabels();
  const wsId = useWorkspaceId();
  const chatTitle = useChatTitle();
  const { chat, isChatMember, issue, openLabel, open } = useQuestionPlace(place, questionId, item.title);
  const { data: question } = useQuestion(wsId, questionId);
  const shown = question && question.id !== "" ? question : null;
  const isSet = !!shown && shown.ask_title !== null && (!chat || isChatMember);
  const showOpen = !!shown && (ANSWERED_IN_PLACE.has(shown.answer_type) || isSet);

  return (
    <div className={cn("flex min-h-0 flex-1 flex-col", className)}>
      <div className="flex h-12 shrink-0 items-center gap-2 border-b px-4">
        {leadingAction}
        <span className="truncate text-body font-medium">{typeLabels[item.type]}</span>
        <span className="shrink-0 text-caption text-muted-foreground">· {timeAgo(item.created_at)}</span>
        <span className="ml-auto flex shrink-0 items-center gap-1">
          {showOpen && (
            <Button type="button" variant="ghost" size="sm" className="text-muted-foreground" onClick={() => open()}>
              <ArrowUpRight />
              {openLabel}
            </Button>
          )}
          {actions}
        </span>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto p-6">
        <div className="mx-auto flex max-w-3xl flex-col gap-4">
          <div className="flex flex-col gap-0.5">
            {issue?.identifier && <span className="text-caption text-muted-foreground">{issue.identifier}</span>}
            <h2 className="text-title font-semibold">{chat ? chatTitle(chat.memberId) : (issue?.title ?? item.title)}</h2>
          </div>
          <InboxQuestionBody {...place} questionId={questionId} fallbackIdentifier={item.title} />
        </div>
      </div>
    </div>
  );
}
