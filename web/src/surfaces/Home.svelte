<script lang="ts">
  // Wiki landing: the campaign home page plus the campaign's state of
  // play. The home note resolves per campaign: the campaign note's
  // `home:` link first, then the type=home note in that campaign, then
  // the campaign note itself.
  import { fetchNote, createNote } from "../lib/api";
  import { vaultStore } from "../lib/vaultstore.svelte";
  import { appState } from "../lib/appstate.svelte";
  import { displayName } from "../lib/display";
  import MarkdownView from "../components/MarkdownView.svelte";
  import VisibilityBadge from "../components/VisibilityBadge.svelte";
  import VisibilityInspector from "../components/VisibilityInspector.svelte";
  import SessionButton from "../components/SessionButton.svelte";
  import Icon from "../components/Icon.svelte";

  const active = $derived(appState.campaignFilter);
  const campaignNote = $derived(
    active
      ? (vaultStore.notes.find(
          (n) =>
            String((n as any).type ?? "") === "campaign" &&
            (displayName((n as any).campaign) === displayName(active) ||
              displayName((n as any).name) === displayName(active)),
        ) ?? null)
      : null,
  );

  let campaignDoc = $state<any>(null);
  let homeUid = $state<string | null>(null);
  let homeSource = $state("");
  let newType = $state("location");
  let newName = $state("");

  function resolveLinkTarget(link: string): string | null {
    const m = String(link ?? "").match(/^\[\[([^\]|]+?)(?:\|([^\]]+?))?\]\]$/);
    const target = (m ? m[1] : String(link ?? "")).trim();
    if (!target) return null;
    const hit = vaultStore.notes.find(
      (n) => String((n as any).name ?? "") === target || String(n.uid) === target,
    );
    return hit ? String(hit.uid) : null;
  }

  async function resolveHome() {
    homeUid = null;
    homeSource = "";
    campaignDoc = null;
    if (!active || !campaignNote) return;
    try {
      campaignDoc = await fetchNote(String(campaignNote.uid));
    } catch { /* fall back to list data */ }
    const declared = (campaignDoc as any)?.home ?? (campaignNote as any)?.home;
    homeUid = (declared && resolveLinkTarget(String(declared))) || null;
    if (!homeUid) {
      const home = vaultStore.notes.find(
        (n) =>
          String((n as any).type ?? "") === "home" &&
          displayName((n as any).campaign) === displayName(active),
      );
      homeUid = home ? String(home.uid) : String(campaignNote.uid);
    }
    try {
      const doc = await fetchNote(homeUid);
      homeSource = doc.content;
    } catch { homeSource = ""; }
  }
  $effect(() => { void active; void campaignNote?.uid; void resolveHome(); });
  $effect(() => {
    appState.rail = railContent;
    return () => { if (appState.rail === railContent) appState.rail = null; };
  });

  const scoped = $derived(
    active ? vaultStore.notes.filter((n) => displayName((n as any).campaign ?? "No campaign") === displayName(active)) : [],
  );
  const byType = $derived(() => {
    const m = new Map<string, number>();
    for (const n of scoped) m.set(String((n as any).type ?? "?"), (m.get(String((n as any).type ?? "?")) ?? 0) + 1);
    return [...m.entries()].sort((a, b) => b[1] - a[1]);
  });
  const attention = $derived(
    scoped.filter((n) => {
      const w = String((n as any).workflow ?? "");
      if (w === "draft" || w === "submitted") return true;
      if (String((n as any).type ?? "") === "handout" && String((n as any).visibility ?? "") === "gm") return true;
      return false;
    }),
  );
  const members: string[] = $derived(
    Array.isArray((campaignDoc as any)?.members) ? (campaignDoc as any).members.map(String) : [],
  );
  const packs: string[] = $derived(
    Array.isArray((campaignDoc as any)?.packs) ? (campaignDoc as any).packs.map(String) : [],
  );
  const wikiIndex = $derived(() => {
    const m = new Map<string, string>();
    for (const n of vaultStore.notes) if ((n as any).name) m.set(String((n as any).name), String(n.uid));
    return m;
  });

  async function create() {
    if (!newName.trim() || !active) return;
    try {
      const j = await createNote({ type: newType, name: newName.trim(), campaign: active });
      const uid = String(j.uid ?? "");
      newName = "";
      if (newType === "character" && uid) {
        appState.selectedCharacterUid = uid;
        appState.go("creator");
      } else if (uid) {
        appState.selectedUid = uid;
        appState.go("wiki", "editor");
      }
    } catch { /* status lives in the footer; keep the card quiet */ }
  }
</script>

