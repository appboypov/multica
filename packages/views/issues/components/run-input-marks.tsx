"use client";

import type { CommentSupplementReceipt, RunInputMark } from "@multica/core/types";
import { useActorName } from "@multica/core/workspace/hooks";
import { Tooltip, TooltipContent, TooltipTrigger } from "@multica/ui/components/ui/tooltip";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";

/** One receiving run's marks; `edit` marks belong to an edited version of a comment. */
export interface RunInputMarkView extends RunInputMark {
  edit?: boolean;
}

/** A comment's marks from its receipts: the original text's and each edit's, per run. */
export function commentRunInputMarks(receipts: readonly CommentSupplementReceipt[]): RunInputMarkView[] {
  return receipts.flatMap((receipt) => receipt.agent_id
    ? [{
      task_id: receipt.task_id,
      agent_id: receipt.agent_id,
      seen_at: receipt.seen_at,
      handled_at: receipt.handled_at,
      edit: receipt.item_kind === "comment_edit",
    }]
    : []);
}

type MarkKind = "seen" | "handled";

const EMOJI: Record<MarkKind, string> = { seen: "👀", handled: "✅" };

/**
 * 👀 and ✅ of the agents' runs that received an item, one chip per run and
 * mark with the agent's name. They are the agents' acknowledgements, not
 * people's reactions, so they cannot be toggled. A run that received several
 * edits shows each mark once for the edits.
 */
export function RunInputMarks({ marks, className }: { marks: readonly RunInputMarkView[] | undefined; className?: string }) {
  const { t } = useT("issues");
  const { getActorName } = useActorName();
  const chips: { key: string; kind: MarkKind; name: string; edit: boolean }[] = [];
  const seen = new Set<string>();
  for (const mark of marks ?? []) {
    const edit = mark.edit === true;
    for (const kind of ["seen", "handled"] as const) {
      if (!(kind === "seen" ? mark.seen_at : mark.handled_at)) continue;
      const key = `${mark.task_id}:${edit ? "edit" : "item"}:${kind}`;
      if (seen.has(key)) continue;
      seen.add(key);
      chips.push({ key, kind, name: getActorName("agent", mark.agent_id), edit });
    }
  }
  if (chips.length === 0) return null;

  return (
    <ul aria-label={t(($) => $.run_input_marks.label)} className={cn("flex flex-wrap items-center gap-1.5", className)}>
      {chips.map((chip) => {
        const description = chip.kind === "seen"
          ? chip.edit
            ? t(($) => $.run_input_marks.seen_edit, { name: chip.name })
            : t(($) => $.run_input_marks.seen, { name: chip.name })
          : chip.edit
            ? t(($) => $.run_input_marks.handled_edit, { name: chip.name })
            : t(($) => $.run_input_marks.handled, { name: chip.name });
        return (
          <li key={chip.key} className="min-w-0">
            <Tooltip>
              <TooltipTrigger
                render={
                  <span className="inline-flex min-w-0 max-w-48 cursor-default items-center gap-1 rounded-full border border-border bg-muted/40 px-2 py-0.5 text-caption text-muted-foreground">
                    <span aria-hidden>{EMOJI[chip.kind]}</span>
                    <span aria-hidden className="truncate">{chip.name}</span>
                    {chip.edit && <span aria-hidden className="shrink-0">· {t(($) => $.run_input_marks.edit)}</span>}
                    <span className="sr-only">{description}</span>
                  </span>
                }
              />
              <TooltipContent side="top">{description}</TooltipContent>
            </Tooltip>
          </li>
        );
      })}
    </ul>
  );
}
