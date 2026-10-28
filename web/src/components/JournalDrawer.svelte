<script lang="ts">
  import { fetchJournal, revertJournal } from "../lib/api";
  import Icon from "./Icon.svelte";

  let { open = true, onclose }: { open?: boolean; onclose?: () => void } = $props();
  let entries = $state<any[]>([]);
  let status = $state("loading");

  async function load() {
    status = "loading";
    try { entries = await fetchJournal(); status = `loaded · ${entries.length}`; }
    catch (e: any) { status = e?.message ?? "failed"; }
  }
  async function revert(id: string) {
    try { await revertJournal(id); await load(); }
    catch (e: any) { status = e?.message ?? "revert failed"; }
  }
  $effect(() => { if (open) void load(); });
</script>

{#if open}
<aside class="drawer" aria-label="Write journal">
  <header class="drawer__head">
    <h2><Icon name="scroll" size={15} /> Write journal</h2>
    <button class="btn btn--sm" onclick={onclose}>Close</button>
  </header>
  <p class="drawer__status mono" role="status">{status}</p>
  <ol class="entries">
    {#each entries.slice().reverse().slice(0, 100) as e (e.id ?? e.uid ?? JSON.stringify(e).slice(0, 24))}
      <li class="entry">
        <span class="entry__id mono">{String(e.id ?? e.uid ?? "?")}</span>
        <span class="entry__op">{String(e.op ?? e.kind ?? e.action ?? "write")}</span>
        <button class="btn btn--sm" onclick={() => revert(String(e.id ?? e.uid))}>Revert</button>
      </li>
    {/each}
  </ol>
</aside>
{/if}

<style>
  .drawer { width: min(26rem, 90vw); border-inline-start: 1px solid var(--border-default); background: var(--bg-surface); box-shadow: var(--shadow-3); display: flex; flex-direction: column; max-height: 100%; }
  .drawer__head { display: flex; align-items: center; justify-content: space-between; padding: var(--space-3) var(--space-4); border-block-end: 1px solid var(--border-subtle); }
  .drawer__status { padding: var(--space-1) var(--space-4); font-size: var(--text-2xs); color: var(--fg-subtle); }
  .entries { list-style: none; margin: 0; padding: var(--space-3) var(--space-4); overflow-y: auto; display: flex; flex-direction: column; gap: var(--space-2); }
  .entry { display: flex; align-items: center; gap: var(--space-2); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: var(--space-2); font-size: var(--text-xs); }
  .entry__id { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--fg-subtle); }
</style>
