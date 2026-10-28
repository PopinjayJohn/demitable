<script lang="ts">
  import { onMount } from "svelte";
  import { fetchNotes, fetchCharacter, createCharacter, submitCharacter, reviewCharacter, patchNote, type NoteSummary } from "../lib/api";
  import { surfaceVisibility } from "../lib/dashboard";
  import { getPrincipal } from "../lib/auth";
  import VisibilityBadge from "../components/VisibilityBadge.svelte";
  import Icon from "../components/Icon.svelte";

  let characters = $state<NoteSummary[]>([]);
  let packs = $state<any[]>([]);
  let selectedUid = $state<string | null>(null);
  let detail = $state<any>(null);
  let status = $state("loading");
  let createName = $state("New Character");
  let editFields = $state<Record<string, string>>({});
  const principal = $derived(getPrincipal());
  const isGM = $derived(principal?.kind === "gm");

  async function load() {
    status = "loading";
    try {
      characters = await fetchNotes({ type: "character" });
      const { apiFetch } = await import("../lib/auth");
      const res = await apiFetch("/api/v1/packs");
      if (res.ok) {
        const j = await res.json();
        packs = Array.isArray(j) ? j : (j.items ?? j.packs ?? []);
      }
      if (!selectedUid && characters.length) await open(String(characters[0].uid));
      status = "loaded";
    } catch (e) { status = "failed"; }
  }
  async function open(uid: string) {
    selectedUid = uid;
    try {
      detail = await fetchCharacter(uid, true);
      const data = detail?.data ?? detail?.fields ?? {};
      editFields = Object.fromEntries(Object.entries(data).map(([k, v]) => [k, typeof v === "object" ? JSON.stringify(v) : String(v ?? "")]));
    } catch { detail = null; }
  }
  async function create() {
    try {
      const j = await createCharacter({ name: createName, system: "dnd5e", kind: "pc" });
      await load();
      if (j.uid) await open(j.uid);
    } catch (e: any) { status = e?.message ?? "create failed"; }
  }
  async function saveFields() {
    if (!selectedUid) return;
    const fields: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(editFields)) {
      try { fields[k] = JSON.parse(v); } catch { fields[k] = v; }
    }
    try {
      await patchNote(selectedUid, { data: fields } as any);
      await open(selectedUid);
      status = "saved";
    } catch (e: any) { status = e?.message ?? "save failed"; }
  }
  function vis(n: NoteSummary): string {
    try { return surfaceVisibility(n); } catch { return "gm"; }
  }
  onMount(load);
</script>

