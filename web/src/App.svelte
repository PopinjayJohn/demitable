<script lang="ts">
  import { onMount } from "svelte";
  import { fetchNotes, fetchNote, saveNote, patchNote, fetchNoteRender, type NoteSummary, type NoteDoc } from "./lib/api";
  import { getToken, setToken, clearToken, getPrincipal, gmLogin, loginPlayer, joinCampaign, logout, type PrincipalClaims } from "./lib/auth";
  import { connectWS, type WSClient } from "./lib/ws";
  import Editor from "./lib/Editor.svelte";
  import VisibilityBadge from "./components/VisibilityBadge.svelte";
  import GmDashboard from "./surfaces/GmDashboard.svelte";
  import PlayerView from "./surfaces/PlayerView.svelte";
  import CharacterPanel from "./surfaces/CharacterPanel.svelte";

  let notes = $state<NoteSummary[]>([]);
  let searchQuery = $state<string>("");
  let searchType = $state<string>("");
  let searchFilter = $state<string>("");
  let view = $state<"notes" | "dashboard" | "player" | "characters">("notes");
  let selectedUid = $state<string | null>(null);
  let doc = $state<NoteDoc | null>(null);
  let buffer = $state<string>("");
  let status = $state<string>("");
  let preview = $state<string>("");

  // frontmatter panel
  let fmType = $state<string>("");
  let fmName = $state<string>("");
  let fmVisibility = $state<string>("");
  let fmWorkflow = $state<string>("");
  let fmGrants = $state<string>("");
  let fmCampaign = $state<string>("");
  let fmKind = $state<string>("");
  let fmPlayer = $state<string>("");

  // auth
  let principal = $state<PrincipalClaims | null>(null);
  let loginMode = $state<"gm" | "player" | "join">("gm");
  let gmSecret = $state<string>("");
  let player = $state<string>("");
  let passphrase = $state<string>("");
  let joinCode = $state<string>("");
  let authError = $state<string>("");

  // conflict
  let conflictServer = $state<NoteDoc | null>(null);
  let conflictDiff = $state<string>("");

  let ws: WSClient | null = null;

  async function loadNotes() {
    try {
      notes = await fetchNotes();
      status = "loaded";
    } catch (e: any) {
      if (e?.status === 401) {
        notes = [];
        doc = null;
        buffer = "";
        selectedUid = null;
        status = "login required";
      } else {
        status = "load failed";
      }
    }
  }

  function syncPrincipal() {
    principal = getPrincipal();
  }

  async function doGmLogin() {
    authError = "";
    try {
      await gmLogin(gmSecret);
      gmSecret = "";
      syncPrincipal();
      await loadNotes();
      connectRealtime();
    } catch (e: any) {
      authError = e?.message ?? "login failed";
    }
  }

  async function doPlayerLogin() {
    authError = "";
    try {
      await loginPlayer(player, passphrase);
      syncPrincipal();
      await loadNotes();
      connectRealtime();
    } catch (e: any) {
      authError = e?.message ?? "login failed";
    }
  }

  async function doJoin() {
    authError = "";
    try {
      await joinCampaign(joinCode);
      syncPrincipal();
      await loadNotes();
      connectRealtime();
    } catch (e: any) {
      authError = e?.message ?? "join failed";
    }
  }

  function doLogout() {
    logout();
    ws?.close();
    ws = null;
    principal = null;
    notes = [];
    doc = null;
    status = "login required";
  }

  function connectRealtime() {
    ws?.close();
    ws = connectWS({
      onVaultChanged: () => { void loadNotes(); if (selectedUid) void openNote(selectedUid); },
      onSecretRevealed: () => { if (selectedUid) void openNote(selectedUid); },
      onNoteSaved: (d) => { if (d?.uid === selectedUid) void openNote(selectedUid!); },
    });
  }

  onMount(async () => {
    syncPrincipal();
    if (getToken()) connectRealtime();
    await loadNotes();
  });

  async function doSearch() {
    try {
      notes = await fetchNotes({ q: searchQuery, filter: searchFilter, type: searchType });
    } catch (e) { status = "search failed"; }
  }

  async function openNote(uid: string) {
    selectedUid = uid;
    status = "loading";
    conflictServer = null;
    conflictDiff = "";
    try {
      doc = await fetchNote(uid);
      buffer = doc.content;
      fmType = String((doc as any).type ?? "");
      fmName = doc.name ?? "";
      fmVisibility = String((doc as any).visibility ?? "");
      fmWorkflow = String((doc as any).workflow ?? "");
      fmGrants = Array.isArray((doc as any).grants) ? (doc as any).grants.join(", ") : String((doc as any).grants ?? "");
      fmCampaign = String((doc as any).campaign ?? "");
      fmKind = String((doc as any).kind ?? "");
      fmPlayer = String((doc as any).player ?? "");
      status = "loaded";
      await renderPreview();
    } catch (e: any) {
      status = "open failed";
    }
  }

  async function renderPreview() {
    if (!selectedUid) { preview = ""; return; }
    try {
      preview = await fetchNoteRender(selectedUid);
    } catch {
      preview = "";
    }
  }

  async function save() {
    if (!selectedUid || !doc) return;
    status = "saving";
    conflictServer = null;
    conflictDiff = "";
    try {
      const updated = await saveNote(selectedUid, buffer, doc.hash);
      doc = updated;
      buffer = updated.content;
      status = "saved";
      await renderPreview();
    } catch (e: any) {
      if (e?.status === 409) {
        status = "conflict";
        try {
          const serverDoc = await fetchNote(selectedUid);
          conflictServer = serverDoc;
          conflictDiff = `--- server ---\n${serverDoc.content}\n--- yours ---\n${buffer}`;
        } catch {
          conflictDiff = "conflict";
        }
      } else {
        status = e?.message ?? "save failed";
      }
    }
  }

  async function rebase() {
    if (!conflictServer) return;
    doc = conflictServer;
    buffer = conflictServer.content;
    conflictServer = null;
    conflictDiff = "";
    status = "rebased";
    await renderPreview();
  }

  async function applyFrontmatter() {
    if (!selectedUid) return;
    const fields: Record<string, unknown> = {};
    if (fmType) fields.type = fmType;
    if (fmName) fields.name = fmName;
    if (fmVisibility) fields.visibility = fmVisibility;
    if (fmWorkflow) fields.workflow = fmWorkflow;
    if (fmGrants) fields.grants = fmGrants.split(",").map((s) => s.trim()).filter(Boolean);
    if (fmCampaign) fields.campaign = fmCampaign;
    if (fmKind) fields.kind = fmKind;
    if (fmPlayer) fields.player = fmPlayer;
    try {
      const updated = await patchNote(selectedUid, fields);
      doc = updated;
      buffer = updated.content;
      status = "frontmatter applied";
      await renderPreview();
    } catch (e: any) {
      status = e?.message ?? "frontmatter failed";
    }
  }

  function onBufferChange(v: string) {
    buffer = v;
    void renderPreview();
  }
