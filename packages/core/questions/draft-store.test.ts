// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import type { Attachment } from "../types";
import { questionDraftKey, useQuestionDraftStore } from "./draft-store";

function attachment(id: string, filename: string): Attachment {
  return { id, filename, url: `https://files.example/${filename}`, download_url: `/api/attachments/${id}/download`, size_bytes: 2048 } as Attachment;
}

function flush(): Promise<void> {
  const { promise, resolve } = Promise.withResolvers<void>();
  queueMicrotask(resolve);
  return promise;
}
const STORAGE_KEY = "multica_question_drafts:acme";
const DAY_MS = 24 * 60 * 60 * 1000;

// Node 25 ships a partial `localStorage` shim under jsdom that's missing
// `clear`/`removeItem`; replace it with a real in-memory Storage so persist
// can round-trip values.
beforeAll(() => {
  if (typeof globalThis.localStorage?.clear !== "function") {
    const values = new Map<string, string>();
    const storage: Storage = {
      get length() { return values.size; },
      clear: () => values.clear(),
      getItem: (k) => values.get(k) ?? null,
      key: (i) => Array.from(values.keys())[i] ?? null,
      removeItem: (k) => { values.delete(k); },
      setItem: (k, v) => { values.set(k, v); },
    };
    Object.defineProperty(globalThis, "localStorage", { configurable: true, value: storage });
    Object.defineProperty(window, "localStorage", { configurable: true, value: storage });
  }
});

beforeEach(() => {
  localStorage.clear();
  setCurrentWorkspace(null, null);
  useQuestionDraftStore.setState({ drafts: {} });
});

afterEach(() => {
  setCurrentWorkspace(null, null);
});

