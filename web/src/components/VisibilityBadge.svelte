<script lang="ts">
  let { visibility, defaulted = false }: { visibility: string; defaulted?: boolean } = $props();
</script>

<span class="vis-badge" data-vis={visibility} title={defaulted ? "Defaulted" : undefined}>
  {visibility}{defaulted ? "*" : ""}
</span>
