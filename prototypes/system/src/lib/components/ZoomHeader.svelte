<script lang="ts">
import { lastZoomId, type Zoom } from "../system.ts";

let { zoom }: { zoom: Zoom } = $props();
</script>

<header class="header">
  <div class="text">
    <h1 data-testid="zoom-title">{zoom.title}</h1>
    <p>{zoom.description}</p>
  </div>
  <span class="badge">{zoom.view === "map" ? "Overview" : `Zoom ${zoom.id} of ${lastZoomId}`}</span>
</header>

<style>
.header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--border);
}

.text {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

h1 {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
  line-height: 32px;
}

p {
  margin: 0;
  color: var(--muted-foreground);
}

.badge {
  flex: none;
  padding: 2px 8px;
  border: 1px solid var(--border);
  border-radius: 16px;
  font-size: 12px;
  font-weight: 600;
  line-height: 16px;
}
</style>
