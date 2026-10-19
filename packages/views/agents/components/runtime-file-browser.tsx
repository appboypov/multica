"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowUp, File, Folder } from "lucide-react";
import type { RuntimeDevice } from "@multica/core/types";
import { createLogger } from "@multica/core/logger";
import { runtimeDirectoryListingOptions } from "@multica/core/runtimes";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Input } from "@multica/ui/components/ui/input";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useT } from "../../i18n";

const logger = createLogger("agents.runtime-browser");

/** The listing path the browser opens on: the user's home on the runtime machine. */
const HOME_PATH = "~";

/**
 * The parent of an absolute path on the runtime machine, POSIX or Windows.
 * A filesystem root, or a path that is not absolute, is its own parent.
 */
function parentPath(path: string): string {
  const trimmed = path.replace(/[\\/]+$/, "");
  const index = Math.max(trimmed.lastIndexOf("/"), trimmed.lastIndexOf("\\"));
  if (index < 0) return path;
  if (index === 0) return trimmed.slice(0, 1);
  return trimmed.slice(0, trimmed[index - 1] === ":" ? index + 1 : index);
}

/**
 * Browses the runtime machine of an agent: its user's home first, then any
 * folder opened, path typed or parent walked up to, up to the filesystem
 * root. Files and folders are ticked across folders and handed to `onAdd` in
 * the order ticked; with `foldersOnly` only folders can be ticked.
 */
export function RuntimeFileBrowser({
  open,
  onOpenChange,
  runtime,
  onAdd,
  foldersOnly = false,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  runtime: RuntimeDevice;
  onAdd: (paths: string[]) => void;
  foldersOnly?: boolean;
}) {
  const { t } = useT("agents");
  const online = runtime.status === "online";
  const [path, setPath] = useState(HOME_PATH);
  const [typedPath, setTypedPath] = useState<string | null>(null);
  const [ticked, setTicked] = useState<string[]>([]);
  const tickedSet = useMemo(() => new Set(ticked), [ticked]);

  const listing = useQuery({
    ...runtimeDirectoryListingOptions(runtime.id, path),
    enabled: open && online,
  });
  const listedPath = listing.data?.path ?? path;
  const parent = parentPath(listedPath);

  useEffect(() => {
    if (!listing.error) return;
    logger.warn("runtime directory listing failed", {
      runtimeId: runtime.id,
      path,
      error: listing.error.message,
    });
  }, [listing.error, runtime.id, path]);

  const handleOpenChange = (next: boolean) => {
    if (!next) {
      setPath(HOME_PATH);
      setTypedPath(null);
      setTicked([]);
    }
    onOpenChange(next);
  };

  const openPath = (next: string) => {
    setTypedPath(null);
    if (next === path) {
      void listing.refetch();
      return;
    }
    setPath(next);
  };

  const handleToggle = (entryPath: string) => {
    setTicked((prev) =>
      prev.includes(entryPath)
        ? prev.filter((p) => p !== entryPath)
        : [...prev, entryPath],
    );
  };

  const handleAdd = () => {
    if (!online || ticked.length === 0) return;
    logger.info("runtime paths added", {
      runtimeId: runtime.id,
      count: ticked.length,
    });
    onAdd(ticked);
    handleOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle className="text-body">
            {t(($) => $.runtime_browser.title)}
          </DialogTitle>
        </DialogHeader>

        {online ? (
          <>
            <div className="flex items-center gap-2">
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={t(($) => $.runtime_browser.up)}
                disabled={parent === listedPath}
                onClick={() => openPath(parent)}
              >
                <ArrowUp />
              </Button>
              <Input
                aria-label={t(($) => $.runtime_browser.path)}
                placeholder={t(($) => $.runtime_browser.path)}
                value={typedPath ?? listedPath}
                onChange={(e) => setTypedPath(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key !== "Enter") return;
                  e.preventDefault();
                  openPath(e.currentTarget.value.trim() || HOME_PATH);
                }}
                className="font-mono"
              />
            </div>

            <div className="h-72 overflow-y-auto rounded-lg border">
              {listing.isPending ? (
                <div className="space-y-2 p-3">
                  <Skeleton className="h-5 w-2/3" />
                  <Skeleton className="h-5 w-1/2" />
                  <Skeleton className="h-5 w-3/5" />
                </div>
              ) : listing.isError ? (
                <p role="alert" className="p-3 text-body text-destructive">
                  {t(($) => $.runtime_browser.error, {
                    message: listing.error.message,
                  })}
                </p>
              ) : listing.data.entries.length === 0 ? (
                <p className="p-3 text-body text-muted-foreground">
                  {t(($) => $.runtime_browser.empty)}
                </p>
              ) : (
                <ul className="py-1">
                  {listing.data.entries.map((entry) => {
                    const checked = tickedSet.has(entry.path);
                    const Icon = entry.kind === "folder" ? Folder : File;
                    return (
                      <li
                        key={entry.path}
                        className={`flex items-center gap-2 px-3 py-1.5 ${
                          checked ? "bg-primary/5" : "hover:bg-accent/40"
                        }`}
                      >
                        {foldersOnly && entry.kind !== "folder" ? (
                          <span aria-hidden className="size-4 shrink-0" />
                        ) : (
                          <Checkbox
                            checked={checked}
                            onCheckedChange={() => handleToggle(entry.path)}
                            aria-label={entry.name}
                          />
                        )}
                        <Icon
                          aria-hidden
                          className="size-4 shrink-0 text-muted-foreground"
                        />
                        {entry.kind === "folder" ? (
                          <button
                            type="button"
                            title={entry.path}
                            onClick={() => openPath(entry.path)}
                            className="min-w-0 truncate text-left text-body hover:underline"
                          >
                            {entry.name}
                          </button>
                        ) : (
                          <span
                            title={entry.path}
                            className="min-w-0 truncate text-body"
                          >
                            {entry.name}
                          </span>
                        )}
                        {entry.hidden && (
                          <Badge variant="secondary">
                            {t(($) => $.runtime_browser.hidden)}
                          </Badge>
                        )}
                      </li>
                    );
                  })}
                </ul>
              )}
            </div>
          </>
        ) : (
          <p role="status" className="text-body text-muted-foreground">
            {t(($) => $.runtime_browser.offline)}
          </p>
        )}

        <DialogFooter>
          <Button
            type="button"
            onClick={handleAdd}
            disabled={!online || ticked.length === 0}
          >
            {t(($) => $.runtime_browser.add)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
