import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import enProjects from "../../locales/en/projects.json";
import { OrganisationPicker } from "./organisation-picker";

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

vi.mock("@multica/core/organisations", () => ({
  useOrganisations: () => ({
    data: [
      { id: "org-das", workspace_id: "workspace-1", name: "De App Specialist", key: "das", created_at: "", updated_at: "" },
      { id: "org-rebl", workspace_id: "workspace-1", name: "Rebl Records", key: "rebl", created_at: "", updated_at: "" },
    ],
  }),
}));

function renderPicker(organisationId: string | null, onChange = vi.fn()) {
  render(
    <I18nProvider locale="en" resources={{ en: { projects: enProjects } }}>
      <OrganisationPicker organisationId={organisationId} onChange={onChange} />
    </I18nProvider>,
  );
  return onChange;
}

describe("OrganisationPicker", () => {
  it("shows No organisation when the project has none, and picks one by name", async () => {
    const user = userEvent.setup();
    const onChange = renderPicker(null);

    await user.click(screen.getByRole("button", { name: "No organisation" }));
    const items = await screen.findAllByRole("menuitem");
    expect(items.map((item) => item.textContent)).toEqual(["De App Specialist", "Rebl Records"]);

    await user.click(screen.getByRole("menuitem", { name: "Rebl Records" }));
    expect(onChange).toHaveBeenCalledWith("org-rebl");
  });

  it("shows the project's organisation and ignores picking it again", async () => {
    const user = userEvent.setup();
    const onChange = renderPicker("org-rebl");

    await user.click(screen.getByRole("button", { name: "Rebl Records" }));
    await user.click(await screen.findByRole("menuitem", { name: "Rebl Records" }));

    expect(onChange).not.toHaveBeenCalled();
  });

  it("shows No organisation for an organisation the workspace does not list", () => {
    renderPicker("org-gone");

    expect(screen.getByRole("button", { name: "No organisation" })).toBeInTheDocument();
  });
});
