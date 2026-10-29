<script lang="ts">
  // Session transcript review surface (P3).
  // Segments with editable speaker labels; promotion of corrections; proposal preview/apply.
  import { onMount } from 'svelte';

  let segments: any[] = [];
  let loading = true;
  export let sessionId: string = '';
  export let token: string = '';

  async function load() {
    loading = true;
    const r = await fetch('/api/v1/sessions/' + sessionId + '/transcript/review', {
      headers: token ? { Authorization: 'Bearer ' + token } : {},
    });
    if (r.ok) {
      const j = await r.json();
      segments = j.segments || [];
    }
    loading = false;
  }

  async function promote(term: string) {
    await fetch('/api/v1/sessions/' + sessionId + '/transcript/promote', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: 'Bearer ' + token } : {}) },
      body: JSON.stringify({ term }),
    });
  }

  async function propose() {
    await fetch('/api/v1/sessions/' + sessionId + '/session-log/propose', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: 'Bearer ' + token } : {}) },
      body: JSON.stringify({}),
    });
  }

  onMount(load);
</script>

{#if loading}
  <p>Loading review…</p>
{:else}
  <section class="review">
    <h2>Transcript review</h2>
    {#each segments as seg}
      <div class="segment">
        <input bind:value={seg.speaker} placeholder="speaker" />
        <textarea bind:value={seg.text}></textarea>
      </div>
    {/each}
    <button on:click={propose}>Propose session log</button>
  </section>
{/if}
EOF