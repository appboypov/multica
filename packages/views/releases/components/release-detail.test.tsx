import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import type * as CoreAPI from "@multica/core/api";
import type * as CorePaths from "@multica/core/paths";
import type * as CoreIssueStatuses from "@multica/core/issue-statuses";
import type * as CorePermissions from "@multica/core/permissions";
import { lineStates, toLine } from "@multica/core/releases";
import type {
  Agent,
  Release,
  ReleaseComment,
  ReleaseIssue,
  ReleaseOwnerActivityEntry,
  Repository,
  Squad,
  Worklane,
} from "@multica/core/types";
import { NavigationProvider } from "../../navigation";
import { renderWithI18n } from "../../test/i18n";
import { ReleaseColumn } from "./release-column";
import { ReleaseDetailRoute } from "./release-detail-route";
import { ReleaseRow } from "./release-row";

const api = vi.hoisted(() => ({
  getRelease: vi.fn(),
  listReleases: vi.fn(),
  listRepositories: vi.fn(),
  listWorklanes: vi.fn(),
  listReleaseProgress: vi.fn(),
  listReleaseIssues: vi.fn(),
  listReleaseComments: vi.fn(),
  listReleaseActivity: vi.fn(),
  setReleaseOwner: vi.fn(),
  createReleaseComment: vi.fn(),
  listAgents: vi.fn(),
  listSquads: vi.fn(),
}));
const push = vi.hoisted(() => vi.fn());
const NAMES: Record<string, string> = {
  "u-brian": "Brian",
  "agent-pieter": "Pieter",
  "agent-kees": "Kees",
  "squad-1": "Release squad",
};

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/api", async (importOriginal) => ({ ...await importOriginal<typeof CoreAPI>(), api }));
vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof CorePaths>();
  return { ...actual, useWorkspacePaths: () => actual.paths.workspace("acme") };
});
vi.mock("@multica/core/permissions", async (importOriginal) => ({
  ...await importOriginal<typeof CorePermissions>(),
  useCurrentMember: () => ({ userId: "u-brian", role: "member", member: null, isLoading: false }),
}));
vi.mock("@multica/core/issue-statuses", async (importOriginal) => ({
  ...await importOriginal<typeof CoreIssueStatuses>(),
  useIssueStatuses: () => ({
    categoryOf: (key: string) => (key === "done" ? "completed" : "unstarted"),
    colorOf: () => null,
    iconOf: () => null,
    labelOf: (key: string) => (key === "done" ? "Done" : "Todo"),
  }),
}));
vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({ getActorName: (_type: string, id: string) => NAMES[id] ?? "Unknown" }),
}));
vi.mock("../../common/actor-avatar", () => ({ ActorAvatar: () => <span data-testid="avatar" /> }));
vi.mock("../../editor/readonly-content", () => ({
  ReadonlyContent: ({ content }: { content: string }) => <div>{content}</div>,
}));
vi.mock("./release-menu", () => ({ ReleaseMenu: () => null }));

const repository = {
  id: "repo-1",
  full_name: "appboypov/rebl",
  last_tag: "1.0.0",
  default_branch: "main",
  version_policy: "semver",
} as unknown as Repository;

const base = (overrides: Partial<Release> & Pick<Release, "id" | "version" | "title" | "position">): Release => ({
  repository_id: "repo-1",
  description: "",
  bump: "minor",
  shipped_at: null,
  created_at: "",
  updated_at: "",
  milestones: [],
  owner_type: null,
  owner_id: null,
  owner_name: null,
  owner_avatar_url: null,
  ...overrides,
});
const shipped = base({
  id: "r-100",
  position: 0,
  version: "1.0.0",
  title: "First production release",
  shipped_at: "2026-01-10T00:00:00Z",
  owner_type: "agent",
  owner_id: "agent-pieter",
  owner_name: "Pieter",
});
const next = base({ id: "r-101", position: 1, version: "1.0.1", title: "Fixes", bump: "patch" });
const planned = base({
  id: "r-110",
  position: 2,
  version: "1.1.0",
  title: "Account and profile",
  description: "Fans make an account, sign in and keep their own profile with a photo.",
});

