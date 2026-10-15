"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { useIsMutating, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, CircleCheck, CircleSlash, CircleX, CornerUpLeft, NotebookPen, Pencil, Send, Check } from "lucide-react";
import { toast } from "sonner";
import { useAuthStore } from "@multica/core/auth";
import { hasUploadingDraft } from "@multica/core/drafts";
import { useWorkspaceId } from "@multica/core/hooks";
import { isPortalLayerShortcutTarget, shortcutMatchesEvent, useShortcut } from "@multica/core/shortcuts";
import { createLogger } from "@multica/core/logger";
import {
  DECLINED_ANSWER,
  EMPTY_QUESTION_DRAFT,
  isDeclined,
  isLastOpenFor,
  isOpen,
  isOpenFor,
  questionAsker,
  questionDraftKey,
  questionMutationKeys,
  useAnswerQuestion,
  useAskBackQuestion,
  useCancelQuestion,
  useQuestionDraftStore,
  type AnswerValidationError,
  type QuestionDraftPatch,
  type QuestionScope,
} from "@multica/core/questions";
import type {
  AnswerValue,
  CancelQuestionRequest,
  DeclinedAnswerValue,
  Question,
  QuestionAnswer,
  QuestionEvent,
} from "@multica/core/types";
import { useActorName } from "@multica/core/workspace/hooks";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { ActorAvatar } from "../common/actor-avatar";
import { useT, useTimeAgo } from "../i18n";
import { QuestionBadges } from "./question-badges";
import { AnswerCountPill, AnswererProgress, showsAnswererProgress } from "./answerer-progress";
import { AnswerControl, handleAnswerKey, useAnswerControlLink } from "./answer-controls/answer-control";
import { controlShowsNote, draftError, valueToSend, type AnswerDraft, type AnswerableQuestion } from "./answer-controls/draft";
import { useSuggestionShown } from "./answer-controls/person-control";
import { ReassignPicker } from "./reassign-picker";
import { RunInputMarks } from "../issues/components/run-input-marks";
import { TYPE_ICONS } from "./type-icon";
import { WakeButtons } from "./wake-button";
import type questionsCopy from "../locales/en/questions.json";

const logger = createLogger("questions.card");

export interface QuestionCardProps {
  question: Question;
  /** The issue or chat the card shows in, whose questions refresh after each action. */
  scope: QuestionScope;
  /** The asking run's place among its agent's runs on the issue, for "asked during run #N". */
  runNumber?: number;
  /**
   * Shown on an open card in place of its controls and summary, where the
   * question is answered somewhere else: the inbox's way to the issue for a
   * type it does not answer in place.
   */
  answerElsewhere?: ReactNode;
  /**
   * The card has no Wake buttons of its own and folds once the viewer has
   * answered: a question of a set outside the set's card, and Answer all,
   * which shows its own wake bars.
   */
  hideWake?: boolean;
  /**
   * The questions of the card's ask this screen holds, the card's own among
   * them: Send offers **Wake {agent}** on the viewer's last open question of them.
   */
  askQuestions: readonly Question[];
  /** The focused card owns the `send` shortcut and sends its draft when its Send is enabled. */
  focused?: boolean;
}

/**
 * A question in an issue's timeline or a chat, in the comment card's frame.
 * Open, it holds its answer type's control for an answerer and a summary for
 * anyone else; once answered, closed, withdrawn or cancelled it folds to one
 * line, a cancelled one with the cancel's note and every answer sent before it.
 */
