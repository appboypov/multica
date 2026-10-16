// Questions an agent asks members during an issue run (ADR-0002). The wire
// contract of `/api/questions` and `GET /api/issues/{id}/questions`; the
// server's `internal/question` registry is the authority on answer types, and
// `api/schemas.ts` reads these shapes leniently.

import type { IssueStatus } from "./issue";
import type { RunInputMark } from "./run-input";

/** The twelve answer types; each has its own settings and value shape. */
export type AnswerType =
  | "text"
  | "yes_no"
  | "single_choice"
  | "multiple_choice"
  | "confirm_facts"
  | "ranking"
  | "scale"
  | "number"
  | "date"
  | "person"
  | "file"
  | "approval";

// ---------------------------------------------------------------------------
// Settings: what the asker gave for the question's answer type. Options, items
// and facts are labels that never change, since a question is never edited.
// ---------------------------------------------------------------------------

export interface TextAnswerSettings {
  length: "short" | "long";
  placeholder?: string;
}

export interface YesNoAnswerSettings {
  /** No needs the answer's note. */
  why_on_no: boolean;
}

export interface SingleChoiceAnswerSettings {
  options: string[];
  allow_other: boolean;
}

export interface MultipleChoiceAnswerSettings {
  options: string[];
  min: number;
  max: number;
}

export interface ConfirmFactsAnswerSettings {
  facts: string[];
}

export interface RankingAnswerSettings {
  items: string[];
}

/** Integer points from `min` to `max`, 2–11 of them. */
export interface ScaleAnswerSettings {
  min: number;
  max: number;
  min_label?: string;
  max_label?: string;
}

export interface NumberAnswerSettings {
  unit?: string;
  min?: number;
  max?: number;
}

export type DateAnswerKind = "date" | "datetime" | "range";

export interface DateAnswerSettings {
  kind: DateAnswerKind;
}

export interface PersonAnswerSettings {
  allow: "members" | "agents" | "both";
}

export interface FileAnswerSettings {
  /** MIME types or `.ext` extensions. */
  accept: string[];
}

export interface ApprovalAnswerSettings {
  subject: string;
  link?: { title: string; url: string };
}

export interface AnswerSettingsByType {
  text: TextAnswerSettings;
  yes_no: YesNoAnswerSettings;
  single_choice: SingleChoiceAnswerSettings;
  multiple_choice: MultipleChoiceAnswerSettings;
  confirm_facts: ConfirmFactsAnswerSettings;
  ranking: RankingAnswerSettings;
  scale: ScaleAnswerSettings;
  number: NumberAnswerSettings;
  date: DateAnswerSettings;
  person: PersonAnswerSettings;
  file: FileAnswerSettings;
  approval: ApprovalAnswerSettings;
}

export type AnswerSettings<T extends AnswerType = AnswerType> = AnswerSettingsByType[T];

// ---------------------------------------------------------------------------
// Values: an answer, or the suggested answer, in the question's answer type.
// The "why" of Yes / No and the note of Request changes are the answer's note,
// not part of the value.
// ---------------------------------------------------------------------------

export interface TextAnswerValue {
  text: string;
  attachment_ids?: string[];
}

export interface YesNoAnswerValue {
  choice: "yes" | "no";
}

export type SingleChoiceAnswerValue = { option: string } | { other: string };

export interface MultipleChoiceAnswerValue {
  options: string[];
}

export type FactCheck = { correct: true } | { correct: false; correction: string };

/** One check per fact, in the order of the settings' facts. */
export interface ConfirmFactsAnswerValue {
  facts: FactCheck[];
}

/** A permutation of the settings' items. */
export interface RankingAnswerValue {
  order: string[];
}

export interface ScaleAnswerValue {
  point: number;
}

export interface NumberAnswerValue {
  number: number;
}

/** `date` is YYYY-MM-DD; `datetime`, `start` and `end` are RFC 3339. */
export type DateAnswerValue = { date: string } | { datetime: string } | { start: string; end: string };

export interface PersonAnswerValue {
  type: "member" | "agent";
  id: string;
}

export interface FileAnswerValue {
  attachment_ids: string[];
}

export type ApprovalDecision = "approve" | "request_changes" | "decline";

export interface ApprovalAnswerValue {
  decision: ApprovalDecision;
}

export interface AnswerValueByType {
  text: TextAnswerValue;
  yes_no: YesNoAnswerValue;
  single_choice: SingleChoiceAnswerValue;
  multiple_choice: MultipleChoiceAnswerValue;
  confirm_facts: ConfirmFactsAnswerValue;
  ranking: RankingAnswerValue;
  scale: ScaleAnswerValue;
  number: NumberAnswerValue;
  date: DateAnswerValue;
  person: PersonAnswerValue;
  file: FileAnswerValue;
  approval: ApprovalAnswerValue;
}

export type AnswerValue<T extends AnswerType = AnswerType> = AnswerValueByType[T];

/** The answer of an answerer who declines to answer, in every answer type; the note is the optional reason. */
export interface DeclinedAnswerValue {
  declined: true;
}