<section class="sheet" aria-label="Character panel">
  <aside class="sheet__side">
    <h2 class="side-title">Characters</h2>
    <div class="create">
      <input bind:value={createName} aria-label="New character name" />
      <button class="btn btn--sm" onclick={create}>New</button>
    </div>
    <ul class="chars">
      {#each characters as c (c.uid)}
        <li>
          <button class="char" class:is-selected={selectedUid === String(c.uid)} onclick={() => open(String(c.uid))}>
            <span class="char__name">{String(c.name ?? c.uid)}</span>
            <span class="char__meta"><VisibilityBadge visibility={vis(c)} size="sm" /></span>
          </button>
        </li>
      {/each}
    </ul>
    <p class="side-note">Sheet renders the pack's declared fields; the projection removes what a principal may not see.</p>
  </aside>

  <section class="sheet__main">
    {#if detail}
      <header class="sheet__head">
        <div class="stack">
          <h2 class="sheet__title">{String(detail.name ?? detail.uid)}</h2>
          <span class="sheet__sub">workflow: {String(detail.workflow ?? "?")}</span>
        </div>
      </header>
        {#if detail.computed}
          <dl class="quick">
            {#each Object.entries(detail.computed as Record<string, unknown>).slice(0, 8) as [k, v]}
              <div class="quick__item"><dt>{k}</dt><dd>{String(v ?? "—")}</dd></div>
            {/each}
          </dl>
        {/if}
        <form class="form" onsubmit={(e) => { e.preventDefault(); void saveFields(); }}>
          {#each Object.entries(editFields) as [k, v]}
            <div class="field">
              <label for={`f-${k}`}>{k}</label>
              <input id={`f-${k}`} bind:value={editFields[k]} />
            </div>
          {/each}
          <div class="actions">
            <button class="btn btn--sm" type="submit">Save fields</button>
            <button class="btn btn--sm" type="button" onclick={() => selectedUid && submitCharacter(selectedUid)}>Submit</button>
            {#if isGM}
              <button class="btn btn--sm" type="button" onclick={() => selectedUid && reviewCharacter(selectedUid, true)}>Approve</button>
              <button class="btn btn--sm" type="button" onclick={() => selectedUid && reviewCharacter(selectedUid, false)}>Reject</button>
            {/if}
          </div>
        </form>
      {:else}
        <p class="side-note">Select a character.</p>
      {/if}
  </section>

  <aside class="sheet__meta">
    <h2 class="side-title">Why</h2>
    {#if !isGM}
      <div class="meta-note"><Icon name="info" size={14} /><p>A player is never told what a sheet is hiding. The absence is the answer.</p></div>
    {:else}
      <p class="side-note">GM view. Withheld properties are simply absent from non-GM payloads — no counts leak.</p>
    {/if}
    <p class="side-note mono">Status: {status}</p>
  </aside>
</section>

<style>
  .sheet { display: grid; grid-template-columns: var(--sidebar-width) minmax(22rem, 1.4fr) minmax(17rem, 0.8fr); height: 100%; min-height: 60vh; overflow: hidden; }
  .sheet__side { border-inline-end: 1px solid var(--border-subtle); background: var(--bg-surface-2); padding: var(--space-4); overflow-y: auto; }
  .sheet__main { display: flex; flex-direction: column; overflow: hidden; background: var(--bg-surface); }
  .sheet__meta { border-inline-start: 1px solid var(--border-subtle); background: var(--bg-surface-2); padding: var(--space-4); overflow-y: auto; }
  .side-title { font-size: var(--text-2xs); font-weight: var(--weight-bold); letter-spacing: 0.08em; text-transform: uppercase; color: var(--fg-subtle); margin-block-end: var(--space-2); }
  .side-note { font-size: var(--text-xs); color: var(--fg-muted); }
  .create { display: flex; gap: var(--space-1); margin-block-end: var(--space-3); }
  .create input { flex: 1; min-width: 0; padding: var(--space-1) var(--space-2); border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--bg-surface); color: var(--fg-default); }
  .chars { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 2px; }
  .char { display: flex; flex-direction: column; gap: var(--space-1); width: 100%; padding: var(--space-2); border: 1px solid transparent; border-radius: var(--radius-md); background: none; text-align: start; cursor: pointer; }
  .char:hover { background: var(--bg-surface-3); }
  .char.is-selected { background: var(--bg-surface); border-color: var(--border-default); box-shadow: inset 3px 0 0 var(--accent); }
  .char__name { font-size: var(--text-sm); font-weight: var(--weight-medium); }
  .sheet__head { padding: var(--space-4); border-block-end: 1px solid var(--border-subtle); }
  .sheet__title { font-size: var(--text-md); }
  .sheet__sub { font-size: var(--text-xs); color: var(--fg-muted); }
  .quick { display: grid; grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr)); gap: var(--space-3); margin: 0; padding: var(--space-4); }
  .quick__item { display: flex; flex-direction: column; gap: 2px; }
  .quick dt { font-size: var(--text-2xs); text-transform: uppercase; letter-spacing: 0.06em; color: var(--fg-subtle); }
  .quick dd { margin: 0; font-size: var(--text-md); font-weight: var(--weight-semibold); }
  .form { flex: 1; overflow-y: auto; padding: var(--space-4); display: grid; grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr)); gap: var(--space-3); align-content: start; }
  .field { display: flex; flex-direction: column; gap: 2px; }
  .field label { font-size: var(--text-xs); color: var(--fg-muted); }
  .field input { padding: var(--space-1) var(--space-2); border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--bg-surface); color: var(--fg-default); }
  .actions { grid-column: 1 / -1; display: flex; gap: var(--space-2); }
  .meta-note { display: flex; gap: var(--space-2); padding: var(--space-3); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--bg-surface); font-size: var(--text-xs); color: var(--fg-muted); }
</style>