export function QuestionCard({
  question,
  scope,
  runNumber,
  answerElsewhere,
  hideWake = false,
  askQuestions,
  focused = false,
}: QuestionCardProps) {
  const userId = useAuthStore((s) => s.user?.id);
  const wsId = useWorkspaceId();
  const draftKey = questionDraftKey(wsId, question.id);
  const hasDraft = useQuestionDraftStore((s) => draftKey in s.drafts);
  const draftEditing = useQuestionDraftStore((s) => s.drafts[draftKey]?.editing ?? false);
  const setDraft = useQuestionDraftStore((s) => s.setDraft);
  const clearDraft = useQuestionDraftStore((s) => s.clearDraft);
  const isAnswerer = !!userId && question.answerers.some((a) => a.user_id === userId);
  const ownAnswer = question.answers.find((a) => a.answerer_user_id === userId) ?? null;

  // The server takes an edit only on an answered question, from a current
  // answerer who has an answer of their own. With several answerers the first
  // answer settles the question for all of them, so the others have none to edit.
  const canEdit =
    isAnswerer &&
    ownAnswer !== null &&
    (question.status === "answered" || question.status === "partly_answered") &&
    question.answer_type !== "unknown";
  // A live update that takes the edit away (closed, withdrawn, reassigned) folds the card.
  const editing = draftEditing && canEdit;
  // A draft outlives the card, not its chance to be sent: one for a question no
  // longer open to this answerer, or an edit no longer allowed, is dropped.
  const stale = hasDraft && !!userId && (draftEditing ? !canEdit : !isOpenFor(question, userId));
  useEffect(() => {
    if (stale) clearDraft(draftKey);
  }, [stale, draftKey, clearDraft]);

  if ((!isOpen(question) || (hideWake && ownAnswer !== null)) && !editing) {
    const startEdit = () =>
      setDraft(draftKey, {
        editing: true,
        answer: ownAnswer && !isDeclined(ownAnswer.value) ? ownAnswer.value : null,
        note: ownAnswer?.note ?? "",
        noteOpen: !!ownAnswer?.note,
        askBackOpen: false,
        askBackText: "",
      });
    return <FoldedQuestion question={question} scope={scope} hideWake={hideWake} onEdit={canEdit ? startEdit : undefined} />;
  }
  return (
    <OpenQuestion
      question={question}
      scope={scope}
      runNumber={runNumber}
      answerElsewhere={answerElsewhere}
      isAnswerer={isAnswerer && (editing || isOpenFor(question, userId))}
      draftKey={draftKey}
      editing={editing}
      onDone={() => clearDraft(draftKey)}
      focused={focused}
      offersWake={questionAsker(question).type === "agent" && isLastOpenFor(askQuestions, question.id, userId)}
    />
  );
}

// ---------------------------------------------------------------------------
// Folded: prompt → answer · who · when, Edit answer, Wake <agent>
// ---------------------------------------------------------------------------

function FoldedQuestion({ question, scope, hideWake, onEdit }: { question: Question; scope: QuestionScope; hideWake: boolean; onEdit?: () => void }) {
  const { t } = useT("questions");
  const timeAgo = useTimeAgo();
  const { getActorName } = useActorName();
  const cancelled = question.status === "cancelled";
  // A cancelled card names the cancel in place of an answer and lists every answer below the line.
  const answer = cancelled ? null : latestAnswer(question.answers);
  const Icon = answer ? CircleCheck : CircleSlash;

  return (
    <div className="flex min-w-0 flex-col gap-1 rounded-xl border bg-card px-3.5 py-1.5 text-body" data-question-state={question.status}>
      <div className="flex min-h-9 min-w-0 items-center gap-2">
        <Icon aria-hidden className={cn("size-4 shrink-0", answer ? "text-success" : "text-muted-foreground")} />
        {/* The prompt grows from nothing into the room the answer leaves, up to its own width,
            so it truncates first; the answer keeps its width until it alone no longer fits. */}
        <span className="min-w-0 max-w-max grow basis-0 truncate text-muted-foreground" title={question.prompt}>{question.prompt}</span>
        <ArrowRight aria-hidden className="size-4 shrink-0 text-muted-foreground" />
        {answer ? (
          <>
            <span className="min-w-0 truncate font-medium">{answer.display_text}</span>
            <span className="shrink-0 text-caption text-muted-foreground">
              · {t(($) => $.card.answered_by, { name: getActorName("member", answer.answerer_user_id), time: timeAgo(answer.updated_at) })}
            </span>
          </>
        ) : cancelled ? (
          <CancelledBy question={question} />
        ) : (
          <span className="min-w-0 truncate font-medium">
            {question.status === "withdrawn" ? t(($) => $.card.withdrawn) : t(($) => $.card.closed)}
          </span>
        )}
        <span className="ml-auto flex shrink-0 items-center gap-1.5">
          {onEdit && (
            <Button type="button" variant="ghost" size="sm" className="text-muted-foreground" onClick={onEdit}>
              <Pencil />
              {t(($) => $.card.edit_answer)}
            </Button>
          )}
          {!hideWake && <WakeButtons questions={[question]} scope={scope} />}
        </span>
      </div>
      <RunInputMarks marks={answer?.run_marks} className="mt-0.5" />
      {cancelled && <CancelDetails question={question} />}
    </div>
  );
}

