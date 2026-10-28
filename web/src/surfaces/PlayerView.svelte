<script lang="ts">
  import { onMount } from "svelte";
  import { fetchNotes, fetchNote } from "../lib/api";
  import { surfaceVisibility } from "../lib/dashboard";
  import { getPrincipal } from "../lib/auth";
  import { appState } from "../lib/appstate.svelte";
  import VisibilityBadge from "../components/VisibilityBadge.svelte";
  import MarkdownView from "../components/MarkdownView.svelte";
  import Icon from "../components/Icon.svelte";
  import type { NoteSummary } from "../lib/api";

  let notes = $state<NoteSummary[]>([]);
  let status = $state("loading");
  let openUid = $state<string | null>(null);
  let openSource = $state("");
  const principal = $derived(getPrincipal());
  const eyeAs = $derived(appState.playerEye && appState.eyeTarget ? appState.eyeTarget : "");

  const visible = $derived(notes.filter((n) => { try { surfaceVisibility(n); return true; } catch { return false; } }));
  const mine = $derived(visible.filter((n) => String((n as any).player ?? "") !== "" && principal && String((n as any).player) === String((principal as any).id)));
  const rest = $derived(visible.filter((n) => !mine.includes(n)));
  const wikiIndex = $derived(() => {
    const m = new Map<string, string>();
    for (const n of visible) if ((n as any).name) m.set(String((n as any).name), String(n.uid));
    return m;
  });

  function vis(n: NoteSummary): string {
    try { return surfaceVisibility(n); } catch { return "gm"; }
  }
  async function load() {
    status = "loading";
    try {
      notes = await fetchNotes({ ...(eyeAs ? { as: eyeAs } : {}), } as any);
      status = "loaded";
    } catch (e: any) { status = e?.message ?? "failed"; }
  }
  async function open(n: NoteSummary) {
    openUid = String(n.uid);
    try {
      const doc = await fetchNote(openUid, eyeAs ? { as: eyeAs } : {});
      openSource = doc.content;
    } catch { openSource = ""; }
  }
  let lastEye = $state("");
  $effect(() => {
    const cur = `${appState.playerEye}|${appState.eyeTarget}`;
    if (cur !== lastEye) { lastEye = cur; openUid = null; void load(); }
  });
  onMount(() => { lastEye = `${appState.playerEye}|${appState.eyeTarget}`; void load(); });
  function typeIcon(t: string) {
    return (({ character: "user", location: "globe", handout: "file", quest: "list", session: "scroll" } as Record<string, any>)[t] ?? "file") as any;
  }
</script>

