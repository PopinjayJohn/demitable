# Demitable — design system (UI spike)

This document writes down the token layer, the type scale, the spacing, and the
colour rules behind the four surfaces (GM dashboard, note editor, character
panel, player's view). It lives next to the code because the code *is* the
design system: every component reads from `src/lib/tokens.css`, and no
component contains a colour literal.

## Source

`src/lib/tokens.css` — one file, unregistered CSS custom properties
(ADR-0029 rule 4: a registered property with `inherits: false` does not cross
the shadow root, so nothing here is `@property`-registered).

A dark theme is one override block on `:root[data-theme="dark"]`; no component
changes between themes. The same token layer is used by plugin surfaces
through custom-property inheritance through shadow roots (ADR-0029).

## Colour

Semantic names, never literal names:

```
--bg-canvas   #f4f2ed   paper, not screen
--bg-surface  #fffefb   cards, panels
--fg-default  #211f1b   prose
--fg-muted    #57524a   secondary text
--accent      #2f5d9e   primary action
```

Accents come in four steps (default / hover / subtle fill / border) so that
*state* is never carried by hue alone: an active button is a fill **plus** an
inset rule **plus** its label weight; an invalid field is a red border **plus**
a warning glyph **plus** a message.

### Visibility vocabulary (ADR-0033 + ADR-0048)

Each visibility value carries **four channels**, so a GM who prints in greyscale
or a player who is colour-blind can still read it:

| Value   | Glyph   | Word      | Border pattern      | Hue        |
|---------|---------|-----------|---------------------|------------|
| gm      | 🔒 lock | GM only   | solid               | violet     |
| party   | 👥 users| Party     | dashed repeat       | green      |
| public  | 🌍 globe| Public    | double-dash repeat  | teal       |
| owner   | 🙍 user | Owner     | dashed long gap     | amber      |
| private | 👁️‍🗨️ eye-off | Private | dash-gap-dash gap   | red        |

The badges also carry a subtle background tint, but that is the *fifth*
channel — colour may fail; glyph + word + pattern must not.

### Secrets

A secret block uses `--secret` violet with a **dashed rule and a hatch
fill** (a repeating diagonal line), so it reads as "concealed" rather than
"warning" under greyscale — secrets are not errors (ADR-0035).

## Type scale

A 1.25 modular scale anchored at 16px, in `rem` so the browser's font-size
setting moves the whole app:

| Token   | Size  |
|---------|-------|
| text-2xs| 10px  |
| text-xs | 12px  |
| text-sm | 14px  |
| text-base| 16px |
| text-md | 20px  |
| text-lg | 24px  |
| text-xl | 30px  |
| text-2xl| 36px  |
| text-3xl| 48px  |

Line heights: tight 1.2, snug 1.35, normal 1.55, relaxed 1.75. A dyslexia font
swap (`--font-reading`) is one token — a setting, not a redesign (ADR-0034).

## Spacing

A 4px grid, `space-0` … `space-16`, plus `--tap-target` (2.25rem minimum
interactive size) and `--measure` (38rem prose measure).

## Shape, motion, z-index

`--radius-sm/md/lg`, `--shadow-1/2/3`, `--sidebar-width`, `--topbar-height`,
`--duration-fast/base/slow`, `prefers-reduced-motion` zeroes all durations.

## ADR-0034 colour-independence — checked in the screenshots

The colour proofs were captured on the GM dashboard and verified by looking at
them:

- **`walkthrough/10-proof-greyscale.png`** — every state still separates.
  Visibility badges differ by glyph, word, and border pattern (solid vs
  dashed vs double-dash), not just tint. Secrets keep their dashed rule plus
  hatch. Selection keeps its inset rule.
- **`walkthrough/10-proof-flat-hue.png`** — every semantic colour collapses to
  one grey (`#8a8378`). The only remaining differences are lightness
  (canvas vs surface vs surface-3), shape (badge pill vs panel), glyph (lock
  vs users vs globe), the word ("GM only" vs "Party"), and border pattern.
  Anything that still reads under flat-hue is not relying on hue.
- **`walkthrough/10-proof-deuteranopia.png`** — green/red states remain
  distinguishable through the pattern channels.

That is ADR-0034's rule *in practice*: no state encoded in colour alone, every
state has a non-colour cue, and it survives greyscale and colour-blindness.

## RTL preparation

`dir="auto"` on prose and logical properties throughout
(`margin-inline-start`, `padding-inline-end`) — prepared, not implemented
(ADR-0034).