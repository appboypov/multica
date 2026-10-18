// The assignee picker offers a client only on issues of that client's
// organisation, in a Clients group of its own. Uses the real PropertyPicker.
import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import enIssues from "../../../locales/en/issues.json";
import { AssigneePicker } from "./assignee-picker";

const MEMBERS = [
  { user_id: "user-ada", name: "Ada Lovelace", role: "owner", organisation_ids: [] },
  { user_id: "user-nasir", name: "Nasir", role: "client", organisation_ids: ["org-a", "org-b"] },
  { user_id: "user-rizgar", name: "Rizgar", role: "client", organisation_ids: ["org-b"] },
];

const PROJECTS = [
  { id: "project-a", organisation_id: "org-a" },
  { id: "project-b", organisation_id: "org-b" },
  { id: "project-c", organisation_id: "org-c" },
  { id: "project-internal", organisation_id: null },
];

vi.mock("@tanstack/react-query", () => ({
  useQuery: ({ queryKey }: { queryKey: string[] }) => {
    if (queryKey[0] === "members") return { data: MEMBERS };
    if (queryKey[0] === "projects") return { data: PROJECTS };
    return { data: [] };
  },
}));

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace-1" }));
vi.mock("@multica/core/auth", () => ({ useAuthStore: () => ({ id: "user-ada" }) }));
vi.mock("@multica/core/agents", () => ({ isAgentRuntimeBound: () => true }));
vi.mock("@multica/core/permissions", () => ({
  canAssignAgentToIssue: () => ({ allowed: true }),
}));
vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({ getActorName: () => "Ada Lovelace" }),
}));
vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({ queryKey: ["members"] }),
  agentListOptions: () => ({ queryKey: ["agents"] }),
  squadListOptions: () => ({ queryKey: ["squads"] }),
  assigneeFrequencyOptions: () => ({ queryKey: ["frequency"] }),
}));
vi.mock("@multica/core/projects/queries", () => ({
  projectListOptions: () => ({ queryKey: ["projects"] }),
}));
vi.mock("../../../common/actor-avatar", () => ({
  ActorAvatar: () => <span data-testid="actor-avatar" />,
}));

function renderPicker(projectIds: (string | null)[], onUpdate = vi.fn()) {
  render(
    <I18nProvider locale="en" resources={{ en: { issues: enIssues } }}>
      <AssigneePicker
        assigneeType={null}
        assigneeId={null}
        projectIds={projectIds}
        onUpdate={onUpdate}
        open
        onOpenChange={() => {}}
      />
    </I18nProvider>,
  );
  return onUpdate;
}

/** The names listed under a picker section label, or null without the section. */
function groupNames(label: string): string[] | null {
  const heading = screen.queryByText(label);
  if (!heading) return null;
  const section = heading.parentElement as HTMLElement;
  return within(section)
    .getAllByRole("button")
    .map((button) => button.textContent ?? "");
}

describe("AssigneePicker clients", () => {
  it("lists the clients of the issue's organisation in their own group", async () => {
    renderPicker(["project-b"]);
    await screen.findByText("Clients");
    expect(groupNames("Clients")).toEqual(["Nasir", "Rizgar"]);
    expect(groupNames("Members")).toEqual(["Ada Lovelace"]);
  });

  it("leaves out clients of other organisations", async () => {
    renderPicker(["project-a"]);
    await screen.findByText("Clients");
    expect(groupNames("Clients")).toEqual(["Nasir"]);
  });

  it.each([
    { name: "an organisation without clients", projectIds: ["project-c"] },
    { name: "a project without an organisation", projectIds: ["project-internal"] },
    { name: "an issue without a project", projectIds: [null] },
    { name: "a batch across two organisations", projectIds: ["project-a", "project-b"] },
  ])("offers no client for $name", async ({ projectIds }) => {
    renderPicker(projectIds);
    await screen.findByText("Members");
    expect(groupNames("Clients")).toBeNull();
    expect(screen.queryByText("Nasir")).toBeNull();
  });

  it("assigns a client as a member", async () => {
    const user = userEvent.setup();
    const onUpdate = renderPicker(["project-a"]);
    await user.click(await screen.findByText("Nasir"));
    expect(onUpdate).toHaveBeenCalledWith({ assignee_type: "member", assignee_id: "user-nasir" });
  });
});
