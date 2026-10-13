"use client";

import { Check } from "lucide-react";
import {
  PROJECT_PRIORITY_CONFIG,
  PROJECT_PRIORITY_ORDER
} from "@multica/core/projects/config";
import { cn } from "@multica/ui/lib/utils";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import type { Project, ProjectPriority, UpdateProjectRequest } from "@multica/core/types";
import { PriorityIcon } from "../../issues/components/priority-icon";
import { useProjectPriorityLabels } from "./labels";

export function ProjectPriorityBadge({ project, handleUpdate, triggerClassName, align = "end" }: { project: Project; handleUpdate: (data: UpdateProjectRequest) => void; triggerClassName?: string; align?: "start" | "end" | "center" }) {
  const priorityLabels = useProjectPriorityLabels();
  const priorityCfg = PROJECT_PRIORITY_CONFIG[project.priority];

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <button type="button" className={cn(
            "inline-flex items-center gap-1 rounded-xs px-1.5 py-0.5 text-caption font-medium hover:bg-accent/60 transition-colors cursor-pointer",
            triggerClassName
          )}>
            <PriorityIcon priority={project.priority} />
            <span className={cn("text-caption", priorityCfg.color)}>{priorityLabels[project.priority]}</span>
          </button>
        }
      />
      <DropdownMenuContent align={align} className="w-44">
        {PROJECT_PRIORITY_ORDER.map((p) => (
          <DropdownMenuItem key={p} onClick={() => handleUpdate({ priority: p as ProjectPriority })}>
            <PriorityIcon priority={p} />
            <span>{priorityLabels[p]}</span>
            {p === project.priority && <Check className="ml-auto h-3.5 w-3.5" />}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
