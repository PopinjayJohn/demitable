<script lang="ts">
  import Icon from "../components/Icon.svelte";
  import VisibilityBadge from "../components/VisibilityBadge.svelte";
  import VisibilityInspector from "../components/VisibilityInspector.svelte";
  import MarkdownView from "../components/MarkdownView.svelte";
  import { store } from "../lib/store.svelte";
  import { PRINCIPALS } from "../lib/campaign";
  import { findSecrets } from "../lib/projection";
  import type { Visibility } from "../lib/model";

  const note = $derived(store.selectedNote);
  const eyePlayer = $derived(PRINCIPALS.find((p) => p.id === store.playerEyeId) ?? PRINCIPALS[1]);
  const effective = $derived(store.effectivePrincipal);

  /** The source pane is a local copy — the buffer is the truth, not the file. */
  let source = $state("");
  let lastUid = $state("");

  $effect(() => {
    if (note.uid !== lastUid) {
      lastUid = note.uid;
      source = note.body;
    }
  });

  /** The frontmatter, as the app would write it. */
  const FRONTMATTER_ORDER = [
    "type", "uid", "name", "system", "campaign", "kind", "player",
    "workflow", "visibility", "grants", "revealed", "discovered",
  ] as const;

  const frontmatter = $derived(() => {
    const n = note;
    const out: Record<string, string> = {};
    for (const k of FRONTMATTER_ORDER) {
      const v = (n as unknown as Record<string, unknown>)[k];
      if (v === undefined) continue;
      out[k] = Array.isArray(v) ? `[${v.join(", ")}]` : String(v);
    }
    return out;
  });

  const secrets = $derived(findSecrets(source, { calloutTypes: ["secret"], codeBlockKeys: ["ad-secret"], statblockGmKey: "gm" }));

  const effectiveVis = $derived<Visibility>(
    note.visibility ?? (note.campaign ? "party" : "gm"),
  );

  const projection = $derived(store.projectionFor(note, effective));
</script>

