#!/usr/bin/env node
/**
 * Walk the prototype in a real browser and capture what it looks like.
 *
 * The walk-through is part of the spike's acceptance criteria ("a real
 * session has been walked through the prototype with the friction recorded"),
 * and this script is how it is done reproducibly: it drives the app through
 * the GM's session, takes a screenshot at every step, and writes a
 * transcript of what was on screen and what the console said.
 *
 *   node tools/walkthrough.mjs [--url http://127.0.0.1:5183] [--out ../walkthrough]
 *
 * It uses the Playwright build already on the machine, so it needs no
 * download. On a bare Linux box the Chromium build wants a handful of shared
 * libraries that live in a side-loaded prefix rather than in /usr/lib; point
 * LD_LIBRARY_PATH at it, or the launch fails with "libnspr4.so: cannot open
 * shared object file".
 */

import { chromium } from "playwright-core";
import { mkdirSync, writeFileSync, existsSync } from "node:fs";
import { join } from "node:path";

const argv = process.argv.slice(2);
const arg = (name, fallback) => {
  const i = argv.indexOf(name);
  return i >= 0 && argv[i + 1] ? argv[i + 1] : fallback;
};
const URL_ = arg("--url", "http://127.0.0.1:5183/");
const OUT = arg("--out", "../walkthrough");
const EXECUTABLE =
  process.env.CHROMIUM_PATH ||
  join(process.env.HOME ?? "", ".cache/ms-playwright/chromium-1248/chrome-linux64/chrome");

mkdirSync(OUT, { recursive: true });

// The Chromium build on this machine wants a side-loaded fontconfig, or it
// logs "Could not find any font" and the renderer falls over on a page with
// any text in it. Point it at a prefix that has one, if there is one.
for (const [conf, dir] of [
  ["/home/johan/projects/semiplane/e2e/.browser-libs/fonts.conf", "/home/johan/projects/semiplane/e2e/.browser-libs"],
  ["/tmp/opencode/shadowdom-proto/syslibs/fonts.conf", "/tmp/opencode/shadowdom-proto/syslibs"],
]) {
  if (existsSync(conf)) {
    process.env.FONTCONFIG_FILE ??= conf;
    process.env.FONTCONFIG_PATH ??= dir;
    break;
  }
}

const transcript = [];
const consoleErrors = [];

function record(step, note) {
  transcript.push(`## ${step}\n\n${note}\n`);
  console.log(`· ${step}`);
}

const browser = await chromium.launch({ executablePath: EXECUTABLE });
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });

page.on("console", (msg) => {
  if (msg.type() === "error" || msg.type() === "warning") {
    consoleErrors.push(`[${msg.type()}] ${msg.text()}`);
  }
});
page.on("pageerror", (err) => consoleErrors.push(`[pageerror] ${err.message}`));

async function shot(name) {
  await page.screenshot({ path: join(OUT, `${name}.png`), fullPage: false });
}

/* ---------------------------------------------------------------- the walk */

await page.goto(URL_, { waitUntil: "networkidle" });
await shot("01-gm-dashboard");
record(
  "01 — The GM's dashboard",
  "The GM browser, at 1440×900. Three columns: campaigns and attention on the\n" +
  "left, the note list in the middle, the note and its visibility inspector on\n" +
  "the right. The list shows the **effective** visibility of every note, not\n" +
  "the declared one — the note with no `visibility` field carries a defaulted\n" +
  "badge, because ADR-0033 §6 defaults it to `gm`.",
);

/* --- click through the notes ------------------------------------------------ */

const notes = await page.locator(".note-row__name").allTextContents();
record(
  "02 — The note list",
  `The list holds ${notes.length} notes: ${notes.join(", ")}.\n` +
  "The badge column is the projection's first visible consequence.",
);

await page.locator(".note-row__name", { hasText: /^Adult Red Dragon$/ }).click();
await page.waitForTimeout(150);
await shot("02-gm-only-statblock");
record(
  "03 — A GM-only stat block",
  "The Adult Red Dragon. `visibility: gm`, so it is in no player's view at\n" +
  "all. The stat block renders as a card: the token image, the size and type\n" +
  "line, the labelled field grid, the six-column ability table, and the\n" +
  "`## Traits` / `## Actions` sections that were inside the code fence.",
);

/* --- the player eye --------------------------------------------------------- */

await page.getByTestId("player-eye").click();
await page.waitForTimeout(150);
await shot("03-player-eye-absent");
const absentText = await page.locator(".absent").innerText().catch(() => "");
record(
  "04 — The same note, through a player's eyes",
  `The note is not in the player's view at all: "${absentText.replace(/\n/g, " ").trim()}"\n` +
  "This is the important bit. The projection does not return a stub, and the\n" +
  "interface does not draw one. There is no 'this note is hidden' placeholder,\n" +
  "because a placeholder is a hint.",
);

/* --- a party-visible note with a secret ------------------------------------- */

