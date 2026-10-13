import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import { createWorkspaceAwareStorage, registerForWorkspaceRehydration } from "../platform/workspace-storage";
import { defaultStorage } from "../platform/storage";
import { registerDraftCleanup } from "../drafts/cleanup-registry";
import { attachmentToDraftUpload, normalizeStoredUploads, type DraftUpload } from "../drafts/draft-upload";
import type { Attachment } from "../types";

/**
 * Unsent work on a question card, kept outside the card so it survives the
 * issue timeline's virtualization: a card scrolled out of the viewport
 * unmounts, and a draft in its `useState` would be lost. Like comment drafts,
 * it is persisted per workspace, so a reload or tab close keeps it too.
 */
export interface QuestionDraft {
  /** The answer being composed, in the answer control's draft shape; `null` while nothing is picked. */
  answer: unknown;
  note: string;
  noteOpen: boolean;
  askBackOpen: boolean;
  askBackText: string;
  /** True while the viewer edits their sent answer, rather than composing a first one. */
  editing: boolean;
  /**
   * The answer's files picked on this question (File and Text): a placeholder
   * while the upload coordinator runs it, then the attachment with its name and
   * size. A card that remounts still waits for, and names, every one of them.
   */
  uploads: DraftUpload[];
  updatedAt: number;
}

export type QuestionDraftPatch = Partial<Omit<QuestionDraft, "updatedAt">>;

/** Drafts are keyed by workspace and question. */
export function questionDraftKey(wsId: string, questionId: string): string {
  return `${wsId}:${questionId}`;
}

const NO_UPLOADS: DraftUpload[] = [];

export const EMPTY_QUESTION_DRAFT: QuestionDraft = Object.freeze({
  answer: null,
  note: "",
  noteOpen: false,
  askBackOpen: false,
  askBackText: "",
  editing: false,
  uploads: NO_UPLOADS,
  updatedAt: 0,
});

interface QuestionDraftStore {
  drafts: Record<string, QuestionDraft>;
  /** Merges the patch into the question's draft; a draft back at empty is dropped. */
  setDraft: (key: string, patch: QuestionDraftPatch) => void;
  clearDraft: (key: string) => void;
  /** Records a file the moment it is picked, before its upload settles. */
  addUpload: (key: string, upload: DraftUpload) => void;
  /** Swaps the placeholder for its attachment; nothing once the placeholder is gone. */
  settleUpload: (key: string, clientUploadId: string, attachment: Attachment) => void;
  removeUpload: (key: string, clientUploadId: string) => void;
  /**
   * After an answer is sent: `sent` is the draft at sending time. What still
   * is as it was then is cleared; what was written since is kept.
   */
  clearSentAnswer: (key: string, sent: QuestionDraft) => void;
  /** After an ask back is sent: empties the ask-back box while it still holds the sent `text`. */
  clearSentAskBack: (key: string, text: string) => void;
}

// Drafts older than 30 days are dropped on load, as comment drafts are.
const TTL_MS = 30 * 24 * 60 * 60 * 1000;

// A draft with nothing picked, typed, opened or uploaded holds nothing to keep.
function isEmpty(draft: QuestionDraft): boolean {
  return (
    draft.answer === null && draft.note === "" && !draft.noteOpen && !draft.askBackOpen && draft.askBackText === "" &&
    !draft.editing && draft.uploads.length === 0
  );
}

function without(drafts: Record<string, QuestionDraft>, key: string): Record<string, QuestionDraft> {
  const next = { ...drafts };
  delete next[key];
  return next;
}

function written(drafts: Record<string, QuestionDraft>, key: string, next: QuestionDraft): { drafts: Record<string, QuestionDraft> } {
  if (!isEmpty(next)) return { drafts: { ...drafts, [key]: { ...next, updatedAt: Date.now() } } };
  return { drafts: key in drafts ? without(drafts, key) : drafts };
}

type DurableDraft = Omit<QuestionDraft, "noteOpen" | "askBackOpen">;

// Whether the note or ask back is open is screen state, so storage keeps only
// what was written; a loaded draft opens the boxes that hold text.
function durableDrafts(drafts: Record<string, QuestionDraft>): Record<string, DurableDraft> {
  const out: Record<string, DurableDraft> = {};
  for (const [key, { answer, note, askBackText, editing, uploads, updatedAt }] of Object.entries(drafts)) {
    if (answer === null && note === "" && askBackText === "" && !editing && uploads.length === 0) continue;
    out[key] = { answer, note, askBackText, editing, uploads, updatedAt };
  }
  return out;
}

