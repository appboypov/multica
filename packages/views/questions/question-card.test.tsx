import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import { forwardRef, useEffect, useImperativeHandle, useRef } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useQuestionDraftStore } from "@multica/core/questions";
import type * as CoreApi from "@multica/core/api";
import type * as CoreQuestions from "@multica/core/questions";
import type { Attachment, Question, TypedQuestion } from "@multica/core/types";
import type { UploadResult } from "@multica/core/hooks/use-file-upload";
import { renderWithI18n } from "../test/i18n";
import { QuestionCard } from "./question-card";
import { QuestionSetCard } from "./question-set-card";
import { OnePage } from "./answer-all/one-page";

const answerMutate = vi.fn();
const wakeMutate = vi.fn();
const { answerQuestion, uploadQuestionFile, listMembers, listAgents } = vi.hoisted(() => ({
  answerQuestion: vi.fn(),
  uploadQuestionFile: vi.fn(),
  listMembers: vi.fn(),
  listAgents: vi.fn(),
}));
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
vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof CoreApi>()),
  // Answers and uploads go to the server through these two calls.
  api: { answerQuestion, uploadFile: uploadQuestionFile },
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({
    getActorName: (type: string, id: string) =>
      type === "agent" ? "Pieter" : id === "user-brian" ? "Brian" : id === "user-anouk" ? "Anouk" : "Someone",
  }),
}));
vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({ queryKey: ["members"], queryFn: () => listMembers() }),
  agentListOptions: () => ({ queryKey: ["agents"], queryFn: () => listAgents() }),
}));
vi.mock("@multica/core/questions", async (importOriginal) => {
  const actual = await importOriginal<typeof CoreQuestions>();
  const pending = { isPending: false };
  return {
    ...actual,
    // The real hook against the mocked API, so what a success does runs as in the app; the spy records each send.
    useAnswerQuestion: (wsId: string, issueId: string, questionId: string) => {
      const mutation = actual.useAnswerQuestion(wsId, issueId, questionId);
      const mutate: typeof mutation.mutate = (variables, options) => {
        answerMutate(variables, options);
        mutation.mutate(variables, options);
      };
      return { ...mutation, mutate };
    },
    useAskBackQuestion: () => ({ ...pending, mutate: vi.fn() }),
    useReassignQuestion: () => ({ ...pending, mutate: vi.fn() }),
    useWakeQuestionAgent: () => ({ ...pending, mutate: wakeMutate }),
  };
});
// The rich-text editor's contract, without ProseMirror: typing reaches `onUpdate`
// only after the debounce, `flushPendingUpdate` hands back what is still
// pending, an update pending at unmount is emitted only with
// `flushPendingOnUnmount`, a file counts as uploading until it settles into a
// link, and a placeholder drawn for an upload of an earlier mount settles into
// a link in place. An editor gone by the time its upload settles writes nothing.
vi.mock("../editor", () => {
  interface MockEditorProps {
    defaultValue?: string;
    debounceMs?: number;
    flushPendingOnUnmount?: boolean;
    onUpdate?: (markdown: string, base: string) => void;
    onSubmit?: () => void;
    onUploadFile?: (file: File, uploadId: string) => Promise<UploadResult | null>;
    onUploadingChange?: (uploading: boolean) => void;
  }
  return {
    ContentEditor: forwardRef<unknown, MockEditorProps>(function MockContentEditor(props, ref) {
      const live = useRef(props);
      live.current = props;
      const markdown = useRef(props.defaultValue ?? "");
      const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
      const uploads = useRef(0);
      const placeholders = useRef(new Set<string>());
      const alive = useRef(true);
      useEffect(() => {
        alive.current = true;
        live.current.onUploadingChange?.(false);
        return () => {
          alive.current = false;
          if (timer.current === undefined) return;
          clearTimeout(timer.current);
          if (live.current.flushPendingOnUnmount) live.current.onUpdate?.(markdown.current, "");
        };
      }, []);
      const emitLater = () => {
        clearTimeout(timer.current);
        timer.current = setTimeout(() => {
          timer.current = undefined;
          live.current.onUpdate?.(markdown.current, "");
        }, live.current.debounceMs ?? 300);
      };
      useImperativeHandle(ref, () => ({
        flushPendingUpdate: () => {
          if (timer.current === undefined) return null;
          clearTimeout(timer.current);
          timer.current = undefined;
          return markdown.current;
        },
        hasActiveUploads: () => uploads.current > 0,
        insertUploadPlaceholder: ({ uploadId }: { uploadId: string }) => {
          placeholders.current.add(uploadId);
          uploads.current += 1;
          return true;
        },
        settleUploadPlaceholder: (uploadId: string, result: UploadResult) => {
          if (!placeholders.current.delete(uploadId)) return false;
          uploads.current -= 1;
          markdown.current = `${markdown.current} [${result.filename}](${result.markdownLink})`;
          emitLater();
          return true;
        },
        insertMarkdownAtEnd: (md: string) => {
          markdown.current = `${markdown.current}${md}`;
          emitLater();
          return true;
        },
      }));
      return (
        <>
          <textarea
            aria-label="Answer text"
            defaultValue={props.defaultValue}
            onChange={(event) => {
              markdown.current = event.target.value;
              emitLater();
            }}
            onKeyDown={(event) => {
              if (event.key === "Enter" && event.metaKey) live.current.onSubmit?.();
            }}
          />
          <input
            type="file"
            aria-label="Attach to answer text"
            onChange={async (event) => {
              const file = event.target.files?.[0];
              if (!file) return;
              uploads.current += 1;
              live.current.onUploadingChange?.(true);
              const result = await live.current.onUploadFile?.(file, "upload-1");
              if (!alive.current) return;
              uploads.current -= 1;
              if (result) {
                markdown.current = `${markdown.current} [${file.name}](${result.markdownLink})`;
                emitLater();
              }
              live.current.onUploadingChange?.(uploads.current > 0);
            }}
          />
        </>
      );
    }),
  };
});
vi.mock("../common/actor-avatar", () => ({ ActorAvatar: () => <span /> }));

