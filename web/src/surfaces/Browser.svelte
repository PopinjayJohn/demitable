<script lang="ts">
  // Wiki browser: the note list plus the reading pane. Campaign context
  // lives in the left sidebar now; note context (badge, inspector,
  // secrets, reveal) is published to the right rail.
  import { fetchNote } from "../lib/api";
  import { vaultStore } from "../lib/vaultstore.svelte";
  import { surfaceVisibility } from "../lib/dashboard";
  import VisibilityBadge from "../components/VisibilityBadge.svelte";
  import VisibilityInspector from "../components/VisibilityInspector.svelte";
  import MarkdownView from "../components/MarkdownView.svelte";
  import Icon from "../components/Icon.svelte";
  import { appState, findSecrets } from "../lib/appstate.svelte";
  import { getToken } from "../lib/auth";
  import type { NoteSummary } from "../lib/api";

  let filter = $state<"all" | "party" | "gm" | "owner" | "private">("all");
  let typeFilter = $state("");
  let source = $state("");
  let previewError = $state("");

  const eyeAs = $derived(appState.playerEye && appState.eyeTarget ? appState.eyeTarget : "");

  function vis(n: NoteSummary): string {
    try { return surfaceVisibility(n); } catch { return "gm"; }
  }
  function defaulted(n: NoteSummary): boolean {
    return (n as any).defaulted ?? (n as any).visibility === undefined;
  }

  const listed = $derived(vaultStore.notes.filter((n) => {
    if (filter !== "all" && vis(n) !== filter) return false;
    if (typeFilter && String((n as any).type ?? "") !== typeFilter) return false;
    return true;
  }));
  const selected = $derived(vaultStore.notes.find((n) => String(n.uid) === appState.selectedUid) ?? null);
  const secrets = $derived(findSecrets(source));
  const wikiIndex = $derived(() => {
    const m = new Map<string, string>();
    for (const n of vaultStore.notes) if ((n as any).name) m.set(String((n as any).name), String(n.uid));
    return m;
  });

  async function loadPreview() {
    const uid = appState.selectedUid;
    if (!uid) { source = ""; previewError = ""; return; }
    try {
      const doc = await fetchNote(uid, eyeAs ? { as: eyeAs } : {});
      source = doc.content;
      previewError = "";
    } catch (e: any) {
      source = "";
      previewError = e?.status === 404 && eyeAs
        ? `No account '${eyeAs}' holds this note — or the account does not exist yet.`
        : (e?.message ?? "preview failed");
    }
  }

  function reveal(blockId: string) {
    const uid = appState.selectedUid;
    if (!uid) return;
    const t = getToken();
    if (!t) return;
    const proto = location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(`${proto}://${location.host}/ws?token=${encodeURIComponent(t)}`);
    ws.onopen = () => {
      ws.send(JSON.stringify({ v: 1, type: "secret.reveal", data: { note_uid: uid, block_id: blockId, audience: { all: true } } }));
      setTimeout(() => ws.close(), 500);
    };
    ws.onmessage = () => loadPreview();
  }

  $effect(() => {
    if (!appState.selectedUid && vaultStore.notes.length) {
      appState.selectedUid = String(vaultStore.notes[0].uid);
    }
  });
  $effect(() => { void appState.selectedUid; void eyeAs; void loadPreview(); });

  // Publish this view's context to the right rail; clear it on unmount
  // so a rail-less view never inherits another view's context.
  $effect(() => {
    appState.rail = railContent;
    return () => { if (appState.rail === railContent) appState.rail = null; };
  });
</script>

