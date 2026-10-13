import { queryOptions, useQuery } from "@tanstack/react-query";
import { api } from "../api";

export const organisationKeys = {
  all: (wsId: string) => ["organisations", wsId] as const,
  list: (wsId: string) => [...organisationKeys.all(wsId), "list"] as const,
  detail: (wsId: string, id: string) =>
    [...organisationKeys.all(wsId), "detail", id] as const,
};

export function organisationListOptions(wsId: string) {
  return queryOptions({
    queryKey: organisationKeys.list(wsId),
    queryFn: () => api.listOrganisations(),
    select: (data) => data.organisations,
  });
}

export function organisationDetailOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: organisationKeys.detail(wsId, id),
    queryFn: () => api.getOrganisation(id),
  });
}

export function useOrganisations(wsId: string) {
  return useQuery(organisationListOptions(wsId));
}

export function useOrganisation(wsId: string, id: string) {
  return useQuery(organisationDetailOptions(wsId, id));
}
