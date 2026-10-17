<script lang="ts">
import { displayPath, type Node, zoomHref } from "../system.ts";
import Icon from "./Icon.svelte";
import Pill from "./Pill.svelte";

/** Node, Node / Focus or Node / Outside of the template, at its exact box. */
let { node, pathRoot }: { node: Node; pathRoot: string } = $props();
</script>

<div
  class="node {node.kind}"
  style:left="{node.x}px"
  style:top="{node.y}px"
  style:width="{node.w}px"
  style:height="{node.h}px"
  data-node={node.id}
  data-testid="node-{node.id}"
>
  <div class="head">
    <span class="title">
      <Icon name={node.icon} color="var(--muted-foreground)" />
      <span class="name">{node.name}</span>
    </span>
    {#if node.path !== undefined}
      <span class="path" title={node.path}>{displayPath(node.path, pathRoot)}</span>
    {:else}
      <span class="caption">{node.caption}</span>
    {/if}
  </div>
  <div class="content">
    {#if node.parts !== undefined && node.opens !== undefined}
      <div class="parts">
        <span class="parts-title">Opens into</span>
        {#each node.parts as part (part)}
          <span class="part">{part}</span>
        {/each}
      </div>
      <div class="badges">
        <Pill variant="kind">{node.badge}</Pill>
        <Pill variant="zoom" href={zoomHref(node.opens)} testid="drill-{node.opens}">
          Zoom {node.opens}
        </Pill>
      </div>
    {:else}
      <div class="badges">
        <Pill variant={node.kind === "outside" ? "outside" : "kind"}>{node.badge}</Pill>
      </div>
    {/if}
  </div>
</div>

<style>
.node {
  position: absolute;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--background);
  box-shadow: var(--shadow);
}

.focus {
  border: 1.5px solid var(--primary);
}

.outside {
  background: var(--secondary);
  box-shadow: none;
}

.head {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 16px 16px 0;
}

.title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.name {
  font-weight: 600;
  line-height: 20px;
  white-space: nowrap;
}

.path,
.caption {
  font-size: 12px;
  line-height: 16px;
  color: var(--muted-foreground);
  white-space: nowrap;
}

.path {
  font-family: var(--font-mono);
}

.content {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 12px 16px 16px;
}

.parts {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-top: 12px;
  border-top: 1px solid var(--border);
}

.parts-title {
  font-size: 12px;
  font-weight: 500;
  line-height: 16px;
  color: var(--muted-foreground);
}

.part {
  font-size: 13px;
  line-height: 18px;
}

.badges {
  display: flex;
  gap: 8px;
}
</style>
