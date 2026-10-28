<script lang="ts">
  // Character stat box: a quick-look card rendered from the pack-shaped
  // sheet data, pack-agnostic — computed values first, then scalar data
  // fields (one nesting level flattened). The full sheet lives in the
  // creator; this is the at-a-glance box.
  import Icon from "./Icon.svelte";

  let {
    name = "",
    computed = {},
    data = {},
    onOpenSheet = undefined,
  }: {
    name?: string;
    computed?: Record<string, unknown>;
    data?: Record<string, unknown>;
    onOpenSheet?: () => void;
  } = $props();

  interface Row { label: string; value: string; computed: boolean }

  function text(v: unknown): string {
    if (v === null || v === undefined) return "—";
    if (Array.isArray(v)) return v.map(String).join(", ") || "—";
    if (typeof v === "object") {
      const entries = Object.entries(v as Record<string, unknown>);
      if (entries.every(([, x]) => typeof x !== "object")) {
        return entries.map(([k, x]) => `${k} ${String(x)}`).join(" · ");
      }
      return `${entries.length} entries`;
    }
    return String(v);
  }

  const rows = $derived<Row[]>([
    ...Object.entries(computed ?? {}).map(([k, v]) => ({ label: k, value: text(v), computed: true })),
    ...Object.entries(data ?? {}).flatMap(([k, v]): Row[] => {
      if (v !== null && typeof v === "object" && !Array.isArray(v)) {
        return Object.entries(v as Record<string, unknown>)
          .filter(([, x]) => x === null || typeof x !== "object" || Array.isArray(x))
          .map(([sk, sv]) => ({ label: `${k}.${sk}`, value: text(sv), computed: false }));
      }
      return [{ label: k, value: text(v), computed: false }];
    }),
  ].slice(0, 14));
</script>

<aside class="statbox" aria-label={name ? `${name} at a glance` : "Character at a glance"}>
  <header class="statbox__head">
    <Icon name="user" size={14} />
    <span class="statbox__name">{name || "Character"}</span>
  </header>
  <dl class="statbox__rows">
    {#each rows as r}
      <div class="statbox__row" class:is-computed={r.computed}>
        <dt>{r.label}</dt>
        <dd>{r.value}</dd>
      </div>
    {/each}
  </dl>
  {#if onOpenSheet}
    <button class="btn btn--sm" onclick={onOpenSheet}>Open full sheet</button>
  {/if}
</aside>

<style>
  .statbox {
    float: inline-end;
    width: min(17rem, 100%);
    margin: 0 0 var(--space-4) var(--space-4);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md);
    background: var(--bg-surface);
    box-shadow: var(--shadow-2);
    padding: var(--space-3);
  }
  .statbox__head { display: flex; align-items: center; gap: var(--space-2); font-weight: var(--weight-bold); font-size: var(--text-sm); margin-block-end: var(--space-2); }
  .statbox__name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .statbox__rows { margin: 0; display: flex; flex-direction: column; gap: 2px; }
  .statbox__row { display: flex; justify-content: space-between; gap: var(--space-2); font-size: var(--text-xs); }
  .statbox__row dt { color: var(--fg-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .statbox__row dd { margin: 0; font-variant-numeric: tabular-nums; text-align: end; overflow-wrap: anywhere; }
  .statbox__row.is-computed dd { font-weight: var(--weight-semibold); }
  .statbox .btn { margin-block-start: var(--space-2); }
</style>
