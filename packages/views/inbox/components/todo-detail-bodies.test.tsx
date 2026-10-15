import { fireEvent, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { TimelineEntry } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { todoFixture } from "../../test/todo-fixture";
import { TodoIssueBody, TodoMentionBody, TodoRunBody, TodoSystemBody } from "./todo-detail-bodies";

const state = vi.hoisted(() => ({
  timeline: [] as TimelineEntry[],
  tasks: [] as Array<{ id: string; agent_id: string; created_at: string }>,
  pullRequests: [] as unknown[],
}));
const updateAutopilot = vi.hoisted(() => vi.fn());
const submitReply = vi.hoisted(() => vi.fn());
const submitComment = vi.hoisted(() => vi.fn());
const push = vi.hoisted(() => vi.fn());
const replyProps = vi.hoisted(() => [] as Array<Record<string, unknown>>);

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: { queryKey: readonly unknown[] }) => ({
    data: options.queryKey[0] === "tasks" ? state.tasks : { pull_requests: state.pullRequests },
  }),
}));
vi.mock("@multica/core/auth", () => ({
  useAuthStore: (selector: (s: { user: { id: string } }) => unknown) => selector({ user: { id: "user-brian" } }),
}));
vi.mock("@multica/core/autopilots", () => ({ useUpdateAutopilot: () => ({ mutate: updateAutopilot, isPending: false }) }));
vi.mock("@multica/core/github", () => ({ issuePullRequestsOptions: () => ({ queryKey: ["prs"] }) }));
vi.mock("@multica/core/issue-statuses/hooks", () => ({
  useIssueStatuses: () => ({ categoryOf: () => "started", colorOf: () => null, iconOf: () => null }),
}));
vi.mock("@multica/core/issues/queries", () => ({ issueTasksOptions: () => ({ queryKey: ["tasks"] }) }));
vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({ autopilotDetail: (id: string) => `/acme/autopilots/${id}` }),
}));
vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({ getActorName: (type: string) => (type === "agent" ? "Pieter" : "Anouk") }),
}));
vi.mock("../../common/actor-avatar", () => ({ ActorAvatar: () => <span data-testid="avatar" /> }));
vi.mock("../../editor", () => ({ ReadonlyContent: ({ content }: { content: string }) => <p>{content}</p> }));
vi.mock("../../issues/components/comment-input", () => ({ CommentInput: () => <div data-testid="comment-input" /> }));
vi.mock("../../issues/components/pull-request-list", () => ({ PullRequestList: () => <div data-testid="pull-requests" /> }));
vi.mock("../../issues/components/reply-input", () => ({
  ReplyInput: (props: Record<string, unknown>) => {
    replyProps.push(props);
    return <div data-testid="reply-input" />;
  },
}));
vi.mock("../../issues/components/status-icon", () => ({ StatusIcon: () => <span data-testid="status-icon" /> }));
vi.mock("../../issues/components/task-run-labels", () => ({
  runNumbersByTask: (tasks: Array<{ id: string }>) => new Map(tasks.map((task, index) => [task.id, index + 1])),
}));
vi.mock("../../issues/hooks/use-issue-timeline", () => ({
  useIssueTimeline: () => ({ timeline: state.timeline, loading: false, submitReply, submitComment }),
}));
vi.mock("../../issues/utils/status-label", () => ({ useStatusLabel: () => (status: string) => (status === "in_review" ? "In Review" : status) }));
vi.mock("../../navigation", () => ({ useNavigation: () => ({ push }) }));
vi.mock("../../modals/use-issue-limit-upgrade-prompt", () => ({ useIssueLimitUpgradePrompt: () => vi.fn() }));
vi.mock("./autopilot-quota-notice", () => ({ AutopilotQuotaNotice: () => <div data-testid="quota-notice" /> }));
vi.mock("./quick-create-recovery", () => ({
  QuickCreateRecovery: ({ retryable }: { retryable: boolean }) => <div data-testid="quick-create-recovery" data-retryable={String(retryable)} />,
}));

function comment(id: string, content: string, overrides: Partial<TimelineEntry> = {}): TimelineEntry {
  return {
    type: "comment",
    id,
    actor_type: "member",
    actor_id: "member-anouk",
    created_at: "2026-10-15T10:00:00Z",
    content,
    parent_id: null,
    ...overrides,
  };
}

beforeEach(() => {
  state.timeline = [];
  state.tasks = [];
  state.pullRequests = [];
  updateAutopilot.mockClear();
  push.mockClear();
  replyProps.length = 0;
});

