import { useEffect } from "react";
import type { SystemIndex, Zoom } from "./model.ts";
import { Icon } from "./parts.tsx";
import type { JourneyStep } from "./system.ts";

interface ModelsViewProps {
  index: SystemIndex;
  zoom: Zoom;
  step: JourneyStep | null;
  selected: string | null;
  onModel: (model: string) => void;
}

/** The models a part owns: their fields and relations. */
export function ModelsView({ index, zoom, step, selected, onModel }: ModelsViewProps) {
  const models = index.modelsOf(zoom.part!.id);
  const refs = [...(step?.in ?? []), ...(step?.out ?? [])];
  const target = selected ?? refs.find((ref) => models.some((model) => model.name === ref.model))?.model ?? null;

  useEffect(() => {
    if (target) document.getElementById(`model-${target}`)?.scrollIntoView({ block: "start", behavior: "smooth" });
  }, [target]);

  if (models.length === 0) return <div className="empty">This part owns no models.</div>;

  return (
    <div className="models-grid">
      {models.map((model) => {
        const modelRefs = refs.filter((ref) => ref.model === model.name);
        const lit = model.name === selected || modelRefs.length > 0;
        const litFields = new Set(modelRefs.flatMap((ref) => ref.fields ?? model.fields.map((field) => field.name)));
        return (
          <article className={`model-card${lit ? " lit" : ""}`} id={`model-${model.name}`} key={model.name}>
            <header className="model-head">
              <Icon name={model.kind === "table" ? "table-2" : "braces"} size={14} color="#737373" />
              <span className="model-name">{model.name}</span>
              <span className="badge">{model.kind === "table" ? "Table" : "Contract"}</span>
              <span className="model-path" title={model.path}>
                {model.path.split("/").pop()}
              </span>
            </header>
            {model.fields.map((field) => (
              <div className={`model-row${litFields.has(field.name) ? " lit" : ""}`} key={field.name}>
                <span>{field.name}</span>
                <span className="type">{field.type}</span>
              </div>
            ))}
            {model.relations && model.relations.length > 0 && (
              <div className="model-relations">
                {model.relations.map((relation) => (
                  <div className="model-relation" key={`${relation.to}-${relation.via}`}>
                    <span>
                      {relation.from} to {relation.toCount}
                    </span>
                    <button type="button" onClick={() => onModel(relation.to)}>
                      {relation.to}
                    </button>
                    <span>
                      via {relation.via}
                      {relation.soft ? " · no foreign key" : ""}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </article>
        );
      })}
    </div>
  );
}
