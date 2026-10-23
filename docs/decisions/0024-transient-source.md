# ADR-0024: The SRD source is transient; the enriched output is published as an importable archive

- **Status:** Accepted, amended by ADR-0025 (content reaches users through

  `demitable import <url>` — an archive at a URL — not through the app) and
  by the phase-1 cut, which moved the SRD content vault to **phase 2** and
  by the amendment below (the enriched vault is *published*, not bundled)
- **Date:** 2026-10-22
- **Amended 2026-10-22:** the corpus is **version 2.28.1 and only manually
  available**. There is no URL to fetch. The scripted fetch is replaced by
  manual acquisition plus a verification step, and CI can no longer re-run
  the pipeline to check the committed output. See §"The corpus is manual".

> **Phase references renumbered 2026-10-24.** The phase map was re-cut:
> phase 1.5 was abolished, the old phase 2 became phases 2 and 3, the old
> phase 3 became phase 4 and the old phase 4 became phase 5. The
> decisions below are unchanged. See
> [ADR-0059](0059-the-phase-map-is-re-cut.md) for the mapping.

## Context

The SRD corpus arrived as `fixtures/` — 2,192 files, 53 MB, containing both
the rendered SRD markdown and the **complete** upstream JSON dataset. That
dataset is 17,939 entities, of which only **7,012 are SRD**; the other 10,927
belong to copyrighted sourcebooks that we have no right to redistribute.

Keeping `fixtures/` in the repository is a licence liability, a 53 MB merge
hazard, and 90% dead weight. But it is also the only copy of the source we
have, and nothing records where it came from — there is no licence file, no
source URL, and no version pin anywhere in the corpus.

Meanwhile the thing we actually want is the *output*: the enriched SRD
content, the 5e pack, and the demo vault. That is small, diffable, and
useful to every contributor and every user of the app.

## Decision

**`fixtures/` is a build input, not a repository artefact. The enriched output
is committed — as a development artefact, and it does not ship.**

### The lifecycle

```
   manual acquisition  ──►  fixtures/   (gitignored, transient)
                                        │
                                 srdenrich pipeline
                                        │
                    ┌───────────────────┴───────────────────┐
                    ▼                                       ▼
        packs/dnd5e.yaml                    dev/srd-vault/
        (the mechanics pack)                (the enriched content vault)
                    │                                       │
                    └────────────►  committed  ◄────────────┘
```

`fixtures/` is never committed and never shipped in the binary.

> **Amended 2026-10-22.** The corpus is **not** deleted after a successful
> build. A **pinned, checksummed copy is retained outside git** — as a
> release asset, or at a documented mirror with a recorded SHA-256 — and
> `srdcheck` verifies it before the pipeline runs.
>
> The earlier rule ("delete it after the build") was unbuildable: the corpus
> is manually acquired, so deleting it made the pipeline one-shot and made any
> later enrichment bug permanently unfixable. Retaining a pinned copy also
> makes the upstream's licence recordable, which is a precondition for
> publishing the derived archive at all.
>
> The *working tree* copy is still transient — gitignored, and removable at
> will — but the pinned copy is not.

### The corpus is manual

**The corpus is version 2.28.1, and it is only manually available.** There is
no URL, no release feed, and no scripted acquisition.

That replaces the fetch step with two things:

- **A documented manual acquisition.** The corpus is placed into a gitignored
  working directory by hand. This is a step in the build instructions, not a
  command.
- **A verification step, `tools/srdcheck`.** Because acquisition is manual,
  the pipeline must not trust what it was handed. `srdcheck` verifies the
  corpus is the expected one before the pipeline runs:

  | Check | Why |
  |---|---|
  | Version is 2.28.1 | A different corpus produces a different vault |
  | Expected file inventory present | Catches a partial or wrong acquisition |
  | Checksums of the index and a sample of entity files match the recorded ones | Catches a modified or substituted corpus |
  | No unexpected top-level entries | Catches a corpus that has acquired extra content |

  A failed check stops the build with a clear message. **Running the pipeline
  against the wrong corpus would produce a vault that looks fine and is
  subtly wrong**, which is worse than not producing one.

