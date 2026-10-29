<script lang="ts">
  // VTT center: the board. Initiative, tokens and the dice log live in
  // the right rail; session inputs live in the left sidebar. Tokens are
  // intents; the server is authoritative.
  import { onMount } from "svelte";
  import { loadTabletop, moveToken, advanceTurn, type TabletopEncounter, type TabletopToken } from "../lib/tabletop";
  import { deriveMood } from "../lib/mood";
  import { cuesForMood, selectCue, cueBasename, createPlayback, type Cue } from "../lib/cues";
  import { apiFetch } from "../lib/auth";
  import { surfaceVisibility } from "../lib/dashboard";
  import { appState } from "../lib/appstate.svelte";
  import Icon from "../components/Icon.svelte";

  let encounter = $state<TabletopEncounter | null>(null);
  let status = $state<string>("idle");
  let dragging = $state<string | null>(null);
  let cues = $state<Cue[]>([]);
  let nowPlaying = $state<string>("");
  const playback = createPlayback();

  const sid = $derived(appState.sessionId);
  const eid = $derived(appState.encounterId);
  const displayMode = $derived(appState.displayMode);
  const currentMood = $derived(deriveMood(encounter ?? {}, []));
  const moodCues = $derived(cuesForMood(cues, currentMood));
  const displayTrack = $derived(selectCue(cues, currentMood) ? cueBasename(selectCue(cues, currentMood)!.src) : "");

  async function load() {
    if (!sid && !eid) { status = "pick a session in the sidebar"; return; }
    status = "loading";
    try {
      const data = await loadTabletop(sid, eid);
      encounter = data.encounter;
      for (const n of data.notes ?? []) surfaceVisibility(n);
      status = "loaded";
    } catch (e: any) {
      status = e?.message ?? "load failed";
    }
    appState.saveStatus = `board: ${status}`;
  }

  async function onDrop(ev: DragEvent) {
    ev.preventDefault();
    if (!encounter) return;
    const raw = ev.dataTransfer?.getData("application/x-demitable-note") || ev.dataTransfer?.getData("text/plain") || "";
    let noteUid = raw;
    try {
      const parsed = JSON.parse(raw);
      if (parsed && typeof parsed === "object" && parsed.uid) noteUid = String(parsed.uid);
    } catch {}
    // re-resolve through server; never trust client data
    const rect = (ev.currentTarget as HTMLElement).getBoundingClientRect();
    const x = Math.round(ev.clientX - rect.left);
    const y = Math.round(ev.clientY - rect.top);
    const res = await apiFetch(`/api/v1/sessions/${encodeURIComponent(sid)}/encounters/${encodeURIComponent(eid)}/tokens`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ note_uid: noteUid, x, y }),
    });
    if (res.ok) await load();
    else status = "drop rejected";
  }

  async function onTokenDropMove(tok: TabletopToken, ev: DragEvent) {
    ev.preventDefault();
    ev.stopPropagation();
    if (!encounter || !tok.id) return;
    const rect = (ev.currentTarget as HTMLElement).getBoundingClientRect();
    const x = Math.round(ev.clientX - rect.left);
    const y = Math.round(ev.clientY - rect.top);
    try {
      await moveToken(sid, eid, String(tok.id), x, y);
      await load();
    } catch (e: any) {
      status = e?.message ?? "move rejected";
    }
  }

  async function doAdvance() {
    try {
      await advanceTurn(sid, eid);
      await load();
    } catch (e: any) {
      status = e?.message ?? "advance failed";
    }
  }

  function onDragStart(tok: TabletopToken, ev: DragEvent) {
    dragging = String(tok.id);
    ev.dataTransfer?.setData("application/x-demitable-token", String(tok.id));
  }

  $effect(() => {
    appState.rail = railContent;
    return () => { if (appState.rail === railContent) appState.rail = null; };
  });
  // Reload when the sidebar session changes.
  $effect(() => { void sid; void eid; void load(); });

  onMount(() => {
    const params = new URLSearchParams(location.search);
    if (params.get("session") || params.get("sid")) appState.sessionId = params.get("session") || params.get("sid") || "";
    if (params.get("encounter") || params.get("eid")) appState.encounterId = params.get("encounter") || params.get("eid") || "";
    if (params.get("display") === "1") appState.displayMode = true;
    void load();
    // Poll at human cadence (<=1 Hz) per F2a: board p95 ~100 ms, so a 1 s
    // refresh is ~10x the measured worst case and never rAF-polls. Refresh
    // also happens on WT event (vault.changed) in production wiring.
    const timer = setInterval(() => { void load(); }, 1000);
    return () => clearInterval(timer);
  });
</script>

