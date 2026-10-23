# The corpus's provenance

> The corpus is a **transient build input**: fetched by hand, gitignored, and
> deleted after a build. This document records what it is, how it is pinned,
> and — the reason the pinning works the way it does — **why it is pinned by
> content rather than by source reference.**

## The record

| Field | Value |
|---|---|
| Corpus version | **2.28.1** |
| Released | **2026-05-24** |
| Files | 2,192 (1,693 markdown, 499 JSON) |
| Tree SHA-256 | `fb62b33304e1e07d99310f3cc00caecdf2343d784a07785a07c247c2b34bec1f` |
| Markdown SHA-256 | `7ccdb2ca0dc628444e825d8ea782e993f43f0dd0ee9541b2c5c8f51eb644dc11` |
| Data SHA-256 | `7d116247f8d21f97d1b8cd40dc613a6c9770f80dafae0b81260df35eb21af194` |
| Acquisition | Manual only |
| Content licence | **CC-BY-4.0** — Wizards of the Coast |
| SPDX identifier | `CC-BY-4.0` |
| Share-alike | No |
| Redistribution of derived output | Yes, with attribution |

The machine-readable copy is [`tools/srdcheck/provenance.json`](../tools/srdcheck/provenance.json),
and [`tools/srdcheck`](../tools/srdcheck/main.go) verifies the corpus against
it:

```
go run ./tools/srdcheck -verify
```

A changed, added or removed file changes the tree hash, and the verifier says
so. That is the whole of the check, and it is enough for the job it does.

## Why the version is self-declared

The corpus carries its own changelog. The version and the release date are
read out of it, so they are stated by the artifact rather than asserted by
this repository. A third party can check `2.28.1 / 2026-05-24` against the
upstream's release announcement without trusting anything here.

The corpus's notes also carry their own provenance markers — `Source: SRD
5.1`, `Source: SRD 5.2 and the Free Rules (2024)`, and the SRD 5.2.1 edition
— so the *content's* licence is legible in the content itself, not only in a
table in a design document.

## Why it is pinned by content and not by URL and commit

The obvious way to pin a corpus is a repository URL and a commit hash. That
is what the design documents originally specified, and it is the wrong pin
here, for a reason that was found by looking rather than by reasoning.

**The upstream's canonical repository is no longer publicly resolvable.**
Checked on 2026-10-23:

- the upstream's GitHub organisation has exactly one public repository, a
  tracker last pushed in 2019;
- the canonical data repository returns 404;
- **four unofficial mirrors exist.** All four are forks. All four carry an
  MIT licence at the *repository* level, which covers the tool and not the
  data. One was last pushed in 2022, two in 2023, and one in 2026.

So a commit hash could only ever have been pinned against a fork. A hash of
a fork is not evidence of anything: the fork may be stale, modified, or
subsequently re-licensed, and there is no authoritative source to compare it
against. Pinning a fork and calling it provenance would be worse than not
pinning at all, because it would look like provenance.

Pinning the **content** has none of those problems:

- it is verifiable with no network and no upstream at all — `sha256sum` and
  a file list;
- it survives the upstream disappearing, which has already happened once;
- it is what the build actually depends on. The build reads the files, not
  the repository.

**The licence claim does not rest on the upstream at all.** The published
vault ships only the SRD subset, and the SRD's terms — CC-BY-4.0, Wizards of
the Coast — are authoritative, independently checkable, and unrelated to
whether a mirror is still up. The upstream's terms matter only for the
non-SRD material, and the build does not ship any of it.

That is why the licence boundary is `srdKeys` **membership** rather than a
source tag: 3,990 keys, with 23 unresolvable. The build emits only what
`fixtures/all-index.json` lists in `srdKeys`, so every byte of output traces
back to the SRD.

## What this changes

- `docs/srd-vault.md` §2.5 no longer promises a URL and a commit hash, and
  no longer promises that `srdcheck` verifies "the licence file's verbatim
  text and its SPDX identifier" from a file it fetches. It verifies the
  corpus it is given, against a recorded hash, and reads the version out of
  the corpus's own changelog.
- The provenance record lives in the repository, so it survives the corpus.
  The corpus does not survive the repository, and that is deliberate.

## What this does not change

- The corpus is still **manually acquired**, and the project still hosts no
  registry and no artefact. The published archive is community-hosted.
- The corpus is still **deleted after a build** and is still gitignored. It
  is a build input, not a dependency the repository carries.
- The licence boundary is still `srdKeys` membership, and a non-SRD entity
  reaching the output is still a hard error.

## A standing risk

The upstream's canonical repository being unresolvable means the corpus
cannot be *re-derived* from an authoritative source. The hash proves that
the corpus in hand is the corpus that was reviewed; it does not prove that a
reviewer could obtain the same corpus again.

That is a real limitation, and it is the argument for the corpus being a
**build input with a recorded hash** rather than a vendored dependency: the
project's licence position never depends on it, and its reproducibility is
bounded by a fact that was true before this project existed.

If the corpus is ever needed again in a form that can be re-derived, the
right answer is to obtain it from the upstream's own distribution channel
and record that channel's version and hash — which is exactly the shape of
the record above.
