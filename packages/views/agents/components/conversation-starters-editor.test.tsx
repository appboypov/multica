// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import type { AgentConversationStarter } from "@multica/core/types";
import enAgents from "../../locales/en/agents.json";
import enChat from "../../locales/en/chat.json";
import { ConversationStartersEditor } from "./conversation-starters-editor";

// The selection matrix (which rows count, when the fallback wins) is covered
// once in packages/core/agents/conversation-starters.test.ts. This suite only
// proves the editor previews what a new chat would actually show.
function renderEditor(value: AgentConversationStarter[]) {
  render(
    <I18nProvider
      locale="en"
      resources={{ en: { agents: enAgents, chat: enChat } }}
    >
      <ConversationStartersEditor value={value} onChange={vi.fn()} />
    </I18nProvider>,
  );
}

describe("ConversationStartersEditor preview", () => {
  afterEach(cleanup);

  it("previews the built-in defaults and says so when nothing is configured", () => {
    renderEditor([]);

    expect(screen.getByText("Preview of a new chat")).toBeInTheDocument();
    expect(screen.getByText("What can you help with?")).toBeInTheDocument();
    expect(
      screen.getByText(
        "These are the built-in defaults. Add a suggestion above to replace all three.",
      ),
    ).toBeInTheDocument();
  });

  it("previews the configured labels and drops the defaults note", () => {
    renderEditor([
      { label: "Review the release PR", prompt: "Review it and list risks." },
    ]);

    // Twice: once as the editable label input's value, once in the preview.
    expect(
      screen.getByDisplayValue("Review the release PR"),
    ).toBeInTheDocument();
    expect(screen.getByText("Review the release PR")).toBeInTheDocument();
    expect(screen.queryByText("What can you help with?")).toBeNull();
    expect(
      screen.queryByText(/These are the built-in defaults/),
    ).toBeNull();
  });

  it("keeps previewing the defaults while a row is still half-filled", () => {
    renderEditor([{ label: "Draft a plan", prompt: "" }]);

    expect(screen.getByText("What can you help with?")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Complete or remove each suggestion before saving.",
      ),
    ).toBeInTheDocument();
  });
});

function StatefulEditor({
  initial,
}: {
  initial: AgentConversationStarter[];
}) {
  const [value, setValue] = useState(initial);
  return <ConversationStartersEditor value={value} onChange={setValue} />;
}

describe("ConversationStartersEditor length warnings", () => {
  afterEach(cleanup);

  it("keeps an 81-character label and a 4,001-character prompt and warns at both", async () => {
    const user = userEvent.setup();
    render(
      <I18nProvider
        locale="en"
        resources={{ en: { agents: enAgents, chat: enChat } }}
      >
        <StatefulEditor initial={[{ label: "", prompt: "" }]} />
      </I18nProvider>,
    );
    const label = "L".repeat(81);
    const prompt = "P".repeat(4001);

    const labelInput = screen.getByLabelText("Suggestion 1 label") as HTMLInputElement;
    await user.click(labelInput);
    await user.paste(label);
    const promptInput = screen.getByLabelText("Suggestion 1 prompt") as HTMLTextAreaElement;
    await user.click(promptInput);
    await user.paste(prompt);

    expect(labelInput.value).toBe(label);
    expect(promptInput.value).toBe(prompt);
    expect(screen.getByText(/81 \/ 80 · 1 over the recommended 80/)).toBeInTheDocument();
    expect(
      screen.getByText(/4001 \/ 4000 · 1 over the recommended 4000/),
    ).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("shows no counter at the thresholds", () => {
    renderEditor([{ label: "L".repeat(80), prompt: "P".repeat(4000) }]);

    expect(screen.queryByText(/over the recommended/)).toBeNull();
    expect(screen.queryByText(/\/ 80/)).toBeNull();
  });
});
