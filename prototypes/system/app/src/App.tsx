import { Route as RouteIcon } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Diagram } from "./Diagram.tsx";
import { JourneyPanel } from "./JourneyPanel.tsx";
import { stepFocus } from "./journey.ts";
import { MAP_ZOOM_ID, SystemIndex, WORLD_ZOOM_ID, zoomNumber, type Zoom } from "./model.ts";
import { ModelsView } from "./ModelsView.tsx";
import { Icon, LineSample } from "./parts.tsx";
import { PathsView } from "./PathsView.tsx";
import { readRoute, routeHash, type Route } from "./route.ts";
import type { LineType, SystemDesign } from "./system.ts";
import { ZoomMap } from "./ZoomMap.tsx";

const LINES: Record<LineType, string> = { call: "Call", event: "Event", data: "Data" };

function zoomIcon(zoom: Zoom): string {
  if (zoom.id === MAP_ZOOM_ID) return "map";
  if (zoom.id === WORLD_ZOOM_ID) return "globe";
  return zoom.part!.icon;
}

export function App({ design }: { design: SystemDesign }) {
  const index = useMemo(() => new SystemIndex(design), [design]);
  const [route, setRoute] = useState<Route>(() => readRoute(window.location.hash));

  useEffect(() => {
    const onHash = () => setRoute(readRoute(window.location.hash));
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  }, []);

  const go = (next: Partial<Route>) => {
    window.location.hash = routeHash({ ...route, model: null, ...next });
  };

  const zoom = index.zoomById.get(route.zoom) ?? index.zoomById.get(MAP_ZOOM_ID)!;
  const journey = design.journeys.find((candidate) => candidate.id === route.journey) ?? null;
  const stepIndex = journey ? Math.min(route.step, journey.steps.length - 1) : 0;
  const step = journey ? journey.steps[stepIndex] : null;
  const focus = step && zoom.view !== "map" ? stepFocus(index, step, zoom) : null;

  useEffect(() => {
    if (!journey) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "ArrowRight" && stepIndex < journey.steps.length - 1) go({ step: stepIndex + 1 });
      if (event.key === "ArrowLeft" && stepIndex > 0) go({ step: stepIndex - 1 });
      if (event.key === "Escape") go({ journey: null, step: 0 });
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  useEffect(() => {
    document.title = `${zoom.info.title} · ${design.product.name} system design`;
  }, [zoom, design]);

  const openModel = (model: string) => {
    const owner = index.models.get(model)?.owner;
    if (owner) go({ zoom: owner, model });
  };

  return (
    <div className="sheet">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-tile">
            <Icon name="orbit" size={18} />
          </div>
          <div>
            <div className="brand-name">{design.product.name}</div>
            <div className="brand-sub">System design</div>
          </div>
        </div>

        <nav className="side-group" aria-label="Zoom">
          <div className="side-title">Zoom</div>
          {index.zooms.map((item) => (
            <a
              key={item.id}
              href={routeHash({ ...route, zoom: item.id, model: null })}
              className={`side-item${item.id === zoom.id ? " active" : ""}`}
              style={{ paddingLeft: 8 + Math.max(0, item.level - 2) * 12 }}
              aria-current={item.id === zoom.id ? "page" : undefined}
            >
              <Icon name={zoomIcon(item)} />
              <span>{item.info.name}</span>
            </a>
          ))}
        </nav>

        {design.journeys.length > 0 && (
          <nav className="side-group" aria-label="Journeys">
            <div className="side-title">Journeys</div>
            {design.journeys.map((item) => (
              <a
                key={item.id}
                href={routeHash({ ...route, journey: item.id, step: 0, model: null })}
                className={`side-item${item.id === journey?.id ? " active" : ""}`}
                title={item.question}
              >
                <RouteIcon size={16} />
                <span>{item.name}</span>
              </a>
            ))}
          </nav>
        )}

        <div className="side-group">
          <div className="side-title">Lines</div>
          {(Object.keys(LINES) as LineType[]).map((line) => (
            <div className="legend-row" key={line}>
              <LineSample line={line} />
              {LINES[line]}
            </div>
          ))}
        </div>

        <div className="side-spacer" />
        <div className="side-footer">
          <div className="side-question">{journey?.question ?? zoom.info.question}</div>
          <div className="side-source">{design.product.source}</div>
        </div>
      </aside>

      <main className="main">
        <header className="header">
          <div>
            <h1>{zoom.info.title}</h1>
            <p>{zoom.info.description}</p>
          </div>
          <span className="badge outline">
            {zoom.view === "map" ? "Overview" : `Zoom ${zoomNumber(zoom.level)} of ${zoomNumber(index.maxLevel)}`}
          </span>
        </header>

        {journey && step && focus && (
          <JourneyPanel
            index={index}
            journey={journey}
            stepIndex={stepIndex}
            zoom={zoom}
            focus={focus}
            onStep={(next) => go({ step: next })}
            onClose={() => go({ journey: null, step: 0 })}
            onOpen={(id) => go({ zoom: id })}
            onModel={openModel}
          />
        )}
        {journey && zoom.view === "map" && (
          <div className="empty">Open a zoom to play {journey.name}.</div>
        )}

        {zoom.view === "map" && <ZoomMap index={index} onOpen={(id) => go({ zoom: id })} />}
        {zoom.view === "diagram" && (
          <Diagram index={index} zoom={zoom} focus={focus} onOpen={(id) => go({ zoom: id })} />
        )}
        {zoom.view === "paths" && (
          <PathsView index={index} zoom={zoom} focus={focus} onOpen={(id) => go({ zoom: id })} />
        )}
        {zoom.view === "models" && (
          <ModelsView index={index} zoom={zoom} step={step} selected={route.model} onModel={openModel} />
        )}
      </main>
    </div>
  );
}
