"use client";

import { use } from "react";
import { WorklanePage } from "@multica/views/worklanes/components";

export default function WorklaneDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  // Keyed by worklane: another worklane starts on its Timeline without the
  // view and Type picks of the one before.
  return <WorklanePage key={id} worklaneId={id} />;
}