describe("TodoMentionBody", () => {
  it("shows the comment before the mention, the mention outlined and a reply box", () => {
    state.timeline = [
      comment("c-1", "Earlier context"),
      comment("c-2", "@Brian can you check the totals?", { actor_type: "agent", actor_id: "agent-1" }),
      comment("c-3", "A later comment"),
    ];
    const { container } = renderWithI18n(
      <TodoMentionBody todo={todoFixture({ kind: "mention", comment_id: "c-2" })} issueId="issue-412" />,
    );
    expect(screen.getByText("Earlier context")).toBeInTheDocument();
    expect(screen.getByText("@Brian can you check the totals?")).toBeInTheDocument();
    expect(screen.queryByText("A later comment")).not.toBeInTheDocument();
    expect(container.querySelector("[data-comment-id='c-2']")?.className).toContain("border-brand");
    expect(container.querySelector("[data-comment-id='c-1']")?.className).not.toContain("border-brand");
    expect(replyProps.at(-1)).toMatchObject({ parentId: "c-2", placeholder: "Reply to Pieter…", avatarId: "user-brian" });
  });

  it("replies in the mention's thread when it is a reply itself", () => {
    state.timeline = [
      comment("root", "Thread start"),
      comment("c-2", "@Brian here", { parent_id: "root" }),
    ];
    renderWithI18n(<TodoMentionBody todo={todoFixture({ kind: "mention", comment_id: "c-2" })} issueId="issue-412" />);
    expect(screen.getByText("Thread start")).toBeInTheDocument();
    expect(replyProps.at(-1)).toMatchObject({ parentId: "root" });
  });

  it("says when the comment is gone", () => {
    renderWithI18n(<TodoMentionBody todo={todoFixture({ kind: "mention", comment_id: "deleted" })} issueId="issue-412" />);
    expect(screen.getByText("This comment can't be shown here.")).toBeInTheDocument();
    expect(screen.queryByTestId("reply-input")).not.toBeInTheDocument();
  });
});

describe("TodoIssueBody", () => {
  it("shows a review's status, that the member created it, its pull requests and the latest comment", () => {
    state.pullRequests = [{ id: "pr-1" }];
    state.timeline = [comment("c-1", "First"), comment("c-2", "Ready for review")];
    renderWithI18n(<TodoIssueBody todo={todoFixture({ kind: "review", issue_status: "in_review", question_id: null })} issueId="issue-412" />);
    expect(screen.getByText("In Review")).toBeInTheDocument();
    expect(screen.getByText(/created by you/)).toBeInTheDocument();
    expect(screen.getByTestId("pull-requests")).toBeInTheDocument();
    expect(screen.getByText("Ready for review")).toBeInTheDocument();
    expect(screen.queryByText("First")).not.toBeInTheDocument();
    expect(screen.getByTestId("comment-input")).toBeInTheDocument();
  });

  it("shows an assignment without pull requests", () => {
    state.pullRequests = [{ id: "pr-1" }];
    renderWithI18n(<TodoIssueBody todo={todoFixture({ kind: "assigned", issue_status: "todo", question_id: null })} issueId="issue-412" />);
    expect(screen.getByText(/assigned to you/)).toBeInTheDocument();
    expect(screen.queryByTestId("pull-requests")).not.toBeInTheDocument();
  });
});

describe("TodoRunBody", () => {
  it("shows the failed run with its number, the agent's last comment and a comment box", () => {
    state.tasks = [
      { id: "t-1", agent_id: "agent-1", created_at: "2026-10-15T08:00:00Z" },
      { id: "t-2", agent_id: "agent-1", created_at: "2026-10-15T09:00:00Z" },
    ];
    state.timeline = [
      comment("c-1", "Pieter's last words", { actor_type: "agent", actor_id: "agent-1" }),
      comment("c-2", "Anouk chimes in"),
    ];
    renderWithI18n(<TodoRunBody todo={todoFixture({ kind: "run_failed", task_id: "t-2", question_id: null })} issueId="issue-412" />);
    expect(screen.getByText("Run #2")).toBeInTheDocument();
    expect(screen.getByText("Failed")).toBeInTheDocument();
    expect(screen.getByText("Pieter's last words")).toBeInTheDocument();
    expect(screen.queryByText("Anouk chimes in")).not.toBeInTheDocument();
    expect(screen.getByTestId("comment-input")).toBeInTheDocument();
  });

  it("shows a blocked agent's latest run", () => {
    state.tasks = [{ id: "t-1", agent_id: "agent-1", created_at: "2026-10-15T08:00:00Z" }];
    renderWithI18n(<TodoRunBody todo={todoFixture({ kind: "agent_blocked", question_id: null })} issueId="issue-412" />);
    expect(screen.getByText("Run #1")).toBeInTheDocument();
    expect(screen.getByText("Blocked")).toBeInTheDocument();
  });
});

describe("TodoSystemBody", () => {
  const paused = todoFixture({
    kind: "autopilot_paused",
    issue_id: null,
    question_id: null,
    autopilot_id: "ap-1",
    title: "Daily triage",
    body: "Auto-paused after 4 of 5 runs failed.",
  });

  it("says what went wrong and resumes the autopilot", () => {
    renderWithI18n(<TodoSystemBody todo={paused} />);
    expect(screen.getByText("Auto-paused after 4 of 5 runs failed.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Resume autopilot" }));
    expect(updateAutopilot).toHaveBeenCalledWith({ id: "ap-1", status: "active" }, expect.anything());
    fireEvent.click(screen.getByRole("button", { name: "View runs" }));
    expect(push).toHaveBeenCalledWith("/acme/autopilots/ap-1");
  });

  it("offers a failed quick create's original input and its retry", () => {
    renderWithI18n(<TodoSystemBody todo={todoFixture({
      kind: "quick_create_failed",
      issue_id: null,
      question_id: null,
      title: "Make a bug for the login loop",
      details: { original_prompt: "Make a bug for the login loop", task_id: "t-1" },
    })} />);
    expect(screen.getByText("Original input")).toBeInTheDocument();
    expect(screen.getByTestId("quick-create-recovery")).toHaveAttribute("data-retryable", "true");
  });

  it("shows the quota notice for a reached quota limit", () => {
    renderWithI18n(<TodoSystemBody todo={todoFixture({ kind: "quota_reached", issue_id: null, question_id: null })} />);
    expect(screen.getByTestId("quota-notice")).toBeInTheDocument();
  });
});
