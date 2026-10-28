<script lang="ts">
  // Breadcrumb for a vault path: folder segments navigate to the vault
  // explorer filtered to that folder; the last segment is the note
  // itself and is not a link.
  import { appState } from "../lib/appstate.svelte";

  let { path = "", current = "" }: { path?: string; current?: string } = $props();

  const parts = $derived(
    String(path ?? "")
      .split("/")
      .filter(Boolean)
      .map((seg) => seg.replace(/\.md$/, ""))
  );

  function openFolder(index: number) {
    appState.pathFilter = parts.slice(0, index + 1).join("/");
    appState.go("wiki", "explorer");
  }
</script>

<nav class="crumbs" aria-label="Note location">
  <button class="crumb crumb--root" onclick={() => { appState.pathFilter = null; appState.go("wiki", "explorer"); }} title="Vault root">⌂</button>
  {#each parts as seg, i}
    <span class="crumb__sep" aria-hidden="true">/</span>
    {#if i < parts.length - 1}
      <button class="crumb" onclick={() => openFolder(i)}>{seg}</button>
    {:else}
      <span class="crumb crumb--here" aria-current="page">{current || seg}</span>
    {/if}
  {/each}
</nav>

<style>
  .crumbs { display: flex; align-items: center; gap: var(--space-1); min-width: 0; flex-wrap: wrap; }
  .crumb {
    background: none; border: none; cursor: pointer; padding: 0;
    font-size: var(--text-xs); color: var(--fg-muted);
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 12rem;
  }
  button.crumb:hover { color: var(--accent); text-decoration: underline; }
  .crumb--root { font-size: var(--text-sm); }
  .crumb--here { color: var(--fg-default); font-weight: var(--weight-medium); }
  .crumb__sep { color: var(--fg-subtle); font-size: var(--text-xs); }
</style>
