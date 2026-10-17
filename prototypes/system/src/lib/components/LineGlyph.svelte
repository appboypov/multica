<script lang="ts">
import { lineStroke } from "../lines.ts";
import type { LineStyle } from "../system.ts";

let { style, width = 28 }: { style: LineStyle; width?: number } = $props();

const stroke = $derived(lineStroke[style]);
</script>

<svg {width} height="8" viewBox="0 0 {width} 8" aria-hidden="true">
  <line
    x1={stroke.cap === "round" ? 1 : 0}
    y1="4"
    x2={stroke.cap === "round" ? width - 1 : width}
    y2="4"
    stroke="var(--line)"
    stroke-width={stroke.width}
    stroke-dasharray={stroke.dash}
    stroke-linecap={stroke.cap}
  />
</svg>
