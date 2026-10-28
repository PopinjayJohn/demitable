<script lang="ts">
  // The contextual right rail. Center surfaces publish a snippet into
  // appState.rail; null means the view has no context and the rail
  // stays out of the layout entirely.
  import { appState } from "../lib/appstate.svelte";
  import Icon from "./Icon.svelte";
</script>

{#if appState.rail && appState.railPinned}
  <aside class="rail" aria-label="Context">
    <button class="rail-hide" onclick={() => (appState.railPinned = false)} title="Hide context panel" aria-label="Hide context panel">×</button>
    {@render appState.rail()}
  </aside>
{:else if appState.rail}
  <button class="rail-fab" onclick={() => (appState.railPinned = true)} title="Show context panel" aria-label="Show context panel">
    <Icon name="info" size={16} />
  </button>
{/if}

<style>
  .rail {
    width: var(--inspector-width);
    min-height: 0;
    border-inline-start: 1px solid var(--border-subtle);
    background: var(--bg-surface-2);
    overflow-y: auto;
    padding: var(--space-4);
    position: relative;
  }
  .rail-hide {
    position: sticky; float: inline-end; inset-block-start: 0;
    background: none; border: none; cursor: pointer;
    color: var(--fg-subtle); font-size: var(--text-md); line-height: 1;
  }
  .rail-fab {
    position: absolute; inset-inline-end: var(--space-3); inset-block-start: var(--space-3);
    width: 2.25rem; height: 2.25rem; display: grid; place-items: center;
    border-radius: var(--radius-pill); border: 1px solid var(--border-default);
    background: var(--bg-surface); color: var(--fg-muted); cursor: pointer;
    box-shadow: var(--shadow-2); z-index: var(--z-sticky);
  }
</style>
