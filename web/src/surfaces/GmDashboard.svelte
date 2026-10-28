<script lang="ts">
  import { onMount } from "svelte";
  import { loadDashboard, DASHBOARD_BUDGET_MS, surfaceVisibility } from "../lib/dashboard";
  import { fetchNote, fetchNoteRender } from "../lib/api";
  import { WSClient } from "../lib/ws";
  import VisibilityBadge from "../components/VisibilityBadge.svelte";
  import VisibilityInspector from "../components/VisibilityInspector.svelte";
  import MarkdownView from "../components/MarkdownView.svelte";
  import Icon from "../components/Icon.svelte";
  import { appState } from "../lib/appstate.svelte";
  import { getToken } from "../lib/auth";
  import type { NoteSummary } from "../lib/api";

  let notes = $state<NoteSummary[]>([]);
  let counts = $state({ party: 0, gm: 0, owner: 0, private: 0, total: 0 });
  let attention = $state<NoteSummary[]>([]);
  let status = $state("loading");
  let elapsedMs = $state(0);
  let error = $state("");
  let filter = $state<"all" | "party" | "gm" | "owner" | "private">("all");
  let selectedUid = $state<string | null>(null);
  let previewHtml = $state("");
  let decisions = $state<any[]>([]);
  let strippedCount = $state(0);

  function vis(n: NoteSummary): string {
    try { return surfaceVisibility(n); } catch { return "gm"; }
  }
  function defaulted(n: NoteSummary): boolean {
    return (n as any).defaulted ?? (n as any).visibility === undefined;
  }
  function effectiveOf(n: NoteSummary): string { return vis(n); }

  const listed = $derived(notes.filter((n) => filter === "all" ? true : vis(n) === filter));
  const selected = $derived(notes.find((n) => String(n.uid) === selectedUid) ?? null);
  const campaigns = $derived(() => {
    const m = new Map<string, number>();
    for (const n of notes) m.set(String((n as any).campaign ?? "No campaign"), (m.get(String((n as any).campaign ?? "No campaign")) ?? 0) + 1);
    return [...m.entries()];
  });

  async function load() {
    status = "loading"; error = "";
    try {
      const data = await loadDashboard();
      notes = data.notes; counts = data.counts; attention = data.attention; elapsedMs = data.elapsedMs;
      if (!selectedUid && notes.length) selectedUid = String(notes[0].uid);
      status = "loaded";
      await loadPreview();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
      status = "failed";
    }
  }

  async function loadPreview() {
    if (!selectedUid) { previewHtml = ""; return; }
    try {
      previewHtml = await fetchNoteRender(selectedUid);
      decisions = [];
      strippedCount = 0;
    } catch { previewHtml = ""; }
  }

  function reveal(blockId: string) {
    const t = getToken();
    if (!t || !selectedUid) return;
    const proto = location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(`${proto}://${location.host}/ws?token=${encodeURIComponent(t)}`);
    ws.onopen = () => {
      ws.send(JSON.stringify({ v: 1, type: "secret.reveal", data: { note_uid: selectedUid, block_id: blockId, audience: { all: true } } }));
      setTimeout(() => ws.close(), 500);
    };
    ws.onmessage = () => loadPreview();
  }

  function openInEditor() {
    if (selectedUid) { appState.selectedUid = selectedUid; appState.surface = "editor"; }
  }

  onMount(() => { void load(); });
</script>

