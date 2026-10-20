"use client";

import { Loader2, Server } from "lucide-react";
import type {
  RuntimeSkillException,
  RuntimeLocalSkillSummary,
  Skill,
} from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { useT } from "../../../i18n";

// The parts the agent and squad Skills tabs share: runtime skill identity,
// the section layout, notices and the skill detail dialog.

export type SelectedSkill =
  | { kind: "workspace"; id: string }
  | { kind: "runtime"; skill: RuntimeLocalSkillSummary }
  | null;

export function runtimeSkillIdentity(skill: RuntimeLocalSkillSummary): string {
  return `runtime:${skill.root ?? "unknown"}:${skill.key}:${skill.plugin ?? ""}`;
}

export function isRuntimeSkillException(
  exceptions: RuntimeSkillException[] | undefined,
  runtimeId: string | undefined,
  skill: RuntimeLocalSkillSummary,
): boolean {
  if (!runtimeId || !skill.root) return false;
  return (exceptions ?? []).some(
    (exception) =>
      exception.runtime_id === runtimeId &&
      exception.provider === skill.provider &&
      exception.root === skill.root &&
      exception.key === skill.key &&
      (exception.plugin ?? "") === (skill.plugin ?? ""),
  );
}

export function CapabilitySection({
  title,
  description,
  action,
  children,
}: {
  title: string;
  description: string;
  action?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <section className="space-y-3">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h3 className="text-body font-medium">{title}</h3>
          <p className="mt-1 text-caption leading-5 text-muted-foreground">{description}</p>
        </div>
        {action}
      </div>
      {children}
    </section>
  );
}

export function EmptyState({ icon, title, hint }: { icon: React.ReactNode; title: string; hint?: string }) {
  return (
    <div className="flex flex-col items-center justify-center rounded-lg border border-dashed py-10 text-muted-foreground">
      <span className="opacity-50">{icon}</span>
      <p className="mt-3 text-body">{title}</p>
      {hint && <p className="mt-1 max-w-sm text-center text-caption">{hint}</p>}
    </div>
  );
}

export function RuntimeNotice({ text, loading = false }: { text: string; loading?: boolean }) {
  return (
    <div className="flex items-center gap-2 rounded-lg border border-dashed px-4 py-6 text-caption text-muted-foreground">
      {loading ? (
        <Loader2 className="h-4 w-4 animate-spin motion-reduce:animate-none" />
      ) : (
        <Server className="h-4 w-4" />
      )}
      {text}
    </div>
  );
}

export function SkillDetailDialog({
  selected,
  onOpenChange,
  workspaceSkill,
  loading,
}: {
  selected: SelectedSkill;
  onOpenChange: (open: boolean) => void;
  workspaceSkill?: Skill;
  loading: boolean;
}) {
  const { t } = useT("agents");
  const runtimeSkill = selected?.kind === "runtime" ? selected.skill : null;
  const title = runtimeSkill?.name || workspaceSkill?.name || t(($) => $.tab_body.skills.detail_title);
  const description = runtimeSkill?.description || workspaceSkill?.description;

  return (
    <Dialog open={selected !== null} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[80vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <div className="flex items-center gap-2 pr-8">
            <DialogTitle>{title}</DialogTitle>
            <Badge variant={runtimeSkill ? "secondary" : "outline"}>
              {runtimeSkill
                ? t(($) => $.tab_body.skills.inherited_badge)
                : t(($) => $.tab_body.skills.workspace_badge)}
            </Badge>
          </div>
          {description && <DialogDescription>{description}</DialogDescription>}
        </DialogHeader>
        {loading ? (
          <div className="flex items-center gap-2 py-8 text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin motion-reduce:animate-none" />
            {t(($) => $.tab_body.skills.detail_loading)}
          </div>
        ) : runtimeSkill ? (
          <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-3 rounded-lg border p-4 text-caption">
            <dt className="text-muted-foreground">{t(($) => $.tab_body.skills.detail_source)}</dt>
            <dd className="break-all">{runtimeSkill.source_path}</dd>
            <dt className="text-muted-foreground">{t(($) => $.tab_body.skills.detail_provider)}</dt>
            <dd>{runtimeSkill.provider}</dd>
            {runtimeSkill.plugin && (
              <>
                <dt className="text-muted-foreground">{t(($) => $.tab_body.skills.detail_plugin)}</dt>
                <dd>{runtimeSkill.plugin}</dd>
              </>
            )}
            <dt className="text-muted-foreground">{t(($) => $.tab_body.skills.detail_files)}</dt>
            <dd>{runtimeSkill.file_count}</dd>
          </dl>
        ) : workspaceSkill ? (
          <div className="space-y-4">
            <div className="max-h-96 overflow-auto rounded-lg border bg-muted/30 p-4">
              <pre className="whitespace-pre-wrap break-words font-mono text-caption leading-5">
                {workspaceSkill.content}
              </pre>
            </div>
            {(workspaceSkill.files ?? []).length > 0 && (
              <div>
                <h4 className="text-caption font-medium">{t(($) => $.tab_body.skills.detail_supporting_files)}</h4>
                <div className="mt-2 flex flex-wrap gap-2">
                  {(workspaceSkill.files ?? []).map((file) => (
                    <Badge key={file.id} variant="outline">{file.path}</Badge>
                  ))}
                </div>
              </div>
            )}
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
