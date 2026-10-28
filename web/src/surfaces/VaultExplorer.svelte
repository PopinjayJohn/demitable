<script lang="ts">
  // Vault explorer: the full note list with expanded filtering and
  // sorting. Selecting a note opens it in the viewer; the floating
  // button edits the selection.
  import { vaultStore } from "../lib/vaultstore.svelte";
  import { surfaceVisibility } from "../lib/dashboard";
  import VisibilityBadge from "../components/VisibilityBadge.svelte";
  import Icon from "../components/Icon.svelte";
  import { appState } from "../lib/appstate.svelte";
  import type { NoteSummary } from "../lib/api";

  let textFilter = $state("");
  let typeFilter = $state("");
  let workflowFilter = $state("");
  let sortKey = $state<"name" | "type" | "path">("name");
  let sortDir = $state<"asc" | "desc">("asc");
  let filter = $state<"all" | "party" | "gm" | "owner" | "private">("all");

  function vis(n: NoteSummary): string {
    try { return surfaceVisibility(n); } catch { return "gm"; }
  }
  function defaulted(n: NoteSummary): boolean {
    return (n as any).defaulted ?? (n as any).visibility === undefined;
  }

  const types = $derived(
    [...new Set(vaultStore.notes.map((n) => String((n as any).type ?? "")))].sort(),
  );

  const listed = $derived(
    vaultStore.notes
      .filter((n) => {
        if (filter !== "all" && vis(n) !== filter) return false;
        if (typeFilter && String((n as any).type ?? "") !== typeFilter) return false;
        if (workflowFilter && String((n as any).workflow ?? "") !== workflowFilter) return false;
        const cf = appState.campaignFilter;
        if (cf && String((n as any).campaign || "No campaign") !== cf) return false;
        if (textFilter) {
          const hay = `${String(n.name ?? "")} ${String((n as any).path ?? "")} ${String((n as any).type ?? "")}`.toLowerCase();
          if (!hay.includes(textFilter.toLowerCase())) return false;
        }
        return true;
      })
      .sort((a, b) => {
        const av = sortKey === "name" ? String(a.name ?? a.uid) : String((a as any)[sortKey === "type" ? "type" : "path"] ?? "");
        const bv = sortKey === "name" ? String(b.name ?? b.uid) : String((b as any)[sortKey === "type" ? "type" : "path"] ?? "");
        const c = av.localeCompare(bv);
        return sortDir === "asc" ? c : -c;
      }),
  );

  const selected = $derived(vaultStore.notes.find((n) => String(n.uid) === appState.selectedUid) ?? null);

  function open(uid: string) {
    appState.selectedUid = uid;
    appState.go("wiki", "viewer");
  }

  // No context of its own: selecting a note leaves for the viewer.
  $effect(() => { appState.rail = null; });
  $effect(() => {
    if (!appState.selectedUid && vaultStore.notes.length) {
      appState.selectedUid = String(vaultStore.notes[0].uid);
    }
  });
</script>

