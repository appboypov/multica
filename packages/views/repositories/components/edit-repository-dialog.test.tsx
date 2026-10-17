import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import type { Repository } from "@multica/core/types";
import enCommon from "../../locales/en/common.json";
import enRepositories from "../../locales/en/repositories.json";

const mockMutateAsync = vi.hoisted(() => vi.fn());
const repositoryRef = vi.hoisted(() => ({ current: null as Repository | null }));

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: { kind: string }) =>
    options.kind === "repositories"
      ? { data: [repositoryRef.current], isPending: false }
      : { data: [], isPending: false },
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace-1" }));
vi.mock("@multica/core/api", () => ({ clientErrorMessage: () => null }));
vi.mock("@multica/core/releases", () => ({
  releaseListOptions: () => ({ kind: "releases" }),
  selectLine: () => [],
}));
vi.mock("@multica/core/repositories", () => ({
  repositoryListOptions: () => ({ kind: "repositories" }),
  repositoryRefusalOf: () => null,
  useUpdateRepository: () => ({ mutateAsync: mockMutateAsync, isPending: false }),
}));
vi.mock("./checkout-field", () => ({ CheckoutField: () => null }));
vi.mock("./releases-field", () => ({ ReleasesField: () => null }));
vi.mock("./install-field", () => ({ InstallField: () => null }));
vi.mock("sonner", () => ({ toast: { success: vi.fn() } }));

import { EditRepositoryDialog } from "./edit-repository-dialog";

const TEST_RESOURCES = { en: { common: enCommon, repositories: enRepositories } };

function Wrapper({ children }: { children: ReactNode }) {
  return (
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      {children}
    </I18nProvider>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockMutateAsync.mockResolvedValue(undefined);
  repositoryRef.current = {
    id: "repo-1",
    workspace_id: "workspace-1",
    project_id: "project-1",
    created_by: null,
    created_at: "",
    updated_at: "",
    github_repo_id: 1,
    full_name: "appboypov/multica",
    private: false,
    default_branch: "main",
    checkout_daemon_id: "daemon-1",
    checkout_path: "/Users/brian/Repos/Forks/multica",
    version_policy: "none",
    install_command: "pnpm install",
    last_tag: null,
    work_in_checkout: false,
  };
});

describe("EditRepositoryDialog", () => {
  it("saves Work in the project folder once it is turned on", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<EditRepositoryDialog onClose={onClose} data={{ repositoryId: "repo-1" }} />, { wrapper: Wrapper });

    const toggle = screen.getByRole("switch", { name: enRepositories.edit.work_in_checkout });
    const save = screen.getByRole("button", { name: enRepositories.edit.submit });
    expect(toggle).not.toBeChecked();
    expect(save).toBeDisabled();

    await user.click(toggle);
    expect(save).toBeEnabled();
    await user.click(save);

    expect(mockMutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({ id: "repo-1", work_in_checkout: true }),
    );
    expect(onClose).toHaveBeenCalled();
  });

  it("shows the saved setting and saves it turned off", async () => {
    repositoryRef.current = { ...repositoryRef.current!, work_in_checkout: true };
    const user = userEvent.setup();
    render(<EditRepositoryDialog onClose={vi.fn()} data={{ repositoryId: "repo-1" }} />, { wrapper: Wrapper });

    const toggle = screen.getByRole("switch", { name: enRepositories.edit.work_in_checkout });
    expect(toggle).toBeChecked();
    await user.click(toggle);
    await user.click(screen.getByRole("button", { name: enRepositories.edit.submit }));

    expect(mockMutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({ id: "repo-1", work_in_checkout: false }),
    );
  });

  it("follows a change saved elsewhere while it is open", () => {
    const { rerender } = render(<EditRepositoryDialog onClose={vi.fn()} data={{ repositoryId: "repo-1" }} />, {
      wrapper: Wrapper,
    });
    repositoryRef.current = { ...repositoryRef.current!, work_in_checkout: true };
    rerender(<EditRepositoryDialog onClose={vi.fn()} data={{ repositoryId: "repo-1" }} />);

    expect(screen.getByRole("switch", { name: enRepositories.edit.work_in_checkout })).toBeChecked();
    expect(screen.getByRole("button", { name: enRepositories.edit.submit })).toBeDisabled();
  });
});
