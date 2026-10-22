# ADR-0034: Accessibility and localisation are design constraints, not a later pass

- **Status:** Accepted
- **Date:** 2026-10-22
- **Addresses:** [#12](https://github.com/PopinjayJohn/demitable/issues/12)
  (accessibility, localisation and RTL — zero mentions in the repository
  before this)

## Context

The research found that the repository contained **no** mention of
accessibility, screen readers, colour-blindness, dyslexia, i18n, l10n,
localisation or RTL. The only "accessibility" mention was an aside about a
rich-text editor being better than a code editor for mobile input.

Some of that is fine — a project is not obliged to ship a full translation
programme. But several of these are *structural*, and structural things are
cheap now and expensive later:

- **Colour-independence** is a rule about the *visual vocabulary*. If fog of
  war, token disposition, condition icons and initiative highlighting encode
  state in colour alone, that is baked into the design and every view.
- **Semantic HTML and ARIA** are a property of how components are built, not a
  pass over the finished UI.
- **Translatable pack labels** are a property of the pack *format*. The pack
  already has `label:` on every field, resource and condition. Whether those
  labels are translatable is a decision about the format, and it costs one
  extension now versus a re-authoring of every pack later.
- **RTL** is a `dir` attribute and logical CSS properties — cheap to prepare
  for, tedious to retrofit.

And the community evidence is unambiguous about the incumbents: Roll20 has been
shipping incremental screen-reader work for years, and colour-blindness is a
live, unresolved failure in its current character sheet.

There is also a specific answer available to us that nobody else has,
available because of a decision we already made.

## Decision

### The pack is data, so the pack's labels are data

The pack format already declares a `label:` on every attribute, resource,
condition, workflow and sheet field. **Those labels become translatable.**

A pack carries its own translations:

```yaml
id: dnd5e
name: D&D 5th Edition
labels:
  en: { hp: "Hit Points", ac: "Armour Class" }
  fr: { hp: "Points de Vie",  ac: "Classe d'Armure" }
```

The app resolves a label against the user's locale, falling back to the
pack's base language. **A French 5e pack ships French labels**, and a
community can maintain one without our involvement — which is exactly how
Foundry solved it, with community translation infrastructure rather than a
first-party programme.

This is the *mechanical* terms being translatable, which is a separate
problem from translating the UI. D&D "uses language and terms to refer to
specific mechanics that everyone can recognize" — the terms are the problem.

The app's own UI strings are a separate matter: string extraction and a
locale file, and a community translation effort later, not a first-party
programme now.

### Colour-independence is a rule, not a goal

**No state may be encoded in colour alone.** Everywhere:

- **Fog of war** — revealed and unrevealed differ by more than brightness.
- **Token disposition** — friendly, neutral and hostile each carry a shape or
  an icon, not only a hue.
- **Conditions** — every condition has an icon, not only a colour.
- **HP bars** — the number is always present and readable; the bar is
  supplementary.
- **Initiative** — the current turn is marked by a label and a position, not
  only a highlight.
- **Selection** — a shape or outline, not only a tint.

The rule is a **phase-3 requirement** and it is the one most likely to be got
wrong by a team that has not thought about it, because "green for friendly"
feels finished.

### Semantic HTML and keyboard operability

Every interactive surface is a real control with a real role, a real name and
a real keyboard path. A character sheet is a **form**, and a form should be a
form.

The plugin surface model (ADR-0027, ADR-0029) makes this achievable: a
plugin surface is a shadow root, and the app can style and label the host
element it provides.

### Type and size are user settings

A sans-serif default, adjustable line height and text size, and a
dyslexia-friendly font option. One set of CSS custom properties, so it is a
setting rather than a redesign.

### RTL is prepared for, not implemented

`dir="auto"` on prose, logical CSS properties throughout (`margin-inline-start`
rather than `margin-left`), and mirrored board controls.

No TTRPG tool ships RTL today, so this is a hypothesis rather than a
requirement — but it is a *cheap* one to prepare for and an expensive one to
retrofit, and the whole project is a text-in-a-box.

## Consequences

**Good:**

- The pack format gains a translatable-label mechanism that makes the
  project genuinely useful to non-English tables, at the cost of one extension.
- Colour-independence makes the board better for *everyone*, not only for
  colour-blind players.
- Semantic HTML and keyboard operability are properties of the plugin API, so
  plugins get them for free.
- None of this is a large amount of work. It is a set of rules applied while
  building, rather than a retrofit.

**Bad:**

- **The rule has to be enforced.** "Never encode state in colour alone" is
  easy to state and easy to violate. It needs a review checklist, and the
  board's visual vocabulary needs to be *designed* rather than improvised.
- **The pack's label translations are a community burden.** A French pack is
  only as good as whoever maintains it. We provide the mechanism; we do not
  provide the translations.
- **RTL preparation touches every stylesheet**, which is a lot of small
  changes for a hypothesis.

**Neutral:**

- No TTRPG tool ships RTL, so this may never matter. Preparing for it is
  cheap; not preparing for it is also defensible. It is recorded so that the
  decision is visible rather than accidental.
- Screen-reader support in a rich client is a long tail. The *structure* —
  semantic roles, names, keyboard paths — is what we commit to; the polish
  is a separate effort.
