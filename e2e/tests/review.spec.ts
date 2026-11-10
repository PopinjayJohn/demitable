import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import { bootServer, createSession, uploadAudio, ENGINE_SCRIPT_ARGS } from '../helpers/boot';

async function seedTranscript(request: any, srv: any, sid: string) {
  const chunk = new Uint8Array([10, 20, 30]);
  await uploadAudio(srv.url, srv.gmToken, sid, chunk, 0, 1);
  const tr = await request.post(srv.url + '/api/v1/sessions/' + sid + '/transcribe', {
    headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + srv.gmToken },
    data: { engine: 'custom', custom_command: '/bin/sh', args: ENGINE_SCRIPT_ARGS, opt_in: true },
  });
  expect(tr.status()).toBe(200);
}

test.describe('review surface', () => {
  test('review -> promote -> propose -> marked section visible', async ({ request }) => {
    const srv = await bootServer();
    try {
      const sid = await createSession(srv.url, srv.gmToken);
      await seedTranscript(request, srv, sid);

      const rev = await request.get(srv.url + '/api/v1/sessions/' + sid + '/transcript/review', {
        headers: { Authorization: 'Bearer ' + srv.gmToken },
      });
      expect(rev.status()).toBe(200);
      const rj = await rev.json();
      expect(rj.segments.length).toBe(2);

      const prom = await request.post(srv.url + '/api/v1/sessions/' + sid + '/transcript/promote', {
        headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + srv.gmToken },
        data: { term: 'Ilya', from: 'Ilya' },
      });
      expect(prom.status()).toBe(200);
      const pj = await prom.json();
      expect(pj.promoted).toBe('Ilya');

      const prop = await request.post(srv.url + '/api/v1/sessions/' + sid + '/session-log/propose', {
        headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + srv.gmToken },
        data: {},
      });
      expect(prop.status()).toBe(200);
      const pp = await prop.json();
      expect(pp.applied).toBe(true);

      // marked section visible on disk
      const notePath = pp.path as string;
      const full = srv.dir + '/' + notePath;
      const content = fs.readFileSync(full, 'utf8');
      expect(content).toContain('<!-- sp:begin-session-log -->');
    } finally {
      await srv.cleanup();
    }
  });

  test('closed session surfaces 409', async ({ request }) => {
    const srv = await bootServer();
    try {
      const sid = await createSession(srv.url, srv.gmToken);
      await seedTranscript(request, srv, sid);

      const closeRes = await request.post(srv.url + '/api/v1/sessions/' + sid + '/close', {
        headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + srv.gmToken },
        data: {},
      });
      expect(closeRes.status()).toBe(200);

      const rev = await request.post(srv.url + '/api/v1/sessions/' + sid + '/transcript/review', {
        headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + srv.gmToken },
        data: {},
      });
      expect(rev.status()).toBe(409);
    } finally {
      await srv.cleanup();
    }
  });
});
