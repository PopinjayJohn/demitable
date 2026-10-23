# ADR-0045: The plugin ABI, the capability mapping, and the `api: 1` freeze

- **Status:** Accepted
- **Date:** 2026-10-23
- **Extends:** ADR-0020 (the tiers) and ADR-0031 (exposure)
- **Addresses:** a blocking open decision — the ABI shape, the
  capability-to-host-function mapping, and the freeze mechanism

## Context

ADR-0020 settled the ABI *shape*: one exported entry point taking and
returning JSON, plus host functions for capabilities, in the style of Extism.

Three things were left open, and the research flagged the last as the one
that matters most:

- The **mapping** from manifest capabilities to host functions was undefined,
  with 14 capabilities and 6 host functions under different names.
- **How `api: 1` is frozen** was undefined. ADR-0019 says freeze only after
  two plugins; but `api: 1` already appeared in every example manifest, no
  milestone froze it, and "the core supports the current major and the
  previous one" is untestable when there is no previous major.
- **The deprecation mechanism** — shims, what breaks, who guarantees them —
  was entirely unspecified, and the whole plugin ecosystem depends on it.

## Decision

### The capability and host-function vocabularies are the same

Each capability has a host function with the same name, prefixed `host_`:

| Capability | Host function |
|---|---|
| `vault.read` | `host_vault_read` |
| `vault.write` | `host_vault_write` |
| `vault.query` | `host_vault_query` |
| `vault.assets` | `host_vault_assets` |
| `http.client` | `host_http_fetch` |
| `log` | `host_log` |

One vocabulary instead of two removes a translation layer and a class of
mismatch. A capability with no host function does not exist; the manifest
validator rejects it, so the two lists cannot drift.

### The freeze

**`api: 1` is frozen when the second plugin lands** — M11, the campaign
export plugin, which is non-UI and therefore an independent consumer.

Within `api: 1`, changes are **additive only**:

- new capabilities, and their host functions;
- new optional manifest fields;
- new event types in the bridge and the WebSocket protocol.

Never a changed signature. Never a repurposed field. Never a changed meaning
of an existing one. A host function that changes shape gets a new name —
`host_vault_read_v2` — and the old one stays.

A **breaking change means `api: 2`**, and the core then serves `api: 1` and
`api: 2` side by side, dispatching on the guest's declared version.

**The deprecation window is at least one major release of the core.** A
plugin built for `api: 1` keeps working for that window.

### Two mechanical guarantees

**The manifest declares `abi: 1`**, and the host checks it *before* loading
the module — so a mismatched plugin is rejected with a legible error rather
than a link error.

**The golden ABI test**: a wasm module that imports every host function the
host offers, built at compile time. If a signature changes, the module fails
to instantiate and CI catches it. Cheap, and it makes the freeze real rather
than aspirational.

## Consequences

**Good:**

- **One vocabulary** for capabilities, host functions and manifest fields.
- **The freeze has a date and a mechanism**, and a compile-time test enforces
  it.
- **A mismatched plugin fails legibly**, before it fails cryptically.

**Bad:**

- **Additive-only is a real constraint on the design.** Getting it wrong in
  version 1 is permanent, so version 1 must be right about what a plugin
  needs.
- **Serving two majors is work** that nobody will exercise until it is
  needed, and untested compatibility shims are exactly the code that rots.

**Neutral:**

- The shape is Extism's, so an Extism-PDK plugin's *shape* is familiar, even
  though our host functions and manifest differ from Extism's.
