<script lang="ts">
  // Projected markdown rendered locally. The server is the authority on
  // *what* may be seen (non-GM callers only receive the projected body);
  // this component decides *how* it looks. Secrets arrive only for
  // principals entitled to them and render with the hatch affordance.

  import { renderMarkdown } from "../lib/markdown";

  let {
    source,
    noteUid = "",
    canReveal = false,
    onReveal = undefined,
    resolveWiki = undefined,
    onOpenNote = undefined,
  }: {
    source: string;
    noteUid?: string;
    canReveal?: boolean;
    onReveal?: (blockId: string) => void;
    resolveWiki?: (name: string) => string | null;
    onOpenNote?: (uid: string) => void;
  } = $props();

  const html = $derived(renderMarkdown(source ?? "", { resolveWiki }));

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
  .markdown :global(table) { border-collapse: collapse; width: 100%; }
  .markdown :global(th), .markdown :global(td) { border: 1px solid var(--border-default); padding: var(--space-1) var(--space-2); font-size: var(--text-sm); }
  .markdown :global(.block-ref) { font-family: var(--font-mono); font-size: 0.75em; color: var(--fg-subtle); user-select: none; }
  .markdown :global(.wikilink) { color: var(--accent); text-decoration: underline dotted; }
  .markdown :global(.wikilink.dangling) { color: var(--fg-subtle); text-decoration: underline dotted; cursor: default; }
</style>
