import React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Project } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { ProjectDetail } from "./project-detail";

const mocks = vi.hoisted(() => ({
  role: "admin",
  copyText: vi.fn(),
  deleteProject: vi.fn(),
  getShareableUrl: vi.fn((path: string) => `https://app.example${path}`),
  push: vi.fn(),
  recordVisit: vi.fn(),
  toastSuccess: vi.fn(),
  updateProject: vi.fn(),
  updateProjectAsync: vi.fn(),
  project: null as Project | null,
}));

vi.mock("@multica/ui/lib/clipboard", () => ({
  copyText: mocks.copyText,
}));

vi.mock("@multica/core/todos", () => ({
  useProjectCheckTrust: () => ({ data: [], isPending: false, isError: false }),
}));

vi.mock("@tanstack/react-query", () => ({
  queryOptions: (options: unknown) => options,
  useQuery: (options: { queryKey?: readonly unknown[] }) => {
    switch (options.queryKey?.[0]) {
      case "project-detail":
        return { data: mocks.project ?? PROJECT, isLoading: false };
      case "projects":
        return {
          data: [
            mocks.project ?? PROJECT,
            { ...PROJECT, id: "project-2", organisation_id: "org-das", slug: "site" },
          ],
          isLoading: false,
        };
      case "members":
        return {
          data: [{ user_id: "user-1", name: "User One", role: mocks.role }],
          isLoading: false,
        };
      case "agents":
      case "pins":
        return { data: [], isLoading: false };
      default:
        return { data: undefined, isLoading: false };
    }
  },
}));

vi.mock("@multica/core/projects/queries", () => ({
  projectDetailOptions: () => ({ queryKey: ["project-detail"] }),
  projectListOptions: () => ({ queryKey: ["projects"] }),
}));

vi.mock("@multica/core/projects/mutations", () => ({
  useUpdateProject: () => ({ mutate: mocks.updateProject, mutateAsync: mocks.updateProjectAsync }),
  useDeleteProject: () => ({ mutate: mocks.deleteProject }),
}));

vi.mock("@multica/core/organisations", () => ({
  useOrganisations: () => ({
    data: [
      { id: "org-das", workspace_id: "workspace-1", name: "De App Specialist", key: "das", created_at: "", updated_at: "" },
      { id: "org-rebl", workspace_id: "workspace-1", name: "Rebl Records", key: "rebl", created_at: "", updated_at: "" },
    ],
  }),
}));

vi.mock("@multica/core/pins", () => ({
  pinListOptions: () => ({ queryKey: ["pins"] }),
  useCreatePin: () => ({ mutate: vi.fn() }),
  useDeletePin: () => ({ mutate: vi.fn() }),
}));

vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({ queryKey: ["members"] }),
  agentListOptions: () => ({ queryKey: ["agents"] }),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: (selector: (state: { user: { id: string } }) => unknown) =>
    selector({ user: { id: "user-1" } }),
}));

vi.mock("@multica/core/chat", () => ({
  useRecentContextStore: (
    selector: (state: { recordVisit: typeof mocks.recordVisit }) => unknown,
  ) => selector({ recordVisit: mocks.recordVisit }),
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    projects: () => "/test-workspace/projects",
  }),
}));

vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({ getActorName: () => "User One" }),
}));

vi.mock("sonner", () => ({
  toast: { success: mocks.toastSuccess },
}));

vi.mock("react-resizable-panels", () => ({
  useDefaultLayout: () => ({
    defaultLayout: undefined,
    onLayoutChanged: vi.fn(),
  }),
  usePanelRef: () => ({
    current: {
      isCollapsed: () => false,
      expand: vi.fn(),
      collapse: vi.fn(),
    },
  }),
}));

vi.mock("@multica/ui/hooks/use-mobile", () => ({
  useIsMobile: () => false,
}));

vi.mock("@multica/ui/components/common/emoji-picker", () => ({
  EmojiPicker: () => null,
}));

vi.mock("@multica/ui/components/ui/resizable", () => ({
  ResizablePanelGroup: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  ResizablePanel: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  ResizableHandle: () => null,
}));

