<script lang="ts">
  import Icon from "../components/Icon.svelte";
  import VisibilityBadge from "../components/VisibilityBadge.svelte";
  import VisibilityInspector from "../components/VisibilityInspector.svelte";
  import MarkdownView from "../components/MarkdownView.svelte";
  import { store } from "../lib/store.svelte";
  import { NOTES, PRINCIPALS } from "../lib/campaign";
  import type { Note } from "../lib/model";

  const eyePlayer = $derived(PRINCIPALS.find((p) => p.id === store.playerEyeId) ?? PRINCIPALS[1]);

  const effective = $derived(store.effectivePrincipal);

  /** The notes the GM may see, grouped. */
  const gmNotes = $derived(NOTES.filter(() => true));
  const visibleToEye = $derived(
    NOTES.filter((n) => store.projectionFor(n, eyePlayer).visible),
  );

  const selected = $derived(store.selectedNote);
  const projection = $derived(store.projectionFor(selected, effective));

  /** The notes that want the GM's attention. */
  const attention = $derived(
    NOTES.filter((n) => {
      if (n.workflow === "draft" || n.workflow === "submitted") return true;
      if (n.type === "handout" && n.visibility === "gm" && !n.grants?.length) return true;
      if (!n.campaign) return true;
      return false;
    }),
  );

  const counts = $derived({
    party: NOTES.filter((n) => (n.visibility ?? (n.campaign ? "party" : "gm")) === "party").length,
    gm: NOTES.filter((n) => (n.visibility ?? (n.campaign ? "party" : "gm")) === "gm").length,
    owner: NOTES.filter((n) => n.visibility === "owner").length,
    private: NOTES.filter((n) => n.visibility === "private").length,
  });

  let filter = $state<"all" | "party" | "gm" | "owner" | "private">("all");

  const listed = $derived(
    gmNotes.filter((n) => {
      if (filter === "all") return true;
      return (n.visibility ?? (n.campaign ? "party" : "gm")) === filter;
    }),
  );

  function effectiveVis(n: Note) {
    return n.visibility ?? (n.campaign ? "party" : "gm");
  }
</script>

