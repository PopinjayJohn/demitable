<script lang="ts">
  // A collapsible sidebar section. Collapsed state persists per section
  // id across sessions; the sidebar toggle still governs the whole rail.
  import Icon from "./Icon.svelte";
  import { appState } from "../lib/appstate.svelte";
  import type { Snippet } from "svelte";

  let { id, title, count = null, children }: { id: string; title: string; count?: number | null; children: Snippet } = $props();

  const collapsed = $derived(appState.sidebarSections[id] === true);

  function toggle() {
    appState.sidebarSections[id] = !collapsed;
    try {
      localStorage.setItem("demitable:sections", JSON.stringify(appState.sidebarSections));
    } catch { /* ignore */ }
  }
</script>

<div class="side-block">
  <button class="side-head" onclick={toggle} aria-expanded={!collapsed} aria-label={(collapsed ? "Expand " : "Collapse ") + title}>
    <Icon name={collapsed ? "chevron-right" : "chevron-down"} size={13} />
    <h3 class="side-title">{title}</h3>
    {#if count !== null}<span class="side-count">{count}</span>{/if}
  </button>
  {#if !collapsed}
    {@render children()}
  {/if}
</div>

<style>
  .side-head { display: flex; align-items: center; gap: var(--space-1); width: 100%; background: none; border: none; cursor: pointer; padding: 0; margin-block-end: var(--space-2); color: var(--fg-subtle); }
  .side-head:hover { color: var(--fg-default); }
  .side-title { font-size: var(--text-2xs); font-weight: var(--weight-bold); letter-spacing: 0.08em; text-transform: uppercase; margin: 0; }
  .side-count { margin-inline-start: auto; font-size: var(--text-2xs); color: var(--fg-subtle); font-variant-numeric: tabular-nums; }
</style>
