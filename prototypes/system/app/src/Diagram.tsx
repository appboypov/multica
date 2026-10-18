import { useEffect, useState } from "react";
import type { StepFocus } from "./journey.ts";
import { layoutDiagram, layoutProblems, type DiagramLayout, type Point } from "./layout.ts";
import type { SystemIndex, Zoom } from "./model.ts";
import { DASH, Node, STROKE_WIDTH } from "./parts.tsx";

const layouts = new Map<string, Promise<DiagramLayout>>();

const CORNER = 8;
const ARROW_LENGTH = 7;

/** The point `ARROW_LENGTH` back from `tip` towards `from`, where the line meets its arrowhead. */
function shortened(tip: Point, from: Point): Point {
  const length = Math.hypot(tip.x - from.x, tip.y - from.y);
  return {
    x: tip.x - ((tip.x - from.x) / length) * ARROW_LENGTH,
    y: tip.y - ((tip.y - from.y) / length) * ARROW_LENGTH,
  };
}

/** An orthogonal polyline with rounded corners, stopped short of its arrowheads. */
function edgePath(points: Point[], both: boolean): string {
  const trimmed = [
    both ? shortened(points[0], points[1]) : points[0],
    ...points.slice(1, -1),
    shortened(points[points.length - 1], points[points.length - 2]),
  ];
  let d = `M${trimmed[0].x} ${trimmed[0].y}`;
  for (let i = 1; i < trimmed.length - 1; i++) {
    const [prev, point, next] = [trimmed[i - 1], trimmed[i], trimmed[i + 1]];
    const inLength = Math.hypot(point.x - prev.x, point.y - prev.y);
    const outLength = Math.hypot(next.x - point.x, next.y - point.y);
    const radius = Math.min(CORNER, inLength / 2, outLength / 2);
    const start = {
      x: point.x - ((point.x - prev.x) / inLength) * radius,
      y: point.y - ((point.y - prev.y) / inLength) * radius,
    };
    const end = {
      x: point.x + ((next.x - point.x) / outLength) * radius,
      y: point.y + ((next.y - point.y) / outLength) * radius,
    };
    d += ` L${start.x} ${start.y} Q${point.x} ${point.y} ${end.x} ${end.y}`;
  }
  const end = trimmed[trimmed.length - 1];
  return `${d} L${end.x} ${end.y}`;
}

function arrowTransform(tip: Point, from: Point): string {
  const angle = (Math.atan2(tip.y - from.y, tip.x - from.x) * 180) / Math.PI + 90;
  return `translate(${tip.x} ${tip.y}) rotate(${angle}) translate(-4.5 0)`;
}

interface DiagramProps {
  index: SystemIndex;
  zoom: Zoom;
  focus: StepFocus | null;
  onOpen: (zoomId: string) => void;
}

export function Diagram({ index, zoom, focus, onOpen }: DiagramProps) {
  const [layout, setLayout] = useState<DiagramLayout | null>(null);

  useEffect(() => {
    let current = true;
    let pending = layouts.get(zoom.id);
    if (!pending) {
      pending = layoutDiagram(index, index.graph(zoom));
      layouts.set(zoom.id, pending);
      pending.then((result) => {
        const problems = layoutProblems(result);
        if (problems.length > 0) console.warn(`[system] zoom ${zoom.id}: ${problems.map((problem) => problem.text).join("; ")}`);
      });
    }
    pending.then((result) => current && setLayout(result));
    return () => {
      current = false;
    };
  }, [index, zoom]);

  if (!layout) return <div className="empty">Laying out {zoom.info.name}</div>;

  const opened = zoom.part?.id ?? null;
  const stepEdge = focus?.edge ? index.design.edges.find((edge) => edge.id === focus.edge) : undefined;
  const litEdges = new Set(
    layout.edges
      .filter((placed) => stepEdge && placed.edge.edges.includes(stepEdge))
      .map((placed) => placed.edge.id),
  );
  const base = zoom.info.base;

  return (
    <div className="diagram" style={{ width: layout.width, height: layout.height }}>
      {layout.boundary && (
        <div
          className="boundary"
          style={{
            left: layout.boundary.x,
            top: layout.boundary.y,
            width: layout.boundary.width,
            height: layout.boundary.height,
          }}
        >
          <span className="boundary-label" style={{ left: layout.boundary.labelX }}>
            {layout.boundary.label}
          </span>
        </div>
      )}

      {layout.chips && (
        <div
          className="chips-block"
          style={{ left: layout.chips.x, top: layout.chips.y, width: layout.chips.width }}
        >
          <span className="caption">Also inside, not on this branch</span>
          <div className="chips-columns">
            {layout.chips.columns.map((column, i) => (
              <div className="chips-column" key={i}>
                {column.map((part) =>
                  index.zoomById.has(part.id) ? (
                    <button type="button" className="chip" key={part.id} onClick={() => onOpen(part.id)}>
                      {part.name}
                    </button>
                  ) : (
                    <span className="chip" key={part.id} title={part.path ?? part.note}>
                      {part.name}
                    </span>
                  ),
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      <svg width={layout.width} height={layout.height} aria-hidden="true">
        {layout.edges.map((placed) => {
          const lit = litEdges.has(placed.edge.id);
          const dim = focus !== null && !lit;
          const { points } = placed;
          const { both } = placed.edge;
          const d = edgePath(points, both);
          const fill = lit ? "#171717" : "#737373";
          const back = lit && both && index.representative(stepEdge!.from, opened).part.id === placed.edge.to;
          return (
            <g className={`edge${dim ? " dim" : ""}`} key={placed.edge.id}>
              <path
                d={d}
                fill="none"
                stroke={lit ? "#171717" : "#737373"}
                strokeWidth={STROKE_WIDTH[placed.edge.line]}
                strokeDasharray={DASH[placed.edge.line]}
                strokeLinecap="round"
                strokeLinejoin="round"
              />
              <path d="M4.5 0l4.5 8-9 0z" fill={fill} transform={arrowTransform(points[points.length - 1], points[points.length - 2])} />
              {both && <path d="M4.5 0l4.5 8-9 0z" fill={fill} transform={arrowTransform(points[0], points[1])} />}
              {lit && (
                <circle r="5" fill="#171717">
                  <animateMotion
                    dur="1.6s"
                    repeatCount="indefinite"
                    path={d}
                    {...(back ? { keyPoints: "1;0", keyTimes: "0;1", calcMode: "linear" } : {})}
                  />
                </circle>
              )}
            </g>
          );
        })}
      </svg>

      {layout.parts.map((placed) => {
        const opens = index.zoomById.get(placed.part.id);
        const lit = focus?.parts.has(placed.part.id) ?? false;
        return (
          <Node
            key={placed.part.id}
            placed={placed}
            base={base}
            zoomLevel={opens?.level}
            className={focus ? (lit ? "lit" : "dim") : undefined}
            onOpen={opens ? () => onOpen(placed.part.id) : undefined}
          />
        );
      })}

      {layout.edges.map((placed) => {
        const lit = litEdges.has(placed.edge.id);
        const dim = focus !== null && !lit;
        return (
          <div
            key={placed.edge.id}
            className={`edge-label${lit ? " lit" : ""}${dim ? " dim" : ""}`}
            title={placed.edge.title}
            style={{
              left: placed.label.x,
              top: placed.label.y,
              width: placed.label.width,
            }}
          >
            {placed.edge.label}
          </div>
        );
      })}
    </div>
  );
}
