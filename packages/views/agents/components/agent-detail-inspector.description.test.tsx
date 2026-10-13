import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as ReactQuery from "@tanstack/react-query";
import type { Agent } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { AgentDetailInspector } from "./agent-detail-inspector";

vi.mock("@tanstack/react-query", async (importOriginal) => ({
  ...(await importOriginal<typeof ReactQuery>()),
  useQuery: () => ({ data: undefined, isSuccess: false }),
}));

vi.mock("../../common/avatar-upload-control", () => ({
  AvatarUploadControl: () => <div data-testid="avatar-upload" />,
}));

vi.mock("./inspector/model-picker", () => ({
  ModelPicker: () => <div data-testid="model-picker" />,
}));

vi.mock("./inspector/runtime-picker", () => ({
  RuntimePicker: () => <div data-testid="runtime-picker" />,
}));

vi.mock("./inspector/thinking-prop-row", () => ({
  ThinkingSettingField: () => <div data-testid="thinking-field" />,
}));

vi.mock("./inspector/service-tier-setting-field", () => ({
  ServiceTierSettingField: () => <div data-testid="service-tier-field" />,
}));

const agent = {
  id: "agent-1",
  workspace_id: "workspace-1",
  name: "Lambda",
  description: "",
  runtime_id: "runtime-1",
} as Agent;

function renderInspector() {
  const onUpdate = vi.fn(async () => {});
  renderWithI18n(
    <AgentDetailInspector
      agent={agent}
      runtime={null}
      runtimes={[]}
      members={[]}
      currentUserId="user-1"
      canEdit
      onUpdate={onUpdate}
    />,
  );
  return { onUpdate };
}

async function enterDescription(text: string) {
  const user = userEvent.setup();
  const field = screen.getByLabelText("Description") as HTMLTextAreaElement;
  await user.click(field);
  await user.paste(text);
  return { user, field };
}

describe("AgentDetailInspector description", () => {
  afterEach(() => {
    cleanup();
  });

  it("keeps a description past 255 characters, warns, and saves it", async () => {
    const { onUpdate } = renderInspector();
    const description = "a".repeat(300);

    const { user, field } = await enterDescription(description);

    expect(field.value).toBe(description);
    expect(
      screen.getByText(/45 over the recommended 255/),
    ).toBeInTheDocument();

    await user.tab();

    await waitFor(() =>
      expect(onUpdate).toHaveBeenCalledWith("agent-1", {
        name: "Lambda",
        description,
      }),
    );
  });

  it("shows no warning at exactly 255 characters", async () => {
    renderInspector();

    const { field } = await enterDescription("a".repeat(255));

    expect(field.value).toHaveLength(255);
    expect(screen.getByText(/255 \/ 255/)).toBeInTheDocument();
    expect(screen.queryByText(/over the recommended/)).toBeNull();
  });
});
