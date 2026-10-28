<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { fetchNotes, sendChatMessage } from "../lib/api";
  import { WSClient } from "../lib/ws";
  import Icon from "../components/Icon.svelte";

  let messages = $state<any[]>([]);
  let body = $state("");
  let status = $state("loading");
  let ws: WSClient | null = null;

  async function load() {
    try {
      const items = await fetchNotes({ type: "message" } as any);
      messages = items.slice(-100);
      status = "loaded";
    } catch (e: any) { status = e?.message ?? "failed"; }
  }
  async function send() {
    if (!body.trim()) return;
    try {
      await sendChatMessage({ content: body.trim(), name: `msg-${Date.now()}`, visibility: "party" });
      body = "";
      await load();
    } catch (e: any) { status = e?.message ?? "send failed"; }
  }
  onMount(() => {
    void load();
    ws = new WSClient({ onNoteSaved: () => void load(), onVaultChanged: () => void load() });
    ws.connect();
    return () => ws?.close();
  });
  onDestroy(() => ws?.close());
</script>

<section class="chat" aria-label="Campaign chat">
  <header class="chat__head"><h2><Icon name="note" size={15} /> Table chat</h2><span class="tag">type:message</span></header>
  <p class="chat__status mono" role="status">{status} · {messages.length} messages</p>
  <ol class="msgs">
    {#each messages as m (m.uid)}
      <li class="msg"><span class="msg__name">{String(m.name ?? m.uid)}</span><span class="msg__body">{String((m as any).content ?? (m as any).snippet ?? "")}</span></li>
    {/each}
  </ol>
  <form class="composer" onsubmit={(e) => { e.preventDefault(); void send(); }}>
    <input bind:value={body} placeholder="Message the table…" aria-label="Chat message" />
    <button class="btn btn--sm" type="submit">Send</button>
  </form>
</section>

<style>
  .chat { display: flex; flex-direction: column; height: 100%; min-height: 50vh; background: var(--bg-surface); }
  .chat__head { display: flex; align-items: center; justify-content: space-between; padding: var(--space-3) var(--space-4); border-block-end: 1px solid var(--border-subtle); }
  .chat__status { padding: var(--space-1) var(--space-4); font-size: var(--text-2xs); color: var(--fg-subtle); }
  .msgs { list-style: none; margin: 0; padding: var(--space-4); flex: 1; overflow-y: auto; display: flex; flex-direction: column; gap: var(--space-2); }
  .msg { border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--bg-canvas); padding: var(--space-2) var(--space-3); }
  .msg__name { display: block; font-size: var(--text-2xs); color: var(--fg-subtle); }
  .msg__body { font-size: var(--text-sm); }
  .composer { display: flex; gap: var(--space-2); padding: var(--space-3) var(--space-4); border-block-start: 1px solid var(--border-subtle); }
  .composer input { flex: 1; padding: var(--space-2); border: 1px solid var(--border-default); border-radius: var(--radius-md); background: var(--bg-surface); color: var(--fg-default); }
</style>
