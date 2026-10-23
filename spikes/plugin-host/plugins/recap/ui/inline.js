// The recap strip: a light-DOM surface.
//
// It registers a custom element and appends it into the host the app
// provided. The app owns the host; the plugin owns its own element inside
// it.
//
// The contract is the same function as a shadow surface:
//
//     export function mount(host, ctx, demitable) -> void | Promise<void>
//
// The only difference is where the markup goes: into the host's shadowRoot
// for a shadow surface, into the host itself for a light one.

class RecapInline extends HTMLElement {
  connectedCallback() {
    this.replaceChildren();
    this.className = 'recap-inline';
    this.dataset.plugin = 'demitable.recap';
    this.dataset.surface = 'recap.inline';

    const heading = document.createElement('h2');
    heading.className = 'recap-inline-title';
    heading.textContent = 'Recent sessions';

    const list = document.createElement('ul');
    list.className = 'recap-inline-list';

    this.append(heading, list);
    this.list = list;

    this.refresh();
    this.unsub = window.__demitableBridge?.on('vault.changed', () => this.refresh());
  }

  disconnectedCallback() {
    this.unsub?.();
  }

  async refresh() {
    const demitable = window.__demitableBridge;
    if (!demitable || !this.list) return;
    const res = await demitable.call('vault.query', { type: 'session', limit: 3 });
    this.list.replaceChildren();
    for (const note of res.items ?? []) {
      const li = document.createElement('li');
      li.className = 'recap-inline-item';

      // These class names are the APP's. The app's stylesheet styles them,
      // because this surface is in the app's document. In the shadow panel
      // the same names are styled by the plugin's own stylesheet, or not at
      // all.
      const chip = document.createElement('span');
      chip.className = 'chip';
      chip.textContent = note.type;

      const title = document.createElement('span');
      title.className = 'recap-inline-title-text';
      title.textContent = note.title;

      li.append(chip, title);
      this.list.appendChild(li);
    }
  }
}

customElements.define('demitable-recap-inline', RecapInline);

export function mount(host, ctx, demitable) {
  window.__demitableBridge = demitable;
  const el = document.createElement('demitable-recap-inline');
  el.setAttribute('part', 'inline');
  host.appendChild(el);
}