function latestAnswer(answers: QuestionAnswer<unknown>[]): QuestionAnswer<unknown> | null {
  return answers.reduce<QuestionAnswer<unknown> | null>(
    (latest, answer) => (!latest || answer.updated_at > latest.updated_at ? answer : latest),
    null,
  );
}

type CancelEvent = Extract<QuestionEvent, { kind: "cancelled" }>;

/** A cancelled question's cancel: the latest of its `cancelled` events, which run oldest first. */
function latestCancel(events: QuestionEvent[]): CancelEvent | null {
  return events.filter((event): event is CancelEvent => event.kind === "cancelled").at(-1) ?? null;
}

/** "Cancelled by {member} · {time}" from a cancelled question's cancel; "Cancelled" when that event is missing. */
export function CancelledBy({ question }: { question: Question }) {
  const { t } = useT("questions");
  const timeAgo = useTimeAgo();
  const { getActorName } = useActorName();
  const cancel = latestCancel(question.events);
  if (!cancel?.actor_id) return <span className="min-w-0 truncate font-medium">{t(($) => $.card.cancelled)}</span>;
  return (
    <>
      <span className="min-w-0 truncate font-medium">
        {t(($) => $.card.cancelled_by, { name: getActorName(cancel.actor_type, cancel.actor_id) })}
      </span>
      <span className="shrink-0 text-caption text-muted-foreground">· {timeAgo(cancel.created_at)}</span>
    </>
  );
}

