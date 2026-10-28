<script lang="ts">
  // Left sidebar: navigation plus the campaign context, in collapsible
  // sections. Collapses to an icon rail; each section remembers its own
  // collapsed state.
  import { appState } from "../lib/appstate.svelte";
  import { vaultStore } from "../lib/vaultstore.svelte";
  import VisibilityBadge from "./VisibilityBadge.svelte";
  import VaultTree from "./VaultTree.svelte";
  import SideSection from "./SideSection.svelte";
  import Icon from "./Icon.svelte";
  import { displayName } from "../lib/display";

  const collapsed = $derived(appState.sidebarCollapsed);

  function openNote(uid: string) {
    appState.selectedUid = uid;
    appState.go("wiki", "viewer");
  }

  function onBestiaryDrag(uid: string, ev: DragEvent) {
    ev.dataTransfer?.setData("application/x-demitable-note", uid);
    ev.dataTransfer!.effectAllowed = "copy";
  }
</script>

<aside class="sidebar" class:collapsed aria-label="Navigation and campaign">
  <button class="collapse" onclick={() => appState.toggleSidebar()} title={collapsed ? "Expand sidebar" : "Collapse to rail"} aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}>
    <Icon name={collapsed ? "chevron-right" : "chevron-down"} size={14} />
  </button>

  {#if !collapsed}
  {#if appState.group === "wiki"}
    <SideSection id="wiki-views" title="Wiki">
      <ul class="side-list">
        {#each [["home", "Home", "scroll"], ["explorer", "Vault explorer", "list"], ["editor", "Editor", "pencil"], ["player", "Player view", "user"], ["chat", "Chat", "note"]] as [id, label, icon]}
          <li>
            <button class="side-item" class:is-active={appState.wikiView === id} onclick={() => (appState.wikiView = id as any)}>
              <Icon name={icon as any} size={14} /><span>{label}</span>
            </button>
          </li>
        {/each}
      </ul>
    </SideSection>

    <SideSection id="wiki-campaigns" title="Campaigns">
      <ul class="side-list">
        <li>
          <button class="side-item" class:is-active={appState.campaignFilter === null} onclick={() => (appState.campaignFilter = null)}>
            <Icon name="globe" size={14} /><span class="side-item__name">All campaigns</span>
          </button>
        </li>
        {#each vaultStore.campaigns as [name, count]}
          <li>
            <button class="side-item" class:is-active={appState.campaignFilter === name} onclick={() => (appState.campaignFilter = appState.campaignFilter === name ? null : name)} title="Filter the browser to this campaign">
              <Icon name="scroll" size={14} /><span class="side-item__name">{displayName(name)}</span><span class="side-count">{count}</span>
            </button>
          </li>
        {/each}
      </ul>
    </SideSection>

    <SideSection id="wiki-vault" title="Vault">
      <VaultTree notes={vaultStore.notes} />
    </SideSection>

    <SideSection id="wiki-attention" title="Needs attention" count={vaultStore.attention.length}>
      {#if vaultStore.attention.length === 0}
        <p class="side-note">Nothing needs attention.</p>
      {:else}
        <ul class="side-list">
          {#each vaultStore.attention as n (n.uid)}
            <li>
              <button class="side-item side-item--warn" onclick={() => openNote(String(n.uid))}>
                <Icon name="warning" size={14} /><span class="side-item__name">{String(n.name ?? n.uid)}</span>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </SideSection>

    <SideSection id="wiki-coverage" title="What the table can see">
      <dl class="counts">
        <div class="counts__row"><dt><VisibilityBadge visibility="party" size="sm" /></dt><dd>{vaultStore.counts.party} of {vaultStore.counts.total} notes</dd></div>
        <div class="counts__row"><dt><VisibilityBadge visibility="gm" size="sm" /></dt><dd>{vaultStore.counts.total - vaultStore.counts.party} hidden</dd></div>
      </dl>
      <p class="side-note">Names of hidden notes stay here — a player never sees this panel.</p>
    </SideSection>
  {:else if appState.group === "vtt"}
    <SideSection id="vtt-session" title="Session">
      <label class="field">Session <input bind:value={appState.sessionId} placeholder="sid" aria-label="Session id" /></label>
      <label class="field">Encounter <input bind:value={appState.encounterId} placeholder="eid" aria-label="Encounter id" /></label>
      <label class="check"><input type="checkbox" bind:checked={appState.displayMode} /> display mode</label>
      <p class="side-note">Party-only shared screen. Tokens are intents; the server is authoritative.</p>
    </SideSection>

    <SideSection id="vtt-bestiary" title="Bestiary" count={vaultStore.characters.length}>
      <p class="side-note">Drag a character onto the board to seat it.</p>
      <ul class="side-list">
        {#each vaultStore.characters as c (c.uid)}
          <li>
            <span
              class="side-item draggable" role="button" tabindex="0" draggable="true"
              ondragstart={(e) => onBestiaryDrag(String(c.uid), e)}
              title="Drag onto the board"
            ><Icon name="user" size={14} /><span class="side-item__name">{String(c.name ?? c.uid)}</span></span>
          </li>
        {/each}
      </ul>
    </SideSection>
  {:else}
    <SideSection id="creator-chars" title="Characters" count={vaultStore.characters.length}>
      <ul class="side-list">
        {#each vaultStore.characters as c (c.uid)}
          <li>
            <button class="side-item" class:is-active={appState.selectedCharacterUid === String(c.uid)} onclick={() => (appState.selectedCharacterUid = String(c.uid))}>
              <Icon name="user" size={14} /><span class="side-item__name">{String(c.name ?? c.uid)}</span>
            </button>
          </li>
        {/each}
      </ul>
      <p class="side-note">Sheets render from the pack; the projection removes what a principal may not see.</p>
    </SideSection>

    {#if vaultStore.reviewQueue.length > 0}
      <SideSection id="creator-review" title="Review queue" count={vaultStore.reviewQueue.length}>
        <ul class="side-list">
          {#each vaultStore.reviewQueue as c (c.uid)}
            <li>
              <button class="side-item side-item--warn" onclick={() => (appState.selectedCharacterUid = String(c.uid))}>
                <Icon name="warning" size={14} /><span class="side-item__name">{String(c.name ?? c.uid)}</span>
                <span class="tag">{String((c as any).workflow)}</span>
              </button>
            </li>
          {/each}
        </ul>
      </SideSection>
    {/if}
  {/if}
  {/if}
</aside>

<style>
  .sidebar {
    width: var(--sidebar-width);
    min-height: 0;
    border-inline-end: 1px solid var(--border-subtle);
    background: var(--bg-surface-2);
    padding: var(--space-3) var(--space-4) var(--space-4);
    overflow-y: auto;
    overflow-x: clip;
    display: flex; flex-direction: column; gap: var(--space-1);
    transition: width var(--duration-base);
  }
  .sidebar.collapsed { width: 3rem; padding: var(--space-3) var(--space-2); align-items: center; }
  .collapse { align-self: flex-end; background: none; border: 1px solid transparent; border-radius: var(--radius-md); cursor: pointer; color: var(--fg-muted); padding: var(--space-1); }
  .collapse:hover { background: var(--bg-surface-3); }
  .collapsed .collapse { align-self: center; }
  @media (max-width: 900px) {
    .collapse { display: none; }
  }
  .side-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 2px; }
  .side-item { display: flex; align-items: center; gap: var(--space-2); width: 100%; padding: var(--space-2); border: 1px solid transparent; border-radius: var(--radius-md); background: none; font-size: var(--text-sm); text-align: start; color: var(--fg-default); }
  button.side-item { cursor: pointer; }
  button.side-item:hover { background: var(--bg-surface-3); }
  .side-item.is-active { background: var(--bg-surface); border-color: var(--border-default); box-shadow: inset 3px 0 0 var(--accent); font-weight: var(--weight-medium); }
  .side-item--warn { color: var(--fg-muted); }
  .side-item__name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .side-count { margin-inline-start: auto; font-size: var(--text-xs); color: var(--fg-subtle); }
  .side-note { font-size: var(--text-xs); color: var(--fg-muted); margin-block: var(--space-2); }
  .counts { margin: 0; display: flex; flex-direction: column; gap: var(--space-2); }
  .counts__row { display: flex; align-items: center; justify-content: space-between; gap: var(--space-2); }
  .counts dt, .counts dd { margin: 0; }
  .counts dd { font-size: var(--text-xs); color: var(--fg-muted); }
  .draggable { cursor: grab; }
  .field { display: flex; flex-direction: column; gap: 2px; font-size: var(--text-xs); color: var(--fg-muted); margin-block-end: var(--space-2); }
  .field input { padding: var(--space-1) var(--space-2); border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--bg-surface); color: var(--fg-default); font-size: var(--text-sm); }
  .check { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-xs); color: var(--fg-muted); }
</style>
