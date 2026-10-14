import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type * as Questions from "@multica/core/questions";
import type { InboxItem, Issue, Question } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { InboxQuestionPane } from "./inbox-question-pane";

const answerMutate = vi.fn();
let question: Question | undefined;

vi.mock("@multica/core/auth", () => {
  const state = { user: { id: "user-brian" } };
  const useAuthStore = Object.assign(
    (selector?: (s: unknown) => unknown) => (selector ? selector(state) : state),
    { getState: () => state },
  );
  return { useAuthStore };
});
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
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
    // The hook is bound to its question; the spy records which one with what was sent.
    useAnswerQuestion: (_wsId: string, _issueId: string, questionId: string) => ({
      ...pending,
      mutate: (variables: object, options: unknown) => answerMutate({ id: questionId, ...variables }, options),
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

describe("InboxQuestionPane", () => {
  beforeEach(() => {
    answerMutate.mockReset();
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
});
