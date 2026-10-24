<script lang="ts">
  import { onMount } from "svelte";
  import { fetchNotes, fetchNote, saveNote, patchNote, type NoteSummary, type NoteDoc } from "./lib/api";
  import Editor from "./lib/Editor.svelte";

  let notes = $state<NoteSummary[]>([]);
  let selectedUid = $state<string | null>(null);
  let doc = $state<NoteDoc | null>(null);
  let buffer = $state<string>("");
  let status = $state<string>("");
  let preview = $state<string>("");

  // frontmatter panel fields
  let fmType = $state<string>("");
  let fmName = $state<string>("");

  onMount(async () => {
    try {
      notes = await fetchNotes();
    } catch (e) {
      status = "load failed";
    }
  });

  async function openNote(uid: string) {
    selectedUid = uid;
    status = "loading";
    try {
      doc = await fetchNote(uid);
      buffer = doc.content;
      fmType = "";
      fmName = doc.name ?? "";
      status = "loaded";
      await renderPreview();
    } catch (e) {
      status = "open failed";
    }
  }

  async function renderPreview() {
    try {
      const { extractMarkdownAsync } = await import("./lib/conformance");
      const extracted = await extractMarkdownAsync(buffer);
      preview = extracted.text.slice(0, 500);
    } catch {
      preview = "";
    }
  }

  async function save() {
    if (!selectedUid || !doc) return;
    status = "saving";
    try {
      const updated = await saveNote(selectedUid, buffer, doc.hash);
      doc = updated;
      buffer = updated.content;
      status = "saved";
    } catch (e) {
      status = "save failed";
    }
  }

  async function applyFrontmatter() {
    if (!selectedUid) return;
    const fields: Record<string, unknown> = {};
    if (fmType) fields.type = fmType;
    if (fmName) fields.name = fmName;
    try {
      const updated = await patchNote(selectedUid, fields);
      doc = updated;
      buffer = updated.content;
      status = "frontmatter applied";
      await renderPreview();
    } catch (e) {
      status = "frontmatter failed";
    }
  }

  function onBufferChange(v: string) {
    buffer = v;
    void renderPreview();
  }
</script>

<main>
  <h1>Demitable</h1>
  <p role="status">Status: {status}</p>

  <div class="layout">
    <aside>
      <h2>Notes</h2>
      <ul>
        {#each notes as n}
          <li>
            <button type="button" onclick={() => openNote(String(n.uid))}>
              {n.name ?? n.uid}
            </button>
          </li>
        {/each}
      </ul>
    </aside>

    <section>
      {#if doc}
        <h2>{doc.name}</h2>
        <Editor value={buffer} onchange={onBufferChange} />
        <div class="actions">
          <button type="button" onclick={save}>Save</button>
        </div>

        <details>
          <summary>Frontmatter panel</summary>
          <label>Type <input bind:value={fmType} /></label>
          <label>Name <input bind:value={fmName} /></label>
          <button type="button" onclick={applyFrontmatter}>Apply</button>
        </details>

        <details>
          <summary>Live preview (extracted text)</summary>
          <pre>{preview}</pre>
        </details>
      {:else}
        <p>Select a note.</p>
      {/if}
    </section>
  </div>
</main>

<style>
  main {
    font-family: var(--font-sans, system-ui);
    color: var(--fg-default);
    background: var(--bg-canvas);
    padding: var(--space-4);
    min-height: 100vh;
  }
  .layout {
    display: grid;
    grid-template-columns: 240px 1fr;
    gap: var(--space-4);
  }
  aside ul { list-style: none; padding: 0; }
  aside button { width: 100%; text-align: left; }
  .actions { margin: var(--space-2) 0; }
  input { margin-left: var(--space-2); }
  pre { white-space: pre-wrap; }
</style>
