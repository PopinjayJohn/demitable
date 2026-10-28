<script lang="ts">
  // Projected markdown rendered locally. The server is the authority on
  // *what* may be seen (non-GM callers only receive the projected body);
  // this component decides *how* it looks. Secrets arrive only for
  // principals entitled to them and render with the hatch affordance.

  import { renderMarkdown } from "../lib/markdown";
  import type { Snippet } from "svelte";

  let {
    source,
    noteUid = "",
    canReveal = false,
    onReveal = undefined,
    revealed = [],
    resolveWiki = undefined,
    onOpenNote = undefined,
    header = undefined,
  }: {
    source: string;
    noteUid?: string;
    canReveal?: boolean;
    onReveal?: (blockId: string) => void;
    revealed?: string[];
    resolveWiki?: (name: string) => string | null;
    onOpenNote?: (uid: string) => void;
    /** Optional lead content rendered inside the prose flow (stat boxes). */
    header?: Snippet;
  } = $props();

  const html = $derived(renderMarkdown(source ?? "", { resolveWiki, revealed, revealButtons: canReveal }));

  function handleClick(ev: MouseEvent) {
    const t = ev.target as HTMLElement;
    const revealBtn = t.closest?.("[data-reveal-block]") as HTMLElement | null;
    if (revealBtn && onReveal) {
      onReveal(revealBtn.getAttribute("data-reveal-block") ?? "");
      return;
    }
    const link = t.closest?.("a.wikilink[data-uid]") as HTMLAnchorElement | null;
    if (link && onOpenNote) {
      ev.preventDefault();
      onOpenNote(link.getAttribute("data-uid") ?? "");
    }
  }
</script>

<div class="prose markdown" onclick={handleClick}>
  {#if header}{@render header()}{/if}
  {@html html}
</div>

<style>
  .markdown :global(.callout) { margin-block: var(--space-4); padding: var(--space-3); border: 1px solid var(--border-default); border-inline-start-width: 4px; border-radius: var(--radius-md); background: var(--bg-surface-2); }
  .markdown :global(.callout__label) { display: block; font-size: var(--text-2xs); font-weight: var(--weight-bold); letter-spacing: 0.06em; text-transform: uppercase; color: var(--fg-subtle); margin-block-end: var(--space-1); }
  .markdown :global(.statblock) { margin-block: var(--space-4); border: 1px solid var(--border-default); border-radius: var(--radius-md); background: var(--bg-surface-2); padding: var(--space-3); }
  .markdown :global(.statblock__head) { font-weight: var(--weight-bold); font-size: var(--text-sm); margin-block-end: var(--space-2); }
  .markdown :global(.statblock pre) { margin: 0; white-space: pre-wrap; font-size: var(--text-xs); }
  .markdown :global(.admonition) { margin-block: var(--space-4); border: 1px solid var(--border-default); border-inline-start-width: 4px; border-radius: var(--radius-md); background: var(--bg-surface-2); padding: var(--space-3); }
  .markdown :global(.admonition__head) { font-weight: var(--weight-bold); font-size: var(--text-sm); margin-block-end: var(--space-2); }
  .markdown :global(.admonition pre) { margin: 0; white-space: pre-wrap; font-size: var(--text-xs); }
  .markdown :global(.md-table) { margin-block: var(--space-4); }
  .markdown :global(.callout), .markdown :global(.statblock),
  .markdown :global(.admonition), .markdown :global(.secret-block) {
    break-inside: avoid;
  }
  .markdown :global(.secret-block) {
    border: var(--border-width-strong) dashed var(--secret-border);
    outline: 2px solid var(--secret-border);
    outline-offset: 2px;
    border-radius: var(--radius-md); background: var(--secret-subtle);
    background-image: repeating-linear-gradient(45deg, transparent 0 8px, rgb(0 0 0 / 0.03) 8px 9px);
    padding: var(--space-3); margin-block: var(--space-4);
  }
  .markdown :global(.secret-block.secret--revealed) {
    border-color: var(--success-border, var(--success));
    outline-color: var(--success-border, var(--success));
    background-color: var(--success-subtle);
  }
  .markdown :global(.secret-block) { position: relative; }
  .markdown :global(.secret-reveal) {
    position: absolute; inset-block-start: var(--space-2); inset-inline-end: var(--space-2);
    padding: var(--space-1) var(--space-2);
    border: 1px solid var(--secret-border); border-radius: var(--radius-sm);
    background: var(--bg-surface); color: var(--fg-default);
    font-size: var(--text-xs); cursor: pointer; opacity: 0;
  }
  .markdown :global(.secret-block:hover .secret-reveal),
  .markdown :global(.secret-block:focus-within .secret-reveal) { opacity: 1; }
  .markdown :global(table) { border-collapse: collapse; width: 100%; }
  .markdown :global(th), .markdown :global(td) { border: 1px solid var(--border-default); padding: var(--space-1) var(--space-2); font-size: var(--text-sm); }
  .markdown :global(.block-ref) { font-family: var(--font-mono); font-size: 0.75em; color: var(--fg-subtle); user-select: none; }
  .markdown :global(.wikilink) { color: var(--accent); text-decoration: underline dotted; }
  .markdown :global(.wikilink.dangling) { color: var(--fg-subtle); text-decoration: underline dotted; cursor: default; }
</style>
