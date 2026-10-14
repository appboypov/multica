"use client";

import { AlertCircle, File as FileIcon, Loader2, X } from "lucide-react";
import type { DraftUpload } from "@multica/core/drafts";
import { Button } from "@multica/ui/components/ui/button";
import { formatBytes } from "../../../common/format-bytes";
import { useT } from "../../../i18n";

interface TemplateAttachmentsProps {
  uploads: readonly DraftUpload[];
  onRemove: (clientUploadId: string) => void;
}

/**
 * New issue's files under the template's fields: one row per upload, as it
 * uploads and once it is done. The issue's Markdown links each finished file
 * after the template's sections.
 */
export function TemplateAttachments({ uploads, onRemove }: TemplateAttachmentsProps) {
  const { t } = useT("editor");
  if (uploads.length === 0) return null;
  return (
    <ul className="flex flex-col gap-1.5">
      {uploads.map((upload) => {
        const name = upload.filename;
        const label =
          upload.status === "uploading"
            ? t(($) => $.upload.uploading_label, { filename: name })
            : upload.status === "uploaded"
              ? name
              : t(($) => $.upload.failed_label, { filename: name });
        return (
          <li
            key={upload.clientUploadId}
            className="flex h-9 items-center gap-2 rounded-lg border border-border px-3 text-body"
          >
            {upload.status === "uploading" ? (
              <Loader2
                aria-hidden
                className="size-4 shrink-0 animate-spin text-muted-foreground motion-reduce:animate-none"
              />
            ) : upload.status === "uploaded" ? (
              <FileIcon aria-hidden className="size-4 shrink-0 text-muted-foreground" />
            ) : (
              <AlertCircle aria-hidden className="size-4 shrink-0 text-destructive" />
            )}
            <span className="min-w-0 truncate">{label}</span>
            <span className="text-caption text-muted-foreground">{formatBytes(upload.size)}</span>
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              className="ml-auto text-muted-foreground"
              aria-label={`${t(($) => $.upload.remove)} ${name}`}
              onClick={() => onRemove(upload.clientUploadId)}
            >
              <X />
            </Button>
          </li>
        );
      })}
    </ul>
  );
}