{#snippet railContent()}
  {#if campaignNote}
    <div class="rail-block">
      <h3 class="rail-title">Campaign</h3>
      <VisibilityBadge
        visibility={String((campaignNote as any).effective_visibility ?? (campaignNote as any).effectiveVisibility ?? "party")}
        defaulted={Boolean((campaignNote as any).defaulted)}
      />
      <div class="rail-gap"></div>
      {#if members.length > 0}
        <p class="rail-item"><strong>{members.length}</strong> members</p>
      {/if}
      {#if packs.length > 0}
        <p class="rail-item">Packs: {packs.join(", ")}</p>
      {/if}
      {#if appState.showInspector}
        <div class="rail-gap"></div>
        <VisibilityInspector decisions={[]} />
      {/if}
    </div>
    <div class="rail-block">
      <button class="btn btn--sm" onclick={() => { appState.selectedUid = String(campaignNote.uid); appState.go("wiki", "editor"); }}>
        <Icon name="pencil" size={13} /> Edit campaign note
      </button>
    </div>
  {/if}
{/snippet}

<div class="home">
  {#if !active}
    <section class="picker" aria-label="Pick a campaign">
      <h2 class="home-title">Campaigns</h2>
      <ul class="cards">
        {#each vaultStore.campaigns as [name, count]}
          <li>
            <button class="card-btn" onclick={() => (appState.campaignFilter = name)}>
              <span class="card-btn__name">{displayName(name)}</span>
              <span class="card-btn__meta">{count} notes</span>
            </button>
          </li>
        {/each}
      </ul>
    </section>
  {:else}
    <header class="home-head">
      <div class="stack">
        <h2 class="home-title">{displayName(active)}</h2>
        <span class="home-sub">{scoped.length} notes · {byType().length} types</span>
      </div>
      <button class="btn btn--sm" onclick={() => (appState.campaignFilter = null)}>All campaigns</button>
    </header>

    <section class="grid">
      <article class="card home-body" aria-label="Campaign home page">
        <MarkdownView
          source={homeSource}
          noteUid={homeUid ?? ""}
          resolveWiki={(name) => wikiIndex().get(name) ?? null}
          onOpenNote={(uid) => { appState.selectedUid = uid; appState.go("wiki", "viewer"); }}
        />
      </article>

      <div class="side-col">
        <section class="card stats" aria-label="Campaign stats">
          <h3 class="card-title">Contents</h3>
          <dl class="stat-list">
            {#each byType() as [type, count]}
              <div class="stat-row"><dt>{type}</dt><dd>{count}</dd></div>
            {/each}
          </dl>
        </section>

        <section class="card session" aria-label="Session">
          <h3 class="card-title">Table</h3>
          {#if appState.sessionId}
            <p class="mono">live: {appState.sessionId.slice(0, 8)}</p>
            <button class="btn btn--sm" onclick={() => appState.go("vtt")}>Open tabletop</button>
          {:else}
            <p class="rail-empty">No active session.</p>
          {/if}
          <SessionButton />
        </section>

        <section class="card quick" aria-label="Quick actions">
          <h3 class="card-title">New note</h3>
          <div class="row">
            <select bind:value={newType} aria-label="Note type">
              {#each ["character", "location", "faction", "item", "quest", "handout", "document"] as t}
                <option value={t}>{t}</option>
              {/each}
            </select>
            <input bind:value={newName} placeholder="Name" aria-label="New note name" />
            <button class="btn btn--sm" onclick={create}>Create</button>
          </div>
        </section>

        {#if attention.length > 0}
          <section class="card attention" aria-label="Needs attention">
            <h3 class="card-title">Needs attention <span class="tag">{attention.length}</span></h3>
            <ul class="attn-list">
              {#each attention as n (n.uid)}
                <li>
                  <button class="attn" onclick={() => { appState.selectedUid = String(n.uid); appState.go("wiki", "viewer"); }}>
                    {String(n.name ?? n.uid)}
                  </button>
                </li>
              {/each}
            </ul>
          </section>
        {/if}
      </div>
    </section>
  {/if}
</div>

<style>
  .home { height: 100%; overflow-y: auto; padding: var(--space-6); background: var(--bg-canvas); }
  .home-head { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--space-4); margin-block-end: var(--space-4); }
  .home-title { font-size: var(--text-lg); }
  .home-sub { font-size: var(--text-xs); color: var(--fg-muted); }
  .picker { max-width: 44rem; }
  .cards { list-style: none; margin: var(--space-4) 0 0; padding: 0; display: grid; grid-template-columns: repeat(auto-fill, minmax(14rem, 1fr)); gap: var(--space-3); }
  .card-btn { width: 100%; text-align: start; padding: var(--space-4); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--bg-surface); cursor: pointer; display: flex; flex-direction: column; gap: var(--space-1); }
  .card-btn:hover { border-color: var(--border-strong); background: var(--bg-surface-2); }
  .card-btn__name { font-size: var(--text-base); font-weight: var(--weight-semibold); }
  .card-btn__meta { font-size: var(--text-xs); color: var(--fg-muted); }
  .grid { display: grid; grid-template-columns: minmax(0, 1.6fr) minmax(16rem, 1fr); gap: var(--space-4); align-items: start; }
  .card { padding: var(--space-4); }
  .card-title { font-size: var(--text-2xs); font-weight: var(--weight-bold); letter-spacing: 0.08em; text-transform: uppercase; color: var(--fg-subtle); margin-block-end: var(--space-2); }
  .side-col { display: flex; flex-direction: column; gap: var(--space-4); }
  .stat-list { margin: 0; display: flex; flex-direction: column; gap: 2px; }
  .stat-row { display: flex; justify-content: space-between; font-size: var(--text-sm); }
  .stat-row dd { margin: 0; color: var(--fg-muted); font-variant-numeric: tabular-nums; }
  .rail-empty { font-size: var(--text-xs); color: var(--fg-subtle); }
  .row { display: flex; gap: var(--space-1); }
  .row select, .row input { flex: 1; min-width: 0; padding: var(--space-1) var(--space-2); border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--bg-surface); color: var(--fg-default); font-size: var(--text-sm); }
  .attn-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 2px; }
  .attn { width: 100%; text-align: start; background: none; border: none; cursor: pointer; font-size: var(--text-sm); color: var(--fg-default); padding: var(--space-1) 0; }
  .attn:hover { color: var(--accent); }
  .rail-block + .rail-block { margin-block-start: var(--space-6); }
  .rail-title { font-size: var(--text-2xs); font-weight: var(--weight-bold); letter-spacing: 0.08em; text-transform: uppercase; color: var(--fg-subtle); margin-block-end: var(--space-2); }
  .rail-gap { height: var(--space-2); }
  .rail-item { font-size: var(--text-xs); color: var(--fg-muted); }
</style>
