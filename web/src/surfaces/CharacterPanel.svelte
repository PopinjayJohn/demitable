<script lang="ts">
  import { onMount } from "svelte";
  import { fetchNotes, type NoteSummary } from "../lib/api";
  import { apiFetch } from "../lib/auth";

  let characters = $state<NoteSummary[]>([]);
  let packs = $state<any[]>([]);
  let selectedPack = $state<string>("dnd5e");
  let selectedUid = $state<string | null>(null);
  let detail = $state<any>(null);
  let status = $state<string>("loading");
  let createName = $state<string>("New Character");
  let principalKind = $state<string>("");

  async function load() {
    status = "loading";
    try {
      const notes = await fetchNotes({ type: "character" });
      characters = notes;
      const res = await apiFetch("/api/v1/packs");
      if (res.ok) {
        const j = await res.json();
        packs = Array.isArray(j) ? j : (j.items ?? j.packs ?? []);
      }
      status = "loaded";
    } catch (e) {
      status = "failed";
    }
  }

  async function open(uid: string) {
    selectedUid = uid;
    const res = await apiFetch(`/api/v1/characters/${encodeURIComponent(uid)}`);
    if (res.ok) detail = await res.json();
    else detail = null;
  }

  async function create() {
    const res = await apiFetch("/api/v1/characters", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: createName, system: selectedPack, kind: "pc" }),
    });
    if (res.ok) {
      const j = await res.json();
      await load();
      if (j.uid) await open(j.uid);
    }
  }

  async function submit() {
    if (!selectedUid) return;
    await apiFetch(`/api/v1/characters/${encodeURIComponent(selectedUid)}/submit`, { method: "POST" });
    await open(selectedUid);
  }

  async function approve() {
    if (!selectedUid) return;
    await apiFetch(`/api/v1/characters/${encodeURIComponent(selectedUid)}/approve`, { method: "POST" });
    await open(selectedUid);
  }

  onMount(load);
</script>

<section class="character-panel" aria-label="Character panel">
  <p role="status">Status: {status}</p>
  <div class="create">
    <input bind:value={createName} />
    <select bind:value={selectedPack}>
      {#each (packs.length ? packs : [{ id: "dnd5e" }, { id: "blades_in_the_dark" }]) as p}
        <option value={p.id ?? p}>{p.id ?? p}</option>
      {/each}
    </select>
    <button type="button" onclick={create}>Create character</button>
  </div>
  <div class="layout">
    <aside>
      <ul>
        {#each characters as c}
          <li><button type="button" onclick={() => open(String(c.uid))}>{c.name ?? c.uid}</button></li>
        {/each}
      </ul>
    </aside>
    <section>
      {#if detail}
        <h2>{detail.name}</h2>
        <p>workflow: {detail.workflow ?? "?"}</p>
        <pre>{JSON.stringify(detail.fields ?? detail, null, 2)}</pre>
        <div class="actions">
          <button type="button" onclick={submit}>Submit</button>
          <button type="button" onclick={approve}>Approve</button>
        </div>
      {:else}
        <p>Select a character.</p>
      {/if}
    </section>
  </div>
</section>

<style>
  .character-panel { padding: 1rem; }
  .layout { display: grid; grid-template-columns: 240px 1fr; gap: 1rem; }
  aside ul { list-style: none; padding: 0; }
  .actions { display: flex; gap: 0.5rem; margin-top: 1rem; }
</style>
