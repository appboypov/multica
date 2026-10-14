"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronRight, Plus } from "lucide-react";
import type { Worklane } from "@multica/core/types";
import { useModalStore } from "@multica/core/modals";
import { repositoriesOfProject, repositoryListOptions } from "@multica/core/repositories";
import { worklaneListOptions } from "@multica/core/worklanes";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useT } from "../../i18n";
import { RepositoryCard } from "./repository-card";

/**
 * The repositories of a project's page, in place of Resources: one card per
 * repository by `owner/name`, each open until it is folded, and Repository
 * to add one. The cards' fold is kept for the visit only, so a repository
 * just added shows open.
 */
export function ProjectRepositoriesSection({
  wsId,
  projectId,
}: {
  wsId: string;
  projectId: string;
}) {
  const { t } = useT("repositories");
  const [open, setOpen] = useState(true);
  const [folded, setFolded] = useState<ReadonlySet<string>>(() => new Set());
  const repositoriesQuery = useQuery(repositoryListOptions(wsId));
  const { data: worklanes = [] } = useQuery(worklaneListOptions(wsId));

  const repositories = useMemo(
    () => repositoriesOfProject(repositoriesQuery.data ?? [], projectId),
    [repositoriesQuery.data, projectId],
  );
  const worklanesByRepository = useMemo(() => {
    const map = new Map<string, Worklane[]>();
    for (const worklane of worklanes) {
      const list = map.get(worklane.repository_id);
      if (list) list.push(worklane);
      else map.set(worklane.repository_id, [worklane]);
    }
    return map;
  }, [worklanes]);

  const setCardOpen = (id: string, next: boolean) => {
    setFolded((prev) => {
      const copy = new Set(prev);
      if (next) copy.delete(id);
      else copy.add(id);
      return copy;
    });
  };

  return (
    <div>
      <div className="mb-2 flex items-center gap-1">
        <button
          type="button"
          aria-expanded={open}
          className={`flex min-w-0 flex-1 items-center gap-1 rounded-md px-2 py-1 text-caption font-medium transition-colors hover:bg-accent/70 ${open ? "" : "text-muted-foreground hover:text-foreground"}`}
          onClick={() => setOpen(!open)}
        >
          {t(($) => $.section.title)}
          <span className="font-normal text-muted-foreground">
            {t(($) => $.count, { count: repositories.length })}
          </span>
          <ChevronRight
            aria-hidden
            className={`!size-3 shrink-0 stroke-[2.5] text-muted-foreground transition-transform ${open ? "rotate-90" : ""}`}
          />
        </button>
        <Button
          variant="ghost"
          size="xs"
          onClick={() => useModalStore.getState().open("add-repository", { projectId })}
        >
          <Plus />
          {t(($) => $.section.add)}
        </Button>
      </div>
      {open && (
        <div className="space-y-2 pl-2">
          {repositoriesQuery.isPending ? (
            <Skeleton className="h-11 w-full rounded-lg" />
          ) : repositoriesQuery.isError ? (
            <p role="alert" className="px-2 text-caption text-destructive">
              {t(($) => $.section.load_failed)}
            </p>
          ) : (
            repositories.map((repository) => (
              <RepositoryCard
                key={repository.id}
                repository={repository}
                worklanes={worklanesByRepository.get(repository.id) ?? []}
                open={!folded.has(repository.id)}
                onOpenChange={(next) => setCardOpen(repository.id, next)}
              />
            ))
          )}
        </div>
      )}
    </div>
  );
}
