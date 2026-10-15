import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type * as CoreApi from "@multica/core/api";
import type * as CorePaths from "@multica/core/paths";
import type { QuestionRow, QuestionRowLink, QuestionTab } from "@multica/core/types";
import { NavigationProvider, type NavigationAdapter } from "../navigation";
import { renderWithI18n } from "../test/i18n";
import { QuestionsPage } from "./questions-page";

const { listQuestionRows, getOpenQuestionCount, viewer } = vi.hoisted(() => ({
  listQuestionRows: vi.fn(),
  getOpenQuestionCount: vi.fn(),
  viewer: { id: "user-brian" },
}));

vi.mock("@multica/core/auth", () => {
  const useAuthStore = Object.assign(
    (selector?: (state: unknown) => unknown) => {
      const state = { user: { id: viewer.id } };
      return selector ? selector(state) : state;
    },
    { getState: () => ({ user: { id: viewer.id } }) },
  );
  return { useAuthStore };
});
vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof CoreApi>()),
  api: { listQuestionRows, getOpenQuestionCount },
}));
vi.mock("@multica/core/hooks", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  useWorkspaceId: () => "ws-1",
}));
vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof CorePaths>();
  return { ...actual, useWorkspacePaths: () => actual.paths.workspace("acme") };
});
vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({
    getActorName: (type: string, id: string) =>
      type === "agent" ? "Pieter" : id === "user-brian" ? "Brian" : id === "user-anouk" ? "Anouk" : "Someone",
  }),
}));
vi.mock("../common/actor-avatar", () => ({ ActorAvatar: () => <span /> }));

/** Brian's chat with Pieter; the server never sends a chat's title, but the page must not show one either way. */
const BRIAN_CHAT: QuestionRowLink = {
  type: "chat",
  id: "chat-1",
  identifier: null,
  title: "Invoice VAT plan",
  status: null,
  member_id: "user-brian",
};

const WORK_412: QuestionRowLink = {
  type: "issue",
  id: "issue-412",
  identifier: "WORK-412",
  title: "VAT rounding",
  status: "in_progress",
  member_id: null,
};

function row(id: string, prompt: string, link: QuestionRowLink, answererId: string): QuestionRow {
  return {
    id,
    ask_id: id,
    ask_position: 0,
    ask_title: null,
    prompt,
    status: "open",
    answer_policy: "any_one",
    asker_agent_id: "agent-pieter",
    answerer_ids: [answererId],
    due_date: null,
    blocks_work: false,
    created_at: "2026-10-15T09:00:00Z",
    link,
    via: null,
    open_issue: link.type === "issue" ? { id: link.id, identifier: link.identifier! } : null,
    my_answer: null,
    answer_type: "single_choice",
    settings: { options: ["Per invoice line", "On the invoice total"], allow_other: false },
  };
}

/** For me holds these rows; every other tab is empty. */
function forMe(rows: QuestionRow[]) {
  listQuestionRows.mockImplementation(async (tab: QuestionTab) => ({ rows: tab === "for_me" ? rows : [], next_cursor: null }));
  getOpenQuestionCount.mockResolvedValue({ count: rows.length });
}

function renderPage() {
  const push = vi.fn();
  const navigation: NavigationAdapter = {
    push,
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/acme/questions",
    searchParams: new URLSearchParams(),
    hash: "",
    getShareableUrl: (path) => path,
  };
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  renderWithI18n(
    <QueryClientProvider client={client}>
      <NavigationProvider value={navigation}>
        <QuestionsPage />
      </NavigationProvider>
    </QueryClientProvider>,
  );
  return push;
}

describe("QuestionsPage chat rows", () => {
  beforeEach(() => {
    viewer.id = "user-brian";
    listQuestionRows.mockReset();
    getOpenQuestionCount.mockReset();
  });

  // Scenario: Brian opens his chat question from the Questions page
  it("groups Brian's chat question under Chat with Brian and opens his chat at its card", async () => {
    forMe([
      row("q-issue", "Which VAT rate applies?", WORK_412, "user-brian"),
      row("q-chat", "Should VAT round per invoice line or on the invoice total?", BRIAN_CHAT, "user-brian"),
    ]);
    const push = renderPage();

    const group = await screen.findByRole("region", { name: "Chat with Brian" });
    expect(screen.queryByText("Invoice VAT plan")).not.toBeInTheDocument();

    fireEvent.click(within(group).getByText("Should VAT round per invoice line or on the invoice total?"));
    expect(push).toHaveBeenCalledWith("/acme/chat?session=chat-1&question=q-chat");

    // The issue's question still opens on its issue.
    fireEvent.click(screen.getByText("Which VAT rate applies?"));
    expect(push).toHaveBeenLastCalledWith("/acme/issues/WORK-412#question-q-issue");
  });

  // Scenario: Anouk answers a question from Brian's chat
  it("opens Anouk's question from Brian's chat in Answer all, never the chat", async () => {
    viewer.id = "user-anouk";
    forMe([row("q-chat", "Should VAT round per invoice line or on the invoice total?", BRIAN_CHAT, "user-anouk")]);
    const push = renderPage();

    // Scenario: Anouk sees the question, not the chat
    const group = await screen.findByRole("region", { name: "Chat with Brian" });
    expect(screen.queryByText("Invoice VAT plan")).not.toBeInTheDocument();

    fireEvent.keyDown(within(group).getByRole("listitem"), { key: "Enter" });
    expect(push).toHaveBeenCalledWith("/acme/questions/answer?question=q-chat");
    expect(push).not.toHaveBeenCalledWith(expect.stringContaining("/chat"));
  });
});
