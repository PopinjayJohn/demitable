# ADR-0025: A vault can be populated by importing an archive from a URL

- **Status:** Accepted, amended by ADR-0027 (the import warning must
  distinguish a sandboxed plugin from one that contributes in-document
  components)
- **Date:** 2026-10-22
- **Supersedes:** an earlier draft of this ADR, which specified
  `demitable init --from <git-url>`. The git route was dropped in favour of
  an archive import. The trust boundary was loosened accordingly, and that
  change is deliberate.

## Context

ADR-0023 split the pack from the content: the pack holds mechanics, the vault
holds the actual entities. ADR-0024 made the SRD content a **development**
artefact — committed to the repository, never shipped.

That leaves a gap, and it is the gap ADR-0023 warned about: **a pack is no
longer self-sufficient.** Installing the 5e pack gives a user the mechanics
and no content. The wizard has no classes to offer. The tabletop has no
monsters.

Something must carry content from wherever it is authored to wherever it is
used. The requirements:

- It must not require a service we operate. ADR-0011 ruled out a registry on
  the grounds that it is a supply-chain surface of our own making; we should
  not reintroduce one in a different shape.
- It must carry **licence terms** with the content, because different content
  has different terms.
- It must work offline afterwards. The whole project is local-first.

An **archive at a URL** satisfies all three. The URL can be a release asset, a
website, an object store — the distribution point is the content author's
choice, and the mechanism is a plain file fetch.

## Decision

**`demitable import <url>` downloads an archive and merges it into a vault.**

### The command

```
demitable import <url>
  --vault <dir>         target vault (default: the current vault)
  --new <name>          create a new vault at ./<name> from the archive
  --force               overwrite conflicting files
  --skip                keep existing files and skip conflicts
  --include-plugins     install plugin bundles without prompting
  --no-plugins          refuse plugin bundles entirely
  --insecure            allow plain http (default: https only)
```

`--new` creates a vault rather than merging into one. Without a manifest the
payload root is imported at the vault root; with one, `target` namespaces it.

### The manifest is optional, and recommended

`demitable-import.json` at the archive root:

```json
{
  "id": "dnd5e-srd",
  "name": "D&D 5e SRD 5.2.1 Content",
  "version": "1.0.0",
  "kind": "content",
  "target": "Notes/Content",
  "requires": { "packs": ["dnd5e"], "app": ">=0.1.0" },
  "license": "CC-BY-4.0",
  "attribution": "Wizards of the Coast — D&D System Reference Document 5.2.1"
}
```

`requires` is checked before anything is written: a missing pack is a clear
error, not a half-imported vault.

**Licence terms travel with the content**, which is the whole reason the
archive carries a manifest rather than being a bare zip of files.

### The import is a core capability

It runs before plugins load and before the vault is fully indexed, so it is
core. The *UI* for it — an "Import" button — is a plugin, because it is a
surface.

### Extraction safety

Downloading and unpacking a zip is a classic attack surface, and this is where
the care goes.

1. **Extract to a temporary directory first.** Validate the entire tree.
   Only then move it into the vault. A failed or hostile archive never leaves
   a half-imported vault behind.
2. **Path traversal is rejected.** Every entry's resolved path must stay
   under the target. Absolute paths, `..` segments, and symlinks are refused.
3. **Size caps.** Maximum entry count, maximum total uncompressed size, and
   a maximum compression ratio — a zip bomb is rejected before it is written.
4. **Private paths are always rejected.** `.demitable/` and `.git/` entries
   are refused and **reported**, never imported. See below.
5. **Checksums are verified** when the manifest or the URL declares one, and
   the download is verified *before* extraction.

### `.demitable/` is never importable

`.demitable/` holds `players.yaml` — accounts, ownership claims and argon2id
passphrase hashes — and the journal.

An imported `.demitable/players.yaml` would silently grant an outsider's
accounts access to the vault. So those entries are **rejected and reported**,
not skipped silently:

```
Import complete.
  1,692 files imported
  3 entries rejected: .demitable/players.yaml, .demitable/journal/…, .git/config
```

