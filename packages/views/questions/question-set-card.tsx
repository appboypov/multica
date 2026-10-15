"use client";

import { useEffect, useState } from "react";
import { useIsMutating } from "@tanstack/react-query";
import { Check, CircleCheck, CirclePause, CircleSlash, CornerUpLeft, ListTodo, Send } from "lucide-react";
import { toast } from "sonner";
import { useAuthStore } from "@multica/core/auth";
import { hasUploadingDraft } from "@multica/core/drafts";
import { useWorkspaceId } from "@multica/core/hooks";
import { createLogger } from "@multica/core/logger";
import {
  DECLINED_ANSWER,
  isDeclined,
  isLastOpenFor,
  isOpen,
  isOpenFor,
  questionAsker,
  questionDraftKey,
  questionMutationKeys,
  setProgress,
  useAnswerQuestion,
  useAskBackQuestion,
  useQuestionDraftStore,
} from "@multica/core/questions";
import type { AnswerValue, DeclinedAnswerValue, Question, QuestionAnswer } from "@multica/core/types";
import { useActorName } from "@multica/core/workspace/hooks";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { AnswerControl, handleAnswerKey, useAnswerControlLink } from "./answer-controls/answer-control";
import { draftError, valueToSend, type AnswerDraft } from "./answer-controls/draft";
import { AnswerCountPill, AnswererProgress, showsAnswererProgress } from "./answerer-progress";
import { AnswerLine, AskedBy, QuestionCard, QuestionHistory } from "./question-card";
import { ReassignPicker } from "./reassign-picker";
import { RunInputMarks } from "../issues/components/run-input-marks";
import { TYPE_ICONS } from "./type-icon";
import { WakeButtons } from "./wake-button";

const logger = createLogger("questions.set-card");

export interface QuestionSetCardProps {
  /** The questions of one ask, in ask order. */
  questions: Question[];
  /** The issue the card shows on, whose questions refresh after each action. */
  issueId: string;
  /** The asking run's place among its agent's runs on the issue, for "asked during run #N". */
  runNumber?: number;
  /** The question the card opens at: a deep link's or the Questions row's. */
  activeQuestionId?: string;
  /**
   * `tabs` (the issue page): one question at a time with the set's footer and Wake.
   * `stacked` (Answer all's One page): every question in order with its own
   * control and Send, and no Wake buttons; the page shows its wake bars instead.
   */
  layout?: "tabs" | "stacked";
  /** Stacked only: the question whose card owns the `send` shortcut. */
  focusedQuestionId?: string;
}

interface Composing {
  draft: AnswerDraft;
  note: string;
}

/**
 * The questions of one ask as one card, as screen 6 draws it: a tab per
 * question, the active question's answer control, the others as rows, and a
 * footer with the active question's actions, the set's progress and **Wake**.
 * Each question is answered and sent on its own.
 */
export function QuestionSetCard({ layout = "tabs", focusedQuestionId, ...props }: QuestionSetCardProps) {
  return layout === "stacked"
    ? <StackedQuestionSet questions={props.questions} issueId={props.issueId} runNumber={props.runNumber} focusedQuestionId={focusedQuestionId} />
    : <TabbedQuestionSet {...props} />;
}

function StackedQuestionSet({
  questions,
  issueId,
  runNumber,
  focusedQuestionId,
}: Pick<QuestionSetCardProps, "questions" | "issueId" | "runNumber" | "focusedQuestionId">) {
  const title = questions[0]?.ask_title;
  return (
    <div className="flex flex-col gap-2">
      {title && <p className="text-title-sm font-semibold">{title}</p>}
      {questions.map((question) => (
        <div key={question.id} data-question-id={question.id}>
          <QuestionCard
            question={question}
            issueId={issueId}
            runNumber={runNumber}
            hideWake
            askQuestions={questions}
            focused={question.id === focusedQuestionId}
          />
        </div>
      ))}
    </div>
  );
}

