"use client";

import { useEffect, useMemo, useState } from "react";
import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, Folder, FolderPlus, Loader2, Trash2 } from "lucide-react";
import { toast } from "sonner";
import type { AgentRuntime, SkillFolder } from "@multica/core/types";
import { api } from "@multica/core/api";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { createLogger } from "@multica/core/logger";
import { resolveSkillFolderScan, runtimeListOptions } from "@multica/core/runtimes";
import {
  skillFolderListOptions,
  workspaceKeys,
} from "@multica/core/workspace/queries";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { RuntimeFileBrowser } from "../../agents/components/runtime-file-browser";
import {
  buildRuntimeMachines,
  type RuntimeMachine,
} from "../../runtimes/components/runtime-machines";
import { useT } from "../../i18n";

const logger = createLogger("skills.folders");

/** The runtime that browses and scans a machine: the user's first online one. */
function machineRuntime(machine: RuntimeMachine, userId: string | null): AgentRuntime | null {
  return (
    machine.runtimes.find((r) => r.owner_id === userId && r.status === "online") ?? null
  );
}

/**
 * The skill folders of the workspace's library. Every folder is read again
 * each time the section opens, so the library holds the skills that are in
 * the folders now. A folder's skills come and go with it.
 */
export function SkillFoldersSection() {
  const { t } = useT("skills");
  const wsId = useWorkspaceId();
  const qc = useQueryClient();
  const userId = useAuthStore((s) => s.user?.id ?? null);
  const { data: folders = [] } = useQuery(skillFolderListOptions(wsId));
  const { data: runtimes = [] } = useQuery(runtimeListOptions(wsId));
  const [browseRuntime, setBrowseRuntime] = useState<AgentRuntime | null>(null);

  const machines = useMemo(
    () =>
      buildRuntimeMachines(
        runtimes.filter((r) => r.runtime_mode === "local"),
        { now: Date.now(), currentUserId: userId },
      ),
    [runtimes, userId],
  );
  const machinesByDaemon = useMemo(() => {
    const map = new Map<string, RuntimeMachine>();
    for (const m of machines) if (m.daemonId) map.set(m.daemonId, m);
    return map;
  }, [machines]);
  const myMachines = useMemo(
    () => machines.filter((m) => m.daemonId && machineRuntime(m, userId)),
    [machines, userId],
  );

  // One scan per machine of the user that holds folders. The folder ids are
  // part of the key, so adding or removing a folder reads the machine again.
  const scanTargets = useMemo(
    () =>
      myMachines.flatMap((machine) => {
        const runtime = machineRuntime(machine, userId);
        const ids = folders
          .filter((f) => f.daemon_id === machine.daemonId)
          .map((f) => f.id);
        return runtime && ids.length > 0 ? [{ machine, runtime, ids }] : [];
      }),
    [myMachines, folders, userId],
  );
  const scans = useQueries({
    queries: scanTargets.map(({ runtime, ids }) => ({
      queryKey: [...workspaceKeys.skillFolders(wsId), "scan", runtime.id, ...ids],
      queryFn: async () => {
        const skipped = await resolveSkillFolderScan(runtime.id);
        await qc.invalidateQueries({ queryKey: workspaceKeys.skills(wsId) });
        return skipped;
      },
      staleTime: 0,
      gcTime: 0,
      retry: false,
      refetchOnWindowFocus: false,
    })),
  });

  useEffect(() => {
    scans.forEach((scan, i) => {
      if (!scan.error) return;
      logger.warn("skill folder scan failed", {
        runtimeId: scanTargets[i]?.runtime.id,
        error: scan.error.message,
      });
    });
  }, [scans, scanTargets]);

  const nameTaken = scans.flatMap((scan) => scan.data ?? []);
  const scanErrors = scans.flatMap((scan, i) =>
    scan.error ? [{ machine: scanTargets[i]!.machine.title, message: scan.error.message }] : [],
  );
  const scanning = scans.some((scan) => scan.isFetching);

  const addFolders = useMutation({
    mutationFn: async ({ runtime, paths }: { runtime: AgentRuntime; paths: string[] }) => {
      for (const path of paths) await api.createSkillFolder(runtime.id, path);
    },
    onSuccess: (_, { runtime, paths }) => {
      logger.info("skill folders added", { runtimeId: runtime.id, count: paths.length });
    },
    onError: (error) => {
      logger.warn("skill folder add failed", { error: error.message });
      toast.error(t(($) => $.folders.add_failed_toast, { message: error.message }));
    },
    onSettled: () => qc.invalidateQueries({ queryKey: workspaceKeys.skillFolders(wsId) }),
  });

  const removeFolder = useMutation({
    mutationFn: (folder: SkillFolder) => api.deleteSkillFolder(folder.id),
    onSuccess: (_, folder) => {
      logger.info("skill folder removed", { folderId: folder.id });
    },
    onError: (error) => {
      logger.warn("skill folder remove failed", { error: error.message });
      toast.error(t(($) => $.folders.remove_failed_toast, { message: error.message }));
    },
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: workspaceKeys.skillFolders(wsId) });
      void qc.invalidateQueries({ queryKey: workspaceKeys.skills(wsId) });
    },
  });

  const addButton = (onClick?: () => void) => (
    <Button
      type="button"
      variant="outline"
      size="xs"
      disabled={myMachines.length === 0 || addFolders.isPending}
      onClick={onClick}
    >
      <FolderPlus />
      {t(($) => $.folders.add)}
    </Button>
  );

  return (
    <section
      aria-label={t(($) => $.folders.title)}
      className="flex shrink-0 flex-col gap-2 border-b px-6 py-3"
    >
      <div className="flex items-center gap-2">
        <h2 className="text-body font-medium">{t(($) => $.folders.title)}</h2>
        {scanning && (
          <Loader2
            aria-label={t(($) => $.folders.scanning)}
            className="size-3.5 animate-spin text-muted-foreground motion-reduce:animate-none"
          />
        )}
        <div className="ml-auto">
          {myMachines.length > 1 ? (
            <DropdownMenu>
              <DropdownMenuTrigger render={addButton()} />
              <DropdownMenuContent align="end">
                {myMachines.map((machine) => (
                  <DropdownMenuItem
                    key={machine.id}
                    onClick={() => setBrowseRuntime(machineRuntime(machine, userId))}
                  >
                    {machine.title}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
          ) : (
            addButton(() => setBrowseRuntime(machineRuntime(myMachines[0]!, userId)))
          )}
        </div>
      </div>

      {folders.length === 0 ? (
        <p className="text-caption text-muted-foreground">
          {myMachines.length === 0 ? t(($) => $.folders.no_machine) : t(($) => $.folders.empty)}
        </p>
      ) : (
        <ul className="flex flex-col gap-1">
          {folders.map((folder) => {
            const machine = machinesByDaemon.get(folder.daemon_id);
            return (
              <li key={folder.id} className="flex items-center gap-2 text-caption">
                <Folder aria-hidden className="size-3.5 shrink-0 text-muted-foreground" />
                <span className="min-w-0 truncate font-mono" title={folder.path}>
                  {folder.path}
                </span>
                <span className="shrink-0 text-muted-foreground">
                  {machine?.title ?? t(($) => $.folders.other_machine)}
                </span>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-xs"
                  className="ml-auto"
                  aria-label={t(($) => $.folders.remove_aria, { path: folder.path })}
                  disabled={removeFolder.isPending}
                  onClick={() => removeFolder.mutate(folder)}
                >
                  <Trash2 />
                </Button>
              </li>
            );
          })}
        </ul>
      )}

      {nameTaken.length > 0 && (
        <ul className="flex flex-col gap-1">
          {nameTaken.map((skill) => (
            <li key={skill.source_path} className="flex items-center gap-2 text-caption">
              <Badge variant="secondary">{t(($) => $.folders.name_taken)}</Badge>
              <span className="font-medium">{skill.name}</span>
              <span className="min-w-0 truncate font-mono text-muted-foreground" title={skill.source_path}>
                {skill.source_path}
              </span>
            </li>
          ))}
        </ul>
      )}

      {scanErrors.map((error) => (
        <p key={error.machine} role="alert" className="flex items-center gap-1.5 text-caption text-warning">
          <AlertTriangle aria-hidden className="size-3.5 shrink-0" />
          {t(($) => $.folders.scan_failed, error)}
        </p>
      ))}

      {browseRuntime && (
        <RuntimeFileBrowser
          open
          foldersOnly
          onOpenChange={(open) => {
            if (!open) setBrowseRuntime(null);
          }}
          runtime={browseRuntime}
          onAdd={(paths) => addFolders.mutate({ runtime: browseRuntime, paths })}
        />
      )}
    </section>
  );
}
