<script lang="ts">
  import { onMount } from "svelte";
  import { loadTabletop, moveToken, advanceTurn, type TabletopEncounter, type TabletopToken } from "../lib/tabletop";
  import { apiFetch } from "../lib/auth";
  import { surfaceVisibility } from "../lib/dashboard";
  import { appState } from "../lib/appstate.svelte";

  let sid = $state<string>("");
  let eid = $state<string>("");
  let encounter = $state<TabletopEncounter | null>(null);
  let status = $state<string>("loading");
  let displayMode = $state<boolean>(false);
  let dragging = $state<string | null>(null);

  async function load() {
    status = "loading";
    try {
      const data = await loadTabletop(sid, eid);
      encounter = data.encounter;
      for (const n of data.notes ?? []) surfaceVisibility(n);
      status = "loaded";
    } catch (e: any) {
      status = e?.message ?? "load failed";
    }
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

  onMount(() => {
    const params = new URLSearchParams(location.search);
    sid = params.get("session") || params.get("sid") || appState.sessionId || "";
    eid = params.get("encounter") || params.get("eid") || appState.encounterId || "";
    displayMode = params.get("display") === "1" || appState.displayMode;
    void load();
    // Poll at human cadence (<=1 Hz) per F2a: board p95 ~100 ms, so a 1 s
    // refresh is ~10x the measured worst case and never rAF-polls. Refresh
    // also happens on WT event (vault.changed) in production wiring.
    const timer = setInterval(() => { void load(); }, 1000);
    return () => clearInterval(timer);
  });
</script>

<section class="tabletop" aria-label="Tabletop">
  <div class="board-wrap">
    <div class="board-bar" role="status">
      <span>Status: {status}</span>
      <label class="sess">session <input bind:value={sid} placeholder="sid" aria-label="Session id" /></label>
      <label class="sess">encounter <input bind:value={eid} placeholder="eid" aria-label="Encounter id" /></label>
      <button class="btn btn--sm" onclick={load}>Load</button>
      <label class="sess"><input type="checkbox" bind:checked={displayMode} /> display</label>
      {#if displayMode}<span class="display-badge">Display mode (party-only)</span>{/if}
      {#if encounter?.round !== undefined}<span class="mono">round {String(encounter.round)}</span>{/if}
    </div>
    <div class="board" role="region" aria-label="board" class:fog={true} ondragover={(e) => e.preventDefault()} ondrop={onDrop}>
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
    <p class="hint">Drag a note onto the board to seat it as a token. Board polls at ≤1 Hz plus WS refresh; tokens are intents, the server is authoritative.</p>
  </div>
  <aside class="initiative card">
    <h3>Initiative</h3>
    <ol>
      {#each (encounter?.initiative_order ?? encounter?.initiative ?? []) as uid, i}
        <li class:current={i === (encounter?.current_turn ?? encounter?.turn ?? 0)}>{uid}</li>
      {/each}
    </ol>
    {#if !displayMode}<button class="btn btn--sm" type="button" onclick={doAdvance}>Advance turn</button>{/if}
  </aside>
</section>

<style>
  .tabletop { display: grid; grid-template-columns: 1fr 240px; gap: var(--space-4); padding: var(--space-4); background: var(--bg-canvas); min-height: 60vh; }
  .board-wrap { display: flex; flex-direction: column; gap: var(--space-2); min-width: 0; }
  .board-bar { display: flex; gap: var(--space-2); align-items: center; font-size: var(--text-xs); color: var(--fg-muted); flex-wrap: wrap; }
  .board-bar input { width: 8rem; padding: 2px var(--space-1); font-size: var(--text-xs); }
  .sess { display: flex; align-items: center; gap: 4px; }
  .display-badge { color: var(--success); font-weight: var(--weight-semibold); }
  .board { position: relative; min-height: 480px; border: 1px solid var(--border-strong); border-radius: var(--radius-lg); background:
    linear-gradient(var(--bg-surface-3) 1px, transparent 1px),
    linear-gradient(90deg, var(--bg-surface-3) 1px, transparent 1px),
    #23211d; background-size: 32px 32px; overflow: hidden; }
  .board.fog::after { content: ""; position: absolute; inset: 0; pointer-events: none;
    background: radial-gradient(ellipse at 50% 50%, transparent 55%, rgb(0 0 0 / 0.45) 100%); }
  .token { position: absolute; width: 36px; height: 36px; border-radius: var(--radius-pill); background: var(--accent-subtle); border: 2px solid var(--accent-border); color: var(--fg-default); display: flex; align-items: center; justify-content: center; font-size: 10px; font-weight: var(--weight-bold); cursor: grab; user-select: none; box-shadow: var(--shadow-2); }
  .token.hidden-tok { opacity: 0.35; border-style: dashed; }
  .initiative { padding: var(--space-3); align-self: start; }
  .initiative h3 { font-size: var(--text-xs); text-transform: uppercase; letter-spacing: 0.06em; color: var(--fg-subtle); margin-block-end: var(--space-2); }
  .initiative ol { list-style: decimal; margin: 0 0 var(--space-3); padding-inline-start: var(--space-4); font-size: var(--text-sm); }
  .current { font-weight: var(--weight-bold); color: var(--accent); }
  .hint { font-size: var(--text-xs); color: var(--fg-subtle); }
</style>
