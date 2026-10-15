"use client";

import { useState, type KeyboardEvent } from "react";
import { ArrowRight, Check, Send, X } from "lucide-react";
import { toast } from "sonner";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { createLogger } from "@multica/core/logger";
import { questionAsker, useAnswerQuestion, viaLabel } from "@multica/core/questions";
import type { QuestionRow, YesNoAnswerValue } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@multica/ui/components/ui/popover";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { ActorAvatar } from "../common/actor-avatar";
import { useT, useTimeAgo } from "../i18n";
import { QuestionBadges } from "./question-badges";
import { useLinkTitle } from "./row-link";
import { LinkIcon, TYPE_ICONS } from "./type-icon";

const logger = createLogger("questions.row");

export interface QuestionListRowProps {
  row: QuestionRow;
  /** Opens the question where its card is: Enter or a click on the row. */
  onOpen: (row: QuestionRow) => void;
}

function isOpenRow(row: QuestionRow): boolean {
  return row.status === "open" || row.status === "partly_answered";
}

/**
 * One question of the Questions page: type glyph, prompt, badges, asker and
 * age, or "prompt → answer · when" once the viewer answered. A question that
 * belongs to a project, agent, squad, autopilot or skill names that object, and
 * a question that reached the viewer's work from below reads how ("via
 * sub-issue WORK-415"). An open Yes / No question of the viewer answers in
 * place: Yes and No on hover or focus, `Y` and `N` on the focused row, and a
 * note popover when No needs a why.
 */
export function QuestionListRow({ row, onOpen }: QuestionListRowProps) {
  const { t } = useT("questions");
  const timeAgo = useTimeAgo();
  const userId = useAuthStore((s) => s.user?.id);
  const wsId = useWorkspaceId();
  const answer = useAnswerQuestion(
    wsId,
    row.open_issue ? { issueId: row.open_issue.id } : { chatSessionId: row.link.id },
    row.id,
  );
  const [noteOpen, setNoteOpen] = useState(false);
  const [note, setNote] = useState("");

  const open = isOpenRow(row);
  const yesNo = row.answer_type === "yes_no" ? row : null;
  const answersInPlace =
    yesNo !== null && open && row.my_answer === null && !!userId && row.answerer_ids.includes(userId);
  const pending = answer.isPending;
  const TypeIcon = TYPE_ICONS[row.answer_type];
  const via = viaLabel(row.via, null, (key, params) => t(($) => $.via[key], params));
  const asker = questionAsker(row);

  const send = (choice: YesNoAnswerValue["choice"], why?: string) => {
    if (!answersInPlace || pending) return;
    answer.mutate(
      { value: { choice }, ...(why ? { note: why } : {}) },
      {
        onSuccess: () => {
          setNoteOpen(false);
          setNote("");
        },
        onError: (error) => {
          logger.error("row answer failed", { questionId: row.id, choice, error });
          toast.error(t(($) => $.card.answer_failed), { description: error.message });
        },
      },
    );
  };

  const whyOnNo = yesNo?.settings.why_on_no ?? false;
  const answerNo = () => {
    if (whyOnNo) setNoteOpen(true);
    else send("no");
  };

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    // Only the row itself: keys typed in its popover bubble here through the portal.
    if (event.target !== event.currentTarget || event.metaKey || event.ctrlKey || event.altKey) return;
    if (event.key === "Enter") {
      event.preventDefault();
      onOpen(row);
    } else if (answersInPlace && (event.key === "y" || event.key === "Y")) {
      event.preventDefault();
      send("yes");
    } else if (answersInPlace && (event.key === "n" || event.key === "N")) {
      event.preventDefault();
      answerNo();
    }
  };

  return (
    <div
      role="listitem"
      tabIndex={0}
      data-question-row={row.id}
      aria-busy={pending || undefined}
      className={cn(
        "group/row flex h-9 cursor-pointer items-center gap-2 px-4 text-body outline-none hover:bg-accent/50 focus-visible:bg-accent",
        pending && "opacity-60",
      )}
      onClick={() => onOpen(row)}
      onKeyDown={onKeyDown}
    >
      <TypeIcon aria-hidden className="size-3.5 shrink-0 text-muted-foreground" />
      {row.my_answer ? (
        <>
          <span className="min-w-0 max-w-max grow basis-0 truncate text-muted-foreground" title={row.prompt}>{row.prompt}</span>
          <ArrowRight aria-hidden className="size-3.5 shrink-0 text-muted-foreground" />
          <span className="min-w-0 truncate font-medium">{row.my_answer.display}</span>
          <span className="shrink-0 text-caption text-muted-foreground">· {timeAgo(row.my_answer.updated_at)}</span>
        </>
      ) : (
        <span className="min-w-0 truncate" title={row.prompt}>{row.prompt}</span>
      )}
      {row.link.type !== "issue" && <LinkObject link={row.link} />}
      {via && <span className="shrink-0 text-caption text-muted-foreground">{via}</span>}

      <span className="ml-auto flex shrink-0 items-center gap-2">
        {answersInPlace && (
          <span
            className={cn(
              "flex items-center gap-1 opacity-0 group-hover/row:opacity-100 group-focus-visible/row:opacity-100",
              noteOpen && "opacity-100",
            )}
            onClick={(event) => event.stopPropagation()}
          >
            <Button type="button" variant="outline" size="xs" disabled={pending} onClick={() => send("yes")}>
              <Check />
              {t(($) => $.controls.yes)}
            </Button>
            {/* The popover opens only when No needs its why; otherwise No sends at once. */}
            <Popover open={noteOpen} onOpenChange={(next) => setNoteOpen(next && whyOnNo)}>
              <PopoverTrigger
                render={
                  <Button type="button" variant="outline" size="xs" disabled={pending} onClick={whyOnNo ? undefined : answerNo}>
                    <X />
                    {t(($) => $.controls.no)}
                  </Button>
                }
              />
              <PopoverContent align="end" className="flex w-72 flex-col gap-2 p-2">
                <Textarea
                  autoFocus
                  value={note}
                  disabled={pending}
                  aria-label={t(($) => $.controls.no_reason_label)}
                  placeholder={t(($) => $.controls.no_reason_placeholder)}
                  className="min-h-16"
                  onChange={(event) => setNote(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
                      event.preventDefault();
                      if (note.trim()) send("no", note.trim());
                    }
                  }}
                />
                <div className="flex justify-end">
                  <Button type="button" size="sm" disabled={!note.trim() || pending} onClick={() => send("no", note.trim())}>
                    <Send />
                    {t(($) => $.row.send_no)}
                  </Button>
                </div>
              </PopoverContent>
            </Popover>
          </span>
        )}
        <QuestionBadges
          blocksWork={row.blocks_work}
          dueDate={row.due_date}
          open={open}
          policy={row.answer_policy}
          answererCount={row.answerer_ids.length}
        />
        <ActorAvatar actorType={asker.type} actorId={asker.id} size="sm" />
        {!row.due_date && !row.my_answer && (
          <span className="w-14 shrink-0 text-right text-caption text-muted-foreground">{timeAgo(row.created_at)}</span>
        )}
      </span>
    </div>
  );
}

/** The object a question belongs to when it is no issue: its kind's icon and its name, "Chat with {member}" for a chat. */
function LinkObject({ link }: { link: QuestionRow["link"] }) {
  const linkTitle = useLinkTitle();
  return (
    <span className="flex min-w-0 shrink items-center gap-1 text-caption text-muted-foreground">
      <LinkIcon type={link.type} />
      <span className="truncate">{linkTitle(link)}</span>
    </span>
  );
}
