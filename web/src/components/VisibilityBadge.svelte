<script lang="ts">
  import Icon from "./Icon.svelte";
  import type { IconName } from "../lib/icons";

  let {
    visibility,
    defaulted = false,
    size = "md",
    showWord = true,
  }: {
    visibility: string;
    defaulted?: boolean;
    size?: "sm" | "md";
    showWord?: boolean;
  } = $props();

  const GLYPH: Record<string, IconName> = {
    gm: "lock",
    party: "users",
    public: "globe",
    owner: "user",
    private: "eye-off",
    discovered: "eye",
  };

  const WORD: Record<string, string> = {
    gm: "GM only",
    party: "Party",
    public: "Public",
    owner: "Owner",
    private: "Private",
    discovered: "Discovered",
  };
  const glyph: IconName = GLYPH[visibility] ?? "file";
  const word: string = WORD[visibility] ?? visibility;
</script>

<span
  class="vis-badge"
  class:vis-badge--sm={size === "sm"}
  class:vis-badge--wordless={!showWord}
  data-vis={visibility}
  title={defaulted ? "Defaulted — the note declares no `visibility` field (ADR-0033 §6)" : undefined}
>
  <span class="vis-badge__rule" aria-hidden="true"></span>
  <Icon name={glyph} size={size === "sm" ? 12 : 14} />
  {#if showWord}
    <span class="vis-badge__word">{word}</span>
    {#if defaulted}<span class="vis-badge__default" aria-hidden="true">*</span>{/if}
  {/if}
  <span class="visually-hidden">{word}{defaulted ? " (defaulted)" : ""}</span>
</span>

<style>
  .vis-badge {
    --vis-hue: var(--fg-muted);
    --vis-bg: var(--bg-surface-2);
    --vis-edge: var(--border-default);
    display: inline-flex; align-items: center; gap: var(--space-1);
    padding-inline-start: var(--space-2); padding-inline-end: var(--space-2);
    min-height: 1.5rem; border: var(--border-width) solid var(--vis-edge);
    border-radius: var(--radius-pill); background: var(--vis-bg);
    color: var(--fg-default); font-size: var(--text-xs);
    font-weight: var(--weight-medium); white-space: nowrap; line-height: 1;
  }
  .vis-badge--sm { font-size: var(--text-2xs); min-height: 1.25rem; }
  .vis-badge--wordless { padding-inline-end: var(--space-1); }
  .vis-badge__rule {
    width: 0.35rem; align-self: stretch; border-radius: var(--radius-sm);
    background: var(--vis-hue); margin-inline-start: calc(var(--space-2) * -1 - 1px); margin-block: -1px;
  }
  .vis-badge__default { font-weight: var(--weight-bold); }
  .vis-badge[data-vis="gm"] { --vis-hue: var(--vis-gm); --vis-bg: var(--vis-gm-bg); --vis-edge: var(--vis-gm-border); }
  .vis-badge[data-vis="gm"] .vis-badge__rule { border-radius: 0; }
  .vis-badge[data-vis="party"] { --vis-hue: var(--vis-party); --vis-bg: var(--vis-party-bg); --vis-edge: var(--vis-party-border); }
  .vis-badge[data-vis="party"] .vis-badge__rule {
    background: repeating-linear-gradient(to bottom, var(--vis-hue) 0 3px, transparent 3px 6px);
  }
  .vis-badge[data-vis="public"] { --vis-hue: var(--vis-public); --vis-bg: var(--vis-public-bg); --vis-edge: var(--vis-public-border); }
  .vis-badge[data-vis="public"] .vis-badge__rule {
    background: repeating-linear-gradient(to bottom, var(--vis-hue) 0 2px, transparent 2px 4px, var(--vis-hue) 4px 6px, transparent 6px 8px);
  }
  .vis-badge[data-vis="owner"] { --vis-hue: var(--vis-owner); --vis-bg: var(--vis-owner-bg); --vis-edge: var(--vis-owner-border); }
  .vis-badge[data-vis="owner"] .vis-badge__rule {
    background: repeating-linear-gradient(to bottom, var(--vis-hue) 0 5px, transparent 5px 9px);
  }
  .vis-badge[data-vis="private"] { --vis-hue: var(--vis-private); --vis-bg: var(--vis-private-bg); --vis-edge: var(--vis-private-border); }
  .vis-badge[data-vis="private"] .vis-badge__rule {
    background: repeating-linear-gradient(to bottom, var(--vis-hue) 0 2px, transparent 2px 4px, var(--vis-hue) 4px 6px, transparent 6px 10px);
  }
</style>
