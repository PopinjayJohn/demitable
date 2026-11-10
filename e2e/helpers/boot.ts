import { spawn, execFileSync, type ChildProcess } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const BIN =
  process.env.DEMITABLE_BIN || path.resolve(__dirname, '..', '..', 'dist', 'demitable');

export interface BootedServer {
  url: string;
  gmToken: string;
  tokenKey: string;
  dir: string;
  proc: ChildProcess;
  cleanup: () => Promise<void>;
}

function readConfig(dir: string): { secret: string; tokenKey: string } {
  const text = fs.readFileSync(path.join(dir, '.demitable', 'config.yaml'), 'utf8');
  const sm = text.match(/gm_secret:\s*(.+)/);
  const tm = text.match(/token_key:\s*(.+)/);
  return {
    secret: (sm ? sm[1] : '').trim(),
    tokenKey: (tm ? tm[1] : '').trim(),
  };
}

async function waitForHealth(url: string): Promise<void> {
  for (let i = 0; i < 100; i++) {
    try {
      const r = await fetch(url + '/api/health');
      if (r.ok) return;
    } catch {
      // retry
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error('health timeout for ' + url);
}

export async function bootServer(): Promise<BootedServer> {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'demitable-e2e-'));
  execFileSync(BIN, ['init', dir], { stdio: 'ignore' });
  const sessionDb = path.join(dir, 'sessions.db');
  const proc = spawn(BIN, ['--vault', dir, '--port', '0', '--no-browser'], {
    env: { ...process.env, DEMITABLE_SESSION_DB: sessionDb },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  const url = await new Promise<string>((resolve, reject) => {
    let buf = '';
    const timer = setTimeout(() => reject(new Error('boot timeout')), 20000);
    proc.stdout.on('data', (d: Buffer) => {
      buf += d.toString();
      const m = buf.match(/http:\/\/127\.0\.0\.1:\d+/);
      if (m) {
        clearTimeout(timer);
        resolve(m[0]);
      }
    });
    proc.on('exit', (code) => {
      clearTimeout(timer);
      reject(new Error('server exited ' + String(code)));
    });
  });
  await waitForHealth(url);
  // ensure config exists (server creates on first auth call too)
  // trigger auth state by reading config first
  let cfg = readConfig(dir);
  if (!cfg.secret) {
    // call gm auth? config should exist; retry after a moment
    await new Promise((r) => setTimeout(r, 200));
    cfg = readConfig(dir);
  }
  const res = await fetch(url + '/api/v1/auth/gm', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ secret: cfg.secret }),
  });
  if (!res.ok) throw new Error('gm auth failed: ' + res.status);
  const body = (await res.json()) as { token: string };
  const cleanup = async () => {
    try {
      proc.kill('SIGTERM');
    } catch {
      // ignore
    }
    await new Promise<void>((resolve) => {
      const t = setTimeout(resolve, 5000);
      proc.on('exit', () => {
        clearTimeout(t);
        resolve();
      });
    });
    try {
      fs.rmSync(dir, { recursive: true, force: true });
    } catch {
      // ignore
    }
  };
  return { url, gmToken: body.token, tokenKey: cfg.tokenKey, dir, proc, cleanup };
}

export async function createSession(url: string, gmToken: string, name = 'e2e'): Promise<string> {
  const res = await fetch(url + '/api/v1/sessions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + gmToken },
    body: JSON.stringify({ name, campaign: '[[Campaign]]' }),
  });
  if (!res.ok) throw new Error('create session failed: ' + res.status);
  const body = (await res.json()) as { id: string };
  return body.id;
}

export function mintToken(tokenKey: string, claims: Record<string, unknown>): string {
  const payload = { ...claims, exp: Math.floor(Date.now() / 1000) + 86400 };
  const p = Buffer.from(JSON.stringify(payload)).toString('base64url');
  const sig = crypto.createHmac('sha256', tokenKey).update(p).digest('base64url');
  return p + '.' + sig;
}

export async function uploadAudio(
  url: string,
  gmToken: string,
  sessionId: string,
  bytes: Uint8Array,
  chunk: number,
  total: number,
): Promise<Response> {
  return fetch(
    url + '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/audio?chunk=' + chunk + '&total=' + total,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/octet-stream', Authorization: 'Bearer ' + gmToken },
      body: bytes as any,
    },
  );
}

export const ENGINE_SCRIPT_ARGS = [
  '-c',
  "cat >/dev/null; printf '%s\\n' '{\"segments\":[{\"text\":\"we meet Ilya\",\"start\":0,\"end\":2,\"speaker\":\"GM\"},{\"text\":\"hello\",\"start\":2,\"end\":4,\"speaker\":\"Player\"}]}'",
];
