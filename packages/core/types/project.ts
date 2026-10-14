export type ProjectStatus = "planned" | "in_progress" | "paused" | "completed" | "cancelled";

export type ProjectPriority = "urgent" | "high" | "medium" | "low" | "none";

export interface Project {
  id: string;
  workspace_id: string;
  title: string;
  description: string | null;
  icon: string | null;
  status: ProjectStatus;
  priority: ProjectPriority;
  lead_type: "member" | "agent" | null;
  lead_id: string | null;
  // The organisation the project belongs to, or null for none. A project
  // points to at most one organisation of its workspace.
  organisation_id: string | null;
  // Calendar days ("YYYY-MM-DD"), no time-of-day or timezone — same contract as
  // issue.start_date / issue.due_date.
  start_date: string | null;
  due_date: string | null;
  created_at: string;
  updated_at: string;
  issue_count: number;
  done_count: number;
}

export interface CreateProjectRequest {
  title: string;
  description?: string;
  icon?: string;
  status?: ProjectStatus;
  priority?: ProjectPriority;
  lead_type?: "member" | "agent";
  lead_id?: string;
  organisation_id?: string;
  start_date?: string;
  due_date?: string;
}

export interface UpdateProjectRequest {
  title?: string;
  description?: string | null;
  icon?: string | null;
  status?: ProjectStatus;
  priority?: ProjectPriority;
  lead_type?: "member" | "agent" | null;
  lead_id?: string | null;
  // Omit the key to keep the organisation; send null to clear it.
  organisation_id?: string | null;
  // Omit the key to leave the date untouched; send null (or "") to clear it.
  start_date?: string | null;
  due_date?: string | null;
}

export interface ListProjectsResponse {
  projects: Project[];
  total: number;
}
