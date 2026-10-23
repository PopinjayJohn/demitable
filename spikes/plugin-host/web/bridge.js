// The browser-side bridge.
//
// Two methods, deliberately:
//
//     demitable.call(op, args)   a *capability* operation. Gated: the host
//                                checks the plugin's manifest and its role.
//     demitable.host(op, args)   a *host primitive*. Ungated: it grants no
//                                reach into anything the role cannot already
//                                see.
//
// The vocabulary of each is fixed and small:
//
//     capabilities   vault.read, vault.write, vault.query, vault.assets,
//                    vault.watch, http.client, log
//     host prims    context.get, pref.get, pref.set, command.register

const JSON_HEADERS = { 'content-type': 'application/json' };

export function createBridge({ pluginId, clientId, base = '' }) {
  const listeners = new Map();
  let readyPromise = null;

  function on(name, handler) {
    if (!listeners.has(name)) listeners.set(name, new Set());
    listeners.get(name).add(handler);
    return () => listeners.get(name)?.delete(handler);
  }

  async function post(path, body) {
    const res = await fetch(base + path, {
      method: 'POST',
      headers: JSON_HEADERS,
      body: JSON.stringify(body),
    });
    const envelope = await res.json();
    if (!envelope.ok) {
      const err = new Error(envelope.error.message);
      err.code = envelope.error.code;
      err.detail = envelope.error.detail;
      throw err;
    }
    return envelope.data;
  }

  const bridge = {
    pluginId,

    ready() {
      if (!readyPromise) {
        readyPromise = fetch(`${base}/api/ready?clientId=${encodeURIComponent(clientId)}`)
          .then((r) => r.json())
          .then((r) => {
            if (r.error) {
              const err = new Error(r.error.message);
              err.code = r.error.code;
              throw err;
            }
            return r;
          });
      }
      return readyPromise;
    },

    /** A capability operation. Rejects if the capability was not granted. */
    call(op, args) {
      return post('/api/call', { op, args: args ?? {}, clientId });
    },

    /** A host primitive. Not gated. */
    host(op, args) {
      return post('/api/host', { op, args: args ?? {}, clientId });
    },

    on,

    emit(name, payload) {
      return post('/api/emit', { name, payload: payload ?? {} });
    },

    context() {
      return bridge.host('context.get');
    },

    pref: {
      get(key) {
        return bridge.host('pref.get', { key });
      },
      set(key, value) {
        return bridge.host('pref.set', { key, value });
      },
    },

    command: {
      register(cmd) {
        return bridge.host('command.register', cmd);
      },
    },

    log(level, message) {
      return bridge.call('log', { level, message });
    },

    /** Called by the shell when an event arrives on the SSE stream. */
    dispatch(event) {
      const keys = [event.name];
      if (event.channel && event.channel !== 'core') {
        keys.push(`${event.channel}:${event.name}`);
      }
      for (const key of keys) {
        const set = listeners.get(key);
        if (!set) continue;
        for (const handler of [...set]) {
          try {
            handler(event.payload ?? {}, event);
          } catch (err) {
            console.error('bridge handler failed', key, err);
          }
        }
      }
    },
  };

  return bridge;
}