### What this costs

**CI can no longer re-run the pipeline.** The check "the committed output is
byte-identical to what the pipeline produces" needs the corpus, and CI cannot
obtain it.

The split is:

| Check | Where it runs | Needs the corpus? |
|---|---|---|
| The committed vault round-trips byte-identically | **CI** | No |
| Every content note validates against its pack | **CI** | No |
| Every wikilink resolves | **CI** | No |
| The committed vault still contains no non-SRD entity | **CI** | No — the vault carries its own `source` markers |
| The pipeline still reproduces the committed vault | **A maintainer, locally** | Yes |

The CI checks are the valuable ones — they run against the committed artefact
and catch any mutation. The reproducibility check becomes a maintainer task,
run when the enrichment rules change. That is a real reduction in
guarantee, and it is the price of manual-only acquisition.

### Contributors

A contributor who wants to *change the enrichment* must obtain the corpus
manually. A contributor who wants to *work on the app* does not: the enriched
vault is committed, so the corpus, the pipeline and the output are all
present, and only the pipeline itself needs the corpus to run.

### What ships, and what does not

| Artefact | Ships? |
|---|---|
| `packs/dnd5e.yaml` — the mechanics pack | **Yes.** It is small, it is our own work, and it cites the SRD |
| `dev/srd-vault/` — the enriched content | **Published, not bundled.** See below |
| `fixtures/` — the source corpus | **No.** Transient, and 90% of it is not ours to ship |

The enriched vault is **published as an importable archive** and reached with
`demitable import <url>` (ADR-0025). It is **not** bundled into the binary —
1,700 files and several megabytes in every install is not worth it, and the
binary stays a clean deliverable.

It remains a **development artefact** in the repository: the development
template, the development demo, the golden corpus, and the source of the
published archive.

### Why the content reaches users as an archive

The licence permits it (CC-BY-4.0, attribution carried in every note), the
import mechanism already exists, and it keeps the binary small. It also
demonstrates the distribution model with our own content: the same channel
any community content uses.

The cost is one command before a new user has a world. That is a fair trade
against several megabytes in every install, and it is reversible — bundling
later would be additive.

### Why the output is committed

- **It is the development template.** A developer opens it and the app is
  immediately interesting.
- **It is the development demo.** A populated world, not an empty vault.
- **It is the versioned artefact.** A diff of the template shows exactly what
  the pipeline changed, in a form a human can review.
- **It is the golden corpus** (ADR-0021). The editor's byte-identical
  round-trip test runs over a committed, diffable tree — and because the tree
  is in git, an accidental mutation of a corpus file shows up in the diff
  rather than silently weakening the test.
- **It is small.** The SRD subset is a fraction of the corpus.

### How users get content instead

**Not from us — a community member hosts the archive.**

We document the format and the build, and a community member publishes and
hosts the enriched archive at a stable URL. We host no index, no registry and
no artefact of our own. This is ADR-0011's position taken literally: the
project distributes *how to build* the content, not the content itself.

The consequence is honest and worth stating: **a new user gets no content
until someone publishes it.** The first-run experience is therefore "find a
community-hosted archive, download it, and unzip it into the vault" — and
because the archive is a zip of plain files, that needs no server feature at
all. `demitable import <url>` (ADR-0025) is a convenience that arrives later,
not the distribution mechanism.

The archive carries its terms with it: licence and attribution are in the
manifest, inside the archive, and in every emitted note.

This is why the licence consequence below is not a product problem. The
template is a development artefact in our repository, where attribution and
share-alike terms are straightforward to honour. It is never redistributed
inside a release.

### The build must be deterministic

Same input, **byte-identical** output. This is what makes committing the
output safe rather than lazy:

- Uids are derived deterministically from the source path, not randomly
  generated.
- No timestamps, no map iteration order, no hash-map ordering in the emitted
  YAML or frontmatter. Every emitted collection is sorted.
