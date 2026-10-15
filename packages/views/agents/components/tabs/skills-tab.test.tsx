// @vitest-environment jsdom

import { describe, it, expect, vi, beforeEach } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Agent, AgentRuntime } from "@multica/core/types";
import { toast } from "sonner";
import { workspaceKeys } from "@multica/core/workspace/queries";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../../../locales/en/common.json";
import enAgents from "../../../locales/en/agents.json";

const TEST_RESOURCES = { en: { common: enCommon, agents: enAgents } };

const mockListSkills = vi.hoisted(() => vi.fn());
const mockGetSkill = vi.hoisted(() => vi.fn());
const mockSetAgentSkillEnabled = vi.hoisted(() => vi.fn());
const mockSetAgentRuntimeSkillEnabled = vi.hoisted(() => vi.fn());
const mockSetAgentRuntimeSkillsInherited = vi.hoisted(() => vi.fn());
const mockRemoveAgentSkill = vi.hoisted(() => vi.fn());
const mockRuntimeCapabilities = vi.hoisted(() => vi.fn());

// ApiError mirrors the production export. The tab branches on
// `instanceof ApiError` for the 403 permission notice, so the class identity
// must match the one the mocked query rejects with.
const { ApiError } = vi.hoisted(() => {
  class ApiError extends Error {
    status: number;
    statusText: string;
    constructor(message: string, status: number, statusText: string) {
      super(message);
      this.status = status;
      this.statusText = statusText;
    }
  }
  return { ApiError };
});

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/api", () => ({
  api: {
    listSkills: (...args: unknown[]) => mockListSkills(...args),
    getSkill: (...args: unknown[]) => mockGetSkill(...args),
    setAgentSkills: vi.fn(),
    setAgentSkillEnabled: (...args: unknown[]) => mockSetAgentSkillEnabled(...args),
    setAgentRuntimeSkillEnabled: (...args: unknown[]) =>
      mockSetAgentRuntimeSkillEnabled(...args),
    setAgentRuntimeSkillsInherited: (...args: unknown[]) =>
      mockSetAgentRuntimeSkillsInherited(...args),
    removeAgentSkill: (...args: unknown[]) => mockRemoveAgentSkill(...args),
  },
  ApiError,
}));

vi.mock("@multica/core/runtimes", async () => {
  const actual =
    await vi.importActual<typeof import("@multica/core/runtimes")>(
      "@multica/core/runtimes",
    );
  return {
    ...actual,
    runtimeCapabilitiesOptions: (runtimeId: string | null) => ({
      queryKey: ["runtime-capabilities", runtimeId],
      queryFn: () => mockRuntimeCapabilities(runtimeId),
      enabled: Boolean(runtimeId),
      retry: false,
    }),
  };
});

vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

import { SkillsTab } from "./skills-tab";

const agent: Agent = {
  id: "agent-1",
  workspace_id: "ws-1",
  runtime_id: "runtime-1",
  name: "Agent",
  description: "",
  instructions: "",
  avatar_url: null,
  runtime_mode: "local",
  runtime_config: {},
  custom_args: [],
  visibility: "workspace",
  permission_mode: "public_to",
  invocation_targets: [{ target_type: "workspace", target_id: null }],
  status: "idle",
  max_concurrent_tasks: 1,
  model: "",
  owner_id: "user-1",
  skills: [],
  created_at: "2026-04-16T00:00:00Z",
  updated_at: "2026-04-16T00:00:00Z",
  archived_at: null,
  archived_by: null,
};

const onlineRuntime: AgentRuntime = {
  id: "runtime-1",
  workspace_id: "ws-1",
  daemon_id: "daemon-1",
  name: "Codex (Mac)",
  runtime_mode: "local",
  provider: "codex",
  launch_header: "",
  status: "online",
  device_info: "Mac",
  metadata: {},
  owner_id: "user-1",
  visibility: "private",
  last_seen_at: null,
  created_at: "2026-07-11T00:00:00Z",
  updated_at: "2026-07-11T00:00:00Z",
};

function renderSkillsTab(
  agentOverrides: Partial<Agent> = {},
  runtime: AgentRuntime | null = null,
  currentUserId: string | null = "user-1",
  canEdit = true,
) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });

  const tab = (overrides: Partial<Agent>) => (
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      <QueryClientProvider client={queryClient}>
        <SkillsTab
          agent={{ ...agent, ...overrides }}
          runtime={runtime}
          currentUserId={currentUserId}
          canEdit={canEdit}
        />
      </QueryClientProvider>
    </I18nProvider>
  );
  const result = render(tab(agentOverrides));
  return {
    ...result,
    queryClient,
    updateAgent: (overrides: Partial<Agent>) => result.rerender(tab(overrides)),
  };
}

