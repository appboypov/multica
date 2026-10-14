"use client";

import { ChevronRight, FolderGit2, GitBranch, MoreHorizontal, Pencil, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";
import type { Repository, Worklane } from "@multica/core/types";
import { ApiError, clientErrorMessage } from "@multica/core/api";
import { RepositoryRefusalSchema } from "@multica/core/api/schemas";
import { useModalStore } from "@multica/core/modals";
import { useWorkspacePaths } from "@multica/core/paths";
import { useDeleteRepository } from "@multica/core/repositories";
import { sortWorklanes } from "@multica/core/worklanes";
import { cn } from "@multica/ui/lib/utils";
import { Button } from "@multica/ui/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { AppLink } from "../../navigation";
import { useLocale, useT } from "../../i18n";
import { ReleaseChips } from "../../releases/components/release-chips";
import { WorklaneRowWork } from "../../worklanes/components/worklane-row-work";
import { WorklaneMenu } from "../../worklanes/components/worklane-menu";
import { WorklaneStateLabel } from "../../worklanes/components/worklane-state";

/**
 * One worklane of the card, opening its page: its name and state, the issue
 * it works on or takes next, and under them its repository's last and next
 * release, so the row fits the project page's side panel. The chips sit
 * outside the row's link, because each chip is a link of its own.
 */
function WorklaneRow({ worklane }: { worklane: Worklane }) {
  const paths = useWorkspacePaths();
  return (
    <li className="group/row flex min-h-11 items-start gap-2 py-1.5 pr-2 pl-10 hover:bg-accent/40">
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <AppLink
          href={paths.worklaneDetail(worklane.id)}
          className="flex min-w-0 items-start gap-2"
        >
          <GitBranch aria-hidden className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
          <span className="flex min-w-0 flex-col gap-0.5">
            <span className="flex min-w-0 items-center gap-2">
              <span className="min-w-0 truncate font-mono text-body">{worklane.name}</span>
              <WorklaneStateLabel state={worklane.state} />
            </span>
            <WorklaneRowWork worklane={worklane} />
          </span>
        </AppLink>
        <ReleaseChips
          repositoryId={worklane.repository_id}
          worklaneId={worklane.id}
          className="flex-wrap pl-5.5"
        />
      </div>
      {worklane.is_main ? (
        <span aria-hidden className="size-7 shrink-0" />
      ) : (
        <WorklaneMenu
          worklane={worklane}
          className="opacity-0 transition-opacity group-hover/row:opacity-100 focus-visible:opacity-100 data-popup-open:opacity-100"
        />
      )}
    </li>
  );
}

/**
 * A repository of a project's page: its `owner/name`, default branch and
 * number of worklanes in a header that folds the card, Add worklane, and its
 * worklanes under it, `main` first. The menu edits the checkout and releases
 * or removes the repository; Remove awaits the server, is refused while a
 * worklane besides `main` exists, and leaves the checkout folder on its
 * machine.
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
  const { t: tWorklanes } = useT("worklanes");
  const locale = useLocale();
  const deleteRepository = useDeleteRepository();
  const bodyId = `repository-${repository.id}-worklanes`;

  const refusalMessage = (err: unknown): string | null => {
    if (!(err instanceof ApiError) || err.body === undefined) return null;
    const parsed = RepositoryRefusalSchema.safeParse(err.body);
    if (!parsed.success || parsed.data.code !== "has_worklanes") return null;
    const names = parsed.data.worklanes ?? [];
    return tWorklanes(($) => $.card.still_has, {
      count: names.length,
      repository: repository.full_name,
      names: new Intl.ListFormat(locale, { type: "conjunction" }).format(names),
    });
  };

  const handleRemove = async () => {
    try {
      await deleteRepository.mutateAsync(repository.id);
      toast.success(t(($) => $.card.toast_removed, { name: repository.full_name }));
    } catch (err) {
      toast.error(
        refusalMessage(err) ??
          clientErrorMessage(err) ??
          t(($) => $.card.remove_failed, { name: repository.full_name }),
      );
    }
  };

  return (
    <section className="overflow-hidden rounded-lg border bg-card">
      <div className="flex min-h-11 items-center gap-2 pr-2">
        <button
          type="button"
          aria-expanded={open}
          aria-controls={bodyId}
          aria-label={t(($) => $.card.toggle, { name: repository.full_name })}
          onClick={() => onOpenChange(!open)}
          className="flex min-h-11 min-w-0 flex-1 items-center gap-2 self-stretch py-1.5 pl-3 text-left hover:bg-accent/40"
        >
          <ChevronRight
            aria-hidden
            className={cn(
              "size-3.5 shrink-0 text-muted-foreground transition-transform",
              open && "rotate-90",
            )}
          />
          <FolderGit2 aria-hidden className="size-4 shrink-0 text-muted-foreground" />
          <span className="flex min-w-0 flex-col">
            <span className="min-w-0 truncate font-mono text-body font-medium">
              {repository.full_name}
            </span>
            <span className="flex min-w-0 flex-wrap items-center gap-x-2 text-caption text-muted-foreground">
              <span
                className="flex min-w-0 items-center gap-1 font-mono"
                title={t(($) => $.card.default_branch, { branch: repository.default_branch })}
              >
                <GitBranch aria-hidden className="size-3 shrink-0" />
                <span className="truncate">{repository.default_branch}</span>
              </span>
              <span className="shrink-0">
                {tWorklanes(($) => $.worklane_count, { count: worklanes.length })}
              </span>
            </span>
          </span>
        </button>
        <Button
          variant="ghost"
          size="sm"
          onClick={() =>
            useModalStore.getState().open("new-worklane", { repositoryId: repository.id })
          }
        >
          <Plus />
          {tWorklanes(($) => $.card.add)}
        </Button>
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