The trade-off is that player accounts do not travel with an imported
campaign. That is correct: passphrases do not belong in an archive. Moving a
group to another machine is a deliberate export, not a side effect of
importing content.

### Plugins may be imported — with a warning

An archive may contain plugin bundles, and they **are** installed. But
because a plugin is executable code, the import says so first:

```
This archive contains 2 plugin bundles:
  demitable.example-plugin   v1.0.0   capabilities: vault.read, vault.write
  demitable.another           v0.4.1   capabilities: vault.read, dom.inline

One of these contributes components directly to the app's interface.
A component with dom.inline runs with your own permissions: in a GM's
session it can read every note in the campaign, including ones marked
GM-only, and act on your behalf.

Plugins are executable code. A plugin runs with the permissions of whoever
opens it — for the GM, that means the entire vault, including notes marked
GM-only. WASM plugins are sandboxed (no network, no filesystem), but they can
still read and write whatever their capabilities allow.

Install them? [y/N]
```

The `dom.inline` paragraph is not decoration. It is a **different kind of
trust** from the WASM sandbox reassurance, and the two must not be read as
one reassurance (ADR-0027).

- **Interactive**: the prompt is required. An explicit yes.
- **Non-interactive**: plugins are **not** installed unless `--include-plugins`
  is passed. A pipeline should not silently execute code.
- `--no-plugins` refuses them outright.

The warning is precise about what a plugin can reach: a WASM plugin is
sandboxed at the OS level — no network, no filesystem, WASI preview 1 has no
sockets — but it reads the vault through the core's capability API, so its
reach is exactly its declared capabilities, running as whoever opened it.

### Pack trust is now a sandbox question, not a locality question

ADR-0011 reasoned that a pack is trusted "by virtue of being local", and
therefore the sandbox could be "reasonable rather than paranoid".

**That reasoning no longer holds.** Packs now arrive from a URL inside an
archive. The sandbox becomes load-bearing, and it must be paranoid:

- step and depth limits on formula evaluation, with a hard timeout;
- no user-defined functions, no assignment, no prototype access;
- the same limits apply to imported packs as to locally installed ones,
  because there is no longer a meaningful difference between them.

This is the one place where dropping the git route made the system *less*
safe, and it is compensated rather than waved away.

### No registry, still

The archive URL is chosen by the content author. There is no index we
operate, no signing keys, no revocation, and no supply chain of our own.
Discovery is a README, a forum post, a link.

The difference from ADR-0011's model is that packs and plugins can now
arrive from a stranger. The difference from a registry is that **we never
list, index, or vouch for anything.**

## Consequences

**Good:**

- The content gap from ADR-0023 is closed, with no service to run.
- Content carries its own licence terms, in the manifest, next to the files.
- The SRD content has a distribution channel that respects its terms: an
  archive hosted wherever the author likes, imported by whoever wants it.
- Campaign templates work the same way — an archive, a URL, an import.
- The import is offline-friendly afterwards, like everything else.
- Optional checksums give authors a way to pin integrity.
- No `git` dependency, and no need for the user to understand a working
  tree to get content.

**Bad:**

- **A URL is a less stable distribution point than a git repository.** A URL
  can rot, and there is no version history on the consumer side. Archives
  must be versioned in their manifest, and authors should serve a stable URL.
- **Importing arbitrary archives is a security surface.** Zip slip, zip
  bombs, symlinks and path traversal all have to be handled correctly, and
  the code that handles them is security-critical. It is also the kind of
  code that gets written once and never tested against a real attack.
- **The sandbox becomes load-bearing** for imported packs and plugins.
  ADR-0011 relaxed it; this decision tightens it again.
- **The plugin warning is a prompt, and prompts get skipped.** Users will
  click through it, and `--include-plugins` makes that easy in scripts. The
  warning is worth having regardless, but it is not a security control.
- **No version history.** A git-based route showed what changed between
  versions; an import does not. A user who imports twice gets no diff.

**Neutral:**

- The app never commits. That is unchanged, and now more relevant, since
  there is no git involved.
- Import is a one-shot operation, not a surface. The UI for it is a plugin.
