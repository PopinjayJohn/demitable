<script lang="ts">
  import { onMount } from "svelte";
  import { loadTabletop, moveToken, advanceTurn, type TabletopEncounter, type TabletopToken } from "../lib/tabletop";
  import { apiFetch } from "../lib/auth";
  import { surfaceVisibility } from "../lib/dashboard";

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
    sid = params.get("session") || params.get("sid") || "";
    eid = params.get("encounter") || params.get("eid") || "";
    displayMode = params.get("display") === "1";
    void load();
    // Poll at human cadence (<=1 Hz) per F2a: board p95 ~100 ms, so a 1 s
    // refresh is ~10x the measured worst case and never rAF-polls. Refresh
    // also happens on WT event (vault.changed) in production wiring.
    const timer = setInterval(() => { void load(); }, 1000);
    return () => clearInterval(timer);
  });
</script>

<section class="tabletop" aria-label="Tabletop">
  <p role="status">Status: {status}</p>
  {#if displayMode}<p class="display-badge">Display mode (party-only)</p>{/if}
  <div class="board" role="region" aria-label="board" ondragover={(e) => e.preventDefault()} ondrop={onDrop}>
    {#if encounter?.tokens}
      {#each encounter.tokens as tok}
        <div
          class="token"
          role="button"
          tabindex="0"
          draggable="true"
          ondragstart={(e) => onDragStart(tok, e)}
          ondragend={() => (dragging = null)}
          style={`left:${Number(tok.x ?? 0)}px; top:${Number(tok.y ?? 0)}px;`}
          title={String(tok.id)}
        >
          {tok.id}
        </div>
      {/each}
    {/if}
  </div>
  <aside class="initiative">
    <h3>Initiative</h3>
    <ol>
      {#each (encounter?.initiative_order ?? encounter?.initiative ?? []) as uid, i}
        <li class:current={i === (encounter?.current_turn ?? encounter?.turn ?? 0)}>{uid}</li>
      {/each}
    </ol>
    <button type="button" onclick={doAdvance}>Advance turn</button>
  </aside>
</section>

<style>
  .tabletop { display: grid; grid-template-columns: 1fr 240px; gap: 1rem; padding: 1rem; }
  .board { position: relative; min-height: 400px; border: 1px solid #ccc; background: #222; }
  .token { position: absolute; width: 32px; height: 32px; border-radius: 50%; background: #8cf; display: flex; align-items: center; justify-content: center; font-size: 10px; cursor: grab; user-select: none; }
  .initiative ol { list-style: decimal; }
  .current { font-weight: bold; color: #f90; }
  .display-badge { color: #0a0; }
</style>
