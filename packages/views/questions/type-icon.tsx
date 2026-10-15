import {
  BookOpenText,
  Bot,
  Calendar,
  CheckCheck,
  CircleDot,
  FolderKanban,
  Gauge,
  Hash,
  ListChecks,
  ListOrdered,
  ListTodo,
  MessageCircleQuestionMark,
  MessageSquare,
  Paperclip,
  Stamp,
  ToggleLeft,
  Type,
  UserRound,
  Users,
  Zap,
  type LucideIcon,
} from "lucide-react";
import type { Question, QuestionRowLink } from "@multica/core/types";

/** The icon of each answer type, as the UX spec's "Answer types" table names it. */
export const TYPE_ICONS: Record<Question["answer_type"], LucideIcon> = {
  text: Type,
  yes_no: ToggleLeft,
  single_choice: CircleDot,
  multiple_choice: ListChecks,
  confirm_facts: CheckCheck,
  ranking: ListOrdered,
  scale: Gauge,
  number: Hash,
  date: Calendar,
  person: UserRound,
  file: Paperclip,
  approval: Stamp,
  unknown: MessageCircleQuestionMark,
};

/** The icon of each page object kind, the one the sidebar shows for it. */
const LINK_ICONS: Record<QuestionRowLink["type"], LucideIcon> = {
  issue: ListTodo,
  project: FolderKanban,
  agent: Bot,
  squad: Users,
  autopilot: Zap,
  skill: BookOpenText,
  chat: MessageSquare,
};

/** The icon of a question's page object kind. */
export function LinkIcon({ type }: { type: QuestionRowLink["type"] }) {
  const Icon = LINK_ICONS[type];
  return <Icon aria-hidden className="size-3.5 shrink-0" />;
}
