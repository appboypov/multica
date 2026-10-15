/**
 * Centralized URL path builder. All navigation in shared packages (packages/views)
 * MUST go through this module — no hardcoded string paths.
 *
 * Two kinds of paths:
 *  - workspace-scoped: paths.workspace(slug).xxx() — carry workspace in URL
 *  - global: paths.login(), paths.newWorkspace(), paths.invite(id) — pre-workspace routes
 *
 * Why pure functions + builder pattern:
 *  - Changing a route shape (e.g. adding workspace slug prefix) becomes a single-file edit
 *  - IDs are always URL-encoded here so callers can't forget
 *  - Zero runtime deps means this module is safe in Node (tests) and browsers
 */

const encode = (id: string) => encodeURIComponent(id);

/**
 * `?focus=` token that scrolls the agent's Instructions tab to its
 * conversation-starters editor and flashes it. Lives here because it is URL
 * vocabulary: `agentConversationStarters()` writes it and the tab reads it,
 * and a shared constant is what stops the two from drifting apart.
 */
export const AGENT_FOCUS_CONVERSATION_STARTERS = "conversation_starters";

/**
 * `?todo=` selection of the to-do Inbox: the selected to-do's own id. URL
 * vocabulary shared by the Inbox page, the tab bar and notification clicks.
 */
export const INBOX_TODO_PARAM = "todo";

/**
 * `?issue=` selection of Updates: the issue id a notice groups under, else the
 * notice's own id.
 */
export const UPDATES_ITEM_PARAM = "issue";

function workspaceScoped(slug: string) {
  const ws = `/${encode(slug)}`;
  return {
    root: () => `${ws}/issues`,
    usage: () => `${ws}/usage`,
    issues: () => `${ws}/issues`,
    issueDetail: (id: string) => `${ws}/issues/${encode(id)}`,
    projects: () => `${ws}/projects`,
    projectDetail: (id: string) => `${ws}/projects/${encode(id)}`,
    organisations: () => `${ws}/organisations`,
    organisationDetail: (id: string) => `${ws}/organisations/${encode(id)}`,
    worklanes: () => `${ws}/worklanes`,
    releases: () => `${ws}/releases`,
    releaseDetail: (id: string) => `${ws}/releases/${encode(id)}`,
    worklaneDetail: (id: string) => `${ws}/worklanes/${encode(id)}`,
    autopilots: () => `${ws}/autopilots`,
    autopilotDetail: (id: string) => `${ws}/autopilots/${encode(id)}`,
    agents: () => `${ws}/agents`,
    newAgent: () => `${ws}/agents/new`,
    // The two creation methods behind the chooser. Each is a real route so a
    // half-filled form survives a refresh and can be linked to directly.
    newAgentManual: () => `${ws}/agents/new/manual`,
    newAgentAi: () => `${ws}/agents/new/ai`,
    // One creation conversation. It is a durable object, not a step of the
    // route above: it survives leaving the studio and is resumed later, so it
    // owns an address instead of being a query param on the "start one" screen.
    newAgentAiSession: (sessionId: string) =>
      `${ws}/agents/new/ai/${encode(sessionId)}`,
    agentDetail: (id: string) => `${ws}/agents/${encode(id)}`,
    // Deep link behind "customize" in a chat's empty state: the agent's
    // Instructions tab, scrolled to the conversation starters that produced
    // the buttons the viewer just looked at.
    agentConversationStarters: (id: string) =>
      `${ws}/agents/${encode(id)}?view=instructions&focus=${AGENT_FOCUS_CONVERSATION_STARTERS}`,
    memberDetail: (id: string) => `${ws}/members/${encode(id)}`,
    squads: () => `${ws}/squads`,
    squadDetail: (id: string) => `${ws}/squads/${encode(id)}`,
    inbox: () => `${ws}/inbox`,
    inboxTodo: (todoId: string) =>
      `${ws}/inbox?${INBOX_TODO_PARAM}=${encode(todoId)}`,
    updates: () => `${ws}/updates`,
    updatesItem: (key: string) =>
      `${ws}/updates?${UPDATES_ITEM_PARAM}=${encode(key)}`,
    questions: () => `${ws}/questions`,
    /** `questionId` opens the session at that question. */
    answerAll: (options?: { questionId?: string }) =>
      `${ws}/questions/answer${options?.questionId ? `?question=${encode(options.questionId)}` : ""}`,
    chat: () => `${ws}/chat`,
    chatWithAgent: (agentId: string) =>
      `${ws}/chat?agent=${encode(agentId)}`,
    /** `questionId` scrolls the chat's message list to that question's card. */
    chatSession: (sessionId: string, options?: { questionId?: string }) =>
      `${ws}/chat?session=${encode(sessionId)}${options?.questionId ? `&question=${encode(options.questionId)}` : ""}`,
    myIssues: () => `${ws}/my-issues`,
    runtimes: () => `${ws}/runtimes`,
    runtimeDetail: (id: string) => `${ws}/runtimes/${encode(id)}`,
    runtimeSettings: (machineId: string, runtimeId: string) =>
      `${ws}/runtimes/${encode(machineId)}/runtime/${encode(runtimeId)}`,
    skills: () => `${ws}/skills`,
    skillDetail: (id: string) => `${ws}/skills/${encode(id)}`,
    settings: () => `${ws}/settings`,
    attachmentPreview: (id: string) => `${ws}/attachments/${encode(id)}/preview`,
  };
}

export const paths = {
  workspace: workspaceScoped,

  // Global (pre-workspace) routes
  login: () => "/login",
  newWorkspace: () => "/workspaces/new",
  invite: (id: string) => `/invite/${encode(id)}`,
  invitations: () => "/invitations",
  onboarding: () => "/onboarding",
  authCallback: () => "/auth/callback",
  root: () => "/",
};

export type WorkspacePaths = ReturnType<typeof workspaceScoped>;

// Prefixes — not slug names — because we match against full URL paths.
// A path is global if it equals or begins with any of these.
// Note: `/workspaces/` (trailing slash) is the prefix — `workspaces` is reserved,
// so any path starting with `/workspaces/...` is system-owned, not user-owned.
const GLOBAL_PREFIXES = ["/login", "/workspaces/", "/invite/", "/invitations", "/onboarding", "/auth/", "/logout", "/signup"];

export function isGlobalPath(path: string): boolean {
  return GLOBAL_PREFIXES.some((p) => path === p || path.startsWith(p));
}
