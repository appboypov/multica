"use client";

import { useImperativeHandle, useMemo, useRef } from "react";
import { uploadedAttachments } from "@multica/core/drafts";
import { EMPTY_QUESTION_DRAFT, useQuestionDraftStore } from "@multica/core/questions";
import { contentReferencesAttachment } from "@multica/core/types";
import { cn } from "@multica/ui/lib/utils";
import { ContentEditor, type ContentEditorRef } from "../../editor";
import { useCoordinatedUploads, type UploadDraftBinding } from "../../editor/use-coordinated-uploads";
import { useUploadGate } from "../../editor/use-upload-gate";
import { useT } from "../../i18n";
import type { AnswerControlProps } from "./control-props";
import type { AnswerDraftByType } from "./draft";

/** A file the answer can bind: an upload made on this question, or one of the answer being edited. */
interface BoundFile {
  id: string;
  url: string;
  download_url?: string;
  markdown_url?: string;
  /** An earlier file the text links in a form not recognised here: no removal of it can be seen. */
  keep?: boolean;
}

/**
 * The files of the draft this control opens on, which come as ids only. The
 * text links each by its download path; one it links otherwise stays bound.
 */
function earlierFiles(draft: AnswerDraftByType["text"] | null): BoundFile[] {
  const text = draft?.text ?? "";
  return (draft?.attachment_ids ?? []).map((id) => {
    const file = { id, url: "" };
    return contentReferencesAttachment(text, file) ? file : { ...file, keep: true };
  });
}

/**
 * The question's draft as the coordinated-upload engine's target: its uploads
 * and, as the body, the answer's text. Every accessor reads the store, since a
 * settling upload calls them after this control unmounted.
 */
function questionUploadBinding(draftKey: string): UploadDraftBinding {
  const store = () => useQuestionDraftStore.getState();
  return {
    registryKey: `question:${draftKey}`,
    getUploads: () => store().drafts[draftKey]?.uploads ?? EMPTY_QUESTION_DRAFT.uploads,
    addUpload: (upload) => store().addUpload(draftKey, upload),
    settleUpload: (id, attachment) => store().settleUpload(draftKey, id, attachment),
    // The engine leaves nothing of a failed upload, so failing is removing.
    failUpload: (id) => store().removeUpload(draftKey, id),
    removeUpload: (id) => store().removeUpload(draftKey, id),
    getBody: () => (store().drafts[draftKey]?.answer as AnswerDraftByType["text"] | null | undefined)?.text ?? "",
    appendToBody: (markdown) => {
      const answer = store().drafts[draftKey]?.answer as AnswerDraftByType["text"] | null | undefined;
      const text = answer?.text ?? "";
      store().setDraft(draftKey, {
        answer: { ...answer, text: text.trim() ? `${text.replace(/\s+$/, "")}\n\n${markdown}` : markdown },
      });
    },
  };
}

/**
 * Rich text as in comments, with mentions, links and attachments; `⌘↵`
 * sends. The answer binds only the files its text still references.
 */
export function TextControl({ question, draft, onDraftChange, onSubmit, draftKey, handle, disabled }: AnswerControlProps<"text">) {
  const { t } = useT("questions");
  const editor = useRef<ContentEditorRef>(null);
  const editorGate = useUploadGate(editor);
  const binding = useMemo(() => questionUploadBinding(draftKey), [draftKey]);
  const storeUploads = useQuestionDraftStore((s) => s.drafts[draftKey]?.uploads ?? EMPTY_QUESTION_DRAFT.uploads);
  const uploads = useCoordinatedUploads(binding, storeUploads, { questionId: question.id }, editorGate, editor);
  const earlier = useRef<BoundFile[] | null>(null);
  if (earlier.current === null) earlier.current = earlierFiles(draft);
  const markdown = useRef(draft?.text ?? "");

  // Emptied text stays an edit, so Accept suggestion does not come back; the card sends no blank text.
  const draftOf = (text: string): AnswerDraftByType["text"] => {
    // Read when called: an upload can settle into the draft after this render.
    const uploaded: BoundFile[] = uploadedAttachments(binding.getUploads());
    const files = [...(earlier.current ?? []).filter((f) => !uploaded.some((u) => u.id === f.id)), ...uploaded];
    const ids = files.filter((f) => f.keep || contentReferencesAttachment(text, f)).map((f) => f.id);
    return ids.length > 0 ? { text, attachment_ids: ids } : { text };
  };

  const report = () => {
    const next = draftOf(markdown.current);
    onDraftChange(next);
    return next;
  };

  useImperativeHandle(handle, () => ({
    // The editor reports typing after its debounce; Send takes what it shows now.
    flush: () => {
      const pending = editor.current?.flushPendingUpdate() ?? null;
      if (pending === null) return draftOf(markdown.current);
      markdown.current = pending;
      return report();
    },
    isUploading: uploads.gate.isBlocked,
  }));

  return (
    <div
      className={cn(
        "rounded-lg border border-input px-3 py-2 text-body focus-within:border-ring focus-within:ring-3 focus-within:ring-ring/50",
        question.settings.length === "long" ? "min-h-24" : "min-h-10",
        disabled && "pointer-events-none opacity-60",
      )}
      aria-busy={disabled || undefined}
    >
      <ContentEditor
        ref={editor}
        defaultValue={draft?.text ?? ""}
        placeholder={question.settings.placeholder || t(($) => $.controls.text_placeholder)}
        debounceMs={100}
        // A card scrolled out of the timeline keeps the typing its editor had not reported yet.
        flushPendingOnUnmount
        onUpdate={(md) => {
          markdown.current = md;
          report();
        }}
        onSubmit={onSubmit}
        onUploadingChange={editorGate.onUploadingChange}
        onUploadFile={uploads.handleUpload}
      />
    </div>
  );
}
