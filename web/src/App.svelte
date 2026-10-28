<script lang="ts">
  import { onMount } from "svelte";
  import { fetchNotes, rollDice, type NoteSummary } from "./lib/api";
  import { getToken, getPrincipal, gmLogin, loginPlayer, joinCampaign, logout } from "./lib/auth";
  import { WSClient, type WSClient as WSClientType } from "./lib/ws";
  import { appState, applyThemeProof, type Surface, type Proof } from "./lib/appstate.svelte";
  import Icon from "./components/Icon.svelte";
  import VisibilityBadge from "./components/VisibilityBadge.svelte";
  import JournalDrawer from "./components/JournalDrawer.svelte";
  import GmDashboard from "./surfaces/GmDashboard.svelte";
  import NoteEditor from "./surfaces/NoteEditor.svelte";
  import CharacterPanel from "./surfaces/CharacterPanel.svelte";
  import PlayerView from "./surfaces/PlayerView.svelte";
  import Tabletop from "./surfaces/Tabletop.svelte";
  import Chat from "./surfaces/Chat.svelte";
  import { surfaceVisibility } from "./lib/dashboard";

  let principal = $state(getPrincipal());
  let loginMode = $state<"gm" | "player" | "join">("gm");
  let gmSecret = $state("");
  let player = $state("");
  let passphrase = $state("");
  let joinCode = $state("");
  let authError = $state("");

  let searchQuery = $state("");
  let searchType = $state("");
  let searchVis = $state("");
  let results = $state<NoteSummary[]>([]);
  let searchStatus = $state("");
  let rollStatus = $state("");
  let diceLog = $state<any[]>([]);

  let ws: WSClientType | null = null;

  const surfaces: { id: Surface; label: string }[] = [
    { id: "dashboard", label: "Dashboard" },
    { id: "editor", label: "Editor" },
    { id: "characters", label: "Characters" },
    { id: "player", label: "Player view" },
    { id: "tabletop", label: "Tabletop" },
    { id: "chat", label: "Chat" },
  ];
  const proofs: Proof[] = ["none", "greyscale", "flat-hue", "protanopia", "deuteranopia", "tritanopia"];

  function syncPrincipal() { principal = getPrincipal(); }

  async function doGmLogin() {
    authError = "";
    try { await gmLogin(gmSecret); gmSecret = ""; syncPrincipal(); connectRealtime(); await doSearch(); }
    catch (e: any) { authError = e?.message ?? "login failed"; }
  }
  async function doPlayerLogin() {
    authError = "";
    try { await loginPlayer(player, passphrase); syncPrincipal(); connectRealtime(); await doSearch(); }
    catch (e: any) { authError = e?.message ?? "login failed"; }
  }
  async function doJoin() {
    authError = "";
    try { await joinCampaign(joinCode); syncPrincipal(); connectRealtime(); await doSearch(); }
    catch (e: any) { authError = e?.message ?? "join failed"; }
  }
  function doLogout() {
    logout(); ws?.close(); ws = null; principal = null; results = [];
  }
  function connectRealtime() {
    ws?.close();
    ws = new WSClient({
      onVaultChanged: () => void doSearch(),
      onNoteSaved: (d) => { if (d?.uid === appState.selectedUid) void doSearch(); },
      onSecretRevealed: () => void doSearch(),
      onRollResult: (d) => { diceLog = [...diceLog, d].slice(-20); },
    });
    ws.connect();
  }
  async function doSearch() {
    searchStatus = "searching";
    try {
      results = await fetchNotes({ q: searchQuery, type: searchType } as any);
      if (searchVis) results = results.filter((n) => { try { return surfaceVisibility(n) === searchVis; } catch { return false; } });
      searchStatus = `${results.length} notes`;
    } catch (e: any) { searchStatus = e?.message ?? "search failed"; }
  }
  function openNote(uid: string) {
    appState.selectedUid = uid;
    appState.surface = "editor";
  }
  async function doRoll() {
    rollStatus = "rolling";
    try {
      const r = await rollDice({ workflow: "attack", shared: true });
      diceLog = [...diceLog, r?.roll ?? r].slice(-20);
      rollStatus = "logged to shared log";
    } catch (e: any) { rollStatus = e?.message ?? "roll failed"; }
  }

  onMount(() => {
    applyThemeProof();
    syncPrincipal();
    if (getToken()) { connectRealtime(); void doSearch(); }
  });
  $effect(() => { applyThemeProof(); void appState.theme; void appState.proof; });
