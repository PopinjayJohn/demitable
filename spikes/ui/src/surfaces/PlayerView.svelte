<script lang="ts">
  import Icon from "../components/Icon.svelte";
  import VisibilityBadge from "../components/VisibilityBadge.svelte";
  import MarkdownView from "../components/MarkdownView.svelte";
  import { store } from "../lib/store.svelte";
  import { NOTES, PRINCIPALS } from "../lib/campaign";
  
  const player = $derived(
    store.principal.kind === "player" ? store.principal : PRINCIPALS[1],
  );

  /** Everything the player may see, and nothing else. */
  const visible = $derived(
    NOTES.filter((n) => store.projectionFor(n, player).visible),
  );

  const theirCharacter = $derived(
    NOTES.find((n) => n.type === "character" && n.player === player.id),
  );

  const sheet = $derived(
    theirCharacter ? store.sheetFor(theirCharacter, player) : null,
  );

  /** The player's own notes, first. */
  const own = $derived(visible.filter((n) => n.player === player.id));
  const rest = $derived(visible.filter((n) => n.player !== player.id));

  let openNote = $state<string | null>(null);

  const open = $derived(visible.find((n) => n.uid === openNote));
  const openProjection = $derived(open ? store.projectionFor(open, player) : null);

  function typeIcon(type: string) {
    switch (type) {
      case "monster": return "shield" as const;
      case "spell": return "sparkle" as const;
      case "character": return "user" as const;
      case "session": return "scroll" as const;
      case "location": return "globe" as const;
      case "handout": return "file" as const;
      case "quest": return "list" as const;
      default: return "file" as const;
    }
  }
</script>

