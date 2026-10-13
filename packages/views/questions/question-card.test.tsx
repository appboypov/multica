import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Question, TypedQuestion } from "@multica/core/types";
import { renderWithI18n } from "../test/i18n";
import { QuestionCard } from "./question-card";

const answerMutate = vi.fn();
const wakeMutate = vi.fn();
let viewerId = "user-brian";

vi.mock("@multica/core/auth", () => {
  const useAuthStore = Object.assign(
    (selector?: (state: unknown) => unknown) => {
      const state = { user: { id: viewerId } };
      return selector ? selector(state) : state;
    },
    { getState: () => ({ user: { id: viewerId } }) },
  );
  return { useAuthStore };
});
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({
    getActorName: (type: string, id: string) =>
      type === "agent" ? "Pieter" : id === "user-brian" ? "Brian" : id === "user-anouk" ? "Anouk" : "Someone",
  }),
}));
vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({ queryKey: ["members"], queryFn: () => [] }),
  agentListOptions: () => ({ queryKey: ["agents"], queryFn: () => [] }),
}));
vi.mock("@multica/core/questions", async (importOriginal) => {
  const pending = { isPending: false };
  return {
    ...(await importOriginal<typeof import("@multica/core/questions")>()),
    useAnswerQuestion: () => ({ ...pending, mutate: answerMutate }),
    useAskBackQuestion: () => ({ ...pending, mutate: vi.fn() }),
    useReassignQuestion: () => ({ ...pending, mutate: vi.fn() }),
    useWakeQuestionAgent: () => ({ ...pending, mutate: wakeMutate }),
    useUploadQuestionFile: () => ({ ...pending, mutateAsync: vi.fn() }),
  };
});
vi.mock("../common/actor-avatar", () => ({ ActorAvatar: () => <span /> }));

function base(): Omit<TypedQuestion<"single_choice">, "answer_type" | "settings" | "suggested_answer" | "answers"> {
  return {
    id: "q-1",
    workspace_id: "ws-1",
    asker_agent_id: "agent-pieter",
    asked_in_task_id: "task-3",
    prompt: "Should VAT round per invoice line or on the invoice total?",
    context_line: null,
    status: "open",
    due_date: null,
    blocks_work: false,
    links: [],
    answerers: [{ user_id: "user-brian", assigned_at: "2026-10-13T10:00:00Z" }],
    events: [],
    unseen_answer_count: 0,
    wake_available: false,
    created_at: "2026-10-13T10:00:00Z",
    updated_at: "2026-10-13T10:00:00Z",
  };
}

function singleChoice(overrides: Partial<TypedQuestion<"single_choice">> = {}): Question {
  return {
    ...base(),
    answer_type: "single_choice",
    settings: { options: ["Per invoice line", "On the invoice total"], allow_other: true },
    suggested_answer: { option: "Per invoice line" },
    answers: [],
    ...overrides,
  };
}

function answered(overrides: Partial<TypedQuestion<"single_choice">> = {}): Question {
  return singleChoice({
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
    ...overrides,
  });
}

function renderCard(question: Question) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return renderWithI18n(
    <QueryClientProvider client={client}>
      <QuestionCard question={question} issueId="issue-412" runNumber={3} />
    </QueryClientProvider>,
  );
}

describe("QuestionCard", () => {
  beforeEach(() => {
    viewerId = "user-brian";
    answerMutate.mockReset();
    wakeMutate.mockReset();
  });

  it("sends the suggested answer on Accept suggestion", () => {
    renderCard(singleChoice());
    expect(screen.getByText("Question for you")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Accept suggestion" }));
    expect(answerMutate).toHaveBeenCalledWith(
      { id: "q-1", value: { option: "Per invoice line" } },
      expect.anything(),
    );
  });

  it("turns Accept suggestion into Send once something is picked", () => {
    renderCard(singleChoice());
    fireEvent.click(screen.getByRole("radio", { name: /On the invoice total/ }));
    expect(screen.queryByRole("button", { name: "Accept suggestion" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(answerMutate).toHaveBeenCalledWith(
      { id: "q-1", value: { option: "On the invoice total" } },
      expect.anything(),
    );
  });

  it("picks an option with its number key", () => {
    renderCard(singleChoice());
    fireEvent.keyDown(screen.getByRole("radio", { name: /Per invoice line/ }), { key: "2" });
    expect(screen.getByRole("radio", { name: /On the invoice total/ })).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("button", { name: "Send" })).toBeEnabled();
  });

  it("disables Send with its message below the minimum of Multiple choice", () => {
    renderCard({
      ...base(),
      answer_type: "multiple_choice",
      settings: { options: ["21% standard", "9% reduced", "0% exempt", "Reverse charge"], min: 2, max: 3 },
      suggested_answer: null,
      answers: [],
    });
    fireEvent.click(screen.getByRole("checkbox", { name: /21% standard/ }));
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();
    expect(screen.getByRole("alert")).toHaveTextContent("Pick at least 2 options.");
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(answerMutate).not.toHaveBeenCalled();
  });

  it("shows a non-answerer no controls", () => {
    viewerId = "user-anouk";
    renderCard(singleChoice());
    expect(screen.queryByText("Question for you")).not.toBeInTheDocument();
    expect(screen.queryByRole("radio")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Accept suggestion" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Send" })).not.toBeInTheDocument();
    expect(screen.getByText("Waiting for Brian")).toBeInTheDocument();
  });

  it("folds an answered card with Edit answer, and a closed one without", () => {
    const { unmount } = renderCard(answered());
    expect(screen.getByText("Per invoice line")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Edit answer" })).toBeInTheDocument();
    unmount();

    renderCard(answered({ status: "closed" }));
    expect(screen.getByText("Per invoice line")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Edit answer" })).not.toBeInTheDocument();
  });

  // jsdom has no layout, so this pins the flex contract; the screen proof shows the rendering.
  it("truncates the folded prompt before the answer", () => {
    renderCard(answered());
    const prompt = screen.getByText("Should VAT round per invoice line or on the invoice total?");
    const answer = screen.getByText("Per invoice line");
    // The prompt starts from no width and grows into the room left, up to its own width.
    expect(prompt).toHaveClass("basis-0", "grow", "max-w-max", "min-w-0", "truncate");
    // The answer keeps its own width, uncapped, and truncates only when it alone no longer fits.
    expect(answer).toHaveClass("min-w-0", "truncate");
    expect(answer.className).not.toMatch(/(^|\s)(basis-0|grow|shrink-\S+|max-w-\S+)(\s|$)/);
  });

  it("offers Wake only while it is available", () => {
    const { unmount } = renderCard(answered({ wake_available: false }));
    expect(screen.queryByRole("button", { name: "Wake Pieter" })).not.toBeInTheDocument();
    unmount();

    renderCard(answered({ wake_available: true, unseen_answer_count: 1 }));
    fireEvent.click(screen.getByRole("button", { name: "Wake Pieter" }));
    expect(wakeMutate).toHaveBeenCalledWith({ id: "q-1" }, expect.anything());
  });
});
