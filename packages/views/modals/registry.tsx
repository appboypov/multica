"use client";

import type { ReactNode } from "react";
import { useModalStore } from "@multica/core/modals";
import { CreateIssueDialog } from "./create-issue-dialog";
import { CreateProjectModal } from "./create-project";
import { CreateOrganisationModal } from "./create-organisation";
import { AddRepositoryDialog } from "../repositories/components/add-repository-dialog";
import { EditRepositoryDialog } from "../repositories/components/edit-repository-dialog";
import { NewWorklaneDialog } from "../worklanes/components/new-worklane-dialog";
import { RenameWorklaneDialog } from "../worklanes/components/rename-worklane-dialog";
import { RemoveWorklaneDialog } from "../worklanes/components/remove-worklane-dialog";
import { ReleaseDialog } from "../releases/components/release-dialog";
import { RemoveReleaseDialog } from "../releases/components/remove-release-dialog";
import { CreateSquadModal } from "./create-squad";
import { FeedbackModal } from "./feedback";
import { SetParentIssueModal } from "./set-parent-issue";
import { MarkDuplicateIssueModal } from "./mark-duplicate-issue";
import { AddChildIssueModal } from "./add-child-issue";
import { DeleteIssueConfirmModal } from "./delete-issue-confirm";
import { RunConfirmModal } from "./run-confirm";
import { IssueLimitUpgradeDialog } from "./issue-limit-upgrade-dialog";

export function ModalRegistry() {
  const modal = useModalStore((s) => s.modal);
  const data = useModalStore((s) => s.data);
  const close = useModalStore((s) => s.close);

  let activeModal: ReactNode = null;
  switch (modal) {
    // Both modal types open the same shell so the in-modal mode switch is
    // instant — only the inner panel swaps, the Dialog Root stays mounted.
    case "create-issue":
      activeModal = (
        <CreateIssueDialog
          onClose={close}
          initialMode="manual"
          data={data}
        />
      );
      break;
    case "quick-create-issue":
      activeModal = (
        <CreateIssueDialog
          onClose={close}
          initialMode="agent"
          data={data}
        />
      );
      break;
    case "create-project":
      activeModal = <CreateProjectModal onClose={close} data={data} />;
      break;
    case "create-organisation":
      activeModal = <CreateOrganisationModal onClose={close} />;
      break;
    case "add-repository":
      activeModal = <AddRepositoryDialog onClose={close} data={data} />;
      break;
    case "edit-repository":
      activeModal = <EditRepositoryDialog onClose={close} data={data} />;
      break;
    case "new-worklane":
      activeModal = <NewWorklaneDialog onClose={close} data={data} />;
      break;
    case "rename-worklane":
      activeModal = <RenameWorklaneDialog onClose={close} data={data} />;
      break;
    case "remove-worklane":
      activeModal = <RemoveWorklaneDialog onClose={close} data={data} />;
      break;
    case "plan-release":
    case "edit-release":
      activeModal = <ReleaseDialog onClose={close} data={data} />;
      break;
    case "remove-release":
      activeModal = <RemoveReleaseDialog onClose={close} data={data} />;
      break;
    case "create-squad":
      activeModal = <CreateSquadModal onClose={close} />;
      break;
    case "feedback":
      activeModal = <FeedbackModal onClose={close} data={data} />;
      break;
    case "issue-set-parent":
      activeModal = <SetParentIssueModal onClose={close} data={data} />;
      break;
    case "issue-mark-duplicate":
      activeModal = <MarkDuplicateIssueModal onClose={close} data={data} />;
      break;
    case "issue-add-child":
      activeModal = <AddChildIssueModal onClose={close} data={data} />;
      break;
    case "issue-delete-confirm":
      activeModal = <DeleteIssueConfirmModal onClose={close} data={data} />;
      break;
    case "issue-run-confirm":
      activeModal = <RunConfirmModal onClose={close} data={data} />;
      break;
  }

  return (
    <>
      {activeModal}
      <IssueLimitUpgradeDialog />
    </>
  );
}
