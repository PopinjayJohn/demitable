<script lang="ts">
  // The global frame: header (left / center / right), collapsible left
  // sidebar, center surface, contextual right rail, footer status strip.
  // Surfaces own their data; the sidebar and rail read shared stores.
  import { onMount } from "svelte";
  import { fetchNotes, fetchPlayers, createPlayer, rollDice, type NoteSummary, type PlayerAccount } from "./lib/api";
  import { getToken, getPrincipal, gmLogin, loginPlayer, joinCampaign, logout } from "./lib/auth";
  import { WSClient, type WSClient as WSClientType } from "./lib/ws";
  import { vaultStore } from "./lib/vaultstore.svelte";
  import { appState, applyThemeProof, type Group, type Proof } from "./lib/appstate.svelte";
  import Icon from "./components/Icon.svelte";
  import Sidebar from "./components/Sidebar.svelte";
  import Rail from "./components/Rail.svelte";
  import Footer from "./components/Footer.svelte";
  import SessionButton from "./components/SessionButton.svelte";
  import SessionJournal from "./components/SessionJournal.svelte";
  import JournalDrawer from "./components/JournalDrawer.svelte";
  import Browser from "./surfaces/Browser.svelte";
  import NoteEditor from "./surfaces/NoteEditor.svelte";
  import CharacterPanel from "./surfaces/CharacterPanel.svelte";
  import PlayerView from "./surfaces/PlayerView.svelte";
  import Tabletop from "./surfaces/Tabletop.svelte";
  import Chat from "./surfaces/Chat.svelte";

  let principal = $state(getPrincipal());
  let loginMode = $state<"gm" | "player" | "join">("gm");
  let gmSecret = $state("");
  let player = $state("");
  let passphrase = $state("");
  let joinCode = $state("");
  let authError = $state("");

  let searchQuery = $state("");
  let results = $state<NoteSummary[]>([]);
  let searchStatus = $state("");
  let showResults = $state(false);

  let accounts = $state<PlayerAccount[]>([]);
  let newPlayerName = $state("");
  let newPlayerCampaign = $state("");
  let showNewPlayer = $state(false);

  let ws: WSClientType | null = null;

  const groups: { id: Group; label: string }[] = [
    { id: "wiki", label: "Wiki" },
    { id: "vtt", label: "VTT" },
    { id: "creator", label: "Character Creator" },
  ];
  const proofs: Proof[] = ["none", "greyscale", "flat-hue", "protanopia", "deuteranopia", "tritanopia"];
  const eyeCandidates = $derived(accounts.map((a) => a.id).sort());
  const campaigns = $derived(
    [...new Set(results.map((n) => String((n as any).campaign || "No campaign")))].sort(),
  );
  const eyeAs = $derived(appState.playerEye && appState.eyeTarget ? appState.eyeTarget : "");

  function syncPrincipal() { principal = getPrincipal(); }

  async function loadAccounts() {
    accounts = [];
    if (principal?.kind !== "gm") return;
    try { accounts = await fetchPlayers(); }
    catch { accounts = []; }
  }

  async function afterAuth() {
    syncPrincipal();
    connectRealtime();
    await loadAccounts();
    await vaultStore.load();
    await doSearch();
  }
  function doLogout() {
    logout(); ws?.close(); ws = null; principal = null; results = []; accounts = [];
    appState.wsConnected = false;
  }

  async function doGmLogin() {
    authError = "";
    try { await gmLogin(gmSecret); gmSecret = ""; await afterAuth(); }
    catch (e: any) { authError = e?.message ?? "login failed"; }
  }
  async function doPlayerLogin() {
    authError = "";
    try { await loginPlayer(player, passphrase); await afterAuth(); }
    catch (e: any) { authError = e?.message ?? "login failed"; }
  }
  async function doJoin() {
    authError = "";
    try { await joinCampaign(joinCode); await afterAuth(); }
    catch (e: any) { authError = e?.message ?? "join failed"; }
  }
  async function doCreatePlayer() {
    if (!newPlayerName.trim()) return;
    try {
      const campaign = newPlayerCampaign || campaigns[0] || "";
      const r = await createPlayer(newPlayerName.trim(), campaign);
      searchStatus = `join code for ${r.name} (${r.campaign}): ${r.join_code} (expires ${r.expires_at})`;
      newPlayerName = "";
      showNewPlayer = false;
      await loadAccounts();
    } catch (e: any) { searchStatus = e?.message ?? "create player failed"; }
  }

  function toggleEye(on: boolean) {
    appState.playerEye = on;
    if (on && !appState.eyeTarget && eyeCandidates.length > 0) {
      appState.eyeTarget = eyeCandidates[0];
    }
  }

  function connectRealtime() {
    ws?.close();
    ws = new WSClient({
      onOpen: () => (appState.wsConnected = true),
      onClose: () => (appState.wsConnected = false),
      // Reconnect (or first connect) re-syncs the shared projection:
      // WS events are not queued, so anything missed while away is
      // picked up here.
      onHello: () => { void vaultStore.load(eyeAs); },
      onVaultChanged: () => { void vaultStore.load(eyeAs); void doSearch(); },
      onNoteSaved: () => { void vaultStore.load(eyeAs); void doSearch(); },
      onSecretRevealed: () => { void vaultStore.load(eyeAs); void doSearch(); },
      onRollResult: (d) => { appState.diceLog = [...appState.diceLog, d].slice(-20); },
    });
    ws.connect();
  }

  async function doSearch() {
    searchStatus = "searching";
    try {
      results = await fetchNotes({ q: searchQuery } as any);
      searchStatus = `${results.length} notes`;
      showResults = searchQuery.trim() !== "" && results.length > 0;
    } catch (e: any) { searchStatus = e?.message ?? "search failed"; }
  }

  function openNote(uid: string) {
    appState.selectedUid = uid;
    appState.go("wiki", "browser");
    showResults = false;
  }

  async function doRoll() {
    appState.rollStatus = "rolling";
    try {
      const r = await rollDice({ workflow: "attack", shared: true });
      appState.diceLog = [...appState.diceLog, r?.roll ?? r].slice(-20);
      appState.rollStatus = "logged to shared log";
    } catch (e: any) { appState.rollStatus = e?.message ?? "roll failed"; }
  }

  // The eye reloads the shared vault projection.
  let lastEye = $state("");
  $effect(() => {
    const cur = `${appState.playerEye}|${appState.eyeTarget}`;
    if (cur === lastEye) return;
    lastEye = cur;
    if (principal) void vaultStore.load(eyeAs);
    if (appState.playerEye && !appState.eyeTarget && eyeCandidates.length > 0) {
      appState.eyeTarget = eyeCandidates[0];
    }
  });

  onMount(() => {
    applyThemeProof();
    syncPrincipal();
    lastEye = `${appState.playerEye}|${appState.eyeTarget}`;
    if (getToken()) {
      connectRealtime();
      void loadAccounts();
      void vaultStore.load();
      void doSearch();
    }
  });
  $effect(() => { applyThemeProof(); void appState.theme; void appState.proof; });
