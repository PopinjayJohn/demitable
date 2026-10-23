// The surface host.
//
// The app creates the host element and the shadow root; the plugin mounts
// into it (ADR-0046). The plugin never attaches its own root, so the app
// can always reach in — `host.shadowRoot`, `::part()`, the focus trap, the
// accessible name.

/**
 * Mount one surface into its host element.
 *
 * @param {object} surface   the manifest's provides entry, plus the URLs the
 *                           server resolved for entry and style
 * @param {HTMLElement} hostEl  the host element the app created
 * @param {object} ctx      the note context
 * @param {object} demitable  the bridge
 */
export async function mountSurface(surface, hostEl, ctx, demitable) {
  hostEl.dataset.plugin = surface.pluginId;
  hostEl.dataset.surface = surface.id;
  hostEl.dataset.dom = surface.dom;

  if (surface.dom === 'shadow') {
    // The app creates the root. Open, because the app is the one that would
    // need to reach in.
    //
    // A shadow root can only be attached once, so a surface that unmounts
    // and mounts again reuses the root the app already made.
    if (!hostEl.__surfaceRoot) {
      hostEl.__surfaceRoot = hostEl.attachShadow({ mode: 'open' });
    }
    const root = hostEl.__surfaceRoot;
    if (surface.style && !root.__styled) {
      await adoptStyles(root, surface.style);
      root.__styled = true;
    }
  } else if (surface.dom === 'light') {
    if (surface.style && !document.querySelector(`style[data-plugin="${surface.pluginId}"]`)) {
      await injectDocumentStyles(surface.style, surface.pluginId);
    }
  } else if (surface.dom === 'frame') {
    const frame = document.createElement('iframe');
    frame.src = surface.entry;
    frame.className = 'surface-frame';
    hostEl.appendChild(frame);
    return;
  }

  const mod = await import(surface.entry);
  if (typeof mod.mount !== 'function') {
    throw new Error(`surface ${surface.id}: entry ${surface.entry} has no mount()`);
  }
  await mod.mount(hostEl, ctx, demitable);
}

/** Unmount a surface, leaving the app's host element in place. */
export function unmountSurface(surface, hostEl) {
  if (surface.dom === 'shadow' && hostEl.__surfaceRoot) {
    hostEl.__surfaceRoot.replaceChildren();
    hostEl.__surfaceRoot.adoptedStyleSheets = [];
    hostEl.__surfaceRoot.__styled = false;
  } else {
    hostEl.replaceChildren();
  }
}

/** Put a stylesheet into a shadow root. adoptedStyleSheets is the modern
 *  way and it is CSP-friendly (R9). */
async function adoptStyles(root, url) {
  const css = await (await fetch(url)).text();
  if ('adoptedStyleSheets' in Document.prototype && 'replaceSync' in CSSStyleSheet.prototype) {
    const sheet = new CSSStyleSheet();
    sheet.replaceSync(css);
    root.adoptedStyleSheets = [...root.adoptedStyleSheets, sheet];
    return;
  }
  const style = document.createElement('style');
  style.textContent = css;
  root.appendChild(style);
}

/** Put a plugin's stylesheet into the app's document, scoped by plugin id. */
async function injectDocumentStyles(url, pluginId) {
  const css = await (await fetch(url)).text();
  const style = document.createElement('style');
  style.dataset.plugin = pluginId;
  style.textContent = css;
  document.head.appendChild(style);
}
