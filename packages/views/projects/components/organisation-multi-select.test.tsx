import { describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithI18n } from "../../test/i18n";
import { OrganisationMultiSelect } from "./organisation-multi-select";

const organisations = vi.hoisted(() => ({
  list: [] as Array<{ id: string; name: string }>,
}));

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/organisations", () => ({
  useOrganisations: () => ({ data: organisations.list }),
}));

function renderSelect(selectedIds: string[]) {
  const onChange = vi.fn();
  renderWithI18n(
    <>
      <label htmlFor="orgs">Organisations</label>
      <OrganisationMultiSelect id="orgs" selectedIds={selectedIds} onChange={onChange} />
    </>,
  );
  return onChange;
}

describe("OrganisationMultiSelect", () => {
  it("shows each chosen organisation as a removable chip", async () => {
    organisations.list = [
      { id: "org-a", name: "Northwind" },
      { id: "org-b", name: "Globex" },
    ];
    const user = userEvent.setup();
    const onChange = renderSelect(["org-a", "org-b"]);

    await user.click(screen.getByRole("button", { name: "Remove Northwind" }));
    expect(onChange).toHaveBeenCalledWith(["org-b"]);
  });

  it("adds an organisation picked from the searchable menu", async () => {
    organisations.list = [
      { id: "org-a", name: "Northwind" },
      { id: "org-b", name: "Globex" },
    ];
    const user = userEvent.setup();
    const onChange = renderSelect(["org-a"]);

    await user.click(screen.getByRole("button", { name: "Organisations" }));
    await user.type(await screen.findByRole("textbox"), "glob");
    expect(screen.queryByRole("button", { name: "Northwind" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "Globex" }));
    expect(onChange).toHaveBeenCalledWith(["org-a", "org-b"]);
  });

  it("says so when the workspace has no organisations", async () => {
    organisations.list = [];
    const user = userEvent.setup();
    renderSelect([]);

    await user.click(screen.getByRole("button", { name: "Organisations" }));
    expect(await screen.findByText("No organisations yet")).toBeInTheDocument();
  });
});