/** Below a cancelled question's line: the cancel's note, then every answer sent before it. */
export function CancelDetails({ question }: { question: Question }) {
  const { t } = useT("questions");
  const timeAgo = useTimeAgo();
  const { getActorName } = useActorName();
  const note = latestCancel(question.events)?.details.note;
  if (!note && question.answers.length === 0) return null;
  return (
    <div className="flex min-w-0 flex-col gap-1 pb-1 pl-6">
      {note && <p className="whitespace-pre-wrap text-body text-muted-foreground">{note}</p>}
      {question.answers.map((answer) => (
        <p key={answer.id} className="flex min-w-0 items-center gap-1.5 text-body">
          <CircleCheck aria-hidden className="size-3.5 shrink-0 text-success" />
          <span className="min-w-0 truncate font-medium">{answer.display_text}</span>
          <span className="shrink-0 text-caption text-muted-foreground">
            · {t(($) => $.card.answered_by, { name: getActorName("member", answer.answerer_user_id), time: timeAgo(answer.updated_at) })}
          </span>
        </p>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Open: overline, asker, prompt, history, control, footer
// ---------------------------------------------------------------------------

function OpenQuestion({
  question,
  scope,
  runNumber,
  answerElsewhere,
  isAnswerer,
  draftKey,
  editing,
  onDone,
  focused,
  offersWake,
}: {
  question: Question;
  scope: QuestionScope;
  runNumber?: number;
  answerElsewhere?: ReactNode;
  isAnswerer: boolean;
  draftKey: string;
  editing: boolean;
  onDone: () => void;
  focused: boolean;
  /** The viewer's last open question of an agent's ask: Send offers to wake its asker. */
  offersWake: boolean;
}) {
  const { t } = useT("questions");
  const { getActorName } = useActorName();
  const asker = questionAsker(question);
  const askerName = getActorName(asker.type, asker.id);
  const answerable = question.answer_type === "unknown" ? null : question;
  const controls = isAnswerer && answerable !== null;
  const TypeIcon = TYPE_ICONS[question.answer_type];

  return (
    <div
      className={cn("flex flex-col gap-3 rounded-xl border bg-card p-3.5", isAnswerer ? "border-brand" : "border-border")}
      data-question-state="open"
    >
      <div className="flex items-center gap-2">
        <TypeIcon aria-hidden className={cn("size-3.5 shrink-0", isAnswerer ? "text-brand" : "text-muted-foreground")} />
        {isAnswerer ? (
          <span className="flex min-w-0 items-baseline gap-1">
            <span className="text-micro font-semibold tracking-[0.08em] text-brand uppercase">{t(($) => $.card.for_you)}</span>
            <span className="truncate text-caption text-muted-foreground">· {t(($) => $.types[question.answer_type])}</span>
          </span>
        ) : (
          <span className="truncate text-micro font-semibold tracking-[0.08em] text-muted-foreground uppercase">
            {t(($) => $.types[question.answer_type])}
          </span>
        )}
        <CardBadges question={question} />
      </div>

      <AskedBy question={question} runNumber={runNumber} />

      <div className="flex flex-col gap-1">
        <p className="text-title-sm font-semibold">{question.prompt}</p>
        {question.context_line && <p className="text-body text-muted-foreground">{question.context_line}</p>}
      </div>

      <QuestionHistory events={question.events} />

      {showsAnswererProgress(question) && <AnswererProgress question={question} />}

      {answerElsewhere ?? (controls ? (
        <AnswerForm
          key={editing ? "editing" : "open"}
          question={answerable}
          scope={scope}
          askerName={askerName}
          draftKey={draftKey}
          editing={editing}
          onDone={onDone}
          offersWake={offersWake && !editing}
          focused={focused}
        />
      ) : (
        <>
          {answerable ? <QuestionSummary question={answerable} /> : isAnswerer && (
            <p className="text-caption text-muted-foreground">{t(($) => $.card.unreadable)}</p>
          )}
          <WaitingFooter question={question} />
        </>
      ))}
    </div>
  );
}

function CardBadges({ question }: { question: Question }) {
  return (
    <span className="ml-auto flex shrink-0 items-center gap-1.5">
      {showsAnswererProgress(question) && <AnswerCountPill question={question} />}
      <QuestionBadges blocksWork={question.blocks_work} dueDate={question.due_date} open={isOpen(question)} />
    </span>
  );
}

/**
 * The asker's avatar and name, and when it asked: an agent's question reads its
 * run, "asked during run #3 · 1m ago"; a member's question, asked outside a run,
 * reads "asked · 1m ago".
 */
export function AskedBy({ question, runNumber }: { question: Question; runNumber?: number }) {
  const { t } = useT("questions");
  const timeAgo = useTimeAgo();
  const { getActorName } = useActorName();
  const asker = questionAsker(question);
  const time = timeAgo(question.created_at);
  return (
    <div className="flex items-center gap-2 text-body">
      <ActorAvatar actorType={asker.type} actorId={asker.id} size="sm" />
      <span className="font-medium">{getActorName(asker.type, asker.id)}</span>
      <span className="truncate text-caption text-muted-foreground">
        {asker.type === "member"
          ? t(($) => $.card.asked, { time })
          : runNumber !== undefined
            ? t(($) => $.card.asked_during_run, { number: runNumber, time })
            : t(($) => $.card.asked_during_a_run, { time })}
      </span>
    </div>
  );
}

/** The question's thread: ask-backs and reassignments, oldest first. */
export function QuestionHistory({ events }: { events: QuestionEvent[] }) {
  const { t } = useT("questions");
  const timeAgo = useTimeAgo();
  const { getActorName } = useActorName();
  const entries = events.filter((e) => e.kind === "ask_back" || e.kind === "reassigned");
  if (entries.length === 0) return null;

  return (
    <ol className="flex flex-col gap-2 border-l-2 border-border pl-3">
      {entries.map((event) => {
        const actor = event.actor_id ? getActorName(event.actor_type, event.actor_id) : "";
        const time = timeAgo(event.created_at);
        return (
          <li key={event.id} className="flex flex-col gap-0.5">
            <span className="text-caption text-muted-foreground">
              {event.kind === "ask_back"
                ? t(($) => $.card.history_ask_back, { name: actor, time })
                : t(($) => $.card.history_reassigned, { name: actor, target: getActorName("member", event.details.user_id), time })}
            </span>
            <span className="text-body whitespace-pre-wrap">{event.kind === "ask_back" ? event.details.text : event.details.note}</span>
          </li>
        );
      })}
    </ol>
  );
}

/** What a non-answerer sees in place of the control: the choices as one line. */
function QuestionSummary({ question }: { question: AnswerableQuestion }) {
  const { t } = useT("questions");
  const items: { label: string; suggested?: boolean }[] = (() => {
    switch (question.answer_type) {
      case "yes_no":
        return [
          { label: t(($) => $.controls.yes), suggested: question.suggested_answer?.choice === "yes" },
          { label: t(($) => $.controls.no), suggested: question.suggested_answer?.choice === "no" },
        ];
      case "single_choice": {
        const suggested = question.suggested_answer && "option" in question.suggested_answer ? question.suggested_answer.option : null;
        return [
          ...question.settings.options.map((label) => ({ label, suggested: label === suggested })),
          ...(question.settings.allow_other ? [{ label: t(($) => $.controls.other).replace(/\.+$/, "") }] : []),
        ];
      }
      case "multiple_choice":
        return question.settings.options.map((label) => ({ label, suggested: question.suggested_answer?.options.includes(label) }));
      case "ranking":
        return (question.suggested_answer?.order ?? question.settings.items).map((label) => ({ label }));
      case "confirm_facts":
        return question.settings.facts.map((label) => ({ label }));
      case "approval":
        return [{ label: question.settings.subject }];
      default:
        return [];
    }
  })();
  if (items.length === 0) return null;

  return (
    <p className="flex flex-wrap items-center gap-x-1.5 gap-y-1 text-body text-muted-foreground">
      {items.map((item, index) => (
        <span key={`${index}:${item.label}`} className="flex items-center gap-1.5">
          {index > 0 && <span aria-hidden>·</span>}
          <span className={cn(item.suggested && "text-foreground")}>{item.label}</span>
          {item.suggested && (
            <Badge variant="outline" className="rounded-sm border-brand text-brand">{t(($) => $.controls.suggested)}</Badge>
          )}
        </span>
      ))}
    </p>
  );
}

function AnswererAvatars({ question }: { question: Question }) {
  return (
    <span className="flex items-center -space-x-1">
      {question.answerers.map((a) => (
        <ActorAvatar key={a.user_id} actorType="member" actorId={a.user_id} size="sm" />
      ))}
    </span>
  );
}

function WaitingFooter({ question }: { question: Question }) {
  const { t } = useT("questions");
  const { getActorName } = useActorName();
  const names = question.answerers.map((a) => getActorName("member", a.user_id)).join(", ");
  return (
    <div className="flex items-center gap-2 text-caption text-muted-foreground">
      <span>{t(($) => $.card.asked_to)}</span>
      <AnswererAvatars question={question} />
      <span className="text-body text-foreground">{names}</span>
      <span className="ml-auto">{t(($) => $.card.waiting_for, { names })}</span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// The answerer's form: control, note, ask back, reassign, primary action
// ---------------------------------------------------------------------------

function AnswerForm({
  question,
  scope,
  askerName,
  draftKey,
  editing,
  onDone,
  focused,
  offersWake,
}: {
  question: AnswerableQuestion;
  scope: QuestionScope;
  askerName: string;
  /** Where the unsent answer, note and ask back live, so they outlast the card. */
  draftKey: string;
  editing: boolean;
  onDone: () => void;
  focused: boolean;
  /** Offers **Wake {agent}**, checked by default, to wake the asker once the answer or cancel is sent. */
  offersWake: boolean;
}) {
  const { t } = useT("questions");
  const wsId = useWorkspaceId();
  const answer = useAnswerQuestion(wsId, scope, question.id);
  const askBack = useAskBackQuestion(wsId, scope, question.id);
  const cancel = useCancelQuestion(wsId, scope, question.id);
  // Read from the query client, not this card's own mutation: a card mounted
  // again while its send, cancel or an ask back is on its way still shows it pending.
  const queryClient = useQueryClient();
  const answerKey = questionMutationKeys.answer(wsId, question.id);
  const askBackKey = questionMutationKeys.askBack(wsId, question.id);
  const cancelKey = questionMutationKeys.cancel(wsId, question.id);
  // An answer and a cancel each end the answerer's turn, so the form waits while either is on its way.
  const answering = useIsMutating({ mutationKey: answerKey }) > 0;
  const cancelling = useIsMutating({ mutationKey: cancelKey }) > 0;
  const pending = answering || cancelling;
  const askingBack = useIsMutating({ mutationKey: askBackKey }) > 0;
  const busy = () => queryClient.isMutating({ mutationKey: answerKey }) > 0 || queryClient.isMutating({ mutationKey: cancelKey }) > 0;

  // Editing starts from the sent answer (seeded on Edit answer); a first answer from nothing picked.
  const stored = useQuestionDraftStore((s) => s.drafts[draftKey]) ?? EMPTY_QUESTION_DRAFT;
  const setStored = useQuestionDraftStore((s) => s.setDraft);
  const update = (patch: QuestionDraftPatch) => setStored(draftKey, patch);
  const draft = stored.answer as AnswerDraft;
  // Until the answerer picks or types, the control shows the suggestion, so
  // Accept suggestion sends what is on screen; an edit starts from it.
  const shown = draft ?? (question.suggested_answer as AnswerDraft);
  const { note, noteOpen, askBackOpen, askBackText } = stored;
  const [serverError, setServerError] = useState<string | null>(null);
  const [wakeAfter, setWakeAfter] = useState(true);

  const changeDraft = (next: AnswerDraft) => {
    update({ answer: next });
    setServerError(null);
  };

  const { accepting, value, error } = toSend(question, draft, note);
  // Accept suggestion sends only what the control shows: a suggested Person once its row is on screen.
  const suggestionShown = useSuggestionShown(question);
  const noteInControl = controlShowsNote(question, shown);
  const control = useAnswerControlLink();
  // The question's uploads outlive the control that started them, so a card mounted again still waits for them.
  const uploading = hasUploadingDraft(stored.uploads);

  // The note goes with an answer, a decline or a cancel as its reason; Wake, when offered and checked, with each.
  const noteAndWake: CancelQuestionRequest = {
    ...(note.trim() ? { note: note.trim() } : {}),
    ...(offersWake && wakeAfter ? { wake: true } : {}),
  };

  const submit = (value: AnswerValue | DeclinedAnswerValue) =>
    answer.mutate(
      { value, ...noteAndWake },
      // A success clears what it sent from the question's draft in `useAnswerQuestion`, ending an edit too.
      {
        onError: (err) => {
          logger.error("answer failed", { questionId: question.id, error: err });
          // The server is the authority; its refusal shows where the client's would.
          setServerError(err.message || t(($) => $.card.answer_failed));
        },
      },
    );

  const send = () => {
    // What the card shows can trail: `⌘↵` bypasses the button, an upload can
    // still be in flight, and the editor reports typing after its debounce.
    const uploads = useQuestionDraftStore.getState().drafts[draftKey]?.uploads ?? EMPTY_QUESTION_DRAFT.uploads;
    if (busy() || hasUploadingDraft(uploads) || control.isUploading()) return;
    const now = toSend(question, control.currentDraft(draft), note);
    if (now.value === null || now.error !== null || (now.accepting && !suggestionShown)) return;
    submit(now.value);
  };

  const decline = () => {
    if (busy()) return;
    logger.info("declining question", { questionId: question.id });
    submit(DECLINED_ANSWER);
  };

  // A success folds the card once the question refetches as cancelled, and the
  // draft, no longer open to this answerer, is dropped as stale.
  const cancelQuestion = () => {
    if (busy()) return;
    logger.info("cancelling question", { questionId: question.id });
    cancel.mutate(noteAndWake, {
      onError: (err) => {
        logger.error("cancel failed", { questionId: question.id, error: err });
        setServerError(err.message || t(($) => $.card.cancel_failed));
      },
    });
  };

  // The focused card owns the send shortcut. A control that already sent on the
  // same keystroke, such as the Text editor, marks the event handled.
  const sendShortcut = useShortcut("send");
  const sendRef = useRef(send);
  sendRef.current = send;
  useEffect(() => {
    if (!focused) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented || !shortcutMatchesEvent(sendShortcut, event)) return;
      if (isPortalLayerShortcutTarget(event.target)) return;
      event.preventDefault();
      sendRef.current();
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [focused, sendShortcut]);

  const sendAskBack = () => {
    const text = askBackText.trim();
    if (!text || queryClient.isMutating({ mutationKey: askBackKey }) > 0) return;
    askBack.mutate(
      { text },
      // A success empties the ask-back box in `useAskBackQuestion`, unless it was written again since.
      {
        onError: (err) => {
          logger.error("ask back failed", { questionId: question.id, error: err });
          toast.error(t(($) => $.card.ask_back_failed), { description: err.message });
        },
      },
    );
  };

  return (
    <div
      className="flex flex-col gap-3"
      onKeyDown={(event) => handleAnswerKey(event, { question, draft: shown, onDraftChange: changeDraft, disabled: pending })}
    >
      <AnswerControl
        question={question}
        draft={shown}
        onDraftChange={changeDraft}
        note={note}
        onNoteChange={(next) => update({ note: next })}
        onSubmit={send}
        draftKey={draftKey}
        handle={control.handle}
        disabled={pending}
      />

      <AnswerLine question={question} error={error} serverError={serverError} uploading={uploading} />

      {noteOpen && !noteInControl && (
        <Textarea
          value={note}
          disabled={pending}
          aria-label={t(($) => $.card.note_label)}
          placeholder={t(($) => $.card.note_placeholder)}
          className="min-h-12"
          onChange={(event) => update({ note: event.target.value })}
        />
      )}

      {askBackOpen && (
        <div className="flex flex-col gap-2 rounded-lg border border-border p-2">
          <Textarea
            autoFocus
            value={askBackText}
            disabled={askingBack}
            aria-label={t(($) => $.card.ask_back_label, { name: askerName })}
            placeholder={t(($) => $.card.ask_back_placeholder, { name: askerName })}
            className="min-h-12 border-0 p-1 shadow-none focus-visible:ring-0"
            onChange={(event) => update({ askBackText: event.target.value })}
            onKeyDown={(event) => {
              if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
                event.preventDefault();
                sendAskBack();
              }
            }}
          />
          <div className="flex justify-end gap-1.5">
            <Button type="button" variant="ghost" size="sm" onClick={() => update({ askBackOpen: false })}>
              {t(($) => $.card.cancel)}
            </Button>
            <Button type="button" size="sm" disabled={!askBackText.trim() || askingBack} onClick={sendAskBack}>
              {t(($) => $.card.ask_back_send)}
            </Button>
          </div>
        </div>
      )}

      <div className="flex flex-wrap items-center gap-1">
        {!noteInControl && (
          <Button type="button" variant="ghost" size="sm" className="text-muted-foreground" aria-expanded={noteOpen} onClick={() => update({ noteOpen: !noteOpen })}>
            <NotebookPen />
            {t(($) => $.card.add_note)}
          </Button>
        )}
        {!editing && (
          <>
            <Button type="button" variant="ghost" size="sm" className="text-muted-foreground" aria-expanded={askBackOpen} onClick={() => update({ askBackOpen: !askBackOpen })}>
              <CornerUpLeft />
              {t(($) => $.card.ask_back)}
            </Button>
            <Button type="button" variant="ghost" size="sm" className="text-muted-foreground" disabled={pending} onClick={decline}>
              <CircleSlash />
              {t(($) => $.card.decline)}
            </Button>
            <Button type="button" variant="ghost" size="sm" className="text-muted-foreground" disabled={pending} onClick={cancelQuestion}>
              <CircleX />
              {t(($) => $.card.cancel_question)}
            </Button>
            <ReassignPicker question={question} scope={scope} disabled={pending} />
          </>
        )}
        <span className="ml-auto flex items-center gap-2">
          <span className="text-caption text-muted-foreground">{t(($) => $.card.asked_to)}</span>
          <AnswererAvatars question={question} />
          {editing && (
            <Button type="button" variant="ghost" size="sm" onClick={onDone}>
              {t(($) => $.card.cancel)}
            </Button>
          )}
          {offersWake && (
            <label className="flex items-center gap-1.5 text-caption text-muted-foreground">
              <Checkbox checked={wakeAfter} disabled={pending} onCheckedChange={(checked) => setWakeAfter(checked === true)} />
              {t(($) => $.card.wake, { name: askerName })}
            </label>
          )}
          <Button type="button" disabled={value === null || error !== null || (accepting && !suggestionShown) || pending || uploading} onClick={send}>
            {accepting ? <Check /> : <Send />}
            {accepting ? t(($) => $.card.accept_suggestion) : t(($) => $.card.send)}
          </Button>
        </span>
      </div>
    </div>
  );
}

/** What Send sends for `draft`: the suggestion while nothing is picked, else the draft; and what holds it back. */
function toSend(
  question: AnswerableQuestion,
  draft: AnswerDraft,
  note: string,
): { accepting: boolean; value: AnswerValue | null; error: AnswerValidationError | null } {
  const accepting = draft === null && question.suggested_answer !== null;
  return {
    accepting,
    value: accepting ? question.suggested_answer : valueToSend(question, draft),
    error: accepting ? validateSuggestion(question, note) : draftError(question, draft, note),
  };
}

/** Accept suggestion sends the suggestion as it is, under the same rules as Send. */
function validateSuggestion(question: AnswerableQuestion, note: string): AnswerValidationError | null {
  return question.suggested_answer === null ? null : draftError(question, question.suggested_answer as AnswerDraft, note);
}

/** The line above the footer: why Send is held back, else the type's hint. */
export function AnswerLine({
  question,
  error,
  serverError,
  uploading,
}: {
  question: AnswerableQuestion;
  error: AnswerValidationError | null;
  serverError: string | null;
  uploading: boolean;
}) {
  const { t } = useT("questions");
  if (serverError) return <p role="alert" className="text-caption text-destructive">{serverError}</p>;
  if (uploading) return <p role="status" className="text-caption text-muted-foreground">{t(($) => $.card.uploading)}</p>;
  // Nothing picked breaks no rule, so an untouched card shows its hint.
  if (error) {
    const name = error.key.slice("validation.".length) as ValidationKey;
    return <p role="alert" className="text-caption text-destructive">{t(($) => $.validation[name], error.values)}</p>;
  }
  const hint = hintFor(question);
  return hint ? <p className="text-caption text-muted-foreground">{t(($) => $.hints[hint])}</p> : null;
}

/** The validation messages by `validateAnswer` key; plural forms share one key. */
type ValidationKey = keyof (typeof questionsCopy)["validation"] extends infer K
  ? K extends `${infer Base}_${"one" | "other"}` ? Base : K
  : never;
type Hint = keyof (typeof questionsCopy)["hints"];

function hintFor(question: AnswerableQuestion): Hint | null {
  switch (question.answer_type) {
    case "text":
    case "ranking":
    case "person":
      return question.answer_type;
    case "yes_no":
      return question.settings.why_on_no ? "yes_no" : null;
    case "single_choice":
      return question.settings.allow_other ? "single_choice" : null;
    case "date":
      return question.settings.kind === "date" ? "date" : null;
    default:
      return null;
  }
}
