// @vitest-environment jsdom

import { describe, it, expect, vi, beforeEach } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type {
  AgentRuntime,
  RuntimeLocalSkillSummary,
  SkillSummary,
  Squad,
} from "@multica/core/types";
import { toast } from "sonner";
import {
  squadDetailOptions,
  workspaceKeys,
} from "@multica/core/workspace/queries";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../../locales/en/common.json";
import enAgents from "../../locales/en/agents.json";
import enSquads from "../../locales/en/squads.json";
import type * as RuntimesModule from "@multica/core/runtimes";

const TEST_RESOURCES = {
  en: { common: enCommon, agents: enAgents, squads: enSquads },
};

const mockListSkills = vi.hoisted(() => vi.fn());
const mockGetSkill = vi.hoisted(() => vi.fn());
const mockListSquadSkills = vi.hoisted(() => vi.fn());
const mockAddSquadSkills = vi.hoisted(() => vi.fn());
const mockSetSquadSkillEnabled = vi.hoisted(() => vi.fn());
const mockRemoveSquadSkill = vi.hoisted(() => vi.fn());
const mockSetSquadRuntimeSkillEnabled = vi.hoisted(() => vi.fn());
const mockSetSquadRuntimeSkillsInherited = vi.hoisted(() => vi.fn());
const mockListRuntimes = vi.hoisted(() => vi.fn());
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
    listSquadSkills: (...args: unknown[]) => mockListSquadSkills(...args),
    addSquadSkills: (...args: unknown[]) => mockAddSquadSkills(...args),
    setSquadSkillEnabled: (...args: unknown[]) => mockSetSquadSkillEnabled(...args),
    removeSquadSkill: (...args: unknown[]) => mockRemoveSquadSkill(...args),
    setSquadRuntimeSkillEnabled: (...args: unknown[]) =>
      mockSetSquadRuntimeSkillEnabled(...args),
    setSquadRuntimeSkillsInherited: (...args: unknown[]) =>
      mockSetSquadRuntimeSkillsInherited(...args),
  },
  ApiError,
}));

