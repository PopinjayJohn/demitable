<script lang="ts">
  // GM session control: start a live session or end the active one.
  // The active session id lives in shared state — the tabletop and the
  // session journal both follow it.
  import { createSession, closeSession } from "../lib/api";
  import { vaultStore } from "../lib/vaultstore.svelte";
  import { appState } from "../lib/appstate.svelte";
  import Icon from "./Icon.svelte";
  import { displayName } from "../lib/display";

  let open = $state(false);
  let name = $state("");
  let campaign = $state("");
  let status = $state("");

  async function start() {
    status = "starting";
    try {
      const camp = campaign || vaultStore.campaigns[0]?.[0] || "";
      const s = await createSession({ name: name.trim() || "Session", campaign: camp });
      appState.sessionId = String(s.id ?? s.session_id ?? "");
      status = `live: ${appState.sessionId}${s.code ? ` · code ${s.code}` : ""}`;
      name = "";
    } catch (e: any) { status = e?.message ?? "start failed"; }
  }
  async function end() {
    if (!appState.sessionId) return;
    status = "closing";
    try {
      await closeSession(appState.sessionId);
      appState.sessionId = "";
      appState.encounterId = "";
      status = "ended";
      open = false;
    } catch (e: any) { status = e?.message ?? "close failed"; }
  }
</script>

<div class="sess">
  <button class="btn btn--sm" class:is-active={appState.sessionId !== ""} onclick={() => (open = !open)} title={appState.sessionId ? `Session ${appState.sessionId} (write-back on close)` : "No active session"}>
    <Icon name="dice" size={13} /> {appState.sessionId ? `Session ${appState.sessionId.slice(0, 8)}` : "Session"}
  </button>
  {#if open}
    <div class="pop" role="dialog" aria-label="Session control">
      {#if appState.sessionId}
        <p class="mono">{appState.sessionId}</p>
        <p class="hint">Closing triggers the write-back contract (partial apply, per-key conflicts).</p>
        <button class="btn btn--sm" onclick={end}>End session</button>
      {:else}
        <label>Name <input bind:value={name} placeholder="Session" aria-label="Session name" /></label>
        <label>Campaign
          <select bind:value={campaign} aria-label="Session campaign">
            <option value="">auto</option>
            {#each vaultStore.campaigns as [c]}
              <option value={c}>{displayName(c)}</option>
            {/each}
          </select>
        </label>
        <button class="btn btn--sm" onclick={start}>Start session</button>
      {/if}
      {#if status}<p class="mono hint" role="status">{status}</p>{/if}
    </div>
  {/if}
</div>

<style>
  .sess { position: relative; }
  .pop {
    position: fixed; inset-block-start: calc(var(--topbar-height) + 4px);
    background: var(--bg-surface); border: 1px solid var(--border-default);
    border-radius: var(--radius-md); box-shadow: var(--shadow-3);
    padding: var(--space-3); z-index: var(--z-dropdown);
    display: flex; flex-direction: column; gap: var(--space-2); min-width: 16rem;
  }
  .pop label { display: flex; flex-direction: column; gap: 2px; font-size: var(--text-xs); color: var(--fg-muted); }
  .pop input, .pop select { padding: var(--space-1) var(--space-2); border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--bg-surface); color: var(--fg-default); font-size: var(--text-sm); }
  .hint { font-size: var(--text-xs); color: var(--fg-subtle); }
</style>
