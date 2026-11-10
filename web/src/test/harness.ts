// Minimal component-test harness for web surfaces (ADR-0064 D3 pinning).
//
// One canonical mechanism: Svelte 5 `mount`/`unmount` into a jsdom container.
// Surfaces are rendered as real components; tests query the rendered DOM.
// No second harness, no testing-library dependency.

import { mount, unmount } from "svelte";

export interface Rendered {
  container: HTMLElement;
  unmount: () => void;
}

export function render(component: any, props: Record<string, unknown> = {}): Rendered {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const instance = mount(component, { target: container, props });
  return {
    container,
    unmount() {
      try { unmount(instance); } catch { /* already unmounted */ }
      if (container.parentNode) container.parentNode.removeChild(container);
    },
  };
}