vi.mock("@multica/ui/components/ui/dropdown-menu", () => ({
  DropdownMenu: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuTrigger: ({ render }: { render: React.ReactNode }) => <>{render}</>,
  DropdownMenuContent: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  DropdownMenuItem: ({
    children,
    onClick,
  }: {
    children: React.ReactNode;
    onClick?: () => void;
  }) => (
    <button type="button" onClick={onClick}>
      {children}
    </button>
  ),
  DropdownMenuSeparator: () => <hr />,
}));

vi.mock("@multica/ui/components/ui/popover", () => ({
  Popover: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  PopoverTrigger: ({ render }: { render: React.ReactNode }) => <>{render}</>,
  PopoverContent: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
}));

vi.mock("@multica/ui/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ render }: { render: React.ReactNode }) => <>{render}</>,
  TooltipContent: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
}));

vi.mock("@multica/ui/components/ui/sheet", () => ({
  Sheet: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  SheetContent: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
}));

vi.mock("@multica/ui/components/ui/alert-dialog", () => ({
  AlertDialog: ({
    open,
    children,
  }: {
    open: boolean;
    children: React.ReactNode;
  }) => (open ? <div role="alertdialog">{children}</div> : null),
  AlertDialogContent: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  AlertDialogHeader: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  AlertDialogTitle: ({ children }: { children: React.ReactNode }) => (
    <h2>{children}</h2>
  ),
  AlertDialogDescription: ({ children }: { children: React.ReactNode }) => (
    <p>{children}</p>
  ),
  AlertDialogFooter: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  AlertDialogCancel: ({ children }: { children: React.ReactNode }) => (
    <button type="button">{children}</button>
  ),
  AlertDialogAction: ({
    children,
    onClick,
  }: {
    children: React.ReactNode;
    onClick?: () => void;
  }) => (
    <button type="button" onClick={onClick}>
      {children}
    </button>
  ),
}));

vi.mock("../../editor", () => ({
  TitleEditor: ({ defaultValue }: { defaultValue: string }) => (
    <div>{defaultValue}</div>
  ),
  ContentEditor: () => null,
}));

vi.mock("../../common/actor-avatar", () => ({
  ActorAvatar: () => null,
}));

vi.mock("../../issues/components/priority-icon", () => ({
  PriorityIcon: () => null,
}));

vi.mock("../../issues/surface/issue-surface", () => ({
  IssueSurface: () => null,
}));

vi.mock("../../layout/breadcrumb-header", () => ({
  BreadcrumbHeader: ({ actions }: { actions: React.ReactNode }) => (
    <header>{actions}</header>
  ),
}));

vi.mock("../../layout/animated-right-sidebar", () => ({
  AnimatedRightSidebar: ({ children }: { children: React.ReactNode }) => (
    <aside>{children}</aside>
  ),
  getAnimatedRightSidebarInitialOpen: () => true,
  rightSidebarPanelMotionProps: {},
  useRightSidebarShortcut: vi.fn(),
  useAnimatedRightSidebarState: () => ({
    open: true,
    visualOpen: true,
    motionEnabled: false,
    beginToggle: vi.fn(),
    handleResize: vi.fn(),
  }),
}));

const PROJECT: Project = {
  id: "project-1",
  workspace_id: "workspace-1",
  title: "Launch Plan",
  description: null,
  icon: null,
  status: "in_progress",
  priority: "high",
  lead_type: null,
  lead_id: null,
  organisation_id: null,
  slug: null,
  start_date: null,
  due_date: null,
  created_at: "2026-06-01T00:00:00Z",
  updated_at: "2026-06-01T00:00:00Z",
  issue_count: 3,
  done_count: 1,
};

function renderProjectDetail() {
  const adapter: NavigationAdapter = {
    push: mocks.push,
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/test-workspace/projects/project-1",
    searchParams: new URLSearchParams(),
    hash: "",
    getShareableUrl: mocks.getShareableUrl,
  };

  renderWithI18n(
    <NavigationProvider value={adapter}>
      <ProjectDetail projectId={PROJECT.id} />
    </NavigationProvider>,
  );
}

beforeEach(() => {
  mocks.role = "admin";
  mocks.copyText.mockReset().mockResolvedValue(true);
  mocks.deleteProject.mockReset();
  mocks.getShareableUrl.mockClear();
  mocks.push.mockReset();
  mocks.recordVisit.mockReset();
  mocks.toastSuccess.mockReset();
  mocks.updateProject.mockReset();
  mocks.updateProjectAsync.mockReset().mockResolvedValue(undefined);
  mocks.project = null;
});

