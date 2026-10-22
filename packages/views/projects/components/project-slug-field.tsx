"use client";

import { useId, useState } from "react";
import { clientErrorMessage } from "@multica/core/api";
import { useUpdateProject } from "@multica/core/projects/mutations";
import type { Project } from "@multica/core/types";
import { folderFromName } from "@multica/core/worklanes";
import { Input } from "@multica/ui/components/ui/input";
import { useT } from "../../i18n";
import { projectSlugProblem, type ProjectSlugProblem } from "../slug";

/**
 * The slug as an inline field, like an organisation's key. Enter saves, an
 * empty field clears it, checked first against the loaded projects; Escape
 * restores the saved slug. A wrong or taken slug shows its message below the
 * field and the saved slug stays. The placeholder is the title as a folder
 * name. Remount it whenever the saved slug changes.
 */
export function ProjectSlugField({
  project,
  projects,
  hasRepositories,
}: {
  project: Project;
  projects: readonly Project[];
  hasRepositories: boolean;
}) {
  const { t } = useT("projects");
  const updateProject = useUpdateProject();
  const messageId = useId();
  const saved = project.slug ?? "";
  const [draft, setDraft] = useState(saved);
  const [problem, setProblem] = useState<ProjectSlugProblem | null>(null);
  const [serverError, setServerError] = useState<string | null>(null);

  const save = async () => {
    if (updateProject.isPending || draft === saved) return;
    const found = projectSlugProblem(draft, project, projects, hasRepositories);
    setProblem(found);
    setServerError(null);
    if (found) return;
    try {
      await updateProject.mutateAsync({ id: project.id, slug: draft || null });
    } catch (err) {
      setServerError(clientErrorMessage(err) ?? t(($) => $.slug.save_failed));
    }
  };

  const restore = () => {
    setDraft(saved);
    setProblem(null);
    setServerError(null);
  };

  const message = problem ? t(($) => $.slug.error[problem]) : serverError;

  return (
    <span className="flex min-w-0 flex-1 flex-col gap-1">
      <Input
        aria-label={t(($) => $.slug.label)}
        value={draft}
        placeholder={folderFromName(project.title)}
        disabled={updateProject.isPending}
        aria-invalid={message ? true : undefined}
        aria-describedby={message ? messageId : undefined}
        onChange={(e) => {
          setDraft(e.target.value);
          setProblem(null);
          setServerError(null);
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            void save();
          } else if (e.key === "Escape") {
            e.preventDefault();
            restore();
          }
        }}
        className="h-7 w-full font-mono text-caption"
      />
      {message ? (
        <span id={messageId} role="alert" className="whitespace-normal text-caption text-destructive">
          {message}
        </span>
      ) : null}
    </span>
  );
}
