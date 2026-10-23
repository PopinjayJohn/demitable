# ADR-0046: The app provides the shadow host, and the core renders plugin config

- **Status:** Accepted
- **Date:** 2026-10-23
- **Extends:** ADR-0027 (light DOM) and ADR-0029 (shadow DOM)
- **Addresses:** two open plugin-API questions — the shadow host contract,
  and who renders a plugin's configuration UI

## Context

ADR-0029 moved plugin UI into a shadow root. It did not say **who creates
the root**, and the answer decides:

- whether the app can style and label its own surfaces;
- whether `::part()` works, and in which direction;
- whether a plugin may use `mode: 'closed'`;
- what the manifest's `entry:` field means.

Separately, the manifest has a `config` block, and nothing said whether the
core or the plugin renders it.

## Decision

### The app provides the shadow root

**The host element is created by the app, and it carries a shadow root the
plugin mounts into.** The plugin does not attach its own root.

Consequences:

- **The app keeps control of the root**, so it can style the surface, label
  it for assistive technology, and enforce the focus trap (ADR-0034).
- **`::part()` works from the app side**, which is the sanctioned channel
  for exactly that. The verification confirmed it works in every engine.
- **A plugin cannot use `mode: 'closed'`**, because the app created the
  root. That is the point: the app is the one that would need to reach in.

### What `entry:` means

`entry:` names the module the **iframe** loads, and it is only meaningful for
iframe surfaces.

A surface with `dom.inline` instead declares the **custom element name it
registers**. The manifest has one `provides:` entry per surface, and each
entry says which kind it is, so the two are never confused.

### Config is rendered by the core

**The core renders the declarative `config` block** — text, number, boolean,
select, path — into a settings form. Every plugin gets one for free,
including a plugin with no UI of its own.

A plugin that wants richer configuration **registers a config surface**, and
that surface takes precedence over the generated form. The core's generated
form is the default, not a limit.

## Consequences

**Good:**

- **The app can style its own surfaces**, which is what a design system
  needs, and it is the one thing a shadow root cannot give the plugin away
  from.
- **The manifest has one obvious shape per surface kind.**
- **Every plugin has a settings UI** without writing one.

**Bad:**

- **A plugin cannot be fully sealed off.** It shares the root, so it can
  reach the app's styles. That is acceptable: the plugin is trusted at the
  user's privilege level (ADR-0019), and a sealed-off plugin was never the
  promise.
- **The core's config renderer is a small UI surface the core now owns**,
  which is a slight return of the thing ADR-0019 moved out of the core. It is
  justified by the alternative: no plugin would have a settings UI at all.

**Neutral:**

- A plugin that wants an isolated surface of its own — a third-party
  webview, say — can still ship one inside its iframe. The app is not in the
  way.
