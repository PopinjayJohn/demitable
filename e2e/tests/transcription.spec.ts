import { test, expect } from '@playwright/test';
import { bootServer, createSession, uploadAudio, ENGINE_SCRIPT_ARGS } from '../helpers/boot';

test.describe('transcription surface', () => {
  test('review snapshot renders after transcribe with custom engine', async ({ request }) => {
    const srv = await bootServer();
    try {
      const sid = await createSession(srv.url, srv.gmToken);
      const chunk = new Uint8Array([10, 20, 30]);
      await uploadAudio(srv.url, srv.gmToken, sid, chunk, 0, 1);

      const tr = await request.post(srv.url + '/api/v1/sessions/' + sid + '/transcribe', {
        headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + srv.gmToken },
        data: { engine: 'custom', custom_command: '/bin/sh', args: ENGINE_SCRIPT_ARGS, opt_in: true },
      });
      // args include -c and script; pass via args
      expect(tr.status()).toBe(200);
      const tj = await tr.json();
      expect(tj.count).toBe(2);

      const rev = await request.get(srv.url + '/api/v1/sessions/' + sid + '/transcript/review', {
        headers: { Authorization: 'Bearer ' + srv.gmToken },
      });
      expect(rev.status()).toBe(200);
      const rj = await rev.json();
      expect(Array.isArray(rj.segments)).toBe(true);
      expect(rj.segments.length).toBe(2);
    } finally {
      await srv.cleanup();
    }
  });

  test('missing engine returns 503 with message', async ({ request }) => {
    const srv = await bootServer();
    try {
      const sid = await createSession(srv.url, srv.gmToken);
      const chunk = new Uint8Array([10, 20, 30]);
      await uploadAudio(srv.url, srv.gmToken, sid, chunk, 0, 1);

      const tr = await request.post(srv.url + '/api/v1/sessions/' + sid + '/transcribe', {
        headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + srv.gmToken },
        data: {},
      });
      expect(tr.status()).toBe(503);
      const tj = await tr.json();
      expect(String(tj.message)).toContain('Transcription engine is not installed');
    } finally {
      await srv.cleanup();
    }
  });

  test('non-GM denied', async ({ request }) => {
    const srv = await bootServer();
    try {
      const sid = await createSession(srv.url, srv.gmToken);
      const r = await request.get(srv.url + '/api/v1/sessions/' + sid + '/transcript');
      expect(r.status()).not.toBe(200);
    } finally {
      await srv.cleanup();
    }
  });
});