describe("question draft store", () => {
  it("keeps each workspace's draft of a question apart", () => {
    const { setDraft } = useQuestionDraftStore.getState();
    setDraft(questionDraftKey("ws-1", "q-1"), { answer: { option: "Per invoice line" }, note: "Line rounding" });
    setDraft(questionDraftKey("ws-2", "q-1"), { askBackOpen: true, askBackText: "Which invoices?" });

    const { drafts } = useQuestionDraftStore.getState();
    expect(drafts[questionDraftKey("ws-1", "q-1")]).toMatchObject({
      answer: { option: "Per invoice line" },
      note: "Line rounding",
      askBackText: "",
    });
    expect(drafts[questionDraftKey("ws-2", "q-1")]).toMatchObject({ answer: null, askBackOpen: true, askBackText: "Which invoices?" });
  });

  it("merges patches and drops a draft once nothing is left in it", () => {
    const key = questionDraftKey("ws-1", "q-1");
    const { setDraft } = useQuestionDraftStore.getState();
    setDraft(key, { answer: { option: "Per invoice line" } });
    setDraft(key, { note: "Line rounding" });
    expect(useQuestionDraftStore.getState().drafts[key]).toMatchObject({ answer: { option: "Per invoice line" }, note: "Line rounding" });

    setDraft(key, { answer: null, note: "" });
    expect(key in useQuestionDraftStore.getState().drafts).toBe(false);
  });

  it("clears one question's draft and leaves the others", () => {
    const sent = questionDraftKey("ws-1", "q-1");
    const other = questionDraftKey("ws-1", "q-2");
    const { setDraft, clearDraft } = useQuestionDraftStore.getState();
    setDraft(sent, { answer: { choice: "yes" } });
    setDraft(other, { note: "Still thinking" });

    clearDraft(sent);
    expect(Object.keys(useQuestionDraftStore.getState().drafts)).toEqual([other]);
  });

  it("persists drafts under the workspace and drops stale ones on load", async () => {
    setCurrentWorkspace("acme", "ws-1");
    await flush();
    await flush();
    const key = questionDraftKey("ws-1", "q-1");
    useQuestionDraftStore.getState().setDraft(key, { note: "Line rounding" });
    expect(JSON.parse(localStorage.getItem(STORAGE_KEY)!).state.drafts[key].note).toBe("Line rounding");

    const saved = JSON.parse(localStorage.getItem(STORAGE_KEY)!);
    saved.state.drafts[questionDraftKey("ws-1", "q-old")] = { ...saved.state.drafts[key], updatedAt: Date.now() - 31 * DAY_MS };
    localStorage.setItem(STORAGE_KEY, JSON.stringify(saved));
    // Another workspace has none of acme's drafts; coming back loads them, without the stale one.
    setCurrentWorkspace("globex", "ws-2");
    await flush();
    await flush();
    expect(useQuestionDraftStore.getState().drafts).toEqual({});
    setCurrentWorkspace("acme", "ws-1");
    await flush();
    await flush();

    expect(Object.keys(useQuestionDraftStore.getState().drafts)).toEqual([key]);
    expect(useQuestionDraftStore.getState().drafts[key]?.note).toBe("Line rounding");
  });

  it("stores what was written, not which box is open, and opens the boxes that hold text on load", async () => {
    setCurrentWorkspace("acme", "ws-1");
    await flush();
    await flush();
    const opened = questionDraftKey("ws-1", "q-1");
    const written = questionDraftKey("ws-1", "q-2");
    const { setDraft } = useQuestionDraftStore.getState();
    setDraft(opened, { noteOpen: true });
    setDraft(written, { askBackOpen: true, askBackText: "Which invoices?" });
    // The opened box still shows while the workspace stays loaded.
    expect(useQuestionDraftStore.getState().drafts[opened]?.noteOpen).toBe(true);

    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY)!).state.drafts;
    expect(Object.keys(stored)).toEqual([written]);
    expect(stored[written]).not.toHaveProperty("askBackOpen");
    expect(stored[written]).not.toHaveProperty("noteOpen");

    setCurrentWorkspace("globex", "ws-2");
    await flush();
    await flush();
    setCurrentWorkspace("acme", "ws-1");
    await flush();
    await flush();
    expect(Object.keys(useQuestionDraftStore.getState().drafts)).toEqual([written]);
    expect(useQuestionDraftStore.getState().drafts[written]).toMatchObject({ askBackOpen: true, askBackText: "Which invoices?", noteOpen: false });
  });

  it("swaps a placeholder for its attachment with name and size, and not once the placeholder is gone", () => {
    const key = questionDraftKey("ws-1", "q-1");
    const { addUpload, settleUpload, removeUpload } = useQuestionDraftStore.getState();
    addUpload(key, { clientUploadId: "u-1", status: "uploading", filename: "a.pdf", size: 2048 });
    addUpload(key, { clientUploadId: "u-2", status: "uploading", filename: "b.pdf", size: 2048 });
    settleUpload(key, "u-1", attachment("att-a", "a.pdf"));
    removeUpload(key, "u-2");
    // A late settle of a removed file writes nothing.
    settleUpload(key, "u-2", attachment("att-b", "b.pdf"));

    expect(useQuestionDraftStore.getState().drafts[key]?.uploads).toEqual([
      expect.objectContaining({ clientUploadId: "u-1", status: "uploaded", filename: "a.pdf", size: 2048 }),
    ]);
  });

  it("clears what was sent and keeps what was written since", () => {
    const key = questionDraftKey("ws-1", "q-1");
    const { setDraft, addUpload, clearSentAnswer } = useQuestionDraftStore.getState();
    setDraft(key, { answer: { attachment_ids: ["att-a"] }, note: "Signed copy", noteOpen: true });
    addUpload(key, { clientUploadId: "u-1", status: "uploading", filename: "a.pdf", size: 2048 });
    const sent = useQuestionDraftStore.getState().drafts[key]!;
    setDraft(key, { askBackOpen: true, askBackText: "Which invoices?" });

    clearSentAnswer(key, sent);
    expect(useQuestionDraftStore.getState().drafts[key]).toMatchObject({
      answer: null, note: "", noteOpen: false, uploads: [], askBackOpen: true, askBackText: "Which invoices?",
    });
    clearSentAnswer(key, useQuestionDraftStore.getState().drafts[key]!);
    expect(key in useQuestionDraftStore.getState().drafts).toBe(false);
  });

  it("stores the answer's uploaded files, and drops one still uploading on load", async () => {
    setCurrentWorkspace("acme", "ws-1");
    await flush();
    await flush();
    const key = questionDraftKey("ws-1", "q-1");
    const { addUpload, settleUpload } = useQuestionDraftStore.getState();
    addUpload(key, { clientUploadId: "u-1", status: "uploading", filename: "a.pdf", size: 2048 });
    settleUpload(key, "u-1", attachment("att-a", "a.pdf"));
    addUpload(key, { clientUploadId: "u-2", status: "uploading", filename: "b.pdf", size: 2048 });

    setCurrentWorkspace("globex", "ws-2");
    await flush();
    await flush();
    setCurrentWorkspace("acme", "ws-1");
    await flush();
    await flush();
    expect(useQuestionDraftStore.getState().drafts[key]?.uploads).toEqual([
      expect.objectContaining({ clientUploadId: "u-1", status: "uploaded", filename: "a.pdf" }),
    ]);
  });
});
