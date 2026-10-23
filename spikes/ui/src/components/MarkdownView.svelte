<script lang="ts">
  import { renderMarkdown, type RenderResult, type SecretSlot } from "../lib/markdown/render";
  import { NOTES, VOCABULARY } from "../lib/campaign";

  let {
    body,
    variant = "full",
    secretRenderer = undefined,
  }: {
    body: string;
    /** `party` renders the reduced stat card. */
    variant?: "full" | "party";
    /** How a GM-only block is drawn. */
    secretRenderer?: (slot: SecretSlot) => string;
  } = $props();

  const PATHS = new Map(NOTES.map((n) => [n.path, n]));

  function resolveLink(href: string) {
    if (/^https?:/.test(href)) return { href, external: true };
    const [path] = href.split("#");
    if (path === "") return { href, external: false };
    const hit = PATHS.get(path);
    if (hit) return { href: `#note-${hit.uid}`, external: false };
    // The corpus's links are vault-root-relative `.md` paths. The vault holds
    // 1,692 of them; this prototype holds eight. A link to a note that is not
    // in this vault is **dangling**, and has to look like it.
    return { href, external: false, dangling: true };
  }

  const result = $derived<RenderResult>(
    renderMarkdown(body, {
      vocab: VOCABULARY,
      secret: secretRenderer,
      resolveLink,
      statBlockVariant: variant,
    }),
  );
</script>

<div class="prose markdown">
  {@html result.html}
</div>

<style>
  .markdown :global(.callout) { margin-block: var(--space-4); }
  .markdown :global(.statblock) { margin-block: var(--space-4); }
  .markdown :global(.md-table) { margin-block: var(--space-4); }
  .markdown :global(.block-ref) {
    font-family: var(--font-mono);
    font-size: 0.75em;
    color: var(--fg-subtle);
    user-select: none;
  }
  .markdown :global(.block-ref a) { color: inherit; text-decoration: none; }
  .markdown :global(.block-ref a:hover) { text-decoration: underline; }
  .markdown :global(.callout table) { margin-block: var(--space-2); }
  .markdown :global(.callout__body > table) { width: auto; }
</style>
