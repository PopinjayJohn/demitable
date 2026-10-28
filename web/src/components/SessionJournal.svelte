<script lang="ts">
  // Session journal: the players' and GM's journal notes for the table.
  // Without an active session there is nothing to journal into, so the
  // panel prompts to start one instead of showing a dead composer.
  import { fetchNotes, createNote } from "../lib/api";
  import { vaultStore } from "../lib/vaultstore.svelte";
  import { appState } from "../lib/appstate.svelte";
  import Icon from "./Icon.svelte";

  let entries = $state<any[]>([]);
  let body = $state("");
  let campaign = $state("");
  let status = $state("");

  const active = $derived(appState.sessionId !== "");

  async function load() {
    status = "loading";
    try {
      const items = await fetchNotes({ type: "journal" } as any);
      entries = items.slice(-50).reverse();
      status = `${entries.length} entries`;
    } catch (e: any) { status = e?.message ?? "failed"; }
  }
  async function save() {
    if (!body.trim()) return;
    const camp = campaign || vaultStore.campaigns[0]?.[0] || "";
    status = "saving";
    try {
      await createNote({ type: "journal", name: `entry-${Date.now()}`, content: body.trim(), campaign: camp, visibility: "party" });
      body = "";
      await load();
    } catch (e: any) { status = e?.message ?? "save failed"; }
  }
  function open(uid: string) {
    appState.selectedUid = uid;
    appState.go("wiki", "browser");
    appState.showSessionJournal = false;
  }
  $effect(() => { if (appState.showSessionJournal) void load(); });
</script>

{#if appState.showSessionJournal}
  <div class="overlay" role="dialog" aria-label="Session journal">
    <header class="head">
      <h2><Icon name="scroll" size={15} /> Session journal</h2>
      <button class="btn btn--sm" onclick={() => (appState.showSessionJournal = false)}>Close</button>
    </header>
    {#if !active}
      <div class="empty">
        <p>No active session. Journals belong to a session — start one first.</p>
        <p class="hint">GM: use the Session button in the header.</p>
      </div>
    {:else}
      <p class="status mono" role="status">{appState.sessionId.slice(0, 8)} · {status}</p>
      <ol class="entries">
        {#each entries as e (e.uid)}
          <li>
            <button class="entry" onclick={() => open(String(e.uid))}>
              <span class="entry__name">{String(e.name ?? e.uid)}</span>
              <span class="entry__body">{String((e as any).snippet ?? (e as any).content ?? "").slice(0, 120)}</span>
            </button>
          </li>
        {/each}
      </ol>
      <div class="composer">
        <textarea bind:value={body} placeholder="Write a journal entry…" aria-label="Journal entry" rows="3"></textarea>
        <div class="row">
          <select bind:value={campaign} aria-label="Entry campaign">
            <option value="">campaign: auto</option>
            {#each vaultStore.campaigns as [c]}
              <option value={c}>{c}</option>
            {/each}
          </select>
          <button class="btn btn--sm" onclick={save}>Save entry</button>
        </div>
      </div>
    {/if}
  </div>
{/if}

<style>
  .overlay {
    position: fixed; inset-block: var(--topbar-height) 0; inset-inline-end: 0;
    width: min(30rem, 92vw); z-index: var(--z-modal);
    background: var(--bg-surface); border-inline-start: 1px solid var(--border-default);
    box-shadow: var(--shadow-3); display: flex; flex-direction: column;
  }
  .head { display: flex; align-items: center; justify-content: space-between; padding: var(--space-3) var(--space-4); border-block-end: 1px solid var(--border-subtle); }
  .head h2 { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-md); }
  .status { padding: var(--space-1) var(--space-4); font-size: var(--text-2xs); color: var(--fg-subtle); }
  .empty { padding: var(--space-6) var(--space-4); color: var(--fg-muted); font-size: var(--text-sm); }
  .hint { font-size: var(--text-xs); color: var(--fg-subtle); }
  .entries { list-style: none; margin: 0; padding: var(--space-3) var(--space-4); overflow-y: auto; flex: 1; display: flex; flex-direction: column; gap: var(--space-2); }
  .entry { width: 100%; text-align: start; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--bg-canvas); padding: var(--space-2) var(--space-3); cursor: pointer; }
  .entry:hover { border-color: var(--border-strong); }
  .entry__name { display: block; font-size: var(--text-sm); font-weight: var(--weight-medium); }
  .entry__body { display: block; font-size: var(--text-xs); color: var(--fg-muted); }
  .composer { padding: var(--space-3) var(--space-4); border-block-start: 1px solid var(--border-subtle); display: flex; flex-direction: column; gap: var(--space-2); }
  .composer textarea { padding: var(--space-2); border: 1px solid var(--border-default); border-radius: var(--radius-md); background: var(--bg-surface); color: var(--fg-default); font-size: var(--text-sm); resize: vertical; }
  .row { display: flex; gap: var(--space-2); align-items: center; }
  .row select { flex: 1; padding: var(--space-1) var(--space-2); border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--bg-surface); color: var(--fg-default); font-size: var(--text-sm); }
</style>
