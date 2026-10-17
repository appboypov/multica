<script lang="ts">
import { openerOf, system, type Zoom, zoomHref } from "../system.ts";
import Pill from "./Pill.svelte";
import ZoomBox from "./ZoomBox.svelte";

/** One zoom on the zoom map: its loose parts as chips, its child zooms nested inside. */
let { zoom }: { zoom: Zoom } = $props();

const children = $derived(system.zooms.filter((child) => child.parent === zoom.id));
const label = $derived(zoom.parent === "00" ? zoom.title : (openerOf(zoom)?.name ?? zoom.title));
const chips = $derived(
  zoom.view !== "diagram"
    ? []
    : zoom.nodes
        .filter((node) => node.kind === (zoom.parent === "00" ? "outside" : "node"))
        .map((node) => node.name),
);
const stats = $derived.by(() => {
  if (children.length > 0) return [];
  if (zoom.view === "paths") {
    return zoom.tables.map((table) => ({
      title: table.title,
      count: table.rows.length,
      badge: table.summary,
    }));
  }
  if (zoom.view === "diagram") {
    return [
      { title: "Parts", count: zoom.nodes.length, badge: "nodes" },
      { title: "Links", count: zoom.edges.length, badge: "edges" },
    ];
  }
  return [];
});
</script>

<section class="box" class:leaf={children.length === 0}>
  <span class="label">
    <Pill variant="boundary" href={zoomHref(zoom.id)} testid="map-zoom-{zoom.id}">
      {label}
      · zoom {zoom.id}
    </Pill>
  </span>

  {#if chips.length > 0}
    <div class="chips">
      {#each chips as chip (chip)}
        <Pill variant="edge">{chip}</Pill>
      {/each}
    </div>
  {/if}

  {#if stats.length > 0}
    <div class="stats">
      {#each stats as stat (stat.title)}
        <div class="stat">
          <span class="stat-title">{stat.title}</span>
          <span class="stat-count">{stat.count}</span>
          <span><Pill variant="zoom">{stat.badge}</Pill></span>
        </div>
      {/each}
    </div>
  {/if}

  {#if children.length > 0}
    <div class="children">
      {#each children as child (child.id)}
        <ZoomBox zoom={child} />
      {/each}
    </div>
  {/if}
</section>

<style>
.box {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 24px;
  padding: 24px;
  border: 1px solid var(--border);
  border-radius: 12px;
}

.leaf {
  border: 1.5px solid var(--primary);
}

.label {
  position: absolute;
  top: 0;
  left: 24px;
  transform: translateY(-50%);
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.children {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 40px 24px;
  padding: 24px 54px;
}

.children > :global(.box:has(.box)) {
  grid-column: 1 / -1;
}

.stats {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  padding-top: 8px;
}

.stat {
  display: flex;
  flex: 1 1 140px;
  flex-direction: column;
  gap: 8px;
  padding: 20px 16px;
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: var(--shadow);
}

.stat-title {
  font-size: 16px;
  font-weight: 500;
  line-height: 24px;
}

.stat-count {
  padding-top: 16px;
  font-size: 24px;
  line-height: 32px;
}
</style>
