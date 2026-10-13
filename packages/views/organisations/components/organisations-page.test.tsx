import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Organisation, Project } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { OrganisationsPage } from "./organisations-page";
import { OrganisationPage } from "./organisation-page";

const mocks = vi.hoisted(() => ({
  organisations: [] as Organisation[],
  projects: [] as Project[],
  organisationLoading: false,
  openModal: vi.fn(),
  updateKey: vi.fn(),
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: { queryKey?: readonly unknown[] }) => {
    switch (options.queryKey?.[0]) {
      case "projects":
        return { data: mocks.projects, isLoading: false };
      case "members":
        return { data: [{ user_id: "user-1", role: "admin" }], isLoading: false };
      default:
        return { data: [], isLoading: false };
    }
  },
}));

vi.mock("@multica/core/organisations", () => ({
  useOrganisations: () => ({ data: mocks.organisations, isLoading: false }),
  useOrganisation: (_wsId: string, id: string) => ({
    data: mocks.organisations.find((organisation) => organisation.id === id),
    isLoading: mocks.organisationLoading,
  }),
  useUpdateOrganisationKey: () => ({ mutateAsync: mocks.updateKey, isPending: false }),
}));

vi.mock("@multica/core/projects", () => ({
  projectListOptions: () => ({ queryKey: ["projects"] }),
  useProjectViewStore: (selector: (state: unknown) => unknown) =>
    selector({ sortField: "name", sortDirection: "asc" }),
}));

// The shared table has its own tests; here it lists the rows it is given.
vi.mock("../../projects/components/project-table", () => ({
  ProjectTable: ({ projects }: { projects: Project[] }) => (
    <ul aria-label="Projects table">
      {projects.map((project) => (
        <li key={project.id}>{project.title}</li>
      ))}
    </ul>
  ),
  ProjectBatchToolbar: () => null,
  sortProjects: (projects: Project[]) => projects,
}));

vi.mock("@multica/core/pins", () => ({
  pinListOptions: () => ({ queryKey: ["pins"] }),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    organisations: () => "/test-workspace/organisations",
    organisationDetail: (id: string) => `/test-workspace/organisations/${id}`,
  }),
}));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: (selector: (state: unknown) => unknown) =>
    selector({ user: { id: "user-1" } }),
}));

vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({ queryKey: ["members"] }),
}));

vi.mock("@multica/core/modals", () => ({
  useModalStore: { getState: () => ({ open: mocks.openModal }) },
}));

vi.mock("@multica/core/api", () => ({
  clientErrorMessage: (err: unknown) => (err instanceof Error ? err.message : undefined),
}));

vi.mock("../../layout/breadcrumb-header", () => ({
  BreadcrumbHeader: ({ leaf, actions }: { leaf: ReactNode; actions?: ReactNode }) => (
    <header>
      {leaf}
      {actions}
    </header>
  ),
}));

function organisation(id: string, name: string, key: string): Organisation {
  return { id, workspace_id: "workspace-1", name, key, created_at: "", updated_at: "" };
}

function project(id: string, title: string, organisationId: string | null): Project {
  return {
    id,
    workspace_id: "workspace-1",
    title,
    description: null,
    icon: null,
    status: "planned",
    priority: "none",
    lead_type: null,
    lead_id: null,
    organisation_id: organisationId,
    start_date: null,
    due_date: null,
    created_at: "",
    updated_at: "",
    issue_count: 0,
    done_count: 0,
    resource_count: 0,
  };
}

const REBL_RECORDS = organisation("org-rebl", "Rebl Records", "rebl");
const DE_APP_SPECIALIST = organisation("org-das", "De App Specialist", "das");
const BUTTLER = organisation("org-btlr", "Buttler", "btlr");

function render(ui: ReactNode, pathname: string, push = vi.fn()) {
  const adapter: NavigationAdapter = {
    push,
    replace: vi.fn(),
    back: vi.fn(),
    pathname,
    searchParams: new URLSearchParams(),
    hash: "",
    getShareableUrl: (p) => p,
  };
  renderWithI18n(<NavigationProvider value={adapter}>{ui}</NavigationProvider>);
  return push;
}