<section class="dash" aria-label="GM dashboard">
  <aside class="dash__side">
    <div class="side-block">
      <h3 class="side-title">Campaigns</h3>
      <ul class="side-list">
        {#each campaigns() as [name, count]}
          <li><span class="side-item"><Icon name="scroll" size={14} /><span>{name}</span><span class="side-count">{count}</span></span></li>
        {/each}
      </ul>
    </div>
    <div class="side-block">
      <h3 class="side-title">Needs attention</h3>
      {#if attention.length === 0}
        <p class="side-note">Nothing needs attention.</p>
      {:else}
      <ul class="side-list">
        {#each attention as n (n.uid)}
          <li><button class="side-item side-item--warn" onclick={() => (selectedUid = String(n.uid), void loadPreview())}><Icon name="warning" size={14} /><span class="side-item__name">{String(n.name ?? n.uid)}</span></button></li>
        {/each}
      </ul>
      {/if}
    </div>
    <div class="side-block">
      <h3 class="side-title">What the table can see</h3>
      <dl class="counts">
        <div class="counts__row"><dt><VisibilityBadge visibility="party" size="sm" /></dt><dd>{counts.party} of {counts.total} notes</dd></div>
        <div class="counts__row"><dt><VisibilityBadge visibility="gm" size="sm" /></dt><dd>{counts.total - counts.party} hidden</dd></div>
      </dl>
      <p class="side-note">Names of hidden notes stay in this panel — a player never sees it.</p>
      <p class="side-note mono">render {elapsedMs.toFixed(1)}ms / {DASHBOARD_BUDGET_MS}ms · {status}{error ? ` · ${error}` : ""}</p>
    </div>
  </aside>

  <section class="dash__list">
    <header class="list-head">
      <h2 class="list-head__title">Notes</h2>
      <div class="list-head__filters" role="group" aria-label="Filter by visibility">
        {#each [["all", "All"], ["party", "Party"], ["gm", "GM only"], ["owner", "Owner"], ["private", "Private"]] as f}
          <button class="btn btn--sm" class:is-active={filter === f[0]} aria-pressed={filter === f[0]} onclick={() => (filter = f[0] as any)}>{f[1]}</button>
        {/each}
      </div>
    </header>
    <ul class="notes">
      {#each listed as n (n.uid)}
        <li>
          <button class="note-row" class:is-selected={selectedUid === String(n.uid)} onclick={() => (selectedUid = String(n.uid), void loadPreview())}>
            <span class="note-row__type">{String((n as any).type ?? "")}</span>
            <span class="note-row__name">{String(n.name ?? n.uid)}</span>
            <span class="note-row__path mono">{String((n as any).path ?? "")}</span>
            <span class="note-row__vis"><VisibilityBadge visibility={vis(n)} defaulted={defaulted(n)} size="sm" /></span>
          </button>
        </li>
      {/each}
    </ul>
  </section>

  <section class="dash__note">
    {#if selected}
    <header class="note-head">
      <div class="stack">
        <h2 class="note-head__title">{String(selected.name ?? selected.uid)}</h2>
        <span class="note-head__path mono">{String((selected as any).path ?? "")}</span>
      </div>
      <div class="note-head__actions">
        <button class="btn btn--sm" class:is-active={appState.playerEye} aria-pressed={appState.playerEye} onclick={() => (appState.playerEye = !appState.playerEye)} title="Render through the projection, for a player">
          <Icon name="eye" size={13} /> {appState.playerEye ? "Player eye on" : "Player eye"}
        </button>
        <button class="btn btn--sm" onclick={openInEditor}><Icon name="pencil" size={13} /> Edit</button>
      </div>
    </header>
    {#if appState.playerEye}
      <div class="eye-banner" role="status"><Icon name="eye" size={14} /><span>Player-eye preview — the body the server sends. GM-only parts are gone, not hidden.</span></div>
    {/if}
    <div class="note-body">
      {#if strippedCount > 0}
        <div class="stripped-note" role="note"><Icon name="lock" size={14} /><span>{strippedCount} GM-only block(s). Text stripped from the projection.</span></div>
      {/if}
      <MarkdownView html={previewHtml} noteUid={String(selected.uid)} canReveal={true} onReveal={reveal} />
    </div>
    {#if appState.showInspector}
      <footer class="note-foot"><VisibilityInspector {decisions} /></footer>
    {/if}
    {:else}
      <p class="side-note">Select a note.</p>
    {/if}
  </section>
</section>

<style>
  .dash { display: grid; grid-template-columns: var(--sidebar-width) minmax(20rem, 1fr) minmax(24rem, 1.4fr); height: 100%; overflow: hidden; min-height: 60vh; }
  .dash__side { border-inline-end: 1px solid var(--border-subtle); background: var(--bg-surface-2); padding: var(--space-4); overflow-y: auto; }
  .side-block + .side-block { margin-block-start: var(--space-6); }
  .side-title { font-size: var(--text-2xs); font-weight: var(--weight-bold); letter-spacing: 0.08em; text-transform: uppercase; color: var(--fg-subtle); margin-block-end: var(--space-2); }
  .side-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 2px; }
  .side-item { display: flex; align-items: center; gap: var(--space-2); width: 100%; padding: var(--space-2); border: 1px solid transparent; border-radius: var(--radius-md); background: none; font-size: var(--text-sm); text-align: start; color: var(--fg-default); }
  button.side-item { cursor: pointer; }
  button.side-item:hover { background: var(--bg-surface-3); }
  .side-item.is-active { background: var(--bg-surface); border-color: var(--border-default); box-shadow: inset 3px 0 0 var(--accent); font-weight: var(--weight-medium); }
  .side-item--warn { color: var(--fg-muted); }
  .side-item__name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .side-count { margin-inline-start: auto; font-size: var(--text-xs); color: var(--fg-subtle); }
  .side-note { font-size: var(--text-xs); color: var(--fg-muted); margin-block-start: var(--space-2); }
  .counts { margin: 0; display: flex; flex-direction: column; gap: var(--space-2); }
  .counts__row { display: flex; align-items: center; justify-content: space-between; gap: var(--space-2); }
  .counts dt, .counts dd { margin: 0; }
  .counts dd { font-size: var(--text-xs); color: var(--fg-muted); }
  .dash__list { border-inline-end: 1px solid var(--border-subtle); overflow-y: auto; background: var(--bg-canvas); }
  .list-head { position: sticky; top: 0; z-index: var(--z-sticky); padding: var(--space-3) var(--space-4); background: var(--bg-canvas); border-block-end: 1px solid var(--border-subtle); }
  .list-head__title { font-size: var(--text-md); }
  .list-head__filters { display: flex; gap: var(--space-1); margin-block-start: var(--space-2); flex-wrap: wrap; }
  .notes { list-style: none; margin: 0; padding: var(--space-2); display: flex; flex-direction: column; gap: 2px; }
  .note-row { display: grid; grid-template-columns: 6.5rem 1fr auto; grid-template-areas: "type name vis" "type path vis"; align-items: center; gap: 0 var(--space-2); width: 100%; padding: var(--space-2) var(--space-3); border: 1px solid transparent; border-radius: var(--radius-md); background: none; text-align: start; cursor: pointer; }
  .note-row:hover { background: var(--bg-surface-2); }
  .note-row.is-selected { background: var(--bg-surface); border-color: var(--border-default); box-shadow: inset 3px 0 0 var(--accent); }
  .note-row__type { grid-area: type; font-size: var(--text-2xs); text-transform: uppercase; letter-spacing: 0.06em; color: var(--fg-subtle); font-weight: var(--weight-semibold); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .note-row__name { grid-area: name; font-size: var(--text-sm); font-weight: var(--weight-medium); }
  .note-row__path { grid-area: path; font-size: var(--text-2xs); color: var(--fg-subtle); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .note-row__vis { grid-area: vis; }
  .dash__note { display: flex; flex-direction: column; overflow: hidden; background: var(--bg-surface); }
  .note-head { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--space-4); padding: var(--space-4); border-block-end: 1px solid var(--border-subtle); }
  .note-head__title { font-size: var(--text-md); }
  .note-head__path { font-size: var(--text-2xs); color: var(--fg-subtle); }
  .note-head__actions { display: flex; gap: var(--space-2); flex: none; }
  .eye-banner { display: flex; align-items: center; gap: var(--space-2); padding: var(--space-2) var(--space-4); background: var(--accent-subtle); border-block-end: 1px solid var(--accent-border); font-size: var(--text-xs); }
  .note-body { flex: 1; overflow-y: auto; padding: var(--space-4); }
  .stripped-note { display: flex; align-items: center; gap: var(--space-2); margin-block-end: var(--space-4); padding: var(--space-2) var(--space-3); border: 2px dashed var(--secret-border); border-radius: var(--radius-md); background: var(--secret-subtle); font-size: var(--text-xs); }
  .note-foot { border-block-start: 1px solid var(--border-subtle); padding: var(--space-3) var(--space-4); }
</style>