</script>

<div id="shell">
  <header class="topbar">
    <div class="topbar__left">
      <button class="iconbtn header-toggle" onclick={() => appState.toggleSidebar()} title="Toggle sidebar" aria-label="Toggle sidebar">
        <Icon name={appState.sidebarCollapsed ? "chevron-right" : "chevron-down"} size={16} />
      </button>
      <span class="brand"><span class="brand__mark" aria-hidden="true">❖</span><h1 class="brand__name">Demitable</h1></span>
      {#if principal}
        <nav class="groups" aria-label="Groups">
          {#each groups as g}
            <button class="btn btn--sm" class:is-active={appState.group === g.id} aria-pressed={appState.group === g.id} onclick={() => appState.go(g.id)}>{g.label}</button>
          {/each}
        </nav>
      {/if}
    </div>

    {#if principal}
      <div class="topbar__center">
        <input placeholder="Search names and prose…" bind:value={searchQuery} onkeydown={(e) => e.key === "Enter" && doSearch()} aria-label="Search" />
        <button class="btn btn--sm" onclick={doSearch}>Search</button>
        <label class="tool">eye <input type="checkbox" checked={appState.playerEye} onchange={(e) => toggleEye(e.currentTarget.checked)} aria-label="Player eye" /></label>
        <select bind:value={appState.eyeTarget} aria-label="Player-eye target" title="Account to borrow eyes from (GM preview-as)">
          <option value="">eye: —</option>
          {#each eyeCandidates as id}
            <option value={id}>eye: {id}</option>
          {/each}
        </select>
      </div>

      <div class="topbar__right">
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
        {#if principal.kind === "gm"}
          <SessionButton />
          <button class="btn btn--sm" onclick={() => (showNewPlayer = !showNewPlayer)} title="Create a player account (join code)">+ player</button>
        {/if}
        <button class="btn btn--sm" onclick={() => (appState.showSessionJournal = true)}>Journal</button>
        <button class="btn btn--sm" onclick={doRoll} title="POST /roll into the shared log"><Icon name="dice" size={13} /> Roll</button>
        <span class="tag principal-chip"><Icon name={principal.kind === "gm" ? "shield" : "user"} size={11} /> {principal.kind} {principal.id}</span>
        <button class="btn btn--sm" onclick={doLogout}>Logout</button>
      </div>
    {/if}
  </header>

  {#if showNewPlayer && principal?.kind === "gm"}
    <div class="playerbar">
      <input placeholder="player name" bind:value={newPlayerName} aria-label="New player name" />
      <select bind:value={newPlayerCampaign} aria-label="New player campaign">
        <option value="">campaign: auto</option>
        {#each campaigns as c}
          <option value={c}>{c}</option>
        {/each}
      </select>
      <button class="btn btn--sm" onclick={doCreatePlayer}>Mint join code</button>
      <span class="mono playerbar__status" role="status">{searchStatus}</span>
    </div>
  {/if}

  {#if showResults && results.length > 0}
    <ul class="results" role="listbox" aria-label="Search results">
      {#each results.slice(0, 8) as n (n.uid)}
        <li><button class="result" onclick={() => openNote(String(n.uid))}>{String(n.name ?? n.uid)}</button></li>
      {/each}
    </ul>
  {/if}

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
      <Sidebar />
      <main class="main">
        {#if appState.group === "wiki"}
          {#if appState.wikiView === "browser"}
            <Browser />
          {:else if appState.wikiView === "editor"}
            <NoteEditor />
          {:else if appState.wikiView === "player"}
            <PlayerView />
          {:else}
            <Chat />
          {/if}
        {:else if appState.group === "vtt"}
          <Tabletop />
        {:else}
          <CharacterPanel />
        {/if}
      </main>
      <Rail />
    </div>
    <Footer />
  {/if}

  {#if appState.showSessionJournal && principal}
    <SessionJournal />
  {/if}

  {#if appState.showJournal && principal}
    <div class="journal-overlay">
      <JournalDrawer open={true} onclose={() => (appState.showJournal = false)} />
    </div>
  {/if}
</div>

<style>
  #shell { height: 100dvh; background: var(--bg-canvas); color: var(--fg-default); font-family: var(--font-sans, system-ui); display: flex; flex-direction: column; overflow: hidden; }
  .topbar { display: grid; grid-template-columns: auto 1fr auto; align-items: center; gap: var(--space-4); min-height: var(--topbar-height); padding: var(--space-2) var(--space-4); background: var(--bg-surface); border-block-end: 1px solid var(--border-subtle); position: sticky; top: 0; z-index: var(--z-sticky); }
  .topbar__left { display: flex; align-items: center; gap: var(--space-2); min-width: 0; }
  .topbar__center { display: flex; align-items: center; gap: var(--space-2); justify-content: center; position: relative; min-width: 0; }
  .topbar__right { display: flex; align-items: center; gap: var(--space-2); margin-inline-start: auto; flex-wrap: wrap; justify-content: flex-end; }
  .iconbtn { background: none; border: 1px solid transparent; border-radius: var(--radius-md); cursor: pointer; color: var(--fg-muted); padding: var(--space-1); display: inline-flex; }
  .iconbtn:hover { background: var(--bg-surface-3); }
  /* One toggle per viewport: sidebar owns it on desktop, header on mobile. */
  .header-toggle { display: none; }
  @media (max-width: 900px) {
    .header-toggle { display: inline-flex; }
  }
  .brand { display: flex; align-items: center; gap: var(--space-2); }
  .brand__mark { color: var(--accent); font-size: var(--text-md); }
  .brand__name { font-size: var(--text-md); font-weight: var(--weight-bold); white-space: nowrap; }
  .groups { display: flex; gap: var(--space-1); margin-inline-start: var(--space-2); }
  .topbar__center input:not([type="checkbox"]) { width: min(22rem, 30vw); padding: var(--space-1) var(--space-2); border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--bg-surface); color: var(--fg-default); font-size: var(--text-sm); }
  .topbar__center input[type="checkbox"] { width: 1rem; height: 1rem; }
  .tool { display: flex; align-items: center; gap: var(--space-1); font-size: var(--text-xs); color: var(--fg-muted); white-space: nowrap; }
  .tool select { font-size: var(--text-xs); }
  .principal-chip { font-size: var(--text-xs); }
  .results {
    position: fixed; inset-block-start: calc(var(--topbar-height) + 4px);
    inset-inline-start: 50%; transform: translateX(-50%);
    width: min(30rem, 90vw); max-height: 50vh; overflow-y: auto;
    list-style: none; padding: var(--space-1); margin: 0;
    background: var(--bg-surface); border: 1px solid var(--border-default);
    border-radius: var(--radius-md); box-shadow: var(--shadow-3);
    z-index: var(--z-dropdown);
  }
  .result { width: 100%; text-align: start; background: none; border: none; cursor: pointer; padding: var(--space-1) var(--space-2); border-radius: var(--radius-sm); font-size: var(--text-sm); color: var(--fg-default); }
  .result:hover { background: var(--bg-surface-2); }
  .playerbar { display: flex; align-items: center; gap: var(--space-2); padding: var(--space-2) var(--space-4); background: var(--bg-surface-2); border-block-end: 1px solid var(--border-subtle); }
  .playerbar input, .playerbar select { padding: var(--space-1) var(--space-2); border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--bg-surface); color: var(--fg-default); font-size: var(--text-sm); }
  .playerbar__status { font-size: var(--text-2xs); color: var(--fg-subtle); }
  .auth { max-width: 28rem; margin: var(--space-8) auto; padding: var(--space-4); display: flex; flex-direction: column; gap: var(--space-2); overflow-y: auto; }
  .login-modes { display: flex; gap: var(--space-1); }
  .auth input { padding: var(--space-2); border: 1px solid var(--border-default); border-radius: var(--radius-md); background: var(--bg-surface); color: var(--fg-default); }
  .error { color: var(--danger, #c00); font-size: var(--text-sm); }
  .workbench { display: flex; flex: 1; min-height: 0; overflow: hidden; }
  .main { flex: 1; min-width: 0; min-height: 0; display: flex; flex-direction: column; overflow: hidden; position: relative; }
  .journal-overlay { position: fixed; inset-block: var(--topbar-height) 0; inset-inline-end: 0; z-index: var(--z-modal); display: flex; box-shadow: var(--shadow-3); }
  @media (max-width: 1100px) {
    .topbar { grid-template-columns: auto 1fr; }
    .topbar__right { grid-column: 1 / -1; justify-content: flex-start; }
  }
  @media (max-width: 900px) {
    .topbar__center input { width: 10rem; }
    .brand__name { display: none; }
  }
</style>
