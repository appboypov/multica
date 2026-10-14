import React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithI18n } from "../test/i18n";

vi.mock("@tanstack/react-query", () => ({
  useQuery: () => ({ data: [] }),
  // The modal now reads the runtime list to gate worktree mode, and
  // runtimeListOptions builds its descriptor with queryOptions.
  queryOptions: (options: unknown) => options,
}));

const createProjectMock = vi.hoisted(() => vi.fn().mockResolvedValue({ id: "p1" }));
// The draft's organisation; each test starts from Rebl Records.
const draftState = vi.hoisted(() => ({ organisationId: "org-rebl" as string | undefined }));
const setDraftMock = vi.hoisted(() => vi.fn());

vi.mock("@multica/core/projects/mutations", () => ({
  useCreateProject: () => ({ mutateAsync: createProjectMock }),
}));

vi.mock("@multica/core/projects", () => ({
  useProjectDraftStore: (selector: (state: unknown) => unknown) =>
    selector({
      draft: {
        title: "",
        description: "",
        priority: "medium",
        leadType: undefined,
        leadId: undefined,
        icon: undefined,
        organisationId: draftState.organisationId,
      },
      setDraft: setDraftMock,
      clearDraft: vi.fn(),
    }),
}));

vi.mock("@multica/core/organisations", () => ({
  useOrganisations: () => ({
    data: [
      { id: "org-das", workspace_id: "workspace-1", name: "De App Specialist", key: "das", created_at: "", updated_at: "" },
      { id: "org-rebl", workspace_id: "workspace-1", name: "Rebl Records", key: "rebl", created_at: "", updated_at: "" },
    ],
  }),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

vi.mock("@multica/core/paths", () => ({
  useCurrentWorkspace: () => ({
    id: "workspace-1",
    name: "Test Workspace",
    slug: "test-workspace",
  }),
  useWorkspacePaths: () => ({
    projectDetail: (id: string) => `/test-workspace/projects/${id}`,
  }),
}));

vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({ queryKey: ["members"], queryFn: vi.fn() }),
  agentListOptions: () => ({ queryKey: ["agents"], queryFn: vi.fn() }),
}));

vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({ getActorName: vi.fn() }),
}));

vi.mock("../navigation", () => ({
  useNavigation: () => ({ push: vi.fn() }),
}));

vi.mock("../editor", () => {
  // Exposes the imperative handle the modal actually calls on submit
  // (getMarkdown). A plain textarea ref makes every submit throw, which is
  // invisible until a test clicks Create Project.
  const ContentEditor = React.forwardRef<
    { getMarkdown: () => string },
    { placeholder?: string }
  >(({ placeholder }, ref) => {
    const inner = React.useRef<HTMLTextAreaElement>(null);
    React.useImperativeHandle(ref, () => ({
      getMarkdown: () => inner.current?.value ?? "",
    }));
    return <textarea ref={inner} placeholder={placeholder} />;
  });
  ContentEditor.displayName = "ContentEditor";

  return {
    ContentEditor,
    // Wires onSubmit to Enter the way the real editor does. Without it the
    // keyboard path into handleSubmit is untestable, and that path bypasses
    // the submit button's disabled state entirely.
    TitleEditor: ({
      placeholder,
      onChange,
      onSubmit,
    }: {
      placeholder?: string;
      onChange?: (value: string) => void;
      onSubmit?: () => void;
    }) => (
      <input
        placeholder={placeholder}
        onChange={(e) => onChange?.(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            onSubmit?.();
          }
        }}
      />
    ),
  };
});

vi.mock("../issues/components/priority-icon", () => ({
  PriorityIcon: () => <span data-testid="priority-icon" />,
}));

vi.mock("../common/actor-avatar", () => ({
  ActorAvatar: () => <span data-testid="actor-avatar" />,
}));

