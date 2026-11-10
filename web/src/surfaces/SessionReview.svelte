<script lang="ts">
  // Session transcript review surface (P3).
  // Segments with editable speaker labels; promotion of corrections; proposal preview/apply.
  import { onMount } from 'svelte';

  let segments: any[] = [];
  let loading = true;
  export let sessionId: string = '';
  export let token: string = '';

  let promoteTerm = '';
  let canonicalTerm = '';
  let closed = false;
  let error = '';
  let busy = false;

  async function load() {
    loading = true;
    error = '';
    const r = await fetch('/api/v1/sessions/' + sessionId + '/transcript/review', {
      headers: token ? { Authorization: 'Bearer ' + token } : {},
    });
    if (r.status === 409) {
      closed = true;
    }
    if (r.ok) {
      const j = await r.json();
      segments = j.segments || [];
    } else if (r.status !== 409) {
      error = 'review load failed (' + r.status + ')';
    }
    loading = false;
  }

  async function saveReview() {
    const r = await fetch('/api/v1/sessions/' + sessionId + '/transcript/review', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: 'Bearer ' + token } : {}) },
      body: JSON.stringify({ segments }),
    });
    if (r.status === 409) {
      closed = true;
      error = 'Session is closed';
      return false;
    }
    if (!r.ok) {
      error = 'review save failed (' + r.status + ')';
      return false;
    }
    return true;
  }

  async function promote(term: string, from?: string) {
    if (closed) return false;
    busy = true;
    error = '';
    const body: any = { term };
    if (from) body.from = from;
    const r = await fetch('/api/v1/sessions/' + sessionId + '/transcript/promote', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: 'Bearer ' + token } : {}) },
      body: JSON.stringify(body),
    });
    busy = false;
    if (r.status === 409) {
      closed = true;
      error = 'Session is closed';
      return false;
    }
    if (!r.ok) {
      error = 'promote failed (' + r.status + ')';
      return false;
    }
    return true;
  }

  async function propose() {
    if (closed) return;
    busy = true;
    error = '';
    // persist reviewed edits first so the proposal reflects them
    const saved = await saveReview();
    if (!saved && closed) {
      busy = false;
      return;
    }
    const r = await fetch('/api/v1/sessions/' + sessionId + '/session-log/propose', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: 'Bearer ' + token } : {}) },
      body: JSON.stringify({}),
    });
    busy = false;
    if (r.status === 409) {
      closed = true;
      error = 'Session is closed';
      return;
    }
    if (!r.ok) {
      error = 'propose failed (' + r.status + ')';
      return;
    }
  }

  async function promoteCorrection() {
    const term = canonicalTerm.trim() || promoteTerm.trim();
    if (!term) return;
    const ok = await promote(term, promoteTerm.trim() || undefined);
    if (ok) {
      promoteTerm = '';
      canonicalTerm = '';
    }
  }

  onMount(load);
</script>

{#if loading}
  <p>Loading review…</p>
{:else}
  <section class="review">
    <h2>Transcript review</h2>
    {#if closed}
      <p class="closed">Session is closed — review is read-only.</p>
    {/if}
    {#if error}
      <p class="error">{error}</p>
    {/if}
    {#each segments as seg}
      <div class="segment">
        <input bind:value={seg.speaker} placeholder="speaker" disabled={closed} />
        <textarea bind:value={seg.text} disabled={closed}></textarea>
      </div>
    {/each}

    <div class="correction">
      <input bind:value={promoteTerm} placeholder="mangled term" disabled={closed} />
      <input bind:value={canonicalTerm} placeholder="canonical term" disabled={closed} />
      <button on:click={promoteCorrection} disabled={closed || busy}>Promote correction</button>
    </div>

    <button on:click={propose} disabled={closed || busy}>Propose session log</button>
  </section>
{/if}

<style>
  .closed { color: #a00; font-weight: bold; }
  .error { color: #a00; }
</style>
