<script lang="ts">
  import { onMount } from "svelte";
  import { fetchNote, fetchNotes, saveNote, patchNote, type NoteDoc } from "../lib/api";
  import { WSClient } from "../lib/ws";
  import { getToken } from "../lib/auth";
  import Editor from "../lib/Editor.svelte";
  import VisibilityBadge from "../components/VisibilityBadge.svelte";
  import VisibilityInspector from "../components/VisibilityInspector.svelte";
  import MarkdownView from "../components/MarkdownView.svelte";
  import Icon from "../components/Icon.svelte";
  import { appState, findSecrets } from "../lib/appstate.svelte";

  let uid = $derived(appState.selectedUid);
  let doc = $state<NoteDoc | null>(null);
  let buffer = $state("");
  // The source pane is always the GM's raw file — editing a projected
  // (secret-stripped) body and saving it back would destroy secrets.
  // The eye affects the preview only.
  let previewSource = $state("");
  let status = $state("");
  let conflict = $state<NoteDoc | null>(null);
  let fmVisibility = $state(""), fmWorkflow = $state(""), fmGrants = $state(""),
      fmCampaign = $state(""), fmKind = $state(""), fmPlayer = $state(""),
      fmType = $state(""), fmName = $state("");
  let debounce: any = null;
  let ws: WSClient | null = null;

  const secrets = $derived(findSecrets(buffer));
  const effVis = $derived(doc?.effectiveVisibility ?? "");
  const isDefaulted = $derived(doc?.defaulted ?? false);
  const eyeAs = $derived(appState.playerEye && appState.eyeTarget ? appState.eyeTarget : "");

  async function open(id: string) {
    status = "loading"; conflict = null;
    try {
      doc = await fetchNote(id);
      buffer = doc.content;
      const rec = doc as any;
      fmType = rec.type ?? ""; fmName = doc.name ?? "";
      fmVisibility = rec.visibility ?? ""; fmWorkflow = rec.workflow ?? "";
      fmGrants = Array.isArray(rec.grants) ? rec.grants.join(", ") : String(rec.grants ?? "");
      fmCampaign = rec.campaign ?? ""; fmKind = rec.kind ?? ""; fmPlayer = rec.player ?? "";
      status = "loaded";
      if (eyeAs) await renderPreview();
      else previewSource = buffer;
    } catch (e: any) { status = e?.message ?? "open failed"; }
  }

  async function renderPreview() {
    if (!uid) { previewSource = ""; return; }
    try {
      // Preview through the eye when it is on; the server projects.
      const prev = await fetchNote(uid, eyeAs ? { as: eyeAs } : {});
      previewSource = prev.content;
    } catch { previewSource = ""; }
  }
  function onEdit(v: string) {
    buffer = v;
    clearTimeout(debounce);
    // Live keystrokes preview from the buffer in GM view. Under the eye
    // the preview is the saved file's projection (see the banner), so
    // typing does not refetch — saving refreshes it.
    if (!eyeAs) {
      debounce = setTimeout(() => { previewSource = buffer; }, 150);
    }
  }
  async function save() {
    if (!uid || !doc) return;
    status = "saving";
    try {
      const updated = await saveNote(uid, buffer, doc.hash);
      doc = updated; buffer = updated.content; status = "saved";
      await renderPreview();
    } catch (e: any) {
      if (e?.status === 409) {
        status = "conflict";
        conflict = await fetchNote(uid);
      } else status = e?.message ?? "save failed";
    }
  }
  async function applyFm() {
    if (!uid) return;
    const fields: Record<string, unknown> = {};
    if (fmType) fields.type = fmType;
    if (fmName) fields.name = fmName;
    if (fmVisibility) fields.visibility = fmVisibility;
    if (fmWorkflow) fields.workflow = fmWorkflow;
    if (fmGrants) fields.grants = fmGrants.split(",").map((s) => s.trim()).filter(Boolean);
    if (fmCampaign) fields.campaign = fmCampaign;
    if (fmKind) fields.kind = fmKind;
    if (fmPlayer) fields.player = fmPlayer;
    try { doc = await patchNote(uid, fields); status = "frontmatter applied"; await renderPreview(); }
    catch (e: any) { status = e?.message ?? "frontmatter failed"; }
  }
  function reveal(blockId: string) {
    if (!uid) return;
    ws?.revealSecret(uid, blockId, { all: true });
    setTimeout(() => void renderPreview(), 600);
  }

  $effect(() => { if (uid) void open(uid); });
  // The eye moves the preview, never the source.
  let lastEye = $state("");
  $effect(() => {
    const cur = `${appState.playerEye}|${appState.eyeTarget}`;
    if (cur !== lastEye) {
      lastEye = cur;
      if (eyeAs) void renderPreview();
      else if (buffer) previewSource = buffer;
    }
  });
  onMount(() => {
    if (!appState.selectedUid) {
      fetchNotes({} as any).then((items: any[]) => {
        if (items.length && !appState.selectedUid) appState.selectedUid = String(items[0].uid);
      }).catch(() => {});
    }
    ws = new WSClient({
      onNoteSaved: (d) => { if (d?.uid === uid) void open(uid!); },
      onSecretRevealed: () => { if (uid) void renderPreview(); },
      onVaultChanged: () => { if (uid) void open(uid!); },
    });
    ws.connect();
    return () => ws?.close();
  });