// ---------------------------------------------------------------------------
// The question aggregate.
// ---------------------------------------------------------------------------

/**
 * Overdue is no status: it derives from `due_date` while open. `open` and
 * `partly_answered` are both open (ADR-0004).
 */
export type QuestionStatus = "open" | "partly_answered" | "answered" | "closed" | "withdrawn";

/**
 * When answers make a question answered: under `any_one` the first answer of a
 * current answerer does; under `each` every current answerer must answer, and
 * the question is `partly_answered` until then.
 */
export type AnswerPolicy = "any_one" | "each";

export type QuestionLinkType =
  | "issue"
  | "task"
  | "comment"
  | "project"
  | "agent"
  | "squad"
  | "autopilot"
  | "skill";

/** Where a question belongs; position 0 is where it was asked. */
export interface QuestionLink {
  link_type: QuestionLinkType;
  link_id: string;
  position: number;
}

/** The first link that reaches the page's object or the viewer's work. */
export interface QuestionVia {
  type: "issue" | "sub_issue" | "comment" | "task";
  id: string;
  identifier: string | null;
  issue_identifier: string | null;
}

/** One agent offered Wake on its own issue; the asking agent is first. */
export interface QuestionWakeTarget {
  agent_id: string;
  issue_id: string;
  unseen_answer_count: number;
  wake_available: boolean;
}

/** A current answerer: always a member. */
export interface QuestionAnswerer {
  user_id: string;
  assigned_at: string;
}

/** One answer per answerer, edited in place. */
export interface QuestionAnswer<V = AnswerValue> {
  id: string;
  answerer_user_id: string;
  value: V;
  /** The server's one-line text of the value, as the folded card shows it. */
  display_text: string;
  note: string | null;
  created_at: string;
  updated_at: string;
  /** Marks on this answer's latest answered/edited event, one per receiving run. */
  run_marks?: RunInputMark[];
}

export type QuestionEventKind =
  | "asked"
  | "answered"
  | "answer_edited"
  | "ask_back"
  | "reassigned"
  | "closed"
  | "withdrawn"
  | "woken";

/** The `details` each event kind carries; kinds without any carry `{}`. */
export interface QuestionEventDetailsByKind {
  asked: Record<string, never>;
  answered: Record<string, never>;
  answer_edited: Record<string, never>;
  /** The answerer's reply in the question's thread. */
  ask_back: { text: string };
  /** The actor reassigned the question to `user_id`. */
  reassigned: { user_id: string; note: string };
  closed: Record<string, never>;
  withdrawn: Record<string, never>;
  /** A member pressed Wake: the run it queued and the answers it carries. */
  woken: { task_id: string; answer_count: number };
}

interface QuestionEventOf<K extends QuestionEventKind> {
  id: string;
  kind: K;
  actor_type: "member" | "agent";
  actor_id: string | null;
  details: QuestionEventDetailsByKind[K];
  created_at: string;
}

/** One entry of a question's history and thread, oldest first. */
export type QuestionEvent = { [K in QuestionEventKind]: QuestionEventOf<K> }[QuestionEventKind];

interface QuestionBase {
  id: string;
  workspace_id: string;
  /** The asking agent; `""` for a question a member asked outside a run. */
  asker_agent_id: string;
  /** The run the agent asked in; `""` for a question a member asked outside a run. */
  asked_in_task_id: string;
  /** The member who asked outside a run; `""` for an agent's question. */
  asker_user_id: string;
  /** The id of the first question of its ask; a one-question ask has `ask_id = id`. */
  ask_id: string;
  /** 0-based place in its ask. */
  ask_position: number;
  /** The set's title, set only when the ask holds several questions. */
  ask_title: string | null;
  answer_policy: AnswerPolicy;
  prompt: string;
  context_line: string | null;
  status: QuestionStatus;
  /** Date-only, YYYY-MM-DD, as issue due dates are. */
  due_date: string | null;
  blocks_work: boolean;
  links: QuestionLink[];
  answerers: QuestionAnswerer[];
  events: QuestionEvent[];
  /** Answers unseen since the claim of the asking agent's latest started run on the issue. */
  unseen_answer_count: number;
  /** True while there are unseen answers and the asking agent has no active task on the issue. */
  wake_available: boolean;
  wake_targets: QuestionWakeTarget[];
  created_at: string;
  updated_at: string;
}

export interface TypedQuestion<T extends AnswerType> extends QuestionBase {
  answer_type: T;
  settings: AnswerSettingsByType[T];
  suggested_answer: AnswerValueByType[T] | null;
  answers: QuestionAnswer<AnswerValueByType[T] | DeclinedAnswerValue>[];
}

/**
 * A question this client cannot read: an answer type from a newer server, or
 * settings or values that do not fit their type. It shows its prompt and the
 * server's display text of each answer, and offers no answer controls.
 */
export interface UnreadableQuestion extends QuestionBase {
  answer_type: "unknown";
  settings: Record<string, unknown>;
  suggested_answer: null;
  answers: QuestionAnswer<unknown>[];
}

/** Narrow on `answer_type` to read `settings`, `suggested_answer` and answer values. */
export type Question = { [T in AnswerType]: TypedQuestion<T> }[AnswerType] | UnreadableQuestion;

