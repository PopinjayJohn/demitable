// The shell.
//
// It plays the part of the app: it creates the host elements, evaluates the
// `when:` condition, mounts the surfaces, keeps the command palette, and
// routes events.

import { createBridge } from './bridge.js';
import { mountSurface, unmountSurface } from './host.js';

const params = new URLSearchParams(location.search);
const role = params.get('role') ?? 'gm';
const clientId = `c-${Math.random().toString(36).slice(2, 9)}`;

const log = (el, text, cls) => {
  const li = document.createElement('li');
  if (cls) li.className = cls;
  li.textContent = text;
  el.prepend(li);
};

const state = {
  bootstrap: null,
  bridge: null,
  ctx: { noteUid: '', noteType: '', campaign: '', path: '' },
  mounted: new Set(),
};

function hostFor(mount) {
  return document.getElementById(`mount-${mount}`);
}

/** A tiny evaluator for the `when:` expression.
 *
 *  The real one is the pack's formula language (ADR-0003), which is M2's
 *  job. The manifest validator needs it, which is why M2 precedes M3. */
function whenMatches(expr, ctx) {
  if (!expr || expr === 'always') return true;
  const m = /^\s*note\.type\s*(==|!=)\s*'([^']*)'\s*$/.exec(expr);
  if (!m) return true;
  const [, op, want] = m;
  return op === '==' ? ctx.noteType === want : ctx.noteType !== want;
}

async function syncSurfaces() {
  const { surfaces } = state.bootstrap;
  for (const surface of surfaces) {
    const hostEl = hostFor(surface.mount);
    if (!hostEl) continue;
    const should = whenMatches(surface.when, state.ctx);
    const isMounted = state.mounted.has(surface.id);
    if (should && !isMounted) {
      try {
        await mountSurface(surface, hostEl, state.ctx, state.bridge);
        state.mounted.add(surface.id);
        log(document.getElementById('events'),
          `mounted ${surface.id} (${surface.dom}) into ${surface.mount}`,
          'src');
      } catch (err) {
        log(document.getElementById('events'), `mount failed ${surface.id}: ${err.message}`, 'src');
      }
    } else if (!should && isMounted) {
      unmountSurface(surface, hostEl);
      state.mounted.delete(surface.id);
      log(document.getElementById('events'), `unmounted ${surface.id}`, 'src');
    }
  }
}

async function renderCommands() {
  const list = document.getElementById('commands');
  list.replaceChildren();
  const res = await fetch('/api/commands');
  const { commands } = await res.json();
  for (const c of commands) {
    const li = document.createElement('li');
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.textContent = c.title;
    btn.addEventListener('click', async () => {
      await fetch('/api/commands/invoke', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ id: c.id, args: {} }),
      });
    });
    const src = document.createElement('span');
    src.className = 'src';
    src.textContent = `${c.source} · ${c.plugin}`;
    li.append(btn, src);
    list.appendChild(li);
  }
}

async function setNote(uid) {
  const res = await fetch('/api/context', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ noteUid: uid }),
  });
  const body = await res.json();
  state.ctx = body.context;
  document.getElementById('note-title').textContent = body.context.title ?? '—';
  await syncSurfaces();
}

async function boot() {
  const res = await fetch(`/api/bootstrap?role=${encodeURIComponent(role)}`);
  state.bootstrap = await res.json();

  const notesRes = await fetch('/api/notes');
  const { notes } = await notesRes.json();
  const sel = document.getElementById('note');
  sel.replaceChildren();
  for (const n of notes) {
    const o = document.createElement('option');
    o.value = n.uid;
    o.textContent = `${n.title} (${n.type})`;
    sel.appendChild(o);
  }
  sel.addEventListener('change', () => setNote(sel.value));

  state.bridge = createBridge({
    pluginId: state.bootstrap.pluginId,
    clientId,
  });

  // The event stream. One per client.
  const es = new EventSource('/api/events');
  es.onmessage = (ev) => {
    const event = JSON.parse(ev.data);
    state.bridge.dispatch(event);
    const label = event.channel === 'core'
      ? event.name
      : `${event.channel}:${event.name}`;
    log(document.getElementById('events'), `${label} ${JSON.stringify(event.payload ?? {})}`);
  };

  await renderCommands();
  if (notes.length > 0) {
    sel.value = notes[0].uid;
    await setNote(notes[0].uid);
  }

  document.getElementById('role').value = role;
  document.getElementById('role').addEventListener('change', (e) => {
    location.search = `?role=${e.target.value}`;
  });
  document.getElementById('touch').addEventListener('click', async () => {
    await fetch('/api/touch', { method: 'POST' });
  });
  document.getElementById('reload').addEventListener('click', () => location.reload());
}

boot();
