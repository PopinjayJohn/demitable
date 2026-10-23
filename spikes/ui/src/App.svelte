<script lang="ts">
  import "./lib/tokens.css";
  import "./lib/base.css";
  import Icon from "./components/Icon.svelte";
    import GmDashboard from "./surfaces/GmDashboard.svelte";
  import NoteEditor from "./surfaces/NoteEditor.svelte";
  import CharacterPanel from "./surfaces/CharacterPanel.svelte";
  import PlayerView from "./surfaces/PlayerView.svelte";
  import { store, type Proof, type Surface } from "./lib/store.svelte";
  import { PRINCIPALS } from "./lib/campaign";

  const SURFACES: { id: Surface; label: string; hint: string }[] = [
    { id: "dashboard", label: "GM browser", hint: "The GM's campaign browser and dashboard" },
    { id: "editor", label: "Note editor", hint: "Source pane, live preview, frontmatter panel" },
    { id: "character", label: "Character panel", hint: "A form rendered from a pack's declared fields" },
    { id: "player", label: "Player's view", hint: "What a player actually receives" },
  ];

  const PROOFS: { id: Proof; label: string; hint: string }[] = [
    { id: "none", label: "Normal", hint: "The design as it ships" },
    { id: "greyscale", label: "Greyscale", hint: "The colour-independence check" },
    { id: "flat-hue", label: "Flat hue", hint: "Every semantic colour becomes one grey — only shape, label and lightness remain" },
    { id: "protanopia", label: "Protanopia", hint: "Red-blindness" },
    { id: "deuteranopia", label: "Deuteranopia", hint: "Green-blindness" },
    { id: "tritanopia", label: "Tritanopia", hint: "Blue-blindness" },
  ];

  const isGm = $derived(store.principal.kind === "gm");

  // The theme is a token override on the document element, so the whole app —
  // including every plugin surface in a shadow root — follows it.
  $effect(() => {
    document.documentElement.setAttribute("data-theme", store.theme);
  });
</script>

<svelte:head>
  <title>Demitable — UI spike</title>
  <meta name="description" content="Phase 0, question 3: what does Demitable look like?" />
</svelte:head>

<!-- The colour-blindness matrices. A filter, so the whole app is affected. -->
<svg width="0" height="0" style="position: absolute" aria-hidden="true">
  <defs>
    <filter id="proof-protanopia">
      <feColorMatrix type="matrix" values="
        0.567 0.433 0 0 0
        0.558 0.442 0 0 0
        0     0.242 0.758 0 0
        0     0     0     1 0" />
    </filter>
    <filter id="proof-deuteranopia">
      <feColorMatrix type="matrix" values="
        0.625 0.375 0 0 0
        0.7   0.3   0 0 0
        0     0.3   0.7 0 0
        0     0     0     1 0" />
    </filter>
    <filter id="proof-tritanopia">
      <feColorMatrix type="matrix" values="
        0.95 0.05 0 0 0
        0    0.433 0.567 0 0
        0    0.475 0.525 0 0
        0    0     0     1 0" />
    </filter>
  </defs>
</svg>

<div id="app" data-proof={store.proof === "none" ? undefined : store.proof}>
  <header class="topbar">
    <div class="topbar__brand">
      <Icon name="sparkle" size={18} />
      <span class="topbar__name">Demitable</span>
      <span class="tag">UI spike</span>
    </div>

    <nav class="topbar__surfaces" aria-label="Surfaces">
      {#each SURFACES as s (s.id)}
        <button
          class="btn btn--sm"
          class:is-active={store.surface === s.id}
          aria-current={store.surface === s.id ? "page" : undefined}
          title={s.hint}
          onclick={() => (store.surface = s.id)}
        >
          {s.label}
        </button>
      {/each}
    </nav>

    <div class="topbar__right">
      <label class="topbar__control">
        <span class="topbar__control-label">As</span>
        <select
          value={store.principalId}
          onchange={(e) => (store.principalId = e.currentTarget.value)}
          aria-label="Render the app as"
        >
          {#each PRINCIPALS as p (p.id)}
            <option value={p.id}>{p.name}</option>
          {/each}
        </select>
      </label>

      <label class="topbar__control">
        <span class="topbar__control-label">Proof</span>
        <select
          value={store.proof}
          onchange={(e) => (store.proof = e.currentTarget.value as Proof)}
          aria-label="Colour proof mode"
        >
          {#each PROOFS as p (p.id)}
            <option value={p.id}>{p.label}</option>
          {/each}
        </select>
      </label>

      <button
        class="btn btn--sm"
        onclick={() => (store.theme = store.theme === "light" ? "dark" : "light")}
        aria-label="Toggle the dark theme"
        title="The token layer is the theming mechanism: no component changes, only the token overrides"
      >
        {store.theme === "light" ? "Dark" : "Light"}
      </button>
    </div>
  </header>

  {#if !isGm}
    <div class="banner" role="status">
      <Icon name="eye" size={14} />
      <span>
        You are looking at the app <strong>as {store.principal.name}</strong>. The
        projection has removed everything {store.principal.name} may not see.
      </span>
    </div>
  {/if}

  <main class="shell">
    {#if store.surface === "dashboard"}
      <GmDashboard />
    {:else if store.surface === "editor"}
      <NoteEditor />
    {:else if store.surface === "character"}
      <CharacterPanel />
    {:else}
      <PlayerView />
    {/if}
  </main>
</div>

<style>
  :global(html) { color-scheme: light; }
  :global(html[data-theme="dark"]) { color-scheme: dark; }

  .topbar {
    display: flex;
    align-items: center;
    gap: var(--space-4);
    height: var(--topbar-height);
    padding-inline: var(--space-4);
    border-block-end: var(--border-width) solid var(--border-subtle);
    background: var(--bg-surface);
  }
  .topbar__brand {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-weight: var(--weight-bold);
  }
  .topbar__name { font-size: var(--text-sm); letter-spacing: 0.01em; }
  .topbar__surfaces { display: flex; gap: var(--space-1); }
  .topbar__right { margin-inline-start: auto; display: flex; align-items: center; gap: var(--space-3); }
  .topbar__control { display: flex; align-items: center; gap: var(--space-2); }
  .topbar__control-label {
    font-size: var(--text-2xs);
    font-weight: var(--weight-semibold);
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--fg-subtle);
  }
  .topbar__control select {
    min-height: 1.75rem;
    padding: 0 var(--space-2);
    border: var(--border-width) solid var(--border-default);
    border-radius: var(--radius-md);
    background: var(--bg-surface);
    font-size: var(--text-xs);
  }

  .banner {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-4);
    background: var(--bg-surface-2);
    border-block-end: var(--border-width) solid var(--border-subtle);
    font-size: var(--text-xs);
    color: var(--fg-muted);
  }
  .banner strong { color: var(--fg-default); }

  .shell { height: calc(100% - var(--topbar-height)); overflow: hidden; }
</style>
