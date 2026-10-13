import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Organisation } from "@multica/core/types";
import { renderWithI18n } from "../test/i18n";

const mocks = vi.hoisted(() => ({
  createOrganisation: vi.fn(),
  isPending: false,
  toastSuccess: vi.fn(),
}));

const REBL_RECORDS: Organisation = {
  id: "org-rebl",
  workspace_id: "ws-1",
  name: "Rebl Records",
  key: "rebl",
  created_at: "",
  updated_at: "",
};
const DE_APP_SPECIALIST: Organisation = {
  id: "org-das",
  workspace_id: "ws-1",
  name: "De App Specialist",
  key: "das",
  created_at: "",
  updated_at: "",
};

vi.mock("@multica/core/organisations", () => ({
  useOrganisations: () => ({ data: [REBL_RECORDS, DE_APP_SPECIALIST] }),
  useCreateOrganisation: () => ({
    mutateAsync: mocks.createOrganisation,
    isPending: mocks.isPending,
  }),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/api", () => ({
  clientErrorMessage: (err: unknown) => (err instanceof Error ? err.message : undefined),
}));

vi.mock("@multica/core/utils", () => ({
  isImeComposing: () => false,
}));

vi.mock("@multica/ui/components/ui/dialog", () => ({
  Dialog: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  DialogContent: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  DialogHeader: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: ReactNode }) => <h2>{children}</h2>,
  DialogDescription: ({ children }: { children: ReactNode }) => <p>{children}</p>,
}));

vi.mock("sonner", () => ({
  toast: { success: mocks.toastSuccess, error: vi.fn() },
}));

import { CreateOrganisationModal } from "./create-organisation";

function renderDialog(onClose = vi.fn()) {
  renderWithI18n(<CreateOrganisationModal onClose={onClose} />);
  return onClose;
}

const nameField = () => screen.getByLabelText("Name");
const keyField = () => screen.getByLabelText("Key");
const createButton = () => screen.getByRole("button", { name: "Create" });

beforeEach(() => {
  mocks.createOrganisation.mockReset().mockResolvedValue({ ...REBL_RECORDS, id: "org-flx" });
  mocks.isPending = false;
  mocks.toastSuccess.mockReset();
});

describe("CreateOrganisationModal", () => {
  it("creates an organisation with its name and key, then closes", async () => {
    const user = userEvent.setup();
    const onClose = renderDialog();

    await user.type(nameField(), "Flextender");
    await user.type(keyField(), "flx");
    await user.click(createButton());

    expect(mocks.createOrganisation).toHaveBeenCalledWith({ name: "Flextender", key: "flx" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("refuses a name that is taken, ignoring case", async () => {
    const user = userEvent.setup();
    renderDialog();

    await user.type(nameField(), "rebl records");
    await user.type(keyField(), "rr");

    expect(screen.getByText("Another organisation has this name")).toBeInTheDocument();
    expect(createButton()).toBeDisabled();
    await user.type(keyField(), "{Enter}");
    expect(mocks.createOrganisation).not.toHaveBeenCalled();
  });

  it("refuses a key that is taken", async () => {
    const user = userEvent.setup();
    renderDialog();

    await user.type(nameField(), "Das Studio");
    await user.type(keyField(), "das");

    expect(screen.getByText("Another organisation has this key")).toBeInTheDocument();
    expect(createButton()).toBeDisabled();
  });

  it("refuses a key that is no folder name", async () => {
    const user = userEvent.setup();
    renderDialog();

    await user.type(nameField(), "Flextender");
    await user.type(keyField(), "Rebl Records");

    expect(
      screen.getByText(
        "A key is lowercase letters, digits and dashes, starting with a letter or digit",
      ),
    ).toBeInTheDocument();
    expect(createButton()).toBeDisabled();
  });

  it("cannot create without a name or a key", async () => {
    const user = userEvent.setup();
    renderDialog();

    expect(createButton()).toBeDisabled();
    await user.type(nameField(), "Flextender");
    expect(createButton()).toBeDisabled();
    await user.clear(nameField());
    await user.type(keyField(), "flx");
    expect(createButton()).toBeDisabled();
  });

  it("keeps the input and shows the server's refusal", async () => {
    mocks.createOrganisation.mockRejectedValue(new Error("organisation key is taken"));
    const user = userEvent.setup();
    const onClose = renderDialog();

    await user.type(nameField(), "Flextender");
    await user.type(keyField(), "flx");
    await user.click(createButton());

    expect(await screen.findByRole("alert")).toHaveTextContent("organisation key is taken");
    expect(onClose).not.toHaveBeenCalled();
    expect(nameField()).toHaveValue("Flextender");
    expect(keyField()).toHaveValue("flx");
  });
});