<div class="editor">
  <!-- ─── the source pane ─────────────────────────────────────────── -->
  <section class="pane pane--source">
    <header class="pane__head">
      <h2 class="pane__title"><Icon name="file" size={14} /> Source</h2>
      <span class="pane__hint mono">{source.length} bytes</span>
    </header>
    <textarea
      class="source"
      spellcheck="false"
      aria-label="The note's markdown source"
      bind:value={source}
    ></textarea>
  </section>

  <!-- ─── the preview ──────────────────────────────────────────────── -->
  <section class="pane pane--preview">
    <header class="pane__head">
      <h2 class="pane__title"><Icon name="eye" size={14} /> Preview</h2>
      <div class="pane__tools">
        <button
          class="btn btn--sm"
          data-testid="player-eye"
          class:is-active={store.playerEye}
          aria-pressed={store.playerEye}
          onclick={() => (store.playerEye = !store.playerEye)}
        >
          {store.playerEye ? `As ${eyePlayer.name}` : "As the GM"}
        </button>
      </div>
    </header>
    <div class="preview">
      {#if !projection.visible}
        <div class="absent">
          <Icon name="eye-off" size={22} />
          <h3>Not in {effective.name}'s view</h3>
          <p>The preview renders the projection, not the file. If the note is not visible to this principal, there is nothing to preview.</p>
        </div>
      {:else}
        <MarkdownView
          body={projection.body}
          secretRenderer={(slot) => `
            <div class="secret-block">
              <div class="secret-block__head">
                <span class="secret-block__lock" aria-hidden="true">◧</span>
                <span class="secret-block__label">${slot.label}</span>
                <span class="secret-block__id mono">^${slot.id}</span>
                <span class="secret-block__state">GM only</span>
              </div>
            </div>`}
        />
      {/if}
    </div>
  </section>

  <!-- ─── the frontmatter panel ────────────────────────────────────── -->
  <section class="pane pane--meta">
    <header class="pane__head">
      <h2 class="pane__title"><Icon name="shield" size={14} /> Frontmatter</h2>
      <span class="pane__hint">ADR-0040's reserved set</span>
    </header>

    <div class="meta">
      {#if !note.campaign}
        <div class="warn" role="alert">
          <Icon name="warning" size={15} />
          <div>
            <strong>No campaign.</strong>
            A note without a campaign defaults to <code>gm</code>. The
            scaffolder has to stamp one, or the note is invisible to the table.
          </div>
        </div>
      {/if}

      <div class="meta__block">
        <h3 class="meta__title">Reserved fields</h3>
        <dl class="fm">
          {#each Object.entries(frontmatter()) as [k, v] (k)}
            <div class="fm__row">
              <dt class="mono">{k}</dt>
              <dd>{v}</dd>
            </div>
          {/each}
          {#each FRONTMATTER_ORDER.filter((k) => !(k in frontmatter())) as k (k)}
            <div class="fm__row fm__row--absent">
              <dt class="mono">{k}</dt>
              <dd class="subtle">— not set</dd>
            </div>
          {/each}
        </dl>
      </div>

      <div class="meta__block">
        <h3 class="meta__title">Effective visibility</h3>
        <div class="meta__vis">
          <VisibilityBadge visibility={effectiveVis} defaulted={note.visibility === undefined} />
          {#if note.visibility === undefined}
            <span class="subtle meta__note">defaulted</span>
          {/if}
        </div>
        <VisibilityInspector decisions={projection.decisions} />
      </div>

      <div class="meta__block">
        <h3 class="meta__title">
          GM-only blocks
          <span class="tag">{secrets.length}</span>
        </h3>
        {#if secrets.length === 0}
          <p class="meta__empty">None. The note's body has no block marked GM-only.</p>
        {:else}
          <ul class="secrets">
            {#each secrets as s (s.id)}
              <li class="secret">
                <span class="secret__icon"><Icon name="lock" size={13} /></span>
                <span class="secret__label">{s.label}</span>
                <span class="secret__id mono">^{s.id}</span>
                <span class="secret__actions">
                  <button class="btn btn--sm">Reveal to the table</button>
                  <button class="btn btn--sm">Reveal to a player</button>
                </span>
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    </div>
  </section>
</div>

<style>
  .editor {
    display: grid;
    grid-template-columns: minmax(18rem, 1fr) minmax(20rem, 1.2fr) minmax(17rem, 0.85fr);
    height: 100%;
    overflow: hidden;
  }
  .pane { display: flex; flex-direction: column; overflow: hidden; min-width: 0; }
  .pane--source { border-inline-end: var(--border-width) solid var(--border-subtle); background: var(--bg-surface-2); }
  .pane--preview { border-inline-end: var(--border-width) solid var(--border-subtle); }
  .pane--meta { background: var(--bg-surface-2); overflow-y: auto; }

  .pane__head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    border-block-end: var(--border-width) solid var(--border-subtle);
    background: var(--bg-surface);
  }
  .pane__title {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-xs);
    font-weight: var(--weight-bold);
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--fg-muted);
  }
  .pane__hint { font-size: var(--text-2xs); color: var(--fg-subtle); }
  .pane__tools { display: flex; gap: var(--space-1); }

  .source {
    flex: 1;
    width: 100%;
    padding: var(--space-3);
    border: 0;
    resize: none;
    background: transparent;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    line-height: var(--leading-snug);
    color: var(--fg-default);
    tab-size: 2;
  }
  .source:focus { outline: none; }

  .preview { flex: 1; overflow-y: auto; padding: var(--space-4); }

  .meta { padding: var(--space-3); display: flex; flex-direction: column; gap: var(--space-4); }
  .meta__block { display: flex; flex-direction: column; gap: var(--space-2); }
  .meta__title {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-2xs);
    font-weight: var(--weight-bold);
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--fg-subtle);
  }
  .meta__vis { display: flex; align-items: center; gap: var(--space-2); }
  .meta__note { font-size: var(--text-2xs); }
  .meta__empty { font-size: var(--text-xs); color: var(--fg-subtle); }

  .warn {
    display: flex;
    gap: var(--space-2);
    padding: var(--space-3);
    border: var(--border-width-strong) solid var(--warn-border);
    border-radius: var(--radius-md);
    background: var(--warn-subtle);
    font-size: var(--text-xs);
  }
  .warn :global(svg) { flex: none; color: var(--warn); margin-block-start: 1px; }
  .warn code { font-family: var(--font-mono); background: var(--bg-surface); padding: 0 0.2em; border-radius: 2px; }

  .fm { margin: 0; border: var(--border-width) solid var(--border-subtle); border-radius: var(--radius-md); overflow: hidden; background: var(--bg-surface); }
  .fm__row { display: grid; grid-template-columns: 5rem 1fr; gap: var(--space-2); padding: var(--space-1) var(--space-2); border-block-start: var(--border-width) solid var(--border-subtle); }
  .fm__row:first-child { border-block-start: 0; }
  .fm__row dt { font-size: var(--text-2xs); color: var(--fg-subtle); }
  .fm__row dd { margin: 0; font-size: var(--text-xs); overflow-wrap: anywhere; }
  .fm__row--absent dd { font-style: italic; }

  .secrets { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: var(--space-2); }
  .secret {
    display: grid;
    grid-template-columns: auto 1fr auto;
    gap: var(--space-1) var(--space-2);
    align-items: center;
    padding: var(--space-2);
    border: var(--border-width) dashed var(--secret-border);
    border-radius: var(--radius-md);
    background: var(--secret-subtle);
  }
  .secret__icon { color: var(--secret); display: inline-flex; }
  .secret__label { font-size: var(--text-xs); font-weight: var(--weight-medium); }
  .secret__id { font-size: var(--text-2xs); color: var(--fg-subtle); grid-column: 2; }
  .secret__actions { grid-column: 1 / -1; display: flex; gap: var(--space-1); }

  .absent {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-10) var(--space-4);
    text-align: center;
    color: var(--fg-muted);
  }
  .absent :global(svg) { color: var(--fg-subtle); }
  .absent h3 { font-size: var(--text-base); color: var(--fg-default); }
  .absent p { font-size: var(--text-sm); max-width: 26rem; }
</style>