// An upload still running at load time is dropped: its bytes were never stored.
function loadedDrafts(drafts: Record<string, QuestionDraft>): Record<string, QuestionDraft> {
  const cutoff = Date.now() - TTL_MS;
  const out: Record<string, QuestionDraft> = {};
  for (const [key, draft] of Object.entries(drafts)) {
    if (draft.updatedAt < cutoff) continue;
    const loaded = { ...EMPTY_QUESTION_DRAFT, ...draft, uploads: normalizeStoredUploads(draft.uploads) };
    const opened = { ...loaded, noteOpen: loaded.note !== "", askBackOpen: loaded.askBackText !== "" };
    if (!isEmpty(opened)) out[key] = opened;
  }
  return out;
}

export const useQuestionDraftStore = create<QuestionDraftStore>()(
  persist(
    (set) => ({
      drafts: {},
      setDraft: (key, patch) =>
        set((s) => {
          const next = { ...(s.drafts[key] ?? EMPTY_QUESTION_DRAFT), ...patch };
          if (isEmpty(next) && !(key in s.drafts)) return s;
          return written(s.drafts, key, next);
        }),
      clearDraft: (key) => set((s) => (key in s.drafts ? { drafts: without(s.drafts, key) } : s)),
      addUpload: (key, upload) =>
        set((s) => {
          const draft = s.drafts[key] ?? EMPTY_QUESTION_DRAFT;
          if (draft.uploads.some((u) => u.clientUploadId === upload.clientUploadId)) return s;
          return written(s.drafts, key, { ...draft, uploads: [...draft.uploads, upload] });
        }),
      settleUpload: (key, clientUploadId, attachment) =>
        set((s) => {
          const draft = s.drafts[key];
          if (!draft?.uploads.some((u) => u.clientUploadId === clientUploadId)) return s;
          const uploads = draft.uploads.map((u) =>
            u.clientUploadId === clientUploadId ? { ...attachmentToDraftUpload(attachment), clientUploadId } : u,
          );
          return written(s.drafts, key, { ...draft, uploads });
        }),
      removeUpload: (key, clientUploadId) =>
        set((s) => {
          const draft = s.drafts[key];
          if (!draft?.uploads.some((u) => u.clientUploadId === clientUploadId)) return s;
          return written(s.drafts, key, { ...draft, uploads: draft.uploads.filter((u) => u.clientUploadId !== clientUploadId) });
        }),
      clearSentAnswer: (key, sent) =>
        set((s) => {
          const draft = s.drafts[key];
          if (!draft) return s;
          // Every write replaces the value it changes, so an unchanged value is the one that was sent.
          const answerSent = draft.answer === sent.answer;
          const noteSent = draft.note === sent.note;
          const askBackSent = draft.askBackText === sent.askBackText;
          return written(s.drafts, key, {
            ...draft,
            answer: answerSent ? null : draft.answer,
            // The sent answer ends the edit it was.
            editing: answerSent ? false : draft.editing,
            // The sent files go with the sent answer; a file picked since stays.
            uploads: answerSent ? draft.uploads.filter((u) => !sent.uploads.includes(u)) : draft.uploads,
            note: noteSent ? "" : draft.note,
            noteOpen: noteSent ? false : draft.noteOpen,
            askBackText: askBackSent ? "" : draft.askBackText,
            askBackOpen: askBackSent ? false : draft.askBackOpen,
          });
        }),
      clearSentAskBack: (key, text) =>
        set((s) => {
          const draft = s.drafts[key];
          if (!draft || draft.askBackText.trim() !== text) return s;
          return written(s.drafts, key, { ...draft, askBackText: "", askBackOpen: false });
        }),
    }),
    {
      name: "multica_question_drafts",
      storage: createJSONStorage(() => createWorkspaceAwareStorage(defaultStorage)),
      partialize: (state) => ({ drafts: durableDrafts(state.drafts) }),
      // An empty namespace means no drafts, not the previous workspace's.
      merge: (persisted, current) => ({
        ...current,
        drafts: (persisted as { drafts?: Record<string, QuestionDraft> } | undefined)?.drafts ?? {},
      }),
      onRehydrateStorage: () => (state) => {
        if (state) state.drafts = loadedDrafts(state.drafts);
      },
    },
  ),
);

registerForWorkspaceRehydration(() => useQuestionDraftStore.persist.rehydrate());

registerDraftCleanup({
  storageKey: "multica_question_drafts",
  workspaceScoped: true,
  resetInMemory: () => useQuestionDraftStore.setState({ drafts: {} }),
});
