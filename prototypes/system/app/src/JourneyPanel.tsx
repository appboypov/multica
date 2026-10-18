import { ArrowLeft, ArrowRight, Route, X } from "lucide-react";
import type { StepFocus } from "./journey.ts";
import { zoomNumber, type SystemIndex, type Zoom } from "./model.ts";
import { LineSample } from "./parts.tsx";
import type { DataRef, Journey } from "./system.ts";

interface JourneyPanelProps {
  index: SystemIndex;
  journey: Journey;
  stepIndex: number;
  zoom: Zoom;
  focus: StepFocus;
  onStep: (step: number) => void;
  onClose: () => void;
  onOpen: (zoomId: string) => void;
  onModel: (model: string) => void;
}

/** The step of a played journey: what happens, its data, and how to go on. */
export function JourneyPanel(props: JourneyPanelProps) {
  const { index, journey, stepIndex, zoom, focus, onStep, onClose, onOpen, onModel } = props;
  const step = journey.steps[stepIndex];
  const next = journey.steps[stepIndex + 1];
  const edge = focus.edge ? index.design.edges.find((candidate) => candidate.id === focus.edge) : undefined;
  const actor = step.part ? index.parts.get(step.part) : undefined;

  const refs = (list: DataRef[]) =>
    list.map((ref) => (
      <button type="button" className="chip strong" key={ref.model} onClick={() => onModel(ref.model)}>
        {ref.model}
        {ref.fields ? `.${ref.fields.join(", ")}` : ""}
      </button>
    ));

  return (
    <section className="journey" aria-live="polite">
      <div>
        <div className="journey-head">
          <Route size={14} />
          <span>{journey.name}</span>
          <span>·</span>
          <span>
            Step {stepIndex + 1} of {journey.steps.length}
          </span>
        </div>
        <h2 className="journey-title">{step.title}</h2>
        <p className="journey-happens">{step.happens}</p>
      </div>
      <div className="journey-controls">
        <button type="button" className="button" disabled={stepIndex === 0} onClick={() => onStep(stepIndex - 1)}>
          <ArrowLeft size={14} />
          Back
        </button>
        <button
          type="button"
          className="button primary"
          disabled={!next}
          onClick={() => onStep(stepIndex + 1)}
        >
          Next
          <ArrowRight size={14} />
        </button>
        <button type="button" className="button" onClick={onClose} aria-label="Close journey">
          <X size={14} />
        </button>
      </div>

      <dl className="journey-facts">
        {edge && (
          <>
            <dt>Moves over</dt>
            <dd>
              <span className="out">
                <LineSample line={edge.line} />
                <span>
                  {index.parts.get(edge.from)!.name} to {index.parts.get(edge.to)!.name}
                </span>
                <span className="chip">
                  {edge.data} · {edge.trigger}
                </span>
                {edge.event && <span className="chip strong">{edge.event}</span>}
              </span>
            </dd>
          </>
        )}
        {actor && (
          <>
            <dt>Acts</dt>
            <dd>{actor.name}</dd>
          </>
        )}
        {step.in && step.in.length > 0 && (
          <>
            <dt>Data in</dt>
            <dd>{refs(step.in)}</dd>
          </>
        )}
        {step.out && step.out.length > 0 && (
          <>
            <dt>Data out</dt>
            <dd>{refs(step.out)}</dd>
          </>
        )}
        {step.store && (
          <>
            <dt>Stored in</dt>
            <dd>{step.store}</dd>
          </>
        )}
        {step.action && (
          <>
            <dt>Agent</dt>
            <dd>{step.action}</dd>
          </>
        )}
      </dl>

      <div className="journey-next">
        {focus.finest.id !== zoom.id && (
          <button type="button" className="chip strong" onClick={() => onOpen(focus.finest.id)}>
            See it in zoom {zoomNumber(focus.finest.level)} · {focus.finest.info.name}
          </button>
        )}
        <span>{next ? `Next: ${next.title}` : "Last step. Close the journey or pick another one."}</span>
      </div>
    </section>
  );
}
