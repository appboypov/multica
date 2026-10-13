import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useWorkspaceId } from "../hooks";
import type { CreateOrganisationRequest, UpdateOrganisationRequest } from "../types";
import { organisationKeys } from "./queries";

export function useCreateOrganisation() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (data: CreateOrganisationRequest) => api.createOrganisation(data),
    onSettled: () => qc.invalidateQueries({ queryKey: organisationKeys.all(wsId) }),
  });
}

export function useUpdateOrganisationKey() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ id, ...data }: { id: string } & UpdateOrganisationRequest) =>
      api.updateOrganisation(id, data),
    onSettled: () => qc.invalidateQueries({ queryKey: organisationKeys.all(wsId) }),
  });
}