function TabbedQuestionSet({ questions, issueId, runNumber, activeQuestionId }: Omit<QuestionSetCardProps, "layout" | "focusedQuestionId">) {
  const { t } = useT("questions");
  const { getActorName } = useActorName();
  const userId = useAuthStore((s) => s.user?.id);
  const wsId = useWorkspaceId();

  const first = questions[0]!;
  const asker = questionAsker(first);
  const askerName = getActorName(asker.type, asker.id);
  const requested = activeQuestionId ? questions.findIndex((q) => q.id === activeQuestionId) : -1;
  const [active, setActive] = useState(() => (requested >= 0 ? requested : Math.max(questions.findIndex((q) => isOpenFor(q, userId)), 0)));
  useEffect(() => {
    if (requested >= 0) setActive(requested);
  }, [requested]);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [composing, setComposing] = useState<Record<string, Composing>>({});
  const [askBackText, setAskBackText] = useState<string | null>(null);
  const [serverError, setServerError] = useState<string | null>(null);
  const [wakeAfter, setWakeAfter] = useState(true);

  const current = questions[Math.min(active, questions.length - 1)]!;
  const answer = useAnswerQuestion(wsId, { issueId }, current.id);
  const askBack = useAskBackQuestion(wsId, { issueId }, current.id);
  // Read from the query client, not the hook's observer: switching tabs rebinds the hooks to another question.
  const pending = useIsMutating({ mutationKey: questionMutationKeys.answer(wsId, current.id) }) > 0;
  const askingBack = useIsMutating({ mutationKey: questionMutationKeys.askBack(wsId, current.id) }) > 0;
  // File and Text keep the answer's uploads in the question's draft; Send waits while one runs.
  const draftKey = questionDraftKey(wsId, current.id);
  const uploading = useQuestionDraftStore((s) => hasUploadingDraft(s.drafts[draftKey]?.uploads ?? []));
  const clearDraft = useQuestionDraftStore((s) => s.clearDraft);
  const control = useAnswerControlLink();
  const answerable = current.answer_type === "unknown" ? null : current;
  const canAnswer = answerable !== null && (isOpenFor(current, userId) || editingId === current.id);
  const isCurrentAnswerer = !!userId && current.answerers.some((a) => a.user_id === userId);
  const { draft, note } = composing[current.id] ?? { draft: null, note: "" };
  const toSend = (shown: AnswerDraft) => {
    const accepting = shown === null && answerable?.suggested_answer != null;
    const value: AnswerValue | null = !answerable ? null : accepting ? answerable.suggested_answer : valueToSend(answerable, shown);
    const error = !answerable ? null : draftError(answerable, accepting ? (answerable.suggested_answer as AnswerDraft) : shown, note);
    return { accepting, value, error };
  };
  const { accepting, value, error } = toSend(draft);
  const forYou = questions.some((q) => isOpenFor(q, userId));
  const blocksWork = questions.some((q) => isOpen(q) && q.blocks_work);
  const progress = setProgress(questions);
  // Sending the viewer's last open question of an agent's set offers to wake the asker.
  const offersWake = asker.type === "agent" && canAnswer && editingId !== current.id && isLastOpenFor(questions, current.id, userId);

  const activate = (index: number) => {
    setActive(index);
    setAskBackText(null);
    setServerError(null);
  };

  const compose = (next: Partial<Composing>) => {
    setComposing((all) => ({ ...all, [current.id]: { draft, note, ...next } }));
    setServerError(null);
  };

  /** Drops a question's draft once it is sent or its edit is cancelled. */
  const discard = (id: string) => {
    setComposing((all) => {
      const rest = { ...all };
      delete rest[id];
      return rest;
    });
    clearDraft(questionDraftKey(wsId, id));
  };

  const submit = (value: AnswerValue | DeclinedAnswerValue) => {
    const sentId = current.id;
    answer.mutate(
      { value, ...(note.trim() ? { note: note.trim() } : {}), ...(offersWake && wakeAfter ? { wake: true } : {}) },
      {
        onSuccess: () => {
          discard(sentId);
          setEditingId(null);
          // The next question still open for the viewer, after this one first.
          const order = questions.map((_, offset) => (active + 1 + offset) % questions.length);
          const next = order.find((index) => questions[index]!.id !== sentId && isOpenFor(questions[index]!, userId));
          if (next !== undefined) activate(next);
        },
        onError: (err) => {
          logger.error("answer failed", { questionId: sentId, askId: first.ask_id, error: err });
          setServerError(err.message || t(($) => $.card.answer_failed));
        },
      },
    );
  };

  const send = () => {
    if (!canAnswer || pending || uploading || control.isUploading()) return;
    // The Text editor reports typing after its debounce; read what it holds now.
    const { value, error: sendError } = toSend(control.currentDraft(draft));
    if (value === null || sendError !== null) return;
    submit(value);
  };

  const decline = () => {
    if (!canAnswer || pending) return;
    logger.info("declining question", { questionId: current.id, askId: first.ask_id });
    submit(DECLINED_ANSWER);
  };

  const sendAskBack = () => {
    const text = askBackText?.trim();
    if (!text || askingBack) return;
    askBack.mutate(
      { text },
      {
        onSuccess: () => setAskBackText(null),
        onError: (err) => {
          logger.error("ask back failed", { questionId: current.id, askId: first.ask_id, error: err });
          toast.error(t(($) => $.card.ask_back_failed), { description: err.message });
        },
      },
    );
  };

  const startEdit = (index: number) => {
    const question = questions[index]!;
    const own = question.answers.find((a) => a.answerer_user_id === userId);
    if (!own) return;
    activate(index);
    setEditingId(question.id);
    setComposing((all) => ({ ...all, [question.id]: { draft: isDeclined(own.value) ? null : (own.value as AnswerDraft), note: own.note ?? "" } }));
  };

  const cancelEdit = () => {
    discard(current.id);
    setEditingId(null);
  };

  return (
    <div
      className={cn("flex flex-col gap-3 rounded-xl border bg-card p-3.5", forYou ? "border-brand" : "border-border")}
      data-question-state={progress.answered === progress.total ? "answered" : "open"}
    >
      <div className="flex items-center gap-2">
        <ListTodo aria-hidden className={cn("size-3.5 shrink-0", forYou ? "text-brand" : "text-muted-foreground")} />
        <span className="flex min-w-0 items-baseline gap-1">
          <span className={cn("text-micro font-semibold tracking-[0.08em] uppercase", forYou ? "text-brand" : "text-muted-foreground")}>
            {forYou ? t(($) => $.set.for_you) : t(($) => $.set.label)}
          </span>
          <span className="truncate text-caption text-muted-foreground">· {t(($) => $.set.count, { count: questions.length })}</span>
        </span>
        {blocksWork && (
          <span className="ml-auto flex shrink-0 items-center">
            <Badge variant="destructive" className="border-destructive">
              <CirclePause />
              {t(($) => $.card.blocks_work)}
            </Badge>
          </span>
        )}
      </div>

      <AskedBy question={first} runNumber={runNumber} />

      {first.ask_title && <p className="text-title-sm font-semibold">{first.ask_title}</p>}

      <div role="tablist" aria-label={t(($) => $.set.tabs_label)} className="inline-flex max-w-full flex-wrap gap-0.5 self-start rounded-lg bg-muted p-0.5">
        {questions.map((question, index) => {
          const done = !isOpen(question) || question.answers.some((a) => a.answerer_user_id === userId);
          return (
            <button
              key={question.id}
              type="button"
              role="tab"
              aria-selected={index === active}
              title={question.prompt}
              className={cn(
                "flex h-7 max-w-48 min-w-0 items-center gap-1.5 rounded-md px-2 text-body",
                index === active ? "bg-card font-medium text-foreground shadow-sm" : "text-muted-foreground hover:text-foreground",
              )}
              onClick={() => activate(index)}
            >
              {done ? (
                <CircleCheck aria-hidden className="size-3.5 shrink-0 text-success" />
              ) : (
                <span className="flex size-4 shrink-0 items-center justify-center rounded-full border border-border text-micro">{index + 1}</span>
              )}
              <span className="truncate">{question.prompt}</span>
            </button>
          );
        })}
      </div>

      <div className="flex flex-col gap-2">
        {questions.map((question, index) => {
          const own = question.answers.find((a) => a.answerer_user_id === userId);
          const isActive = index === active;
          if (isActive && canAnswer) {
            return (
              <div
                key={question.id}
                role="tabpanel"
                className="flex flex-col gap-3 rounded-lg border border-border p-3"
                onKeyDown={(event) => handleAnswerKey(event, { question: answerable, draft, onDraftChange: (next) => compose({ draft: next }), disabled: pending })}
              >
                <QuestionHeading question={question} />
                <QuestionHistory events={question.events} />
                {showsAnswererProgress(question) && <AnswererProgress question={question} />}
                <AnswerControl
                  question={answerable}
                  draft={draft}
                  onDraftChange={(next) => compose({ draft: next })}
                  note={note}
                  onNoteChange={(next) => compose({ note: next })}
                  onSubmit={send}
                  draftKey={draftKey}
                  handle={control.handle}
                  disabled={pending}
                />
                <AnswerLine question={answerable} error={error} serverError={serverError} uploading={uploading} />
                {askBackText !== null && (
                  <AskBackField
                    askerName={askerName}
                    text={askBackText}
                    pending={askingBack}
                    onChange={setAskBackText}
                    onCancel={() => setAskBackText(null)}
                    onSend={sendAskBack}
                  />
                )}
              </div>
            );
          }
          if (own || !isOpen(question)) {
            const answers: QuestionAnswer<unknown>[] = question.answers;
            const shown = own ?? answers.reduce<QuestionAnswer<unknown> | undefined>(
              (latest, a) => (!latest || a.updated_at > latest.updated_at ? a : latest),
              undefined,
            );
            const Icon = shown ? CircleCheck : CircleSlash;
            const canChange = !!own && (question.status === "answered" || question.status === "partly_answered") && question.answer_type !== "unknown";
            return (
              <div key={question.id} className="min-w-0 rounded-lg bg-muted px-3 py-1.5 text-body" data-question-state={question.status}>
                <div className="flex min-h-7 min-w-0 items-center gap-2">
                <Icon aria-hidden className={cn("size-4 shrink-0", shown ? "text-success" : "text-muted-foreground")} />
                <span className="min-w-0 max-w-max grow basis-0 truncate font-medium" title={question.prompt}>{question.prompt}</span>
                <span className="min-w-0 truncate text-caption text-muted-foreground">
                  {shown ? shown.display_text : question.status === "withdrawn" ? t(($) => $.card.withdrawn) : t(($) => $.card.closed)}
                </span>
                {canChange && (
                  <Button type="button" variant="ghost" size="sm" className="ml-auto shrink-0" onClick={() => startEdit(index)}>
                    {t(($) => $.set.change)}
                  </Button>
                )}
                </div>
                <RunInputMarks marks={shown?.run_marks} className="mt-1.5" />
              </div>
            );
          }
          if (isActive) {
            return (
              <div key={question.id} role="tabpanel" className="flex flex-col gap-3 rounded-lg border border-border p-3">
                <QuestionHeading question={question} />
                <QuestionHistory events={question.events} />
                {showsAnswererProgress(question) ? (
                  <AnswererProgress question={question} />
                ) : (
                  <p className="text-caption text-muted-foreground">
                    {t(($) => $.card.waiting_for, { names: question.answerers.map((a) => getActorName("member", a.user_id)).join(", ") })}
                  </p>
                )}
              </div>
            );
          }
          const TypeIcon = TYPE_ICONS[question.answer_type];
          return (
            <button
              key={question.id}
              type="button"
              className="flex h-10 min-w-0 items-center gap-2 rounded-lg border border-border px-3 text-left text-body hover:bg-accent"
              onClick={() => activate(index)}
            >
              <TypeIcon aria-hidden className="size-4 shrink-0 text-muted-foreground" />
              <span className="min-w-0 flex-1 truncate text-muted-foreground">{question.prompt}</span>
              <span className="shrink-0 text-caption text-muted-foreground">{t(($) => $.types[question.answer_type])}</span>
            </button>
          );
        })}
      </div>

      <div className="flex flex-wrap items-center gap-1">
        {isCurrentAnswerer && isOpen(current) && editingId !== current.id && (
          <>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="text-muted-foreground"
              aria-expanded={askBackText !== null}
              onClick={() => setAskBackText((text) => (text === null ? "" : null))}
            >
              <CornerUpLeft />
              {t(($) => $.card.ask_back)}
            </Button>
            {canAnswer && (
              <Button type="button" variant="ghost" size="sm" className="text-muted-foreground" disabled={pending} onClick={decline}>
                <CircleSlash />
                {t(($) => $.card.decline)}
              </Button>
            )}
            <ReassignPicker question={current} issueId={issueId} disabled={pending} />
          </>
        )}
        <span className="ml-auto flex items-center gap-2">
          <span className="text-caption text-muted-foreground">{t(($) => $.set.progress, progress)}</span>
          {offersWake ? (
            <label className="flex items-center gap-1.5 text-caption text-muted-foreground">
              <Checkbox checked={wakeAfter} disabled={pending} onCheckedChange={(checked) => setWakeAfter(checked === true)} />
              {t(($) => $.card.wake, { name: askerName })}
            </label>
          ) : (
            <WakeButtons questions={questions} issueId={issueId} />
          )}
          {editingId === current.id && (
            <Button type="button" variant="ghost" size="sm" onClick={cancelEdit}>
              {t(($) => $.card.cancel)}
            </Button>
          )}
          {canAnswer && (
            <Button type="button" disabled={value === null || error !== null || pending} onClick={send}>
              {accepting ? <Check /> : <Send />}
              {accepting ? t(($) => $.card.accept_suggestion) : t(($) => $.card.send)}
            </Button>
          )}
        </span>
      </div>
    </div>
  );
}