function base(): Omit<TypedQuestion<"single_choice">, "answer_type" | "settings" | "suggested_answer" | "answers"> {
  return {
    id: "q-1",
    workspace_id: "ws-1",
    asker_agent_id: "agent-pieter",
    asked_in_task_id: "task-3",
    ask_id: "q-1",
    ask_position: 0,
    ask_title: null,
    answer_policy: "any_one",
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
    wake_targets: [],
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

function textQuestion(overrides: Partial<TypedQuestion<"text">> = {}): Question {
  return { ...base(), answer_type: "text", settings: { length: "short" }, suggested_answer: null, answers: [], ...overrides };
}

function fileQuestion(): Question {
  return { ...base(), answer_type: "file", settings: { accept: [] }, suggested_answer: null, answers: [] };
}

// A person answer carries the person's ID, which `validateAnswer` holds to the UUID form.
const ANOUK_ID = "4a1d2c3b-5e6f-4a7b-8c9d-0e1f2a3b4c5d";

function personQuestion(): Question {
  return { ...base(), answer_type: "person", settings: { allow: "both" }, suggested_answer: { type: "member", id: ANOUK_ID }, answers: [] };
}

const PLAN_ID = "0b6f7c1e-2a8d-4c3e-9f10-5d2a1b3c4e5f";
const PLAN_LINK = `[plan.pdf](/api/attachments/${PLAN_ID}/download)`;

function upload(id: string, filename: string): Attachment {
  return {
    id,
    filename,
    url: `https://files.example/${filename}`,
    download_url: `/api/attachments/${id}/download`,
    markdown_url: `/api/attachments/${id}/download`,
    size_bytes: 1024,
  } as Attachment;
}

/** An upload the test settles by hand. */
function deferredUpload() {
  const { promise, resolve } = Promise.withResolvers<Attachment>();
  uploadQuestionFile.mockImplementationOnce(() => promise);
  return (attachment: Attachment) => act(async () => resolve(attachment));
}

/** Waits out the editor's debounce, so what was typed is the draft the card holds. */
async function pastDebounce() {
  const { promise, resolve } = Promise.withResolvers<void>();
  setTimeout(resolve, 150);
  await act(() => promise);
}

// One client per test, as the app has one: a card mounted again sees the mutations an earlier one started.
let client: QueryClient;

function renderCard(question: Question) {
  const card = (q: Question) => (
    <QueryClientProvider client={client}>
      <QuestionCard question={q} issueId="issue-412" runNumber={3} askQuestions={[q]} />
    </QueryClientProvider>
  );
  const result = renderWithI18n(card(question));
  return { ...result, rerenderCard: (q: Question) => result.rerender(card(q)) };
}

describe("QuestionCard", () => {
  beforeEach(() => {
    client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    viewerId = "user-brian";
    answerMutate.mockReset();
    answerQuestion.mockReset();
    answerQuestion.mockResolvedValue({});
    wakeMutate.mockReset();
    uploadQuestionFile.mockReset();
    listMembers.mockReset();
    listMembers.mockResolvedValue([]);
    listAgents.mockReset();
    listAgents.mockResolvedValue([]);
    useQuestionDraftStore.setState({ drafts: {} });
  });

  it("sends the suggested answer on Accept suggestion", () => {
    renderCard(singleChoice());
    expect(screen.getByText("Question for you")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Accept suggestion" }));
    expect(answerMutate).toHaveBeenCalledWith(
      { value: { option: "Per invoice line" }, wake: true },
      expect.anything(),
    );
  });

  it("turns Accept suggestion into Send once something is picked", () => {
    renderCard(singleChoice());
    fireEvent.click(screen.getByRole("radio", { name: /On the invoice total/ }));
    expect(screen.queryByRole("button", { name: "Accept suggestion" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(answerMutate).toHaveBeenCalledWith(
      { value: { option: "On the invoice total" }, wake: true },
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
    // Unpicked again, the answer is still an edit below the minimum, not untouched.
    fireEvent.click(screen.getByRole("checkbox", { name: /21% standard/ }));
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();
    expect(screen.getByRole("alert")).toHaveTextContent("Pick at least 2 options.");
  });

  it("sends no options to a Multiple choice whose minimum is 0, once everything is unpicked", () => {
    renderCard({
      ...base(),
      answer_type: "multiple_choice",
      settings: { options: ["21% standard", "9% reduced", "0% exempt"], min: 0, max: 2 },
      suggested_answer: { options: ["9% reduced"] },
      answers: [],
    });
    fireEvent.click(screen.getByRole("checkbox", { name: /21% standard/ }));
    for (const name of [/21% standard/, /9% reduced/]) fireEvent.click(screen.getByRole("checkbox", { name }));
    expect(screen.queryByRole("button", { name: "Accept suggestion" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(answerMutate).toHaveBeenCalledWith({ value: { options: [] }, wake: true }, expect.anything());
  });

  it("shows a suggested Ranking order before Accept suggestion sends it", () => {
    renderCard({
      ...base(),
      answer_type: "ranking",
      settings: { items: ["Speed", "Cost", "Quality"] },
      suggested_answer: { order: ["Quality", "Speed", "Cost"] },
      answers: [],
    });
    const rows = within(screen.getByRole("list", { name: "Order" })).getAllByRole("listitem");
    const shown = rows.map((row) => row.textContent?.match(/Speed|Cost|Quality/)?.[0]);
    expect(shown).toEqual(["Quality", "Speed", "Cost"]);
    fireEvent.click(screen.getByRole("button", { name: "Accept suggestion" }));
    expect(answerMutate).toHaveBeenCalledWith({ value: { order: shown }, wake: true }, expect.anything());
  });

  it("shows a suggested Number in its input before Accept suggestion sends it", () => {
    renderCard({ ...base(), answer_type: "number", settings: { unit: "%" }, suggested_answer: { number: 21 }, answers: [] });
    expect(screen.getByRole("spinbutton", { name: "Number" })).toHaveValue(21);
    fireEvent.click(screen.getByRole("button", { name: "Accept suggestion" }));
    expect(answerMutate).toHaveBeenCalledWith({ value: { number: 21 }, wake: true }, expect.anything());
  });

  it("keeps a suggested Person on top as picked while a search hides it, without listing it twice", async () => {
    listMembers.mockResolvedValue([{ user_id: "user-brian", name: "Brian" }, { user_id: ANOUK_ID, name: "Anouk" }]);
    listAgents.mockResolvedValue([{ id: "agent-pieter", name: "Pieter", archived_at: null }]);
    renderCard(personQuestion());
    const people = () => within(screen.getByRole("listbox")).getAllByRole("option");
    await waitFor(() => expect(people()).toHaveLength(3));

    fireEvent.change(screen.getByRole("textbox", { name: "Search people and agents" }), { target: { value: "Pie" } });
    expect(people().map((row) => [row.textContent?.match(/Brian|Anouk|Pieter/)?.[0], row.getAttribute("aria-selected")])).toEqual([
      ["Anouk", "true"],
      ["Pieter", "false"],
    ]);

    fireEvent.change(screen.getByRole("textbox", { name: "Search people and agents" }), { target: { value: "Ano" } });
    expect(people().map((row) => row.textContent?.match(/Brian|Anouk|Pieter/)?.[0])).toEqual(["Anouk"]);
  });

  it("holds Accept suggestion back until the suggested Person is on screen", async () => {
    const members = Promise.withResolvers<unknown[]>();
    listMembers.mockReturnValue(members.promise);
    renderCard(personQuestion());
    const accept = await screen.findByRole("button", { name: "Accept suggestion" });
    expect(accept).toBeDisabled();
    expect(screen.queryByRole("option", { name: /Anouk/ })).not.toBeInTheDocument();
    fireEvent.click(accept);
    expect(answerMutate).not.toHaveBeenCalled();

    await act(async () => members.resolve([{ user_id: ANOUK_ID, name: "Anouk" }]));
    expect(await screen.findByRole("option", { name: /Anouk/ })).toHaveAttribute("aria-selected", "true");
    await waitFor(() => expect(accept).toBeEnabled());
    fireEvent.click(accept);
    expect(answerMutate).toHaveBeenCalledWith({ value: { type: "member", id: ANOUK_ID }, wake: true }, expect.anything());
  });

  it("keeps Accept suggestion held back when the suggested Person is in neither list", async () => {
    listMembers.mockResolvedValue([{ user_id: "user-brian", name: "Brian" }]);
    renderCard(personQuestion());
    await screen.findByRole("option", { name: /Brian/ });
    expect(screen.getByRole("button", { name: "Accept suggestion" })).toBeDisabled();
  });

  it("holds Send back while one end of an edited suggested range is missing, then sends the edited range", () => {
    const day = (date: string, time: string) => new Date(`${date}T${time}`).toISOString();
    renderCard({
      ...base(),
      answer_type: "date",
      settings: { kind: "range" },
      suggested_answer: { start: day("2026-10-20", "00:00:00"), end: day("2026-10-23", "23:59:59") },
      answers: [],
    });
    const until = screen.getByLabelText("Until");
    expect(until).toHaveValue("2026-10-23");
    fireEvent.change(until, { target: { value: "" } });
    expect(screen.queryByRole("button", { name: "Accept suggestion" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();
    expect(screen.getByRole("alert")).toHaveTextContent("Pick both the start and the end.");
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(answerMutate).not.toHaveBeenCalled();

    fireEvent.change(until, { target: { value: "2026-10-24" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(answerMutate).toHaveBeenCalledWith(
      { value: { start: day("2026-10-20", "00:00:00"), end: day("2026-10-24", "23:59:59") }, wake: true },
      expect.anything(),
    );
  });

  // Scenario: Request changes needs a note
  it("holds back Request changes on an Approval question until Brian writes a note", () => {
    renderCard({ ...base(), answer_type: "approval", settings: { subject: "Release 1.4" }, suggested_answer: null, answers: [] });
    fireEvent.click(screen.getByRole("button", { name: "Request changes" }));
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();
    expect(screen.getByRole("alert")).toHaveTextContent("Request changes needs a note.");
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(answerMutate).not.toHaveBeenCalled();

    fireEvent.change(screen.getByRole("textbox", { name: /What needs to change/ }), { target: { value: "Split the VAT line" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(answerMutate).toHaveBeenCalledWith(
      { value: { decision: "request_changes" }, note: "Split the VAT line", wake: true },
      expect.anything(),
    );
  });

  // Scenario: Single choice with Other
  it("sends Other with its text on a Single choice question with Other allowed", () => {
    renderCard(singleChoice());
    fireEvent.click(screen.getByRole("radio", { name: /^Other/ }));
    fireEvent.change(screen.getByRole("textbox", { name: "Your answer for Other" }), { target: { value: "per invoice" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(answerMutate).toHaveBeenCalledWith({ value: { other: "per invoice" }, wake: true }, expect.anything());
  });

  // Scenario: Confirm facts with a correction
  it("holds back a Confirm facts answer until the Not correct fact has its correction", () => {
    const facts = ["VAT is 21%", "Invoices are monthly", "Credit notes are separate"];
    renderCard({ ...base(), answer_type: "confirm_facts", settings: { facts }, suggested_answer: null, answers: [] });
    fireEvent.click(within(screen.getByRole("group", { name: facts[0] })).getByRole("button", { name: "Correct" }));
    fireEvent.click(within(screen.getByRole("group", { name: facts[1] })).getByRole("button", { name: "Correct" }));
    fireEvent.click(within(screen.getByRole("group", { name: facts[2] })).getByRole("button", { name: "Not correct" }));
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();
    expect(screen.getByRole("alert")).toHaveTextContent("Write the correction of statement 3 to send.");
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(answerMutate).not.toHaveBeenCalled();

    fireEvent.change(screen.getByRole("textbox", { name: /Correction of/ }), { target: { value: "They share the invoice" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(answerMutate).toHaveBeenCalledWith(
      { value: { facts: [{ correct: true }, { correct: true }, { correct: false, correction: "They share the invoice" }] }, wake: true },
      expect.anything(),
    );
  });

  describe("on 2026-10-15", () => {
    beforeEach(() => {
      vi.useFakeTimers({ toFake: ["Date"] });
      vi.setSystemTime(new Date("2026-10-15T12:00:00"));
      return () => vi.useRealTimers();
    });

    // Scenario: An open question passes its due date
    it("shows an overdue badge on an open question due 2026-10-14", () => {
      renderCard(singleChoice({ due_date: "2026-10-14" }));
      expect(screen.getByText("Overdue")).toBeInTheDocument();
    });

    // Scenario: An answered question shows no overdue badge
    it("shows no overdue badge on a question due 2026-10-14 that Brian answered on 2026-10-13", () => {
      renderCard(answered({ due_date: "2026-10-14" }));
      expect(screen.getByText("Per invoice line")).toBeInTheDocument();
      expect(screen.queryByText("Overdue")).not.toBeInTheDocument();
    });
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

  it("offers Wake only while it is available", () => {
    const { unmount } = renderCard(answered({ wake_available: false }));
    expect(screen.queryByRole("button", { name: "Wake Pieter" })).not.toBeInTheDocument();
    unmount();

    renderCard(answered({
      wake_available: true,
      unseen_answer_count: 1,
      wake_targets: [{ agent_id: "agent-pieter", issue_id: "issue-412", unseen_answer_count: 1, wake_available: true }],
    }));
    fireEvent.click(screen.getByRole("button", { name: "Wake Pieter" }));
    expect(wakeMutate).toHaveBeenCalledWith(
      { id: "q-1", target: { agentId: "agent-pieter", issueId: "issue-412" } },
      expect.anything(),
    );
  });

  // Scenario: Brian declines
  it("declines with the note as its reason, waking Pieter", () => {
    renderCard(singleChoice());
    fireEvent.click(screen.getByRole("button", { name: "Add a note" }));
    fireEvent.change(screen.getByRole("textbox", { name: "Note to your answer" }), { target: { value: "Not my area" } });
    fireEvent.click(screen.getByRole("button", { name: "Decline to answer" }));
    expect(answerMutate).toHaveBeenCalledWith({ value: { declined: true }, note: "Not my area", wake: true }, expect.anything());
  });

  // Scenario: Brian unchecks Wake
  it("sends without waking Pieter once Brian unchecks Wake Pieter", () => {
    renderCard(singleChoice());
    const wake = screen.getByRole("checkbox", { name: "Wake Pieter" });
    expect(wake).toHaveAttribute("aria-checked", "true");
    fireEvent.click(wake);
    fireEvent.click(screen.getByRole("button", { name: "Accept suggestion" }));
    expect(answerMutate).toHaveBeenCalledWith({ value: { option: "Per invoice line" } }, expect.anything());
  });

  // Scenario: Brian answers the last question
  it("offers Wake Pieter in a set only on the last question still open for Brian", () => {
    const second = (first: Question) => [first, singleChoice({ id: "q-2", ask_id: "q-1", ask_position: 1, ask_title: "VAT", prompt: "Which VAT rate applies?" })];
    const set = (questions: Question[]) => (
      <QueryClientProvider client={client}>
        <QuestionSetCard questions={questions} issueId="issue-412" runNumber={3} />
      </QueryClientProvider>
    );
    const { rerender } = renderWithI18n(set(second(singleChoice({ ask_title: "VAT" }))));
    expect(screen.queryByRole("checkbox", { name: "Wake Pieter" })).not.toBeInTheDocument();

    rerender(set(second(answered({ ask_title: "VAT" }))));
    fireEvent.click(screen.getByRole("tab", { name: /Which VAT rate applies/ }));
    expect(screen.getByRole("checkbox", { name: "Wake Pieter" })).toHaveAttribute("aria-checked", "true");
    fireEvent.click(screen.getByRole("button", { name: "Accept suggestion" }));
    expect(answerMutate).toHaveBeenCalledWith({ value: { option: "Per invoice line" }, wake: true }, expect.anything());
  });

  // Scenario: Brian answers the last question, on Answer all's One page
  it("offers Wake Pieter on One page for a one-question ask and a set's last question still open for Brian", () => {
    const set = (first: Question) => [first, singleChoice({ id: "q-2", ask_id: "q-1", ask_position: 1, ask_title: "VAT", prompt: "Which VAT rate applies?" })];
    const entry = (id: string, askId: string) => ({ id, askId, askTitle: askId === "q-1" ? "VAT" : null, issueId: "issue-412", identifier: "WORK-412" });
    const session = [entry("s-1", "s-1"), entry("q-1", "q-1"), entry("q-2", "q-1")];
    const page = (questions: Question[]) => (
      <QueryClientProvider client={client}>
        <OnePage session={session} questions={new Map(questions.map((q) => [q.id, q]))} focusedId="s-1" onFocus={vi.fn()} />
      </QueryClientProvider>
    );
    const card = (id: string) => within(document.querySelector<HTMLElement>(`[data-question-id="${id}"]`)!);
    const single = singleChoice({ id: "s-1", ask_id: "s-1", prompt: "Which ledger books the VAT?" });

    const { rerender } = renderWithI18n(page([single, ...set(singleChoice({ ask_title: "VAT" }))]));
    expect(screen.getAllByRole("checkbox", { name: "Wake Pieter" })).toHaveLength(1);
    expect(card("s-1").getByRole("checkbox", { name: "Wake Pieter" })).toHaveAttribute("aria-checked", "true");

    rerender(page([single, ...set(answered({ ask_title: "VAT" }))]));
    expect(screen.getAllByRole("checkbox", { name: "Wake Pieter" })).toHaveLength(2);
    expect(card("q-2").getByRole("checkbox", { name: "Wake Pieter" })).toHaveAttribute("aria-checked", "true");
    fireEvent.click(card("q-2").getByRole("button", { name: "Accept suggestion" }));
    expect(answerMutate).toHaveBeenCalledWith({ value: { option: "Per invoice line" }, wake: true }, expect.anything());
  });

  it("keeps the unsent answer, note and ask back when the card unmounts, and drops them once sent", async () => {
    const first = renderCard(singleChoice());
    fireEvent.click(screen.getByRole("radio", { name: /On the invoice total/ }));
    fireEvent.click(screen.getByRole("button", { name: "Add a note" }));
    fireEvent.change(screen.getByRole("textbox", { name: "Note to your answer" }), { target: { value: "Totals match the ledger" } });
    fireEvent.click(screen.getByRole("button", { name: "Ask back" }));
    fireEvent.change(screen.getByRole("textbox", { name: "Ask Pieter back" }), { target: { value: "Which invoices?" } });
    // The virtualized timeline unmounts a card scrolled out of view.
    first.unmount();

    const second = renderCard(singleChoice());
    expect(screen.getByRole("radio", { name: /On the invoice total/ })).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("textbox", { name: "Note to your answer" })).toHaveValue("Totals match the ledger");
    expect(screen.getByRole("textbox", { name: "Ask Pieter back" })).toHaveValue("Which invoices?");

    // Closing the ask-back box keeps its text; sending the answer drops it with the rest.
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    await waitFor(() => expect(answerQuestion).toHaveBeenCalledWith("q-1", { value: { option: "On the invoice total" }, note: "Totals match the ledger", wake: true }));
    await waitFor(() => expect(screen.getByRole("radio", { name: /On the invoice total/ })).toHaveAttribute("aria-checked", "false"));
    second.unmount();

    renderCard(singleChoice());
    expect(screen.getByRole("radio", { name: /On the invoice total/ })).toHaveAttribute("aria-checked", "false");
    expect(screen.queryByRole("textbox", { name: "Note to your answer" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Accept suggestion" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Ask back" }));
    expect(screen.getByRole("textbox", { name: "Ask Pieter back" })).toHaveValue("");
  });

  it("ends a sent edit even when the card unmounted before the server replied", async () => {
    const reply = Promise.withResolvers<unknown>();
    answerQuestion.mockReturnValueOnce(reply.promise);
    const first = renderCard(answered());
    fireEvent.click(screen.getByRole("button", { name: "Edit answer" }));
    fireEvent.click(screen.getByRole("radio", { name: /On the invoice total/ }));
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    await waitFor(() => expect(answerQuestion).toHaveBeenCalledWith("q-1", { value: { option: "On the invoice total" } }));
    // Scrolled out of view while the edit is on its way.
    first.unmount();
    reply.resolve({});

    renderCard(answered());
    await waitFor(() => expect(screen.getByRole("button", { name: "Edit answer" })).toBeInTheDocument());
    expect(screen.queryByRole("radio")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Send" })).not.toBeInTheDocument();
  });

  it("offers Edit answer on an Any-one question only to the answerer who answered it", () => {
    const anyOne = singleChoice({
      status: "answered",
      answerers: [
        { user_id: "user-brian", assigned_at: "2026-10-13T10:00:00Z" },
        { user_id: "user-anouk", assigned_at: "2026-10-13T10:00:00Z" },
      ],
      answers: [{
        id: "a-2",
        answerer_user_id: "user-anouk",
        value: { option: "On the invoice total" },
        display_text: "On the invoice total",
        note: null,
        created_at: "2026-10-13T10:06:00Z",
        updated_at: "2026-10-13T10:06:00Z",
      }],
    });
    const brian = renderCard(anyOne);
    expect(screen.getByText("On the invoice total")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Edit answer" })).not.toBeInTheDocument();
    brian.unmount();

    viewerId = "user-anouk";
    renderCard(anyOne);
    expect(screen.getByRole("button", { name: "Edit answer" })).toBeInTheDocument();
  });

  it.each([
    ["closed", answered({ status: "closed" })],
    ["withdrawn", answered({ status: "withdrawn" })],
    ["reassigned away", answered({ answerers: [{ user_id: "user-anouk", assigned_at: "2026-10-13T10:10:00Z" }] })],
  ])("ends an open edit once the question is %s", (_, update) => {
    const { rerenderCard } = renderCard(answered());
    fireEvent.click(screen.getByRole("button", { name: "Edit answer" }));
    expect(screen.getByRole("radio", { name: /Per invoice line/ })).toHaveAttribute("aria-checked", "true");

    rerenderCard(update);
    expect(screen.queryByRole("radio")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Send" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Edit answer" })).not.toBeInTheDocument();

    // The edit ended rather than hid: the answered card comes back folded.
    rerenderCard(answered());
    expect(screen.queryByRole("radio")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Edit answer" })).toBeInTheDocument();
  });

  it.each([
    ["⌘↵", () => fireEvent.keyDown(screen.getByRole("textbox", { name: "Answer text" }), { key: "Enter", metaKey: true })],
    ["Send", () => fireEvent.click(screen.getByRole("button", { name: "Send" }))],
  ])("sends the text typed just before %s, not the draft the editor reported before it", async (_, submit) => {
    renderCard(textQuestion());
    const editor = screen.getByRole("textbox", { name: "Answer text" });
    fireEvent.change(editor, { target: { value: "Round per line" } });
    await waitFor(() => expect(screen.getByRole("button", { name: "Send" })).toBeEnabled());

    // Within the editor's debounce: the card has not heard of this text yet.
    fireEvent.change(editor, { target: { value: "Round per line, then on the total" } });
    submit();

    expect(answerMutate).toHaveBeenCalledTimes(1);
    expect(answerMutate).toHaveBeenCalledWith({ value: { text: "Round per line, then on the total" }, wake: true }, expect.anything());
  });

  it("keeps the text typed just before the card scrolls out of the timeline", () => {
    const first = renderCard(textQuestion());
    // Within the editor's debounce: the card has not heard of this text yet.
    fireEvent.change(screen.getByRole("textbox", { name: "Answer text" }), { target: { value: "Round per line" } });
    first.unmount();

    renderCard(textQuestion());
    expect(screen.getByRole("textbox", { name: "Answer text" })).toHaveValue("Round per line");
  });

  it("waits for every picked file before Send sends them", async () => {
    const { container } = renderCard(fileQuestion());
    const settleFirst = deferredUpload();
    const settleSecond = deferredUpload();
    const files = [new File(["a"], "a.pdf"), new File(["b"], "b.pdf")];
    fireEvent.change(container.querySelector("input[type=file]")!, { target: { files } });

    const [aId, bId] = ["1c2d3e4f-5a6b-4c7d-8e9f-0a1b2c3d4e5f", "2d3e4f5a-6b7c-4d8e-9f0a-1b2c3d4e5f60"];
    await settleFirst(upload(aId, "a.pdf"));
    expect(screen.getByText("a.pdf")).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("Send waits until the upload finishes");
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(answerMutate).not.toHaveBeenCalled();

    await settleSecond(upload(bId, "b.pdf"));
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(answerMutate).toHaveBeenCalledWith({ value: { attachment_ids: [aId, bId] }, wake: true }, expect.anything());
  });

  it("waits for a file uploading into the text before Send or ⌘↵ sends it", async () => {
    renderCard(textQuestion());
    const editor = screen.getByRole("textbox", { name: "Answer text" });
    fireEvent.change(editor, { target: { value: "The plan:" } });
    await waitFor(() => expect(screen.getByRole("button", { name: "Send" })).toBeEnabled());

    const settle = deferredUpload();
    fireEvent.change(screen.getByLabelText("Attach to answer text"), { target: { files: [new File(["p"], "plan.pdf")] } });
    expect(await screen.findByRole("status")).toHaveTextContent("Send waits until the upload finishes");
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();
    fireEvent.keyDown(editor, { key: "Enter", metaKey: true });
    expect(answerMutate).not.toHaveBeenCalled();

    await settle(upload(PLAN_ID, "plan.pdf"));
    fireEvent.keyDown(editor, { key: "Enter", metaKey: true });
    expect(answerMutate).toHaveBeenCalledWith(
      { value: { text: `The plan: ${PLAN_LINK}`, attachment_ids: [PLAN_ID] }, wake: true },
      expect.anything(),
    );
  });

  it("waits after a remount for a file the earlier card still uploads, and sends it with the files picked since", async () => {
    const first = renderCard(fileQuestion());
    const settleFirst = deferredUpload();
    fireEvent.change(first.container.querySelector("input[type=file]")!, { target: { files: [new File(["a"], "a.pdf")] } });
    // Scrolled out of the timeline while a.pdf uploads.
    first.unmount();

    const second = renderCard(fileQuestion());
    const settleSecond = deferredUpload();
    fireEvent.change(second.container.querySelector("input[type=file]")!, { target: { files: [new File(["b"], "b.pdf")] } });
    const [aId, bId] = ["1c2d3e4f-5a6b-4c7d-8e9f-0a1b2c3d4e5f", "2d3e4f5a-6b7c-4d8e-9f0a-1b2c3d4e5f60"];
    await settleSecond(upload(bId, "b.pdf"));
    expect(screen.getByRole("status")).toHaveTextContent("Send waits until the upload finishes");
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(answerMutate).not.toHaveBeenCalled();

    // The earlier card's upload joins the files as they are now.
    await settleFirst(upload(aId, "a.pdf"));
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Remove a.pdf" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Remove b.pdf" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(answerMutate).toHaveBeenCalledWith({ value: { attachment_ids: [bId, aId] }, wake: true }, expect.anything());
  });

  it("waits after a remount for a file the earlier card still uploads into the text", async () => {
    const first = renderCard(textQuestion());
    fireEvent.change(screen.getByRole("textbox", { name: "Answer text" }), { target: { value: "The plan:" } });
    await pastDebounce();
    const settle = deferredUpload();
    fireEvent.change(screen.getByLabelText("Attach to answer text"), { target: { files: [new File(["p"], "plan.pdf")] } });
    first.unmount();

    renderCard(textQuestion());
    const editor = screen.getByRole("textbox", { name: "Answer text" });
    expect(await screen.findByRole("status")).toHaveTextContent("Send waits until the upload finishes");
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();
    fireEvent.keyDown(editor, { key: "Enter", metaKey: true });
    expect(answerMutate).not.toHaveBeenCalled();

    await settle(upload(PLAN_ID, "plan.pdf"));
    fireEvent.keyDown(editor, { key: "Enter", metaKey: true });
    expect(answerMutate).toHaveBeenCalledWith(
      { value: { text: `The plan: ${PLAN_LINK}`, attachment_ids: [PLAN_ID] }, wake: true },
      expect.anything(),
    );
  });

  it("shows a send still on its way after a remount, and keeps what was written since once it lands", async () => {
    const reply = Promise.withResolvers<unknown>();
    answerQuestion.mockReturnValueOnce(reply.promise);
    const first = renderCard(singleChoice());
    fireEvent.click(screen.getByRole("radio", { name: /On the invoice total/ }));
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    await waitFor(() => expect(answerQuestion).toHaveBeenCalledTimes(1));
    first.unmount();

    renderCard(singleChoice());
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(answerQuestion).toHaveBeenCalledTimes(1);

    // Written after the send started, so its success leaves it.
    fireEvent.click(screen.getByRole("button", { name: "Ask back" }));
    fireEvent.change(screen.getByRole("textbox", { name: "Ask Pieter back" }), { target: { value: "Which invoices?" } });
    await act(async () => reply.resolve({}));
    await waitFor(() => expect(screen.getByRole("radio", { name: /On the invoice total/ })).toHaveAttribute("aria-checked", "false"));
    expect(screen.getByRole("textbox", { name: "Ask Pieter back" })).toHaveValue("Which invoices?");
  });

  it("names each file of an answer being edited, so their Remove buttons differ", () => {
    const [aId, bId] = ["1c2d3e4f-5a6b-4c7d-8e9f-0a1b2c3d4e5f", "2d3e4f5a-6b7c-4d8e-9f0a-1b2c3d4e5f60"];
    renderCard({
      ...fileQuestion(),
      status: "answered",
      answers: [{
        id: "a-1",
        answerer_user_id: "user-brian",
        value: { attachment_ids: [aId, bId] },
        display_text: "2 files",
        note: null,
        created_at: "2026-10-13T10:05:00Z",
        updated_at: "2026-10-13T10:05:00Z",
      }],
    } as Question);
    fireEvent.click(screen.getByRole("button", { name: "Edit answer" }));
    const names = screen.getAllByRole("button", { name: /^Remove / }).map((button) => button.getAttribute("aria-label"));
    expect(names).toEqual(["Remove Attached file 1", "Remove Attached file 2"]);
  });

  it.each([
    ["keeps the file of an edited text answer while its link stays", `Round per line, see ${PLAN_LINK}`, [PLAN_ID]],
    ["drops the file of an edited text answer once its link is removed", "Round per line", undefined],
  ])("%s", async (_, edited, attachmentIds) => {
    renderCard(textQuestion({
      status: "answered",
      answers: [{
        id: "a-1",
        answerer_user_id: "user-brian",
        value: { text: `Per line, see ${PLAN_LINK}`, attachment_ids: [PLAN_ID] },
        display_text: "Per line, see plan.pdf",
        note: null,
        created_at: "2026-10-13T10:05:00Z",
        updated_at: "2026-10-13T10:05:00Z",
      }],
    }));
    fireEvent.click(screen.getByRole("button", { name: "Edit answer" }));
    fireEvent.change(screen.getByRole("textbox", { name: "Answer text" }), { target: { value: edited } });
    await pastDebounce();
    fireEvent.click(screen.getByRole("button", { name: "Send" }));

    const value = attachmentIds ? { text: edited, attachment_ids: attachmentIds } : { text: edited };
    expect(answerMutate).toHaveBeenCalledWith({ value }, expect.anything());
  });
});
