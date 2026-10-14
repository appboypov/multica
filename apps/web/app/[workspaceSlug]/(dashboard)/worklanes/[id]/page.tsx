"use client";

import { use } from "react";
import { WorklanePage } from "@multica/views/worklanes/components";

export default function WorklaneDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  return <WorklanePage worklaneId={id} />;
}