{#snippet railContent()}
  <div class="rail-block">
    <h3 class="rail-title">Initiative</h3>
    <ol class="init">
      {#each (encounter?.initiative_order ?? encounter?.initiative ?? []) as uid, i}
        <li class:current={i === (encounter?.current_turn ?? encounter?.turn ?? 0)}>{uid}</li>
      {/each}
    </ol>
    {#if !displayMode}<button class="btn btn--sm" type="button" onclick={doAdvance}>Advance turn</button>{/if}
  </div>
  <div class="rail-block">
    <h3 class="rail-title">Shared dice log</h3>
    {#if appState.diceLog.length === 0}
      <p class="rail-empty">No rolls yet. Roll from the header.</p>
    {:else}
      <ol class="dicelog">
        {#each appState.diceLog.slice().reverse() as r, i (i)}
          <li class="mono">{JSON.stringify(r).slice(0, 120)}</li>
        {/each}
      </ol>
    {/if}
  </div>
{#if true}
  <div class="rail-block">
    <h3 class="rail-title">Music</h3>
    <p class="mono">mood: {currentMood}</p>
    {#if displayMode}
      {#if displayTrack}<p class="mono">track: {displayTrack}</p>{/if}
    {:else}
      {#if nowPlaying}<p class="mono">playing: {cueBasename(nowPlaying)}</p>{/if}
      {#each moodCues as cue}
        <button class="btn btn--sm" type="button" onclick={() => { playback.play(cue); nowPlaying = cue.src; }}>Play {cue.title ?? cueBasename(cue.src)}</button>
      {/each}
      {#if moodCues.length === 0}<p class="rail-empty">No cues for this mood.</p>{/if}
    {/if}
  </div>
{/if}
{/snippet}

<section class="tabletop" aria-label="Tabletop">
  <div class="board-wrap">
    <div class="board-bar" role="status">
      <span>Status: {status}</span>
      {#if displayMode}<span class="display-badge">Display mode (party-only)</span>{/if}
      {#if encounter?.round !== undefined}<span class="mono">round {String(encounter.round)}</span>{/if}
    </div>
    <div class="board" role="region" aria-label="board" ondragover={(e) => e.preventDefault()} ondrop={onDrop}>
      {#if encounter?.tokens}
        {#each encounter.tokens as tok}
          <div
            class="token"
            class:hidden-tok={(tok as any).hidden === true}
            role="button"
            tabindex="0"
            draggable={displayMode ? "false" : "true"}
            ondragstart={(e) => onDragStart(tok, e)}
            ondragend={() => (dragging = null)}
            style={`left:${Number(tok.x ?? 0)}px; top:${Number(tok.y ?? 0)}px;`}
            title={String(tok.id)}
          >
            {String(tok.id).slice(0, 2)}
          </div>
        {/each}
      {/if}
    </div>
    <p class="hint">Drag a character from the sidebar onto the board. Board polls at ≤1 Hz plus WS refresh.</p>
  </div>
</section>

<style>
  .tabletop { background: var(--bg-canvas); height: 100%; overflow-y: auto; padding: var(--space-4); }
  .board-wrap { display: flex; flex-direction: column; gap: var(--space-2); min-width: 0; height: 100%; }
  .board-bar { display: flex; gap: var(--space-2); align-items: center; font-size: var(--text-xs); color: var(--fg-muted); flex-wrap: wrap; }
  .display-badge { color: var(--success); font-weight: var(--weight-semibold); }
  .board { position: relative; flex: 1; min-height: 480px; border: 1px solid var(--border-strong); border-radius: var(--radius-lg); background:
    linear-gradient(var(--bg-surface-3) 1px, transparent 1px),
    linear-gradient(90deg, var(--bg-surface-3) 1px, transparent 1px),
    #23211d; background-size: 32px 32px; overflow: hidden; }
  .token { position: absolute; width: 36px; height: 36px; border-radius: var(--radius-pill); background: var(--accent-subtle); border: 2px solid var(--accent-border); color: var(--fg-default); display: flex; align-items: center; justify-content: center; font-size: 10px; font-weight: var(--weight-bold); cursor: grab; user-select: none; box-shadow: var(--shadow-2); }
  .token.hidden-tok { opacity: 0.35; border-style: dashed; }
  .hint { font-size: var(--text-xs); color: var(--fg-subtle); }
  .rail-block + .rail-block { margin-block-start: var(--space-6); }
  .rail-title { font-size: var(--text-2xs); font-weight: var(--weight-bold); letter-spacing: 0.08em; text-transform: uppercase; color: var(--fg-subtle); margin-block-end: var(--space-2); }
  .rail-empty { font-size: var(--text-xs); color: var(--fg-subtle); }
  .init { list-style: decimal; margin: 0 0 var(--space-3); padding-inline-start: var(--space-4); font-size: var(--text-sm); }
  .current { font-weight: var(--weight-bold); color: var(--accent); }
  .dicelog { margin: 0; padding: 0; list-style: none; display: flex; flex-direction: column; gap: var(--space-1); font-size: var(--text-xs); }
  .dicelog li { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
