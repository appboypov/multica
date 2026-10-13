"use client";

import { useState, type ReactNode } from "react";
import { ArrowRight, Calendar, CircleCheck, CirclePause, CircleSlash, ClockAlert, CornerUpLeft, NotebookPen, Pencil, Send, Check } from "lucide-react";
import { toast } from "sonner";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { formatDateOnly, isPastDateOnly, todayDateOnly } from "@multica/core/issues/date";
import { createLogger } from "@multica/core/logger";
import { useAnswerQuestion, useAskBackQuestion, useUploadQuestionFile, type AnswerValidationError } from "@multica/core/questions";
import type { AnswerValue, Question, QuestionAnswer, QuestionEvent } from "@multica/core/types";
import { useActorName } from "@multica/core/workspace/hooks";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { ActorAvatar } from "../common/actor-avatar";
import { useLocale, useT, useTimeAgo } from "../i18n";
import { AnswerControl, handleAnswerKey } from "./answer-controls/answer-control";
import { controlShowsNote, draftError, valueToSend, type AnswerDraft, type AnswerableQuestion } from "./answer-controls/draft";
import { ReassignPicker } from "./reassign-picker";
import { TYPE_ICONS } from "./type-icon";
import { WakeButton } from "./wake-button";
import type questionsCopy from "../locales/en/questions.json";

const logger = createLogger("questions.card");

export interface QuestionCardProps {
  question: Question;
  /** The issue the card shows on, whose questions refresh after each action. */
  issueId: string;
  /** The asking run's place among its agent's runs on the issue, for "asked during run #N". */
  runNumber?: number;
  /**
   * Shown on an open card in place of its controls and summary, where the
   * question is answered somewhere else: the inbox's way to the issue for a
   * type it does not answer in place.
   */
  answerElsewhere?: ReactNode;
}

/**
 * A question in the issue's timeline, in the comment card's frame. Open, it
 * holds its answer type's control for an answerer and a summary for anyone
 * else; once answered, closed or withdrawn it folds to one line.
 */
export function QuestionCard({ question, issueId, runNumber, answerElsewhere }: QuestionCardProps) {
  const userId = useAuthStore((s) => s.user?.id);
  const [editing, setEditing] = useState(false);
  const isAnswerer = !!userId && question.answerers.some((a) => a.user_id === userId);

  if (question.status !== "open" && !editing) {
    const canEdit = isAnswerer && question.status === "answered" && question.answer_type !== "unknown";
    return <FoldedQuestion question={question} issueId={issueId} onEdit={canEdit ? () => setEditing(true) : undefined} />;
  }
  return (
    <OpenQuestion
      question={question}
      issueId={issueId}
      runNumber={runNumber}
      answerElsewhere={answerElsewhere}
      isAnswerer={isAnswerer}
      ownAnswer={question.answers.find((a) => a.answerer_user_id === userId) ?? null}
      editing={editing}
      onDone={() => setEditing(false)}
    />
  );
}

// ---------------------------------------------------------------------------
// Folded: prompt → answer · who · when, Edit answer, Wake <agent>
// ---------------------------------------------------------------------------

