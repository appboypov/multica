"use client";

import { useRef, type KeyboardEvent } from "react";
import { ArrowUpDown, GripVertical } from "lucide-react";
import { DndContext, PointerSensor, closestCenter, useSensor, useSensors, type DragEndEvent } from "@dnd-kit/core";
import { SortableContext, arrayMove, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { Kbd } from "@multica/ui/components/ui/kbd";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import type { AnswerControlProps } from "./control-props";

/**
 * Rows to drag into order; `⌥↑` and `⌥↓` move the focused row. An untouched
 * list is the asker's order, which **Send** sends as it is.
 */
export function RankingControl({ question, draft, onDraftChange, disabled }: AnswerControlProps<"ranking">) {
  const { t } = useT("questions");
  const order = draft?.order ?? question.settings.items;
  const rows = useRef(new Map<string, HTMLElement>());
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 4 } }));

  const move = (from: number, to: number) => {
    if (to < 0 || to >= order.length || from === to) return;
    onDraftChange({ order: arrayMove(order, from, to) });
  };

  const onDragEnd = ({ active, over }: DragEndEvent) => {
    if (over) move(order.indexOf(String(active.id)), order.indexOf(String(over.id)));
  };

  const onKeyDown = (event: KeyboardEvent, index: number) => {
    if (!event.altKey || (event.key !== "ArrowUp" && event.key !== "ArrowDown")) return;
    event.preventDefault();
    const to = event.key === "ArrowUp" ? index - 1 : index + 1;
    const item = order[index];
    move(index, to);
    // The row keeps focus at its new place, so the keys can move it again.
    if (item !== undefined) requestAnimationFrame(() => rows.current.get(item)?.focus());
  };

  return (
    <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
      <SortableContext items={order} strategy={verticalListSortingStrategy} disabled={disabled}>
        <ol aria-label={t(($) => $.controls.ranking_label)} className="flex flex-col gap-2">
          {order.map((item, index) => (
            <RankingRow
              key={item}
              item={item}
              position={index + 1}
              disabled={disabled}
              rowRef={(node) => {
                if (node) rows.current.set(item, node);
                else rows.current.delete(item);
              }}
              onKeyDown={(event) => onKeyDown(event, index)}
            />
          ))}
        </ol>
      </SortableContext>
    </DndContext>
  );
}

function RankingRow({
  item,
  position,
  disabled,
  rowRef,
  onKeyDown,
}: {
  item: string;
  position: number;
  disabled: boolean;
  rowRef: (node: HTMLElement | null) => void;
  onKeyDown: (event: KeyboardEvent) => void;
}) {
  const { t } = useT("questions");
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: item, disabled });
  return (
    <li
      ref={(node) => {
        setNodeRef(node);
        rowRef(node);
      }}
      tabIndex={disabled ? -1 : 0}
      aria-label={t(($) => $.controls.ranking_item, { item, position })}
      onKeyDown={disabled ? undefined : onKeyDown}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className={cn(
        "flex h-10 items-center gap-3 rounded-lg border border-border bg-card px-3 text-body outline-none motion-reduce:transition-none!",
        "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
        isDragging && "z-10 shadow-[var(--surface-shadow)]",
      )}
    >
      <span
        aria-hidden
        className={cn("flex shrink-0 touch-none text-muted-foreground", !disabled && "cursor-grab active:cursor-grabbing")}
        {...attributes}
        {...listeners}
        tabIndex={-1}
        role={undefined}
      >
        <GripVertical className="size-4" />
      </span>
      <Kbd className="border bg-transparent">{position}</Kbd>
      <span className="min-w-0 flex-1 truncate">{item}</span>
      <ArrowUpDown aria-hidden className="size-4 shrink-0 text-muted-foreground" />
    </li>
  );
}
