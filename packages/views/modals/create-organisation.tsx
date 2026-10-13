"use client";

import { useId, useState } from "react";
import { toast } from "sonner";
import { clientErrorMessage } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCreateOrganisation, useOrganisations } from "@multica/core/organisations";
import { isImeComposing } from "@multica/core/utils";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { useT } from "../i18n";
import { organisationKeyProblem, organisationNameProblem } from "../organisations/validation";

/**
 * New organisation: a name and a key. It checks the loaded list first, so a
 * taken name (ignoring case), a taken key or a key that is no folder name
 * shows at once and Create stays disabled; an empty field only disables
 * Create. Create awaits the server and closes on success; a refusal keeps the
 * input and shows the server's message.
 */
export function CreateOrganisationModal({ onClose }: { onClose: () => void }) {
  const { t } = useT("modals");
  const { t: tOrganisations } = useT("organisations");
  const wsId = useWorkspaceId();
  const { data: organisations = [] } = useOrganisations(wsId);
  const createOrganisation = useCreateOrganisation();
  const nameId = useId();
  const keyId = useId();
  const nameMessageId = useId();
  const keyMessageId = useId();

  const [name, setName] = useState("");
  const [key, setKey] = useState("");
  const [serverError, setServerError] = useState<string | null>(null);

  const nameProblem = organisationNameProblem(name, organisations);
  const keyProblem = organisationKeyProblem(key, organisations);
  // An empty field disables Create without a message; the others explain why.
  const nameMessage =
    nameProblem && nameProblem !== "required"
      ? tOrganisations(($) => $.fields.name_error[nameProblem])
      : null;
  const keyMessage =
    keyProblem && keyProblem !== "required"
      ? tOrganisations(($) => $.fields.key_error[keyProblem])
      : null;
  const canSubmit = !nameProblem && !keyProblem && !createOrganisation.isPending;

  const handleSubmit = async () => {
    if (!canSubmit) return;
    setServerError(null);
    try {
      await createOrganisation.mutateAsync({ name: name.trim(), key });
      onClose();
      toast.success(t(($) => $.create_organisation.toast_created));
    } catch (err) {
      setServerError(clientErrorMessage(err) ?? t(($) => $.create_organisation.toast_failed));
    }
  };

  const submitOnEnter = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (isImeComposing(e)) return;
    if (e.key === "Enter") void handleSubmit();
  };

  return (
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t(($) => $.create_organisation.title)}</DialogTitle>
          <DialogDescription>{t(($) => $.create_organisation.description)}</DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-1">
            <Label htmlFor={nameId} className="text-caption text-muted-foreground">
              {tOrganisations(($) => $.fields.name_label)}
            </Label>
            <Input
              id={nameId}
              autoFocus
              value={name}
              onChange={(e) => {
                setName(e.target.value);
                setServerError(null);
              }}
              onKeyDown={submitOnEnter}
              placeholder={tOrganisations(($) => $.fields.name_placeholder)}
              aria-invalid={nameMessage ? true : undefined}
              aria-describedby={nameMessage ? nameMessageId : undefined}
            />
            {nameMessage ? (
              <p id={nameMessageId} className="text-caption text-destructive">
                {nameMessage}
              </p>
            ) : null}
          </div>

          <div className="space-y-1">
            <Label htmlFor={keyId} className="text-caption text-muted-foreground">
              {tOrganisations(($) => $.fields.key_label)}
            </Label>
            <Input
              id={keyId}
              value={key}
              onChange={(e) => {
                setKey(e.target.value);
                setServerError(null);
              }}
              onKeyDown={submitOnEnter}
              placeholder={tOrganisations(($) => $.fields.key_placeholder)}
              className="font-mono"
              aria-invalid={keyMessage ? true : undefined}
              aria-describedby={keyMessageId}
            />
            <p id={keyMessageId} className={keyMessage ? "text-caption text-destructive" : "text-caption text-muted-foreground"}>
              {keyMessage ?? tOrganisations(($) => $.fields.key_hint)}
            </p>
          </div>

          {serverError ? (
            <p role="alert" className="text-caption text-destructive">
              {serverError}
            </p>
          ) : null}
        </div>

        <div className="flex items-center justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>
            {t(($) => $.create_organisation.cancel)}
          </Button>
          <Button onClick={() => void handleSubmit()} disabled={!canSubmit}>
            {createOrganisation.isPending
              ? t(($) => $.create_organisation.submitting)
              : t(($) => $.create_organisation.submit)}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
