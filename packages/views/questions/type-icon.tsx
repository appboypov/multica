import {
  Calendar,
  CheckCheck,
  CircleDot,
  Gauge,
  Hash,
  ListChecks,
  ListOrdered,
  MessageCircleQuestionMark,
  Paperclip,
  Stamp,
  ToggleLeft,
  Type,
  UserRound,
  type LucideIcon,
} from "lucide-react";
import type { Question } from "@multica/core/types";

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
