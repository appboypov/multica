// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import type {
  RuntimeDevice,
  RuntimeDirectoryEntry,
  RuntimeDirectoryListingRequest,
} from "@multica/core/types";
import enAgents from "../../locales/en/agents.json";
import enCommon from "../../locales/en/common.json";

const mockInitiateDirectoryListing = vi.hoisted(() => vi.fn());
const mockGetDirectoryListing = vi.hoisted(() => vi.fn());

vi.mock("@multica/core/api", () => ({
  api: {
    initiateDirectoryListing: (...args: unknown[]) =>
      mockInitiateDirectoryListing(...args),
    getDirectoryListing: (...args: unknown[]) => mockGetDirectoryListing(...args),
  },
}));

import { RuntimeFileBrowser } from "./runtime-file-browser";

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
  owner_id: "user-1",
  visibility: "private",
  last_seen_at: null,
  created_at: "2026-10-12T00:00:00Z",
  updated_at: "2026-10-12T00:00:00Z",
};

function folder(path: string, name = path.split("/").pop()!): RuntimeDirectoryEntry {
  return { name, path, kind: "folder", hidden: name.startsWith(".") };
}

function file(path: string): RuntimeDirectoryEntry {
  const name = path.split("/").pop()!;
  return { name, path, kind: "file", hidden: name.startsWith(".") };
}

/** Listings by requested path, as the runtime machine answers them. */
const LISTINGS: Record<string, { path: string; entries: RuntimeDirectoryEntry[] }> = {
  "~": { path: "/Users/brian", entries: [folder("/Users/brian/.omp")] },
  "/Users/brian/.omp": {
    path: "/Users/brian/.omp",
    entries: [folder("/Users/brian/.omp/agent")],
  },
  "/Users/brian/.omp/agent/skills/as-ux-designer": {
    path: "/Users/brian/.omp/agent/skills/as-ux-designer",
    entries: [
      folder("/Users/brian/.omp/agent/skills/as-ux-designer/.claude"),
      file("/Users/brian/.omp/agent/skills/as-ux-designer/SKILL.md"),
    ],
  },
  "/Users": { path: "/Users", entries: [folder("/Users/brian")] },
  "/": { path: "/", entries: [folder("/Users")] },
  "/s": { path: "/s", entries: [folder("/s/as-ux-designer")] },
  "C:\\Users\\brian": {
    path: "C:\\Users\\brian",
    entries: [folder("C:\\Users\\brian\\.omp", ".omp")],
  },
  "C:\\Users": { path: "C:\\Users", entries: [folder("C:\\Users\\brian", "brian")] },
  "C:\\": { path: "C:\\", entries: [folder("C:\\Users", "Users")] },
  "/n": { path: "/n", entries: [file("/n/b.md"), file("/n/c.md")] },
};

function listingRequest(path: string): RuntimeDirectoryListingRequest {
  const listing = LISTINGS[path];
  return {
    id: `request-${path}`,
    runtime_id: RUNTIME.id,
    status: listing ? "completed" : "failed",
    path: listing?.path ?? path,
    entries: listing?.entries ?? [],
    error: listing ? "" : `path does not exist: ${path}`,
    created_at: "2026-10-12T00:00:00Z",
    updated_at: "2026-10-12T00:00:00Z",
  };
}

function renderBrowser(runtime: RuntimeDevice = RUNTIME) {
  const onAdd = vi.fn();
  const onOpenChange = vi.fn();
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <I18nProvider
      locale="en"
      resources={{ en: { common: enCommon, agents: enAgents } }}
    >
      <QueryClientProvider client={queryClient}>
        <RuntimeFileBrowser
          open
          onOpenChange={onOpenChange}
          runtime={runtime}
          onAdd={onAdd}
        />
      </QueryClientProvider>
    </I18nProvider>,
  );
  return { onAdd, onOpenChange, user: userEvent.setup() };
}

async function typePath(user: UserEvent, path: string) {
  const input = screen.getByRole("textbox", { name: "Path" });
  await user.clear(input);
  await user.type(input, `${path}{Enter}`);
}

