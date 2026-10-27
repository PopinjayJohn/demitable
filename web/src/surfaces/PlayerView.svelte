<script lang="ts">
  import { onMount } from "svelte";
  import { fetchNotes } from "../lib/api";
  import { surfaceVisibility } from "../lib/dashboard";
  import VisibilityBadge from "../components/VisibilityBadge.svelte";
  import type { NoteSummary } from "../lib/api";

  let notes = $state<NoteSummary[]>([]);
  let status = $state<string>("loading");

  onMount(async () => {
    try {
      notes = await fetchNotes();
      status = "loaded";
    } catch {
      status = "failed";
    }
  });
</script>

<section class="player-view" aria-label="Player view">
  <p role="status">Status: {status}</p>
  <ul class="notes">
    {#each notes as n (n.uid)}
      <li>
        <span>{n.name ?? n.uid}</span>
        <VisibilityBadge visibility={surfaceVisibility(n)} />
      </li>
    {/each}
  </ul>
</section>

<style>
  .player-view { padding: 1rem; }
  .notes { list-style: none; padding: 0; }
</style>
