<script lang="ts">
  // Collapsible vault tree: folders from note paths, notes as leaves.
  // Clicking a note opens it in the wiki browser. Folders remember
  // their collapsed state for the session.
  import { appState } from "../lib/appstate.svelte";
  import Icon from "./Icon.svelte";
  import type { NoteSummary } from "../lib/api";

  let { notes = [] }: { notes: NoteSummary[] } = $props();

  interface TreeNode {
    name: string;
    path: string;
    children: Map<string, TreeNode>;
    notes: NoteSummary[];
  }

  function buildTree(items: NoteSummary[]): TreeNode {
    const root: TreeNode = { name: "", path: "", children: new Map(), notes: [] };
    for (const n of items) {
      const p = String((n as any).path ?? String(n.name ?? n.uid));
      const parts = p.split("/").filter(Boolean);
      let node = root;
      let prefix = "";
      for (let i = 0; i < parts.length - 1; i++) {
        prefix += (prefix ? "/" : "") + parts[i];
        let next = node.children.get(parts[i]);
        if (!next) {
          next = { name: parts[i], path: prefix, children: new Map(), notes: [] };
          node.children.set(parts[i], next);
        }
        node = next;
      }
      node.notes.push(n);
    }
    return root;
  }

  const tree = $derived(buildTree(notes));
  let collapsed = $state<Set<string>>(new Set());

  function toggle(path: string) {
    const next = new Set(collapsed);
    if (next.has(path)) next.delete(path);
    else next.add(path);
    collapsed = next;
  }

  function openNote(uid: string) {
    appState.selectedUid = uid;
    appState.go("wiki", "explorer");
    appState.campaignFilter = null;
  }
</script>

{#snippet folder(node: TreeNode, depth: number)}
  {@const isCollapsed = collapsed.has(node.path)}
  <li>
    <button class="fold" style={`padding-inline-start: calc(var(--space-2) + ${depth} * 0.9rem)`} onclick={() => toggle(node.path)} aria-expanded={!isCollapsed}>
      <Icon name={isCollapsed ? "chevron-right" : "chevron-down"} size={13} />
      <span class="fold__name">{node.name}</span>
      <span class="side-count">{node.notes.length + [...node.children.values()].reduce((a, c) => a + c.notes.length, 0)}</span>
    </button>
    {#if !isCollapsed}
      <ul class="tree">
        {#each [...node.children.values()].sort((a, b) => a.name.localeCompare(b.name)) as child (child.path)}
          {@render folder(child, depth + 1)}
        {/each}
        {#each node.notes.slice().sort((a, b) => String(a.name ?? a.uid).localeCompare(String(b.name ?? b.uid))) as n (n.uid)}
          <li>
            <button
              class="leaf"
              class:is-active={appState.selectedUid === String(n.uid)}
              style={`padding-inline-start: calc(var(--space-2) + ${(depth + 1)} * 0.9rem + 13px)`}
              onclick={() => openNote(String(n.uid))}
              title={String((n as any).path ?? n.name ?? n.uid)}
            >
              <span class="leaf__name">{String(n.name ?? n.uid)}</span>
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </li>
{/snippet}

{#if notes.length === 0}
  <p class="side-note">No notes.</p>
{:else}
  <ul class="tree">
    {#each [...tree.children.values()].sort((a, b) => a.name.localeCompare(b.name)) as child (child.path)}
      {@render folder(child, 0)}
    {/each}
    {#each tree.notes as n (n.uid)}
      <li>
        <button class="leaf" class:is-active={appState.selectedUid === String(n.uid)} onclick={() => openNote(String(n.uid))}>
          <span class="leaf__name">{String(n.name ?? n.uid)}</span>
        </button>
      </li>
    {/each}
  </ul>
{/if}

<style>
  .tree { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 1px; }
  .fold, .leaf {
    display: flex; align-items: center; gap: var(--space-1);
    width: 100%; padding-block: var(--space-1); padding-inline-end: var(--space-2);
    border: 1px solid transparent; border-radius: var(--radius-md);
    background: none; color: var(--fg-default); font-size: var(--text-sm);
    text-align: start; cursor: pointer; min-width: 0;
  }
  .fold:hover, .leaf:hover { background: var(--bg-surface-3); }
  .leaf.is-active { background: var(--bg-surface); border-color: var(--border-default); box-shadow: inset 3px 0 0 var(--accent); font-weight: var(--weight-medium); }
  .fold__name, .leaf__name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; min-width: 0; }
  .fold__name { font-weight: var(--weight-medium); color: var(--fg-muted); }
  .side-count { font-size: var(--text-2xs); color: var(--fg-subtle); }
  .side-note { font-size: var(--text-xs); color: var(--fg-muted); }
</style>
