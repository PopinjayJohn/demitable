# The threat model — what is actually being protected, from whom, and by what?

- **Status:** written 2026-10-23 as phase 0's answer to *“What is actually being
  protected, from whom, and by what?”*, and **made the source of truth** for
  what the app defends by [ADR-0051](decisions/0051-threat-model-and-not-defended-list.md).
- **This document is living.** The phase-1 exposure checklist is **derived from
  it**, and the not-defended list is binding: phase 1 must either close an item
  on it or record it as an accepted risk with the milestone that owns the
  closure.
- **Scope of this document:** the projection as the trust boundary, the phase-1 exposure posture, the credentials, the vault as untrusted input, the plugin tiers, and — most importantly — **the things that are not defended**.
- **Sources:** [`docs/phase-0-plan.md`(phase-0-plan.md) §2.10 · [`docs/phase-1-plan.md`(phase-1-plan.md) §5 · [`docs/phase-0-brief.md`(phase-0-brief.md) §10 · the ADRs ADR-0002 through ADR-0050 · [`docs/research/adversarial-review.md`(research/adversarial-review.md) §1–§3 · [`docs/design.md`(design.md) §3–§6, §8–§9.

---

## The question

> *What is actually being protected, from whom, and by what?*

## The short answer

Demitable's core promise is that **no player's client ever holds a GM's secret**. The mechanism is a server-side projection (ADR-0002, design §3 Rule 4): the vault never leaves the GM's machine, and a player receives a filtered view assembled per principal, per campaign, per session.

Phase 0's threat model says:

1. **The assets** are the GM's vault content — GM-only notes, secret blocks, GM asides — plus the players' own private content, plus the credential surface itself.
2. **The actors** are: the GM (trusted operator, and the holder of the crown-jewel credential), the legitimate player (low capability; the realistic bug class is *accidental over-inclusion*, not malice), the credential holder (join code / session code / GM secret / bearer / MCP token), the credential-less network attacker, the compromised host, and the untrusted-input contributor (imported archive, imported pack, imported plugin).
3. **The boundary** is the projection, resolved in a fixed order (scope → absolute deny → workflow gate → base level → grants → default → property level → masking).
4. **The defences** are: visibility layers at note and property level, server-side secret stripping, masking/display principals, fail-closed unknown values, expiry + single-use + revocation for credentials, SSRF-hardened opt-in import, opt-in token-gated MCP, offline-first local exposure.

And — because that is the part this phase found valuable — **the things that are not defended are now written down explicitly**, rather than implied.

## 1. The assets

| # | Asset | Owner | A leak means… | Severity |
|---|---|---|---|---|
| 1 | GM-only notes, lore and asides | GM | Campaign canon disclosed; a secret becomes public before the party is ready | **High** |
| 2 | Unrevealed / revealed secret-block text | GM | The single most valuable asset in the product: a secret that has not been revealed yet | **High** |
| 3 | Player-private content (`private` journals, owner-scoped character data not for others) | Player | A player's personal journal or private HP reaches someone it should not — or, for `private`, reaches the GM at all, which is a design bug | **High** |
| 4 | Player character sheets (`owner` sheets, `discovered` lists, grants) | Player / GM | Inconsistent visibility of character state; a party-visible sheet must not publish undiscovered locations (ADR-0050 D5) | **High** |
| 5 | Credentials: GM secret, campaign join code, session code, passphrases (argon2id), bearer tokens, cookies, MCP token | GM / player / host | GM-secret theft = whole vault; join-code theft = persistent account for one campaign; session-code theft = one seat for one session | **Critical** |
| 6 | The promise itself | — | A projection leak is not a "bad UX" — it is a correctness bug whose severity scales with the secret | **High** |

The asset that is *designed* to be leaked but only ever *revealed* is the secret block (ADR-0033/0035): a secret is a deferred grant, and the reveal is a journalled, broadcast event.

## 2. The actors

| Actor | Capability | What it means for the model |
|---|---|---|
| **The GM** | Trusted operator; holder of the GM secret | The GM's view is never filtered (principal `gm`). The GM is the only actor whose credential buys everything, and the GM machine is the only trusted host. |
| **The legitimate player** | Holds a membership (via join code → account) or a seat (via session code → guest seat) | Low threat initially. The realistic failure mode is accidental over-inclusion, not malice. |
| **The join-code holder** | A code that has not yet expired / not yet consumed | Buys a persistent player account for one campaign, for the code's 24-hour single-use window (after consumption the account persists and is revocable). |
| **The session-code holder** | A code minted for the current session | Buys a guest seat at one session, bound to a character the GM assigned; the seat sees its bound character and the live table, and nothing else. |
| **The GM-secret holder** | The long-lived `gm` credential | Buys the entire vault, including GM-only notes and unrevealed secrets; the crown jewel. Invalidated only by rotation. |
| **The bearer / MCP token holder** | An account token or the MCP token | Account token: its principal's content. MCP token: the GM's **full** view — the most powerful read surface in the app, and deliberately so (ADR-0014). |
| **The credential-less network attacker** | No credential | Sees `visibility: public` content only; on an internet-exposed server: enumeration, auth brute-force, XSS via markdown, SSRF via import, DoS. |
| **The compromised host** | The GM's machine | Whole vault. No partial exposure — this is a *known, accepted* limit, not an oversight. |
| **The untrusted-input contributor** | Archive author / pack author / plugin author | Content is attacker-influenced by construction; pack logic is sandboxed; plugin UI is not (see §8). |

## 3. The trust boundary

The boundary is the server-side projection (ADR-0002; design §3 Rule 4; ADR-0033):

- The **vault never leaves the GM's machine**.
- Players **never receive files**; they receive projections.
- A native client cannot leak GM-only notes, because it never holds them (api.md §1).

Resolution order, first match wins (ADR-0033, amended by ADR-0050/D13, ADR-0048):

1. **Scope** — campaign membership.
2. **Absolute deny** — `visibility: gm`, never serialised to a non-GM response; no grant overrides.
3. **Workflow gate** — `draft`/`submitted` visible only to owner + GM, regardless of visibility or grants.
4. **Base level** — `public` / `party` / `owner` / `private` / `discovered` / `gm`.
5. **Grants** — additional parties; never override 2 or 3.
6. **Default** — `party` inside a campaign, `gm` outside one.
7. **Property level** — pack-declared; `owner` resolves against the note's `player`, falling back to the note's visibility when there is no player; `gm` is absolute; reserved-field exceptions (`discovered`, `grants`) are `owner`-scoped.
8. **Masking** — `display:` overrides rendered values per audience, applied last.

## 4. The credentials — two in disjoint code spaces, plus the GM secret

Per [ADR-0038(decisions/0038-seats-and-code-spaces.md) and [ADR-0009(decisions/0009-player-accounts.md):

| Credential | Accepted by | Scope | Lifetime | Exchanges for | Invalidated by |
|---|---|---|---|---|---|
| **Campaign join code** | Account endpoint (`POST /api/v1/auth/join`) | One campaign | 24 h, **single use** | An **account** + campaign membership (passphrase → argon2id → bearer token) | Consumption; GM revoking the membership |
| **Session code** | Session endpoint (`POST /api/v1/sessions/:id/join`) | One session | The session | A **guest seat** (or account seat) | Session end / seat revocation |
| **GM secret** | GM-authenticated path | The vault | Long-lived, scoped to the `gm` principal | — (it *is* the GM principal) | Rotation by the GM |

Defence-in-depth on the disjoint code spaces: different endpoints, different stores, different lengths (eight characters vs six), so a session code is not a campaign code and vice versa.

**What each one actually buys an attacker:**
- A holder of a valid campaign join code can read the campaign's content the account can read — i.e. effectively the party-visible world — **until the code expires (24 h) or is consumed**, after which a persistent account exists that the GM can revoke. A player holding the code can therefore read every `party` note in the campaign for the remaining life of the code.
- A holder of a session code can reach the live table and their bound character for the life of the session; the code is read aloud over voice chat, so it is short-lived rather than secret.
- A holder of the GM secret can read everything, including GM-only notes and unrevealed secrets, for as long as the secret holds.

## 5. The projection as the boundary — the leak paths

Each path was checked against the design and the phase-0 findings:

| Path | Status |
|---|---|
| Note-level visibility | **Defended by design.** Fail-closed on unknown values; `discovered`/`private` handled (ADR-0050/0048); `gm` absolute (ADR-0033). Phase-0 Q2/Q8 exercised it on real notes; the property test now covers the demo campaign. |
| Property-level visibility | **Defended by design**, with reserved-field exceptions (ADR-0050 D5). Pack-declared visibility; `owner` fallback to note visibility; NPC-HP bug fixed by the no-owner fallback. |
| Workflow gate | **Defended.** `draft`/`submitted` → owner + GM only. `retired` is *not* a visibility gate (ADR-0050 D6). |
| Secrets | **Defended by design, by stripping.** Server strips `[!secret]` blocks / marked blocks from non-GM projections; reveal is a live, journalled event; reveal implies note visibility. |
| **The accidental class — comments** | **Found and fixed in phase 0 (ADR-0050 D12).** Obsidian `%%comments%%` were serialised into non-GM projections (hidden in preview, not stripped) — the most likely place for a GM's aside to leak. Fixed by stripping comments from non-GM projections. |
| Wikilinks into GM-only notes | **Defended by design, by rendering.** A link whose target the caller may not see renders *present but not followable*, not as a 404 (ADR-0050 D15). |
| Search ranking | **Measured, not yet remediated.** Design claims search is filtered before results reach a player; phase-0 Q6 **measured the IDF side channel** (GM-only notes shift BM25/IDF of visible results). Remedy: filter before ranking. Status: measured, remedy pending. |
| The vault index / note existence | Design position: a note's existence is not secret. Player index used; GM-only notes not listed. Content notes declared `party` make the bestiary enumerable — a scoping decision, flagged. |
| Asset endpoint | **Defended by design.** Role-scoped, ETag-cached, **404 not 403** for may-not-see; same asset 200 + ETag for may-see, 304 on `If-None-Match`; handout reveal promotes. (M1c.) |
| Write-back | **Defended by design.** Only app-initiated writes journaled; world state has one writer (GM); players get read-side surfaces. |
| MCP endpoint | **Defended by design, opt-in.** Read-only GM full view, `/mcp`, off by default, token-gated, never binds publicly by default; privacy trade-off stated at point of enabling. (Phase 1.5.) |

**The leak class that phase 0 found, and that this model generalises:** the leaks were not in the designed denial mechanism but in an *accidental* serialisation/rendering divergence — the design/implementation boundary said “hidden in preview”, while the projection serialised the bytes anyway. The generalised rule for phase 1: **any construct the renderer conceals must be re-checked at the serialisation boundary, and the projection must be the single place that decides what a role may see.**

## 6. The vault as untrusted input

A note is attacker-controlled text (GM machine compromised, imported archive, player write-back, plugin logic). The parser must survive:

- size caps — 8 MB note, 1 MB frontmatter, depth 32, 10 000 keys — rejected with clear errors, not crashes;
- YAML alias bombs — never `node.Decode` on untrusted frontmatter; bounded tree walk; yaml.v3 ≥ 3.0.1;
- duplicate keys refused; writer refuses to patch rather than patch the wrong occurrence (ADR-0049);
- type-correct YAML (booleans as `true`/`false`, wikilinks quoted, dates quoted);
- unknown `visibility` → fail closed (treat as `gm`);
- markdown rendering → **sanitisation policy must exist before the editor ships** (adversarial review §3.20; raw HTML present in the corpus).

## 7. The server's own exposure (ADR-0031)

- Topology: GM terminates TLS (Caddy/nginx/tunnel); app speaks plain HTTP on loopback behind the proxy.
- `--expose` turns on the dangerous defaults together: bind `0.0.0.0` + warning, `Secure` cookie always on, `X-Forwarded-*` trusted only from the configured proxy, stricter auth rate limits, GM auth always required, MCP off, import allowed with SSRF defence, audit log, `--public-url` required.
- Flags: `--expose`, `--public-url`, `--trust-proxy`, `--allow-http-import`, `--no-import` — the unsafe path must be made *hard*, not merely discouraged.
- Import: off by default; https only unless `--allow-http-import`; private/loopback/link-local ranges refused after DNS resolution; resolve-then-connect; redirect re-check; size/time/content-type caps; zip-slip/bomb/symlink refusal; checksums; `.demitable/` and `.git/` never imported.
- Offline is a hard requirement; no automatic update.

Flag-name inconsistency to fix in phase 1: ADR-0025 says `--insecure`, ADR-0031 and the phase-1 plan say `--allow-http-import` (contradiction table, adversarial review §2/§3.1).

## 8. The plugin tiers (ADR-0020, ADR-0029, ADR-0027)

- **Tier A (compiled-in):** trusted with everything — it *is* the program. Go's `plugin` package not used.
- **Tier B (WASM):** sandboxed logic (wazero, Extism-shaped host ABI, host functions `vault.read/write/query`, `http.fetch`, `log`, `now`; capability- and role-scoped; no WASI fs preopens; no network). The sandbox models a hostile *compiled-in author*, not a hostile plugin.
- **Tier C (vault CSS):** inert.

**Corrected privilege statement (ADR-0029):** plugin logic is sandboxed; **plugin UI is trusted** — it runs in the app's document with the app's privileges; what is offered is style/layout isolation, not security.

Consequences for a vault holding real secrets: a `dom.inline` plugin (or a compiled-in plugin) running in a GM session can read every note in the campaign, including GM-only ones, and act on the GM's behalf. The capability model constrains *accidental* overreach; it is not a defence against a hostile plugin. The import warning exists precisely because of this, and distinguishes the sandboxed tier from the in-document tier.

## 9. What is not defended — stated plainly

The phase-0 threat model, being honest about the remainder:

1. **A compromised GM host.** Whole vault exposure; no partial. Defended only by the backup story (which is app-independent by design).
2. **A GM-installed plugin whose UI is trusted** (or a `dom.inline` plugin). Accepted risk, now written down; capability model covers logic only.
3. **Credential theft inside its window.** Join code: 24 h single-use; session code: session lifetime; GM secret: long-lived until rotation. Expiry/revocation exist; enforcement (rate limits, attempt counting on the session endpoint) is phase-1 work.
4. **Search ranking side channel.** Measured in phase 0; remedy pending.
5. **Markdown-renderer XSS.** Sanitisation policy pending before the editor ships.
6. **Plugin UI isolation in Firefox/Safari.** Shadow DOM behaviour unverified (Chromium only, ADR-0029).
7. **Phase 3 live surfaces** (chat-as-notes, scratch log, shared display, seats) — modelled at ADR level, threat modelling still partial.

## 10. The phase-1 exposure checklist — adjudicated

Source: [`docs/phase-1-plan.md`(phase-1-plan.md) §5.

Verdicts under the threat model written here:

| Check | Owner | Verdict under the threat model |
|---|---|---|
| Auth rate-limited and backed off on every auth endpoint | M5 | **Design upheld; mechanism pending.** Must cover the session-code and GM endpoints too. |
| Campaign code single-use, expires, accepted only by the account endpoint | M5 | **Design upheld (ADR-0038); mechanism pending.** Disjoint stores/lengths are design-level. |
| Guest seat sees its bound character, the live table, and nothing else | Phase 3 | **Survives.** Tight by construction; guest is not a campaign member. |
| A seat token grants nothing outside its session | Phase 3 | **Survives.** Design-level; phase 3. |
| GM authenticates with a credential, not by being local | M5 | **Design upheld (ADR-0031).** Under `--expose`, GM auth is always required. |
| Absolute URLs from configured public base URL, never `Host` | M3 | **Design upheld (ADR-0031/api).** Mechanism pending. |
| Forwarded headers trusted only from configured proxy | M3 | **Design upheld (ADR-0031).** Mechanism pending. |
| Secure cookie follows configured public scheme | M5 | **Design upheld; always on under `--expose`.** |
| CORS off by default; API same-origin | M3 | **Design upheld.** |
| MCP endpoint off by default, logs warning if enabled | Phase 1.5 | **Design upheld (ADR-0014).** Mechanism pending. |
| Import refuses private address / refuses to reconnect elsewhere | Phase 1.5 | **Design upheld (ADR-0031).** Mechanism + enablement pending. |
| Import requires https, off unless explicitly enabled | Phase 1.5 | **Design upheld.** Flag-name inconsistency to fix. |
| App starts/works with NIC down, DNS unavailable | M0 | **Design upheld.** |
| No automatic update path | M0 | **Design upheld.** |

**Adjudication:** the checklist is conceptually consistent with the threat model, and its design-level claims survive. What does **not** survive as an unexamined item: the checklist predates the seat/session-code credentials and does not cover them (adversarial review §3.26) — that gap is closed here, and phase 1 should add rows. The checklist is also not the source of the threat model; under the accept-condition of question 10 it should be **derived from** it.

## 11. Contradictions found, with the ADR that now disagrees

1. **“GM-only notes are unreachable by players, **and by plugins**.”** — phase-1 DoD, design §4, ADR-0019 vs ADR-0020 (compiled-in = trusted with everything), ADR-0027/0029 (UI = trusted). **ADR-0029 now corrects the UI half; the phase-1 DoD and design §8 residue remains.** The threat model adopts the honest position: plugins are trusted at the privilege of whoever opened them; the capability model constrains accidental overreach.
2. **“The threat model is small and honest: the network the GM is on, plus one join code.”** — ADR-0010. **ADR-0038 now supersedes it** (two credentials in disjoint code spaces, plus the GM secret). The ADR-0010 text survives as history, not as the current model.
3. **“Hidden in preview”** (design §5.6) vs serialisation reality → **ADR-0050 D12** (comments stripped; fixed).
4. **`--insecure` vs `--allow-http-import`** → ADR-0031 (supersedes).
5. **Operations §5 checklist row** saying import “allowed / allowed” vs “off unless enabled” → ADR-0031/M10.
6. **The exposure checklist unowned and seat-blind** → phase-1 plan gained it mid-review; the threat model must be its source.

## 12. What phase 1 should change — concrete, with the milestone

| Change required by this threat model | Milestone |
|---|---|
| Make the phase-1 exposure checklist **derived from** this threat model, and give it an owner | M5 |
| Add checklist rows the threat model exposes but the checklist lacks: session code, GM secret, audit log, credential windows, MCP token | M5 |
| Session-code endpoint: rate limited + attempt counted; GM secret rotatable | M5 |
| GM-only-read audit log, and asset endpoint 404-not-403 (M1c already specifies the latter) | M5 / M1 |
| Search: filter before BM25/IDF ranking | Phase 1 (before M6 ships) |
| Markdown sanitisation policy before editor ships | M4 |
| Plugin UI trust decision (position (a) vs (b)) + delete “unreachable by plugins” from the phase-1 DoD | M3 / M8 |
| Browser matrix for shadow DOM before plugin host done | Phase 2 |
| Import: unified flag, default-off, https-only, SSRF defence | Phase 1.5 |
| MCP: token-gated, off by default, never public by default | Phase 1.5 |

## 13. What I would do differently

- I would have written the threat model **before** the checklist, and made the checklist a derived artefact rather than an independent list. The accept-condition of question 10 exists precisely because the two had drifted.
- I would test “hidden in preview” claims empirically instead of asserting them — which is how the `%%comments%%` leak was found.
- I would keep the not-defended list alive and growing; a threat model that cannot name what it will not defend is not one.

## The final answer

- **One-paragraph summary:** Phase 0's threat model concludes that Demitable defends, through a server-side projection and a credentialed access path, the GM's vault content (GM-only notes, secrets, asides) and the players' private content; that the two credentials (campaign join code, session code) live in disjoint code spaces with expiry/single-use/revocation; and that the internet-exposed posture is made safe by `--expose`, an opt-in SSRF-hardened import and an opt-in token-gated MCP. The phase-0 research then found, in the **accidental** rather than the designed failure class, the leak that mattered: renderer-hiding without serialisation-stripping (`%%comments%%`), now fixed. The model now also names plainly what it does not defend.

- **The answer, in three sentences or fewer:**
  1. *What is being protected, from whom, and by what:* the GM's vault content and the players' private content, from unauthorised readers — defended by the server-side projection, the credential surface and the opt-in exposure posture, and no more.
  2. *What is not defended:* a compromised GM host, a GM-installed plugin's UI, mid-window credential theft, search-ranking side channels, markdown XSS, and (for plugin UI) non-Chromium browsers — all now written down.
  3. *The generalisation:* leaks are in the accidental class (designed denial, accidental serialisation), which is why the phase-0 finding generalised from `%%comments%%` to a rule about every hiding mechanism.

- **What is now known that was previously assumed** — the most valuable section:
  - Previously assumed: “the projection hides GM-only content.” Now known: hiding was partly a rendering concern and partly a serialisation concern; the design said “hidden in preview” while the bytes were serialised — the `%%comments%%` leak. The rule: the projection is the single serialisation boundary for secrecy.
  - Previously assumed: “a plugin cannot read GM-only notes.” Now known: the phase-1 DoD claimed unreachability by plugins; the compiled-in tier is the program, and plugin UI runs in the app's document with the app's privileges (ADR-0029 correction). The honest position is adopted in this model.
  - Previously assumed: “search is part of the security boundary.” Now known: BM25/IDF is computed over the whole corpus; phase-0 Q6 **measured** the rank inflation; the remedy is filtering before ranking.

- **What is NOT defended** — the second most valuable section, and the one the project most needs: see §9.

- **Contradictions found, with the ADR that now disagrees:** §11.

- **What phase 1 should change — concrete, with the milestone:** §12.

- **What I would do differently, given what I now know:** §13.

- **Be honest about what failed:** phase 0 did not fail; the phase-0 research programme did its job badly in one respect — it assumed hiding was a serialisation concern and did not test it, then found the opposite on real data. The threat model written from that finding is the deliverable.
