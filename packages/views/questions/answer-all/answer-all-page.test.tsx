import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type * as CoreApi from "@multica/core/api";
import type * as CorePaths from "@multica/core/paths";
import type { Issue, Question, QuestionRow, QuestionRowLink, TypedQuestion } from "@multica/core/types";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { renderWithI18n } from "../../test/i18n";
import { AnswerAllPage } from "./answer-all-page";

const api = vi.hoisted(() => ({
  listQuestionRows: vi.fn(),
  listIssueQuestions: vi.fn(),
  getQuestion: vi.fn(),
  answerQuestion: vi.fn(),
  wakeQuestionAgent: vi.fn(),
}));

vi.mock("@multica/core/auth", () => {
  const state = { user: { id: "user-anouk" } };
  const useAuthStore = Object.assign(
    (selector?: (s: unknown) => unknown) => (selector ? selector(state) : state),
    { getState: () => state },
  );
  return { useAuthStore };
});
// Only these calls exist: a chat's question list, which is Brian's alone, is never asked for.
vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof CoreApi>()),
  api,
}));
vi.mock("@multica/core/hooks", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  useWorkspaceId: () => "ws-1",
}));
vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof CorePaths>();
  return { ...actual, useWorkspacePaths: () => actual.paths.workspace("acme") };
});
vi.mock("@multica/core/issues/queries", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  issueDetailOptions: (_wsId: string, issueId: string) => ({
    queryKey: ["issue", issueId],
    queryFn: () => ({ id: "issue-412", identifier: "WORK-412", title: "VAT rounding", status: "in_progress", description: null }) as unknown as Issue,
  }),
}));
vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({
    getActorName: (type: string, id: string) =>
      type === "agent" ? "Pieter" : id === "user-brian" ? "Brian" : id === "user-anouk" ? "Anouk" : "Someone",
  }),
}));
vi.mock("@multica/core/workspace/queries", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  memberListOptions: () => ({ queryKey: ["members"], queryFn: () => [] }),
  agentListOptions: () => ({ queryKey: ["agents"], queryFn: () => [] }),
}));
vi.mock("../../common/actor-avatar", () => ({ ActorAvatar: () => <span /> }));

/** Brian's chat with Pieter, as Anouk's rows carry it: by its member, with no title sent. */
const BRIAN_CHAT: QuestionRowLink = { type: "chat", id: "chat-1", identifier: null, title: "", status: null, member_id: "user-brian" };
const WORK_412: QuestionRowLink = {
  type: "issue",
  id: "issue-412",
  identifier: "WORK-412",
  title: "VAT rounding",
  status: "in_progress",
  member_id: null,
};

const ISSUE_PROMPT = "Which VAT rate applies?";
const CHAT_PROMPT = "Should VAT round per invoice line or on the invoice total?";

