"use client";

import { useImperativeHandle, useRef, useState, type ClipboardEvent, type DragEvent } from "react";
import { CloudUpload, File as FileIcon, Loader2, X } from "lucide-react";
import { toast } from "sonner";
import { createLogger } from "@multica/core/logger";
import { Button } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { formatBytes } from "../../common/format-bytes";
import { useT } from "../../i18n";
import type { AnswerControlProps } from "./control-props";

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

interface UploadedFile {
  id: string;
  name: string;
  size: number;
}

/**
 * A drop zone and one row per file; a pasted screenshot counts (`⌘V`). Each
 * file uploads onto the question as it lands, and the answer is their ids.
 */
export function FileControl({
  question,
  draft,
  onDraftChange,
  uploadFile,
  handle,
  onUploadingChange,
  disabled,
}: AnswerControlProps<"file">) {
  const { t } = useT("questions");
  const { accept } = question.settings;
  const input = useRef<HTMLInputElement>(null);
  const [files, setFiles] = useState<UploadedFile[]>([]);
  // Files picked and not settled yet; the ref is what Send reads at sending time.
  const [uploading, setUploading] = useState(0);
  const uploadingRef = useRef(0);
  const [dragging, setDragging] = useState(false);
  const ids = draft?.attachment_ids ?? [];
  // The draft is read again after each upload, so files landing together all count.
  const idsRef = useRef(ids);
  idsRef.current = ids;

  useImperativeHandle(handle, () => ({ isUploading: () => uploadingRef.current > 0 }));

  const setIds = (next: string[]) => {
    idsRef.current = next;
    onDraftChange({ attachment_ids: next });
  };

  const track = (delta: number) => {
    uploadingRef.current += delta;
    setUploading(uploadingRef.current);
    onUploadingChange(uploadingRef.current > 0);
  };

  const add = async (picked: File[]) => {
    const accepted = picked.filter((file) => {
      if (acceptsFile(accept, file)) return true;
      toast.error(t(($) => $.controls.file_refused, { name: file.name }));
      return false;
    });
    if (accepted.length === 0) return;
    // Every accepted file counts from the start, so Send waits for the last of them.
    track(accepted.length);
    for (const file of accepted) {
      try {
        const attachment = await uploadFile(file);
        setFiles((current) => [...current, { id: attachment.id, name: attachment.filename, size: attachment.size_bytes }]);
        setIds([...idsRef.current, attachment.id]);
      } catch (error) {
        logger.error("upload failed", { questionId: question.id, error });
        toast.error(t(($) => $.controls.file_failed, { name: file.name }));
      } finally {
        track(-1);
      }
    }
  };

  const onDrop = (event: DragEvent) => {
    event.preventDefault();
    setDragging(false);
    if (!disabled) void add(Array.from(event.dataTransfer.files));
  };

  const onPaste = (event: ClipboardEvent) => {
    const pasted = Array.from(event.clipboardData.files);
    if (disabled || pasted.length === 0) return;
    event.preventDefault();
    void add(pasted);
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
        {uploading > 0 ? (
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
          void add(Array.from(event.target.files ?? []));
          event.target.value = "";
        }}
      />
      {ids.map((id) => {
        const file = files.find((f) => f.id === id);
        return (
          <div key={id} className="flex h-10 items-center gap-2 rounded-lg border border-border px-3 text-body">
            <FileIcon aria-hidden className="size-4 shrink-0 text-muted-foreground" />
            <span className="min-w-0 truncate">{file?.name ?? t(($) => $.controls.file_earlier)}</span>
            {file && <span className="text-caption text-muted-foreground">{formatBytes(file.size)}</span>}
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              disabled={disabled}
              className="ml-auto text-muted-foreground"
              aria-label={t(($) => $.controls.file_remove, { name: file?.name ?? "" })}
              onClick={() => setIds(ids.filter((other) => other !== id))}
            >
              <X />
            </Button>
          </div>
        );
      })}
    </div>
  );
}