</script>

<div class="editor">
  <section class="pane pane--source">
    <header class="pane__head">
      <h2 class="pane__title"><Icon name="file" size={14} /> Source</h2>
      <span class="pane__hint mono">{buffer.length} bytes · {status}</span>
    </header>
    <div class="source-wrap">
      <Editor bind:value={buffer} onchange={onEdit} />
    </div>
    <div class="pane__foot"><button class="btn btn--sm" onclick={save}>Save</button></div>
    {#if conflict}
      <details open class="conflict">
        <summary>Conflict (409) — server changed underneath</summary>
        <button class="btn btn--sm" onclick={() => { if (conflict) { doc = conflict; buffer = conflict.content; conflict = null; status = "rebased"; void renderPreview(); } }}>Rebase to server version</button>
      </details>
    {/if}
  </section>

  <section class="pane pane--preview">
    <header class="pane__head">
      <h2 class="pane__title"><Icon name="eye" size={14} /> Preview</h2>
      <span class="pane__hint">{eyeAs ? `as ${eyeAs} · saved file` : "GM source"}</span>
    </header>
    {#if eyeAs}
      <div class="eye-note" role="status">Preview of the last <strong>save</strong> as {eyeAs} — save to refresh.</div>
    {/if}
    <div class="preview">
      <MarkdownView source={previewSource} noteUid={uid ?? ""} canReveal={false} />
    </div>
  </section>

  <section class="pane pane--meta">
    <header class="pane__head">
      <h2 class="pane__title"><Icon name="shield" size={14} /> Frontmatter</h2>
      <span class="pane__hint">reserved 12</span>
    </header>
    <div class="meta">
      {#if doc}
        {#if !(doc as any).campaign && !(fmCampaign)}
          <div class="warn" role="alert"><Icon name="warning" size={15} /><div><strong>No campaign.</strong> A note without a campaign defaults to <code>gm</code>.</div></div>
        {/if}
        <div class="meta__block">
          <h3 class="meta__title">Effective visibility</h3>
          <div class="meta__vis">
            {#if effVis}<VisibilityBadge visibility={effVis} defaulted={isDefaulted} />{/if}
            {#if isDefaulted}<span class="subtle meta__note">defaulted</span>{/if}
          </div>
          <VisibilityInspector decisions={[]} />
        </div>
        <div class="meta__block">
          <h3 class="meta__title">GM-only blocks <span class="tag">{secrets.length}</span></h3>
          {#if secrets.length === 0}
            <p class="meta__empty">None. No block marked GM-only.</p>
          {:else}
            <ul class="secrets">
              {#each secrets as s (s.id)}
                <li class="secret">
                  <span class="secret__label">{s.label}</span>
                  <span class="secret__id mono">^{s.id}</span>
                  <span class="secret__actions">
                    <button class="btn btn--sm" onclick={() => reveal(s.id)}>Reveal to the table</button>
                  </span>
                </li>
              {/each}
            </ul>
          {/if}
        </div>
        <details class="meta__block">
          <summary>Edit frontmatter</summary>
          <label>Type <input bind:value={fmType} /></label>
          <label>Name <input bind:value={fmName} /></label>
          <label>Visibility <input bind:value={fmVisibility} /></label>
          <label>Workflow <input bind:value={fmWorkflow} /></label>
          <label>Grants <input bind:value={fmGrants} /></label>
          <label>Campaign <input bind:value={fmCampaign} /></label>
          <label>Kind <input bind:value={fmKind} /></label>
          <label>Player <input bind:value={fmPlayer} /></label>
          <button class="btn btn--sm" onclick={applyFm}>Apply</button>
        </details>
      {:else}
        <p class="meta__empty">Select a note from the dashboard or search.</p>
      {/if}
    </div>
  </section>
</div>

<style>
  .editor { display: grid; grid-template-columns: minmax(18rem, 1fr) minmax(20rem, 1.2fr) minmax(17rem, 0.85fr); height: 100%; min-height: 60vh; overflow: hidden; }
  .pane { display: flex; flex-direction: column; overflow: hidden; min-width: 0; }
  .pane--source { border-inline-end: 1px solid var(--border-subtle); background: var(--bg-surface-2); }
  .pane--preview { border-inline-end: 1px solid var(--border-subtle); }
  .pane--meta { background: var(--bg-surface-2); overflow-y: auto; }
  .pane__head { display: flex; align-items: center; justify-content: space-between; gap: var(--space-2); padding: var(--space-2) var(--space-3); border-block-end: 1px solid var(--border-subtle); background: var(--bg-surface); }
  .pane__title { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-xs); font-weight: var(--weight-bold); letter-spacing: 0.06em; text-transform: uppercase; color: var(--fg-muted); }
  .pane__hint { font-size: var(--text-2xs); color: var(--fg-subtle); }
  .pane__foot { padding: var(--space-2) var(--space-3); border-block-start: 1px solid var(--border-subtle); }
  .source-wrap { flex: 1; overflow-y: auto; }
  .preview { flex: 1; overflow-y: auto; padding: var(--space-4); }
  .eye-note { padding: var(--space-2) var(--space-4); background: var(--accent-subtle); border-block-end: 1px solid var(--accent-border); font-size: var(--text-xs); }
  .meta { padding: var(--space-3); display: flex; flex-direction: column; gap: var(--space-4); }
  .meta__title { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-2xs); font-weight: var(--weight-bold); letter-spacing: 0.08em; text-transform: uppercase; color: var(--fg-subtle); }
  .meta__vis { display: flex; align-items: center; gap: var(--space-2); }
  .meta__empty { font-size: var(--text-xs); color: var(--fg-subtle); }
  .warn { display: flex; gap: var(--space-2); padding: var(--space-3); border: 2px solid var(--warn-border); border-radius: var(--radius-md); background: var(--warn-subtle); font-size: var(--text-xs); }
  .secrets { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: var(--space-2); }
  .secret { display: flex; flex-direction: column; gap: 2px; padding: var(--space-2); border: 1px dashed var(--secret-border); border-radius: var(--radius-md); background: var(--bg-surface); }
  .secret__label { font-size: var(--text-xs); font-weight: var(--weight-semibold); }
  .secret__id { font-size: var(--text-2xs); color: var(--fg-subtle); }
  .secret__actions { display: flex; gap: var(--space-1); }
  .conflict { padding: var(--space-2) var(--space-3); }
  label { display: flex; flex-direction: column; gap: 2px; font-size: var(--text-xs); margin-block: var(--space-1); }
  input { padding: var(--space-1) var(--space-2); border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--bg-surface); color: var(--fg-default); }
</style>
