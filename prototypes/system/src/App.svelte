<script lang="ts">
import Diagram from "./lib/components/Diagram.svelte";
import PathTables from "./lib/components/PathTables.svelte";
import Sidebar from "./lib/components/Sidebar.svelte";
import ZoomBox from "./lib/components/ZoomBox.svelte";
import ZoomHeader from "./lib/components/ZoomHeader.svelte";
import { system, zoomById, zoomIdFromHash } from "./lib/system.ts";

/** The zoom lives in the hash (#/zoom/NN), so links drill down and browser back climbs up. */
let zoomId = $state(zoomIdFromHash(location.hash));
const zoom = $derived(zoomById(zoomId) ?? system.zooms[0]);

$effect(() => {
  const follow = () => {
    zoomId = zoomIdFromHash(location.hash);
  };
  addEventListener("hashchange", follow);
  return () => removeEventListener("hashchange", follow);
});
</script>

{#if zoom !== undefined}
  <div class="sheet" data-testid="sheet" data-zoom={zoom.id}>
    <Sidebar current={zoom.id} />
    <main class="main">
      <ZoomHeader {zoom} />
      {#if zoom.view === "map"}
        <div class="map">
          {#each system.zooms.filter((child) => child.parent === zoom.id) as child (child.id)}
            <ZoomBox zoom={child} />
          {/each}
        </div>
      {:else if zoom.view === "diagram"}
        <Diagram {zoom} />
      {:else}
        <PathTables {zoom} />
      {/if}
    </main>
  </div>
{/if}

<style>
.sheet {
  display: flex;
  width: 1600px;
  min-height: 1000px;
  background: var(--background);
}

.main {
  display: flex;
  flex-direction: column;
  gap: 24px;
  width: 1344px;
  padding: 32px 40px;
}

.map {
  display: flex;
  flex-direction: column;
  gap: 40px;
  padding: 24px 0 8px;
}
</style>
