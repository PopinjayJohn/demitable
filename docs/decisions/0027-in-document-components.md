# ADR-0027: Light-DOM plugin surfaces are opt-in, under an explicit capability

- **Status:** Accepted, **substantially amended by ADR-0029** — the default
  plugin surface is a shadow root, and the privilege statement below was
  backwards
- **Date:** 2026-10-22
- **Amends:** ADR-0020

## Context

This ADR originally argued for allowing a plugin to contribute **in-document
web components** — custom elements mounted into the app's own DOM, outside
the iframe that was then the default plugin surface.

The argument was that an iframe is a hard boundary in every direction, and
that breaking keyboard shortcuts, IME, clipboard and drag-and-drop across it
is a real cost for the surfaces that need to integrate with the app.

That argument was right. What was wrong was the **privilege model** it
implied.

The original reasoning treated the iframe as a sandbox and light-DOM
components as the dangerous exception. But a same-origin iframe was never a
sandbox: its script could always reach `document`, `localStorage`, and the
API as the user. It isolated *styles and layout*, not *privilege*. The
exception was not an exception — light-DOM components had the same reach all
along.

ADR-0029 fixes the isolation primitive (a shadow root, not a frame) and
corrects the model. This ADR now records only what is left: **mounting into
the app's own DOM is opt-in, and it is a style-and-layout decision, not a
privilege one.**

## Decision

**A plugin's surface is a shadow root by default. Mounting into the app's
document — light DOM — requires the explicit `dom.inline` capability.**

### The capability

| | |
|---|---|
| `dom.inline` | The plugin may mount into the app's own DOM rather than into a shadow root |

- **Compiled-in plugins have it implicitly.** They are part of the program.
- **A WASM plugin must declare it.** It is not implied by the plugin's
  existence, and it is not implied by the plugin shipping a UI.

### What it actually controls

`dom.inline` controls **style and layout participation**, not privilege:

| | Shadow surface (default) | Light-DOM surface (`dom.inline`) |
|---|---|---|
| Styles | Contained by the shadow root | Participate in the app's own styles |
| Layout | Its own box, inside a host | Flows with the app's layout, grid, fullscreen |
| Placement | A named host element the app provides | Anywhere the app allows |
| **Privilege** | **The app's** | **The app's** |

The last row is the correction. **Both kinds have the app's privileges**,
because both run in the app's document. A shadow surface is not safer; it is
tidier.

### The privilege statement, stated once and correctly

> **All plugin UI runs in the app's document, and therefore has the app's
> privileges. A plugin's *logic* is sandboxed; a plugin's *UI* is trusted.**

This is what the import warning should say, plainly:

```
This archive contains 2 plugin bundles:
  demitable.example-plugin   v1.0.0   capabilities: vault.read, vault.write
  demitable.another           v0.4.1   capabilities: vault.read, dom.inline

One of these contributes components directly to the app's interface.
A component with dom.inline can restyle and rearrange the app around it.

Plugins are executable code. A plugin's logic is sandboxed — no network, no
filesystem — but its interface code runs in the app's own page and can read
and change what you see. That includes every note in the campaign, including
ones marked GM-only.

Install them? [y/N]
```

The prompt is required interactively. A non-interactive import skips plugins
unless told otherwise.

### Scope of a mounted surface

A light-DOM component is mounted into a **named host element** the app
provides. The app decides where hosts exist — a note's sidebar, a view's
footer, a table panel — and the plugin declares which hosts it wants.

A shadow surface is mounted the same way, into a host that carries a shadow
root. The default placement is therefore scoped and deliberate either way, so
an accidental component does not end up covering the app.

### The editor's extension registry is a different path

The editor plugin (ADR-0021) exposes a registry for *inline content inside a
note* — a third-party plugin registering a spell-slot block or a stat line in
the body. That is the sanctioned way to put plugin content in a note.

`dom.inline` is for components in the **app's chrome**, not for note content.
The two do not overlap, and the editor's registry is not an escalation
hatch.

## Consequences

**Good:**

- The default surface is tidy without being hostile: styles contained, and
  every native interaction working.
- `dom.inline` is a visible, per-plugin, human-granted decision, and it now
  means something precise — *this plugin participates in the app's own
  interface*, not *this plugin may read your notes* (which is already true
  of every plugin surface).
- The import warning is accurate rather than reassuring.
- The editor and the table can compose with the app's chrome, which is what
  they need to be.

**Bad:**

- **The privilege model is weaker than the docs used to imply**, and that is
  the honest position rather than a regression. A plugin's interface code can
  read and change what the user sees.
- **`dom.inline` is now a less alarming-sounding capability than it was**,
  which is a communication risk: "it can restyle the app" does not land with
  the force of "it can read every note". The warning text has to carry that,
  because the capability name no longer does.
- A light-DOM plugin can genuinely break the app's layout, and there is no
  recovery other than disabling it.

**Neutral:**

- Compiled-in plugins — the editor, the table — have it implicitly, so
  first-party integration is unimpeded.
- The capability model is unchanged otherwise.
- ADR-0029 records the isolation primitive and the corrected model; this ADR
  records the opt-in.