const lane = (id: string, name: string, position: number, is_main = false) =>
  ({ id, name, position, is_main, repository_id: "repo-1" }) as Worklane;
const worklanes = [lane("w-main", "main", 0, true), lane("w-accounts", "Fan accounts and push", 1)];

const issue = (overrides: Partial<ReleaseIssue> & Pick<ReleaseIssue, "id" | "identifier" | "title">): ReleaseIssue => ({
  status: "todo",
  assignee_type: null,
  assignee_id: null,
  parent_issue_id: null,
  worklane_id: "w-accounts",
  worklane_name: "Fan accounts and push",
  pull_requests: [],
  ...overrides,
});
const issues = [
  issue({ id: "i-501", identifier: "WORK-501", title: "Sign up with email", status: "done", assignee_type: "agent", assignee_id: "agent-pieter" }),
  issue({ id: "i-502", identifier: "WORK-502", title: "Profile photo" }),
];

const agents = [
  { id: "agent-pieter", name: "Pieter", owner_id: "u-brian", permission_mode: "private", visibility: "workspace", archived_at: null },
  { id: "agent-kees", name: "Kees", owner_id: "u-other", permission_mode: "private", visibility: "private", archived_at: null },
  { id: "agent-old", name: "Old bot", owner_id: "u-brian", permission_mode: "private", visibility: "workspace", archived_at: "2026-01-01T00:00:00Z" },
] as unknown as Agent[];
const squads = [{ id: "squad-1", name: "Release squad", archived_at: null }] as unknown as Squad[];

const ownerSet: ReleaseOwnerActivityEntry = {
  id: "e-1",
  actor_type: "member",
  actor_id: "u-brian",
  created_at: "2026-02-01T10:00:00Z",
  action: "release_owner_changed",
  details: {
    release_id: "r-110",
    version: "1.1.0",
    title: "Account and profile",
    owner_type: "agent",
    owner_id: "agent-pieter",
    owner_name: "Pieter",
    previous_owner_type: null,
    previous_owner_id: null,
    previous_owner_name: null,
  },
};
const comment = (overrides: Partial<ReleaseComment> & Pick<ReleaseComment, "id" | "content" | "created_at">): ReleaseComment => ({
  release_id: "r-110",
  author_type: "member",
  author_id: "u-brian",
  parent_id: null,
  attachments: [],
  updated_at: overrides.created_at,
  source_task_id: null,
  ...overrides,
});
const pieterComment = comment({
  id: "c-1",
  author_type: "agent",
  author_id: "agent-pieter",
  content: "I start with WORK-501.",
  created_at: "2026-02-01T11:00:00Z",
});

function seed(release: Release, overrides: { issues?: ReleaseIssue[]; comments?: ReleaseComment[]; entries?: ReleaseOwnerActivityEntry[] } = {}) {
  api.getRelease.mockResolvedValue(release);
  api.listReleases.mockResolvedValue({ releases: [shipped, next, planned] });
  api.listRepositories.mockResolvedValue({ repositories: [repository] });
  api.listWorklanes.mockResolvedValue({ worklanes, total: worklanes.length });
  api.listReleaseProgress.mockResolvedValue({
    rows: [{ worklane_id: "w-accounts", release_id: "r-110", done: 1, total: 2 }],
    releases: [],
  });
  api.listReleaseIssues.mockResolvedValue({ issues: overrides.issues ?? issues });
  api.listReleaseComments.mockResolvedValue(overrides.comments ?? []);
  api.listReleaseActivity.mockResolvedValue({ entries: overrides.entries ?? [] });
  api.listAgents.mockResolvedValue(agents);
  api.listSquads.mockResolvedValue(squads);
}

