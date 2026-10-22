# ADR-0011: Packs are local files only — no registry, no signing

- **Status:** Accepted, amended by ADR-0025 (packs and plugins may now arrive
  in an archive imported from a URL, so the sandbox becomes load-bearing
  rather than merely prudent)
- **Date:** 2026-10-21

## Context

The pack format (ADR-0003) is designed to be shared: YAML files, text, diffable
in git. The question is how packs reach a user's machine.

Three options: local files only; a registry with signed packs; a registry with
unsigned packs.

A pack is not inert data. A pack contains formulas, and formulas are code
evaluated by the formula engine (ADR-0003). The engine is sandboxed, but
"sandboxed" is a claim about an implementation, not a guarantee, and a pack
registry turns "run code from a stranger" into a routine user action.

## Decision

**Packs are local files. Nothing is fetched at runtime.**

- Built-in packs ship inside the binary (D&D 5e SRD 5.2 to begin with).
- Additional packs are read from `Packs/*.yaml` in the vault, and from a
  user-level directory (`~/.local/share/demitable/packs/`).
- A vault pack shadows a built-in pack of the same id.
- There is no registry, no update mechanism, and no network fetch for pack
  content.
- Sharing a pack is sharing a file. The mechanism is the same as sharing a
  character sheet: send it to the other person.

> **Amended by ADR-0025.** Packs can now arrive inside an archive imported
> from a URL, so *locality* is no longer the property that carries the trust
> argument — the download is what makes it local, and the author is still a
> stranger. The conclusion of this ADR (no registry, no signing) stands; the
> reasoning underneath it does not, and the sandbox must therefore be
> **paranoid rather than reasonable**:
>
> - a hard timeout and step limit on formula evaluation, not just depth;
> - no user-defined functions, no assignment, no prototype access;
> - the same limits for imported packs as for locally installed ones, because
>   there is no longer a meaningful difference between them.

**A pack is trusted by virtue of being local.** The user's responsibility is
the same as running a script someone sent them, which is a bar users already
understand.

The formula engine still sandboxes (ADR-0003), for two reasons: the engine is
also reachable from character data that a *user* wrote and might not fully
control, and a sandbox that is only exercised by trusted input is a sandbox
that rots. **Imported packs are untrusted input, so the second reason becomes
the primary one.**

## Consequences

**Good:**

- No supply-chain surface at all. No keys, no revocation, no registry to
  host, moderate, or pay for.
- Packs work offline and inside a restricted network, like everything else in
  the app.
- A community can share packs through exactly the channel they already use
  for everything else — a file, a gist, a forum post, or an archive at a URL
  (ADR-0025).

**Bad:**

- **Discovery.** Nobody will find packs unless someone points them at them.
  This is the price of no registry, and it is a real price: the ecosystem
  effect that makes a format succeed may not happen.
- No version compatibility signalling between a pack and the app. The pack
  declares a minimum app version, and the app refuses to load a pack that
  needs a newer one.
- Distribution is manual, so the "share my custom pack" flow is a manual
  copy for now.
- **The sandbox is now load-bearing** (amended by ADR-0025). It was
  previously "reasonable rather than paranoid"; it now has to be paranoid,
  and the limits it imposes are a real constraint on what pack authors can
  write.

**Neutral:**

- If a registry ever becomes necessary, the pack format is already a
  self-contained file with an id, version and declared minimum app version.
  That is most of what a registry index would need.
- A future in-app "import pack" button is a file picker, not a network
  feature.
