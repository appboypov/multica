"use client";

import { useLayoutEffect, useRef, useState } from "react";
import { FolderOpen, X } from "lucide-react";
import {
  appendSelection,
  insertSelectionPlaceholder,
  SELECTION_PLACEHOLDER,
  selectionWarning,
} from "@multica/core/agents";
import type { RuntimeDevice } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@multica/ui/components/ui/tooltip";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { RuntimeFileBrowser } from "./runtime-file-browser";

/**
 * A home folder at the start of an absolute path: macOS, Linux or Windows.
 * Chips show the path after it as `~/...`.
 */
const HOME_PREFIX = /^(?:\/Users\/[^/]+|\/home\/[^/]+|[A-Za-z]:[\\/]Users[\\/][^\\/]+)(?=[\\/]|$)/;

export interface InstructionsFieldProps {
  id: string;
  value: string;
  onChange: (value: string) => void;
  selection: string[];
  onSelectionChange: (selection: string[]) => void;
  runtime: RuntimeDevice | null;
  currentUserId: string | null;
  placeholder?: string;
  disabled?: boolean;
  rows?: number;
  className?: string;
}

/**
 * The agent instructions text with the files and folders selected on the
 * agent's runtime. Only the runtime's owner browses and removes paths;
 * anyone else sees them read-only. The first selection inserts the
 * `<SystemIdentity>` block with `{selection}` at the cursor so the daemon
 * loads the paths there.
 */
export function InstructionsField({
  id,
  value,
  onChange,
  selection,
  onSelectionChange,
  runtime,
  currentUserId,
  placeholder,
  disabled = false,
  rows,
  className,
}: InstructionsFieldProps) {
  const { t } = useT("agents");
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const hadFocusRef = useRef(false);
  const pendingCursorRef = useRef<number | null>(null);
  const [browseOpen, setBrowseOpen] = useState(false);
  const canSelect =
    runtime !== null &&
    currentUserId !== null &&
    runtime.owner_id === currentUserId;
  const warning = selectionWarning(value, selection);

  // The inserted placeholder arrives through `value`; place the cursor after
  // it once the textarea holds the new text.
  useLayoutEffect(() => {
    const cursor = pendingCursorRef.current;
    const textarea = textareaRef.current;
    if (cursor === null || !textarea) return;
    pendingCursorRef.current = null;
    textarea.setSelectionRange(cursor, cursor);
  }, [value]);

  const handleAdd = (paths: string[]) => {
    if (selection.length === 0 && !value.includes(SELECTION_PLACEHOLDER)) {
      const inserted = insertSelectionPlaceholder(
        value,
        hadFocusRef.current
          ? (textareaRef.current?.selectionStart ?? null)
          : null,
      );
      pendingCursorRef.current = inserted.cursor;
      hadFocusRef.current = true;
      onChange(inserted.text);
    }
    onSelectionChange(appendSelection(selection, paths));
  };

  return (
    <div className="space-y-2">
      <Textarea
        ref={textareaRef}
        id={id}
        autoComplete="off"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        onFocus={() => {
          hadFocusRef.current = true;
        }}
        placeholder={placeholder}
        disabled={disabled}
        rows={rows}
        className={cn("resize-y leading-6", className)}
      />

      {(selection.length > 0 || canSelect) && (
        <div className="flex flex-wrap items-center gap-1.5">
          {selection.length > 0 && (
            <ul
              aria-label={t(($) => $.instructions.selection.list)}
              className="contents"
            >
              {selection.map((path) => (
                <li
                  key={path}
                  className="flex max-w-full items-center gap-1 rounded-md border bg-muted/40 py-0.5 pr-1 pl-2 text-caption"
                >
                  <Tooltip>
                    <TooltipTrigger
                      render={<span className="min-w-0 truncate font-mono" />}
                    >
                      {path.replace(HOME_PREFIX, "~")}
                    </TooltipTrigger>
                    <TooltipContent>{path}</TooltipContent>
                  </Tooltip>
                  {canSelect && (
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-xs"
                      aria-label={t(($) => $.instructions.selection.remove, {
                        path,
                      })}
                      disabled={disabled}
                      onClick={() =>
                        onSelectionChange(selection.filter((p) => p !== path))
                      }
                    >
                      <X />
                    </Button>
                  )}
                </li>
              ))}
            </ul>
          )}
          {canSelect && (
            <Button
              type="button"
              variant="outline"
              size="xs"
              disabled={disabled}
              onClick={() => setBrowseOpen(true)}
            >
              <FolderOpen />
              {t(($) => $.instructions.selection.browse)}
            </Button>
          )}
        </div>
      )}

      {warning && (
        <p className="text-caption text-warning">
          {warning === "not_loaded"
            ? t(($) => $.instructions.selection.warning_not_loaded)
            : t(($) => $.instructions.selection.warning_empty_placeholder)}
        </p>
      )}

      {canSelect && (
        <RuntimeFileBrowser
          open={browseOpen}
          onOpenChange={setBrowseOpen}
          runtime={runtime}
          onAdd={handleAdd}
        />
      )}
    </div>
  );
}
