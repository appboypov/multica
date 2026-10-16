"use client";

import { use } from "react";
import { ReleaseDetailRoute } from "@multica/views/releases/components";

export default function ReleaseDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  return <ReleaseDetailRoute key={id} releaseId={id} />;
}
