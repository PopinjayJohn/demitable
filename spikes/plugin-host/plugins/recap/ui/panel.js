// The recap panel: a shadow surface.
//
// The surface contract is one function:
//
//     export function mount(host, ctx, demitable) -> void | Promise<void>
//
// The app created `host` and its shadow root (ADR-0046); the plugin does not
// attach its own root. The app has already put this surface's stylesheet
// into the shadow root, so the plugin never touches <style> or
// document.head.

const state = { ready: null, entries: 2 };

export async function mount(host, ctx, demitable) {
  const root = host.shadowRoot;
  const wrap = document.createElement('div');
  wrap.className = 'recap-panel';
  wrap.setAttribute('part', 'panel');
  root.appendChild(wrap);

  state.ready = await demitable.ready();
  state.entries = Number(state.ready.config.entries ?? 2);

  const head = document.createElement('div');
  head.className = 'recap-head';
  const title = document.createElement('h2');
  title.textContent = 'Recap';
  const toggle = document.createElement('button');
  toggle.type = 'button';
  toggle.className = 'recap-toggle';
  toggle.textContent = 'Hide';
  head.append(title, toggle);
  wrap.appendChild(head);

  const list = document.createElement('div');
  list.className = 'recap-list';
  wrap.appendChild(list);

  // A runtime preference: whether the panel is collapsed. It is NOT in the
  // manifest's config block, because it is not a setting — it is UI state.
  // It is stored through the bridge rather than in localStorage, because
  // localStorage is shared by every role in one browser.
  const collapsed = await demitable.pref.get('collapsed');
  if (collapsed && collapsed.value) {
    list.hidden = true;
    toggle.textContent = 'Show';
  }

  toggle.addEventListener('click', async () => {
    const nowHidden = !list.hidden;
    list.hidden = nowHidden;
    toggle.textContent = nowHidden ? 'Show' : 'Hide';
    await demitable.pref.set('collapsed', nowHidden);
  });

  async function refresh() {
    const res = await demitable.call('vault.query', { type: 'session', limit: state.entries });
    list.replaceChildren();
    const items = res.items ?? [];
    if (items.length === 0) {
      const empty = document.createElement('p');
      empty.className = 'recap-empty';
      empty.textContent = 'No session logs yet.';
      list.appendChild(empty);
      return;
    }
    for (const note of items) {
      const card = document.createElement('article');
      card.className = 'recap-card';
      card.dataset.uid = note.uid;

      const h = document.createElement('h3');
      h.textContent = note.title;

      // A chip. In the shadow root this is the plugin's chip, styled by
      // panel.css. In light DOM it would be the app's chip, styled by the
      // app's stylesheet. The two look different on purpose: that is the
      // difference the spike has to make visible.
      const chip = document.createElement('span');
      chip.className = 'chip';
      chip.textContent = note.type;

      const p = document.createElement('p');
      p.textContent = note.body.slice(0, 90) + (note.body.length > 90 ? '…' : '');

      card.append(h, chip, p);
      list.appendChild(card);
    }
  }

  await refresh();

  // The vault changed. The widget is live.
  const off = demitable.on('vault.changed', () => { refresh(); });

  // The app's context moved: a different note is in view.
  demitable.on('context.changed', (e) => {
    wrap.dataset.note = e.noteUid ?? '';
    refresh();
  });

  // A client-side command. The core routes it back to this browser when
  // the palette invokes it.
  await demitable.command.register({
    id: 'recap.refresh',
    title: 'Recap: refresh',
    description: 'Re-read the latest session logs.',
  });

  demitable.on('command.invoked', (e) => {
    if (e.id === 'recap.refresh') {
      refresh();
      demitable.emit('refreshed', { at: new Date().toISOString() });
    }
  });

  // An event handler identifies the real element with composedPath()[0],
  // never event.target — event.target is retargeted to the shadow host
  // (R1 in docs/shadow-dom-verification.md).
  list.addEventListener('click', (event) => {
    const target = event.composedPath()[0];
    if (target instanceof HTMLElement && target.dataset.uid) {
      demitable.emit('opened', { uid: target.dataset.uid });
    }
  });
}