vi.mock("@multica/ui/components/ui/dialog", () => ({
  Dialog: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

vi.mock("@multica/ui/components/ui/dropdown-menu", () => ({
  DropdownMenu: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuTrigger: ({ render }: { render: React.ReactNode }) => <>{render}</>,
  DropdownMenuContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
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
}));

vi.mock("@multica/ui/components/ui/popover", () => ({
  Popover: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  PopoverTrigger: ({ render }: { render: React.ReactNode }) => <>{render}</>,
  PopoverContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

vi.mock("@multica/ui/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ render }: { render: React.ReactNode }) => <>{render}</>,
  TooltipContent: ({ children }: { children: React.ReactNode }) => (
    <div role="tooltip">{children}</div>
  ),
}));

vi.mock("@multica/ui/components/ui/button", () => ({
  Button: ({
    children,
    disabled,
    onClick,
    type = "button",
  }: {
    children: React.ReactNode;
    disabled?: boolean;
    onClick?: () => void;
    type?: "button" | "submit" | "reset";
  }) => (
    <button type={type} disabled={disabled} onClick={onClick}>
      {children}
    </button>
  ),
}));

vi.mock("@multica/ui/components/common/emoji-picker", () => ({
  EmojiPicker: () => null,
}));

vi.mock("@multica/ui/lib/utils", () => ({
  cn: (...values: Array<string | false | null | undefined>) =>
    values.filter(Boolean).join(" "),
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

import { CreateProjectModal } from "./create-project";

describe("CreateProjectModal", () => {
  beforeEach(() => {
    draftState.organisationId = "org-rebl";
    createProjectMock.mockClear();
  });

  it("asks for no status, start date or due date", () => {
    renderWithI18n(<CreateProjectModal onClose={vi.fn()} />);

    for (const name of [/planned/i, /start date/i, /due date/i, /more options/i]) {
      expect(screen.queryByRole("button", { name })).not.toBeInTheDocument();
    }
  });

  it("sends the draft's organisation", async () => {
    const user = userEvent.setup();
    renderWithI18n(<CreateProjectModal onClose={vi.fn()} />);

    await user.type(screen.getByPlaceholderText(/project title/i), "Fan app");
    await user.click(screen.getByRole("button", { name: /^create project$/i }));

    expect(createProjectMock).toHaveBeenCalledTimes(1);
    expect(createProjectMock.mock.calls[0]?.[0]).toMatchObject({
      title: "Fan app",
      organisation_id: "org-rebl",
    });
  });

  it("starts in the organisation it is opened from", async () => {
    draftState.organisationId = undefined;
    const user = userEvent.setup();
    renderWithI18n(<CreateProjectModal onClose={vi.fn()} data={{ organisationId: "org-das" }} />);

    await user.type(screen.getByPlaceholderText(/project title/i), "Fan app");
    await user.click(screen.getByRole("button", { name: /^create project$/i }));

    expect(createProjectMock.mock.calls[0]?.[0]).toMatchObject({ organisation_id: "org-das" });
    // The draft keeps it, so a draft resumed from Projects returns to it.
    expect(setDraftMock).toHaveBeenCalledWith({ organisationId: "org-das" });
  });

  it("cannot create without an organisation, until one is picked", async () => {
    draftState.organisationId = undefined;
    const user = userEvent.setup();
    renderWithI18n(<CreateProjectModal onClose={vi.fn()} />);

    await user.type(screen.getByPlaceholderText(/project title/i), "Fan app");
    const create = screen.getByRole("button", { name: /^create project$/i });
    expect(create).toBeDisabled();
    fireEvent.keyDown(screen.getByPlaceholderText(/project title/i), { key: "Enter" });
    expect(createProjectMock).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Rebl Records" }));
    await user.click(create);

    expect(createProjectMock.mock.calls[0]?.[0]).toMatchObject({ organisation_id: "org-rebl" });
  });

  it("cannot create without a name", () => {
    renderWithI18n(<CreateProjectModal onClose={vi.fn()} />);

    expect(screen.getByRole("button", { name: /^create project$/i })).toBeDisabled();
  });
});
