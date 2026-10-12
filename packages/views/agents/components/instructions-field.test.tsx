// @vitest-environment jsdom

import { useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import type {
  RuntimeDevice,
  RuntimeDirectoryListingRequest,
} from "@multica/core/types";
import enAgents from "../../locales/en/agents.json";
import enCommon from "../../locales/en/common.json";

const mockInitiateDirectoryListing = vi.hoisted(() => vi.fn());

vi.mock("@multica/core/api", () => ({
  api: {
    initiateDirectoryListing: (...args: unknown[]) =>
      mockInitiateDirectoryListing(...args),
    getDirectoryListing: vi.fn(),
  },
}));

import { InstructionsField } from "./instructions-field";

const SKILLS = "/Users/brian/.omp/agent/skills";
const UX = `${SKILLS}/as-ux-designer`;
const PM = `${SKILLS}/as-product-manager`;

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

function rootsListing(): RuntimeDirectoryListingRequest {
  return {
    id: "request-roots",
    runtime_id: RUNTIME.id,
    status: "completed",
    path: "",
    entries: [
      { name: "as-ux-designer", path: UX, kind: "folder", hidden: false },
      { name: "as-product-manager", path: PM, kind: "folder", hidden: false },
    ],
    error: "",
    created_at: "2026-10-12T00:00:00Z",
    updated_at: "2026-10-12T00:00:00Z",
  };
}

function Harness({
  initialValue,
  initialSelection,
  currentUserId,
  onChange,
  onSelectionChange,
}: {
  initialValue: string;
  initialSelection: string[];
  currentUserId: string | null;
  onChange: (value: string) => void;
  onSelectionChange: (selection: string[]) => void;
}) {
  const [value, setValue] = useState(initialValue);
  const [selection, setSelection] = useState(initialSelection);
  return (
    <InstructionsField
      id="instructions"
      value={value}
      onChange={(next) => {
        onChange(next);
        setValue(next);
      }}
      selection={selection}
      onSelectionChange={(next) => {
        onSelectionChange(next);
        setSelection(next);
      }}
      runtime={RUNTIME}
      currentUserId={currentUserId}
    />
  );
}

function renderField({
  value = "",
  selection = [] as string[],
  currentUserId = "brian" as string | null,
} = {}) {
  const onChange = vi.fn();
  const onSelectionChange = vi.fn();
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <I18nProvider
      locale="en"
      resources={{ en: { common: enCommon, agents: enAgents } }}
    >
      <QueryClientProvider client={queryClient}>
        <Harness
          initialValue={value}
          initialSelection={selection}
          currentUserId={currentUserId}
          onChange={onChange}
          onSelectionChange={onSelectionChange}
        />
      </QueryClientProvider>
    </I18nProvider>,
  );
  return { onChange, onSelectionChange, user: userEvent.setup() };
}

async function addThroughBrowser(user: UserEvent, name: string) {
  await user.click(screen.getByRole("button", { name: "Browse" }));
  await user.click(await screen.findByRole("checkbox", { name }));
  await user.click(screen.getByRole("button", { name: "Add" }));
}

const textarea = () => screen.getByRole<HTMLTextAreaElement>("textbox");

describe("InstructionsField", () => {
  beforeEach(() => {
    mockInitiateDirectoryListing.mockImplementation(async () => rootsListing());
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("gives the runtime's owner Browse and removable chips", () => {
    renderField({ value: "{selection}", selection: [UX, "/opt/skills/x"] });

    const chips = screen.getAllByRole("listitem");
    expect(chips.map((chip) => chip.textContent)).toEqual([
      "~/.omp/agent/skills/as-ux-designer",
      "/opt/skills/x",
    ]);
    expect(screen.getByRole("button", { name: "Browse" })).toBeEnabled();
    expect(screen.getByRole("button", { name: `Remove ${UX}` })).toBeEnabled();
  });

  it("shows another member the chips read-only, without Browse", () => {
    renderField({
      value: "{selection}",
      selection: [UX],
      currentUserId: "member-2",
    });

    expect(screen.getByRole("listitem")).toHaveTextContent(
      "~/.omp/agent/skills/as-ux-designer",
    );
    expect(screen.queryByRole("button", { name: "Browse" })).toBeNull();
    expect(screen.queryByRole("button", { name: /^Remove/ })).toBeNull();
  });

  it("renders no chip row for another member when nothing is selected", () => {
    renderField({ currentUserId: "member-2" });

    expect(screen.queryByRole("list")).toBeNull();
    expect(screen.queryByRole("button", { name: "Browse" })).toBeNull();
  });

  it("inserts {selection} at the cursor on the first add, then only appends", async () => {
    const { onChange, onSelectionChange, user } = renderField({
      value: "Follow this. Then go.",
    });
    await user.click(textarea());
    textarea().setSelectionRange(12, 12);

    await addThroughBrowser(user, "as-ux-designer");

    expect(textarea()).toHaveValue("Follow this.{selection} Then go.");
    expect(textarea().selectionStart).toBe(23);
    expect(onSelectionChange).toHaveBeenLastCalledWith([UX]);

    onChange.mockClear();
    await addThroughBrowser(user, "as-product-manager");

    expect(onChange).not.toHaveBeenCalled();
    expect(textarea()).toHaveValue("Follow this.{selection} Then go.");
    expect(onSelectionChange).toHaveBeenLastCalledWith([UX, PM]);
  });

  it("appends {selection} after a blank line when the text never had focus", async () => {
    const { user } = renderField({ value: "Follow this." });

    await addThroughBrowser(user, "as-ux-designer");

    expect(textarea()).toHaveValue("Follow this.\n\n{selection}");
  });

  it("removes a chip from the selection", async () => {
    const { onSelectionChange, user } = renderField({
      value: "{selection}",
      selection: [UX, PM],
    });

    await user.click(screen.getByRole("button", { name: `Remove ${UX}` }));

    expect(onSelectionChange).toHaveBeenCalledWith([PM]);
  });

  it("warns when paths are selected and the text has no {selection}", () => {
    renderField({ value: "Follow this.", selection: [UX] });

    expect(
      screen.getByText(
        "The selected paths aren't loaded: the instructions have no {selection}.",
      ),
    ).toBeInTheDocument();
    expect(textarea()).toBeEnabled();
    expect(screen.getByRole("button", { name: "Browse" })).toBeEnabled();
  });

  it("warns when {selection} has no paths", () => {
    renderField({ value: "Follow {selection}." });

    expect(
      screen.getByText("{selection} stays empty: no path is selected."),
    ).toBeInTheDocument();
    expect(textarea()).toBeEnabled();
    expect(screen.getByRole("button", { name: "Browse" })).toBeEnabled();
  });

  it("shows no warning when text and selection match", () => {
    renderField({ value: "Follow {selection}.", selection: [UX] });

    expect(screen.queryByText(/aren't loaded|stays empty/)).toBeNull();
  });
});