describe("RuntimeFileBrowser", () => {
  beforeEach(() => {
    mockInitiateDirectoryListing.mockImplementation(
      async (_runtimeId: string, path: string) => listingRequest(path),
    );
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("opens on the user's home", async () => {
    renderBrowser();

    expect(await screen.findByRole("button", { name: ".omp" })).toBeInTheDocument();
    expect(mockInitiateDirectoryListing).toHaveBeenCalledWith("runtime-1", "~");
    expect(screen.getByRole("textbox", { name: "Path" })).toHaveValue("/Users/brian");
    expect(screen.getByRole("button", { name: "Up" })).toBeEnabled();
  });

  it("opens a folder and lists its entries", async () => {
    const { user } = renderBrowser();

    await user.click(await screen.findByRole("button", { name: ".omp" }));

    expect(await screen.findByRole("button", { name: "agent" })).toBeInTheDocument();
    expect(mockInitiateDirectoryListing).toHaveBeenLastCalledWith(
      "runtime-1",
      "/Users/brian/.omp",
    );
    expect(screen.getByRole("textbox", { name: "Path" })).toHaveValue(
      "/Users/brian/.omp",
    );
  });

  it("lists a typed path and shows the error of a missing one", async () => {
    const { user } = renderBrowser();
    await screen.findByRole("button", { name: ".omp" });

    await typePath(user, "/n");
    expect(await screen.findByText("c.md")).toBeInTheDocument();
    expect(mockInitiateDirectoryListing).toHaveBeenLastCalledWith("runtime-1", "/n");

    await typePath(user, "/missing");
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Can't list this path: path does not exist: /missing",
    );
    expect(screen.queryByText("c.md")).not.toBeInTheDocument();
  });

  it("opens home when a cleared path is submitted, so Up still walks up", async () => {
    const { user } = renderBrowser();
    await typePath(user, "/n");
    await screen.findByText("c.md");

    await typePath(user, "");
    expect(await screen.findByRole("button", { name: ".omp" })).toBeInTheDocument();
    expect(mockInitiateDirectoryListing).not.toHaveBeenCalledWith("runtime-1", "");
    expect(screen.getByRole("textbox", { name: "Path" })).toHaveValue("/Users/brian");

    const up = screen.getByRole("button", { name: "Up" });
    expect(up).toBeEnabled();
    await user.click(up);
    expect(await screen.findByRole("button", { name: "brian" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Path" })).toHaveValue("/Users");
  });

  it("marks a hidden entry and lets it be ticked", async () => {
    const { user, onAdd } = renderBrowser();
    await typePath(user, "/Users/brian/.omp/agent/skills/as-ux-designer");

    const hiddenRow = (await screen.findByRole("button", { name: ".claude" })).closest(
      "li",
    )!;
    expect(hiddenRow).toHaveTextContent("hidden");
    expect(screen.getByText("SKILL.md").closest("li")).not.toHaveTextContent(
      "hidden",
    );

    await user.click(screen.getByRole("checkbox", { name: ".claude" }));
    await user.click(screen.getByRole("button", { name: "Add" }));

    expect(onAdd).toHaveBeenCalledWith([
      "/Users/brian/.omp/agent/skills/as-ux-designer/.claude",
    ]);
  });

  it("adds ticks from two folders in tick order and closes", async () => {
    const { user, onAdd, onOpenChange } = renderBrowser();
    const add = screen.getByRole("button", { name: "Add" });
    expect(add).toBeDisabled();

    await typePath(user, "/s");
    await user.click(await screen.findByRole("checkbox", { name: "as-ux-designer" }));
    await typePath(user, "/n");
    await user.click(await screen.findByRole("checkbox", { name: "c.md" }));
    await typePath(user, "/s");
    expect(
      await screen.findByRole("checkbox", { name: "as-ux-designer" }),
    ).toBeChecked();

    await user.click(add);

    expect(onAdd).toHaveBeenCalledTimes(1);
    expect(onAdd).toHaveBeenCalledWith(["/s/as-ux-designer", "/n/c.md"]);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("walks up from home to the filesystem root and stops there", async () => {
    const { user } = renderBrowser();
    await screen.findByRole("button", { name: ".omp" });
    const up = screen.getByRole("button", { name: "Up" });

    await user.click(up);
    expect(await screen.findByRole("button", { name: "brian" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Path" })).toHaveValue("/Users");

    await user.click(up);
    expect(await screen.findByRole("button", { name: "Users" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Path" })).toHaveValue("/");
    expect(up).toBeDisabled();
  });

  it("walks up a Windows path to its drive root and stops there", async () => {
    const { user } = renderBrowser();
    await typePath(user, "C:\\Users\\brian");
    await screen.findByRole("button", { name: ".omp" });
    const up = screen.getByRole("button", { name: "Up" });

    await user.click(up);
    expect(await screen.findByRole("button", { name: "brian" })).toBeInTheDocument();
    expect(mockInitiateDirectoryListing).toHaveBeenLastCalledWith("runtime-1", "C:\\Users");

    await user.click(up);
    expect(await screen.findByRole("button", { name: "Users" })).toBeInTheDocument();
    expect(mockInitiateDirectoryListing).toHaveBeenLastCalledWith("runtime-1", "C:\\");
    expect(up).toBeDisabled();
  });

  it("says an offline runtime is offline, requests nothing and adds nothing", () => {
    renderBrowser({ ...RUNTIME, status: "offline" });

    expect(screen.getByRole("status")).toHaveTextContent("The runtime is offline.");
    expect(screen.queryByRole("textbox", { name: "Path" })).not.toBeInTheDocument();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Add" })).toBeDisabled();
    expect(mockInitiateDirectoryListing).not.toHaveBeenCalled();
    expect(mockGetDirectoryListing).not.toHaveBeenCalled();
  });
});
