"use client";

import { useId, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { Check, Circle, CircleDot, Rocket } from "lucide-react";
import { clientErrorMessage } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import {
  PLANNED_RELEASE_ID,
  bumpOptions,
  formatVersion,
  parseVersion,
  previewEdit,
  previewPlan,
  releaseRefusalOf,
  useEditRelease,
  usePlanRelease,
  versionBefore,
  versionString,
  type LinePreview,
  type ReleaseDraft,
} from "@multica/core/releases";
import { repositoryListOptions } from "@multica/core/repositories";
import type { Release, ReleaseBump, ReleaseOwnerRequest, Repository } from "@multica/core/types";
import { cn } from "@multica/ui/lib/utils";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../../i18n";
import { useLastTag, useReleaseLine } from "../use-release-line";
import { problemMessage, refusalMessage } from "./release-messages";
import { ReleaseIssuesField } from "./release-issues-field";
import { ReleaseMilestones } from "./release-milestones";
import { NO_OWNER, ReleaseOwnerLabel, ReleaseOwnerPicker } from "./release-owner-picker";

type Choice = ReleaseBump | "custom";

const NO_REPOSITORIES: Repository[] = [];

/**
 * Plan release and Edit release (the `plan-release` and `edit-release` spec
 * deltas). Planning adds a release at the end of the repository's line;
 * editing changes one that is on it. The version follows the release before
 * it by a bump, or is set by hand; the dialog previews the line with the
 * client port of the rules, and the server decides on save.
 */
export function ReleaseDialog({
  onClose,
  data,
}: {
  onClose: () => void;
  data: Record<string, unknown> | null;
}) {
  const repositoryId = typeof data?.repositoryId === "string" ? data.repositoryId : "";
  const releaseId = typeof data?.releaseId === "string" ? data.releaseId : null;
  const worklaneId = typeof data?.worklaneId === "string" ? data.worklaneId : null;
  const wsId = useWorkspaceId();
  const { data: repositories = NO_REPOSITORIES } = useQuery(repositoryListOptions(wsId));
  const repository = repositories.find((r) => r.id === repositoryId);
  const { releases, line, isLoading } = useReleaseLine(repositoryId, repository?.last_tag ?? null);
  const editing = releases.find((r) => r.id === releaseId);

  if (!repository || isLoading || (releaseId !== null && !editing)) {
    return (
      <DialogFrame onClose={onClose} title={null}>
        {!isLoading && repository && releaseId !== null ? <NotFound onClose={onClose} /> : null}
      </DialogFrame>
    );
  }

  return (
    <ReleaseForm
      key={editing?.id ?? "plan"}
      repository={repository}
      line={line}
      release={editing ?? null}
      worklaneId={worklaneId}
      onClose={onClose}
    />
  );
}

function DialogFrame({
  onClose,
  title,
  children,
}: {
  onClose: () => void;
  title: string | null;
  children: React.ReactNode;
}) {
  return (
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-lg">
        {title !== null ? (
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
          </DialogHeader>
        ) : null}
        {children}
      </DialogContent>
    </Dialog>
  );
}

function NotFound({ onClose }: { onClose: () => void }) {
  const { t } = useT("releases");
  return (
    <>
      <DialogHeader>
        <DialogTitle>{t(($) => $.menu.edit)}</DialogTitle>
        <DialogDescription>{t(($) => $.dialog.not_found)}</DialogDescription>
      </DialogHeader>
      <div className="flex justify-end">
        <Button variant="ghost" onClick={onClose}>
          {t(($) => $.dialog.cancel)}
        </Button>
      </div>
    </>
  );
}


function ReleaseForm({
  repository,
  line,
  release,
  worklaneId,
  onClose,
}: {
  repository: Repository;
  line: LinePreview["line"];
  release: Release | null;
  /** Plan release from a worklane's Timeline: its issues in no release to tick. */
  worklaneId: string | null;
  onClose: () => void;
}) {
  const { t } = useT("releases");
  const versionId = useId();
  const customId = useId();
  const customMessageId = useId();
  const titleId = useId();
  const descriptionId = useId();
  const ownerId = useId();
  const planRelease = usePlanRelease();
  const editRelease = useEditRelease();
  const lastTag = useLastTag(repository);
  const shipped = release?.shipped_at != null;

  const [choice, setChoice] = useState<Choice>(release ? (release.bump ?? "custom") : "patch");
  const [issueIds, setIssueIds] = useState<string[]>([]);
  const [custom, setCustom] = useState(release && release.bump === null ? formatVersion(release.version) : "");
  const [title, setTitle] = useState(release?.title ?? "");
  const [description, setDescription] = useState(release?.description ?? "");
  const [refusal, setRefusal] = useState<string | null>(null);
  const [owner, setOwner] = useState<ReleaseOwnerRequest>(
    release ? { owner_type: release.owner_type, owner_id: release.owner_id } : NO_OWNER,
  );
  // The server's name for the release's own owner, while the directory may still be loading.
  const ownerName = release && owner.owner_id === release.owner_id ? release.owner_name : null;

  const targetId = release?.id ?? PLANNED_RELEASE_ID;
  const index = release ? line.releases.findIndex((r) => r.id === release.id) : line.releases.length;
  const options = bumpOptions(line, index);
  const before = versionBefore(line, index);
  const parsed = choice === "custom" ? parseVersion(custom) : null;
  const version = shipped
    ? release.version
    : choice === "custom"
      ? parsed
        ? versionString(parsed)
        : ""
      : (options.find((o) => o.bump === choice)?.version ?? "");

  const draft: ReleaseDraft = {
    title: title.trim(),
    bump: choice === "custom" ? null : choice,
    version,
  };
  const preview = version ? (release ? previewEdit(line, release.id, draft) : previewPlan(line, draft)) : null;
  const versionError =
    choice === "custom" && !parsed
      ? custom.trim()
        ? t(($) => $.dialog.error_form)
        : null
      : preview?.problem
        ? problemMessage(t, preview, targetId)
        : null;
  const renumbered = preview && !preview.problem
    ? preview.changes.filter((change) => change.release_id !== targetId)
    : [];

  const hint =
    shipped ? t(($) => $.refusal.release_shipped) : choice === "custom"
      ? before.kind === "none"
        ? t(($) => $.dialog.hint_set_by_hand_first)
        : t(($) => $.dialog.hint_set_by_hand)
      : before.kind === "release"
        ? t(($) => $.dialog.hint_follows_release, {
            version: formatVersion(before.release.version),
            title: before.release.title,
          })
        : before.kind === "tag"
          ? t(($) => $.dialog.hint_follows_tag, { tag: formatVersion(before.tag) })
          : t(($) => $.dialog.hint_first);

  const pending = planRelease.isPending || editRelease.isPending;
  const finding = lastTag === "finding";
  const ready = !!title.trim() && !!version && !versionError && !pending && !finding;
  const shownVersion = version ? formatVersion(version) : "";

  const handleSubmit = async () => {
    if (!ready) return;
    setRefusal(null);
    const fields = { title: title.trim(), description: description.trim() };
    const versioned = choice === "custom" ? { ...fields, version } : { ...fields, bump: choice };
    // Plan sends an owner only when one is picked; Edit only when it changed, so an
    // unchanged owner is kept as it is on the server.
    const ownerChanged = release
      ? !shipped && (owner.owner_type !== release.owner_type || owner.owner_id !== release.owner_id)
      : owner.owner_id !== null;
    const body = ownerChanged ? { ...versioned, ...owner } : versioned;
    try {
      if (release) {
        await editRelease.mutateAsync({ id: release.id, repositoryId: repository.id, data: body });
        toast.success(t(($) => $.dialog.toast_saved, { version: shownVersion, title: fields.title }));
      } else {
        await planRelease.mutateAsync({
          repositoryId: repository.id,
          data: issueIds.length > 0 ? { ...body, issue_ids: issueIds } : body,
        });
        toast.success(t(($) => $.dialog.toast_planned, { version: shownVersion, title: fields.title }));
      }
      onClose();
    } catch (err) {
      const answer = releaseRefusalOf(err);
      setRefusal(
        answer
          ? refusalMessage(t, answer, repository.full_name)
          : (clientErrorMessage(err) ?? t(($) => $.dialog.failed)),
      );
    }
  };

  const choices: Choice[] = ["patch", "minor", "major", "custom"];

  return (
    <DialogFrame
      onClose={onClose}
      title={
        release
          ? t(($) => $.dialog.edit_title, { repository: repository.full_name })
          : t(($) => $.dialog.plan_title, { repository: repository.full_name })
      }
    >
      <div className="space-y-4">
        <div className="space-y-1.5">
          <Label id={versionId} className="text-caption text-muted-foreground">
            {t(($) => $.dialog.version_label)}
          </Label>
          <fieldset disabled={shipped} role="radiogroup" aria-labelledby={versionId} className="grid grid-cols-4 gap-2 disabled:opacity-60">
            {choices.map((value) => {
              const selected = value === choice;
              const option = options.find((o) => o.bump === value);
              return (
                <button
                  key={value}
                  type="button"
                  role="radio"
                  aria-checked={selected}
                  onClick={() => {
                    setChoice(value);
                    setRefusal(null);
                  }}
                  className={cn(
                    "rounded-md border px-3 py-2 text-left transition-colors hover:bg-accent/60 focus-visible:outline-2 focus-visible:outline-ring",
                    selected
                      ? versionError
                        ? "border-destructive bg-accent"
                        : "border-ring bg-accent"
                      : "border-border bg-background",
                  )}
                >
                  <span className="flex items-center justify-between text-label font-medium">
                    {t(($) => $.dialog.bump[value])}
                    {selected ? (
                      <CircleDot className="size-3.5" aria-hidden="true" />
                    ) : (
                      <Circle className="size-3.5 text-muted-foreground" aria-hidden="true" />
                    )}
                  </span>
                  <span
                    className={cn(
                      "mt-1 block",
                      option ? "font-mono text-body-lg font-semibold" : "text-label text-muted-foreground",
                    )}
                  >
                    {option ? formatVersion(option.version) : t(($) => $.dialog.custom_note)}
                  </span>
                </button>
              );
            })}
          </fieldset>
          {choice === "custom" ? (
            <div className="space-y-1 pt-1">
              <Label htmlFor={customId} className="sr-only">
                {t(($) => $.dialog.custom_label)}
              </Label>
              <Input
                id={customId}
                autoFocus
                readOnly={shipped}
                value={custom}
                onChange={(e) => {
                  setCustom(e.target.value);
                  setRefusal(null);
                }}
                placeholder={formatVersion(options.find((o) => o.bump === "minor")?.version ?? "0.1.0")}
                className="font-mono"
                aria-label={t(($) => $.dialog.custom_label)}
                aria-invalid={versionError ? true : undefined}
                aria-describedby={customMessageId}
              />
            </div>
          ) : null}
          <div id={customMessageId} className="space-y-0.5">
            {versionError ? (
              <p className="text-caption text-destructive">{versionError}</p>
            ) : (
              <p className="text-caption text-muted-foreground">{hint}</p>
            )}
            {renumbered.length > 0 ? (
              <p className="text-caption text-muted-foreground">
                {t(($) => $.dialog.renumber, {
                  changes: renumbered
                    .map((change) =>
                      t(($) => $.dialog.renumber_change, {
                        from: formatVersion(change.from),
                        to: formatVersion(change.to),
                      }),
                    )
                    .join(", "),
                })}
              </p>
            ) : null}
          </div>
        </div>

        <div className="space-y-1">
          <div className="flex items-baseline justify-between">
            <Label htmlFor={titleId} className="text-caption text-muted-foreground">
              {t(($) => $.dialog.title_label)}
            </Label>
            <span className="text-caption text-muted-foreground">{t(($) => $.dialog.required)}</span>
          </div>
          <Input
            id={titleId}
            autoFocus={choice !== "custom"}
            value={title}
            onChange={(e) => {
              setTitle(e.target.value);
              setRefusal(null);
            }}
            placeholder={t(($) => $.dialog.title_placeholder)}
          />
        </div>

        <div className="space-y-1">
          <div className="flex items-baseline justify-between">
            <Label htmlFor={descriptionId} className="text-caption text-muted-foreground">
              {t(($) => $.dialog.description_label)}
            </Label>
            <span className="text-caption text-muted-foreground">{t(($) => $.dialog.optional)}</span>
          </div>
          <Textarea
            id={descriptionId}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder={t(($) => $.dialog.description_placeholder)}
            rows={3}
          />
        </div>

        <div className="space-y-1">
          <div className="flex items-baseline justify-between">
            <Label id={ownerId} className="text-caption text-muted-foreground">
              {t(($) => $.owner.label)}
            </Label>
            <span className="text-caption text-muted-foreground">{t(($) => $.dialog.optional)}</span>
          </div>
          {shipped ? (
            <div aria-labelledby={ownerId} role="group" className="flex h-9 items-center px-3 text-body">
              <ReleaseOwnerLabel ownerType={release.owner_type} ownerId={release.owner_id} ownerName={release.owner_name} />
            </div>
          ) : (
            <ReleaseOwnerPicker
              ownerType={owner.owner_type}
              ownerId={owner.owner_id}
              ownerName={ownerName}
              labelledBy={ownerId}
              variant="field"
              disabled={pending}
              onChange={(next) => {
                setOwner(next);
                setRefusal(null);
              }}
            />
          )}
        </div>

        {!release && worklaneId ? (
          <ReleaseIssuesField worklaneId={worklaneId} selected={issueIds} onChange={setIssueIds} showLater />
        ) : null}
        {release ? <ReleaseMilestones release={release} /> : null}

        <div className="flex gap-2 rounded-md border border-border bg-muted/40 px-3 py-2 text-caption text-muted-foreground">
          <Rocket className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
          <p>
            {t(($) => $.dialog.ships, {
              version: shownVersion || t(($) => $.dialog.its_version),
              branch: repository.default_branch,
            })}
          </p>
        </div>

        {finding ? (
          <p role="status" className="text-caption text-muted-foreground">
            {t(($) => $.dialog.finding_tag)}
          </p>
        ) : null}
        {refusal ? (
          <p role="alert" className="text-caption text-destructive">
            {refusal}
          </p>
        ) : null}
      </div>

      <div className="flex items-center justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>
          {t(($) => $.dialog.cancel)}
        </Button>
        <Button onClick={() => void handleSubmit()} disabled={!ready}>
          <Check className="size-4" aria-hidden="true" />
          {pending
            ? t(($) => $.dialog.saving)
            : release
              ? t(($) => $.dialog.submit_save)
              : shownVersion
                ? t(($) => $.dialog.submit_plan, { version: shownVersion })
                : t(($) => $.dialog.submit_plan_empty)}
        </Button>
      </div>
    </DialogFrame>
  );
}
