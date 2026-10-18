import { icons, type LucideProps } from "lucide-react";
import type { CSSProperties } from "react";
import type { PlacedPart } from "./layout.ts";
import { shownPath, zoomNumber } from "./model.ts";
import type { LineType } from "./system.ts";

/** A lucide icon by its kebab-case name, such as `git-pull-request`. */
export function Icon({ name, ...props }: { name: string } & LucideProps) {
  const pascal = name
    .split("-")
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join("") as keyof typeof icons;
  const Component = icons[pascal] ?? icons.Box;
  return <Component size={16} strokeWidth={2} {...props} />;
}

/** Stroke pattern of a line type in the diagram. */
export const DASH: Record<LineType, string | undefined> = {
  call: undefined,
  event: "5 4",
  data: "0.01 3.99",
};

export const STROKE_WIDTH: Record<LineType, number> = {
  call: 1.5,
  event: 1.5,
  data: 2,
};

/** The 28x8 line sample of the sidebar legend and table outputs. */
export function LineSample({ line }: { line: LineType }) {
  const path: Record<LineType, string> = {
    call: "M1 4h26",
    event: "M1 4h4m3 0h4m3 0h4m3 0h4",
    data: "M1 4h0.01m3.99 0h0.01m3.99 0h0.01m3.99 0h0.01m3.99 0h0.01m3.99 0h0.01m3.99 0h0.01",
  };
  return (
    <svg width="28" height="8" viewBox="0 0 28 8" aria-hidden="true">
      <path
        d={path[line]}
        fill="none"
        stroke="#737373"
        strokeWidth={STROKE_WIDTH[line]}
        strokeLinecap="round"
      />
    </svg>
  );
}

interface NodeProps {
  placed: PlacedPart;
  base: string | undefined;
  /** Level of the zoom the focus node opens. */
  zoomLevel?: number;
  className?: string;
  onOpen?: () => void;
}

/** Node, Node / Outside and Node / Focus from the template. */
export function Node({ placed, base, zoomLevel, className, onOpen }: NodeProps) {
  const { part, variant } = placed;
  const path = shownPath(part, base);
  const style: CSSProperties = {
    left: placed.x,
    top: placed.y,
    width: placed.width,
    height: placed.height,
  };
  const classes = ["node", variant === "node" ? "" : variant, className ?? ""].join(" ").trim();
  const body = (
    <>
      <div className="node-header">
        <div className="node-top">
          <Icon name={part.icon} />
          <span className="node-name">{part.name}</span>
        </div>
        {path && (
          <div className="node-path" title={part.path ?? part.note}>
            {path}
          </div>
        )}
      </div>
      <div className="node-content">
        {variant === "focus" && placed.opens.length > 0 && (
          <div className="node-parts">
            <div className="node-parts-title">Opens into</div>
            {placed.opens.map((name) => (
              <div className="node-part" key={name}>
                {name}
              </div>
            ))}
          </div>
        )}
        <div className="node-badges">
          <span className={variant === "outside" ? "badge outline" : "badge"}>
            {variant === "outside" ? "Outside" : part.kind}
          </span>
          {variant === "focus" && zoomLevel !== undefined && (
            <span className="badge dark">Zoom {zoomNumber(zoomLevel)}</span>
          )}
        </div>
      </div>
    </>
  );
  if (onOpen) {
    return (
      <button type="button" className={classes} style={style} onClick={onOpen}>
        {body}
      </button>
    );
  }
  return (
    <div className={classes} style={style}>
      {body}
    </div>
  );
}