/** A question's type, prompt and context line inside the set. */
function QuestionHeading({ question }: { question: Question }) {
  const TypeIcon = TYPE_ICONS[question.answer_type];
  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center gap-2">
        <TypeIcon aria-hidden className="size-4 shrink-0 text-muted-foreground" />
        <p className="min-w-0 flex-1 font-medium">{question.prompt}</p>
        {showsAnswererProgress(question) && <AnswerCountPill question={question} />}
      </div>
      {question.context_line && <p className="text-body text-muted-foreground">{question.context_line}</p>}
    </div>
  );
}

function AskBackField({
  askerName,
  text,
  pending,
  onChange,
  onCancel,
  onSend,
}: {
  askerName: string;
  text: string;
  pending: boolean;
  onChange: (text: string) => void;
  onCancel: () => void;
  onSend: () => void;
}) {
  const { t } = useT("questions");
  return (
    <div className="flex flex-col gap-2 rounded-lg border border-border p-2">
      <Textarea
        autoFocus
        value={text}
        disabled={pending}
        aria-label={t(($) => $.card.ask_back_label, { name: askerName })}
        placeholder={t(($) => $.card.ask_back_placeholder, { name: askerName })}
        className="min-h-12 border-0 p-1 shadow-none focus-visible:ring-0"
        onChange={(event) => onChange(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
            event.preventDefault();
            onSend();
          }
        }}
      />
      <div className="flex justify-end gap-1.5">
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
          {t(($) => $.card.cancel)}
        </Button>
        <Button type="button" size="sm" disabled={!text.trim() || pending} onClick={onSend}>
          {t(($) => $.card.ask_back_send)}
        </Button>
      </div>
    </div>
  );
}
