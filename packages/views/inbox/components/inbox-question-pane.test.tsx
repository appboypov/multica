import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useQuestionDraftStore } from "@multica/core/questions";
import type * as Questions from "@multica/core/questions";
import type { InboxItem, Issue, Question } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { InboxQuestionBody, InboxQuestionPane } from "./inbox-question-pane";

const answerMutate = vi.fn();
let question: Question | undefined;
let issueQuestions: Question[] = [];

const viewer = vi.hoisted(() => ({ id: "user-brian" }));
vi.mock("@multica/core/auth", () => {
  const state = () => ({ user: { id: viewer.id } });
  const useAuthStore = Object.assign(
    (selector?: (s: unknown) => unknown) => (selector ? selector(state()) : state()),
    { getState: state },
  );
  return { useAuthStore };
});
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
const { push } = vi.hoisted(() => ({ push: vi.fn() }));
vi.mock("../../navigation", () => ({ useNavigation: () => ({ push }) }));
vi.mock("@multica/core/paths", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  useWorkspacePaths: () => ({
    chatSession: (id: string, options?: { questionId?: string }) => `/acme/chat?session=${id}&question=${options?.questionId}`,
    answerAll: (options?: { questionId?: string }) => `/acme/questions/answer?question=${options?.questionId}`,
  }),
}));
vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({ getActorName: (type: string) => (type === "agent" ? "Pieter" : "Brian") }),
}));
vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({ queryKey: ["members"], queryFn: () => [] }),
  agentListOptions: () => ({ queryKey: ["agents"], queryFn: () => [] }),
}));
vi.mock("@multica/core/issues/queries", () => ({
  issueDetailOptions: () => ({
    queryKey: ["issue"],
    queryFn: () => ({ id: "issue-412", identifier: "WORK-412", title: "VAT rounding" }) as Issue,
  }),
  issueTasksOptions: () => ({ queryKey: ["tasks"], queryFn: () => [] }),
}));
vi.mock("@multica/core/questions", async (importOriginal) => {
  const pending = { isPending: false };
  return {
    ...(await importOriginal<typeof Questions>()),
    useQuestion: () => ({ data: question, isPending: question === undefined }),
    issueQuestionsOptions: () => ({ queryKey: ["issue-questions"], queryFn: () => issueQuestions }),
    // The hook is bound to its question and scope; the spy records both with what was sent.
    useAnswerQuestion: (_wsId: string, scope: Questions.QuestionScope, questionId: string) => ({
      ...pending,
      mutate: (variables: object, options: unknown) => answerMutate({ id: questionId, scope, ...variables }, options),
    }),
    useAskBackQuestion: () => ({ ...pending, mutate: vi.fn() }),
    useReassignQuestion: () => ({ ...pending, mutate: vi.fn() }),
    useWakeQuestionAgent: () => ({ ...pending, mutate: vi.fn() }),
  };
});
vi.mock("../../common/actor-avatar", () => ({ ActorAvatar: () => <span /> }));

const item = {
  id: "inbox-1",
  type: "question_asked",
  issue_id: "issue-412",
  title: "VAT rounding",
  created_at: "2026-10-13T10:00:00Z",
  details: { question_id: "q-1" },
} as unknown as InboxItem;

function base() {
  return {
    id: "q-1",
    workspace_id: "ws-1",
    asker_agent_id: "agent-pieter",
    asked_in_task_id: "task-3",
    asker_user_id: "",
    ask_id: "q-1",
    ask_position: 0,
    ask_title: null,
    answer_policy: "any_one" as const,
    prompt: "Should VAT round per invoice line or on the invoice total?",
    context_line: null,
    status: "open" as const,
    due_date: null,
    blocks_work: false,
    links: [],
    answerers: [{ user_id: "user-brian", assigned_at: "2026-10-13T10:00:00Z" }],
    answers: [],
    events: [],
    unseen_answer_count: 0,
    wake_available: false,
    wake_targets: [],
    created_at: "2026-10-13T10:00:00Z",
    updated_at: "2026-10-13T10:00:00Z",
  };
}

