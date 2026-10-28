<script lang="ts">
  // The viewer: pages open here, in reading layout. The vault explorer
  // lists; this reads. A two-column toggle maximises above-the-fold
  // prose; the floating button jumps to the editor. Reached by opening
  // any note — there is deliberately no sidebar link.
  import { fetchNote } from "../lib/api";
  import { vaultStore } from "../lib/vaultstore.svelte";
  import MarkdownView from "../components/MarkdownView.svelte";
  import NoteContext from "../components/NoteContext.svelte";
  import Icon from "../components/Icon.svelte";
  import { appState, findSecrets, revealedIds } from "../lib/appstate.svelte";
  import { getToken } from "../lib/auth";

  let source = $state("");
  let previewError = $state("");
  let twoCol = $state(false);
  let revealed: string[] = $state([]);

  const eyeAs = $derived(appState.playerEye && appState.eyeTarget ? appState.eyeTarget : "");
  const selected = $derived(vaultStore.notes.find((n) => String(n.uid) === appState.selectedUid) ?? null);
  const secrets = $derived(findSecrets(source));
  const wikiIndex = $derived(() => {
    const m = new Map<string, string>();
    for (const n of vaultStore.notes) if ((n as any).name) m.set(String((n as any).name), String(n.uid));
    return m;
  });

  async function loadPreview() {
    const uid = appState.selectedUid;
    if (!uid) { source = ""; previewError = ""; revealed = []; return; }
    try {
      const doc = await fetchNote(uid, eyeAs ? { as: eyeAs } : {});
      source = doc.content;
      revealed = revealedIds((doc as any).revealed);
      previewError = "";
    } catch (e: any) {
      source = "";
      revealed = [];
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

  $effect(() => { void appState.selectedUid; void eyeAs; void loadPreview(); });
  $effect(() => {
    appState.rail = railContent;
    return () => { if (appState.rail === railContent) appState.rail = null; };
  });
</script>

{#snippet railContent()}
  <NoteContext note={selected} {secrets} {revealed} onReveal={reveal} />
{/snippet}

<section class="viewer" aria-label="Note viewer">
  {#if selected}
    <header class="note-head">
      <div class="stack">
        <h2 class="note-head__title">{String(selected.name ?? selected.uid)}</h2>
        <span class="note-head__path mono">{String((selected as any).path ?? "")}</span>
      </div>
      <div class="note-head__actions">
        <button class="btn btn--sm" class:is-active={twoCol} aria-pressed={twoCol} onclick={() => (twoCol = !twoCol)} title="Two-column layout uses the full area">
          <Icon name="list" size={13} /> {twoCol ? "One column" : "Two columns"}
        </button>
      </div>
    </header>
    {#if appState.playerEye}
      <div class="eye-banner" role="status"><Icon name="eye" size={14} /><span>Rendered as <strong>{eyeAs || "…"}</strong> — what the server sends them.</span></div>
    {/if}
    <div class="note-body" class:cols={twoCol}>
      {#if previewError}
        <div class="absent" role="status"><Icon name="eye-off" size={22} /><h3>Not in this view</h3><p>{previewError}</p></div>
      {:else}
        <MarkdownView
          {source}
          noteUid={String(selected.uid)}
          canReveal={!eyeAs}
          onReveal={reveal}
          {revealed}
          resolveWiki={(name) => wikiIndex().get(name) ?? null}
          onOpenNote={(uid) => (appState.selectedUid = uid)}
        />
      {/if}
    </div>
    <button class="fab" onclick={() => appState.go("wiki", "editor")} title="Edit in editor" aria-label="Edit in editor">
      <Icon name="pencil" size={18} />
    </button>
  {:else}
    <div class="absent">
      <Icon name="eye" size={22} />
      <h3>No page open</h3>
      <p>Pick a page in the vault explorer.</p>
      <button class="btn btn--sm" onclick={() => appState.go("wiki", "explorer")}>Open the vault explorer</button>
    </div>
  {/if}
</section>

<style>
  .viewer { position: relative; display: flex; flex-direction: column; height: 100%; overflow: hidden; background: var(--bg-surface); }
  .note-head { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--space-4); padding: var(--space-4); border-block-end: 1px solid var(--border-subtle); }
  .note-head__title { font-size: var(--text-md); }
  .note-head__path { font-size: var(--text-2xs); color: var(--fg-subtle); }
  .note-head__actions { display: flex; gap: var(--space-2); flex: none; }
  .eye-banner { display: flex; align-items: center; gap: var(--space-2); padding: var(--space-2) var(--space-4); background: var(--accent-subtle); border-block-end: 1px solid var(--accent-border); font-size: var(--text-xs); }
  .note-body { flex: 1; overflow-y: auto; padding: var(--space-4); }
  .note-body.cols :global(.prose) { columns: 2; column-gap: var(--space-6); max-width: none; }
  .absent { display: flex; flex-direction: column; align-items: center; gap: var(--space-2); padding: var(--space-10) var(--space-4); text-align: center; color: var(--fg-muted); margin: auto; }
  .absent h3 { font-size: var(--text-base); color: var(--fg-default); }
  .absent p { font-size: var(--text-sm); max-width: 30rem; }
  .fab {
    position: absolute; inset-inline-end: var(--space-4); inset-block-end: var(--space-4);
    width: 3rem; height: 3rem; display: grid; place-items: center;
    border-radius: var(--radius-pill); border: 1px solid var(--accent-border);
    background: var(--accent); color: var(--fg-on-accent, #fff); cursor: pointer;
    box-shadow: var(--shadow-3);
  }
  .fab:hover { background: var(--accent-hover); }
</style>