function mount(ui: ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  const url = new URL("/acme/releases/r-110", "https://example.test");
  renderWithI18n(
    <QueryClientProvider client={qc}>
      <NavigationProvider
        value={{
          pathname: url.pathname,
          searchParams: url.searchParams,
          hash: "",
          push,
          replace: push,
          back: () => {},
          getShareableUrl: (v) => v,
        }}
      >
        {ui}
      </NavigationProvider>
    </QueryClientProvider>,
  );
  return qc;
}

beforeEach(() => vi.resetAllMocks());

describe("release page", () => {
  it("shows the release, its owner state and its issues in order with their issue pages", async () => {
    const user = userEvent.setup();
    seed(planned);
    mount(<ReleaseDetailRoute releaseId="r-110" />);

    expect(await screen.findByRole("heading", { name: "Account and profile" })).toBeTruthy();
    expect(await screen.findByText("appboypov/rebl")).toBeTruthy();
    expect(screen.getByText("v1.1.0")).toBeTruthy();
    expect(await screen.findByText("Planned")).toBeTruthy();
    expect(screen.getByText(planned.description)).toBeTruthy();
    expect(screen.getByRole("button", { name: "Owner No owner" })).toBeTruthy();

    const first = await screen.findByRole("link", { name: /WORK-501/ });
    const second = screen.getByRole("link", { name: /WORK-502/ });
    expect(first.compareDocumentPosition(second) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(within(first).getByText("Done")).toBeTruthy();
    expect(within(first).getByText("Pieter")).toBeTruthy();
    expect(within(first).getByText("Fan accounts and push")).toBeTruthy();
    expect(within(second).getByText("Unassigned")).toBeTruthy();

    await user.click(second);
    expect(push).toHaveBeenCalledWith("/acme/issues/i-502");
  });

  it("says a release has no issues yet", async () => {
    seed(planned, { issues: [] });
    mount(<ReleaseDetailRoute releaseId="r-110" />);
    expect(await screen.findByText("No issues in this release yet.")).toBeTruthy();
  });

  it("shows a failed read of the issues as an error with retry, never as no issues", async () => {
    const user = userEvent.setup();
    seed(planned);
    api.listReleaseIssues.mockRejectedValue(new Error("network down"));
    mount(<ReleaseDetailRoute releaseId="r-110" />);

    expect(await screen.findByText("Couldn't load the issues of this release.")).toBeTruthy();
    expect(screen.queryByText("No issues in this release yet.")).toBeNull();

    api.listReleaseIssues.mockResolvedValue({ issues });
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByRole("link", { name: /WORK-501/ })).toBeTruthy();
    expect(screen.queryByText("Couldn't load the issues of this release.")).toBeNull();
  });

  it("opens the Timeline of the first worklane holding the release with it in view", async () => {
    seed(planned);
    mount(<ReleaseDetailRoute releaseId="r-110" />);
    const link = await screen.findByRole("link", { name: "Open on Timeline" });
    expect(link.getAttribute("href")).toBe("/acme/worklanes/w-accounts?release=r-110");
  });

  it("shows a shipped release's owner without an Owner picker", async () => {
    seed(shipped, { issues: [] });
    mount(<ReleaseDetailRoute releaseId="r-100" />);
    expect(await screen.findByRole("heading", { name: "First production release" })).toBeTruthy();
    expect(screen.getByText("Pieter")).toBeTruthy();
    expect(screen.queryByRole("button", { name: /^Owner/ })).toBeNull();
  });

  it("offers No owner, assignable agents and squads but no members, and saves the pick", async () => {
    const user = userEvent.setup();
    seed(planned);
    const owned = { ...planned, owner_type: "agent" as const, owner_id: "agent-pieter", owner_name: "Pieter" };
    api.setReleaseOwner.mockResolvedValue({ releases: [shipped, next, owned] });
    mount(<ReleaseDetailRoute releaseId="r-110" />);

    await user.click(await screen.findByRole("button", { name: "Owner No owner" }));
    expect(await screen.findByRole("button", { name: "No owner" })).toBeTruthy();
    expect(await screen.findByRole("button", { name: "Release squad" })).toBeTruthy();
    expect((screen.getByRole("button", { name: "Kees" }) as HTMLButtonElement).disabled).toBe(true);
    expect(screen.queryByRole("button", { name: /Brian/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /Old bot/ })).toBeNull();

    await user.click(screen.getByRole("button", { name: "Pieter" }));
    await waitFor(() =>
      expect(api.setReleaseOwner).toHaveBeenCalledWith("r-110", { owner_type: "agent", owner_id: "agent-pieter" }),
    );
    expect(await screen.findByRole("button", { name: "Owner Pieter" })).toBeTruthy();
  });

  it("shows a missing release as not found", async () => {
    const actual = await vi.importActual<typeof CoreAPI>("@multica/core/api");
    seed(planned);
    api.getRelease.mockRejectedValue(new actual.ApiError("release not found", 404, "Not Found"));
    mount(<ReleaseDetailRoute releaseId="gone" />);
    expect(await screen.findByText("This release doesn't exist or was removed.")).toBeTruthy();
    expect(screen.getByRole("link", { name: "Back to releases" }).getAttribute("href")).toBe("/acme/releases");
  });
});

