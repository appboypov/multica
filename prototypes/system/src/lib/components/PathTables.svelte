<script lang="ts">
import CornerDownRight from "@lucide/svelte/icons/corner-down-right";
import { type CallsTable, displayPath, type FactsTable, type PathsZoom } from "../system.ts";
import LineGlyph from "./LineGlyph.svelte";
import Pill from "./Pill.svelte";

/** The leaf zoom of the template: the call path on the left, plain facts on the right. */
let { zoom }: { zoom: PathsZoom } = $props();

const calls = $derived(zoom.tables.filter((table): table is CallsTable => table.kind === "calls"));
const facts = $derived(zoom.tables.filter((table): table is FactsTable => table.kind === "facts"));
</script>

<div class="columns">
  <div class="column">
    {#each calls as table (table.title)}
      <section class="block">
        <div class="block-head">
          <h2>{table.title}</h2>
          <span>{table.hint}</span>
        </div>
        <table>
          <thead>
            <tr>
              {#each table.columns as column (column)}
                <th>{column}</th>
              {/each}
            </tr>
          </thead>
          <tbody>
            {#each table.rows as row, index (index)}
              <tr data-row>
                <td>
                  <span
                    class="function"
                    class:root={row.indent === 0}
                    style:padding-left="{Math.max(row.indent - 1, 0) * 16}px"
                  >
                    {#if row.indent > 0}
                      <CornerDownRight
                        size={14}
                        color="var(--muted-foreground)"
                        aria-hidden="true"
                      />
                    {/if}
                    {row.name}
                  </span>
                </td>
                <td class="path" title={row.path}>{displayPath(row.path, zoom.pathRoot)}</td>
                <td>
                  {#if row.out !== undefined}
                    <span class="out">
                      <LineGlyph style={row.out.style} width={16} />
                      <Pill variant="outside">{row.out.text}</Pill>
                    </span>
                  {/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </section>
    {/each}
  </div>

  <div class="column">
    {#each facts as table (table.title)}
      <section class="block">
        <div class="block-head">
          <h2>{table.title}</h2>
          <span>{table.hint}</span>
        </div>
        <table>
          <thead>
            <tr>
              {#each table.columns as column (column)}
                <th>{column}</th>
              {/each}
            </tr>
          </thead>
          <tbody>
            {#each table.rows as row, index (index)}
              <tr data-row>
                {#each row as cell, column (column)}
                  {#if column === row.length - 1}
                    <td class="path" title={cell}>{displayPath(cell, zoom.pathRoot)}</td>
                  {:else}
                    <td class:fact-key={column === 0}>{cell}</td>
                  {/if}
                {/each}
              </tr>
            {/each}
          </tbody>
        </table>
      </section>
    {/each}
  </div>
</div>

<style>
.columns {
  display: grid;
  grid-template-columns: 1fr max-content;
  gap: 48px;
  align-items: start;
}

.column {
  display: flex;
  flex-direction: column;
  gap: 40px;
}

.block {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.block-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 16px;
  color: var(--muted-foreground);
}

h2 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  line-height: 20px;
  color: var(--foreground);
}

table {
  width: 100%;
  border: 1px solid var(--border);
  border-radius: 8px;
  border-spacing: 0;
  background: var(--background);
}

th {
  padding: 10px 12px;
  font-weight: 400;
  line-height: 20px;
  text-align: left;
  color: var(--muted-foreground);
}

td {
  padding: 9px 12px;
  border-top: 1px solid var(--border);
  line-height: 20px;
  white-space: nowrap;
}

.function {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-family: var(--font-mono);
  font-size: 13px;
}

.function.root {
  font-weight: 600;
}

.path {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--muted-foreground);
}

.fact-key {
  font-family: var(--font-mono);
  font-size: 13px;
}

.out {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
</style>