{#snippet railContent()}
  {#if selected}
    <div class="rail-block">
      <h3 class="rail-title">Visibility</h3>
      <VisibilityBadge visibility={vis(selected)} defaulted={defaulted(selected)} />
      {#if appState.showInspector}
        <div class="rail-gap"></div>
        <VisibilityInspector decisions={[]} />
      {/if}
    </div>
    <div class="rail-block">
      <h3 class="rail-title">GM-only blocks <span class="tag">{secrets.length}</span></h3>
      {#if secrets.length === 0}
        <p class="rail-empty">None. No block marked GM-only.</p>
      {:else}
        <ul class="secrets">
          {#each secrets as s (s.id)}
            <li class="secret">
              <span class="secret__label">{s.label}</span>
              <span class="secret__id mono">^{s.id}</span>
              <button class="btn btn--sm" onclick={() => reveal(s.id)}>Reveal to the table</button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
    <div class="rail-block">
      <button class="btn btn--sm" onclick={() => appState.go("wiki", "editor")}>
        <Icon name="pencil" size={13} /> Open in editor
      </button>
    </div>
  {:else}
    <p class="rail-empty">Select a note.</p>
  {/if}
{/snippet}

<section class="browser" aria-label="Note browser">
  <section class="browser__list">
    <header class="list-head">
      <h2 class="list-head__title">Notes{#if eyeAs} <span class="tag">as {eyeAs}</span>{/if}</h2>
      <div class="list-head__filters" role="group" aria-label="Filter by visibility">
        <input class="type-filter" placeholder="type" bind:value={typeFilter} aria-label="Type filter" />
        {#each [["all", "All"], ["party", "Party"], ["gm", "GM only"], ["owner", "Owner"], ["private", "Private"]] as f}
          <button class="btn btn--sm" class:is-active={filter === f[0]} aria-pressed={filter === f[0]} onclick={() => (filter = f[0] as any)}>{f[1]}</button>
        {/each}
      </div>
    </header>
    <ul class="notes">
      {#each listed as n (n.uid)}
        <li>
          <button class="note-row" class:is-selected={appState.selectedUid === String(n.uid)} onclick={() => (appState.selectedUid = String(n.uid))}>
            <span class="note-row__type">{String((n as any).type ?? "")}</span>
            <span class="note-row__name">{String(n.name ?? n.uid)}</span>
            <span class="note-row__path mono">{String((n as any).path ?? "")}</span>
            <span class="note-row__vis"><VisibilityBadge visibility={vis(n)} defaulted={defaulted(n)} size="sm" /></span>
          </button>
        </li>
      {/each}
    </ul>
  </section>

  <section class="browser__note">
    {#if selected}
      <header class="note-head">
        <div class="stack">
          <h2 class="note-head__title">{String(selected.name ?? selected.uid)}</h2>
          <span class="note-head__path mono">{String((selected as any).path ?? "")}</span>
        </div>
      </header>
      {#if appState.playerEye}
        <div class="eye-banner" role="status"><Icon name="eye" size={14} /><span>Rendered as <strong>{eyeAs || "…"}</strong> — what the server sends them.</span></div>
      {/if}
      <div class="note-body">
        {#if previewError}
          <div class="absent" role="status"><Icon name="eye-off" size={22} /><h3>Not in this view</h3><p>{previewError}</p></div>
        {:else}
          <MarkdownView
            {source}
            noteUid={String(selected.uid)}
            canReveal={!eyeAs}
            onReveal={reveal}
            resolveWiki={(name) => wikiIndex().get(name) ?? null}
            onOpenNote={(uid) => (appState.selectedUid = uid)}
          />
        {/if}
      </div>
    {:else}
      <p class="side-note">Select a note.</p>
    {/if}
  </section>
</section>

<style>
  .browser { display: grid; grid-template-columns: minmax(20rem, 1fr) minmax(24rem, 1.4fr); height: 100%; overflow: hidden; }
  .browser__list { border-inline-end: 1px solid var(--border-subtle); overflow-y: auto; background: var(--bg-canvas); }
  .list-head { position: sticky; top: 0; z-index: var(--z-sticky); padding: var(--space-3) var(--space-4); background: var(--bg-canvas); border-block-end: 1px solid var(--border-subtle); }
  .list-head__title { font-size: var(--text-md); }
  .list-head__filters { display: flex; gap: var(--space-1); margin-block-start: var(--space-2); flex-wrap: wrap; align-items: center; }
  .type-filter { padding: var(--space-1) var(--space-2); border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--bg-surface); color: var(--fg-default); font-size: var(--text-xs); width: 7rem; }
  .notes { list-style: none; margin: 0; padding: var(--space-2); display: flex; flex-direction: column; gap: 2px; }
  .note-row { display: grid; grid-template-columns: 6.5rem 1fr auto; grid-template-areas: "type name vis" "type path vis"; align-items: center; gap: 0 var(--space-2); width: 100%; padding: var(--space-2) var(--space-3); border: 1px solid transparent; border-radius: var(--radius-md); background: none; text-align: start; cursor: pointer; }
  .note-row:hover { background: var(--bg-surface-2); }
  .note-row.is-selected { background: var(--bg-surface); border-color: var(--border-default); box-shadow: inset 3px 0 0 var(--accent); }
  .note-row__type { grid-area: type; font-size: var(--text-2xs); text-transform: uppercase; letter-spacing: 0.06em; color: var(--fg-subtle); font-weight: var(--weight-semibold); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .note-row__name { grid-area: name; font-size: var(--text-sm); font-weight: var(--weight-medium); }
  .note-row__path { grid-area: path; font-size: var(--text-2xs); color: var(--fg-subtle); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .note-row__vis { grid-area: vis; }
  .browser__note { display: flex; flex-direction: column; overflow: hidden; background: var(--bg-surface); }
  .note-head { padding: var(--space-4); border-block-end: 1px solid var(--border-subtle); }
  .note-head__title { font-size: var(--text-md); }
  .note-head__path { font-size: var(--text-2xs); color: var(--fg-subtle); }
  .eye-banner { display: flex; align-items: center; gap: var(--space-2); padding: var(--space-2) var(--space-4); background: var(--accent-subtle); border-block-end: 1px solid var(--accent-border); font-size: var(--text-xs); }
  .note-body { flex: 1; overflow-y: auto; padding: var(--space-4); }
  .absent { display: flex; flex-direction: column; align-items: center; gap: var(--space-2); padding: var(--space-10) var(--space-4); text-align: center; color: var(--fg-muted); }
  .absent h3 { font-size: var(--text-base); color: var(--fg-default); }
  .absent p { font-size: var(--text-sm); max-width: 30rem; }
  .side-note { font-size: var(--text-xs); color: var(--fg-muted); padding: var(--space-4); }
  .rail-block + .rail-block { margin-block-start: var(--space-6); }
  .rail-title { font-size: var(--text-2xs); font-weight: var(--weight-bold); letter-spacing: 0.08em; text-transform: uppercase; color: var(--fg-subtle); margin-block-end: var(--space-2); display: flex; align-items: center; gap: var(--space-2); }
  .rail-gap { height: var(--space-2); }
  .rail-empty { font-size: var(--text-xs); color: var(--fg-subtle); }
  .secrets { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: var(--space-2); }
  .secret { display: flex; flex-direction: column; gap: 2px; padding: var(--space-2); border: 1px dashed var(--secret-border); border-radius: var(--radius-md); background: var(--bg-surface); }
  .secret__label { font-size: var(--text-xs); font-weight: var(--weight-semibold); }
  .secret__id { font-size: var(--text-2xs); color: var(--fg-subtle); }
</style>