<div class="player">
  <header class="player__head">
    <div class="player__identity">
      <span class="player__avatar" aria-hidden="true">{player.name.slice(0, 1)}</span>
      <div class="stack">
        <h1 class="player__name">{player.name}</h1>
        <span class="player__campaign">Stormdark · the Drowned March</span>
      </div>
    </div>
    <div class="player__head-right">
      <span class="tag"><Icon name="eye" size={11} /> read-only between sessions</span>
    </div>
  </header>

  <div class="player__body">
    <!-- ─── the character's own sheet ──────────────────────────────── -->
    {#if theirCharacter && sheet}
      <section class="card mine">
        <header class="card__head">
          <h2 class="card__title"><Icon name="user" size={15} /> Your character</h2>
          <span class="subtle">{theirCharacter.name}</span>
        </header>
        <dl class="quick">
          {#each sheet.fields.filter((f) => f.visible && ["level", "hp_max", "hp_current", "ac", "speed", "inspiration", "conditions"].includes(f.field.key)) as row (row.field.key)}
            <div class="quick__item">
              <dt>{row.field.label}</dt>
              <dd>{Array.isArray(row.value) ? row.value.join(", ") : String(row.value ?? "—")}</dd>
            </div>
          {/each}
        </dl>
        <p class="mine__note">
          This is the sheet the server sends you. Fields you may not see are
          not blanked out — they are not in the payload.
        </p>
      </section>
    {/if}

    <!-- ─── the campaign wiki ──────────────────────────────────────── -->
    <section class="wiki">
      <h2 class="wiki__title">The campaign</h2>

      {#if own.length > 0}
        <h3 class="wiki__sub">Yours</h3>
        <ul class="cards">
          {#each own as n (n.uid)}
            <li>
              <button class="wcard" onclick={() => (openNote = n.uid)}>
                <span class="wcard__icon"><Icon name={typeIcon(n.type)} size={15} /></span>
                <span class="wcard__body">
                  <span class="wcard__name">{n.name}</span>
                  <span class="wcard__type">{n.type}</span>
                </span>
                <VisibilityBadge
                  visibility={n.visibility ?? (n.campaign ? "party" : "gm")}
                  defaulted={n.visibility === undefined}
                  size="sm"
                />
              </button>
            </li>
          {/each}
        </ul>
      {/if}

      <h3 class="wiki__sub">What you have discovered</h3>
      <ul class="cards">
        {#each rest as n (n.uid)}
          <li>
            <button class="wcard" onclick={() => (openNote = n.uid)}>
              <span class="wcard__icon"><Icon name={typeIcon(n.type)} size={15} /></span>
              <span class="wcard__body">
                <span class="wcard__name">{n.name}</span>
                <span class="wcard__type">{n.type}</span>
              </span>
              <VisibilityBadge
                visibility={n.visibility ?? (n.campaign ? "party" : "gm")}
                defaulted={n.visibility === undefined}
                size="sm"
              />
            </button>
          </li>
        {/each}
      </ul>

      <p class="wiki__note">
        Nothing here says a note is missing. The projection does not return a
        placeholder for a note you may not see, so a note you have not
        discovered is indistinguishable from a note that does not exist.
      </p>
    </section>
  </div>

  <!-- ─── the reading pane ─────────────────────────────────────────── -->
  {#if open && openProjection}
    <div class="reader" role="dialog" aria-label={open.name}>
      <header class="reader__head">
        <h2 class="reader__title">{open.name}</h2>
        <button class="btn btn--sm" onclick={() => (openNote = null)}>Close</button>
      </header>
      <div class="reader__body">
        <MarkdownView body={openProjection.body} />
      </div>
    </div>
  {/if}
</div>

<style>
  .player {
    height: 100%;
    overflow-y: auto;
    background: var(--bg-canvas);
  }
  .player__head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
    padding: var(--space-4) var(--space-6);
    border-block-end: var(--border-width) solid var(--border-subtle);
    background: var(--bg-surface);
  }
  .player__identity { display: flex; align-items: center; gap: var(--space-3); }
  .player__avatar {
    width: 2.5rem; height: 2.5rem;
    display: grid; place-items: center;
    border-radius: var(--radius-pill);
    background: var(--accent-subtle);
    border: var(--border-width) solid var(--accent-border);
    color: var(--accent);
    font-weight: var(--weight-bold);
  }
  .player__name { font-size: var(--text-md); }
  .player__campaign { font-size: var(--text-xs); color: var(--fg-muted); }

  .player__body {
    max-width: 60rem;
    margin-inline: auto;
    padding: var(--space-6);
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }

  .card { padding: var(--space-4); }
  .card__head { display: flex; align-items: baseline; justify-content: space-between; margin-block-end: var(--space-3); }
  .card__title { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-base); }

  .quick { display: grid; grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr)); gap: var(--space-3); margin: 0; }
  .quick__item { display: flex; flex-direction: column; gap: 2px; }
  .quick dt { font-size: var(--text-2xs); font-weight: var(--weight-bold); letter-spacing: 0.06em; text-transform: uppercase; color: var(--fg-subtle); }
  .quick dd { margin: 0; font-size: var(--text-md); font-weight: var(--weight-semibold); font-variant-numeric: tabular-nums; }
  .mine__note { margin-block-start: var(--space-3); font-size: var(--text-xs); color: var(--fg-muted); }

  .wiki__title { font-size: var(--text-lg); }
  .wiki__sub {
    font-size: var(--text-2xs);
    font-weight: var(--weight-bold);
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--fg-subtle);
    margin-block: var(--space-4) var(--space-2);
  }
  .cards { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: var(--space-2); }
  .wcard {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    width: 100%;
    padding: var(--space-3);
    border: var(--border-width) solid var(--border-subtle);
    border-radius: var(--radius-md);
    background: var(--bg-surface);
    text-align: start;
    cursor: pointer;
  }
  .wcard:hover { border-color: var(--border-strong); background: var(--bg-surface-2); }
  .wcard__icon { color: var(--fg-subtle); display: inline-flex; }
  .wcard__body { display: flex; flex-direction: column; gap: 1px; flex: 1; min-width: 0; }
  .wcard__name { font-size: var(--text-sm); font-weight: var(--weight-medium); }
  .wcard__type { font-size: var(--text-2xs); color: var(--fg-subtle); }
  .wiki__note { margin-block-start: var(--space-4); font-size: var(--text-xs); color: var(--fg-subtle); max-width: 34rem; }

  .reader {
    position: absolute;
    inset-block: var(--topbar-height) 0;
    inset-inline-end: 0;
    width: min(42rem, 90vw);
    border-inline-start: var(--border-width) solid var(--border-default);
    background: var(--bg-surface);
    box-shadow: var(--shadow-3);
    display: flex;
    flex-direction: column;
  }
  .reader__head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-3) var(--space-4);
    border-block-end: var(--border-width) solid var(--border-subtle);
  }
  .reader__title { font-size: var(--text-md); }
  .reader__body { flex: 1; overflow-y: auto; padding: var(--space-4); }
</style>
