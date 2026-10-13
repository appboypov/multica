import type { ProjectPriority } from "../types";
import { createDraftStore } from "../drafts/create-draft-store";

interface ProjectDraft {
  title: string;
  description: string;
  organisationId?: string;
  priority: ProjectPriority;
  leadType?: "member" | "agent";
  leadId?: string;
  icon?: string;
}

const EMPTY_DRAFT: ProjectDraft = {
  title: "",
  description: "",
  priority: "none",
  leadType: undefined,
  leadId: undefined,
  icon: undefined,
  organisationId: undefined,
};

export const useProjectDraftStore = createDraftStore<ProjectDraft>({
  storageKey: "multica_project_draft",
  emptyData: EMPTY_DRAFT,
  hasMeaningful: (d) => !!(d.title || d.description || d.organisationId),
  migrateData: (persisted) => {
    const data = (persisted ?? {}) as Partial<ProjectDraft>;
    return {
      title: data.title ?? "",
      description: data.description ?? "",
      organisationId: data.organisationId,
      priority: data.priority ?? "none",
      leadType: data.leadType,
      leadId: data.leadId,
      icon: data.icon,
    };
  },
});
