import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Organisation, Project } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { sortProjects } from "@multica/core/projects";
import { ProjectTable } from "./project-table";

const mocks = vi.hoisted(() => ({ viewStore: null as unknown as {
  setState: (state: Record<string, unknown>) => void;
  getState: () => { collapsedOrganisations: string[] };
} }));

// A real zustand store, so folding re-renders the table as the app's
// persisted view store does. vi.mock factories are hoisted above the file's
// imports, so the factory cannot use a static `zustand` import.
vi.mock("@multica/core/projects", async () => {
  const { create } = await import("zustand");
  const store = create<Record<string, unknown>>((set) => ({
    sortField: "name",
    sortDirection: "asc",
    hiddenColumns: [],
    collapsedOrganisations: [] as string[],
    expandedProjects: [] as string[],
    expandedRepositories: [] as string[],
    toggleSort: vi.fn(),
    toggleProject: vi.fn(),
    toggleRepository: vi.fn(),
    toggleOrganisation: (id: string) =>
      set((state) => {
        const collapsed = state.collapsedOrganisations as string[];
        return {
          collapsedOrganisations: collapsed.includes(id)
            ? collapsed.filter((value) => value !== id)
            : [...collapsed, id],
        };
      }),
  }));
  mocks.viewStore = store as unknown as typeof mocks.viewStore;
  return {
    sortProjects: (await vi.importActual<{ sortProjects: typeof sortProjects }>("@multica/core/projects"))
      .sortProjects,
    projectListOptions: () => ({ queryKey: ["projects"] }),
    useProjectViewStore: store,
    useUpdateProject: () => ({ mutate: vi.fn() }),
    useDeleteProject: () => ({ mutate: vi.fn() }),
  };
});

