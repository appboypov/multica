"use client";

import { useId, useMemo, useState } from "react";
import { Building2, FolderGit2, FolderKanban, GitBranch, Plus } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { clientErrorMessage } from "@multica/core/api";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceId } from "@multica/core/hooks";
import { useModalStore } from "@multica/core/modals";
import {
  useOrganisation,
  useOrganisations,
  useUpdateOrganisationKey,
} from "@multica/core/organisations";
import { useWorkspacePaths } from "@multica/core/paths";
import { pinListOptions } from "@multica/core/pins";
import { projectListOptions, sortProjects, useProjectViewStore } from "@multica/core/projects";
import { repositoryListOptions } from "@multica/core/repositories";
import type { MemberWithUser, Organisation } from "@multica/core/types";
import { memberListOptions } from "@multica/core/workspace/queries";
import { Input } from "@multica/ui/components/ui/input";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { cn } from "@multica/ui/lib/utils";
import { BreadcrumbHeader } from "../../layout/breadcrumb-header";
import {
  CollectionPageHeaderAction,
  CollectionPageState,
} from "../../layout/collection-page";
import { PAGE_GUTTER } from "../../layout/page-header";
import { useNavigation } from "../../navigation";
import { useT } from "../../i18n";
import { ProjectBatchToolbar, ProjectTable } from "../../projects/components/project-table";
import { useWorklaneCounts } from "../../worklanes/use-worklane-counts";
import { organisationKeyProblem, type OrganisationKeyProblem } from "../validation";

/**
 * An organisation's page: its name, its number of projects and its key, which
 * can change here, then its projects in the table Projects uses, without the
 * group rows. New project starts in this organisation.
 */
export function OrganisationPage() {
  const { t } = useT("organisations");
  const { t: tWorklanes } = useT("worklanes");
  const worklaneCounts = useWorklaneCounts();
  const { t: tRepositories } = useT("repositories");
  const wsId = useWorkspaceId();
  const { data: repositories = [] } = useQuery(repositoryListOptions(wsId));
  const paths = useWorkspacePaths();
  const { pathname } = useNavigation();
  const organisationId = pathname.split("/").pop() ?? "";
  const currentUser = useAuthStore((s) => s.user);

  const { data: organisation, isLoading } = useOrganisation(wsId, organisationId);
  const { data: organisations = [] } = useOrganisations(wsId);
  const { data: projects = [] } = useQuery(projectListOptions(wsId));
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const { data: pins = [] } = useQuery({
    ...pinListOptions(wsId, currentUser?.id ?? ""),
    enabled: !!wsId && !!currentUser?.id,
  });
  const sortField = useProjectViewStore((s) => s.sortField);
  const sortDirection = useProjectViewStore((s) => s.sortDirection);
  const [selectedIds, setSelectedIds] = useState<ReadonlySet<string>>(new Set());

  const isWorkspaceAdmin = useMemo(() => {
    if (!currentUser) return false;
    const me = members.find((m: MemberWithUser) => m.user_id === currentUser.id);
    return me?.role === "owner" || me?.role === "admin";
  }, [members, currentUser]);

  const pinnedProjectIds = useMemo(() => {
    const ids = new Set<string>();
    for (const pin of pins) if (pin.item_type === "project") ids.add(pin.item_id);
    return ids;
  }, [pins]);

  const organisationProjects = useMemo(
    () =>
      sortProjects(
        projects.filter((project) => project.organisation_id === organisationId),
        sortField,
        sortDirection,
      ),
    [projects, organisationId, sortField, sortDirection],
  );
  const repositoryCount = useMemo(() => {
    const ids = new Set(organisationProjects.map((project) => project.id));
    return repositories.filter((repository) => ids.has(repository.project_id)).length;
  }, [repositories, organisationProjects]);
  const selectedProjects = organisationProjects.filter((p) => selectedIds.has(p.id));

  const breadcrumb = [{ href: paths.organisations(), label: t(($) => $.page.title) }];

  if (isLoading) {
    return (
      <div className="flex flex-1 min-h-0 flex-col">
        <BreadcrumbHeader segments={breadcrumb} leaf={<Skeleton className="h-4 w-32" />} />
        <div className={cn("flex items-center gap-4 py-6", PAGE_GUTTER)}>
          <Skeleton className="size-12 rounded-lg" />
          <div className="space-y-2">
            <Skeleton className="h-5 w-48" />
            <Skeleton className="h-3 w-32" />
          </div>
        </div>
      </div>
    );
  }

  // A 404 leaves the query without data; a malformed row parses to the empty
  // organisation, which has no id.
  if (!organisation?.id) {
    return (
      <div className="flex flex-1 min-h-0 flex-col">
        <BreadcrumbHeader
          segments={breadcrumb}
          leaf={
            <span className="truncate text-muted-foreground">
              {t(($) => $.detail.not_found)}
            </span>
          }
        />
        <CollectionPageState
          icon={Building2}
          title={t(($) => $.detail.not_found)}
          description={t(($) => $.detail.not_found_description)}
        />
      </div>
    );
  }

  return (
    // relative: positioning anchor for the page-centered batch toolbar.
    <div className="relative flex flex-1 min-h-0 flex-col">
      <BreadcrumbHeader
        segments={breadcrumb}
        leaf={
          <span className="truncate font-medium text-foreground">{organisation.name}</span>
        }
        actions={
          <CollectionPageHeaderAction
            icon={Plus}
            label={t(($) => $.detail.new_project)}
            onClick={() =>
              useModalStore
                .getState()
                .open("create-project", { organisationId: organisation.id })
            }
          />
        }
      />

      <div className={cn("flex shrink-0 items-start gap-4 py-6", PAGE_GUTTER)}>
        <span className="flex size-12 shrink-0 items-center justify-center rounded-lg border bg-muted/40 text-muted-foreground">
          <Building2 aria-hidden="true" className="size-5" />
        </span>
        <div className="min-w-0 flex-1 space-y-1.5">
          <h1 className="truncate text-title-sm font-semibold">{organisation.name}</h1>
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5 text-caption text-muted-foreground">
            <span className="inline-flex items-center gap-1.5">
              <FolderKanban aria-hidden="true" className="size-3.5" />
              {t(($) => $.project_count, { count: organisationProjects.length })}
            </span>
            <span className="inline-flex items-center gap-1.5">
              <FolderGit2 aria-hidden="true" className="size-3.5" />
              {tRepositories(($) => $.count, { count: repositoryCount })}
            </span>
            <span className="inline-flex items-center gap-1.5">
              <GitBranch aria-hidden="true" className="size-3.5" />
              {tWorklanes(($) => $.worklane_count, {
                count: worklaneCounts.byOrganisation.get(organisation.id) ?? 0,
              })}
            </span>
            <OrganisationKeyField
              key={organisation.key}
              organisation={organisation}
              organisations={organisations}
            />
          </div>
        </div>
      </div>

      {organisationProjects.length === 0 ? (
        <CollectionPageState icon={FolderKanban} title={t(($) => $.detail.no_projects)} />
      ) : (
        <ProjectTable
          projects={organisationProjects}
          pinnedIds={pinnedProjectIds}
          canDelete={isWorkspaceAdmin}
          selectedIds={selectedIds}
          onSelectedIdsChange={setSelectedIds}
        />
      )}

      <ProjectBatchToolbar
        rows={selectedProjects}
        pinnedIds={pinnedProjectIds}
        canDelete={isWorkspaceAdmin}
        onClear={() => setSelectedIds(new Set())}
      />
    </div>
  );
}

