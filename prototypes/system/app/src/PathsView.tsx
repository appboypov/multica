import { CornerDownRight } from "lucide-react";
import type { StepFocus } from "./journey.ts";
import { shownPath, type SystemIndex, type Zoom } from "./model.ts";
import { LineSample } from "./parts.tsx";
import type { Edge, Part } from "./system.ts";

interface PathsViewProps {
  index: SystemIndex;
  zoom: Zoom;
  focus: StepFocus | null;
  onOpen: (zoomId: string) => void;
}

interface Row {
  part: Part;
  depth: number;
}

/** A zoom of functions: the call path as an indented table, and the lines in and out. */
export function PathsView({ index, zoom, focus, onOpen }: PathsViewProps) {
  const opened = zoom.part!;
  const children = index.childrenOf(opened.id);
  const ids = new Set(children.map((part) => part.id));
  const calls = index.design.edges.filter(
    (edge) => edge.line === "call" && ids.has(edge.from) && ids.has(edge.to),
  );
  const called = new Set(calls.map((edge) => edge.to));

  const rows: Row[] = [];
  const seen = new Set<string>();
  const walk = (part: Part, depth: number) => {
    if (seen.has(part.id)) return;
    seen.add(part.id);
    rows.push({ part, depth });
    for (const edge of calls.filter((call) => call.from === part.id)) {
      walk(index.parts.get(edge.to)!, depth + 1);
    }
  };
  for (const part of children.filter((child) => !called.has(child.id))) walk(part, 0);
  for (const part of children) walk(part, 0);

  const graph = index.graph(zoom);
  const leaving = (part: Part): Edge[] =>
    index.design.edges.filter((edge) => edge.from === part.id && !(edge.line === "call" && ids.has(edge.to)));

  const base = zoom.info.base;
  return (
    <div className="views">
      <section className="section" style={{ flex: "3 1 0" }}>
        <div className="section-head">
          <span className="section-title">Call path</span>
          <span className="section-note">indent = called by the row above</span>
        </div>
        <table className="table">
          <thead>
            <tr>
              <th>Function</th>
              <th>File</th>
              <th>Output</th>
            </tr>
          </thead>
          <tbody>
            {rows.map(({ part, depth }) => (
              <tr key={part.id} className={focus?.parts.has(part.id) ? "lit" : undefined}>
                <td>
                  <span className="fn mono" style={{ paddingLeft: Math.max(0, depth - 1) * 16 }}>
                    {depth > 0 && <CornerDownRight size={14} />}
                    {part.name}
                  </span>
                </td>
                <td className="file" title={part.path}>
                  {shownPath(part, base)}
                </td>
                <td>
                  <div className="out">
                    {leaving(part).map((edge) => (
                      <span className="out" key={edge.id} title={`${edge.data} · ${edge.trigger}`}>
                        <LineSample line={edge.line} />
                        <span className="chip strong">{edge.event ?? edge.data}</span>
                      </span>
                    ))}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      <section className="section" style={{ flex: "2 1 0" }}>
        <div className="section-head">
          <span className="section-title">Lines in and out</span>
          <span className="section-note">as drawn one zoom up</span>
        </div>
        <table className="table">
          <thead>
            <tr>
              <th>From</th>
              <th>Line</th>
              <th>To</th>
            </tr>
          </thead>
          <tbody>
            {graph.edges
              .filter((edge) => !ids.has(edge.from) || !ids.has(edge.to))
              .map((edge) => {
                const end = (id: string) => {
                  const part = index.parts.get(id)!;
                  return index.zoomById.has(id) && !ids.has(id) ? (
                    <button type="button" className="chip" onClick={() => onOpen(id)}>
                      {part.name}
                    </button>
                  ) : (
                    <span className="mono">{part.name}</span>
                  );
                };
                const lit = focus?.edge ? edge.edges.some((candidate) => candidate.id === focus.edge) : false;
                return (
                  <tr key={edge.id} className={lit ? "lit" : undefined}>
                    <td>{end(edge.from)}</td>
                    <td title={edge.title}>
                      <span className="out">
                        <LineSample line={edge.line} />
                        <span className="muted">{edge.label}</span>
                      </span>
                    </td>
                    <td>{end(edge.to)}</td>
                  </tr>
                );
              })}
          </tbody>
        </table>
      </section>
    </div>
  );
}
