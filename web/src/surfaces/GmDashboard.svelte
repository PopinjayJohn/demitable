<script lang="ts">
  import { onMount } from "svelte";
  import { loadDashboard, DASHBOARD_BUDGET_MS, surfaceVisibility } from "../lib/dashboard";
  import VisibilityBadge from "../components/VisibilityBadge.svelte";
  import type { NoteSummary } from "../lib/api";

  let notes = $state<NoteSummary[]>([]);
  let counts = $state({ party: 0, gm: 0, owner: 0, private: 0, total: 0 });
  let attention = $state<NoteSummary[]>([]);
  let status = $state<string>("loading");
  let elapsedMs = $state<number>(0);
  let error = $state<string>("");

  onMount(async () => {
    try {
      const data = await loadDashboard();
      notes = data.notes;
      counts = data.counts;
      attention = data.attention;
      elapsedMs = data.elapsedMs;
      status = "loaded";
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
      status = "failed";
    }
  });

  function vis(n: NoteSummary): string {
    return surfaceVisibility(n);
  }
</script>

<section class="gm-dashboard" aria-label="GM dashboard">
  <p role="status">Status: {status}</p>
  {#if status === "failed"}
    <p class="error">{error}</p>
  {:else if status === "loaded"}
    <div class="stats">
      <span>Party {counts.party}</span>
      <span>GM {counts.gm}</span>
      <span>Owner {counts.owner}</span>
      <span>Private {counts.private}</span>
      <span>Total {counts.total}</span>
    </div>
    <p class="budget">Rendered in {elapsedMs.toFixed(1)}ms (budget {DASHBOARD_BUDGET_MS}ms)</p>

    {#if attention.length > 0}
      <section class="attention">
        <h3>Needs attention</h3>
        <ul>
          {#each attention as n (n.uid)}
            <li>{n.name ?? n.uid}</li>
          {/each}
        </ul>
      </section>
    {/if}

    <ul class="notes">
      {#each notes as n (n.uid)}
        <li>
          <span>{n.name ?? n.uid}</span>
          <VisibilityBadge visibility={vis(n)} />
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .gm-dashboard { padding: 1rem; }
  .stats { display: flex; gap: 1rem; flex-wrap: wrap; }
  .error { color: var(--danger, #c00); }
  .budget { font-size: 0.8rem; color: var(--fg-subtle, #888); }
  .notes { list-style: none; padding: 0; }
</style>
