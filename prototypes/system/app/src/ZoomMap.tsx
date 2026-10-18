import { WORLD_ZOOM_ID, zoomNumber, type SystemIndex, type Zoom } from "./model.ts";

interface ZoomMapProps {
  index: SystemIndex;
  onOpen: (zoomId: string) => void;
}

/** Zoom 00: every zoom as a boundary inside the zoom it opens from. */
export function ZoomMap({ index, onOpen }: ZoomMapProps) {
  const boundary = (zoom: Zoom, parentId: string | null) => {
    const parts = index.childrenOf(parentId);
    const nested = parts.filter((part) => index.zoomById.has(part.id));
    const chips = parts.filter((part) => !index.zoomById.has(part.id));
    const models = zoom.part ? index.modelsOf(zoom.part.id) : [];
    return (
      <div className={`map-boundary${nested.length === 0 ? " innermost" : ""}`} key={zoom.id}>
        <button type="button" className="boundary-label" onClick={() => onOpen(zoom.id)}>
          {zoom.part?.name ?? zoom.info.name} · zoom {zoomNumber(zoom.level)}
        </button>
        {(chips.length > 0 || models.length > 0) && (
          <div className="map-chips">
            {chips.map((part) => (
              <span className="chip" key={part.id} title={part.path ?? part.note}>
                {part.name}
              </span>
            ))}
            {models.length > 0 && (
              <button type="button" className="chip strong" onClick={() => onOpen(zoom.id)}>
                {models.length} models
              </button>
            )}
          </div>
        )}
        {nested.length > 0 && (
          <div className="map-children">
            {nested.map((part) => boundary(index.zoomById.get(part.id)!, part.id))}
          </div>
        )}
      </div>
    );
  };
  return boundary(index.zoomById.get(WORLD_ZOOM_ID)!, null);
}