function FoldedQuestion({ question, issueId, onEdit }: { question: Question; issueId: string; onEdit?: () => void }) {
  const { t } = useT("questions");
  const timeAgo = useTimeAgo();
  const { getActorName } = useActorName();
  const answer = latestAnswer(question.answers);
  const Icon = answer ? CircleCheck : CircleSlash;

  return (
    <div className="flex min-h-12 min-w-0 items-center gap-2 rounded-xl border bg-card px-3.5 py-1.5 text-body" data-question-state={question.status}>
      <Icon aria-hidden className={cn("size-4 shrink-0", answer ? "text-success" : "text-muted-foreground")} />
      <span className="min-w-0 max-w-[40%] truncate text-muted-foreground" title={question.prompt}>{question.prompt}</span>
      <ArrowRight aria-hidden className="size-4 shrink-0 text-muted-foreground" />
      {answer ? (
        <>
          <span className="min-w-0 truncate font-medium">{answer.display_text}</span>
          <span className="shrink-0 text-caption text-muted-foreground">
            · {t(($) => $.card.answered_by, { name: getActorName("member", answer.answerer_user_id), time: timeAgo(answer.updated_at) })}
          </span>
        </>
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
        {question.wake_available && <WakeButton question={question} issueId={issueId} />}
      </span>
    </div>
  );
}

function latestAnswer(answers: QuestionAnswer<unknown>[]): QuestionAnswer<unknown> | null {
  return answers.reduce<QuestionAnswer<unknown> | null>(
    (latest, answer) => (!latest || answer.updated_at > latest.updated_at ? answer : latest),
    null,
  );
}

// ---------------------------------------------------------------------------
// Open: overline, asker, prompt, history, control, footer
// ---------------------------------------------------------------------------

function OpenQuestion({
  question,
  issueId,
  runNumber,
  answerElsewhere,
  isAnswerer,
  ownAnswer,
  editing,
  onDone,
}: {
  question: Question;
  issueId: string;
  runNumber?: number;
  answerElsewhere?: ReactNode;
  isAnswerer: boolean;
  ownAnswer: QuestionAnswer<unknown> | null;
  editing: boolean;
  onDone: () => void;
}) {
  const { t } = useT("questions");
  const timeAgo = useTimeAgo();
  const { getActorName } = useActorName();
  const agentName = getActorName("agent", question.asker_agent_id);
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
        <QuestionBadges question={question} />
      </div>

      <div className="flex items-center gap-2 text-body">
        <ActorAvatar actorType="agent" actorId={question.asker_agent_id} size="sm" />
        <span className="font-medium">{agentName}</span>
        <span className="truncate text-caption text-muted-foreground">
          {runNumber !== undefined
            ? t(($) => $.card.asked_during_run, { number: runNumber, time: timeAgo(question.created_at) })
            : t(($) => $.card.asked_during_a_run, { time: timeAgo(question.created_at) })}
        </span>
      </div>

      <div className="flex flex-col gap-1">
        <p className="text-title-sm font-semibold">{question.prompt}</p>
        {question.context_line && <p className="text-body text-muted-foreground">{question.context_line}</p>}
      </div>

      <QuestionHistory events={question.events} />

      {answerElsewhere ?? (controls ? (
        <AnswerForm
          key={editing ? "editing" : "open"}
          question={answerable}
          issueId={issueId}
          agentName={agentName}
          ownAnswer={editing ? ownAnswer : null}
          editing={editing}
          onDone={onDone}
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

function QuestionBadges({ question }: { question: Question }) {
  const { t } = useT("questions");
  const locale = useLocale();
  const overdue = question.status === "open" && isPastDateOnly(question.due_date);
  return (
    <span className="ml-auto flex shrink-0 items-center gap-1.5">
      {question.blocks_work && (
        <Badge variant="destructive" className="border-destructive">
          <CirclePause />
          {t(($) => $.card.blocks_work)}
        </Badge>
      )}
      {overdue ? (
        <Badge variant="destructive" className="border-destructive">
          <ClockAlert />
          {t(($) => $.card.overdue)}
        </Badge>
      ) : question.due_date && (
        <Badge variant="secondary">
          <Calendar />
          {question.due_date === todayDateOnly()
            ? t(($) => $.card.due_today)
            : t(($) => $.card.due_date, { date: formatDateOnly(question.due_date, { month: "short", day: "numeric" }, locale) })}
        </Badge>
      )}
    </span>
  );
}

/** The question's thread: ask-backs and reassignments, oldest first. */
function QuestionHistory({ events }: { events: QuestionEvent[] }) {
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
  issueId,
  agentName,
  ownAnswer,
  editing,
  onDone,
}: {
  question: AnswerableQuestion;
  issueId: string;
  agentName: string;
  ownAnswer: QuestionAnswer<unknown> | null;
  editing: boolean;
  onDone: () => void;
}) {
  const { t } = useT("questions");
  const wsId = useWorkspaceId();
  const answer = useAnswerQuestion(wsId, issueId);
  const askBack = useAskBackQuestion(wsId, issueId);
  const upload = useUploadQuestionFile(wsId, issueId);

  // Editing starts from the sent answer; a first answer from nothing picked.
  const [draft, setDraft] = useState<AnswerDraft>((ownAnswer?.value as AnswerDraft | undefined) ?? null);
  const [note, setNote] = useState(ownAnswer?.note ?? "");
  const [noteOpen, setNoteOpen] = useState(!!ownAnswer?.note);
  const [askBackOpen, setAskBackOpen] = useState(false);
  const [askBackText, setAskBackText] = useState("");
  const [serverError, setServerError] = useState<string | null>(null);

  const changeDraft = (next: AnswerDraft) => {
    setDraft(next);
    setServerError(null);
  };

  const accepting = draft === null && question.suggested_answer !== null;
  const value: AnswerValue | null = accepting ? question.suggested_answer : valueToSend(question, draft);
  const error = accepting ? validateSuggestion(question, note) : draftError(question, draft, note);
  const noteInControl = controlShowsNote(question, draft);
  const pending = answer.isPending;

  const send = () => {
    if (value === null || error !== null || pending) return;
    answer.mutate(
      { id: question.id, value, ...(note.trim() ? { note: note.trim() } : {}) },
      {
        onSuccess: onDone,
        onError: (err) => {
          logger.error("answer failed", { questionId: question.id, error: err });
          // The server is the authority; its refusal shows where the client's would.
          setServerError(err.message || t(($) => $.card.answer_failed));
        },
      },
    );
  };

  const sendAskBack = () => {
    const text = askBackText.trim();
    if (!text || askBack.isPending) return;
    askBack.mutate(
      { id: question.id, text },
      {
        onSuccess: () => {
          setAskBackText("");
          setAskBackOpen(false);
        },
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
      onKeyDown={(event) => handleAnswerKey(event, { question, draft, onDraftChange: changeDraft, disabled: pending })}
    >
      <AnswerControl
        question={question}
        draft={draft}
        onDraftChange={changeDraft}
        note={note}
        onNoteChange={setNote}
        onSubmit={send}
        uploadFile={(file) => upload.mutateAsync({ id: question.id, file })}
        disabled={pending}
      />

      <AnswerLine question={question} error={error} serverError={serverError} />

      {noteOpen && !noteInControl && (
        <Textarea
          value={note}
          disabled={pending}
          aria-label={t(($) => $.card.note_label)}
          placeholder={t(($) => $.card.note_placeholder)}
          className="min-h-12"
          onChange={(event) => setNote(event.target.value)}
        />
      )}

      {askBackOpen && (
        <div className="flex flex-col gap-2 rounded-lg border border-border p-2">
          <Textarea
            autoFocus
            value={askBackText}
            disabled={askBack.isPending}
            aria-label={t(($) => $.card.ask_back_label, { name: agentName })}
            placeholder={t(($) => $.card.ask_back_placeholder, { name: agentName })}
            className="min-h-12 border-0 p-1 shadow-none focus-visible:ring-0"
            onChange={(event) => setAskBackText(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
                event.preventDefault();
                sendAskBack();
              }
            }}
          />
          <div className="flex justify-end gap-1.5">
            <Button type="button" variant="ghost" size="sm" onClick={() => setAskBackOpen(false)}>
              {t(($) => $.card.cancel)}
            </Button>
            <Button type="button" size="sm" disabled={!askBackText.trim() || askBack.isPending} onClick={sendAskBack}>
              {t(($) => $.card.ask_back_send)}
            </Button>
          </div>
        </div>
      )}

      <div className="flex flex-wrap items-center gap-1">
        {!noteInControl && (
          <Button type="button" variant="ghost" size="sm" className="text-muted-foreground" aria-expanded={noteOpen} onClick={() => setNoteOpen((open) => !open)}>
            <NotebookPen />
            {t(($) => $.card.add_note)}
          </Button>
        )}
        {!editing && (
          <>
            <Button type="button" variant="ghost" size="sm" className="text-muted-foreground" aria-expanded={askBackOpen} onClick={() => setAskBackOpen((open) => !open)}>
              <CornerUpLeft />
              {t(($) => $.card.ask_back)}
            </Button>
            <ReassignPicker question={question} issueId={issueId} disabled={pending} />
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
          <Button type="button" disabled={value === null || error !== null || pending} onClick={send}>
            {accepting ? <Check /> : <Send />}
            {accepting ? t(($) => $.card.accept_suggestion) : t(($) => $.card.send)}
          </Button>
        </span>
      </div>
    </div>
  );
}

/** Accept suggestion sends the suggestion as it is, under the same rules as Send. */
function validateSuggestion(question: AnswerableQuestion, note: string): AnswerValidationError | null {
  return question.suggested_answer === null ? null : draftError(question, question.suggested_answer as AnswerDraft, note);
}

/** The line above the footer: why Send is held back, else the type's hint. */
function AnswerLine({
  question,
  error,
  serverError,
}: {
  question: AnswerableQuestion;
  error: AnswerValidationError | null;
  serverError: string | null;
}) {
  const { t } = useT("questions");
  if (serverError) return <p role="alert" className="text-caption text-destructive">{serverError}</p>;
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
