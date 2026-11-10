import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { bootServer, createSession, uploadAudio } from '../helpers/boot';
import { fileURLToPath } from 'node:url';
const __dirname = path.dirname(fileURLToPath(import.meta.url));

const CONSENT_COPY =
  'Audio is recorded. Enabling capture records audio of the table and stores it on this machine; obtain the table\'s consent first.';

test.describe('capture surface', () => {
  test('consent string is pinned verbatim', async () => {
    const src = fs.readFileSync(
      path.resolve(__dirname, '..', '..', 'web', 'src', 'lib', 'capture.ts'),
      'utf8',
    );
    expect(src).toContain('export const CONSENT_COPY =');
    const m = src.match(/export const CONSENT_COPY =\s*\n?\s*"([^"]*)"/);
    expect(m).not.toBeNull();
    expect(m![1]).toBe(CONSENT_COPY);
  });

  test('chunked upload assembles and GM download works; excluded caller gets 404', async ({ request }) => {
    const srv = await bootServer();
    try {
      const sid = await createSession(srv.url, srv.gmToken);
      const chunk = new Uint8Array([1, 2, 3, 4]);
      const r0 = await uploadAudio(srv.url, srv.gmToken, sid, chunk, 0, 2);
      expect(r0.status).toBe(200);
      const j0 = await r0.json();
      expect(j0.assembled).toBe(false);
      const r1 = await uploadAudio(srv.url, srv.gmToken, sid, chunk, 1, 2);
      expect(r1.status).toBe(200);
      const j1 = await r1.json();
      expect(j1.assembled).toBe(true);

      // GM download returns bytes
      const dl = await request.get(srv.url + '/api/v1/sessions/' + sid + '/audio', {
        headers: { Authorization: 'Bearer ' + srv.gmToken },
      });
      expect(dl.status()).toBe(200);
      const buf = await dl.body();
      expect(buf.length).toBe(8);

      // excluded caller (no token) -> 404
      const anon = await request.get(srv.url + '/api/v1/sessions/' + sid + '/audio');
      expect(anon.status()).toBe(404);
      const ab = await anon.body();
      expect(ab.length).toBe(0);
    } finally {
      await srv.cleanup();
    }
  });
});
