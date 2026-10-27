// WebSocket client (F5a). Authenticates via ?token= (browser WS API
// cannot set headers). Handles the server's push types and triggers
// refetches; never applies authoritative state from the client.

export interface WSEnvelope {
  v: number;
  type: string;
  id?: string;
  data?: any;
  principal?: any;
}

export type WSEventHandler = (env: WSEnvelope) => void;

export interface WSClientOptions {
  token?: string | null;
  onVaultChanged?: (data: any) => void;
  onSecretRevealed?: (data: any) => void;
  onNoteSaved?: (data: any) => void;
  onRollResult?: (data: any) => void;
  onSessionState?: (data: any) => void;
  onSessionPatch?: (data: any) => void;
  onError?: (data: any) => void;
  onHello?: (data: any) => void;
  onMessage?: WSEventHandler;
  reconnect?: boolean;
}

export class WSClient {
  private ws: WebSocket | null = null;
  private opts: WSClientOptions;
  private url: string;
  private shouldReconnect = true;
  private reconnectTimer: any = null;

  constructor(opts: WSClientOptions = {}) {
    this.opts = opts;
    const token = opts.token ?? (typeof localStorage !== "undefined" ? localStorage.getItem("demitable_token") : null);
    const proto = typeof location !== "undefined" && location.protocol === "https:" ? "wss" : "ws";
    const host = typeof location !== "undefined" ? location.host : "localhost";
    this.url = token ? `${proto}://${host}/ws?token=${encodeURIComponent(token)}` : `${proto}://${host}/ws`;
    this.shouldReconnect = opts.reconnect !== false;
  }

  connect(): void {
    if (this.ws) return;
    const ws = new WebSocket(this.url);
    this.ws = ws;
    ws.onmessage = (ev) => {
      let env: WSEnvelope;
      try {
        env = JSON.parse(ev.data);
      } catch {
        return;
      }
      this.opts.onMessage?.(env);
      switch (env.type) {
        case "hello":
          this.opts.onHello?.(env.data);
          break;
        case "vault.changed":
          this.opts.onVaultChanged?.(env.data);
          break;
        case "secret.revealed":
          this.opts.onSecretRevealed?.(env.data);
          break;
        case "note.saved":
          this.opts.onNoteSaved?.(env.data);
          break;
        case "roll.result":
          this.opts.onRollResult?.(env.data);
          break;
        case "session.state":
          this.opts.onSessionState?.(env.data);
          break;
        case "session.patch":
          this.opts.onSessionPatch?.(env.data);
          break;
        case "error":
          this.opts.onError?.(env.data);
          break;
      }
    };
    ws.onclose = () => {
      this.ws = null;
      if (this.shouldReconnect) {
        this.reconnectTimer = setTimeout(() => this.connect(), 1000);
      }
    };
    ws.onerror = () => {
      /* ignore; onclose handles reconnect */
    };
  }

  send(type: string, data: any = {}, id?: string): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return;
    this.ws.send(JSON.stringify({ v: 1, type, id, data }));
  }

  revealSecret(noteUid: string, blockId: string, audience: any = { all: true }): void {
    this.send("secret.reveal", { note_uid: noteUid, block_id: blockId, audience });
  }

  ping(): void {
    this.send("ping");
  }

  close(): void {
    this.shouldReconnect = false;
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.ws?.close();
    this.ws = null;
  }
}

export function connectWS(opts: WSClientOptions = {}): WSClient {
  const c = new WSClient(opts);
  c.connect();
  return c;
}

// Secret-reveal helper used by UI.
export function revealSecret(noteUid: string, blockId: string, audience?: any): void {
  // no-op standalone; callers use a client instance
}
