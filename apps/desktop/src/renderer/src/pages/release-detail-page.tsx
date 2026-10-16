import { useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { formatVersion, releaseDetailOptions } from "@multica/core/releases";
import { ReleaseDetailRoute } from "@multica/views/releases/components";
import { useDocumentTitle } from "@/hooks/use-document-title";

export function ReleaseDetailPage() {
  const { id } = useParams<{ id: string }>();
  const wsId = useWorkspaceId();
  const { data: release } = useQuery({ ...releaseDetailOptions(wsId, id ?? ""), enabled: !!id });

  useDocumentTitle(release?.id ? `${formatVersion(release.version)} ${release.title}` : "Release");

  if (!id) return null;
  return <ReleaseDetailRoute key={id} releaseId={id} />;
}
