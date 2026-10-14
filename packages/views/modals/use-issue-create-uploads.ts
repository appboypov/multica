"use client";

/**
 * Issue-create binding for the coordinated-upload engine (MUL-5181, L2).
 *
 * Both create panels (manual form and agent prompt) share ONE upload pool —
 * `draft.shared.attachments` — so a file survives a mode switch from either
 * side. What differs per mode is the BODY the upload's link belongs to. The
 * agent prompt holds its links as written, and so does the manual form's free
 * description with No template. With a template the manual form has no free
 * body: its fields fill the description, and the links of the finished
 * uploads follow that Markdown (`manualAttachmentMarkdown`), so that body is
 * derived from the pool and nothing is appended to it.
 *
 * Uploads here carry no issue/comment context — the issue does not exist yet;
 * the server binds attachments at create time from `attachment_ids`.
 */

import { useMemo, type RefObject } from "react";
import type { Attachment } from "@multica/core/types";
import { attachmentToDraftUpload, type DraftUpload } from "@multica/core/drafts";
import { useIssueDraftStore } from "@multica/core/issues/stores";
import type { CreateMode } from "@multica/core/issues/stores";
import type { UploadGate } from "../editor/use-upload-gate";
import type { ContentEditorRef } from "../editor/content-editor";
import {
  attachmentMarkdown,
  useCoordinatedUploads,
  type CoordinatedUploads,
  type UploadDraftBinding,
} from "../editor/use-coordinated-uploads";

function appendMarkdown(existing: string, markdown: string): string {
  return existing.trim() ? `${existing.replace(/\s+$/, "")}\n\n${markdown}` : markdown;
}

function sharedUploads(): DraftUpload[] {
  return useIssueDraftStore.getState().draft.shared.attachments ?? [];
}

function setSharedUploads(next: DraftUpload[]): void {
  useIssueDraftStore.getState().setShared({ attachments: next });
}

/** The finished uploads of the pool, which the manual form's issue keeps. */
export function uploadedAttachments(uploads: readonly DraftUpload[]): Attachment[] {
  const done: Attachment[] = [];
  for (const u of uploads) {
    if (u.status === "uploaded") done.push(u.attachment);
  }
  return done;
}

/** The links of the finished uploads, as they follow the template's Markdown. */
export function manualAttachmentMarkdown(uploads: readonly DraftUpload[]): string {
  return uploadedAttachments(uploads).map(attachmentMarkdown).join("\n\n");
}

export function useIssueCreateUploads(
  mode: CreateMode,
  editorGate: UploadGate,
  editorRef: RefObject<ContentEditorRef | null>,
): CoordinatedUploads {
  const uploads = useIssueDraftStore((s) => s.draft.shared.attachments);

  const binding = useMemo<UploadDraftBinding>(
    () => ({
      registryKey: `issue-create:${mode}`,
      getUploads: sharedUploads,
      addUpload: (u) => {
        const cur = sharedUploads();
        if (cur.some((x) => x.clientUploadId === u.clientUploadId)) return;
        setSharedUploads([...cur, u]);
      },
      settleUpload: (id, att) => {
        const cur = sharedUploads();
        if (!cur.some((u) => u.clientUploadId === id)) return;
        // attachmentToDraftUpload strips the response-scoped signed
        // download_url before the row is persisted.
        setSharedUploads(
          cur.map((u) =>
            u.clientUploadId === id ? { ...attachmentToDraftUpload(att), clientUploadId: id } : u,
          ),
        );
      },
      failUpload: (id, error) => {
        const cur = sharedUploads();
        const target = cur.find((u) => u.clientUploadId === id);
        if (!target) return;
        setSharedUploads(
          cur.map((u) =>
            u.clientUploadId === id
              ? {
                  clientUploadId: id,
                  status: "failed",
                  filename: target.filename,
                  size: target.size,
                  contentType: target.contentType,
                  error,
                }
              : u,
          ),
        );
      },
      removeUpload: (id) => {
        const cur = sharedUploads();
        if (!cur.some((u) => u.clientUploadId === id)) return;
        setSharedUploads(cur.filter((u) => u.clientUploadId !== id));
      },
      getBody: () => {
        const { draft } = useIssueDraftStore.getState();
        if (mode === "agent") return draft.agent.prompt;
        return draft.manual.type === null
          ? draft.manual.description
          : manualAttachmentMarkdown(sharedUploads());
      },
      appendToBody: (md) => {
        const state = useIssueDraftStore.getState();
        if (mode === "agent") {
          state.setAgent({ prompt: appendMarkdown(state.draft.agent.prompt, md) });
          return;
        }
        // With a template the body is derived from the pool; a settled upload is in it.
        if (state.draft.manual.type !== null) return;
        state.setManual({ description: appendMarkdown(state.draft.manual.description, md) });
      },
    }),
    [mode],
  );

  return useCoordinatedUploads(binding, uploads, {}, editorGate, editorRef);
}
