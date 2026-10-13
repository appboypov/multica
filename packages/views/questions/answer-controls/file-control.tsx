"use client";

import { useRef, useState, type ClipboardEvent, type DragEvent } from "react";
import { CloudUpload, File as FileIcon, Loader2, X } from "lucide-react";
import { toast } from "sonner";
import { api } from "@multica/core/api";
import { hasUploadingDraft, isUploaded, startUpload } from "@multica/core/drafts";
import { createLogger } from "@multica/core/logger";
import { EMPTY_QUESTION_DRAFT, useQuestionDraftStore } from "@multica/core/questions";
import { createSafeId } from "@multica/core/utils";
import { Button } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { formatBytes } from "../../common/format-bytes";
import { useT } from "../../i18n";
import type { AnswerControlProps } from "./control-props";
import type { AnswerDraftByType } from "./draft";

const logger = createLogger("questions.file");

/** True when the file is of a kind the asker accepts: a MIME type, `type/*` or `.ext`. */
export function acceptsFile(accept: string[], file: { name: string; type: string }): boolean {
  if (accept.length === 0) return true;
  const name = file.name.toLowerCase();
  const type = file.type.toLowerCase();
  return accept.some((kind) => {
    const k = kind.trim().toLowerCase();
    if (k.startsWith(".")) return name.endsWith(k);
    if (k.endsWith("/*")) return type.startsWith(k.slice(0, -1));
    return type === k;
  });
}

/**
 * Uploads a picked file onto the question through the upload coordinator,
 * which owns the request rather than the control: a card scrolled out of the
 * timeline unmounts, and its file still lands in the question's draft.
 */
function uploadAnswerFile(draftKey: string, questionId: string, file: File, onFailed: () => void) {
  const clientUploadId = createSafeId();
  useQuestionDraftStore.getState().addUpload(draftKey, {
    clientUploadId,
    status: "uploading",
    filename: file.name,
    size: file.size,
    contentType: file.type || undefined,
  });
  startUpload({
    clientUploadId,
    file,
    api,
    ctx: { questionId },
    onSettled: (outcome) => {
      const { drafts, settleUpload, removeUpload, setDraft } = useQuestionDraftStore.getState();
      const draft = drafts[draftKey];
      // The draft was sent or dropped, or the file removed, while it uploaded.
      if (!draft?.uploads.some((u) => u.clientUploadId === clientUploadId)) return;
      if (outcome.status === "failed") {
        removeUpload(draftKey, clientUploadId);
        logger.error("upload failed", { questionId, error: outcome.error });
        onFailed();
        return;
      }
      settleUpload(draftKey, clientUploadId, outcome.attachment);
      // Added to the files as they are now, with the picks and removals made while it uploaded.
      const ids = (draft.answer as AnswerDraftByType["file"] | null)?.attachment_ids ?? [];
      setDraft(draftKey, { answer: { attachment_ids: [...ids, outcome.attachment.id] } });
    },
  });
}

/**
 * A drop zone and one row per file; a pasted screenshot counts (`⌘V`). Each
 * file uploads onto the question as it lands, and the answer is their ids.
 */
export function FileControl({ question, draft, onDraftChange, draftKey, disabled }: AnswerControlProps<"file">) {
  const { t } = useT("questions");
  const { accept } = question.settings;
  const input = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);
  const uploads = useQuestionDraftStore((s) => s.drafts[draftKey]?.uploads ?? EMPTY_QUESTION_DRAFT.uploads);
  const removeUpload = useQuestionDraftStore((s) => s.removeUpload);
  const ids = draft?.attachment_ids ?? [];

  const add = (picked: File[]) => {
    for (const file of picked) {
      if (!acceptsFile(accept, file)) {
        toast.error(t(($) => $.controls.file_refused, { name: file.name }));
        continue;
      }
      uploadAnswerFile(draftKey, question.id, file, () => toast.error(t(($) => $.controls.file_failed, { name: file.name })));
    }
  };

  const onDrop = (event: DragEvent) => {
    event.preventDefault();
    setDragging(false);
    if (!disabled) add(Array.from(event.dataTransfer.files));
  };

  const onPaste = (event: ClipboardEvent) => {
    const pasted = Array.from(event.clipboardData.files);
    if (disabled || pasted.length === 0) return;
    event.preventDefault();
    add(pasted);
  };

  return (
    <div className="flex flex-col gap-2" onPaste={onPaste}>
      <button
        type="button"
        disabled={disabled}
        onClick={() => input.current?.click()}
        onDragOver={(event) => {
          event.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={onDrop}
        className={cn(
          "flex flex-col items-center gap-1 rounded-lg border border-dashed px-4 py-5 text-center outline-none transition-colors",
          "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50",
          dragging ? "border-ring bg-muted" : "border-transparent bg-muted/60 hover:bg-muted",
        )}
      >
        {hasUploadingDraft(uploads) ? (
          <Loader2 aria-hidden className="size-4 animate-spin text-muted-foreground motion-reduce:animate-none" />
        ) : (
          <CloudUpload aria-hidden className="size-4 text-muted-foreground" />
        )}
        <span className="text-body font-medium">{t(($) => $.controls.file_drop)}</span>
        {accept.length > 0 && <span className="text-caption text-muted-foreground">{accept.join(", ")}</span>}
      </button>
      <input
        ref={input}
        type="file"
        multiple
        hidden
        accept={accept.join(",") || undefined}
        onChange={(event) => {
          add(Array.from(event.target.files ?? []));
          event.target.value = "";
        }}
      />
      {ids.map((id, index) => {
        // A file of the answer being edited comes as an id only; its place in the list names it.
        const upload = uploads.find((u) => isUploaded(u) && u.attachment.id === id);
        const name = upload?.filename ?? t(($) => $.controls.file_earlier, { number: index + 1 });
        return (
          <div key={id} className="flex h-10 items-center gap-2 rounded-lg border border-border px-3 text-body">
            <FileIcon aria-hidden className="size-4 shrink-0 text-muted-foreground" />
            <span className="min-w-0 truncate">{name}</span>
            {upload && <span className="text-caption text-muted-foreground">{formatBytes(upload.size)}</span>}
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              disabled={disabled}
              className="ml-auto text-muted-foreground"
              aria-label={t(($) => $.controls.file_remove, { name })}
              onClick={() => {
                onDraftChange({ attachment_ids: ids.filter((other) => other !== id) });
                if (upload) removeUpload(draftKey, upload.clientUploadId);
              }}
            >
              <X />
            </Button>
          </div>
        );
      })}
    </div>
  );
}
