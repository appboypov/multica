"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { ExternalLink, Lock, Search } from "lucide-react";
import type {
  AvailableRepository,
  RepositoryCheckoutRequest,
  VersionPolicy,
} from "@multica/core/types";
import { clientErrorMessage } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { projectListOptions } from "@multica/core/projects/queries";
import {
  availableRepositoriesOptions,
  repositoryListOptions,
  useCreateRepository,
} from "@multica/core/repositories";
import { cn } from "@multica/ui/lib/utils";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useT } from "../../i18n";
import { useNavigation } from "../../navigation";
import { openExternal } from "../../platform";
import { CheckoutField, defaultCheckoutPath, type CheckoutValue } from "./checkout-field";
import { ReleasesField } from "./releases-field";
import { InstallField } from "./install-field";

/**
 * Add repository on a project: the repositories the workspace's GitHub App
 * can see, a repository already in a project shown with that project and not
 * pickable, then its Checkout, Releases and Install. Add repository stays disabled
 * until a picked repository's checkout passes its check; it awaits the server
 * and shows the project's page, where the new card is open.
 */
export function AddRepositoryDialog({
  onClose,
  data,
}: {
  onClose: () => void;
  data: Record<string, unknown> | null;
}) {
  const projectId = typeof data?.projectId === "string" ? data.projectId : "";
  const { t } = useT("repositories");
  const wsId = useWorkspaceId();
  const wsPaths = useWorkspacePaths();
  const router = useNavigation();
  const createRepository = useCreateRepository();

  const { data: projects = [] } = useQuery(projectListOptions(wsId));
  const { data: repositories = [] } = useQuery(repositoryListOptions(wsId));
  const available = useQuery(availableRepositoriesOptions(wsId));

  const [query, setQuery] = useState("");
  const [picked, setPicked] = useState<AvailableRepository | null>(null);
  const [checkout, setCheckout] = useState<CheckoutValue>({ daemonId: "", path: "" });
  const [check, setCheck] = useState<RepositoryCheckoutRequest | null>(null);
  const [policy, setPolicy] = useState<VersionPolicy | null>(null);
  const [install, setInstall] = useState<string | null>(null);
  const [serverError, setServerError] = useState<string | null>(null);

  const project = projects.find((p) => p.id === projectId);
  const projectTitle = project?.title ?? "";

  // Every repository a project holds, by lower-cased `owner/name`, with that project's title.
  const takenBy = useMemo(() => {
    const titles = new Map(projects.map((p) => [p.id, p.title]));
    return new Map(
      repositories.map((r) => [r.full_name.toLowerCase(), titles.get(r.project_id) ?? ""]),
    );
  }, [projects, repositories]);

  const needle = query.trim().toLowerCase();
  const listed = (available.data?.repositories ?? [])
    .filter((r) => r.full_name.toLowerCase().includes(needle))
    .sort((a, b) => a.full_name.toLowerCase().localeCompare(b.full_name.toLowerCase()));

  const pick = (repository: AvailableRepository) => {
    setPicked(repository);
    setCheck(null);
    setPolicy(null);
    setInstall(null);
    setServerError(null);
    setCheckout((value) => ({ ...value, path: defaultCheckoutPath(repository.full_name) }));
  };

  const canSubmit =
    !!picked && !!check?.passed && policy !== null && install !== null && !createRepository.isPending;

  const handleSubmit = async () => {
    if (!canSubmit || !picked || !check || !policy || install === null) return;
    setServerError(null);
    try {
      await createRepository.mutateAsync({
        project_id: projectId,
        full_name: picked.full_name,
        checkout_check_id: check.id,
        version_policy: policy,
        install_command: install.trim(),
      });
      onClose();
      toast.success(t(($) => $.add.toast_added, { name: picked.full_name, project: projectTitle }));
      router.push(wsPaths.projectDetail(projectId));
    } catch (err) {
      setServerError(clientErrorMessage(err) ?? t(($) => $.add.failed));
    }
  };

  return (
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t(($) => $.add.title, { project: projectTitle })}</DialogTitle>
        </DialogHeader>

        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label>{t(($) => $.picker.label)}</Label>
            <div className="relative">
              <Search
                aria-hidden
                className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground"
              />
              <Input
                autoFocus
                aria-label={t(($) => $.picker.search_placeholder)}
                placeholder={t(($) => $.picker.search_placeholder)}
                className="pl-8"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
            </div>
            <div
              role="listbox"
              aria-label={t(($) => $.picker.label)}
              className="max-h-56 overflow-y-auto rounded-lg border"
            >
              {available.isPending ? (
                <div className="flex flex-col gap-2 p-3">
                  <Skeleton className="h-5 w-2/3" />
                  <Skeleton className="h-5 w-1/2" />
                </div>
              ) : available.isError ? (
                <p role="alert" className="p-3 text-caption text-destructive">
                  {t(($) => $.picker.load_failed)}
                </p>
              ) : listed.length === 0 ? (
                <p className="p-3 text-caption text-muted-foreground">
                  {needle
                    ? t(($) => $.picker.no_match, { query: query.trim() })
                    : t(($) => $.picker.empty)}
                </p>
              ) : (
                listed.map((repository) => {
                  const owner = takenBy.get(repository.full_name.toLowerCase());
                  const taken = owner !== undefined;
                  const selected = picked?.full_name === repository.full_name;
                  return (
                    <button
                      key={repository.github_repo_id}
                      type="button"
                      role="option"
                      aria-selected={selected}
                      aria-disabled={taken || undefined}
                      onClick={() => {
                        if (!taken) pick(repository);
                      }}
                      className={cn(
                        "flex w-full items-center gap-2 px-3 py-2 text-left",
                        taken
                          ? "cursor-not-allowed text-muted-foreground"
                          : "hover:bg-accent/50",
                        selected && "bg-accent",
                      )}
                    >
                      {repository.private && (
                        <Lock aria-hidden className="size-3.5 shrink-0 text-muted-foreground" />
                      )}
                      <span className="min-w-0 flex-1 truncate font-mono text-body">
                        {repository.full_name}
                      </span>
                      <span className="shrink-0 text-caption text-muted-foreground">
                        {taken
                          ? t(($) => $.picker.in_project, { project: owner })
                          : t(($) => $.picker.meta, {
                              visibility: repository.private
                                ? t(($) => $.picker.private)
                                : t(($) => $.picker.public),
                              branch: repository.default_branch,
                            })}
                      </span>
                    </button>
                  );
                })
              )}
            </div>
            {available.data?.install_url && (
              <Button
                type="button"
                variant="link"
                size="xs"
                className="self-start px-0"
                onClick={() => openExternal(available.data.install_url)}
              >
                {t(($) => $.picker.missing)}
                <ExternalLink />
              </Button>
            )}
          </div>

          <CheckoutField
            fullName={picked?.full_name ?? null}
            defaultBranch={picked?.default_branch ?? null}
            value={checkout}
            onChange={setCheckout}
            onCheck={setCheck}
          />

          <ReleasesField
            versionFiles={check ? check.version_files : null}
            lastTag={check ? check.last_tag : null}
            value={policy}
            onChange={setPolicy}
            disabled={!check}
          />

          <InstallField
            key={picked?.full_name ?? ""}
            found={check ? check.install_commands : null}
            value={install}
            onChange={setInstall}
            finding={!!picked}
            disabled={!check}
          />

          {serverError ? (
            <p role="alert" className="text-caption text-destructive">
              {serverError}
            </p>
          ) : null}
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>
            {t(($) => $.add.cancel)}
          </Button>
          <Button
            type="button"
            disabled={!canSubmit}
            aria-busy={createRepository.isPending || undefined}
            onClick={() => void handleSubmit()}
          >
            {createRepository.isPending ? t(($) => $.add.submitting) : t(($) => $.add.submit)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
