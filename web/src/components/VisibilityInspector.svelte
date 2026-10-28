<script lang="ts">
  interface Decision { ref: string; detail: string; verdict: string; }
  let { decisions = [] }: { decisions?: Decision[] } = $props();
</script>

<div class="inspector">
  <h4 class="inspector__title">Why this role sees it</h4>
  {#if decisions.length === 0}
    <p class="inspector__empty">No rule chain from the server yet. The badge above is the projected value.</p>
  {:else}
  <ol class="inspector__chain">
    {#each decisions as d, i (i)}
      <li class="inspector__step" data-verdict={d.verdict}>
        <span class="inspector__mark" aria-hidden="true">
          {#if d.verdict === "allow"}✓{:else if d.verdict === "deny"}✕{:else}◐{/if}
        </span>
        <span class="inspector__body">
          <span class="inspector__rule">{d.ref}</span>
          <span class="inspector__detail">{d.detail}</span>
        </span>
      </li>
    {/each}
  </ol>
  {/if}
</div>

<style>
  .inspector {
    border: var(--border-width) solid var(--border-subtle);
    border-radius: var(--radius-md); background: var(--bg-surface-2);
    padding: var(--space-3);
  }
  .inspector__title {
    font-size: var(--text-xs); font-weight: var(--weight-semibold);
    letter-spacing: 0.04em; text-transform: uppercase; color: var(--fg-muted);
    margin-block-end: var(--space-2);
  }
  .inspector__empty { font-size: var(--text-xs); color: var(--fg-subtle); }
  .inspector__chain { list-style: none; margin: 0; padding: 0; }
  .inspector__step { display: flex; gap: var(--space-2); padding-block: var(--space-2); border-block-start: var(--border-width) solid var(--border-subtle); }
  .inspector__step:first-child { border-block-start: 0; }
  .inspector__mark {
    flex: none; width: 1.25rem; height: 1.25rem; display: grid; place-items: center;
    border-radius: var(--radius-sm); border: var(--border-width) solid var(--border-default);
    font-size: var(--text-xs); font-weight: var(--weight-bold); line-height: 1;
  }
  .inspector__step[data-verdict="allow"] .inspector__mark { background: var(--success-subtle); border-color: var(--success-border, var(--success)); color: var(--success); }
  .inspector__step[data-verdict="deny"] .inspector__mark { background: var(--danger-subtle); border-color: var(--danger-border); color: var(--danger); }
  .inspector__step[data-verdict="conditional"] .inspector__mark { background: var(--warn-subtle); border-color: var(--warn-border); color: var(--warn); }
  .inspector__body { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
  .inspector__rule { font-size: var(--text-2xs); font-weight: var(--weight-bold); letter-spacing: 0.06em; text-transform: uppercase; color: var(--fg-subtle); }
  .inspector__detail { font-size: var(--text-sm); color: var(--fg-default); }
</style>