<div class="player">
  <header class="player__head">
    <div class="player__identity">
      <span class="player__avatar" aria-hidden="true">{String((principal as any)?.id ?? "?").slice(0, 1).toUpperCase()}</span>
      <div class="stack">
        <h1 class="player__name">{String((principal as any)?.id ?? "Player")}{#if eyeAs} <span class="tag">as {eyeAs}</span>{/if}</h1>
        <span class="player__campaign">{String((principal as any)?.kind ?? "")}</span>
      </div>
    </div>
    <div class="player__head-right"><span class="tag"><Icon name="eye" size={11} /> read-only between sessions</span></div>
  </header>

  <div class="player__body">
    <section class="wiki">
      <h2 class="wiki__title">The campaign</h2>
      {#if mine.length > 0}
        <h3 class="wiki__sub">Yours</h3>
        <ul class="cards">
          {#each mine as n (n.uid)}
            <li><button class="wcard" onclick={() => open(n)}><span class="wcard__icon"><Icon name={typeIcon(String((n as any).type ?? ""))} size={15} /></span><span class="wcard__body"><span class="wcard__name">{String(n.name ?? n.uid)}</span><span class="wcard__type">{String((n as any).type ?? "")}</span></span><VisibilityBadge visibility={vis(n)} size="sm" /></button></li>
          {/each}
        </ul>
      {/if}
      <h3 class="wiki__sub">What you have discovered</h3>
      <ul class="cards">
        {#each rest as n (n.uid)}
          <li><button class="wcard" onclick={() => open(n)}><span class="wcard__icon"><Icon name={typeIcon(String((n as any).type ?? ""))} size={15} /></span><span class="wcard__body"><span class="wcard__name">{String(n.name ?? n.uid)}</span><span class="wcard__type">{String((n as any).type ?? "")}</span></span><VisibilityBadge visibility={vis(n)} size="sm" /></button></li>
        {/each}
      </ul>
      <p class="wiki__note">Nothing here says a note is missing. A note you have not discovered is indistinguishable from one that does not exist. ({status})</p>
    </section>
  </div>

  {#if openUid}
    <div class="reader" role="dialog" aria-label="Note">
      <header class="reader__head"><h2 class="reader__title">{openUid}</h2><button class="btn btn--sm" onclick={() => (openUid = null)}>Close</button></header>
      <div class="reader__body">
        <MarkdownView
          source={openSource}
          resolveWiki={(name) => wikiIndex().get(name) ?? null}
          onOpenNote={async (uid) => {
            try {
              const doc = await fetchNote(uid, eyeAs ? { as: eyeAs } : {});
              openUid = uid; openSource = doc.content;
            } catch { /* dangling in this projection */ }
          }}
        />
      </div>
    </div>
  {/if}
</div>

<style>
  .player { height: 100%; overflow-y: auto; background: var(--bg-canvas); min-height: 60vh; }
  .player__head { display: flex; align-items: center; justify-content: space-between; gap: var(--space-4); padding: var(--space-4) var(--space-6); border-block-end: 1px solid var(--border-subtle); background: var(--bg-surface); }
  .player__identity { display: flex; align-items: center; gap: var(--space-3); }
  .player__avatar { width: 2.5rem; height: 2.5rem; display: grid; place-items: center; border-radius: var(--radius-pill); background: var(--accent-subtle); border: 1px solid var(--accent-border); color: var(--accent); font-weight: var(--weight-bold); }
  .player__name { font-size: var(--text-md); }
  .player__campaign { font-size: var(--text-xs); color: var(--fg-muted); }
  .player__body { max-width: 60rem; margin-inline: auto; padding: var(--space-6); display: flex; flex-direction: column; gap: var(--space-6); }
  .wiki__title { font-size: var(--text-lg); }
  .wiki__sub { font-size: var(--text-2xs); font-weight: var(--weight-bold); letter-spacing: 0.08em; text-transform: uppercase; color: var(--fg-subtle); margin-block: var(--space-4) var(--space-2); }
  .cards { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: var(--space-2); }
  .wcard { display: flex; align-items: center; gap: var(--space-3); width: 100%; padding: var(--space-3); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--bg-surface); text-align: start; cursor: pointer; }
  .wcard:hover { border-color: var(--border-strong); background: var(--bg-surface-2); }
  .wcard__icon { color: var(--fg-subtle); display: inline-flex; }
  .wcard__body { display: flex; flex-direction: column; gap: 1px; flex: 1; min-width: 0; }
  .wcard__name { font-size: var(--text-sm); font-weight: var(--weight-medium); }
  .wcard__type { font-size: var(--text-2xs); color: var(--fg-subtle); }
  .wiki__note { margin-block-start: var(--space-4); font-size: var(--text-xs); color: var(--fg-subtle); max-width: 34rem; }
  .reader { position: absolute; inset-block: var(--topbar-height) 0; inset-inline-end: 0; width: min(42rem, 90vw); border-inline-start: 1px solid var(--border-default); background: var(--bg-surface); box-shadow: var(--shadow-3); display: flex; flex-direction: column; z-index: var(--z-modal); }
  .reader__head { display: flex; align-items: center; justify-content: space-between; padding: var(--space-3) var(--space-4); border-block-end: 1px solid var(--border-subtle); }
  .reader__title { font-size: var(--text-md); }
  .reader__body { flex: 1; overflow-y: auto; padding: var(--space-4); }
</style>
