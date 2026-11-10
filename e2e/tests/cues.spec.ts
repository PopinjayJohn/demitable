import { test, expect } from '@playwright/test';
import { bootServer, createSession, mintToken } from '../helpers/boot';

test.describe('cues surface', () => {
  test('mood drives cue selection display', async ({ request }) => {
    const srv = await bootServer();
    try {
      const sid = await createSession(srv.url, srv.gmToken);

      // configure cues for this session scope
      const cfg = await request.post(srv.url + '/api/v1/sessions/' + sid + '/cues', {
        headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + srv.gmToken },
        data: { cues: [{ mood: 'calm', src: '/tmp/calm.mp3', title: 'Calm' }] },
      });
      expect(cfg.status()).toBe(200);

      // mood endpoint returns a tag
      const mood = await request.post(srv.url + '/api/v1/sessions/' + sid + '/mood', {
        headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + srv.gmToken },
        data: { keywords: ['calm'] },
      });
      expect(mood.status()).toBe(200);
      const mj = await mood.json();
      expect(mj.mood || mj.tag || mj.value).toBeTruthy();

      // select cue by mood
      const sel = await request.post(srv.url + '/api/v1/sessions/' + sid + '/cues/select', {
        headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + srv.gmToken },
        data: { mood: 'calm' },
      });
      expect(sel.status()).toBe(200);
      const sj = await sel.json();
      const selected = sj.selected || sj.cue || sj;
      expect(String(selected.src || selected.track || '')).toContain('calm');
    } finally {
      await srv.cleanup();
    }
  });

  test('cross-session 401 posture for cues', async ({ request }) => {
    const srv = await bootServer();
    try {
      const sidA = await createSession(srv.url, srv.gmToken);
      const sidB = await createSession(srv.url, srv.gmToken);

      // create a real seat bound to session A via GM join
      const jr = await request.post(srv.url + '/api/v1/sessions/' + sidA + '/join', {
        headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + srv.gmToken },
        data: {},
      });
      expect(jr.status()).toBe(200);
      const jj = await jr.json();
      const seatToken = jj.token as string;

      // reading session B cues with session-A seat -> 401
      const r = await request.get(srv.url + '/api/v1/sessions/' + sidB + '/cues', {
        headers: { Authorization: 'Bearer ' + seatToken },
      });
      expect(r.status()).toBe(401);
    } finally {
      await srv.cleanup();
    }
  });
});
