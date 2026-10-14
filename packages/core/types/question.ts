// Questions an agent asks members during an issue run (ADR-0002). The wire
// contract of `/api/questions` and `GET /api/issues/{id}/questions`; the
// server's `internal/question` registry is the authority on answer types, and
// `api/schemas.ts` reads these shapes leniently.

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

export type QuestionLinkType = "issue" | "task";

/** Where a question belongs; position 0 is where it was asked. */
export interface QuestionLink {
  link_type: QuestionLinkType;
  link_id: string;
  position: number;
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
  asker_agent_id: string;
  asked_in_task_id: string;
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
  created_at: string;
  updated_at: string;
}

export interface TypedQuestion<T extends AnswerType> extends QuestionBase {
  answer_type: T;
  settings: AnswerSettingsByType[T];
  suggested_answer: AnswerValueByType[T] | null;
  answers: QuestionAnswer<AnswerValueByType[T]>[];
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
  value: AnswerValue;
  note?: string;
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
