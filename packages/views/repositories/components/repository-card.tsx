"use client";

import { ChevronRight, FolderGit2, GitBranch, MoreHorizontal, Pencil, Trash2 } from "lucide-react";
import { toast } from "sonner";
import type { Repository, Worklane } from "@multica/core/types";
import { clientErrorMessage } from "@multica/core/api";
import { useModalStore } from "@multica/core/modals";
import { useDeleteRepository } from "@multica/core/repositories";
import { sortWorklanes } from "@multica/core/worklanes";
import { cn } from "@multica/ui/lib/utils";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { useT } from "../../i18n";

/** One worklane of the card, with its state. */
function WorklaneRow({ worklane }: { worklane: Worklane }) {
  const { t } = useT("repositories");
  return (
    <li className="flex h-9 items-center gap-2 px-4 pl-10">
      <GitBranch aria-hidden className="size-3.5 shrink-0 text-muted-foreground" />
      <span className="min-w-0 truncate font-mono text-body">{worklane.name}</span>
      <Badge variant={worklane.state === "busy" ? "default" : "secondary"} className="ml-auto">
        {worklane.state === "busy"
          ? t(($) => $.card.worklane_busy)
          : t(($) => $.card.worklane_free)}
      </Badge>
    </li>
  );
}

/**
 * A repository of a project's page: its `owner/name` and default branch in a
 * header that folds the card, and its worklanes under it. The menu edits the
 * checkout and releases or removes the repository; Remove awaits the server
 * and leaves the checkout folder on its machine.
 */
export function RepositoryCard({
  repository,
  worklanes,
  open,
  onOpenChange,
}: {
  repository: Repository;
  /** The repository's worklanes; the card sorts them. */
  worklanes: readonly Worklane[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useT("repositories");
  const deleteRepository = useDeleteRepository();
  const bodyId = `repository-${repository.id}-worklanes`;

  const handleRemove = async () => {
    try {
      await deleteRepository.mutateAsync(repository.id);
      toast.success(t(($) => $.card.toast_removed, { name: repository.full_name }));
    } catch (err) {
      toast.error(
        clientErrorMessage(err) ?? t(($) => $.card.remove_failed, { name: repository.full_name }),
      );
    }
  };

  return (
    <section className="overflow-hidden rounded-lg border bg-card">
      <div className="flex h-11 items-center gap-2 pr-2">
        <button
          type="button"
          aria-expanded={open}
          aria-controls={bodyId}
          aria-label={t(($) => $.card.toggle, { name: repository.full_name })}
          onClick={() => onOpenChange(!open)}
          className="flex h-full min-w-0 flex-1 items-center gap-2 pl-3 text-left hover:bg-accent/40"
        >
          <ChevronRight
            aria-hidden
            className={cn(
              "size-3.5 shrink-0 text-muted-foreground transition-transform",
              open && "rotate-90",
            )}
          />
          <FolderGit2 aria-hidden className="size-4 shrink-0 text-muted-foreground" />
          <span className="min-w-0 truncate font-mono text-body font-medium">
            {repository.full_name}
          </span>
          <span
            className="flex shrink-0 items-center gap-1 font-mono text-caption text-muted-foreground"
            title={t(($) => $.card.default_branch, { branch: repository.default_branch })}
          >
            <GitBranch aria-hidden className="size-3" />
            {repository.default_branch}
          </span>
        </button>
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={t(($) => $.card.menu)}
                disabled={deleteRepository.isPending}
                aria-busy={deleteRepository.isPending || undefined}
              />
            }
          >
            <MoreHorizontal />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-auto">
            <DropdownMenuItem
              onClick={() =>
                useModalStore.getState().open("edit-repository", { repositoryId: repository.id })
              }
            >
              <Pencil className="size-3.5" />
              {t(($) => $.card.edit)}
            </DropdownMenuItem>
            <DropdownMenuItem variant="destructive" onClick={() => void handleRemove()}>
              <Trash2 className="size-3.5" />
              {t(($) => $.card.remove)}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      {open && (
        <ul id={bodyId} className="border-t py-1">
          {sortWorklanes(worklanes).map((worklane) => (
            <WorklaneRow key={worklane.id} worklane={worklane} />
          ))}
        </ul>
      )}
    </section>
  );
}
