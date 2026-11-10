<script lang="ts">
  // Shared note context for the right rail, used by both the vault
  // explorer and the viewer: effective badge, permissions, secrets with
  // reveal controls, and a jump to the editor.
  import VisibilityBadge from "./VisibilityBadge.svelte";
  import Icon from "./Icon.svelte";
  import { appState } from "../lib/appstate.svelte";
  import type { SecretRef } from "../lib/appstate.svelte";
  import type { NoteSummary } from "../lib/api";
  import { isGM } from "../lib/auth";
  import { patchNote } from "../lib/api";
  import { vaultStore } from "../lib/vaultstore.svelte";

  let {
    note = null,
    secrets = [],
    revealed = [],
    onReveal,
    onUnreveal,
  }: {
    note: NoteSummary | null;
    secrets: SecretRef[];
    revealed?: string[];
    onReveal: (blockId: string, audience?: any) => void;
    onUnreveal?: (blockId: string, audience?: any) => void;
  } = $props();

  const revealedSet = $derived(new Set(revealed));
  let revealInputs = $state<Record<string, string>>({});
  let unrevealInputs = $state<Record<string, string>>({});

  function vis(n: NoteSummary): string {
    return String((n as any).effective_visibility ?? (n as any).effectiveVisibility ?? "gm");
  }
  function defaulted(n: NoteSummary): boolean {
    return Boolean((n as any).defaulted) || (n as any).visibility === undefined;
  }

  const VISIBILITY_VALUES = ["public", "party", "owner", "private", "discovered", "gm"];

  async function changeVisibility(value: string) {
    if (!note?.uid || !isGM()) return;
    try {
      await patchNote(String(note.uid), { visibility: value });
      await vaultStore.load();
    } catch {
      /* server enforces; ignore client-side */
    }
  }
</script>

{#if note}
  <div class="rail-block">
    <h3 class="rail-title">Visibility</h3>
    <VisibilityBadge visibility={vis(note)} defaulted={defaulted(note)} />
    <div class="rail-gap"></div>
    <div class="permissions">
      <div class="perm-row">
        <span class="perm-label">Owner</span>
        <span class="perm-value">{String((note as any).player ?? "").trim() || "—"}</span>
      </div>
      <div class="perm-row">
        <span class="perm-label">Visibility</span>
        {#if isGM()}
          <select
            class="perm-select"
            value={vis(note)}
            onchange={(e) => changeVisibility((e.currentTarget as HTMLSelectElement).value)}
          >
            {#each VISIBILITY_VALUES as v}
              <option value={v}>{v}</option>
            {/each}
          </select>
        {:else}
          <span class="perm-value">{vis(note)}{defaulted(note) ? " (defaulted)" : ""}</span>
        {/if}
      </div>
    </div>
    {#if appState.showInspector}
      <div class="rail-gap"></div>
    {/if}
  </div>
  <div class="rail-block">
    <h3 class="rail-title">GM-only blocks <span class="tag">{secrets.length}</span></h3>
    {#if secrets.length === 0}
      <p class="rail-empty">None. No block marked GM-only.</p>
    {:else}
      <ul class="secrets">
        {#each secrets as s (s.id)}
          <li class="secret" class:is-revealed={revealedSet.has(s.id)}>
            <span class="secret__label">{s.label}</span>
            <span class="secret__id mono">^{s.id}</span>
            {#if revealedSet.has(s.id)}
              <span class="tag tag--revealed">Revealed</span>
              {#if isGM()}
                <div class="reveal-row">
                  <button class="btn btn--sm" onclick={() => onUnreveal?.(s.id, { all: true })}>Un-reveal from table</button>
                  <button class="btn btn--sm" onclick={() => onUnreveal?.(s.id, { display: true })}>Un-reveal from display</button>
                  <input class="reveal-players" placeholder="player ids" bind:value={unrevealInputs[s.id]} />
                  <button class="btn btn--sm" onclick={() => onUnreveal?.(s.id, { players: (unrevealInputs[s.id] ?? "").split(",").map((x) => x.trim()).filter(Boolean) })}>Un-reveal from players</button>
                </div>
              {/if}
            {:else}
              <div class="reveal-row">
                <button class="btn btn--sm" onclick={() => onReveal(s.id, { all: true })}>Reveal to table</button>
                <button class="btn btn--sm" onclick={() => onReveal(s.id, { display: true })}>Reveal to display</button>
                <input class="reveal-players" placeholder="player ids" bind:value={revealInputs[s.id]} />
                <button class="btn btn--sm" onclick={() => onReveal(s.id, { players: (revealInputs[s.id] ?? "").split(",").map((x) => x.trim()).filter(Boolean) })}>Reveal to players</button>
              </div>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
  </div>
  <div class="rail-block">
    <button class="btn btn--sm" onclick={() => appState.go("wiki", "editor")}>
      <Icon name="pencil" size={13} /> Open in editor
    </button>
  </div>
{:else}
  <p class="rail-empty">Select a note.</p>
{/if}

<style>
  .rail-block + .rail-block { margin-block-start: var(--space-6); }
  .rail-title { font-size: var(--text-2xs); font-weight: var(--weight-bold); letter-spacing: 0.08em; text-transform: uppercase; color: var(--fg-subtle); margin-block-end: var(--space-2); display: flex; align-items: center; gap: var(--space-2); }
  .rail-gap { height: var(--space-2); }
  .rail-empty { font-size: var(--text-xs); color: var(--fg-subtle); }
  .secrets { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: var(--space-2); }
  .secret { display: flex; flex-direction: column; gap: 2px; padding: var(--space-2); border: 1px dashed var(--secret-border); border-radius: var(--radius-md); background: var(--bg-surface); }
  .secret.is-revealed { border-style: solid; border-color: var(--success-border, var(--success)); }
  .tag--revealed { color: var(--success); border-color: var(--success-border, var(--success)); background: var(--success-subtle); }
  .secret__label { font-size: var(--text-xs); font-weight: var(--weight-semibold); }
  .secret__id { font-size: var(--text-2xs); color: var(--fg-subtle); }
  .reveal-row { display: flex; flex-wrap: wrap; gap: var(--space-2); margin-block-start: var(--space-2); }
  .reveal-players { font-size: var(--text-xs); }
  .permissions { display: flex; flex-direction: column; gap: var(--space-2); }
  .perm-row { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-xs); }
  .perm-label { color: var(--fg-subtle); min-width: 5rem; }
  .perm-value { color: var(--fg-default); }
  .perm-select { font-size: var(--text-xs); }
</style>