function render(onOpenIssue = vi.fn()) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  renderWithI18n(
    <QueryClientProvider client={client}>
      <InboxQuestionPane item={item} issueId="issue-412" questionId="q-1" onOpenIssue={onOpenIssue} />
    </QueryClientProvider>,
  );
  return onOpenIssue;
}

/** Pieter's question asked in Brian's chat: the item has no issue and names the chat by its id and member. */
const chatItem = {
  id: "inbox-2",
  type: "question_asked",
  issue_id: null,
  title: "Should VAT round per invoice line or on the invoice total?",
  created_at: "2026-10-13T10:00:00Z",
  details: { question_id: "q-1", chat_session_id: "chat-1", member_id: "user-brian" },
} as unknown as InboxItem;

function renderChat() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  renderWithI18n(
    <QueryClientProvider client={client}>
      <InboxQuestionPane item={chatItem} questionId="q-1" chat={{ chatSessionId: "chat-1", memberId: "user-brian" }} />
    </QueryClientProvider>,
  );
}

describe("InboxQuestionPane", () => {
  beforeEach(() => {
    answerMutate.mockReset();
    push.mockReset();
    viewer.id = "user-brian";
    useQuestionDraftStore.setState({ drafts: {} });
  });

  it("answers a Single choice question in place", async () => {
    question = {
      ...base(),
      answer_type: "single_choice",
      settings: { options: ["Per invoice line", "On the invoice total"], allow_other: false },
      suggested_answer: { option: "Per invoice line" },
    };
    render();
    fireEvent.click(screen.getByRole("button", { name: /Accept suggestion/ }));
    expect(answerMutate).toHaveBeenCalledWith(
      expect.objectContaining({ id: "q-1", value: { option: "Per invoice line" } }),
      expect.anything(),
    );
    expect(await screen.findByRole("button", { name: /Open on WORK-412/ })).toBeInTheDocument();
  });

  it("sends a Ranking question to its issue without controls", async () => {
    question = {
      ...base(),
      answer_type: "ranking",
      settings: { items: ["Speed", "Cost", "Quality"] },
      suggested_answer: null,
    };
    const onOpenIssue = render();
    expect(screen.getByText("Ranking is answered on its issue.")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Send/ })).not.toBeInTheDocument();
    fireEvent.click(await screen.findByRole("button", { name: /Open on WORK-412/ }));
    expect(onOpenIssue).toHaveBeenCalledOnce();
  });

  // Scenario: Brian answers the last question of a set in the inbox
  it("offers Wake Pieter in a set only on the last question still open for Brian", async () => {
    const single = (overrides: Partial<Question>) => ({
      ...base(),
      ask_title: "VAT",
      answer_type: "single_choice",
      settings: { options: ["Per invoice line", "On the invoice total"], allow_other: false },
      suggested_answer: { option: "Per invoice line" },
      ...overrides,
    }) as Question;
    const second = single({ id: "q-2", ask_position: 1, prompt: "Which VAT rate applies?" });
    question = single({});
    issueQuestions = [question, second];
    render();
    expect(await screen.findByText("Which VAT rate applies?")).toBeInTheDocument();
    expect(screen.queryByRole("checkbox", { name: "Wake Pieter" })).not.toBeInTheDocument();
    cleanup();

    question = single({
      status: "answered",
      answers: [{
        id: "a-1",
        answerer_user_id: "user-brian",
        value: { option: "Per invoice line" },
        display_text: "Per invoice line",
        note: null,
        created_at: "2026-10-13T10:05:00Z",
        updated_at: "2026-10-13T10:05:00Z",
      }],
    });
    issueQuestions = [question, second];
    render();
    const wake = await screen.findByRole("checkbox", { name: "Wake Pieter" });
    expect(screen.getAllByRole("checkbox", { name: "Wake Pieter" })).toHaveLength(1);
    expect(wake).toHaveAttribute("aria-checked", "true");
    const card = within(screen.getByText("Which VAT rate applies?").closest<HTMLElement>("[data-question-state=open]")!);
    fireEvent.click(card.getByRole("button", { name: "Accept suggestion" }));
    expect(answerMutate).toHaveBeenCalledWith(
      expect.objectContaining({ id: "q-2", value: { option: "Per invoice line" }, wake: true }),
      expect.anything(),
    );
  });

  describe("a question asked in Brian's chat", () => {
    const singleChoice = (): Question => ({
      ...base(),
      answer_type: "single_choice",
      settings: { options: ["Per invoice line", "On the invoice total"], allow_other: false },
      suggested_answer: { option: "Per invoice line" },
    });

    // Scenario: Inbox item
    it("opens Brian's chat at the card and answers in place in the chat", () => {
      question = singleChoice();
      renderChat();
      expect(screen.getByRole("heading", { name: "Chat with Brian" })).toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: /Accept suggestion/ }));
      expect(answerMutate).toHaveBeenCalledWith(
        expect.objectContaining({ id: "q-1", scope: { chatSessionId: "chat-1" }, value: { option: "Per invoice line" } }),
        expect.anything(),
      );

      fireEvent.click(screen.getByRole("button", { name: /Open in chat/ }));
      expect(push).toHaveBeenCalledWith("/acme/chat?session=chat-1&question=q-1");
    });

    it("sends Brian to his chat for a Ranking question", () => {
      question = { ...base(), answer_type: "ranking", settings: { items: ["Speed", "Cost", "Quality"] }, suggested_answer: null };
      renderChat();
      expect(screen.getByText("Ranking is answered in the chat.")).toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: /Open in chat/ }));
      expect(push).toHaveBeenCalledWith("/acme/chat?session=chat-1&question=q-1");
    });

    // Scenario: Anouk sees the question, not the chat
    it("opens Anouk's question from Brian's chat in Answer all, naming the chat by its member", () => {
      viewer.id = "user-anouk";
      question = { ...singleChoice(), answerers: [{ user_id: "user-anouk", assigned_at: "2026-10-13T10:00:00Z" }] };
      renderChat();
      expect(screen.getByRole("heading", { name: "Chat with Brian" })).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: /Open in chat/ })).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: /Open in Answer all/ }));
      expect(push).toHaveBeenCalledWith("/acme/questions/answer?question=q-1");
    });
  });

  describe("without an issue", () => {
    function renderBody() {
      const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
      renderWithI18n(
        <QueryClientProvider client={client}>
          <InboxQuestionBody issueId={null} questionId="q-1" fallbackIdentifier="Approve the export" onOpenIssue={vi.fn()} />
        </QueryClientProvider>,
      );
    }

    it("answers an Approval question in place", () => {
      question = { ...base(), answer_type: "approval", settings: { subject: "The export" }, suggested_answer: null };
      renderBody();
      fireEvent.click(screen.getByRole("button", { name: "Approve" }));
      fireEvent.click(screen.getByRole("button", { name: /Send/ }));
      expect(answerMutate).toHaveBeenCalledWith(
        expect.objectContaining({ id: "q-1", value: { decision: "approve" } }),
        expect.anything(),
      );
      expect(screen.queryByRole("button", { name: /Open on/ })).not.toBeInTheDocument();
    });

    it("answers a Ranking question in place, since it has no issue to open", () => {
      question = { ...base(), answer_type: "ranking", settings: { items: ["Speed", "Cost", "Quality"] }, suggested_answer: null };
      renderBody();
      expect(screen.queryByText("Ranking is answered on its issue.")).not.toBeInTheDocument();
      expect(screen.getByRole("button", { name: /Send/ })).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: /Open on/ })).not.toBeInTheDocument();
    });
  });
});
