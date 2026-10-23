<script lang="ts">
  import Icon from "../components/Icon.svelte";
  import VisibilityBadge from "../components/VisibilityBadge.svelte";
  import VisibilityInspector from "../components/VisibilityInspector.svelte";
  import { store } from "../lib/store.svelte";
  import { NOTES } from "../lib/campaign";
  import { SHEET_GROUPS, SHEET_FIELDS, PACK_NAME } from "../lib/pack";
  
  const characters = $derived(
    NOTES.filter((n) => n.type === "character"),
  );

  const note = $derived(store.selectedCharacter);
  const viewer = $derived(store.principal);
  const isGm = $derived(viewer.kind === "gm");

  const sheet = $derived(store.sheetFor(note, viewer));
  const projection = $derived(sheet.projection);

  /** The groups, with only the fields this principal may see. */
  const groups = $derived(
    SHEET_GROUPS.map((g) => ({
      ...g,
      rows: sheet.fields.filter((f) => f.field.group === g.id && f.visible),
    })).filter((g) => g.rows.length > 0),
  );

  /** The fields the principal may not see — the GM's side panel only. */
  const withheld = $derived(
    sheet.fields.filter((f) => !f.visible),
  );

  function maskedValue(key: string, value: unknown): { text: string; masked: boolean } {
    const field = SHEET_FIELDS.find((f) => f.key === key);
    const mask = field?.visibility?.display?.[viewer.id];
    if (mask !== undefined) return { text: mask, masked: true };
    if (Array.isArray(value)) return { text: value.join(", "), masked: false };
    if (typeof value === "boolean") return { text: value ? "Yes" : "No", masked: false };
    return { text: value === undefined ? "" : String(value), masked: false };
  }

  function inputType(type: string): string {
    return type === "number" ? "number" : type === "longtext" ? "textarea" : "text";
  }
</script>

