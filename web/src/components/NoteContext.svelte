<script lang="ts">
  // Shared note context for the right rail, used by both the vault
  // explorer and the viewer: effective badge, inspector, secrets with
  // reveal controls, and a jump to the editor.
  import VisibilityBadge from "./VisibilityBadge.svelte";
  import VisibilityInspector from "./VisibilityInspector.svelte";
  import Icon from "./Icon.svelte";
  import { appState } from "../lib/appstate.svelte";
  import type { SecretRef } from "../lib/appstate.svelte";
  import type { NoteSummary } from "../lib/api";

  let {
    note = null,
    secrets = [],
    onReveal,
  }: {
    note: NoteSummary | null;
    secrets: SecretRef[];
    onReveal: (blockId: string) => void;
  } = $props();

  function vis(n: NoteSummary): string {
    return String((n as any).effective_visibility ?? (n as any).effectiveVisibility ?? "gm");
  }
  function defaulted(n: NoteSummary): boolean {
    return Boolean((n as any).defaulted) || (n as any).visibility === undefined;
  }
</script>

{#if note}
  <div class="rail-block">
    <h3 class="rail-title">Visibility</h3>
    <VisibilityBadge visibility={vis(note)} defaulted={defaulted(note)} />
    {#if appState.showInspector}
      <div class="rail-gap"></div>
      <VisibilityInspector decisions={[]} />
    {/if}
  </div>
  <div class="rail-block">
    <h3 class="rail-title">GM-only blocks <span class="tag">{secrets.length}</span></h3>
    {#if secrets.length === 0}
      <p class="rail-empty">None. No block marked GM-only.</p>
    {:else}
      <ul class="secrets">
        {#each secrets as s (s.id)}
          <li class="secret">
            <span class="secret__label">{s.label}</span>
            <span class="secret__id mono">^{s.id}</span>
            <button class="btn btn--sm" onclick={() => onReveal(s.id)}>Reveal to the table</button>
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
  .secret__label { font-size: var(--text-xs); font-weight: var(--weight-semibold); }
  .secret__id { font-size: var(--text-2xs); color: var(--fg-subtle); }
</style>
