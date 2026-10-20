"use client";

import { forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState } from "react";
import { Textarea } from "@multica/ui/components/ui/textarea";

export interface DescriptionMarkdownEditorRef {
  /** The Markdown in the textarea, saved or not. */
  getMarkdown: () => string;
  /** Adds Markdown (an uploaded file's link) as its own paragraph and saves it like typing. */
  append: (markdown: string) => void;
}

interface DescriptionMarkdownEditorProps {
  /** The Markdown the textarea opens with. */
  initialMarkdown: string;
  /** The stored description. A change replaces the text while nothing typed is unsaved. */
  value: string;
  /** Saves the typed Markdown: after `debounceMs` without typing, on blur and on unmount. */
  onSave: (markdown: string) => void;
  ariaLabel: string;
  debounceMs?: number;
}

/** The issue description as editable Markdown. */
export const DescriptionMarkdownEditor = forwardRef<
  DescriptionMarkdownEditorRef,
  DescriptionMarkdownEditorProps
>(function DescriptionMarkdownEditor(
  { initialMarkdown, value, onSave, ariaLabel, debounceMs = 1500 },
  ref,
) {
  const [markdown, setMarkdown] = useState(initialMarkdown);
  const markdownRef = useRef(initialMarkdown);
  const baseRef = useRef(initialMarkdown);
  const dirtyRef = useRef(false);
  const timerRef = useRef<number | undefined>(undefined);
  const lastValueRef = useRef(value);
  // The unmount flush runs after the last render, so it reads the save of
  // that render — the issue this textarea was opened for.
  const onSaveRef = useRef(onSave);
  onSaveRef.current = onSave;

  const flush = useCallback(() => {
    window.clearTimeout(timerRef.current);
    timerRef.current = undefined;
    if (!dirtyRef.current) return;
    dirtyRef.current = false;
    // Trimmed like the rich editor's saves, so both views store the same text.
    const next = markdownRef.current.trim();
    if (next === baseRef.current.trim()) return;
    baseRef.current = next;
    onSaveRef.current(next);
  }, []);

  const update = useCallback(
    (next: string) => {
      markdownRef.current = next;
      setMarkdown(next);
      dirtyRef.current = true;
      window.clearTimeout(timerRef.current);
      timerRef.current = window.setTimeout(flush, debounceMs);
    },
    [debounceMs, flush],
  );

  useEffect(() => flush, [flush]);

  // Someone else's save lands while nothing typed here is unsaved.
  useEffect(() => {
    if (value === lastValueRef.current) return;
    lastValueRef.current = value;
    if (dirtyRef.current) return;
    baseRef.current = value;
    if (value.trim() === markdownRef.current.trim()) return;
    markdownRef.current = value;
    setMarkdown(value);
  }, [value]);

  useImperativeHandle(
    ref,
    () => ({
      getMarkdown: () => markdownRef.current,
      append: (fragment: string) => {
        const current = markdownRef.current.trimEnd();
        update(current ? `${current}\n\n${fragment}` : fragment);
      },
    }),
    [update],
  );

  return (
    <Textarea
      value={markdown}
      onChange={(event) => update(event.target.value)}
      onBlur={flush}
      aria-label={ariaLabel}
      spellCheck={false}
      className="resize-none font-mono text-caption md:text-caption"
    />
  );
});
