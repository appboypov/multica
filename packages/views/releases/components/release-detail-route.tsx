"use client";

import { useQuery } from "@tanstack/react-query";
import { ApiError } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { releaseDetailOptions } from "@multica/core/releases";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Spinner } from "@multica/ui/components/ui/spinner";
import { AppLink } from "../../navigation";
import { useT } from "../../i18n";
import { ReleaseDetail } from "./release-detail";

/**
 * The release page as web and desktop mount it: loads the release in the
 * current workspace and shows it, a not-found state for a missing release,
 * or a failure with retry.
 */
export function ReleaseDetailRoute({ releaseId }: { releaseId: string }) {
  const wsId = useWorkspaceId();
  const { t } = useT("releases");
  const paths = useWorkspacePaths();
  const { data: release, isLoading, error, refetch, isFetching } = useQuery(
    releaseDetailOptions(wsId, releaseId),
  );

  if (isLoading) {
    return (
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-4 px-6 py-8" aria-busy="true">
        <Skeleton className="h-4 w-40" />
        <Skeleton className="h-8 w-80" />
        <Skeleton className="h-24 w-full" />
      </div>
    );
  }

  // An answer without a release id is the parsed fallback of a release the
  // server no longer knows.
  const notFound = (error instanceof ApiError && error.status === 404) || (!error && !release?.id);
  if (notFound) {
    return (
      <div className="mx-auto flex w-full max-w-3xl flex-col items-start gap-3 px-6 py-16">
        <p className="text-body">{t(($) => $.page.not_found)}</p>
        <AppLink href={paths.releases()} className="text-label text-primary hover:underline">
          {t(($) => $.page.back)}
        </AppLink>
      </div>
    );
  }

  if (error || !release) {
    return (
      <div role="alert" className="mx-auto flex w-full max-w-3xl flex-col items-start gap-3 px-6 py-16">
        <p className="text-body">{t(($) => $.page.load_failed)}</p>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => void refetch()}
          disabled={isFetching}
          aria-busy={isFetching}
        >
          {isFetching ? <Spinner /> : null}
          {t(($) => $.page.retry)}
        </Button>
      </div>
    );
  }

  return <ReleaseDetail key={release.id} release={release} />;
}
