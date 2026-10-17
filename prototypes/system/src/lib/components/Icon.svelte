<script lang="ts">
import { icons } from "../icons.ts";
import type { IconName } from "../system.ts";

let {
  name,
  size = 16,
  color = "currentColor",
}: { name: IconName; size?: number; color?: string } = $props();

const Glyph = $derived(icons[name]);
</script>

<Glyph {size} {color} strokeWidth={2} aria-hidden="true" />