vi.mock("@multica/core/runtimes", async () => {
  const actual = await vi.importActual<typeof RuntimesModule>("@multica/core/runtimes");
  return {
    ...actual,
    runtimeListOptions: (wsId: string) => ({
      queryKey: ["runtimes", wsId, "list"],
      queryFn: () => mockListRuntimes(wsId),
    }),
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

import { SquadSkillsTab } from "./squad-skills-tab";

const squad: Squad = {
  id: "squad-1",
  workspace_id: "ws-1",
  name: "MadSpec Backend",
  description: "",
  instructions: "",
  avatar_url: null,
  leader_id: "agent-lead",
  creator_id: "user-1",
  created_at: "2026-10-01T00:00:00Z",
  updated_at: "2026-10-01T00:00:00Z",
  archived_at: null,
  archived_by: null,
};

function makeRuntime(overrides: Partial<AgentRuntime> = {}): AgentRuntime {
  return {
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
    ...overrides,
  };
}

const codexRuntime = makeRuntime();
const ompRuntime = makeRuntime({
  id: "runtime-2",
  name: "Oh-My-Pi (Mac)",
  provider: "omp",
});

function makeSkill(overrides: Partial<SkillSummary> = {}): SkillSummary {
  return {
    id: "skill-1",
    workspace_id: "ws-1",
    name: "Review changes",
    description: "Review a patch",
    config: {},
    created_by: null,
    created_at: "2026-10-01T00:00:00Z",
    updated_at: "2026-10-01T00:00:00Z",
    enabled: true,
    ...overrides,
  };
}

const localReview: RuntimeLocalSkillSummary = {
  key: "local-review",
  name: "Local review",
  description: "Host-level review workflow",
  source_path: "~/.codex/skills/local-review",
  provider: "codex",
  root: "provider",
  can_disable: true,
  file_count: 1,
};

function capabilities(skills: RuntimeLocalSkillSummary[], supported = true) {
  return { skills, supported, mcpServers: [], mcpSupported: true };
}

function renderTab(
  squadOverrides: Partial<Squad> = {},
  currentUserId: string | null = "user-1",
  canEdit = true,
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const tab = (overrides: Partial<Squad>) => (
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      <QueryClientProvider client={queryClient}>
        <SquadSkillsTab
          squad={{ ...squad, ...overrides }}
          currentUserId={currentUserId}
          canEdit={canEdit}
        />
      </QueryClientProvider>
    </I18nProvider>
  );
  const result = render(tab(squadOverrides));
  return {
    ...result,
    queryClient,
    updateSquad: (overrides: Partial<Squad>) => result.rerender(tab(overrides)),
  };
}

const squadSkillsKey = workspaceKeys.squadSkills("ws-1", "squad-1");
const squadKey = squadDetailOptions("ws-1", "squad-1").queryKey;

const sectionSwitch = () =>
  screen.findByRole("switch", { name: /Turn on every skill from/i });

describe("SquadSkillsTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListSkills.mockResolvedValue([]);
    mockListSquadSkills.mockResolvedValue([]);
    mockAddSquadSkills.mockResolvedValue([]);
    mockSetSquadSkillEnabled.mockResolvedValue([]);
    mockRemoveSquadSkill.mockResolvedValue([]);
    mockSetSquadRuntimeSkillEnabled.mockResolvedValue(undefined);
    mockSetSquadRuntimeSkillsInherited.mockResolvedValue(undefined);
    mockListRuntimes.mockResolvedValue([codexRuntime]);
    mockRuntimeCapabilities.mockResolvedValue(capabilities([localReview]));
  });

  describe("assigned skills", () => {
    it("lists the squad's assigned workspace skills", async () => {
      mockListSquadSkills.mockResolvedValue([makeSkill()]);
      renderTab();

      expect(screen.getByText("Assigned to squad")).toBeInTheDocument();
      expect(await screen.findByText("Review changes")).toBeInTheDocument();
      expect(mockListSquadSkills).toHaveBeenCalledWith("squad-1");
    });

    it("disables an assigned skill without removing it", async () => {
      const user = userEvent.setup();
      mockListSquadSkills.mockResolvedValue([makeSkill()]);
      const { queryClient } = renderTab();
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");

      await user.click(
        await screen.findByRole("switch", { name: /Toggle Review changes/i }),
      );

      expect(mockSetSquadSkillEnabled).toHaveBeenCalledWith("squad-1", "skill-1", false);
      expect(mockRemoveSquadSkill).not.toHaveBeenCalled();
      await waitFor(() =>
        expect(invalidate).toHaveBeenCalledWith({ queryKey: squadSkillsKey }),
      );
    });

    it("removes an assigned skill", async () => {
      const user = userEvent.setup();
      mockListSquadSkills.mockResolvedValue([makeSkill()]);
      renderTab();

      await user.click(
        await screen.findByRole("button", { name: /Remove Review changes/i }),
      );

      expect(mockRemoveSquadSkill).toHaveBeenCalledWith("squad-1", "skill-1");
    });

    it("adds the picked workspace skills next to the assigned ones", async () => {
      const user = userEvent.setup();
      mockListSquadSkills.mockResolvedValue([makeSkill()]);
      mockListSkills.mockResolvedValue([
        makeSkill(),
        makeSkill({ id: "skill-2", name: "Plan sprint", description: "" }),
      ]);
      renderTab();

      await screen.findByText("Review changes");
      await user.click(screen.getByRole("button", { name: "Add skill" }));
      const dialog = await screen.findByRole("dialog");
      expect(within(dialog).queryByText("Review changes")).toBeNull();
      await user.click(within(dialog).getByText("Plan sprint"));
      await user.click(within(dialog).getByRole("button", { name: "Add 1 skill" }));

      expect(mockAddSquadSkills).toHaveBeenCalledWith("squad-1", {
        skill_ids: ["skill-2"],
      });
    });

    it("reports a failed assigned-skill change and keeps the list as it was", async () => {
      const user = userEvent.setup();
      mockListSquadSkills.mockResolvedValue([makeSkill()]);
      mockSetSquadSkillEnabled.mockRejectedValueOnce(new Error("Permission denied"));
      const { queryClient } = renderTab();
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");

      const toggle = await screen.findByRole("switch", {
        name: /Toggle Review changes/i,
      });
      await user.click(toggle);

      await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Permission denied"));
      expect(invalidate).not.toHaveBeenCalled();
      expect(
        screen.getByRole("switch", { name: /Toggle Review changes/i }),
      ).toBeChecked();
    });
  });

  describe("runtime picker", () => {
    it("lists only online local runtimes whose provider enforces skill filters", async () => {
      const user = userEvent.setup();
      mockListRuntimes.mockResolvedValue([
        codexRuntime,
        ompRuntime,
        makeRuntime({ id: "runtime-cloud", name: "Codex (Cloud)", runtime_mode: "cloud" }),
        makeRuntime({ id: "runtime-off", name: "Claude (Laptop)", provider: "claude", status: "offline" }),
        makeRuntime({ id: "runtime-gemini", name: "Gemini (Mac)", provider: "gemini" }),
      ]);
      renderTab();

      const picker = await screen.findByRole("combobox", { name: "Runtime" });
      await waitFor(() => expect(picker).toHaveTextContent("Codex (Mac)"));
      await user.click(picker);

      const options = await screen.findAllByRole("option");
      expect(options.map((option) => option.textContent)).toEqual([
        "Codex (Mac)",
        "Oh-My-Pi (Mac)",
      ]);
    });

    it("shows the skills of the picked runtime", async () => {
      const user = userEvent.setup();
      mockListRuntimes.mockResolvedValue([codexRuntime, ompRuntime]);
      mockRuntimeCapabilities.mockImplementation(async (runtimeId: string) =>
        runtimeId === "runtime-2"
          ? capabilities([
              { ...localReview, key: "omp-plan", name: "Omp plan", provider: "omp" },
            ])
          : capabilities([localReview]),
      );
      renderTab();

      expect(await screen.findByText("Local review")).toBeInTheDocument();
      await user.click(screen.getByRole("combobox", { name: "Runtime" }));
      await user.click(await screen.findByRole("option", { name: "Oh-My-Pi (Mac)" }));

      expect(await screen.findByText("Omp plan")).toBeInTheDocument();
      expect(screen.queryByText("Local review")).toBeNull();
    });

    it("explains when no runtime can switch skills", async () => {
      mockListRuntimes.mockResolvedValue([
        makeRuntime({ provider: "gemini", name: "Gemini (Mac)" }),
      ]);
      renderTab();

      expect(
        await screen.findByText(enSquads.skills_tab.runtime_none),
      ).toBeInTheDocument();
      expect(screen.queryByRole("combobox", { name: "Runtime" })).toBeNull();
      expect(mockRuntimeCapabilities).not.toHaveBeenCalled();
    });

    it("does not discover skills on another member's private runtime", async () => {
      mockListRuntimes.mockResolvedValue([{ ...codexRuntime, owner_id: "user-2" }]);
      renderTab({}, "user-1");

      expect(
        await screen.findByText(enSquads.skills_tab.runtime_forbidden),
      ).toBeInTheDocument();
      expect(mockRuntimeCapabilities).not.toHaveBeenCalled();
    });
  });

  describe("runtime skills", () => {
    it("treats an omitted section switch as off, so the squad gives no runtime skills", async () => {
      renderTab();

      expect(await sectionSwitch()).not.toBeChecked();
      expect(
        screen.getByRole("switch", { name: /Toggle runtime skill Local review/i }),
      ).not.toBeChecked();
    });

    it("turns the section on and refreshes the squad after saving", async () => {
      const user = userEvent.setup();
      let finishWrite!: () => void;
      mockSetSquadRuntimeSkillsInherited.mockReturnValueOnce(
        new Promise<void>((resolve) => {
          finishWrite = resolve;
        }),
      );
      const { queryClient } = renderTab();
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");

      const toggle = await sectionSwitch();
      expect(toggle).toHaveAccessibleName(/Codex \(Mac\)/);
      await user.click(toggle);

      expect(mockSetSquadRuntimeSkillsInherited).toHaveBeenCalledWith("squad-1", true);
      expect(toggle).toHaveAttribute("aria-disabled", "true");
      await user.click(toggle);
      expect(mockSetSquadRuntimeSkillsInherited).toHaveBeenCalledTimes(1);
      expect(invalidate).not.toHaveBeenCalled();

      finishWrite();
      await waitFor(() =>
        expect(invalidate).toHaveBeenCalledWith({ queryKey: squadKey, exact: true }),
      );
      expect(mockSetSquadRuntimeSkillEnabled).not.toHaveBeenCalled();
    });

    it("turns one runtime skill on for the picked runtime while the section is off", async () => {
      const user = userEvent.setup();
      renderTab({ inherit_runtime_skills: false });

      await user.click(
        await screen.findByRole("switch", { name: /Toggle runtime skill Local review/i }),
      );

      expect(mockSetSquadRuntimeSkillEnabled).toHaveBeenCalledWith("squad-1", {
        runtime_id: "runtime-1",
        root: "provider",
        key: "local-review",
        name: "Local review",
        plugin: undefined,
        enabled: true,
      });
    });

    it("reads exceptions per runtime", async () => {
      renderTab({
        inherit_runtime_skills: false,
        runtime_skill_exceptions: [
          {
            runtime_id: "runtime-2",
            provider: "codex",
            root: "provider",
            key: "local-review",
          },
        ],
      });

      expect(
        await screen.findByRole("switch", { name: /Toggle runtime skill Local review/i }),
      ).not.toBeChecked();
    });

    it("renders a stored exception as on while the section is off", async () => {
      renderTab({
        inherit_runtime_skills: false,
        runtime_skill_exceptions: [
          {
            runtime_id: "runtime-1",
            provider: "codex",
            root: "provider",
            key: "local-review",
          },
        ],
      });

      expect(
        await screen.findByRole("switch", { name: /Toggle runtime skill Local review/i }),
      ).toBeChecked();
    });

    it("reports a failed section change and leaves the section as it was", async () => {
      const user = userEvent.setup();
      mockSetSquadRuntimeSkillsInherited.mockRejectedValueOnce(new Error("offline"));
      const { queryClient } = renderTab();
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");

      const toggle = await sectionSwitch();
      await user.click(toggle);

      await waitFor(() => expect(toast.error).toHaveBeenCalledWith("offline"));
      expect(toggle).not.toBeChecked();
      expect(toggle).not.toHaveAttribute("aria-disabled", "true");
      expect(invalidate).not.toHaveBeenCalled();
    });

    it("shows a retry notice when discovery fails and lists the skills after a refresh", async () => {
      const user = userEvent.setup();
      mockRuntimeCapabilities.mockRejectedValueOnce(
        new Error("daemon did not respond within 3 minutes"),
      );
      renderTab();

      expect(
        await screen.findByText(enSquads.skills_tab.runtime_failed),
      ).toBeInTheDocument();
      expect(screen.queryByRole("switch", { name: /Turn on every skill from/i })).toBeNull();

      await user.click(screen.getByRole("button", { name: "Refresh" }));

      expect(await screen.findByText("Local review")).toBeInTheDocument();
      expect(await sectionSwitch()).toBeInTheDocument();
    });

    it("shows a permission notice when discovery is forbidden", async () => {
      mockRuntimeCapabilities.mockRejectedValue(
        new ApiError("insufficient permissions", 403, "Forbidden"),
      );
      renderTab();

      expect(
        await screen.findByText(enSquads.skills_tab.runtime_forbidden),
      ).toBeInTheDocument();
    });

    it("shows no switches for a runtime that does not expose local skills", async () => {
      mockRuntimeCapabilities.mockResolvedValue(capabilities([], false));
      renderTab();

      expect(
        await screen.findByText(enSquads.skills_tab.runtime_unsupported),
      ).toBeInTheDocument();
      expect(screen.queryByRole("switch")).toBeNull();
    });
  });

  describe("read-only", () => {
    it("lists skills without any control to change them", async () => {
      mockListSquadSkills.mockResolvedValue([makeSkill()]);
      renderTab({}, "user-1", false);

      expect(await screen.findByText("Review changes")).toBeInTheDocument();
      expect(await screen.findByText("Local review")).toBeInTheDocument();
      expect(screen.queryByRole("switch")).toBeNull();
      expect(screen.queryByRole("button", { name: "Add skill" })).toBeNull();
      expect(screen.queryByRole("button", { name: /Remove Review changes/i })).toBeNull();
      expect(screen.getByRole("combobox", { name: "Runtime" })).toBeInTheDocument();
    });
  });
});
