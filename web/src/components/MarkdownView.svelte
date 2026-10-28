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
    twoCol = false,
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
    /** Two explicit grid columns with the header pinned top-right.
     * Unlike CSS multicol this paints reliably and keeps DOM order. */
    twoCol?: boolean;
  } = $props();

  const html = $derived(renderMarkdown(source ?? "", { resolveWiki, revealed, revealButtons: canReveal }));

  let colsEl: HTMLDivElement | null = $state(null);
  let colA: HTMLDivElement | null = $state(null);
  let colB: HTMLDivElement | null = $state(null);
  let movedEl: HTMLDivElement | null = $state(null);
  let balanceQueued = false;

  // Distribute colA's top-level nodes across the two explicit columns so
  // the columns end up roughly balanced. The header (stat box) stays
  // pinned at the top of the right column; moved nodes keep document
  // order. Plain grid, no multicol fragmentainer.
  function balance() {
    balanceQueued = false;
    if (!twoCol || !colA || !colB || !movedEl) return;
    const kids = (el: HTMLElement) => [...el.children].filter((n) => n.nodeType === 1) as HTMLElement[];
    // Restore: everything back into A, in original order.
    const frag = document.createDocumentFragment();
    for (const n of kids(movedEl)) frag.appendChild(n);
    colA.appendChild(frag);
    // Shift trailing nodes into B until the columns balance
    // (keep at least one node in A).
    const TOL = 4;
    let guard = kids(colA).length + 8;
    while (guard-- > 0) {
      const rest = kids(colA);
      if (rest.length <= 1) break;
      if (colA.scrollHeight <= colB.scrollHeight + TOL) break;
      movedEl.prepend(rest[rest.length - 1]);
    }
  }

  function queueBalance() {
    if (balanceQueued) return;
    balanceQueued = true;
    requestAnimationFrame(balance);
  }

  $effect(() => {
    void html;
    void twoCol;
    if (twoCol) queueBalance();
  });

  $effect(() => {
    if (!colsEl) return;
    const ro = new ResizeObserver(() => { if (twoCol) queueBalance(); });
    ro.observe(colsEl);
    const onFonts = () => { if (twoCol) queueBalance(); };
    try { (document as any).fonts?.ready.then(onFonts); } catch { /* ignore */ }
    return () => ro.disconnect();
  });

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

<div class="markdown" onclick={handleClick}>
  {#if twoCol}
    <div class="cols" bind:this={colsEl}>
      <div class="col prose" bind:this={colA}>{@html html}</div>
      <div class="col prose" bind:this={colB}>
        {#if header}{@render header()}{/if}
        <div class="moved" bind:this={movedEl}></div>
      </div>
    </div>
  {:else}
    <div class="prose">
      {#if header}{@render header()}{/if}
      {@html html}
    </div>
  {/if}
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
  /* Explicit two-column grid with JS-balanced content. Plain grid, not
     CSS multicol: no fragmentainer, so painting is reliable and the
     right column can start with its own pinned header. */
  .markdown :global(.cols) { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-6); align-items: start; }
  .markdown :global(.col) { min-width: 0; }
  .markdown :global(.col .statbox) { float: none; width: auto; margin: 0 0 var(--space-4); }
  .markdown :global(.wikilink) { color: var(--accent); text-decoration: underline dotted; }
  .markdown :global(.wikilink.dangling) { color: var(--fg-subtle); text-decoration: underline dotted; cursor: default; }
</style>
