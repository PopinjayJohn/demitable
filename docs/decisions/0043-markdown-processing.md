# ADR-0043: Markdown is read in three places, and the write path uses no parser at all

- **Status:** Accepted
- **Date:** 2026-10-23
- **Extends:** ADR-0021 (the editor) and ADR-0030 (the FTS5 index)
- **Addresses:** a blocking open decision — the Go side needs a markdown
  parser for the indexer and the link checker, and the browser side needs one
  for the preview

## Context

Markdown is parsed in three places, with different needs:

| Where | Needs | Language |
|---|---|---|
| The editor | Must **not** change a byte | Browser |
| The FTS5 indexer | Plain text, links resolved | Go |
| The link checker, the preview | Links, structure | Go and browser |

The research found that no markdown library re-serialises byte-identically,
because CommonMark's parse is lossy by design (ADR-0021). So the write path
cannot use a parser at all — which removes one of the three.

That leaves two parsers, in two languages, that must agree.

## Decision

### The write path uses no parser

**The CodeMirror buffer is the source of truth and is written back verbatim.**
There is no parse, no serialise, and therefore no opportunity to change a
byte. This is the splice contract in its strongest form: the contract is
satisfied by *not transforming the text at all*.

The editor still needs to *read* markdown — to render a preview, to find
links, to know where a callout ends. That is a read-only parse, and its
result never reaches the file.

### The browser parser: `remark` / `micromark`

Chosen because it is position-aware: every AST node carries a byte offset.
That makes it the right tool for the preview, for link extraction, and for
computing the ranges a future splice would need.

### The Go parser: `github.com/yuin/goldmark`

Chosen because it is pure Go, CommonMark-compliant with a GFM extension, fast,
and gives a real AST for link extraction. No cgo, so the single-binary
property holds.

### The two parsers must agree

**A conformance fixture set**: the same corpus files are parsed by both, and
the extracted plain text and the extracted links are compared.

A disagreement fails CI. This matters because the FTS5 index is built from
the Go parse, and the preview is rendered from the browser parse — if they
disagree, a search result and a preview show different things for the same
note, and the bug is nearly impossible to spot by hand.

## Consequences

**Good:**

- **The write path cannot corrupt a file**, because it does not understand the
  file at all.
- **One parser per language, both replaceable.** A Go implementation of
  remark's semantics, or a WASM build of the Go parser, is a swap rather
  than a rewrite.
- **The cross-parser check turns a silent inconsistency into a build
  failure.**

**Bad:**

- **Two parsers is one more thing to keep in step**, and the conformance
  suite is the cost of that.
- **`goldmark`'s GFM support is not Obsidian's dialect.** Obsidian adds
  wikilinks, callouts, embeds and block references on top. Those are
  extensions we implement ourselves, in both parsers, and they are the
  likeliest source of a disagreement.

**Neutral:**

- The golden corpus doubles as the conformance fixture set. One corpus, two
  jobs.
