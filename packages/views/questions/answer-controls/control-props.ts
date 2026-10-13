import type { RefObject } from "react";
import type { AnswerType, Attachment, TypedQuestion } from "@multica/core/types";
import type { AnswerDraft, AnswerDraftByType } from "./draft";

/**
 * What the card reads from its control at the moment of sending: the last
 * reported draft and upload state can trail what the control holds.
 */
export interface AnswerControlHandle {
  /** Reports typing not reported yet and returns the draft as it now stands. */
  flush?: () => AnswerDraft;
  /** True while a file of the answer is still uploading. */
  isUploading?: () => boolean;
}

/** What every answer control gets from the card that holds it. */
export interface AnswerControlProps<T extends AnswerType> {
  question: TypedQuestion<T>;
  /**
   * What the control shows: the answerer's draft, else the asker's suggestion
   * that Accept suggestion sends; `null` while there is neither.
   */
  draft: AnswerDraftByType[T] | null;
  /** An edit is never `null`: emptied again, it still holds Accept suggestion back. */
  onDraftChange: (draft: AnswerDraftByType[T]) => void;
  /** The answer's note: the reason of a No, the note of Request changes. */
  note: string;
  onNoteChange: (note: string) => void;
  /** Sends the answer, as **Send** does (⌘↵ in Text). */
  onSubmit: () => void;
  /** Uploads a file of the answer onto the question (File and Text). */
  uploadFile: (file: File) => Promise<Attachment>;
  /** Filled by a control whose draft or uploads Send must read at sending time (Text, File). */
  handle: RefObject<AnswerControlHandle | null>;
  /** Called as files of the answer start and stop uploading; Send waits meanwhile. */
  onUploadingChange: (uploading: boolean) => void;
  disabled: boolean;
}