describe("ProjectDetail sharing", () => {
  it("shows the project check record", () => {
    renderProjectDetail();
    expect(screen.getByRole("region", { name: "Project check record" })).toBeInTheDocument();
  });
  it("copies the platform shareable URL instead of the renderer URL", async () => {
    const user = userEvent.setup();
    renderProjectDetail();

    await user.click(screen.getByRole("button", { name: "Copy link" }));

    expect(mocks.getShareableUrl).toHaveBeenCalledWith(
      "/test-workspace/projects/project-1",
    );
    expect(mocks.copyText).toHaveBeenCalledWith(
      "https://app.example/test-workspace/projects/project-1",
    );
  });
});

describe("ProjectDetail project deletion", () => {
  it("requires confirmation and navigates only after deletion succeeds", async () => {
    const user = userEvent.setup();
    renderProjectDetail();

    await user.click(screen.getByRole("button", { name: "Delete project" }));

    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(mocks.deleteProject).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Delete" }));

    expect(mocks.deleteProject).toHaveBeenCalledWith(
      PROJECT.id,
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    );
    expect(mocks.push).not.toHaveBeenCalled();

    const options = mocks.deleteProject.mock.calls[0]?.[1] as {
      onSuccess: () => void;
    };
    options.onSuccess();

    expect(mocks.toastSuccess).toHaveBeenCalledWith("Project deleted");
    expect(mocks.push).toHaveBeenCalledWith("/test-workspace/projects");
  });

  it("does not offer project deletion to regular members", () => {
    mocks.role = "member";

    renderProjectDetail();

    expect(
      screen.queryByRole("button", { name: "Delete project" }),
    ).not.toBeInTheDocument();
  });
});

describe("ProjectDetail organisation", () => {
  it("shows No organisation for a project without one and gives it the picked one", async () => {
    const user = userEvent.setup();
    renderProjectDetail();

    expect(screen.getByText("Organisation")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "No organisation" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "De App Specialist" }));

    expect(mocks.updateProject).toHaveBeenCalledWith({
      id: PROJECT.id,
      organisation_id: "org-das",
    });
  });

  it("shows no status, start date or due date", () => {
    renderProjectDetail();

    for (const text of ["Status", "In Progress", "Start date", "Due date"]) {
      expect(screen.queryByText(text)).not.toBeInTheDocument();
    }
  });
});

describe("ProjectDetail slug", () => {
  it("shows the saved slug and saves a new one on Enter", async () => {
    mocks.project = { ...PROJECT, organisation_id: "org-das", slug: "launch-plan" };
    const user = userEvent.setup();
    renderProjectDetail();

    const slug = screen.getByRole("textbox", { name: "Slug" });
    expect(slug).toHaveValue("launch-plan");
    await user.clear(slug);
    await user.type(slug, "launch{Enter}");

    expect(mocks.updateProjectAsync).toHaveBeenCalledWith({ id: PROJECT.id, slug: "launch" });
  });

  it("offers the title as a folder name and clears the slug when emptied", async () => {
    mocks.project = { ...PROJECT, slug: "launch-plan" };
    const user = userEvent.setup();
    renderProjectDetail();

    const slug = screen.getByRole("textbox", { name: "Slug" });
    expect(slug).toHaveAttribute("placeholder", "launch-plan");
    await user.clear(slug);
    await user.keyboard("{Enter}");

    expect(mocks.updateProjectAsync).toHaveBeenCalledWith({ id: PROJECT.id, slug: null });
  });

  it("explains a wrong or taken slug without saving and restores it on Escape", async () => {
    mocks.project = { ...PROJECT, organisation_id: "org-das", slug: "launch-plan" };
    const user = userEvent.setup();
    renderProjectDetail();

    const slug = screen.getByRole("textbox", { name: "Slug" });
    await user.clear(slug);
    await user.type(slug, "Launch{Enter}");
    expect(screen.getByRole("alert")).toHaveTextContent(
      "A slug is lowercase letters, digits and dashes, starting with a letter or digit",
    );

    await user.clear(slug);
    await user.type(slug, "site{Enter}");
    expect(screen.getByRole("alert")).toHaveTextContent("Another project of this organisation has this slug");
    expect(mocks.updateProjectAsync).not.toHaveBeenCalled();

    await user.keyboard("{Escape}");
    expect(slug).toHaveValue("launch-plan");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