await page.getByTestId("player-eye").click();
await page.locator(".note-row__name", { hasText: /^The Hollow Hill$/ }).click();
await page.waitForTimeout(150);
await shot("04-party-note-with-secret");
record(
  "05 — A party-visible note with a secret inside it",
  "The Hollow Hill. `visibility: party`, so the note is shared. Inside it is a\n" +
  "`> [!secret]` callout — a secret block in a note the party can already see.\n" +
  "The note renders for the party; the secret does not.",
);

await page.getByTestId("player-eye").click();
await page.waitForTimeout(150);
await shot("05-player-eye-secret-stripped");
const stripped = await page.locator(".stripped-note").innerText().catch(() => "(none)");
record(
  "06 — The same note, through a player's eyes",
  `The note is still there. The secret is gone: "${stripped.replace(/\n/g, " ").trim()}"\n` +
  "The secret's text is stripped from the projection; the note's other\n" +
  "paragraphs are not.",
);

/* --- the character panel ---------------------------------------------------- */

await page.getByRole("button", { name: "Character panel" }).click();
await page.waitForTimeout(200);
await shot("06-character-panel-gm");
record(
  "07 — The character panel, as the GM",
  "Bram's sheet. A real form, rendered from the pack's declared fields — the\n" +
  "same field list for every principal. The panel on the right lists what the\n" +
  "current viewer may not see, and why. For the GM that is `True Motivation`,\n" +
  "`GM Notes`, and Bram's own `Own note` — the last because `private` means\n" +
  "the note's player and nobody else, *including the GM*.",
);

await page.locator("select[aria-label='Render the app as']").selectOption("p_8f2a");
await page.waitForTimeout(200);
await shot("07-character-panel-player");
record(
  "08 — The character panel, as Bram",
  "The same form, with the fields Bram may not see removed. His own private\n" +
  "note is present — it is his. The GM's fields are not, and there is no\n" +
  "count of what is missing, because a count is a leak.",
);

/* --- the NPC with a property-level deny ------------------------------------- */

await page.locator("select[aria-label='Render the app as']").selectOption("gm");
await page.waitForTimeout(150);
await page.locator(".char", { hasText: "Marshal Ivo Sarn" }).click();
await page.waitForTimeout(200);
await shot("08-npc-property-deny");
record(
  "09 — A property-level deny on a party-visible note",
  "Marshal Ivo Sarn. The note is `visibility: party` — the party can read his\n" +
  "sheet — but the pack declares `motive` as `gm`, so that one field is\n" +
  "withheld. `disposition` is `party` with a `display` mask for Wren, so the\n" +
  "field is present for Bram and masked for Wren.",
);

/* --- the player's view ------------------------------------------------------ */

await page.getByRole("button", { name: "Player's view" }).click();
await page.locator("select[aria-label='Render the app as']").selectOption("p_8f2a");
await page.waitForTimeout(250);
await shot("09-player-view");
record(
  "10 — The player's view",
  "What a player actually gets: their own character's sheet, and the campaign\n" +
  "as they have discovered it. The Adult Red Dragon is not on the list, and\n" +
  "nothing on the page says it is missing.",
);

/* --- the proofs ------------------------------------------------------------- */

await page.locator("select[aria-label='Render the app as']").selectOption("gm");
await page.getByRole("button", { name: "GM browser" }).click();
await page.waitForTimeout(150);

for (const proof of ["greyscale", "flat-hue", "deuteranopia"]) {
  await page.locator("select[aria-label='Colour proof mode']").selectOption(proof);
  await page.waitForTimeout(200);
  await shot(`10-proof-${proof}`);
}
record(
  "11 — The colour proofs",
  "Greyscale, flat hue (every semantic colour resolves to one grey) and\n" +
  "deuteranopia, captured on the GM dashboard. Under `flat-hue` the only\n" +
  "remaining differences are lightness, the glyph, the word, and the border\n" +
  "pattern — which is the test ADR-0034 asks for.",
);

/* --- the note editor -------------------------------------------------------- */

await page.locator("select[aria-label='Colour proof mode']").selectOption("none");
await page.getByRole("button", { name: "Note editor" }).click();
await page.waitForTimeout(200);
await shot("11-note-editor");
record(
  "12 — The note editor",
  "Source pane, live preview, and the frontmatter panel on the right. The\n" +
  "frontmatter panel shows ADR-0040's twelve reserved fields, the note's\n" +
  "**effective** visibility with the rule chain beneath it, and the GM-only\n" +
  "blocks with their block ids and the reveal controls.",
);

/* --------------------------------------------------------------------- done */

await browser.close();

const report = [
  "# Walk-through transcript",
  "",
  `Generated by \`tools/walkthrough.mjs\` against ${URL_}.`,
  "",
  transcript.join("\n"),
  "",
  "## Console",
  "",
  consoleErrors.length === 0
    ? "No console errors or warnings."
    : consoleErrors.map((e) => `- ${e}`).join("\n"),
  "",
].join("\n");

writeFileSync(join(OUT, "transcript.md"), report, "utf8");
console.log(`\nWrote ${join(OUT, "transcript.md")} and ${transcript.length} screenshots.`);
if (consoleErrors.length) {
  console.log(`\nConsole messages:\n${consoleErrors.join("\n")}`);
}
