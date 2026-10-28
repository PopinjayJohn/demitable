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
    <span class="side-head__rule" aria-hidden="true"></span>
    <Icon name={collapsed ? "chevron-right" : "chevron-down"} size={13} />
    <h3 class="side-title">{title}</h3>
    {#if count !== null}<span class="side-count">{count}</span>{/if}
  </button>
  {#if !collapsed}
    <div class="side-body">
      {@render children()}
    </div>
  {/if}
</div>

<style>
  .side-block { padding-block: var(--space-1); }
  /* Svelte flags this as unused (it cannot see sibling component
     instances), but every section root carries this class, so adjacent
     sections really do match. */
  .side-block + .side-block { border-block-start: 1px solid var(--border-subtle); padding-block-start: var(--space-3); }
  .side-head {
    display: flex; align-items: center; gap: var(--space-1);
    width: 100%; background: none; border: none; cursor: pointer;
    padding: var(--space-1); margin: 0 0 var(--space-2);
    border-radius: var(--radius-sm); color: var(--fg-subtle);
  }
  .side-head:hover { background: var(--bg-surface-3); color: var(--fg-default); }
  .side-head__rule { width: 3px; align-self: stretch; border-radius: var(--radius-sm); background: var(--accent-border); }
  .side-title { font-size: var(--text-2xs); font-weight: var(--weight-bold); letter-spacing: 0.08em; text-transform: uppercase; margin: 0; }
  .side-count { margin-inline-start: auto; font-size: var(--text-2xs); color: var(--fg-subtle); font-variant-numeric: tabular-nums; }
  .side-body { padding-inline-start: var(--space-1); }
</style>
