"use client";

import { toast } from "sonner";
import { ArrowDown, ArrowUp, ListPlus, MoreHorizontal, Pencil, Trash2 } from "lucide-react";
import { clientErrorMessage } from "@multica/core/api";
import { useModalStore } from "@multica/core/modals";
import {
  formatVersion,
  previewMove,
  releaseRefusalOf,
  useMoveRelease,
  type Line,
} from "@multica/core/releases";
import type { Release, Repository } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { useT } from "../../i18n";
import { moveProblemMessage, refusalMessage } from "./release-messages";

type Direction = "earlier" | "later";

/**
 * The release's editing and line actions. Published releases keep their
 * version and position; their title and description remain editable.
 */
export function ReleaseMenu({
  repository,
  release,
  line,
  findingTag = false,
  worklaneId,
}: {
  repository: Repository;
  release: Release;
  line: Line;
  /** The repository's last tag is being found; moves and Remove wait for it. */
  findingTag?: boolean;
  /** On a worklane: Add issue to release ticks its issues in no release. */
  worklaneId?: string;
}) {
  const { t } = useT("releases");
  const open = useModalStore((s) => s.open);
  const moveRelease = useMoveRelease();

  const reasonOf = (direction: Direction): string | null => {
    if (findingTag) return t(($) => $.menu.finding_tag);
    return moveProblemMessage(t, previewMove(line, release.id, direction));
  };
  const earlier = reasonOf("earlier");
  const later = reasonOf("later");
  const shipped = release.shipped_at !== null;

  const move = async (direction: Direction) => {
    try {
      const answer = await moveRelease.mutateAsync({
        id: release.id,
        repositoryId: repository.id,
        direction,
      });
      const moved = answer.releases.find((r) => r.id === release.id) ?? release;
      toast.success(
        t(($) => $.menu.toast_moved, {
          version: formatVersion(moved.version),
          title: moved.title,
          direction: t(($) => $.menu.direction[direction]),
        }),
      );
    } catch (err) {
      const refusal = releaseRefusalOf(err);
      toast.error(
        refusal
          ? refusalMessage(t, refusal, repository.full_name)
          : (clientErrorMessage(err) ?? t(($) => $.menu.move_failed)),
      );
    }
  };

  const data = { repositoryId: repository.id, releaseId: release.id };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={t(($) => $.menu.trigger, { version: formatVersion(release.version), title: release.title })}
            className="text-muted-foreground"
          >
            <MoreHorizontal className="size-4" aria-hidden="true" />
          </Button>
        }
      />
      <DropdownMenuContent align="end" className="w-auto min-w-48 max-w-80">
        <DropdownMenuItem onClick={() => open("edit-release", data)}>
          <Pencil className="size-3.5" aria-hidden="true" />
          {t(($) => $.menu.edit)}
        </DropdownMenuItem>
        {worklaneId && !shipped ? (
          <DropdownMenuItem
            onClick={() => open("add-issues-to-release", { releaseId: release.id, worklaneId })}
          >
            <ListPlus className="size-3.5" aria-hidden="true" />
            {t(($) => $.issues.add_issue)}
          </DropdownMenuItem>
        ) : null}
        <MoveItem
          label={t(($) => $.menu.move_earlier)}
          icon={<ArrowUp className="size-3.5" aria-hidden="true" />}
          reason={earlier}
          disabled={moveRelease.isPending}
          onMove={() => void move("earlier")}
        />
        <MoveItem
          label={t(($) => $.menu.move_later)}
          icon={<ArrowDown className="size-3.5" aria-hidden="true" />}
          reason={later}
          disabled={moveRelease.isPending}
          onMove={() => void move("later")}
        />
        <DropdownMenuSeparator />
        <DropdownMenuItem
          variant="destructive"
          disabled={findingTag || shipped}
          onClick={() => open("remove-release", data)}
          className="items-start"
        >
          <Trash2 className="mt-0.5 size-3.5" aria-hidden="true" />
          <span className="flex min-w-0 flex-col">
            <span>{t(($) => $.menu.remove)}</span>
            {findingTag ? (
              <span className="text-caption text-muted-foreground">{t(($) => $.menu.finding_tag)}</span>
            ) : null}
            {shipped ? <span className="text-caption text-muted-foreground">{t(($) => $.refusal.release_shipped)}</span> : null}
          </span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function MoveItem({
  label,
  icon,
  reason,
  disabled,
  onMove,
}: {
  label: string;
  icon: React.ReactNode;
  reason: string | null;
  disabled: boolean;
  onMove: () => void;
}) {
  return (
    <DropdownMenuItem disabled={disabled || reason !== null} onClick={onMove} className="items-start">
      <span className="mt-0.5">{icon}</span>
      <span className="flex min-w-0 flex-col">
        <span>{label}</span>
        {reason ? <span className="text-caption text-muted-foreground">{reason}</span> : null}
      </span>
    </DropdownMenuItem>
  );
}