/**
 * The key as an inline field. Enter saves under New organisation's rules,
 * checked first against the loaded list; Escape restores the saved key. A
 * wrong or taken key shows its message beside the field and the saved key
 * stays. Remounted by its parent whenever the saved key changes.
 */
function OrganisationKeyField({
  organisation,
  organisations,
}: {
  organisation: Organisation;
  organisations: readonly Organisation[];
}) {
  const { t } = useT("organisations");
  const updateKey = useUpdateOrganisationKey();
  const inputId = useId();
  const messageId = useId();
  const [draft, setDraft] = useState(organisation.key);
  const [problem, setProblem] = useState<OrganisationKeyProblem | null>(null);
  const [serverError, setServerError] = useState<string | null>(null);

  const save = async () => {
    if (updateKey.isPending || draft === organisation.key) return;
    const found = organisationKeyProblem(draft, organisations, organisation.id);
    setProblem(found);
    setServerError(null);
    if (found) return;
    try {
      await updateKey.mutateAsync({ id: organisation.id, key: draft });
    } catch (err) {
      setServerError(clientErrorMessage(err) ?? t(($) => $.detail.key_save_failed));
    }
  };

  const message = problem ? t(($) => $.fields.key_error[problem]) : serverError;

  return (
    <span className="inline-flex min-w-0 flex-wrap items-center gap-1.5">
      <label htmlFor={inputId}>{t(($) => $.fields.key_label)}</label>
      <Input
        id={inputId}
        value={draft}
        disabled={updateKey.isPending}
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
            setDraft(organisation.key);
            setProblem(null);
            setServerError(null);
          }
        }}
        className="h-7 w-32 font-mono text-caption"
      />
      {message ? (
        <span id={messageId} role="alert" className="text-caption text-destructive">
          {message}
        </span>
      ) : null}
    </span>
  );
}
