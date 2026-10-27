import { test, expect } from '@playwright/test';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { pathToFileURL } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const fixtureUrl = pathToFileURL(
  path.resolve(__dirname, '..', 'fixtures', 'shadow-host.html'),
).href;

test.beforeEach(async ({ page }) => {
  await page.goto(fixtureUrl);
});

test.describe('ADR-0029 shadow-host contract', () => {
  test('R1: composedPath()[0] is the real inner element; event.target is the host', async ({ page }) => {
    const result = await page.evaluate(() => {
      const host = (window as any).__host;
      const shadow = (window as any).__shadow;
      const inner = (window as any).__inner;
      let seenTarget: any = null;
      let seenPath0: any = null;
      document.addEventListener('click', (e) => {
        seenTarget = e.target;
        seenPath0 = (e as any).composedPath()[0];
      }, true);
      inner.click();
      return { targetIsHost: seenTarget === host, path0IsInner: seenPath0 === inner };
    });
    expect(result.targetIsHost).toBe(true);
    expect(result.path0IsInner).toBe(true);
  });

  test('R1b: keyboard event crosses shadow boundary; target retargeted to host', async ({ page }) => {
    const result = await page.evaluate(() => {
      const host = (window as any).__host;
      const inner = (window as any).__inner;
      let docSaw = false;
      let targetAtDoc: any = null;
      document.addEventListener('keydown', (e) => { docSaw = true; targetAtDoc = e.target; }, true);
      inner.focus();
      const ev = new KeyboardEvent('keydown', { key: 'a', bubbles: true, composed: true });
      inner.dispatchEvent(ev);
      return { docSaw, targetAtDocIsHost: targetAtDoc === host };
    });
    expect(result.docSaw).toBe(true);
    expect(result.targetAtDocIsHost).toBe(true);
  });

  test('R2: selection path works per browser (chromium: shadow.getSelection; firefox: document.getSelection)', async ({ page, browserName }) => {
    const result = await page.evaluate(() => {
      const shadow = (window as any).__shadow;
      const span = shadow.getElementById('span-elem');
      const tn = span.firstChild;
      const hasShadowSel = typeof shadow.getSelection === 'function';
      let shadowToString = '';
      let shadowAnchorIsReal = false;
      if (hasShadowSel) {
        const ssel = shadow.getSelection();
        try { ssel.setBaseAndExtent(tn, 0, tn, 8); } catch (e) {}
        shadowToString = ssel.toString();
        shadowAnchorIsReal = ssel.anchorNode === tn;
      }
      // document-level selection
      const dsel = window.getSelection();
      if (dsel) {
        try { dsel.setBaseAndExtent(tn, 0, tn, 8); } catch (e) {}
      }
      return {
        hasShadowSel,
        shadowToString,
        shadowAnchorIsReal,
        docToString: dsel?.toString() ?? '',
        docAnchorIsReal: dsel?.anchorNode === tn,
      };
    });

    if (browserName === 'chromium') {
      // Chromium: shadow.getSelection exists and returns the real node;
      // document.getSelection is retargeted (anchorNode !== tn) but toString works.
      expect(result.hasShadowSel).toBe(true);
      expect(result.shadowToString).toBe('SpanText');
      expect(result.shadowAnchorIsReal).toBe(true);
    } else if (browserName === 'firefox') {
      // Firefox: no shadow.getSelection; document.getSelection returns the real node directly.
      expect(result.hasShadowSel).toBe(false);
      expect(result.docToString).toBe('SpanText');
      expect(result.docAnchorIsReal).toBe(true);
    } else {
      // WebKit: record actual behaviour (may differ); at minimum toString works via one path.
      expect(result.docToString === 'SpanText' || result.shadowToString === 'SpanText').toBe(true);
    }
  });

  test('R4: unregistered custom property on :root resolves inside shadow root', async ({ page }) => {
    const result = await page.evaluate(() => {
      const shadow = (window as any).__shadow;
      const brand = shadow.getElementById('brand') as HTMLElement;
      return { color: getComputedStyle(brand).color };
    });
    expect(result.color).toBe('rgb(0, 128, 0)');
  });

  test('R5: outer-tree CSS beats shadow :host at equal importance (scope beats specificity)', async ({ page }) => {
    const result = await page.evaluate(() => {
      const host = (window as any).__host;
      return { color: getComputedStyle(host).color };
    });
    // outer #host rule rgb(4,5,6) should win over shadow :host rgb(100,100,100)
    expect(result.color).toBe('rgb(4, 5, 6)');
  });

  test('R6: all:initial on :host sets display:inline (documented gotcha)', async ({ page }) => {
    // `all: initial` resets inherited values and sets display:inline; a later
    // display declaration restores the layout, and custom properties are not
    // reset by `all`.
    const result = await page.evaluate(() => {
      const hostReset = (window as any).__hostReset as HTMLElement;
      const hostResetFixed = (window as any).__hostResetFixed as HTMLElement;
      const shadowReset = hostReset.shadowRoot as ShadowRoot;
      const brand = shadowReset.getElementById('reset-brand') as HTMLElement;
      return {
        resetDisplay: getComputedStyle(hostReset).display,
        fixedDisplay: getComputedStyle(hostResetFixed).display,
        brandColor: getComputedStyle(brand).color,
      };
    });
    expect(result.resetDisplay).toBe('inline');
    expect(result.fixedDisplay).toBe('block');
    // custom properties survive `all: initial` on the host
    expect(result.brandColor).toBe('rgb(0, 128, 0)');
  });

  test('inheritance: font-family crosses the boundary into shadow content', async ({ page }) => {
    const result = await page.evaluate(() => {
      const shadow = (window as any).__shadow;
      const inner = shadow.getElementById('inner') as HTMLElement;
      return { fontFamily: getComputedStyle(inner).fontFamily };
    });
    expect(result.fontFamily).toContain('DejaVu');
  });

  test('R8: form controls do not inherit font-family (needs font:inherit)', async ({ page }) => {
    const result = await page.evaluate(() => {
      const shadow = (window as any).__shadow;
      const btn = shadow.getElementById('btn') as HTMLElement;
      return { fontFamily: getComputedStyle(btn).fontFamily };
    });
    // button has explicit font-family: Arial in shadow CSS
    expect(result.fontFamily.toLowerCase()).toContain('arial');
  });

  test('R9: adoptedStyleSheets applies styles inside shadow root', async ({ page }) => {
    const result = await page.evaluate(() => {
      const shadow = (window as any).__shadow;
      const probe = shadow.getElementById('adopted-probe') as HTMLElement;
      return { color: getComputedStyle(probe).color, sheetCount: shadow.adoptedStyleSheets.length };
    });
    expect(result.sheetCount).toBe(1);
    expect(result.color).toBe('rgb(7, 7, 7)');
  });

  test('R9b: adopted stylesheet does not leak into light DOM', async ({ page }) => {
    const result = await page.evaluate(() => {
      const light = document.getElementById('light-ancestor') as HTMLElement;
      return { color: getComputedStyle(light).color };
    });
    // light ancestor keeps its own color (rgb(1,2,3) inherited from body)
    expect(result.color).toBe('rgb(1, 2, 3)');
  });

  test('keyboard: trusted keydown inside shadow observed at document', async ({ page }) => {
    await page.evaluate(() => {
      const inner = (window as any).__inner;
      inner.focus();
      (window as any).__docKeydownSeen = false;
      document.addEventListener('keydown', () => { (window as any).__docKeydownSeen = true; }, { once: true });
    });
    await page.keyboard.press('a');
    await page.waitForTimeout(50);
    const seen = await page.evaluate(() => (window as any).__docKeydownSeen === true);
    expect(seen).toBe(true);
  });

  test('dialog: showModal moves focus out of shadow; close restores into shadow', async ({ page }) => {
    const result = await page.evaluate(async () => {
      const host = (window as any).__host;
      const shadow = (window as any).__shadow;
      const focusable = shadow.getElementById('btn') as HTMLElement;
      focusable.focus();
      const before = shadow.activeElement === focusable;
      const dlg = document.getElementById('dlg') as HTMLDialogElement;
      dlg.showModal();
      const during = document.activeElement === document.getElementById('dlg-a');
      dlg.close();
      await new Promise((r) => setTimeout(r, 50));
      const afterDoc = document.activeElement === host;
      const afterShadow = shadow.activeElement === focusable;
      return { before, during, afterDoc, afterShadow };
    });
    expect(result.before).toBe(true);
    expect(result.during).toBe(true);
    expect(result.afterDoc).toBe(true);
    expect(result.afterShadow).toBe(true);
  });

  test('drag: light -> shadow native (isTrusted) drop fires with payload', async ({ page }) => {
    await page.dragAndDrop('#light-src', '#shadow-target');
    const dropped = await page.locator('#shadow-target').getAttribute('data-dropped');
    expect(dropped).toBe('payload-light');
    const trusted = await page.locator('#shadow-target').getAttribute('data-drop-trusted');
    expect(trusted).toBe('true');
  });

  test('drag: shadow -> light native (isTrusted) drop fires with payload', async ({ page }) => {
    await page.dragAndDrop('#shadow-src', '#light-target');
    const dropped = await page.locator('#light-target').getAttribute('data-dropped');
    expect(dropped).toBe('payload-shadow');
    const trusted = await page.locator('#light-target').getAttribute('data-drop-trusted');
    expect(trusted).toBe('true');
  });

  test('drag: custom MIME type survives the shadow boundary', async ({ page }) => {
    // attach a drop handler that reads the custom mime
    await page.evaluate(() => {
      const t = document.getElementById('light-target') as HTMLElement;
      t.addEventListener('drop', (e) => {
        const m = (e as DragEvent).dataTransfer?.getData('application/x-monster');
        t.setAttribute('data-monster', m || '');
      }, { once: true });
    });
    await page.dragAndDrop('#shadow-src', '#light-target');
    const monster = await page.locator('#light-target').getAttribute('data-monster');
    expect(monster).toContain('Gelatinous Cube');
  });
});
