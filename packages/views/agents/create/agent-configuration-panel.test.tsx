// @vitest-environment jsdom

import { useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  buildCreateAgentRequest,
  EMPTY_AGENT_DRAFT,
  type AgentDraft,
} from "@multica/core/agents";
import { I18nProvider } from "@multica/core/i18n/react";
import type {
  RuntimeDevice,
  RuntimeDirectoryListingRequest,
} from "@multica/core/types";
import enAgents from "../../locales/en/agents.json";
import enCommon from "../../locales/en/common.json";
import enSettings from "../../locales/en/settings.json";

const mockInitiateDirectoryListing = vi.hoisted(() => vi.fn());

vi.mock("@multica/core/api", () => ({
  api: {
    initiateDirectoryListing: (...args: unknown[]) =>
      mockInitiateDirectoryListing(...args),
    getDirectoryListing: vi.fn(),
  },
  ApiError: class ApiError extends Error {},
}));

// The skill picker reads the workspace's skills, which this test does not need.
vi.mock("../components/skill-multi-select", () => ({
  SkillMultiSelect: () => null,
}));

import { AgentConfigurationPanel } from "./agent-configuration-panel";

const UX_DESIGNER = "/Users/brian/.omp/agent/skills/as-ux-designer";

const RUNTIME: RuntimeDevice = {
  id: "runtime-1",
  workspace_id: "ws-1",
  daemon_id: "daemon-1",
  name: "omp (MacBook)",
  runtime_mode: "local",
  provider: "omp",
  launch_header: "",
  status: "online",
  device_info: "",
  metadata: {},
  owner_id: "brian",
  visibility: "public",
  last_seen_at: null,
  created_at: "2026-10-12T00:00:00Z",
  updated_at: "2026-10-12T00:00:00Z",
};

const ROOTS: RuntimeDirectoryListingRequest = {
  id: "request-roots",
  runtime_id: RUNTIME.id,
  status: "completed",
  path: "",
  entries: [
    { name: "as-ux-designer", path: UX_DESIGNER, kind: "folder", hidden: false },
  ],
  error: "",
  created_at: "2026-10-12T00:00:00Z",
  updated_at: "2026-10-12T00:00:00Z",
};

function renderPanel() {
  const drafts: AgentDraft[] = [];
  function Harness() {
    const [draft, setDraft] = useState<AgentDraft>({
      ...EMPTY_AGENT_DRAFT,
      name: "Guus",
      instructions: "Follow this.",
      runtimeId: RUNTIME.id,
    });
    drafts.push(draft);
    return (
      <AgentConfigurationPanel
        draft={draft}
        onChange={setDraft}
        runtimes={[RUNTIME]}
        runtimesLoading={false}
        members={[]}
        currentUserId="brian"
        nameError={null}
        onNameChange={(name) => setDraft((current) => ({ ...current, name }))}
      />
    );
  }
  render(
    <I18nProvider
      locale="en"
      resources={{
        en: {
          common: enCommon,
          agents: enAgents,
          settings: enSettings,
        },
      }}
    >
      <QueryClientProvider
        client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
      >
        <Harness />
      </QueryClientProvider>
    </I18nProvider>,
  );
  return { latestDraft: () => drafts[drafts.length - 1]!, user: userEvent.setup() };
}

describe("AgentConfigurationPanel instructions field", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("adds a selected folder to the draft and its create request", async () => {
    mockInitiateDirectoryListing.mockResolvedValue(ROOTS);
    const { latestDraft, user } = renderPanel();

    await user.click(screen.getByRole("button", { name: "Browse" }));
    await user.click(
      await screen.findByRole("checkbox", { name: "as-ux-designer" }),
    );
    await user.click(screen.getByRole("button", { name: "Add" }));

    expect(screen.getByLabelText("Instructions")).toHaveValue(
      "Follow this.\n\n{selection}",
    );
    const request = buildCreateAgentRequest({
      draft: latestDraft(),
      runtimeId: RUNTIME.id,
    });
    expect(request.instructions).toBe("Follow this.\n\n{selection}");
    expect(request.instruction_selection).toEqual([UX_DESIGNER]);
  });
});
