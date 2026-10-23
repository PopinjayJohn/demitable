// The spike's browser verification.
//
//   node spikes/plugin-host/verify/browser.mjs
//
// It drives the spike in a real Chromium and asserts the things the spike
// is about:
//
//   1. a shadow surface mounts into a shadow root the app created;
//   2. a light-DOM surface mounts into the app's document;
//   3. the difference is visible — the app's rules reach the light surface
//      and do not reach the shadow surface;
//   4. event.target is retargeted and composedPath()[0] is not;
//   5. the capability model refuses a call the manifest did not ask for;
//   6. the role scope holds — a player sees two session notes, not three.
//
// It writes a screenshot to spikes/plugin-host/verify/evidence.png.

import { chromium } from '/tmp/opencode/node_modules/playwright/index.mjs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const BASE = process.env.SPIKE_BASE ?? 'http://127.0.0.1:8777';

let failures = 0;
const check = (name, ok, detail = '') => {
  if (ok) {
    console.log(`  ok   ${name}`);
  } else {
    failures++;
    console.log(`  FAIL ${name}${detail ? ` — ${detail}` : ''}`);
  }
};

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1280, height: 760 } });
page.on('pageerror', (e) => console.log('  pageerror:', e.message));

// ---------------------------------------------------------------------------
console.log('\n1. the GM view');
// ---------------------------------------------------------------------------
await page.goto(`${BASE}/?role=gm`);
await page.waitForFunction(() => {
  const host = document.getElementById('mount-sidebar.right');
  return host && host.shadowRoot && host.shadowRoot.querySelector('.recap-card');
}, null, { timeout: 10_000 });

const shadow = await page.evaluate(() => {
  const host = document.getElementById('mount-sidebar.right');
  const chip = host.shadowRoot.querySelector('.chip');
  const card = host.shadowRoot.querySelector('.recap-card');
  return {
    hasRoot: !!host.shadowRoot,
    rootMode: host.shadowRoot ? 'open' : null,
    chips: host.shadowRoot.querySelectorAll('.chip').length,
    cards: host.shadowRoot.querySelectorAll('.recap-card').length,
    // The app's chip is a filled, round pill. The plugin's chip, inside the
    // shadow root, is square and outlined.
    chipRadius: getComputedStyle(chip).borderRadius,
    chipBackground: getComputedStyle(chip).backgroundColor,
    cardBorder: getComputedStyle(card).borderTopLeftRadius,
    // Does the app's own stylesheet reach inside?
    appRuleReachesIn: getComputedStyle(chip).borderTopWidth,
  };
});
check('the app created the shadow root', shadow.hasRoot && shadow.rootMode === 'open');
check('the shadow surface rendered its cards', shadow.cards === 2, JSON.stringify(shadow));
check('the app\'s .chip rule does NOT reach into the shadow root',
  shadow.chipRadius !== '999px', `radius was ${shadow.chipRadius}`);

const light = await page.evaluate(() => {
  const host = document.getElementById('mount-note.sidebar');
  const el = host.querySelector('demitable-recap-inline');
  const chip = el ? el.querySelector('.chip') : null;
  return {
    hasElement: !!el,
    chips: el ? el.querySelectorAll('.chip').length : 0,
    radius: chip ? getComputedStyle(chip).borderRadius : null,
    background: chip ? getComputedStyle(chip).backgroundColor : null,
    // A light-DOM surface sits in the app's document, so the app's rules
    // match it.
    inDocument: el ? document.contains(el) : false,
  };
});
check('the light-DOM surface mounted a custom element', light.hasElement);
check('the app\'s .chip rule DOES reach the light surface',
  light.radius === '999px', `radius was ${light.radius}`);
check('the two chips look different', shadow.chipRadius !== light.radius,
  `shadow ${shadow.chipRadius} vs light ${light.radius}`);

// ---------------------------------------------------------------------------
console.log('\n2. event.target is retargeted; composedPath()[0] is not');
// ---------------------------------------------------------------------------
const retarget = await page.evaluate(async () => {
  const host = document.getElementById('mount-sidebar.right');
  const card = host.shadowRoot.querySelector('.recap-card');
  const seen = {};
  card.addEventListener('click', (e) => {
    seen.target = e.target.tagName + (e.target.dataset?.uid ? `[${e.target.dataset.uid}]` : '');
    seen.path0 = e.composedPath()[0].tagName + (e.composedPath()[0].dataset?.uid ? `[${e.composedPath()[0].dataset.uid}]` : '');
  }, { once: true });
  card.click();
  await new Promise((r) => setTimeout(r, 50));
  return seen;
});
check('event.target was retargeted to the host', retarget.target === 'DIV' && !retarget.target.includes('n-session'),
  `target was ${retarget.target}`);
