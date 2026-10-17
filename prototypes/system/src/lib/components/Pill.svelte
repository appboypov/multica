<script lang="ts">
import type { Snippet } from "svelte";

/**
 * The template's pill badges. edge: Edge label. boundary: Boundary label, a
 * link when href is set. kind, outside, zoom: the badges inside a Node.
 */
let {
  variant,
  href,
  testid,
  children,
}: {
  variant: "edge" | "boundary" | "kind" | "outside" | "zoom";
  href?: string | undefined;
  testid?: string | undefined;
  children: Snippet;
} = $props();
</script>

{#if href === undefined}
  <span class="pill {variant}" data-testid={testid}>{@render children()}</span>
{:else}
  <a class="pill {variant}" {href} data-testid={testid}>{@render children()}</a>
{/if}

<style>
.pill {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border: 1px solid transparent;
  border-radius: 16px;
  font-size: 12px;
  font-weight: 600;
  line-height: 16px;
  white-space: nowrap;
}

.edge {
  border-color: var(--border);
  background: var(--background);
  color: var(--muted-foreground);
  font-weight: 500;
}

.boundary,
.zoom {
  background: var(--primary);
  color: var(--primary-foreground);
}

.kind {
  background: var(--secondary);
  color: var(--secondary-foreground);
}

.outside {
  border-color: var(--border);
  color: var(--foreground);
}

a.pill:hover {
  background: #404040;
}
</style>
