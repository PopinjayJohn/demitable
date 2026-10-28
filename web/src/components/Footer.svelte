<script lang="ts">
  // Footer status strip: at a glance, is everything ok. Surfaces report
  // into appstate and stop rendering their own status lines.
  import { appState } from "../lib/appstate.svelte";
  import { vaultStore } from "../lib/vaultstore.svelte";
  import { DASHBOARD_BUDGET_MS } from "../lib/dashboard";
  import { displayName } from "../lib/display";

  const eye = $derived(appState.playerEye && appState.eyeTarget ? appState.eyeTarget : "");
  const topCampaign = $derived(vaultStore.campaigns[0]?.[0] ?? "");
</script>

<footer class="statusbar" aria-label="Status">
  <span class="cell" title={appState.wsConnected ? "WebSocket connected" : "WebSocket disconnected"}>
    <span class="dot" class:on={appState.wsConnected}></span>
    {appState.wsConnected ? "live" : "offline"}
  </span>
  <span class="cell mono" title="Last save state">{appState.saveStatus}</span>
  <span class="cell mono" title="Dashboard render budget">notes {vaultStore.elapsedMs.toFixed(0)}ms/{DASHBOARD_BUDGET_MS}ms</span>
  {#if eye}
    <span class="cell" title="Player eye target">eye: {eye}</span>
  {/if}
  <span class="spacer"></span>
  {#if appState.rollStatus}
    <span class="cell mono">{appState.rollStatus}</span>
  {/if}
  <button class="footbtn" onclick={() => (appState.showJournal = !appState.showJournal)} title="App write journal with revert">ops log</button>
  <span class="cell">{vaultStore.counts.total} notes{#if topCampaign} · {displayName(topCampaign)}{/if}</span>
</footer>

<style>
  .statusbar {
    display: flex; align-items: center; gap: var(--space-4);
    min-height: 1.75rem; padding: 0 var(--space-4);
    background: var(--bg-surface-2); border-block-start: 1px solid var(--border-subtle);
    font-size: var(--text-2xs); color: var(--fg-muted);
  }
  .cell { display: inline-flex; align-items: center; gap: var(--space-1); white-space: nowrap; }
  .spacer { flex: 1; }
  .footbtn { background: none; border: 1px solid transparent; border-radius: var(--radius-sm); cursor: pointer; color: var(--fg-muted); font-size: var(--text-2xs); padding: 0 var(--space-1); }
  .footbtn:hover { background: var(--bg-surface-3); color: var(--fg-default); }
  .dot { width: 0.5rem; height: 0.5rem; border-radius: var(--radius-pill); background: var(--danger); }
  .dot.on { background: var(--success); }
</style>