describe("release discussion", () => {
  it("keeps nested agent replies in the root thread in time order", async () => {
    const reply = comment({ id: "c-2", parent_id: "c-1", content: "Review is open.", created_at: "2026-02-01T12:00:00Z" });
    const nested = comment({ id: "c-3", parent_id: "c-2", content: "Review passed.", created_at: "2026-02-01T13:00:00Z" });
    seed(planned, { comments: [nested, pieterComment, reply] });
    mount(<ReleaseDetailRoute releaseId="r-110" />);
    const root = await screen.findByText(pieterComment.content);
    const thread = within(root.closest("li") as HTMLElement);
    const first = await thread.findByText(reply.content);
    const last = await thread.findByText(nested.content);
    expect(first.compareDocumentPosition(last) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("shows the owner change before a later comment, and replies under their root", async () => {
    const user = userEvent.setup();
    seed(planned, { comments: [pieterComment], entries: [ownerSet] });
    const reply = comment({
      id: "c-2",
      parent_id: "c-1",
      content: "Start WORK-502 once WORK-501's pull request is open.",
      created_at: "2026-02-01T12:00:00Z",
    });
    api.createReleaseComment.mockResolvedValue(reply);
    mount(<ReleaseDetailRoute releaseId="r-110" />);

    const sentence = await screen.findByText("Brian made Pieter owner of v1.1.0 Account and profile");
    const root = await screen.findByText("I start with WORK-501.");
    expect(sentence.compareDocumentPosition(root) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

    await user.click(screen.getByRole("button", { name: "Reply" }));
    const box = screen.getByRole("textbox", { name: "Write a reply…" });
    await user.type(box, reply.content);
    api.listReleaseComments.mockResolvedValue([pieterComment, reply]);
    await user.click(within(box.closest("form") as HTMLElement).getByRole("button", { name: "Reply" }));

    await waitFor(() =>
      expect(api.createReleaseComment).toHaveBeenCalledWith("r-110", { content: reply.content, parent_id: "c-1" }),
    );
    const thread = root.closest("li") as HTMLElement;
    expect(await within(thread).findByText(reply.content)).toBeTruthy();
    expect(screen.queryByRole("textbox", { name: "Write a reply…" })).toBeNull();
  });

  it("offers Reply on a reply and sends it with that reply as parent", async () => {
    const user = userEvent.setup();
    const reply = comment({ id: "c-2", parent_id: "c-1", content: "Review is open.", created_at: "2026-02-01T12:00:00Z" });
    const answer = comment({ id: "c-3", parent_id: "c-2", content: "Review passed.", created_at: "2026-02-01T13:00:00Z" });
    seed(planned, { comments: [pieterComment, reply] });
    api.createReleaseComment.mockResolvedValue(answer);
    mount(<ReleaseDetailRoute releaseId="r-110" />);

    const nested = (await screen.findByText(reply.content)).closest("article") as HTMLElement;
    await user.click(within(nested).getByRole("button", { name: "Reply" }));
    const box = screen.getByRole("textbox", { name: "Write a reply…" });
    await user.type(box, answer.content);
    api.listReleaseComments.mockResolvedValue([pieterComment, reply, answer]);
    await user.click(within(box.closest("form") as HTMLElement).getByRole("button", { name: "Reply" }));

    await waitFor(() =>
      expect(api.createReleaseComment).toHaveBeenCalledWith("r-110", { content: answer.content, parent_id: "c-2" }),
    );
    const thread = (await screen.findByText(pieterComment.content)).closest("li") as HTMLElement;
    expect(await within(thread).findByText(answer.content)).toBeTruthy();
    expect(screen.queryByRole("textbox", { name: "Write a reply…" })).toBeNull();
  });

  it("shows a failed read of the discussion as an error with Retry, never as no comments", async () => {
    const user = userEvent.setup();
    seed(planned);
    api.listReleaseComments.mockRejectedValue(new Error("network down"));
    mount(<ReleaseDetailRoute releaseId="r-110" />);

    expect(await screen.findByRole("alert")).toHaveTextContent("Couldn't load the comments.");
    expect(screen.queryByText("No comments yet.")).toBeNull();

    api.listReleaseComments.mockResolvedValue([pieterComment]);
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByText(pieterComment.content)).toBeTruthy();
    expect(screen.queryByText("Couldn't load the comments.")).toBeNull();
  });

  it("keeps the text when sending fails and sends it on Retry", async () => {
    const user = userEvent.setup();
    seed(planned);
    const text = "Start WORK-502 once WORK-501's pull request is open.";
    const sent = comment({ id: "c-9", content: text, created_at: "2026-02-02T09:00:00Z" });
    api.createReleaseComment.mockRejectedValueOnce(new Error("network down")).mockResolvedValueOnce(sent);
    mount(<ReleaseDetailRoute releaseId="r-110" />);

    const box = await screen.findByRole("textbox", { name: "Write a comment…" });
    await user.type(box, text);
    await user.click(screen.getByRole("button", { name: "Comment" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Couldn't send your comment.");
    expect(box).toHaveValue(text);

    api.listReleaseComments.mockResolvedValue([sent]);
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByText(text, { selector: "div" })).toBeTruthy();
    await waitFor(() => expect(box).toHaveValue(""));
    expect(api.createReleaseComment).toHaveBeenNthCalledWith(2, "r-110", { content: text });
  });
});

describe("entry points to the release page", () => {
  it("opens the release page from its Releases row", async () => {
    const user = userEvent.setup();
    seed(planned);
    mount(
      <ReleaseRow
        indent=""
        row={{ release: planned, state: "planned", setByHand: false, done: 1, total: 2, worklanes: [worklanes[1]!] }}
      />,
    );
    const row = screen.getByRole("link", { name: /Account and profile/ });
    expect(row.getAttribute("href")).toBe("/acme/releases/r-110");
    await user.click(row);
    expect(push).toHaveBeenCalledWith("/acme/releases/r-110");
  });

  it("opens the release page from its title on the Timeline", async () => {
    const user = userEvent.setup();
    seed(planned);
    const line = toLine([shipped, next, planned], "1.0.0");
    mount(
      <ReleaseColumn
        repository={repository}
        release={planned}
        state={lineStates(line).get("r-110") ?? "planned"}
        line={line}
      />,
    );
    await user.click(screen.getByRole("link", { name: "Account and profile" }));
    expect(push).toHaveBeenCalledWith("/acme/releases/r-110");
  });
});