</script>

<div id="shell">
  <header class="topbar">
    <div class="brand"><span class="brand__mark" aria-hidden="true">❖</span><h1 class="brand__name">Demitable</h1></div>
    <nav class="surfaces" aria-label="Surfaces">
      {#each surfaces as s}
        <button class="btn btn--sm" class:is-active={appState.surface === s.id} aria-pressed={appState.surface === s.id} onclick={() => (appState.surface = s.id)}>{s.label}</button>
      {/each}
    </nav>
    <div class="topbar__tools">
      <label class="tool">Theme
        <select bind:value={appState.theme} aria-label="Theme">
          <option value="light">Light</option>
          <option value="dark">Dark</option>
        </select>
      </label>
      <label class="tool">Proof
        <select bind:value={appState.proof} aria-label="Colour proof">
          {#each proofs as p}<option value={p}>{p}</option>{/each}
        </select>
      </label>
      {#if principal}
        <span class="tag principal-chip"><Icon name={principal.kind === "gm" ? "shield" : "user"} size={11} /> {principal.kind} {principal.id}</span>
        <button class="btn btn--sm" onclick={doLogout}>Logout</button>
      {/if}
    </div>
  </header>

  {#if !principal}
    <section class="auth card" aria-label="Login">
      <div class="login-modes" role="tablist" aria-label="Login mode">
        <button class="btn btn--sm" class:is-active={loginMode === "gm"} onclick={() => (loginMode = "gm")}>GM</button>
        <button class="btn btn--sm" class:is-active={loginMode === "player"} onclick={() => (loginMode = "player")}>Account</button>
        <button class="btn btn--sm" class:is-active={loginMode === "join"} onclick={() => (loginMode = "join")}>Join</button>
      </div>
      {#if loginMode === "gm"}
        <input placeholder="GM secret" type="password" bind:value={gmSecret} aria-label="GM secret" />
        <button class="btn btn--sm" onclick={doGmLogin}>Login as GM</button>
      {:else if loginMode === "player"}
        <input placeholder="player" bind:value={player} aria-label="Player id" />
        <input placeholder="passphrase" type="password" bind:value={passphrase} aria-label="Passphrase" />
        <button class="btn btn--sm" onclick={doPlayerLogin}>Login</button>
      {:else}
        <input placeholder="join code" bind:value={joinCode} aria-label="Join code" />
        <button class="btn btn--sm" onclick={doJoin}>Join campaign</button>
      {/if}
      {#if authError}<p class="error">{authError}</p>{/if}
    </section>
  {:else}
    <div class="workbench">
      <section class="searchbar" aria-label="Search">
        <input placeholder="Search names and prose…" bind:value={searchQuery} onkeydown={(e) => e.key === "Enter" && doSearch()} aria-label="Search" />
        <input placeholder="type" bind:value={searchType} aria-label="Type filter" />
        <select bind:value={searchVis} aria-label="Visibility filter">
          <option value="">all vis</option>
          <option value="party">party</option>
          <option value="gm">gm</option>
          <option value="owner">owner</option>
          <option value="private">private</option>
        </select>
        <button class="btn btn--sm" onclick={doSearch}>Search</button>
        <span class="mono search-status" role="status">{searchStatus}</span>
        <label class="tool"><input type="checkbox" bind:checked={appState.playerEye} /> player eye</label>
        <label class="tool"><input type="checkbox" bind:checked={appState.showInspector} /> inspector</label>
        <button class="btn btn--sm" onclick={() => (appState.showJournal = !appState.showJournal)}>{appState.showJournal ? "Hide journal" : "Journal"}</button>
        <button class="btn btn--sm" onclick={doRoll} title="POST /roll into the shared log"><Icon name="dice" size={13} /> Roll</button>
        <span class="mono search-status">{rollStatus}</span>
      </section>

      {#if appState.surface !== "dashboard" && appState.surface !== "tabletop" && results.length > 0}
        <ul class="quick-results">
          {#each results.slice(0, 8) as n (n.uid)}
            <li>
              <button class="qr" onclick={() => openNote(String(n.uid))}>
                <span>{String(n.name ?? n.uid)}</span>
              </button>
            </li>
          {/each}
        </ul>
      {/if}

      <div class="mainrow">
        <main class="main">
          {#if appState.surface === "dashboard"}
            <GmDashboard />
          {:else if appState.surface === "editor"}
            <NoteEditor />
          {:else if appState.surface === "characters"}
            <CharacterPanel />
          {:else if appState.surface === "player"}
            <PlayerView />
          {:else if appState.surface === "tabletop"}
            <Tabletop />
          {:else if appState.surface === "chat"}
            <Chat />
          {/if}
          {#if diceLog.length > 0}
            <section class="dicelog card" aria-label="Shared dice log">
              <h3><Icon name="dice" size={14} /> Shared dice log</h3>
              <ol>
                {#each diceLog.slice().reverse() as r, i (i)}
                  <li class="mono">{JSON.stringify(r).slice(0, 160)}</li>
                {/each}
              </ol>
            </section>
          {/if}
        </main>
        {#if appState.showJournal}
          <JournalDrawer open={true} onclose={() => (appState.showJournal = false)} />
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  #shell { min-height: 100vh; background: var(--bg-canvas); color: var(--fg-default); font-family: var(--font-sans, system-ui); display: flex; flex-direction: column; }
  .topbar { display: flex; align-items: center; gap: var(--space-4); min-height: var(--topbar-height); padding: var(--space-2) var(--space-4); background: var(--bg-surface); border-block-end: 1px solid var(--border-subtle); position: sticky; top: 0; z-index: var(--z-sticky); flex-wrap: wrap; }
  .brand { display: flex; align-items: center; gap: var(--space-2); }
  .brand__mark { color: var(--accent); font-size: var(--text-md); }
  .brand__name { font-size: var(--text-md); font-weight: var(--weight-bold); }
  .surfaces { display: flex; gap: var(--space-1); flex-wrap: wrap; }
  .topbar__tools { margin-inline-start: auto; display: flex; gap: var(--space-2); align-items: center; flex-wrap: wrap; }
  .tool { display: flex; align-items: center; gap: var(--space-1); font-size: var(--text-xs); color: var(--fg-muted); }
  .tool select, .tool input[type="text"] { font-size: var(--text-xs); }
  .principal-chip { font-size: var(--text-xs); }
  .auth { max-width: 28rem; margin: var(--space-8) auto; padding: var(--space-4); display: flex; flex-direction: column; gap: var(--space-2); }
  .login-modes { display: flex; gap: var(--space-1); }
  .auth input { padding: var(--space-2); border: 1px solid var(--border-default); border-radius: var(--radius-md); background: var(--bg-surface); color: var(--fg-default); }
  .error { color: var(--danger, #c00); font-size: var(--text-sm); }
  .workbench { display: flex; flex-direction: column; flex: 1; min-height: 0; }
  .searchbar { display: flex; gap: var(--space-2); align-items: center; padding: var(--space-2) var(--space-4); background: var(--bg-surface-2); border-block-end: 1px solid var(--border-subtle); flex-wrap: wrap; }
  .searchbar input, .searchbar select { padding: var(--space-1) var(--space-2); border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--bg-surface); color: var(--fg-default); font-size: var(--text-sm); }
  .search-status { font-size: var(--text-2xs); color: var(--fg-subtle); }
  .quick-results { list-style: none; display: flex; gap: var(--space-1); margin: 0; padding: var(--space-2) var(--space-4); overflow-x: auto; background: var(--bg-surface); border-block-end: 1px solid var(--border-subtle); }
  .qr { border: 1px solid var(--border-default); background: var(--bg-surface-2); border-radius: var(--radius-pill); padding: 0 var(--space-2); font-size: var(--text-xs); cursor: pointer; white-space: nowrap; }
  .mainrow { display: flex; flex: 1; min-height: 0; }
  .main { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .dicelog { margin: var(--space-4); padding: var(--space-3); }
  .dicelog h3 { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-sm); margin-block-end: var(--space-2); }
  .dicelog ol { margin: 0; padding-inline-start: var(--space-4); font-size: var(--text-xs); }
</style>
