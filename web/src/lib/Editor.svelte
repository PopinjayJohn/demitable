<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { EditorState } from "@codemirror/state";
  import { EditorView, keymap, lineNumbers } from "@codemirror/view";
  import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
  import { markdown } from "@codemirror/lang-markdown";

  let { value = $bindable(""), onchange }: { value?: string; onchange?: (v: string) => void } = $props();

  let host: HTMLDivElement;
  let view: EditorView | null = null;

  onMount(() => {
    const state = EditorState.create({
      doc: value,
      extensions: [
        lineNumbers(),
        history(),
        markdown(),
        keymap.of([...defaultKeymap, ...historyKeymap]),
        EditorView.updateListener.of((u) => {
          if (u.docChanged) {
            const next = u.state.doc.toString();
            value = next;
            onchange?.(next);
          }
        }),
      ],
    });
    view = new EditorView({ state, parent: host });
  });

  $effect(() => {
    if (view && value !== view.state.doc.toString()) {
      view.dispatch({
        changes: { from: 0, to: view.state.doc.length, insert: value },
      });
    }
  });

  onDestroy(() => {
    view?.destroy();
  });
</script>

<div class="editor-host" bind:this={host}></div>

<style>
  .editor-host {
    border: 1px solid var(--border-default, #ccc);
    min-height: 60vh;
  }
  .editor-host :global(.cm-editor) {
    height: 100%;
  }
</style>
