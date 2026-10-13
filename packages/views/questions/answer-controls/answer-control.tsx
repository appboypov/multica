"use client";

import { useRef, useState, type KeyboardEvent, type ReactNode, type RefObject } from "react";
import type { AnswerType, Attachment } from "@multica/core/types";
import type { AnswerControlHandle, AnswerControlProps } from "./control-props";
import type { AnswerDraft, AnswerableQuestion } from "./draft";
import { draftForKey } from "./keys";
import { ApprovalControl } from "./approval-control";
import { ConfirmFactsControl } from "./confirm-facts-control";
import { DateControl } from "./date-control";
import { FileControl } from "./file-control";
import { MultipleChoiceControl } from "./multiple-choice-control";
import { NumberControl } from "./number-control";
import { PersonControl } from "./person-control";
import { RankingControl } from "./ranking-control";
import { ScaleControl } from "./scale-control";
import { SingleChoiceControl } from "./single-choice-control";
import { TextControl } from "./text-control";
import { YesNoControl } from "./yes-no-control";

// Each control takes the props of its own type; the map hands every question
// to the control of its `answer_type`.
const CONTROLS: { [T in AnswerType]: (props: AnswerControlProps<T>) => ReactNode } = {
  text: TextControl,
  yes_no: YesNoControl,
  single_choice: SingleChoiceControl,
  multiple_choice: MultipleChoiceControl,
  confirm_facts: ConfirmFactsControl,
  ranking: RankingControl,
  scale: ScaleControl,
  number: NumberControl,
  date: DateControl,
  person: PersonControl,
  file: FileControl,
  approval: ApprovalControl,
};

export interface AnswerControlInput {
  question: AnswerableQuestion;
  draft: AnswerDraft;
  onDraftChange: (draft: AnswerDraft) => void;
  note: string;
  onNoteChange: (note: string) => void;
  onSubmit: () => void;
  uploadFile: (file: File) => Promise<Attachment>;
  handle: RefObject<AnswerControlHandle | null>;
  onUploadingChange: (uploading: boolean) => void;
  disabled: boolean;
}

/** The control of the question's answer type. */
export function AnswerControl(props: AnswerControlInput) {
  const Control = CONTROLS[props.question.answer_type] as (props: AnswerControlInput) => ReactNode;
  return <Control {...props} />;
}

/** The card's side of its control: what Send waits for and what it sends. */
export interface AnswerControlLink {
  /** Pass to the control as `handle`. */
  handle: RefObject<AnswerControlHandle | null>;
  /** True while a file of the answer is still uploading, for the rendered card. */
  uploading: boolean;
  /** Pass to the control as `onUploadingChange`. */
  onUploadingChange: (uploading: boolean) => void;
  /** True while a file of the answer is still uploading, read when sending. */
  isUploading: () => boolean;
  /** The draft to send: the shown draft with typing the control has not reported yet. */
  currentDraft: (shown: AnswerDraft) => AnswerDraft;
}

/**
 * Links a card to its control. The rendered `uploading` drives the card; the
 * reads at sending time cover what it trails: `⌘↵` bypasses the button, and
 * the editor reports typing only after its debounce.
 */
export function useAnswerControlLink(): AnswerControlLink {
  const handle = useRef<AnswerControlHandle | null>(null);
  const [uploading, setUploading] = useState(false);
  return {
    handle,
    uploading,
    onUploadingChange: setUploading,
    isUploading: () => handle.current?.isUploading?.() === true,
    currentDraft: (shown) => (handle.current?.flush ? handle.current.flush() : shown),
  };
}

/**
 * The focus keys of the UX spec for a card that holds an answer control: `Y`
 * `N` for Yes / No and `1`-`9` for choices. Keys typed into a field, or
 * pressed with a modifier, stay with that field.
 */
export function handleAnswerKey(
  event: KeyboardEvent,
  { question, draft, onDraftChange, disabled }: Pick<AnswerControlInput, "question" | "draft" | "onDraftChange" | "disabled">,
) {
  if (disabled || event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey) return;
  const target = event.target as HTMLElement;
  if (target.closest("input, textarea, select, [contenteditable='true']")) return;
  const next = draftForKey(question, draft, event.key);
  if (next === undefined) return;
  event.preventDefault();
  onDraftChange(next);
}
