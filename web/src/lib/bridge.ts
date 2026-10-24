// Browser bridge types for plugin surfaces (M3b). Provisional.
export interface PluginContext {
  id: string;
  name: string;
  version: string;
  api: number;
  principal: string;
  role: string;
  config: Record<string, unknown>;
  capabilities: string[];
  surfaces: string[];
}

export interface Bridge {
  ready(): Promise<PluginContext>;
  call(op: string, args?: unknown): Promise<unknown>;
  on(event: string, handler: (e: unknown) => void): () => void;
  emit(name: string, payload?: unknown): void;
  pref: { get(key: string): Promise<unknown>; set(key: string, value: unknown): Promise<void> };
}

export function createHost(element: HTMLElement, mode: 'shadow' | 'light' | 'frame'): ShadowRoot | HTMLElement {
  if (mode === 'light') return element;
  if (mode === 'frame') return element;
  return element.attachShadow({ mode: 'open' });
}