beforeEach(() => {
  mocks.organisations = [REBL_RECORDS, DE_APP_SPECIALIST, BUTTLER];
  mocks.projects = [
    project("p-rebl", "Rebl", REBL_RECORDS.id),
    ...Array.from({ length: 13 }, (_, i) => project(`p-das-${i}`, `DAS ${i}`, DE_APP_SPECIALIST.id)),
    project("p-none", "das-overview", null),
  ];
  mocks.organisationLoading = false;
  mocks.openModal.mockReset();
  mocks.updateKey.mockReset().mockResolvedValue(REBL_RECORDS);
});

describe("OrganisationsPage", () => {
  it("lists every organisation by name with its number of projects, and counts the organisations", () => {
    render(<OrganisationsPage />, "/test-workspace/organisations");

    const rows = screen.getAllByRole("row").slice(1);
    expect(rows.map((row) => row.textContent)).toEqual([
      "Buttler0 projects",
      "De App Specialist13 projects",
      "Rebl Records1 project",
    ]);
    const heading = screen.getByRole("heading", { name: "Organisations" });
    expect(heading.parentElement).toHaveTextContent("3");
  });

  it("opens an organisation's page from its row", async () => {
    const user = userEvent.setup();
    const push = render(<OrganisationsPage />, "/test-workspace/organisations");

    await user.click(screen.getByText("Rebl Records"));

    expect(push).toHaveBeenCalledWith("/test-workspace/organisations/org-rebl");
  });

  it("opens New organisation", async () => {
    const user = userEvent.setup();
    render(<OrganisationsPage />, "/test-workspace/organisations");

    await user.click(screen.getByRole("button", { name: "New organisation" }));

    expect(mocks.openModal).toHaveBeenCalledWith("create-organisation");
  });
});

describe("OrganisationPage", () => {
  it("shows the name, the project count and only its own projects", () => {
    render(<OrganisationPage />, "/test-workspace/organisations/org-rebl");

    expect(screen.getByRole("heading", { name: "Rebl Records" })).toBeInTheDocument();
    expect(screen.getByText("1 project")).toBeInTheDocument();
    const table = screen.getByRole("list", { name: "Projects table" });
    expect(table).toHaveTextContent("Rebl");
    expect(table).not.toHaveTextContent("DAS");
  });

  it("starts New project in the organisation", async () => {
    const user = userEvent.setup();
    render(<OrganisationPage />, "/test-workspace/organisations/org-rebl");

    await user.click(screen.getByRole("button", { name: "New project" }));

    expect(mocks.openModal).toHaveBeenCalledWith("create-project", { organisationId: "org-rebl" });
  });

  it("says the organisation was not found", () => {
    render(<OrganisationPage />, "/test-workspace/organisations/org-missing");

    expect(screen.getAllByText("Organisation not found").length).toBeGreaterThan(0);
  });

  it("changes the key", async () => {
    const user = userEvent.setup();
    render(<OrganisationPage />, "/test-workspace/organisations/org-rebl");

    const key = screen.getByLabelText("Key");
    await user.clear(key);
    await user.type(key, "rebl-records{Enter}");

    expect(mocks.updateKey).toHaveBeenCalledWith({ id: "org-rebl", key: "rebl-records" });
  });

  it("keeps the key and says a taken key is taken", async () => {
    const user = userEvent.setup();
    render(<OrganisationPage />, "/test-workspace/organisations/org-rebl");

    const key = screen.getByLabelText("Key");
    await user.clear(key);
    await user.type(key, "das{Enter}");

    expect(screen.getByRole("alert")).toHaveTextContent("Another organisation has this key");
    expect(mocks.updateKey).not.toHaveBeenCalled();

    await user.type(key, "{Escape}");
    expect(key).toHaveValue("rebl");
  });
});