<div class="dash">
  <!-- ─── the sidebar ─────────────────────────────────────────────── -->
  <aside class="dash__side">
    <div class="side-block">
      <h3 class="side-title">Campaigns</h3>
      <ul class="side-list">
        <li>
          <button class="side-item is-active">
            <Icon name="scroll" size={14} />
            <span>Stormdark</span>
            <span class="side-count">{NOTES.filter((n) => n.campaign === "stormdark").length}</span>
          </button>
        </li>
        <li>
          <button class="side-item">
            <Icon name="scroll" size={14} />
            <span>Emberfall</span>
            <span class="side-count">{NOTES.filter((n) => n.campaign === "emberfall").length}</span>
          </button>
        </li>
        <li>
          <button class="side-item">
            <Icon name="file" size={14} />
            <span>No campaign</span>
            <span class="side-count">{NOTES.filter((n) => !n.campaign).length}</span>
          </button>
        </li>
      </ul>
    </div>

    <div class="side-block">
      <h3 class="side-title">Needs attention</h3>
      <ul class="side-list">
        {#each attention as n (n.uid)}
          <li>
            <button
              class="side-item side-item--warn"
              onclick={() => (store.selectedNoteId = n.uid)}
            >
              <Icon name="warning" size={14} />
              <span class="side-item__name">{n.name}</span>
            </button>
          </li>
        {/each}
      </ul>
    </div>

    <div class="side-block">
      <h3 class="side-title">What the table can see</h3>
      <dl class="counts">
        <div class="counts__row">
          <dt><VisibilityBadge visibility="party" size="sm" /></dt>
          <dd>{visibleToEye.length} of {NOTES.length} notes</dd>
        </div>
        <div class="counts__row">
          <dt><VisibilityBadge visibility="gm" size="sm" /></dt>
          <dd>{NOTES.length - visibleToEye.length} hidden</dd>
        </div>
      </dl>
      <p class="side-note">
        A count, because a GM has to know the shape of what they are sharing.
        The <em>names</em> of the hidden notes stay in this panel — a player
        never sees this panel at all.
      </p>
    </div>
  </aside>

  <!-- ─── the note list ───────────────────────────────────────────── -->
  <section class="dash__list">
    <header class="list-head">
      <h2 class="list-head__title">Stormdark</h2>
      <div class="list-head__filters" role="group" aria-label="Filter by visibility">
        {#each [["all", "All"], ["party", "Party"], ["gm", "GM only"], ["owner", "Owner"], ["private", "Private"]] as const as f (f[0])}
          <button
            class="btn btn--sm"
            class:is-active={filter === f[0]}
            aria-pressed={filter === f[0]}
            onclick={() => (filter = f[0])}
          >
            {f[1]}
            <span class="filter-count">
              {f[0] === "all" ? NOTES.length : counts[f[0]]}
            </span>
          </button>
        {/each}
      </div>
    </header>

    <ul class="notes">
      {#each listed as n (n.uid)}
        <li>
          <button
            class="note-row"
            class:is-selected={store.selectedNoteId === n.uid}
            onclick={() => (store.selectedNoteId = n.uid)}
          >
            <span class="note-row__type">{n.type}</span>
            <span class="note-row__name">{n.name}</span>
            <span class="note-row__path mono">{n.path}</span>
            <span class="note-row__vis">
              <VisibilityBadge
                visibility={effectiveVis(n)}
                defaulted={n.visibility === undefined}
                size="sm"
              />
            </span>
            {#if n.workflow === "draft" || n.workflow === "submitted"}
              <span class="tag note-row__gate">
                <Icon name="lock" size={11} /> {n.workflow}
              </span>
            {/if}
          </button>
        </li>
      {/each}
    </ul>
  </section>

  <!-- ─── the note ────────────────────────────────────────────────── -->
  <section class="dash__note">
    <header class="note-head">
      <div class="stack">
        <h2 class="note-head__title">{selected.name}</h2>
        <span class="note-head__path mono">{selected.path}</span>
      </div>
      <div class="note-head__actions">
        <button
          class="btn btn--sm"
          data-testid="player-eye"
          class:is-active={store.playerEye}
          aria-pressed={store.playerEye}
          onclick={() => (store.playerEye = !store.playerEye)}
          title="Render this note through the projection, for a player"
        >
          <Icon name="eye" size={13} />
          {store.playerEye ? `Seeing as ${eyePlayer.name}` : "Player eye"}
        </button>
        <button class="btn btn--sm" onclick={() => (store.surface = "editor")}>
          <Icon name="pencil" size={13} /> Edit
        </button>
      </div>
    </header>

    {#if store.playerEye}
      <div class="eye-banner" role="status">
        <Icon name="eye" size={14} />
        <span>
          Rendered as <strong>{eyePlayer.name}</strong>. This is the body the
          server would send — the GM-only parts are gone, not hidden.
        </span>
      </div>
    {/if}

    <div class="note-body">
      {#if !projection.visible}
        <div class="absent">
          <Icon name="eye-off" size={22} />
          <h3>This note is not in {effective.name}'s view</h3>
          <p>
            It is not on the list, it is not in search, and it is not in the
            payload. The projection does not return a stub — a stub is a hint.
          </p>
        </div>
      {:else}
        {#if projection.strippedSecrets.length > 0}
          <div class="stripped-note" role="note">
            <Icon name="lock" size={14} />
            <span>
              {projection.strippedSecrets.length} GM-only block{projection.strippedSecrets.length === 1 ? "" : "s"}
              in this note. The text is stripped from the projection.
            </span>
          </div>
        {/if}
        <MarkdownView
          body={projection.body}
          variant={effective.kind === "gm" ? "full" : "full"}
          secretRenderer={(slot) => `
            <div class="secret-block" data-secret-id="${slot.id}">
              <div class="secret-block__head">
                <span class="secret-block__lock" aria-hidden="true">🔒</span>
                <span class="secret-block__label">${slot.label}</span>
                <span class="secret-block__id mono">^${slot.id}</span>
                <span class="secret-block__state">GM only — not sent to players</span>
              </div>
            </div>`}
        />
      {/if}
    </div>

    {#if store.showInspector}
      <footer class="note-foot">
        <VisibilityInspector decisions={projection.decisions} />
      </footer>
    {/if}
  </section>
</div>

<style>
  .dash {
    display: grid;
    grid-template-columns: var(--sidebar-width) minmax(20rem, 1fr) minmax(24rem, 1.4fr);
    height: 100%;
    overflow: hidden;
  }

  /* ─── sidebar ─── */
  .dash__side {
    border-inline-end: var(--border-width) solid var(--border-subtle);
    background: var(--bg-surface-2);
    padding: var(--space-4);
    overflow-y: auto;
  }
  .side-block + .side-block { margin-block-start: var(--space-6); }
  .side-title {
    font-size: var(--text-2xs);
    font-weight: var(--weight-bold);
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--fg-subtle);
    margin-block-end: var(--space-2);
  }
  .side-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 2px; }
  .side-item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-2);
    border: var(--border-width) solid transparent;
    border-radius: var(--radius-md);
    background: none;
    font-size: var(--text-sm);
    text-align: start;
    cursor: pointer;
    color: var(--fg-default);
  }
  .side-item:hover { background: var(--bg-surface-3); }
  .side-item.is-active {
    background: var(--bg-surface);
    border-color: var(--border-default);
    box-shadow: inset 3px 0 0 var(--accent);
    font-weight: var(--weight-medium);
  }
  .side-item--warn { color: var(--fg-muted); }
  .side-item--warn :global(svg) { color: var(--warn); }
  .side-item__name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .side-count { margin-inline-start: auto; font-size: var(--text-xs); color: var(--fg-subtle); font-variant-numeric: tabular-nums; }
  .side-note { font-size: var(--text-xs); color: var(--fg-muted); margin-block-start: var(--space-2); }

  .counts { margin: 0; display: flex; flex-direction: column; gap: var(--space-2); }
  .counts__row { display: flex; align-items: center; justify-content: space-between; gap: var(--space-2); }
  .counts dt, .counts dd { margin: 0; }
  .counts dd { font-size: var(--text-xs); color: var(--fg-muted); font-variant-numeric: tabular-nums; }

  /* ─── list ─── */
  .dash__list {
    border-inline-end: var(--border-width) solid var(--border-subtle);
    overflow-y: auto;
    background: var(--bg-canvas);
  }
  .list-head {
    position: sticky;
    top: 0;
    z-index: var(--z-sticky);
    padding: var(--space-3) var(--space-4);
    background: var(--bg-canvas);
    border-block-end: var(--border-width) solid var(--border-subtle);
  }
  .list-head__title { font-size: var(--text-md); }
  .list-head__filters { display: flex; gap: var(--space-1); margin-block-start: var(--space-2); flex-wrap: wrap; }
  .filter-count { font-variant-numeric: tabular-nums; color: var(--fg-subtle); font-size: var(--text-2xs); }

  .notes { list-style: none; margin: 0; padding: var(--space-2); display: flex; flex-direction: column; gap: 2px; }
  .note-row {
    display: grid;
    grid-template-columns: 6.5rem 1fr auto;
    grid-template-areas: "type name vis" "type path vis";
    align-items: center;
    gap: 0 var(--space-2);
    width: 100%;
    padding: var(--space-2) var(--space-3);
    border: var(--border-width) solid transparent;
    border-radius: var(--radius-md);
    background: none;
    text-align: start;
    cursor: pointer;
  }
  .note-row:hover { background: var(--bg-surface-2); }
  .note-row.is-selected {
    background: var(--bg-surface);
    border-color: var(--border-default);
    box-shadow: inset 3px 0 0 var(--accent);
  }
  .note-row__type {
    grid-area: type;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-2xs);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--fg-subtle);
    font-weight: var(--weight-semibold);
  }
  .note-row__name { grid-area: name; font-size: var(--text-sm); font-weight: var(--weight-medium); }
  .note-row__path { grid-area: path; font-size: var(--text-2xs); color: var(--fg-subtle); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .note-row__vis { grid-area: vis; }
  .note-row__gate { color: var(--warn); border-color: var(--warn-border); background: var(--warn-subtle); }

  /* ─── the note ─── */
  .dash__note { display: flex; flex-direction: column; overflow: hidden; background: var(--bg-surface); }
  .note-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-4);
    padding: var(--space-4);
    border-block-end: var(--border-width) solid var(--border-subtle);
  }
  .note-head__title { font-size: var(--text-md); }
  .note-head__path { font-size: var(--text-2xs); color: var(--fg-subtle); }
  .note-head__actions { display: flex; gap: var(--space-2); flex: none; }

  .eye-banner {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-4);
    background: var(--accent-subtle);
    border-block-end: var(--border-width) solid var(--accent-border);
    font-size: var(--text-xs);
    color: var(--fg-default);
  }
  .eye-banner :global(svg) { color: var(--accent); flex: none; }

  .note-body { flex: 1; overflow-y: auto; padding: var(--space-4); }

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
  .absent p { font-size: var(--text-sm); max-width: 30rem; }

  .stripped-note {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin-block-end: var(--space-4);
    padding: var(--space-2) var(--space-3);
    border: var(--border-width-strong) dashed var(--secret-border);
    border-radius: var(--radius-md);
    background: var(--secret-subtle);
    font-size: var(--text-xs);
  }
  .stripped-note :global(svg) { color: var(--secret); flex: none; }

  .note-foot { border-block-start: var(--border-width) solid var(--border-subtle); padding: var(--space-3) var(--space-4); }
</style>
