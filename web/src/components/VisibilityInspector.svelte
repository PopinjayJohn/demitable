<script lang="ts">
  interface Decision { ref: string; detail: string; verdict: string; }
  let { decisions = [] }: { decisions?: Decision[] } = $props();
</script>

<div class="inspector">
  <h4>Why this role sees it</h4>
  <ol>
    {#each decisions as d}
      <li data-verdict={d.verdict}>{d.ref}: {d.detail}</li>
    {/each}
  </ol>
</div>
