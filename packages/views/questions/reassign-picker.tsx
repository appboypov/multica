"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { UserRoundPen } from "lucide-react";
import { toast } from "sonner";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { createLogger } from "@multica/core/logger";
import { useReassignQuestion } from "@multica/core/questions";
import type { Question } from "@multica/core/types";
import { memberListOptions } from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { ActorAvatar } from "../common/actor-avatar";
import { matchesPinyin } from "../editor/extensions/pinyin-match";
import { useT } from "../i18n";
import { PickerEmpty, PickerItem, PickerSection, PropertyPicker } from "../issues/components/pickers";

const logger = createLogger("questions.reassign");

/**
 * **Reassign**: the assignee picker's popover with members only, since only
 * members answer, and a note the new answerer reads, without which the
 * question is not handed over.
 */
export function ReassignPicker({
  question,
  issueId,
  disabled = false,
}: {
  question: Question;
  issueId: string;
  disabled?: boolean;
}) {
  const { t } = useT("questions");
  const wsId = useWorkspaceId();
  const userId = useAuthStore((s) => s.user?.id);
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const reassign = useReassignQuestion(wsId, issueId);
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState("");
  const [picked, setPicked] = useState<{ user_id: string; name: string } | null>(null);
  const [note, setNote] = useState("");

  const query = filter.trim().toLowerCase();
  const answererIds = new Set(question.answerers.map((a) => a.user_id));
  // The viewer and the current answerers are no one to hand it to.
  const candidates = members.filter(
    (m) => m.user_id !== userId && !answererIds.has(m.user_id)
      && (!query || m.name.toLowerCase().includes(query) || matchesPinyin(m.name, query)),
  );

  const close = (next: boolean) => {
    setOpen(next);
    if (!next) {
      setFilter("");
      setPicked(null);
      setNote("");
    }
  };

  const submit = () => {
    if (!picked || !note.trim()) return;
    reassign.mutate(
      { id: question.id, user_id: picked.user_id, note: note.trim() },
      {
        onSuccess: () => close(false),
        onError: (error) => {
          logger.error("reassign failed", { questionId: question.id, error });
          toast.error(t(($) => $.reassign.failed), { description: error.message });
        },
      },
    );
  };

  return (
    <PropertyPicker
      open={open}
      onOpenChange={close}
      width="w-72"
      align="start"
      searchable
      searchPlaceholder={t(($) => $.reassign.search_placeholder)}
      onSearchChange={setFilter}
      triggerRender={<Button type="button" variant="ghost" size="sm" disabled={disabled} className="text-muted-foreground" />}
      trigger={
        <>
          <UserRoundPen />
          {t(($) => $.card.reassign)}
        </>
      }
      footer={
        picked && (
          <form
            className="flex flex-col gap-2 p-1"
            onSubmit={(event) => {
              event.preventDefault();
              submit();
            }}
          >
            <Textarea
              autoFocus
              value={note}
              aria-label={t(($) => $.reassign.note_label, { name: picked.name })}
              placeholder={t(($) => $.reassign.note_placeholder)}
              className="min-h-16"
              onChange={(event) => setNote(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
                  event.preventDefault();
                  submit();
                }
              }}
            />
            <Button type="submit" size="sm" className="self-end" disabled={!note.trim() || reassign.isPending}>
              {t(($) => $.reassign.submit)}
            </Button>
          </form>
        )
      }
    >
      {candidates.length > 0 && (
        <PickerSection label={t(($) => $.reassign.members_group)}>
          {candidates.map((m) => (
            <PickerItem
              key={m.user_id}
              selected={picked?.user_id === m.user_id}
              onClick={() => setPicked({ user_id: m.user_id, name: m.name })}
            >
              <ActorAvatar actorType="member" actorId={m.user_id} size="sm" />
              <span className="truncate">{m.name}</span>
            </PickerItem>
          ))}
        </PickerSection>
      )}
      {candidates.length === 0 && <PickerEmpty />}
    </PropertyPicker>
  );
}
