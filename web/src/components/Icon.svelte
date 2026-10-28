<script lang="ts">
  import { ICON_PATHS, type IconName } from "../lib/icons";

  let {
    name,
    size = 16,
    label = "",
  }: { name: IconName; size?: number; label?: string } = $props();
</script>

<svg
  class="icon"
  width={size}
  height={size}
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width="2"
  stroke-linecap="round"
  stroke-linejoin="round"
  role={label ? "img" : "presentation"}
  aria-label={label || undefined}
  aria-hidden={label ? undefined : "true"}
>
  {#if label}<title>{label}</title>{/if}
  <path d={ICON_PATHS[name]} />
</svg>

<style>
  .icon { flex: none; display: inline-block; vertical-align: -0.15em; }
</style>
