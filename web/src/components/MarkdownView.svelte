<script lang="ts">
  // Server-projected HTML view. The server is the renderer of record:
  // secrets are stripped before they reach us, so {@html} is safe here
  // (server sanitises). Secret slots are rendered by the server as
  // .secret-block hosts; we add reveal controls around them.

  let {
    html,
    noteUid = "",
    canReveal = false,
    onReveal = undefined,
  }: {
    html: string;
    noteUid?: string;
    canReveal?: boolean;
    onReveal?: (blockId: string) => void;
  } = $props();

  function handleClick(ev: MouseEvent) {
    const t = ev.target as HTMLElement;
    const btn = t.closest?.("[data-reveal-block]") as HTMLElement | null;
    if (btn && onReveal) {
      onReveal(btn.getAttribute("data-reveal-block") ?? "");
    }
  }
</script>

<div class="prose markdown" onclick={handleClick}>
  {@html html}
  {#if canReveal && noteUid}
    <p class="reveal-hint">GM view: secret blocks render with their block id. Use the frontmatter panel or the button on the block to reveal.</p>
  {/if}
</div>

<style>
  .markdown :global(.callout) { margin-block: var(--space-4); }
  .markdown :global(.statblock) { margin-block: var(--space-4); }
  .markdown :global(.md-table) { margin-block: var(--space-4); }
  .markdown :global(table) { border-collapse: collapse; width: 100%; }
  .markdown :global(th), .markdown :global(td) { border: 1px solid var(--border-default); padding: var(--space-1) var(--space-2); font-size: var(--text-sm); }
  .reveal-hint { font-size: var(--text-xs); color: var(--fg-subtle); }
</style>
