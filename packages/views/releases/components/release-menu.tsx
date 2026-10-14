"use client";

import { toast } from "sonner";
import { ArrowDown, ArrowUp, MoreHorizontal, Pencil, Trash2 } from "lucide-react";
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
 * The menu of an unshipped release on a worklane's Timeline: Edit release,
 * Move earlier, Move later and Remove release. A move that would break the
 * line is disabled with its reason, previewed with the client rules.
 */
export function ReleaseMenu({
  repository,
  release,
  line,
  findingTag = false,
}: {
  repository: Repository;
  release: Release;
  line: Line;
  /** The repository's last tag is being found; moves wait for it. */
  findingTag?: boolean;
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
            aria-label={t(($) => $.menu.trigger)}
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
        <DropdownMenuItem variant="destructive" onClick={() => open("remove-release", data)}>
          <Trash2 className="size-3.5" aria-hidden="true" />
          {t(($) => $.menu.remove)}
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
