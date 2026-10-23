<script lang="ts">
  import Icon from "./Icon.svelte";
  import type { IconName } from "../lib/icons";
  import type { Visibility } from "../lib/model";

  let {
    visibility,
    defaulted = false,
    size = "md",
    showWord = true,
  }: {
    /** The **effective** visibility, not the declared one. */
    visibility: Visibility;
    /** True when the value came from ADR-0033's default rather than a field. */
    defaulted?: boolean;
    size?: "sm" | "md";
    showWord?: boolean;
  } = $props();

  const GLYPH: Record<Visibility, IconName> = {
    gm: "lock",
    party: "users",
    public: "globe",
    owner: "user",
    private: "eye-off",
  };

  const WORD: Record<Visibility, string> = {
    gm: "GM only",
    party: "Party",
    public: "Public",
    owner: "Owner",
    private: "Private",
  };
</script>

<!--
  A visibility badge carries FOUR channels: a glyph, a word, a border style
  and a hue. The hue is the one that is allowed to fail.

  The border style is the channel that survives greyscale and every kind of
  colour blindness, because it is a difference in the *pattern* of the ink
  rather than in its wavelength. See design-system.md §5.
-->
<span
  class="vis-badge"
  class:vis-badge--sm={size === "sm"}
  class:vis-badge--wordless={!showWord}
  data-vis={visibility}
  title={defaulted
    ? "Defaulted — the note declares no `visibility` field (ADR-0033 §6)"
    : undefined}
>
  <span class="vis-badge__rule" aria-hidden="true"></span>
  <Icon name={GLYPH[visibility]} size={size === "sm" ? 12 : 14} />
  {#if showWord}
    <span class="vis-badge__word">{WORD[visibility]}</span>
    {#if defaulted}<span class="vis-badge__default" aria-hidden="true">*</span>{/if}
  {/if}
  <span class="visually-hidden">
    {WORD[visibility]}{defaulted ? " (defaulted)" : ""}
  </span>
</span>

<style>
  .vis-badge {
    --vis-hue: var(--fg-muted);
    --vis-bg: var(--bg-surface-2);
    --vis-edge: var(--border-default);
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding-inline-start: var(--space-2);
    padding-inline-end: var(--space-2);
    min-height: 1.5rem;
    border: var(--border-width) solid var(--vis-edge);
    border-radius: var(--radius-pill);
    background: var(--vis-bg);
    color: var(--fg-default);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    white-space: nowrap;
    line-height: 1;
  }
  .vis-badge--sm { font-size: var(--text-2xs); min-height: 1.25rem; }
  .vis-badge--wordless { padding-inline-end: var(--space-1); }

  /* The rule: the third channel. Pattern, not wavelength. */
  .vis-badge__rule {
    width: 0.35rem;
    align-self: stretch;
    border-radius: var(--radius-sm);
    background: var(--vis-hue);
    margin-inline-start: calc(var(--space-2) * -1 - 1px);
    margin-block: -1px;
  }

  .vis-badge__default { font-weight: var(--weight-bold); }

  .vis-badge[data-vis="gm"] {
    --vis-hue: var(--vis-gm);
    --vis-bg: var(--vis-gm-bg);
    --vis-edge: var(--vis-gm-border);
  }
  .vis-badge[data-vis="gm"] .vis-badge__rule { border-radius: 0; }
  .vis-badge[data-vis="party"] {
    --vis-hue: var(--vis-party);
    --vis-bg: var(--vis-party-bg);
    --vis-edge: var(--vis-party-border);
  }
  .vis-badge[data-vis="party"] .vis-badge__rule {
    background: repeating-linear-gradient(
      to bottom, var(--vis-hue) 0 3px, transparent 3px 6px);
  }
  .vis-badge[data-vis="public"] {
    --vis-hue: var(--vis-public);
    --vis-bg: var(--vis-public-bg);
    --vis-edge: var(--vis-public-border);
  }
  .vis-badge[data-vis="public"] .vis-badge__rule {
    background: repeating-linear-gradient(
      to bottom, var(--vis-hue) 0 2px, transparent 2px 4px, var(--vis-hue) 4px 6px, transparent 6px 8px);
  }
  .vis-badge[data-vis="owner"] {
    --vis-hue: var(--vis-owner);
    --vis-bg: var(--vis-owner-bg);
    --vis-edge: var(--vis-owner-border);
  }
  .vis-badge[data-vis="owner"] .vis-badge__rule {
    background: repeating-linear-gradient(
      to bottom, var(--vis-hue) 0 5px, transparent 5px 9px);
  }
  .vis-badge[data-vis="private"] {
    --vis-hue: var(--vis-private);
    --vis-bg: var(--vis-private-bg);
    --vis-edge: var(--vis-private-border);
  }
  .vis-badge[data-vis="private"] .vis-badge__rule {
    background: repeating-linear-gradient(
      to bottom, var(--vis-hue) 0 2px, transparent 2px 4px, var(--vis-hue) 4px 6px, transparent 6px 10px);
  }
</style>
