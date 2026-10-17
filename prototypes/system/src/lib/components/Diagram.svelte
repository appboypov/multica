<script lang="ts">
import { lineStroke } from "../lines.ts";
import { type DiagramZoom, zoomHref } from "../system.ts";
import Node from "./Node.svelte";
import Pill from "./Pill.svelte";

/** A zoom drawn at the coordinates system.json gives: boundaries, lines, nodes, labels. */
let { zoom }: { zoom: DiagramZoom } = $props();
</script>

<div class="diagram" style:height="{zoom.height}px">
  {#each zoom.boundaries as boundary (boundary.label)}
    <div
      class="boundary"
      style:left="{boundary.x}px"
      style:top="{boundary.y}px"
      style:width="{boundary.w}px"
      style:height="{boundary.h}px"
    >
      <span class="boundary-label">
        <Pill variant="boundary" href={zoomHref(boundary.up)} testid="boundary-up">
          {boundary.label}
        </Pill>
      </span>
    </div>
  {/each}

  <svg class="lines" viewBox="0 0 1264 {zoom.height}" aria-hidden="true">
    <defs>
      <marker
        id="arrow"
        viewBox="0 0 8 8"
        refX="8"
        refY="4"
        markerWidth="8"
        markerHeight="8"
        markerUnits="userSpaceOnUse"
        orient="auto"
      >
        <path d="M0 0.5 L8 4 L0 7.5 Z" fill="var(--line)" />
      </marker>
    </defs>
    {#each zoom.edges as edge (edge.id)}
      <polyline
        points={edge.points.map((point) => point.join(",")).join(" ")}
        fill="none"
        stroke="var(--line)"
        stroke-width={lineStroke[edge.style].width}
        stroke-dasharray={lineStroke[edge.style].dash}
        stroke-linecap={lineStroke[edge.style].cap}
        stroke-linejoin="round"
        marker-end="url(#arrow)"
        data-edge={edge.id}
        data-testid="edge-{edge.id}"
      />
    {/each}
  </svg>

  {#each zoom.nodes as node (node.id)}
    <Node {node} pathRoot={zoom.pathRoot} />
  {/each}

  {#each zoom.edges as edge (edge.id)}
    <span class="edge-label" style:left="{edge.label[0]}px" style:top="{edge.label[1]}px">
      <Pill variant="edge">{edge.data} · {edge.trigger}</Pill>
    </span>
  {/each}
</div>

<style>
.diagram {
  position: relative;
  width: 1264px;
}

.boundary {
  position: absolute;
  border: 1px solid var(--border);
  border-radius: 12px;
}

.boundary-label {
  position: absolute;
  top: 0;
  left: 24px;
  transform: translateY(-50%);
}

.lines {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  overflow: visible;
  pointer-events: none;
}

.edge-label {
  position: absolute;
  transform: translate(-50%, -50%);
}
</style>
