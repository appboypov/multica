import { useParams } from "react-router-dom";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorklane } from "@multica/core/worklanes";
import { WorklanePage } from "@multica/views/worklanes/components";
import { useDocumentTitle } from "@/hooks/use-document-title";

export function WorklaneDetailPage() {
  const { id } = useParams<{ id: string }>();
  const wsId = useWorkspaceId();
  const { data: worklane } = useWorklane(wsId, id ?? "");

  useDocumentTitle(worklane ? worklane.name : "Worklane");

  if (!id) return null;
  // Keyed by worklane: another worklane opened in the same tab starts on its
  // Timeline without the view and Type picks of the one before.
  return <WorklanePage key={id} worklaneId={id} />;
}
