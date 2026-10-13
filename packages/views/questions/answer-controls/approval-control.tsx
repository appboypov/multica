"use client";

import { ArrowUpRight, Ban, Check, Pencil } from "lucide-react";
import type { ApprovalDecision } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import type { AnswerControlProps } from "./control-props";

/**
 * The link to what is to be approved, then **Approve**, **Request changes**
 * and **Decline**; Request changes opens a textarea for its note.
 */
export function ApprovalControl({ question, draft, onDraftChange, note, onNoteChange, disabled }: AnswerControlProps<"approval">) {
  const { t } = useT("questions");
  const { subject } = question.settings;
  // The asker gives the link; only a web link opens, never a `javascript:` one.
  const link = question.settings.link && /^https?:\/\//i.test(question.settings.link.url) ? question.settings.link : null;
  const decision = draft?.decision ?? null;
  const pick = (next: ApprovalDecision) => onDraftChange({ decision: next });

  return (
    <div className="flex flex-col gap-2">
      {link ? (
        <a
          href={link.url}
          target="_blank"
          rel="noreferrer"
          className="flex h-9 items-center gap-2 rounded-lg bg-muted/60 px-3 text-body hover:bg-muted"
        >
          <span className="min-w-0 truncate font-medium">{subject}</span>
          <span className="min-w-0 flex-1 truncate text-caption text-muted-foreground">{link.title}</span>
          <ArrowUpRight aria-hidden className="size-4 shrink-0 text-muted-foreground" />
        </a>
      ) : (
        <div className="flex h-9 items-center rounded-lg bg-muted/60 px-3 text-body font-medium">
          <span className="min-w-0 truncate">{subject}</span>
        </div>
      )}
      <div className="flex flex-wrap items-center gap-2">
        <Button
          type="button"
          variant={decision === "approve" ? "default" : "outline"}
          aria-pressed={decision === "approve"}
          disabled={disabled}
          onClick={() => pick("approve")}
        >
          <Check />
          {t(($) => $.controls.approve)}
        </Button>
        <Button
          type="button"
          variant={decision === "request_changes" ? "default" : "outline"}
          aria-pressed={decision === "request_changes"}
          disabled={disabled}
          onClick={() => pick("request_changes")}
        >
          <Pencil />
          {t(($) => $.controls.request_changes)}
        </Button>
        <Button
          type="button"
          variant="ghost"
          aria-pressed={decision === "decline"}
          disabled={disabled}
          className={cn("ml-auto text-destructive hover:text-destructive", decision === "decline" && "bg-destructive/10")}
          onClick={() => pick("decline")}
        >
          <Ban />
          {t(($) => $.controls.decline)}
        </Button>
      </div>
      {decision === "request_changes" && (
        <Textarea
          autoFocus
          value={note}
          disabled={disabled}
          aria-label={t(($) => $.controls.changes_label)}
          placeholder={t(($) => $.controls.changes_placeholder)}
          aria-invalid={!note.trim()}
          onChange={(event) => onNoteChange(event.target.value)}
        />
      )}
    </div>
  );
}