<section class="explorer" aria-label="Vault explorer">
  <header class="list-head">
    <div class="list-head__row">
      <h2 class="list-head__title">Vault</h2>
      <span class="count mono">{listed.length} of {vaultStore.notes.length}</span>
    </div>
    <input class="text-filter" placeholder="Filter by name, path, type…" bind:value={textFilter} aria-label="Filter notes by text" />
    <div class="list-head__filters">
      <select bind:value={typeFilter} aria-label="Type filter">
        <option value="">all types</option>
        {#each types as t}<option value={t}>{t || "(no type)"}</option>{/each}
      </select>
      <select bind:value={workflowFilter} aria-label="Workflow filter">
        <option value="">any workflow</option>
        <option value="draft">draft</option>
        <option value="submitted">submitted</option>
        <option value="active">active</option>
        <option value="retired">retired</option>
      </select>
      <select bind:value={sortKey} aria-label="Sort by">
        <option value="name">name</option>
        <option value="type">type</option>
        <option value="path">path</option>
      </select>
      <button class="btn btn--sm" onclick={() => (sortDir = sortDir === "asc" ? "desc" : "asc")} title="Sort direction" aria-label="Toggle sort direction">
        {sortDir === "asc" ? "↑" : "↓"}
      </button>
    </div>
    {#if appState.campaignFilter}
      <div class="campaign-chip">
        <span class="tag"><Icon name="scroll" size={11} /> {appState.campaignFilter}</span>
        <button class="btn btn--sm" onclick={() => (appState.campaignFilter = null)}>Clear</button>
      </div>
    {/if}
    <div class="list-head__filters" role="group" aria-label="Filter by visibility">
      {#each [["all", "All"], ["party", "Party"], ["gm", "GM only"], ["owner", "Owner"], ["private", "Private"]] as f}
        <button class="btn btn--sm" class:is-active={filter === f[0]} aria-pressed={filter === f[0]} onclick={() => (filter = f[0] as any)}>{f[1]}</button>
      {/each}
    </div>
  </header>
  <ul class="notes">
    {#each listed as n (n.uid)}
      <li>
        <button class="note-row" class:is-selected={appState.selectedUid === String(n.uid)} onclick={() => open(String(n.uid))}>
          <span class="note-row__type">{String((n as any).type ?? "")}</span>
          <span class="note-row__name">{String(n.name ?? n.uid)}</span>
          <span class="note-row__path mono">{String((n as any).path ?? "")}</span>
          <span class="note-row__vis"><VisibilityBadge visibility={vis(n)} defaulted={defaulted(n)} size="sm" /></span>
        </button>
      </li>
    {:else}
      <li class="empty">No notes match these filters.</li>
    {/each}
  </ul>
  {#if selected}
    <button class="fab" onclick={() => { appState.selectedUid = String(selected.uid); appState.go("wiki", "editor"); }} title="Edit selected note" aria-label="Edit selected note">
      <Icon name="pencil" size={18} />
    </button>
  {/if}
</section>

<style>
  .explorer { position: relative; height: 100%; overflow: hidden; display: flex; flex-direction: column; background: var(--bg-canvas); }
  .list-head { padding: var(--space-3) var(--space-4); background: var(--bg-canvas); border-block-end: 1px solid var(--border-subtle); }
  .list-head__row { display: flex; align-items: baseline; justify-content: space-between; gap: var(--space-2); }
  .list-head__title { font-size: var(--text-md); }
  .count { font-size: var(--text-2xs); color: var(--fg-subtle); font-variant-numeric: tabular-nums; }
  .text-filter { width: 100%; margin-block-start: var(--space-2); padding: var(--space-1) var(--space-2); border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--bg-surface); color: var(--fg-default); font-size: var(--text-sm); }
  .list-head__filters { display: flex; gap: var(--space-1); margin-block-start: var(--space-2); flex-wrap: wrap; align-items: center; }
  .list-head__filters select { padding: var(--space-1) var(--space-2); border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: var(--bg-surface); color: var(--fg-default); font-size: var(--text-xs); }
  .campaign-chip { display: flex; align-items: center; gap: var(--space-1); margin-block-start: var(--space-2); }
  .notes { list-style: none; margin: 0; padding: var(--space-2); overflow-y: auto; display: flex; flex-direction: column; gap: 2px; }
  .note-row { display: grid; grid-template-columns: 6.5rem 1fr auto; grid-template-areas: "type name vis" "type path vis"; align-items: center; gap: 0 var(--space-2); width: 100%; padding: var(--space-2) var(--space-3); border: 1px solid transparent; border-radius: var(--radius-md); background: none; text-align: start; cursor: pointer; }
  .note-row:hover { background: var(--bg-surface-2); }
  .note-row.is-selected { background: var(--bg-surface); border-color: var(--border-default); box-shadow: inset 3px 0 0 var(--accent); }
  .note-row__type { grid-area: type; font-size: var(--text-2xs); text-transform: uppercase; letter-spacing: 0.06em; color: var(--fg-subtle); font-weight: var(--weight-semibold); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .note-row__name { grid-area: name; font-size: var(--text-sm); font-weight: var(--weight-medium); }
  .note-row__path { grid-area: path; font-size: var(--text-2xs); color: var(--fg-subtle); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .note-row__vis { grid-area: vis; }
  .empty { padding: var(--space-4); font-size: var(--text-sm); color: var(--fg-muted); text-align: center; }
  .fab {
    position: absolute; inset-inline-end: var(--space-4); inset-block-end: var(--space-4);
    width: 3rem; height: 3rem; display: grid; place-items: center;
    border-radius: var(--radius-pill); border: 1px solid var(--accent-border);
    background: var(--accent); color: var(--fg-on-accent, #fff); cursor: pointer; box-shadow: var(--shadow-3);
  }
  .fab:hover { background: var(--accent-hover); }
</style>
