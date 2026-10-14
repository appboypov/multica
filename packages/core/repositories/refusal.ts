import { ApiError, parseWithFallback } from "../api";
import { RepositoryRefusalSchema, UNKNOWN_REPOSITORY_REFUSAL } from "../api/schemas";
import type { RepositoryRefusal } from "../types";

/** The structured refusal of a failed repository write, or null for any other error. */
export function repositoryRefusalOf(error: unknown): RepositoryRefusal | null {
  if (!(error instanceof ApiError) || error.status !== 409) return null;
  return parseWithFallback(error.body, RepositoryRefusalSchema, UNKNOWN_REPOSITORY_REFUSAL, {
    endpoint: "repository refusal",
  });
}
