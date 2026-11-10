import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig(({ command }) => ({
  plugins: [svelte()],
  // `resolve.conditions: ['browser']` is required for the Vitest/jsdom
  // component harness (Svelte 5 `mount` needs the browser build of
  // `esm-env`), but it displaces Vite's default conditions in a production
  // build and pulls in `esm-env/dev-fallback.js`. Scope it to non-build
  // (test/dev) runs only, so `vite build` keeps Vite's defaults untouched.
  ...(command !== 'build' ? { resolve: { conditions: ['browser'] } } : {}),
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8080',
      '/ws': { target: 'ws://127.0.0.1:8080', ws: true },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  test: {
    environment: 'jsdom',
  },
}));