check('composedPath()[0] is the real element', (retarget.path0 || '').includes('n-session'),
  `path0 was ${retarget.path0}`);

// ---------------------------------------------------------------------------
console.log('\n3. the capability model');
// ---------------------------------------------------------------------------
const caps = await page.evaluate(async () => {
  const bridge = window.__demitableBridge;
  const out = {};
  try {
    await bridge.call('vault.write', { uid: 'n-session-1' });
    out.write = 'allowed';
  } catch (e) {
    out.write = e.code;
  }
  try {
    await bridge.call('vault.read', { uid: 'n-character-1' });
    out.outOfScope = 'allowed';
  } catch (e) {
    out.outOfScope = e.code;
  }
  try {
    const res = await bridge.call('vault.query', { type: 'session' });
    out.count = res.count;
  } catch (e) {
    out.count = 'error:' + e.code;
  }
  try {
    await bridge.call('vault.teleport', {});
    out.unknown = 'allowed';
  } catch (e) {
    out.unknown = e.code;
  }
  return out;
});
check('vault.write was refused', caps.write === 'ungranted_capability', `got ${caps.write}`);
check('a note outside the scope was refused', caps.outOfScope === 'forbidden', `got ${caps.outOfScope}`);
check('an unknown operation was refused', caps.unknown === 'invalid', `got ${caps.unknown}`);
check('the GM sees three session notes', caps.count === 3, `got ${caps.count}`);

// ---------------------------------------------------------------------------
console.log('\n4. the player view');
// ---------------------------------------------------------------------------
await page.goto(`${BASE}/?role=account`);
await page.waitForFunction(() => {
  const host = document.getElementById('mount-sidebar.right');
  return host && host.shadowRoot && host.shadowRoot.querySelector('.recap-card');
}, null, { timeout: 10_000 });

const player = await page.evaluate(async () => {
  const bridge = window.__demitableBridge;
  const res = await bridge.call('vault.query', { type: 'session' });
  let hidden = null;
  try {
    await bridge.call('vault.read', { uid: 'n-session-3' });
    hidden = 'allowed';
  } catch (e) {
    hidden = e.code;
  }
  const host = document.getElementById('mount-sidebar.right');
  return {
    count: res.count,
    hidden,
    cards: host.shadowRoot.querySelectorAll('.recap-card').length,
  };
});
check('the player sees two session notes, not three', player.count === 2, `got ${player.count}`);
check('the GM-only note is not_found for a player', player.hidden === 'not_found', `got ${player.hidden}`);
check('the player\'s widget shows two cards', player.cards === 2, `got ${player.cards}`);

// ---------------------------------------------------------------------------
console.log('\n5. the command palette');
// ---------------------------------------------------------------------------
const commands = await page.evaluate(async () => {
  const res = await fetch('/api/commands');
  return (await res.json()).commands.map((c) => `${c.id}:${c.source}`);
});
check('the client command was registered', commands.some((c) => c.startsWith('recap.refresh:client')),
  JSON.stringify(commands));

await page.goto(`${BASE}/?role=gm`);
await page.waitForFunction(() => {
  const host = document.getElementById('mount-sidebar.right');
  return host && host.shadowRoot && host.shadowRoot.querySelector('.recap-card');
}, null, { timeout: 10_000 });
const gmCommands = await page.evaluate(async () => {
  const res = await fetch('/api/commands');
  return (await res.json()).commands.map((c) => `${c.id}:${c.source}`);
});
check('the server command is registered for the GM only',
  gmCommands.includes('export.campaign:server'), JSON.stringify(gmCommands));

// ---------------------------------------------------------------------------
console.log('\n6. evidence');
// ---------------------------------------------------------------------------
await page.screenshot({ path: join(here, 'evidence.png'), fullPage: true });
console.log(`  wrote ${join(here, 'evidence.png')}`);

await browser.close();

console.log(`\n${failures === 0 ? 'all checks passed' : `${failures} check(s) failed`}`);
process.exit(failures === 0 ? 0 : 1);