describe("SkillsTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListSkills.mockResolvedValue([]);
    mockSetAgentSkillEnabled.mockResolvedValue(undefined);
    mockSetAgentRuntimeSkillEnabled.mockResolvedValue(undefined);
    mockSetAgentRuntimeSkillsInherited.mockResolvedValue(undefined);
    mockRemoveAgentSkill.mockResolvedValue(undefined);
    mockRuntimeCapabilities.mockResolvedValue({
      skills: [],
      supported: true,
      mcpServers: [],
      mcpSupported: true,
    });
  });

  it("separates workspace assignments from inherited runtime skills", async () => {
    renderSkillsTab();

    expect(
      await screen.findByText("Assigned to agent"),
    ).toBeInTheDocument();
    expect(screen.getByText("Inherited from runtime")).toBeInTheDocument();
    expect(screen.getByText(/Assign a local runtime/i)).toBeInTheDocument();
  });

  it("disables an assigned skill without removing it", async () => {
    const user = userEvent.setup();
    renderSkillsTab({
      skills: [
        {
          id: "skill-1",
          name: "Review changes",
          description: "Review a patch",
          enabled: true,
        },
      ],
    });

    await user.click(screen.getByRole("switch", { name: /Toggle Review changes/i }));

    expect(mockSetAgentSkillEnabled).toHaveBeenCalledWith(
      "agent-1",
      "skill-1",
      false,
    );
    expect(mockRemoveAgentSkill).not.toHaveBeenCalled();
  });

  it("shows inherited skills discovered from the assigned runtime", async () => {
    mockRuntimeCapabilities.mockResolvedValue({
      skills: [
        {
          key: "local-review",
          name: "Local review",
          description: "Host-level review workflow",
          source_path: "~/.codex/skills/local-review",
          provider: "codex",
          root: "provider",
          file_count: 2,
        },
      ],
      supported: true,
      mcpServers: [],
      mcpSupported: true,
    });

    renderSkillsTab({}, onlineRuntime);

    expect(await screen.findByText("Local review")).toBeInTheDocument();
    expect(screen.getByText("Host-level review workflow")).toBeInTheDocument();
  });

  it("turns a controllable inherited skill off for this agent", async () => {
    const user = userEvent.setup();
    mockRuntimeCapabilities.mockResolvedValue({
      skills: [
        {
          key: "local-review",
          name: "Local review",
          source_path: "~/.codex/skills/local-review",
          provider: "codex",
          root: "provider",
          can_disable: true,
          file_count: 1,
        },
      ],
      supported: true,
      mcpServers: [],
      mcpSupported: true,
    });

    renderSkillsTab({}, onlineRuntime);
    await user.click(
      await screen.findByRole("switch", {
        name: /Toggle inherited Local review/i,
      }),
    );

    expect(mockSetAgentRuntimeSkillEnabled).toHaveBeenCalledWith("agent-1", {
      runtime_id: "runtime-1",
      root: "provider",
      key: "local-review",
      name: "Local review",
      plugin: undefined,
      enabled: false,
    });
  });

  it("renders a persisted inherited-skill override as off", async () => {
    mockRuntimeCapabilities.mockResolvedValue({
      skills: [
        {
          key: "local-review",
          name: "Local review",
          source_path: "~/.codex/skills/local-review",
          provider: "codex",
          root: "provider",
          can_disable: true,
          file_count: 1,
        },
      ],
      supported: true,
      mcpServers: [],
      mcpSupported: true,
    });

    renderSkillsTab(
      {
        disabled_runtime_skills: [
          {
            runtime_id: "runtime-1",
            provider: "codex",
            root: "provider",
            key: "local-review",
          },
        ],
      },
      onlineRuntime,
    );

    expect(
      await screen.findByRole("switch", {
        name: /Toggle inherited Local review/i,
      }),
    ).not.toBeChecked();
  });

  describe("runtime inheritance", () => {
    const localReview = {
      key: "local-review",
      name: "Local review",
      source_path: "~/.codex/skills/local-review",
      provider: "codex",
      root: "provider" as const,
      can_disable: true,
      file_count: 1,
    };
    const sectionSwitch = () =>
      screen.findByRole("switch", { name: /Inherit skills from/i });

    beforeEach(() => {
      mockRuntimeCapabilities.mockResolvedValue({
        skills: [localReview],
        supported: true,
        mcpServers: [],
        mcpSupported: true,
      });
    });

    it("turns inheritance off and refreshes the agent after saving", async () => {
      const user = userEvent.setup();
      let finishWrite!: () => void;
      mockSetAgentRuntimeSkillsInherited.mockReturnValueOnce(
        new Promise<void>((resolve) => {
          finishWrite = resolve;
        }),
      );
      const { queryClient } = renderSkillsTab(
        { inherit_runtime_skills: true },
        { ...onlineRuntime, custom_name: "My Mac" },
      );
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");
      const toggle = await sectionSwitch();

      expect(toggle).toBeChecked();
      expect(toggle).toHaveAccessibleName(/My Mac/);
      await user.click(toggle);

      expect(mockSetAgentRuntimeSkillsInherited).toHaveBeenCalledWith(
        "agent-1",
        false,
      );
      expect(toggle).toHaveAttribute("aria-disabled", "true");
      await user.click(toggle);
      expect(mockSetAgentRuntimeSkillsInherited).toHaveBeenCalledTimes(1);
      expect(invalidate).not.toHaveBeenCalled();
      finishWrite();
      await waitFor(() =>
        expect(invalidate).toHaveBeenCalledWith({
          queryKey: workspaceKeys.agents("ws-1"),
        }),
      );
      await waitFor(() =>
        expect(toggle).not.toHaveAttribute("aria-disabled", "true"),
      );
      expect(mockSetAgentRuntimeSkillEnabled).not.toHaveBeenCalled();
    });

    it("treats an omitted inheritance setting as on", async () => {
      renderSkillsTab({}, onlineRuntime);
      expect(await sectionSwitch()).toBeChecked();
    });

    it("keeps stored per-skill choices across section changes", async () => {
      const user = userEvent.setup();
      mockRuntimeCapabilities.mockResolvedValue({
        skills: [
          localReview,
          { ...localReview, key: "local-plan", name: "Local plan" },
        ],
        supported: true,
        mcpServers: [],
        mcpSupported: true,
      });
      const savedChoices = {
        disabled_runtime_skills: [
          {
            runtime_id: "runtime-1",
            provider: "codex",
            root: "provider" as const,
            key: "local-review",
          },
        ],
      };
      const { updateAgent } = renderSkillsTab(
        { ...savedChoices, inherit_runtime_skills: true },
        onlineRuntime,
      );
      const review = await screen.findByRole("switch", {
        name: /Toggle inherited Local review/i,
      });
      const plan = screen.getByRole("switch", {
        name: /Toggle inherited Local plan/i,
      });
      expect(review).not.toBeChecked();
      expect(plan).toBeChecked();

      updateAgent({ ...savedChoices, inherit_runtime_skills: false });
      expect(await sectionSwitch()).not.toBeChecked();
      for (const row of [review, plan]) {
        expect(row).not.toBeChecked();
        expect(row).toHaveAttribute("aria-disabled", "true");
        await user.click(row);
      }
      expect(mockSetAgentRuntimeSkillEnabled).not.toHaveBeenCalled();

      await user.click(await sectionSwitch());
      expect(mockSetAgentRuntimeSkillsInherited).toHaveBeenCalledWith(
        "agent-1",
        true,
      );
      updateAgent({ ...savedChoices, inherit_runtime_skills: true });
      expect(review).not.toBeChecked();
      expect(review).not.toHaveAttribute("aria-disabled", "true");
      expect(plan).toBeChecked();
      expect(plan).not.toHaveAttribute("aria-disabled", "true");
    });

    it("shows section and row switches for controllable omp skills", async () => {
      mockRuntimeCapabilities.mockResolvedValue({
        skills: [{ ...localReview, provider: "omp" }],
        supported: true,
        mcpServers: [],
        mcpSupported: true,
      });
      renderSkillsTab({}, { ...onlineRuntime, provider: "omp" });

      expect(await sectionSwitch()).toBeChecked();
      expect(
        screen.getByRole("switch", { name: /Toggle inherited Local review/i }),
      ).toBeChecked();
    });

    it("hides switches from someone who cannot edit the agent", async () => {
      renderSkillsTab({}, onlineRuntime, "user-1", false);
      await screen.findByText("Local review");
      expect(screen.queryByRole("switch")).not.toBeInTheDocument();
    });

    it("hides the section switch when discovery fails after a successful listing", async () => {
      const user = userEvent.setup();
      renderSkillsTab({}, onlineRuntime);
      await sectionSwitch();
      mockRuntimeCapabilities.mockRejectedValueOnce(new Error("offline"));
      await user.click(screen.getByRole("button", { name: "Refresh" }));

      await screen.findByText("Couldn't discover runtime skills. Try again.");
      expect(
        screen.queryByRole("switch", { name: /Inherit skills from/i }),
      ).not.toBeInTheDocument();
    });

    it.each([
      ["empty", []],
      ["unsupported provider", [{ ...localReview, provider: "other", can_disable: false }]],
      ["missing skill root", [{ ...localReview, root: undefined }]],
    ])("hides the section switch for a %s listing", async (_name, skills) => {
      mockRuntimeCapabilities.mockResolvedValue({
        skills,
        supported: true,
        mcpServers: [],
        mcpSupported: true,
      });
      renderSkillsTab({}, onlineRuntime);
      if (skills.length === 0) {
        await screen.findByText(enAgents.tab_body.skills.runtime_empty);
      } else {
        await screen.findByText("Local review");
      }
      expect(screen.queryByRole("switch")).not.toBeInTheDocument();
    });

    it("keeps a hidden off setting when a skill appears on refresh", async () => {
      const user = userEvent.setup();
      mockRuntimeCapabilities.mockResolvedValueOnce({
        skills: [],
        supported: true,
        mcpServers: [],
        mcpSupported: true,
      });
      renderSkillsTab({ inherit_runtime_skills: false }, onlineRuntime);
      await screen.findByText(enAgents.tab_body.skills.runtime_empty);
      expect(screen.queryByRole("switch")).not.toBeInTheDocument();
      await user.click(screen.getByRole("button", { name: "Refresh" }));

      expect(await sectionSwitch()).not.toBeChecked();
      const row = screen.getByRole("switch", {
        name: /Toggle inherited Local review/i,
      });
      expect(row).not.toBeChecked();
      expect(row).toHaveAttribute("aria-disabled", "true");
      await user.click(row);
      expect(mockSetAgentRuntimeSkillEnabled).not.toHaveBeenCalled();
    });

    it.each([
      ["localized fallback", null],
      ["server error", new Error("Permission denied")],
    ])("reports a failed write with its %s and leaves inheritance on", async (_name, error) => {
      const user = userEvent.setup();
      mockSetAgentRuntimeSkillsInherited.mockRejectedValueOnce(error);
      const { queryClient } = renderSkillsTab({}, onlineRuntime);
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");
      const toggle = await sectionSwitch();
      await user.click(toggle);

      await waitFor(() =>
        expect(toast.error).toHaveBeenCalledWith(
          error instanceof Error
            ? error.message
            : "Could not change skill inheritance",
        ),
      );
      expect(toggle).toBeChecked();
      expect(toggle).not.toHaveAttribute("aria-disabled", "true");
      expect(invalidate).not.toHaveBeenCalled();
    });
  });

  it("shows a permission notice when capability discovery is forbidden", async () => {
    mockRuntimeCapabilities.mockRejectedValue(
      new ApiError("insufficient permissions", 403, "Forbidden"),
    );

    renderSkillsTab({}, onlineRuntime);

    expect(
      await screen.findByText(
        "You don't have permission to view this runtime's skills.",
      ),
    ).toBeInTheDocument();
  });

  it("does not discover skills for another member's private runtime", async () => {
    renderSkillsTab({}, { ...onlineRuntime, owner_id: "user-2" }, "admin-1");

    expect(
      await screen.findByText(
        "You don't have permission to view this runtime's skills.",
      ),
    ).toBeInTheDocument();
    expect(mockRuntimeCapabilities).not.toHaveBeenCalled();
  });

  it("shows a retry notice when capability discovery fails", async () => {
    mockRuntimeCapabilities.mockRejectedValue(
      new Error("daemon did not respond within 3 minutes"),
    );

    renderSkillsTab({}, onlineRuntime);

    expect(
      await screen.findByText("Couldn't discover runtime skills. Try again."),
    ).toBeInTheDocument();
  });
});