</script>

<main>
  <h1>Demitable</h1>
  <p role="status">Status: {status}</p>

  <!-- principal switcher / auth -->
  <section class="auth">
    {#if principal}
      <span>Principal: {principal.kind} {principal.id}</span>
      <button type="button" onclick={doLogout}>Logout</button>
    {:else}
      <div class="login-modes">
        <button type="button" onclick={() => (loginMode = "gm")}>GM</button>
        <button type="button" onclick={() => (loginMode = "player")}>Account</button>
        <button type="button" onclick={() => (loginMode = "join")}>Join</button>
      </div>
      {#if loginMode === "gm"}
        <input placeholder="GM secret" bind:value={gmSecret} />
        <button type="button" onclick={doGmLogin}>Login as GM</button>
      {:else if loginMode === "player"}
        <input placeholder="player" bind:value={player} />
        <input placeholder="passphrase" type="password" bind:value={passphrase} />
        <button type="button" onclick={doPlayerLogin}>Login</button>
      {:else}
        <input placeholder="join code" bind:value={joinCode} />
        <button type="button" onclick={doJoin}>Join campaign</button>
      {/if}
      {#if authError}<p class="error">{authError}</p>{/if}
    {/if}
  </section>

  <nav class="views">
    <button onclick={() => (view = "notes")}>Notes</button>
    <button onclick={() => (view = "dashboard")}>Dashboard</button>
    <button onclick={() => (view = "player")}>Player view</button>
    <button onclick={() => (view = "characters")}>Characters</button>
  </nav>

  {#if view === "dashboard"}
    <GmDashboard />
  {:else if view === "player"}
    <PlayerView />
  {:else if view === "characters"}
    <CharacterPanel />
  {:else}
  <div class="layout">
    <aside>
      <h2>Notes</h2>
      <div class="search">
        <input placeholder="q" bind:value={searchQuery} />
        <input placeholder="filter" bind:value={searchFilter} />
        <input placeholder="type" bind:value={searchType} />
        <button type="button" onclick={doSearch}>Search</button>
      </div>
      <ul>
        {#each notes as n}
          <li>
            <button type="button" onclick={() => openNote(String(n.uid))}>
              {n.name ?? n.uid}
            </button>
          </li>
        {/each}
      </ul>
    </aside>

    <section>
      {#if doc}
        <h2>{doc.name} {#if doc.effectiveVisibility}<VisibilityBadge visibility={doc.effectiveVisibility} defaulted={doc.defaulted ?? false} />{/if}</h2>
        <Editor value={buffer} onchange={onBufferChange} />
        <div class="actions">
          <button type="button" onclick={save}>Save</button>
        </div>

        {#if conflictServer}
          <details open>
            <summary>Conflict (409) — rebase offered</summary>
            <pre>{conflictDiff}</pre>
            <button type="button" onclick={rebase}>Rebase to server version</button>
          </details>
        {/if}

        <details>
          <summary>Frontmatter panel</summary>
          <label>Type <input bind:value={fmType} /></label>
          <label>Name <input bind:value={fmName} /></label>
          <label>Visibility <input bind:value={fmVisibility} /></label>
          <label>Workflow <input bind:value={fmWorkflow} /></label>
          <label>Grants <input bind:value={fmGrants} /></label>
          <label>Campaign <input bind:value={fmCampaign} /></label>
          <label>Kind <input bind:value={fmKind} /></label>
          <label>Player <input bind:value={fmPlayer} /></label>
          <button type="button" onclick={applyFrontmatter}>Apply</button>
        </details>

        <details>
          <summary>Live preview (server-projected)</summary>
          <div class="preview">{@html preview}</div>
        </details>
      {:else}
        <p>Select a note.</p>
      {/if}
    </section>
  </div>
  {/if}
</main>

<style>
  main {
    font-family: var(--font-sans, system-ui);
    color: var(--fg-default);
    background: var(--bg-canvas);
    padding: var(--space-4);
    min-height: 100vh;
  }
  .layout {
    display: grid;
    grid-template-columns: 240px 1fr;
    gap: var(--space-4);
  }
  aside ul { list-style: none; padding: 0; }
  aside button { width: 100%; text-align: left; }
  .actions { margin: var(--space-2) 0; }
  input { margin-left: var(--space-2); }
  pre { white-space: pre-wrap; }
  .error { color: var(--danger, #c00); }
  .auth { margin-bottom: var(--space-3); display: flex; gap: var(--space-2); align-items: center; flex-wrap: wrap; }
  .login-modes { display: flex; gap: var(--space-1); }
  .preview { padding: var(--space-2); border: 1px solid var(--border-subtle, #ccc); }
</style>