function row(id: string, prompt: string, link: QuestionRowLink): QuestionRow {
  return {
    id,
    ask_id: id,
    ask_position: 0,
    ask_title: null,
    prompt,
    status: "open",
    answer_policy: "any_one",
    asker_agent_id: "agent-pieter",
    answerer_ids: ["user-anouk"],
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

function question(id: string, prompt: string, overrides: Partial<TypedQuestion<"single_choice">> = {}): Question {
  return {
    id,
    workspace_id: "ws-1",
    asker_agent_id: "agent-pieter",
    asked_in_task_id: "task-3",
    ask_id: id,
    ask_position: 0,
    ask_title: null,
    answer_policy: "any_one",
    prompt,
    context_line: null,
    status: "open",
    due_date: null,
    blocks_work: false,
    links: [],
    answerers: [{ user_id: "user-anouk", assigned_at: "2026-10-15T09:00:00Z" }],
    events: [],
    unseen_answer_count: 0,
    wake_available: false,
    wake_targets: [],
    created_at: "2026-10-15T09:00:00Z",
    updated_at: "2026-10-15T09:00:00Z",
    answer_type: "single_choice",
    settings: { options: ["Per invoice line", "On the invoice total"], allow_other: false },
    suggested_answer: { option: "Per invoice line" },
    answers: [],
    ...overrides,
  };
}

/** Pieter's question to Anouk in Brian's chat, as the server holds it now. */
let chatQuestion: Question;

function renderPage(search: string) {
  const navigation: NavigationAdapter = {
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/acme/questions/answer",
    searchParams: new URLSearchParams(search),
    hash: "",
    getShareableUrl: (path) => path,
  };
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  renderWithI18n(
    <QueryClientProvider client={client}>
      <NavigationProvider value={navigation}>
        <AnswerAllPage />
      </NavigationProvider>
    </QueryClientProvider>,
  );
}

describe("AnswerAllPage with a question from Brian's chat", () => {
  beforeEach(() => {
    for (const call of Object.values(api)) call.mockReset();
    chatQuestion = question("q-chat", CHAT_PROMPT);
    api.listQuestionRows.mockResolvedValue({
      rows: [row("q-issue", ISSUE_PROMPT, WORK_412), row("q-chat", CHAT_PROMPT, BRIAN_CHAT)],
      next_cursor: null,
    });
    api.listIssueQuestions.mockResolvedValue([question("q-issue", ISSUE_PROMPT)]);
    api.getQuestion.mockImplementation(async (id: string) => (id === "q-chat" ? chatQuestion : undefined));
    // Anouk's answer reaches Pieter only once she wakes him in Brian's chat.
    api.answerQuestion.mockImplementation(async () => {
      chatQuestion = question("q-chat", CHAT_PROMPT, {
        status: "answered",
        answers: [{
          id: "a-anouk",
          answerer_user_id: "user-anouk",
          value: { option: "Per invoice line" },
          display_text: "Per invoice line",
          note: null,
          created_at: "2026-10-15T09:10:00Z",
          updated_at: "2026-10-15T09:10:00Z",
        }],
        unseen_answer_count: 1,
        wake_available: true,
        wake_targets: [{ agent_id: "agent-pieter", issue_id: null, chat_session_id: "chat-1", unseen_answer_count: 1, wake_available: true }],
      });
      return chatQuestion;
    });
    api.wakeQuestionAgent.mockResolvedValue({});
  });

  // Scenario: Anouk answers a question from Brian's chat
  it("opens at the chat question, loaded on its own, and names the chat only by its member", async () => {
    renderPage("question=q-chat");

    expect(await screen.findByText(CHAT_PROMPT)).toBeInTheDocument();
    expect(api.getQuestion).toHaveBeenCalledWith("q-chat");
    expect(api.listIssueQuestions).toHaveBeenCalledWith("issue-412");
    // Anouk may not open Brian's chat: its name is plain text, not a link.
    const chat = screen.getByText("Chat with Brian");
    expect(chat.closest("a")).toBeNull();
    // Up next names the issue's question by its issue.
    const upNext = screen.getByRole("button", { name: new RegExp(ISSUE_PROMPT) });
    expect(within(upNext).getByText("WORK-412")).toBeInTheDocument();
  });

  // Scenario: Anouk wakes Pieter from Answer all
  it("lets Anouk answer it and wake Pieter in Brian's chat", async () => {
    renderPage("question=q-chat");

    fireEvent.click(await screen.findByRole("button", { name: /Accept suggestion/ }));
    await waitFor(() => expect(api.answerQuestion).toHaveBeenCalledWith("q-chat", expect.objectContaining({ value: { option: "Per invoice line" } })));
    expect(api.answerQuestion.mock.calls[0]![1]).not.toHaveProperty("wake");

    expect(await screen.findByText("Pieter has 1 answer from you in the chat with Brian")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wake Pieter" }));
    await waitFor(() => expect(api.wakeQuestionAgent).toHaveBeenCalledWith("q-chat", "agent-pieter"));
  });
});