- The attribution block carries no build date, or carries one in a field
  excluded from the comparison.

**A CI check verifies that the committed vault is byte-identical to what the
pipeline produces.** If they diverge, the build fails. The committed
artefact can therefore never drift from the pipeline that owns it.

> **Amended.** The corpus is manually available, so CI cannot re-run the
> pipeline. This check is now a maintainer task, run locally. CI still runs
> every check that works against the *committed* vault — the round-trip, the
> validation suite and the link checker — and additionally verifies that the
> committed vault contains no non-SRD entity. See §"The corpus is manual".

This also means the corpus and the pipeline are the *only* things that need
to be trusted to reproduce the content — and they are far smaller and
more reviewable than 53 MB of data.

### Two works, one licence

This is the part that must not be got wrong.

| What | Licence |
|---|---|
| The SRD 5.2.1 content itself | **CC-BY-4.0**, Wizards of the Coast. Attribution required |
| The upstream community markdown rendering and the upstream JSON database, at version 2.28.1 | **CC-BY-4.0** — the same terms |

Both halves carry the same permissive licence, so the enriched vault is a
CC-BY-4.0 derived work: redistributable, including commercially, with
attribution. There is no share-alike obligation, and nothing constrains how
the repository may be shared.

The corpus is manually acquired, so its terms cannot be auto-discovered from
a licence file beside it — they are **recorded in the repository**, next to
the output they govern, in `docs/srd-vault.md` §2.5 and in the pack header.

Because a pinned copy is now retained (see §The lifecycle), the record can be
*verified* rather than remembered: `srdcheck` checks the upstream repository
URL, the commit hash, the licence file's verbatim text and its SPDX identifier
alongside the corpus inventory. **If those cannot be pinned, the archive is
not published.** That is the licence boundary's actual precondition, and it
is now a build step rather than a note.

**The licence boundary is enforced in the build regardless**: only `xphb`,
`xdmg` and `xmm` entities may reach the output, and a non-SRD entity is a
hard build error. Because the emitted notes carry their own `source` marker,
CI can re-verify this against the committed vault without the corpus.

The licence no longer forces that rule — it is now conservatism rather than
compliance. It costs nothing, because the 10,927 non-SRD entities have no use
to us, and it removes an entire category of doubt: every byte we emit traces
back to the SRD, whatever the upstream's own terms happen to say.

## Consequences

**Good:**

- The repository holds only what we want to maintain: the pipeline, the pack,
  and the content.
- The licence exposure is minimal and settled: the SRD subset and the upstream
  are both CC-BY-4.0, so the derived vault redistributes freely with
  attribution and imposes nothing on us.
- Contributors clone a small repo and get a working corpus immediately.
- The determinism check turns "the committed thing might be stale" into a
  hard failure.
- The template is diffable, so a change to the enrichment rules shows up as a
  reviewable diff across thousands of files rather than as a silent
  regeneration.

**Bad:**

- **The build is no longer hermetic, and CI cannot run it.** The corpus is
  manually acquired, so the reproducibility check moved from CI to a
  maintainer's machine. The corpus's version is pinned at 2.28.1 and verified
  by `srdcheck`, so a wrong corpus is caught — but a maintainer has to be
  there to catch it.
- **Contributors who want to change the enrichment must obtain the corpus
  manually.** Everyone else can work on the app, because the enriched vault
  is committed.
- The determinism requirement constrains the pipeline: every emitted
  collection must be explicitly sorted, and nothing may depend on map
  iteration order or wall-clock time.
- Contributors who want to *modify* the content must fork the template, not
  the source.

**Neutral:**

- The SRD content remains the project's best test corpus. That does not change
  with where the bytes live.
- The pipeline is still a build-time tool, not a plugin, and is still not
  shipped in the binary.
- **The licence no longer constrains whether the vault could ship.** It could
  legally be redistributed, bundled in the binary, or published as an archive
  for users to import — all three are permitted with attribution. Publishing
  it as an archive is now the choice; bundling it is deliberately not.