// ---------------------------------------------------------------------------
// Request and response bodies of the member endpoints.
// ---------------------------------------------------------------------------

/** Body of `POST /api/questions/{id}/answer`: a first answer or an edit. */
export interface AnswerQuestionRequest {
  value: AnswerValue | DeclinedAnswerValue;
  note?: string;
  /** Wakes the asking agent once the answer is saved. */
  wake?: boolean;
}

/** Body of `POST /api/questions/{id}/ask-back`. */
export interface AskBackQuestionRequest {
  text: string;
}

/** Body of `POST /api/questions/{id}/reassign`; the note is required. */
export interface ReassignQuestionRequest {
  user_id: string;
  note: string;
}

/** 202 body of `POST /api/questions/{id}/wake`. */
export interface QuestionWakeResult {
  task_id: string;
  answer_count: number;
}

/** Body of `GET`/`PUT /api/agent-questions`: the workspace Questions default. */
export interface AgentQuestionsSettings {
  enabled: boolean;
  /** Open issues with their own Agent questions; they ignore the default. */
  own_setting_count: number;
  /** The brief's Questions section agents get while questions are on. */
  brief_section: string;
}

/** 201 body of `POST /api/questions`: one ask of one or more questions, in ask order. */
export interface QuestionAskResult {
  ask_id: string;
  questions: Question[];
}

// ---------------------------------------------------------------------------
// The row list of the Questions page (`GET /api/questions`, ADR-0006): one
// projection of a question per row, listed for the viewer by tab.
// ---------------------------------------------------------------------------

/**
 * `for_me` holds the questions open for the viewer (ADR-0004).
 * `on_my_work` holds open questions on the viewer's work even when
 * they are not an answerer, `all_open` every open question in the workspace,
 * `answered` the questions the viewer answered, newest answer first.
 */
export type QuestionTab = "for_me" | "on_my_work" | "all_open" | "answered";

/** The first link's page object: comments and runs use their issue. */
export interface QuestionRowLink {
  type: Exclude<QuestionLinkType, "comment" | "task">;
  id: string;
  /** The issue's identifier, such as WORK-412; null for objects without one. */
  identifier: string | null;
  title: string;
  status: IssueStatus | null;
}

/** The viewer's own answer to the row's question. */
export interface QuestionRowAnswer {
  /** The server's one-line text of the value, as the folded card shows it. */
  display: string;
  updated_at: string;
}

interface QuestionRowBase {
  id: string;
  /** The id of the first question of its ask; a one-question ask has `ask_id = id`. */
  ask_id: string;
  /** 0-based place in its ask. */
  ask_position: number;
  /** The set's title, set only when the ask holds several questions. */
  ask_title: string | null;
  prompt: string;
  status: QuestionStatus;
  answer_policy: AnswerPolicy;
  /** The asking agent; `""` for a question a member asked outside a run. */
  asker_agent_id: string;
  /** The member who asked outside a run; `""` for an agent's question. */
  asker_user_id: string;
  /** The current answerers' user ids. */
  answerer_ids: string[];
  /** Date-only, YYYY-MM-DD, as issue due dates are. */
  due_date: string | null;
  blocks_work: boolean;
  created_at: string;
  link: QuestionRowLink;
  /** Null for a direct link or outside a parent list and On my work. */
  via: QuestionVia | null;
  /** The issue where the question's card opens, regardless of the first link's type. */
  open_issue: { id: string; identifier: string };
  /** Null while the viewer has not answered. */
  my_answer: QuestionRowAnswer | null;
}

export interface TypedQuestionRow<T extends AnswerType> extends QuestionRowBase {
  answer_type: T;
  settings: AnswerSettingsByType[T];
}

/** A row whose answer type or settings this client cannot read; it offers no answer controls. */
export interface UnreadableQuestionRow extends QuestionRowBase {
  answer_type: "unknown";
  settings: Record<string, unknown>;
}

/** Narrow on `answer_type` to read `settings`. */
export type QuestionRow = { [T in AnswerType]: TypedQuestionRow<T> }[AnswerType] | UnreadableQuestionRow;

/** 200 body of `GET /api/questions`: one page of rows; `next_cursor` is null on the last page. */
export interface QuestionRowPage {
  rows: QuestionRow[];
  next_cursor: string | null;
}

/**
 * The filters of `GET /api/questions`. Each list sends one repeated query
 * parameter per entry (`answer_type`, `asker_id`, `answerer_id`, `link_id`);
 * an empty list filters nothing. `blocks_work` true keeps only the questions
 * that block work.
 */
export interface QuestionRowFilters {
  readonly answer_types: readonly AnswerType[];
  readonly asker_ids: readonly string[];
  readonly answerer_ids: readonly string[];
  readonly link_ids: readonly string[];
  readonly blocks_work: boolean;
  readonly parent?: { type: "issue" | "project"; id: string };
}

/** 200 body of `GET /api/questions/open-count`, scoped by tab and optional parent. */
export interface QuestionOpenCount {
  count: number;
  blocking_count: number;
}
