<script lang="ts">
  let theme = $state<'light' | 'dark'>('light');
  let health = $state<string>('…');

  async function checkHealth() {
    try {
      const res = await fetch('/api/health');
      const json = (await res.json()) as { status: string };
      health = json.status;
    } catch {
      health = 'error';
    }
  }

  $effect(() => {
    void checkHealth();
  });

  function toggleTheme() {
    theme = theme === 'light' ? 'dark' : 'light';
    if (theme === 'dark') {
      document.documentElement.dataset.theme = 'dark';
    } else {
      delete document.documentElement.dataset.theme;
    }
  }
</script>

<main>
  <h1>Demitable</h1>
  <p role="status">API health: {health}</p>
  <button type="button" onclick={toggleTheme}>Toggle theme</button>
</main>

<style>
  main {
    font-family: var(--font-sans, system-ui);
    color: var(--fg-default);
    background: var(--bg-canvas);
    padding: var(--space-4);
    min-height: 100vh;
  }
  button {
    color: var(--fg-default);
    background: var(--bg-surface);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md);
    padding: var(--space-2) var(--space-3);
    min-height: var(--tap-target);
    cursor: pointer;
  }
  button:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
</style>
