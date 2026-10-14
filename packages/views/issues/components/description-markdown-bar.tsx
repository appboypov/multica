"use client";

import { useEffect, useState } from "react";
import { Check, Copy } from "lucide-react";
import { copyText } from "@multica/ui/lib/clipboard";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";

interface DescriptionMarkdownBarProps {
  /** Whether the Markdown shows in place of the normal view. */
  markdown: boolean;
  onMarkdownChange: (markdown: boolean) => void;
  /** Read at click time, so the copy holds what was typed a moment ago. */
  getMarkdown: () => string;
  className?: string;
}

/**
 * The row above a description: the Markdown toggle and a button that copies
 * the Markdown, whichever view shows. Used by the issue page and New issue.
 */
export function DescriptionMarkdownBar({
  markdown,
  onMarkdownChange,
  getMarkdown,
  className,
}: DescriptionMarkdownBarProps) {
  const { t } = useT("issues");
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(timer);
  }, [copied]);

  const handleCopy = () => {
    void copyText(getMarkdown()).then((ok) => {
      if (ok) setCopied(true);
    });
  };

  return (
    <div className={cn("flex items-center gap-1", className)}>
      <button
        type="button"
        aria-pressed={markdown}
        onClick={() => onMarkdownChange(!markdown)}
        className={cn(
          "rounded-md border px-2 py-0.5 text-caption transition-colors",
          markdown ? "bg-accent text-foreground" : "text-muted-foreground hover:text-foreground",
        )}
      >
        {t(($) => $.detail.description_markdown)}
      </button>
      <button
        type="button"
        onClick={handleCopy}
        aria-label={t(($) => $.detail.description_copy_markdown)}
        className="shrink-0 rounded-md p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        {copied ? (
          <Check className="size-3.5 text-success" aria-hidden />
        ) : (
          <Copy className="size-3.5" aria-hidden />
        )}
      </button>
    </div>
  );
}
