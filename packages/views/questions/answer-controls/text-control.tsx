"use client";

import { useImperativeHandle, useRef } from "react";
import { toast } from "sonner";
import { createLogger } from "@multica/core/logger";
import { toUploadResult } from "@multica/core/hooks/use-file-upload";
import { contentReferencesAttachment } from "@multica/core/types";
import { cn } from "@multica/ui/lib/utils";
import { ContentEditor, type ContentEditorRef } from "../../editor";
import { useT } from "../../i18n";
import type { AnswerControlProps } from "./control-props";
import type { AnswerDraftByType } from "./draft";

const logger = createLogger("questions.text");

/** A file the answer can bind: an upload made here, or one of the answer being edited. */
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
 * Rich text as in comments, with mentions, links and attachments; `⌘↵`
 * sends. The answer binds only the files its text still references.
 */
export function TextControl({
  question,
  draft,
  onDraftChange,
  onSubmit,
  uploadFile,
  handle,
  onUploadingChange,
  disabled,
}: AnswerControlProps<"text">) {
  const { t } = useT("questions");
  const editor = useRef<ContentEditorRef>(null);
  const files = useRef<BoundFile[] | null>(null);
  if (files.current === null) files.current = earlierFiles(draft);
  const markdown = useRef(draft?.text ?? "");

  // Emptied text stays an edit, so Accept suggestion does not come back; the card sends no blank text.
  const draftOf = (text: string): AnswerDraftByType["text"] => {
    const ids = (files.current ?? []).filter((f) => f.keep || contentReferencesAttachment(text, f)).map((f) => f.id);
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
    isUploading: () => editor.current?.hasActiveUploads() === true,
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
        onUploadingChange={onUploadingChange}
        onUploadFile={async (file) => {
          try {
            const attachment = await uploadFile(file);
            files.current = [...(files.current ?? []), attachment];
            return toUploadResult(attachment);
          } catch (error) {
            logger.error("upload failed", { questionId: question.id, error });
            toast.error(t(($) => $.controls.file_failed, { name: file.name }));
            return null;
          }
        }}
      />
    </div>
  );
}