<div class="sheet">
  <!-- ─── the character list ──────────────────────────────────────── -->
  <aside class="sheet__side">
    <h2 class="side-title">Characters</h2>
    <ul class="chars">
      {#each characters as c (c.uid)}
        <li>
          <button
            class="char"
            class:is-selected={c.uid === store.selectedCharacterId}
            onclick={() => (store.selectedCharacterId = c.uid)}
          >
            <span class="char__name">{c.name}</span>
            <span class="char__meta">
              <span class="tag">{c.kind ?? "npc"}</span>
              {#if c.player}
                <span class="tag">{c.player}</span>
              {/if}
            </span>
          </button>
        </li>
      {/each}
    </ul>

    <div class="side-block">
      <h2 class="side-title">The pack</h2>
      <p class="pack-name">{PACK_NAME}</p>
      <p class="side-note">
        The sheet is a <strong>form</strong>, rendered from the pack's
        <code>views.sheet</code>. The same field list for every principal; the
        projection removes what a principal may not see.
      </p>
    </div>
  </aside>

  <!-- ─── the sheet ───────────────────────────────────────────────── -->
  <section class="sheet__main">
    <header class="sheet__head">
      <div class="stack">
        <h2 class="sheet__title">{note.name}</h2>
        <span class="sheet__sub">
          {note.kind === "pc" ? "Player character" : "Non-player character"}
          {#if note.player}· owned by {note.player}{/if}
        </span>
      </div>
      <div class="sheet__head-right">
        <VisibilityBadge
          visibility={note.visibility ?? (note.campaign ? "party" : "gm")}
          defaulted={note.visibility === undefined}
        />
        <span class="tag">{note.type}</span>
      </div>
    </header>

    {#if !projection.visible}
      <div class="absent">
        <Icon name="eye-off" size={22} />
        <h3>This sheet is not in {viewer.name}'s view</h3>
        <p>
          The character note itself is withheld, so the sheet is not rendered at
          all — not blanked, not stubbed.
        </p>
      </div>
    {:else}
      <form class="form" onsubmit={(e) => e.preventDefault()}>
        {#each groups as g (g.id)}
          <fieldset class="group">
            <legend class="group__legend">{g.label}</legend>
            <div class="group__grid">
              {#each g.rows as row (row.field.key)}
                {@const v = maskedValue(row.field.key, row.value)}
                <div class="field" class:field--wide={row.field.type === "longtext"}>
                  <label for={`f-${row.field.key}`}>
                    {row.field.label}
                    {#if v.masked}
                      <!-- The mask is announced, not hidden. -->
                      <span class="mask" title="Masked for this audience by the pack's `display` rule">masked</span>
                    {/if}
                  </label>
                  {#if row.field.type === "longtext"}
                    <textarea id={`f-${row.field.key}`} rows="3" readonly>{v.text}</textarea>
                  {:else}
                    <input
                      id={`f-${row.field.key}`}
                      type={inputType(row.field.type)}
                      value={v.text}
                      readonly
                    />
                  {/if}
                </div>
              {/each}
            </div>
          </fieldset>
        {/each}
      </form>
    {/if}
  </section>

  <!-- ─── the withheld-fields panel ───────────────────────────────── -->
  <aside class="sheet__meta">
    <h2 class="side-title">
      What {viewer.name} may not see
    </h2>

    {#if !isGm}
      <div class="meta-note">
        <Icon name="info" size={14} />
        <p>
          A player is never told what a sheet is hiding. The absence is the
          answer. Showing a count, or a "3 hidden fields" line, is a leak of
          exactly the thing the projection is hiding.
        </p>
      </div>
    {:else}
      <ul class="withheld">
        {#each withheld as row (row.field.key)}
          {@const prop = note.properties.find((p) => p.key === row.field.key)}
          {@const reason = projection.deniedProperties.find((d) => d.key === row.field.key)?.reason}
          <li class="withheld__item">
            <span class="withheld__label">{row.field.label}</span>
            <span class="withheld__reason">{reason ?? "Not visible to this principal."}</span>
            <span class="withheld__value mono">{String(prop?.value ?? "").slice(0, 60)}</span>
          </li>
        {:else}
          <li class="withheld__empty">Nothing is withheld from {viewer.name}.</li>
        {/each}
      </ul>

      <div class="meta-block">
        <h3 class="side-title">Why</h3>
        <VisibilityInspector decisions={projection.decisions} />
      </div>

      <div class="meta-block">
        <h3 class="side-title">The field's own rule</h3>
        {#each SHEET_FIELDS.filter((f) => f.visibility) as f (f.key)}
          <div class="field-rule">
            <span class="field-rule__label">{f.label}</span>
            <VisibilityBadge
              visibility={f.visibility?.visibility ?? "party"}
              size="sm"
            />
            {#if f.visibility?.display}
              <span class="field-rule__mask">
                masked for {Object.keys(f.visibility.display).join(", ")}
              </span>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </aside>
</div>

<style>
  .sheet {
    display: grid;
    grid-template-columns: var(--sidebar-width) minmax(22rem, 1.4fr) minmax(17rem, 0.8fr);
    height: 100%;
    overflow: hidden;
  }
  .sheet__side {
    border-inline-end: var(--border-width) solid var(--border-subtle);
    background: var(--bg-surface-2);
    padding: var(--space-4);
    overflow-y: auto;
  }
  .sheet__main { display: flex; flex-direction: column; overflow: hidden; }
  .sheet__meta {
    border-inline-start: var(--border-width) solid var(--border-subtle);
    background: var(--bg-surface-2);
    padding: var(--space-4);
    overflow-y: auto;
  }

  .side-title {
    font-size: var(--text-2xs);
    font-weight: var(--weight-bold);
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--fg-subtle);
    margin-block-end: var(--space-2);
  }
  .side-block { margin-block-start: var(--space-6); }
  .side-note { font-size: var(--text-xs); color: var(--fg-muted); }
  .side-note code { font-family: var(--font-mono); font-size: 0.9em; }
  .pack-name { font-size: var(--text-sm); font-weight: var(--weight-medium); margin-block-end: var(--space-2); }

  .chars { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 2px; }
  .char {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    width: 100%;
    padding: var(--space-2);
    border: var(--border-width) solid transparent;
    border-radius: var(--radius-md);
    background: none;
    text-align: start;
    cursor: pointer;
  }
  .char:hover { background: var(--bg-surface-3); }
  .char.is-selected {
    background: var(--bg-surface);
    border-color: var(--border-default);
    box-shadow: inset 3px 0 0 var(--accent);
  }
  .char__name { font-size: var(--text-sm); font-weight: var(--weight-medium); }
  .char__meta { display: flex; gap: var(--space-1); flex-wrap: wrap; }

  .sheet__head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-4);
    padding: var(--space-4);
    border-block-end: var(--border-width) solid var(--border-subtle);
    background: var(--bg-surface);
  }
  .sheet__title { font-size: var(--text-md); }
  .sheet__sub { font-size: var(--text-xs); color: var(--fg-muted); }
  .sheet__head-right { display: flex; align-items: center; gap: var(--space-2); }

  .form { flex: 1; overflow-y: auto; padding: var(--space-4); }
  .group { border: 0; padding: 0; margin: 0 0 var(--space-6); }
  .group__legend {
    font-size: var(--text-2xs);
    font-weight: var(--weight-bold);
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--fg-subtle);
    padding: 0;
    margin-block-end: var(--space-2);
  }
  .group__grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
    gap: var(--space-3);
  }
  .field--wide { grid-column: 1 / -1; }
  .mask {
    margin-inline-start: var(--space-1);
    padding: 0 var(--space-1);
    border: var(--border-width) solid var(--border-default);
    border-radius: var(--radius-sm);
    font-size: var(--text-2xs);
    font-weight: var(--weight-semibold);
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: var(--fg-muted);
    background: var(--bg-surface-3);
  }

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
  .absent p { font-size: var(--text-sm); max-width: 28rem; }

  .meta-note {
    display: flex;
    gap: var(--space-2);
    padding: var(--space-3);
    border: var(--border-width) solid var(--border-subtle);
    border-radius: var(--radius-md);
    background: var(--bg-surface);
    font-size: var(--text-xs);
    color: var(--fg-muted);
  }
  .meta-note :global(svg) { flex: none; color: var(--fg-subtle); margin-block-start: 1px; }
  .meta-block { margin-block-start: var(--space-6); }

  .withheld { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: var(--space-2); }
  .withheld__item {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: var(--space-2);
    border: var(--border-width) dashed var(--border-default);
    border-radius: var(--radius-md);
    background: var(--bg-surface);
  }
  .withheld__label { font-size: var(--text-xs); font-weight: var(--weight-semibold); }
  .withheld__reason { font-size: var(--text-2xs); color: var(--fg-muted); }
  .withheld__value { font-size: var(--text-2xs); color: var(--fg-subtle); overflow-wrap: anywhere; }
  .withheld__empty { font-size: var(--text-xs); color: var(--fg-subtle); }

  .field-rule {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding-block: var(--space-1);
    border-block-start: var(--border-width) solid var(--border-subtle);
    font-size: var(--text-xs);
  }
  .field-rule__label { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .field-rule__mask { font-size: var(--text-2xs); color: var(--fg-subtle); }
</style>
