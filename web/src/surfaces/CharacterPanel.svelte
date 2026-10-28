<script lang="ts">
  // Character creator: the sheet form, rendered from the pack and
  // editable. The character list lives in the left sidebar; withheld
  // fields and the review queue live in the right rail.
  import { onMount } from "svelte";
  import { fetchCharacter, createCharacter, submitCharacter, reviewCharacter, patchNote } from "../lib/api";
  import { vaultStore } from "../lib/vaultstore.svelte";
  import { getPrincipal } from "../lib/auth";
  import { appState } from "../lib/appstate.svelte";
  import VisibilityBadge from "../components/VisibilityBadge.svelte";
  import VisibilityInspector from "../components/VisibilityInspector.svelte";
  import Icon from "../components/Icon.svelte";

  let detail = $state<any>(null);
  let status = $state("idle");
  let createName = $state("New Character");
  let editFields = $state<Record<string, string>>({});
  const principal = $derived(getPrincipal());
  const isGM = $derived(principal?.kind === "gm");
  const uid = $derived(appState.selectedCharacterUid);

  async function open(id: string) {
    status = "loading";
    try {
      detail = await fetchCharacter(id, true);
      const data = detail?.data ?? detail?.fields ?? {};
      editFields = Object.fromEntries(Object.entries(data).map(([k, v]) => [k, typeof v === "object" ? JSON.stringify(v) : String(v ?? "")]));
      status = "loaded";
    } catch (e: any) { status = e?.message ?? "failed"; detail = null; }
  }
  $effect(() => { if (uid) void open(uid); });
  $effect(() => {
    appState.rail = railContent;
    return () => { if (appState.rail === railContent) appState.rail = null; };
  });
  onMount(() => {
    appState.saveStatus = status;
    if (!uid && vaultStore.characters.length) {
      appState.selectedCharacterUid = String(vaultStore.characters[0].uid);
    }
  });
  $effect(() => { appState.saveStatus = `sheet: ${status}`; });

  async function create() {
    try {
      const j = await createCharacter({ name: createName, system: "dnd5e", kind: "pc" });
      status = "created";
      if (j.uid) appState.selectedCharacterUid = String(j.uid);
    } catch (e: any) { status = e?.message ?? "create failed"; }
  }
  async function saveFields() {
    if (!uid) return;
    const fields: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(editFields)) {
      try { fields[k] = JSON.parse(v); } catch { fields[k] = v; }
    }
    try {
      await patchNote(uid, { data: fields } as any);
      await open(uid);
      status = "saved";
    } catch (e: any) { status = e?.message ?? "save failed"; }
  }
</script>

{#snippet railContent()}
  <div class="rail-block">
    <h3 class="rail-title">Review</h3>
    {#if !isGM}
      <div class="meta-note"><Icon name="info" size={14} /><p>A player is never told what a sheet is hiding. The absence is the answer.</p></div>
    {:else}
      <p class="rail-empty">GM view. Withheld properties are simply absent from non-GM payloads — no counts leak.</p>
    {/if}
    {#if appState.showInspector}
      <div class="rail-gap"></div>
      <VisibilityInspector decisions={[]} />
    {/if}
  </div>
  {#if vaultStore.reviewQueue.length > 0}
    <div class="rail-block">
      <h3 class="rail-title">Review queue <span class="tag">{vaultStore.reviewQueue.length}</span></h3>
      <ul class="queue">
        {#each vaultStore.reviewQueue as c (c.uid)}
          <li>
            <button class="qitem" onclick={() => (appState.selectedCharacterUid = String(c.uid))}>
              <span>{String(c.name ?? c.uid)}</span><span class="tag">{String((c as any).workflow)}</span>
            </button>
          </li>
        {/each}
      </ul>
    </div>
  {/if}
{/snippet}

<section class="sheet" aria-label="Character creator">
  {#if detail}
    <header class="sheet__head">
      <div class="stack">
        <h2 class="sheet__title">{String(detail.name ?? detail.uid)}</h2>
        <span class="sheet__sub">workflow: {String(detail.workflow ?? "?")}</span>
      </div>
      <button class="btn btn--sm" onclick={create} title="New character">+ New</button>
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
        <button class="btn btn--sm" type="button" onclick={() => uid && submitCharacter(uid)}>Submit</button>
        {#if isGM}
          <button class="btn btn--sm" type="button" onclick={() => uid && reviewCharacter(uid, true)}>Approve</button>
          <button class="btn btn--sm" type="button" onclick={() => uid && reviewCharacter(uid, false)}>Reject</button>
        {/if}
      </div>
    </form>
  {:else}
    <div class="sheet__empty">
      <p>Select a character in the sidebar, or create one.</p>
      <div class="create">
        <input bind:value={createName} aria-label="New character name" />
        <button class="btn btn--sm" onclick={create}>New character</button>
      </div>
      <p class="mono">Status: {status}</p>
    </div>
  {/if}
</section>

<style>
  .sheet { display: flex; flex-direction: column; height: 100%; overflow: hidden; background: var(--bg-surface); }
  .sheet__head { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--space-4); padding: var(--space-4); border-block-end: 1px solid var(--border-subtle); }
  .sheet__title { font-size: var(--text-md); }
  .sheet__sub { font-size: var(--text-xs); color: var(--fg-muted); }
  .quick { display: grid; grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr)); gap: var(--space-3); margin: 0; padding: var(--space-4); border-block-end: 1px solid var(--border-subtle); }
  .quick__item { display: flex; flex-direction: column; gap: 2px; }
  .quick dt { font-size: var(--text-2xs); text-transform: uppercase; letter-spacing: 0.06em; color: var(--fg-subtle); }
  .quick dd { margin: 0; font-size: var(--text-md); font-weight: var(--weight-semibold); }
  .form { flex: 1; overflow-y: auto; padding: var(--space-4); display: grid; grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr)); gap: var(--space-3); align-content: start; }
  .field { display: flex; flex-direction: column; gap: 2px; }
  .field label { font-size: var(--text-xs); color: var(--fg-muted); }
  .field input { padding: var(--space-1) var(--space-2); border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--bg-surface); color: var(--fg-default); }
  .actions { grid-column: 1 / -1; display: flex; gap: var(--space-2); }
  .sheet__empty { padding: var(--space-6); display: flex; flex-direction: column; gap: var(--space-3); }
  .create { display: flex; gap: var(--space-1); }
  .create input { flex: 1; padding: var(--space-1) var(--space-2); border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--bg-surface); color: var(--fg-default); }
  .rail-block + .rail-block { margin-block-start: var(--space-6); }
  .rail-title { font-size: var(--text-2xs); font-weight: var(--weight-bold); letter-spacing: 0.08em; text-transform: uppercase; color: var(--fg-subtle); margin-block-end: var(--space-2); display: flex; align-items: center; gap: var(--space-2); }
  .rail-gap { height: var(--space-2); }
  .rail-empty { font-size: var(--text-xs); color: var(--fg-subtle); }
  .meta-note { display: flex; gap: var(--space-2); padding: var(--space-3); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--bg-surface); font-size: var(--text-xs); color: var(--fg-muted); }
  .queue { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 2px; }
  .qitem { display: flex; align-items: center; justify-content: space-between; gap: var(--space-2); width: 100%; padding: var(--space-2); border: 1px solid transparent; border-radius: var(--radius-md); background: none; font-size: var(--text-sm); cursor: pointer; color: var(--fg-default); }
  .qitem:hover { background: var(--bg-surface-3); }
</style>
