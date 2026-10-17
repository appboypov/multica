<script lang="ts">
import { system, zoomHref } from "../system.ts";
import Icon from "./Icon.svelte";
import LineGlyph from "./LineGlyph.svelte";

let { current }: { current: string } = $props();
</script>

<aside class="sidebar">
  <div class="brand">
    <span class="logo"><Icon name="box" size={18} color="var(--primary-foreground)" /></span>
    <span class="brand-text">
      <span class="brand-name">{system.product}</span>
      <span class="brand-description">{system.description}</span>
    </span>
  </div>

  <nav class="group" aria-label="Zoom">
    <span class="group-title">Zoom</span>
    {#each system.zooms as zoom (zoom.id)}
      <a
        class="item"
        class:active={zoom.id === current}
        href={zoomHref(zoom.id)}
        aria-current={zoom.id === current ? "page" : undefined}
        data-testid="zoom-link-{zoom.id}"
      >
        <Icon name={zoom.icon} />
        <span>{zoom.title}</span>
      </a>
    {/each}
  </nav>

  <div class="spacer"></div>

  <div class="group">
    <span class="group-title">Lines</span>
    <span class="item static"><LineGlyph style="call" /><span>Call</span></span>
    <span class="item static"><LineGlyph style="event" /><span>Event</span></span>
    <span class="item static"><LineGlyph style="data" /><span>Data</span></span>
  </div>

  <div class="footer">
    <span class="question">{system.question}</span>
    <span class="source">{system.source}</span>
  </div>
</aside>

<style>
.sidebar {
  display: flex;
  flex-direction: column;
  gap: 16px;
  width: 256px;
  flex: none;
  padding: 8px;
  border-right: 1px solid var(--sidebar-border);
  color: var(--sidebar-foreground);
}

.brand {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
}

.logo {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border-radius: 10px;
  background: var(--sidebar-brand);
}

.brand-text {
  display: flex;
  flex-direction: column;
}

.brand-name {
  font-weight: 500;
}

.brand-description {
  font-size: 12px;
  line-height: 16px;
}

.group {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.group-title {
  padding: 8px;
  font-size: 12px;
  font-weight: 500;
  line-height: 16px;
  color: var(--muted-foreground);
}

.item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 6px;
}

a.item:hover,
.item.active {
  background: var(--sidebar-accent);
}

.spacer {
  flex: 1;
}

.footer {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px;
}

.question {
  width: 200px;
  font-weight: 500;
}

.source {
  font-size: 12px;
  line-height: 16px;
  color: var(--muted-foreground);
}
</style>