// Repository and worklane rows have their own tests; here no project has any.
vi.mock("@tanstack/react-query", () => ({
  queryOptions: (options: unknown) => options,
  useQuery: () => ({ data: undefined, isLoading: false }),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

vi.mock("@multica/core/pins", () => ({
  useCreatePin: () => ({ mutate: vi.fn() }),
  useDeletePin: () => ({ mutate: vi.fn() }),
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    projectDetail: (id: string) => `/acme/projects/${id}`,
    organisationDetail: (id: string) => `/acme/organisations/${id}`,
  }),
}));

vi.mock("./project-lead-picker", () => ({
  ProjectLeadPicker: () => <span />,
}));

function organisation(id: string, name: string): Organisation {
  return { id, workspace_id: "ws-1", name, key: id, created_at: "", updated_at: "" };
}

function project(id: string, title: string, overrides: Partial<Project> = {}): Project {
  return {
    id,
    workspace_id: "ws-1",
    title,
    description: null,
    icon: null,
    status: "planned",
    priority: "none",
    lead_type: null,
    lead_id: null,
    organisation_id: null,
    start_date: null,
    due_date: null,
    created_at: "2026-10-01T00:00:00Z",
    updated_at: "2026-10-01T00:00:00Z",
    issue_count: 0,
    done_count: 0,
    ...overrides,
  };
}

const DAS = organisation("org-das", "De App Specialist");
const REBL = organisation("org-rebl", "Rebl Records");
const MULTICA = project("p-multica", "Multica", { organisation_id: DAS.id });
const FAN_APP = project("p-fan", "Fan app", { organisation_id: REBL.id });
const REBL_APP = project("p-rebl", "Rebl", { organisation_id: REBL.id });
const OVERVIEW = project("p-overview", "das-overview");
const PROJECTS = [MULTICA, FAN_APP, REBL_APP, OVERVIEW];
const GROUPS = [
  { organisation: DAS, projects: [MULTICA] },
  { organisation: REBL, projects: [FAN_APP, REBL_APP] },
  { organisation: null, projects: [OVERVIEW] },
];

const adapter: NavigationAdapter = {
  push: vi.fn(),
  replace: vi.fn(),
  back: vi.fn(),
  pathname: "/acme/projects",
  searchParams: new URLSearchParams(),
  hash: "",
  getShareableUrl: (p) => p,
};

function renderTable(
  props: Partial<React.ComponentProps<typeof ProjectTable>> = {},
) {
  const onSelectedIdsChange = vi.fn();
  renderWithI18n(
    <NavigationProvider value={adapter}>
      <ProjectTable
        projects={PROJECTS}
        pinnedIds={new Set()}
        canDelete={false}
        selectedIds={new Set()}
        onSelectedIdsChange={onSelectedIdsChange}
        {...props}
      />
    </NavigationProvider>,
  );
  return onSelectedIdsChange;
}

function groupRow(name: string) {
  const row = screen.getByText(name).closest('[role="row"]');
  if (!row) throw new Error(`group row ${name} not found`);
  return row as HTMLElement;
}

beforeEach(() => {
  mocks.viewStore.setState({ collapsedOrganisations: [] });
});

describe("ProjectTable", () => {
  it("heads each organisation's projects with its name, linked to its page, and its count", () => {
    renderTable({ groups: GROUPS });

    const das = groupRow("De App Specialist");
    expect(within(das).getByRole("link", { name: "De App Specialist" })).toHaveAttribute(
      "href",
      "/acme/organisations/org-das",
    );
    expect(within(das).getByText(/^1 project · /)).toBeInTheDocument();
    expect(within(groupRow("Rebl Records")).getByText(/^2 projects · /)).toBeInTheDocument();

    const none = groupRow("No organisation");
    expect(within(none).queryByRole("link")).not.toBeInTheDocument();
    expect(within(none).getByText(/^1 project · /)).toBeInTheDocument();
  });

  it("renders the groups in the order given, each above its projects", () => {
    renderTable({ groups: GROUPS });

    const order = [
      "De App Specialist",
      "Multica",
      "Rebl Records",
      "Fan app",
      "Rebl",
      "No organisation",
      "das-overview",
    ].map((text) => screen.getByText(text));
    for (let i = 1; i < order.length; i++) {
      expect(
        order[i - 1]!.compareDocumentPosition(order[i]!) & Node.DOCUMENT_POSITION_FOLLOWING,
      ).toBeTruthy();
    }
  });

  it("folds and unfolds a group through collapsedOrganisations", async () => {
    const user = userEvent.setup();
    renderTable({ groups: GROUPS });

    const fold = within(groupRow("Rebl Records")).getByRole("button", { expanded: true });
    await user.click(fold);

    expect(mocks.viewStore.getState().collapsedOrganisations).toEqual(["org-rebl"]);
    expect(screen.queryByText("Fan app")).not.toBeInTheDocument();
    expect(screen.queryByText("Rebl")).not.toBeInTheDocument();
    expect(screen.getByText("Multica")).toBeInTheDocument();
    expect(within(groupRow("Rebl Records")).getByText(/^2 projects · /)).toBeInTheDocument();

    await user.click(within(groupRow("Rebl Records")).getByRole("button", { expanded: false }));
    expect(mocks.viewStore.getState().collapsedOrganisations).toEqual([]);
    expect(screen.getByText("Fan app")).toBeInTheDocument();
  });

  it("folds No organisation under the key none", async () => {
    const user = userEvent.setup();
    renderTable({ groups: GROUPS });

    await user.click(within(groupRow("No organisation")).getByRole("button", { expanded: true }));

    expect(mocks.viewStore.getState().collapsedOrganisations).toEqual(["none"]);
    expect(screen.queryByText("das-overview")).not.toBeInTheDocument();
  });

  it("renders flat rows without group rows when no groups are given", () => {
    renderTable({ projects: [FAN_APP, REBL_APP] });

    expect(screen.getByText("Fan app")).toBeInTheDocument();
    expect(screen.getByText("Rebl")).toBeInTheDocument();
    expect(screen.queryByText("Rebl Records")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { expanded: true })).not.toBeInTheDocument();
  });

  it("selects every project, folded groups included, from the header", async () => {
    const user = userEvent.setup();
    mocks.viewStore.setState({ collapsedOrganisations: ["org-rebl"] });
    const onSelectedIdsChange = renderTable({ groups: GROUPS });

    const header = screen.getAllByRole("row")[0]!;
    await user.click(within(header).getByRole("button", { pressed: false }));

    expect(onSelectedIdsChange).toHaveBeenCalledWith(
      new Set(["p-multica", "p-fan", "p-rebl", "p-overview"]),
    );
  });

  it("shows no status or date column", () => {
    renderTable({ groups: GROUPS });

    const headers = screen.getAllByRole("columnheader").map((cell) => cell.textContent);
    expect(headers).not.toContain("Status");
    expect(headers.join(" ")).not.toMatch(/date/i);
  });
});

describe("sortProjects", () => {
  it("orders by priority, ties by title, without touching the input", () => {
    const input = [
      project("a", "Beta", { priority: "low" }),
      project("b", "Alpha", { priority: "urgent" }),
      project("c", "Alpha 2", { priority: "low" }),
    ];

    const sorted = sortProjects(input, "priority", "desc");

    expect(sorted.map((p) => p.id)).toEqual(["b", "c", "a"]);
    expect(input.map((p) => p.id)).toEqual(["a", "b", "c"]);
  });

  it("orders by name and by creation time in both directions", () => {
    const old = project("old", "Zulu", { created_at: "2026-01-01T00:00:00Z" });
    const recent = project("new", "Alpha", { created_at: "2026-09-01T00:00:00Z" });

    expect(sortProjects([old, recent], "name", "asc").map((p) => p.id)).toEqual(["new", "old"]);
    expect(sortProjects([old, recent], "created", "desc").map((p) => p.id)).toEqual(["new", "old"]);
    expect(sortProjects([old, recent], "created", "asc").map((p) => p.id)).toEqual(["old", "new"]);
  });

  it("orders by progress, projects without issues last when descending", () => {
    const empty = project("empty", "Empty");
    const half = project("half", "Half", { issue_count: 2, done_count: 1 });
    const done = project("done", "Done", { issue_count: 1, done_count: 1 });

    expect(sortProjects([empty, half, done], "progress", "desc").map((p) => p.id)).toEqual([
      "done",
      "half",
      "empty",
    ]);
  });
});
