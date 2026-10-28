# The API contract

> What a client — the browser app today, a native mobile app tomorrow — is
> allowed to rely on.
>
> **Status: draft.** The architecture is client-agnostic, but the contract has
> never been written down. This document is that first draft, and it closes
> the gaps a native client would hit.
>
> Related: ADR-0002 (trust boundary), ADR-0009 (accounts), ADR-0014 (MCP),
> ADR-0017 (between-sessions access), ADR-0025 (archive import),
> ADR-0026 (assets), ADR-0030 (derived caches).

---

## 1. Is the app actually server–client separated?

**Mostly — and the separation is in the right place, but three things are
missing.**

### Already separated

| Concern | Where it lives | Native client impact |
|---|---|---|
| The vault — markdown, YAML, attachments | Files on the GM's machine | A client never touches them; it asks the server |
| **The projection** — visibility, ownership, discovery | **Server-side** | A client receives *already-filtered* data. This is the important one: a native app cannot leak GM-only notes, because it never holds them |
| The formula and dice engines | Server-side (Go) | **Gap — see §4** |
| Live session state | Server-side, streamed over WebSocket | A client is a thin view; it sends commands, not state |
| Accounts and sessions | Server-side | A client authenticates and receives a token |
| Assets | Server-side, role-scoped | Fetched like any other resource |

The projection being server-side is what makes a native client *safe* rather
than merely possible. A client that read the vault directly would have to
reimplement the visibility rules, and a bug in that reimplementation would
leak secrets. One implementation, on the server, is the right shape.

### Not separated — the three gaps

**Gap 1 — No written contract.** Every endpoint, event, error shape and
versioning rule has been discussed and never specified. A native client
developer would be guessing.

**Gap 2 — Auth was browser-shaped. Closed in implementation.** Every route
accepts a bearer token, the session cookie, or `?token=` alike, and every
auth endpoint returns the token as JSON. No refresh endpoint (re-authenticate
on expiry). What remains is UI: the webapp spends no credential yet (F5).

**Gap 3 — The compute boundary is half-specified.** The sheet computes
server-side (`GET /characters/:uid` returns `computed` plus `?expand=1`;
`POST /roll` rolls into the shared log). The generic escape hatch
(`POST /compute` for wizard preview) does not exist yet — it is F0 scope.

**Gap 2 and 3 are small and fixable. Gap 1 is the real work.**

---

## 2. Authentication

**Two credentials, in disjoint code spaces** (ADR-0038). The server accepts
**either** a session cookie or a bearer token on every request.

```
POST /api/v1/auth/join      { code }              → { token, expires_at, player }
POST /api/v1/auth/login     { player, passphrase } → { token, expires_at, player }
POST /api/v1/auth/gm        { secret }             → { token, expires_at, role: "gm" }
POST /api/v1/auth/set-passphrase { passphrase }    → { token? }   (aliases: /auth/passphrase, /auth/set_passphrase)
POST /api/v1/auth/gm/rotate { secret }            → { gm_secret, rotated }   (aliases: /auth/rotate, /auth/gm-secret/rotate, /auth/rotate-gm-secret)
POST /api/v1/sessions/:id/join { code }           → { token, expires_at, seat }
```

| | Campaign join code | Session code |
|---|---|---|
| Scope | One campaign | One session |
| Lifetime | 24 hours, single use | The session |
| Exchanges for | An **account** and a campaign membership | A **guest seat** |
| Ends with | Never | The session ends |

**They are accepted by different endpoints and validated against different
stores.** A campaign code presented to the session endpoint is not a code that
endpoint knows. Their lengths differ — eight characters against six — so they
are not confusable by eye or by voice either.

- **The GM secret** lives in the vault's `.demitable/config.yaml` and is
  printed at startup. It is a long-lived token scoped to the `gm` principal,
  and it is what a native GM app presents.
- **The browser app uses the cookie.** The same endpoints set it. The native
  app ignores the cookie and stores the token.
- **Tokens are bearer tokens.** `Authorization: Bearer <token>`. They expire;
  the client refreshes by re-authenticating.
- **WebSocket clients pass the token in the query string** (`/ws?token=…`),
  because the browser WebSocket API cannot set headers. A native client may
  use the header instead; both are accepted.

### Principals, not roles

The projection resolves against a **principal**, not a role enum:

| Principal | Who it is | Resolves against |
|---|---|---|
| `gm` | The vault's owner | Everything — the GM's view is never filtered |
| `account` | An account, by id | Its campaign memberships, its owned notes, its grants |
| `seat` | A session seat, by id | Its bound character, and the grants made to it |
| `anonymous` | No credential | `visibility: public` only |

**A principal says who is asking; the visibility model says what they may
see.** An account and a seat differ only in how they resolve — one through
campaign membership and ownership, the other through its character binding
and its grants.

A principal cannot be upgraded by a request. A seat token cannot become an
account token, and neither can become `gm`: that needs the GM secret, which
is a different credential. And a player-scoped client cannot ask for GM-only
data, because the server filters by the *principal's* resolution, not by a
parameter in the request.

### A seat binds to a character

- an **account seat** — one of the account's own characters, or one the GM
  assigned to it;
- a **guest seat** — a character the GM assigned in advance, since a guest
  owns nothing;
- a **spectator seat** — no character, for someone watching.

**The seat's bound character is implicitly visible to that seat**, as if the
seat had been granted it. That is the seat-scoped projection, and it is what
makes the guest case work: a guest sees their character and nothing else.

A guest seat does **not** see campaign notes — a guest is not a campaign
member. It sees the session's live state, which is the table rather than the
campaign, and anything explicitly granted to it.

---

## 3. REST surface

All under `/api/v1/`. All responses are JSON.

### Notes

```
GET    /api/v1/notes                      list, filtered and paged
GET    /api/v1/notes/:uid                 one note
PUT    /api/v1/notes/:uid                 save (optimistic concurrency)
PATCH  /api/v1/notes/:uid                 per-key frontmatter merge
DELETE /api/v1/notes/:uid                 delete
GET    /api/v1/notes/:uid/links           outgoing links and backlinks
GET    /api/v1/notes/:uid/render          rendered markdown, for preview
```

`GET /notes`, `GET /notes/:uid` and `GET /notes/:uid/render` accept
`?as=<account>`: a GM-gated preview-as. The GM names an account (by id
or player name) and receives exactly what that account would receive —
the principal is built from the stored account record, so parity with
the account's own request holds by construction. Unknown targets are a
bare 404 (indistinguishable from a hidden note); a non-GM naming anyone
but themselves is a 403. The read is audited as the GM caller, never as
the target. This is what the webapp's player-eye renders through.

Listing is cursor-paged and filterable:

```
GET /api/v1/notes?type=character&system=dnd5e&tag=pc&status=active
               &q=ilia&limit=50&cursor=…
```

### Characters and entities

```
GET  /api/v1/characters/:uid          sheet: data, computed values, equipment
POST /api/v1/characters               create from the wizard's result
GET  /api/v1/characters/:uid/requests pending review items for this player
POST /api/v1/characters/:uid/submit   submit for GM approval
```

### Packs

```
GET  /api/v1/packs                    installed packs
GET  /api/v1/packs/:id                the pack, including its content declarations
GET  /api/v1/packs/:id/validate       lint result
GET  /api/v1/packs/:id/content        content notes of a given content_type
```

### Sessions and seats

Status: F1 scope; none of these endpoints are implemented in F0.

A **session** is one sitting of a table. A **seat** is one person at that
table, playing one character, or watching.

Two identities, and they are not the same thing:

- An **account** is who you are *in this campaign* (ADR-0009). It is
  long-lived, it owns your characters, and it is what gives you the
  between-sessions view.
- A **seat** is who is *at this table, right now*. It is session-scoped, it
  names the character being played, and it may belong to an account or to a
  guest who has none.

They are separate because the cases are genuinely separate: a player may have
an account and not be at the table today; a guest may come to one session
without ever making an account; a player may sit at two tables in a campaign,
playing a different character at each.

```
POST /api/v1/sessions                 create a live session
GET  /api/v1/sessions/:id             session state
POST /api/v1/sessions/:id/seats       add a seat in advance (GM)
POST /api/v1/sessions/:id/join        take a seat
POST /api/v1/sessions/:id/close       end it, and trigger write-back
GET  /api/v1/sessions/:id/log         the dice and event log
```

#### Joining

```
POST /api/v1/sessions/:id/join
{ "code": "K7F2-9QX4" }                       → { seat, token }
{ "player": "p_8f2a", "passphrase": "…" }    → { seat, token }
```

- The **session code** is what the GM reads out, or types into a chat. It is
  short, unambiguous (no `O`/`0`, `I`/`1`, `L`), shown on the GM's screen,
  and **rotates per session**. It is six characters, against the campaign
  code's eight, so the two are not confusable by eye or by voice.
- An **account holder** joins with their passphrase instead, and chooses which
  of their characters to sit as. If they have no character, they take a
  spectator seat.
- A **guest** joins with the code alone and gets a session-scoped identity.
  A guest has no between-sessions access — they were at the table, and that
  is all they get. This is deliberate: a guest is not asked to create a
  passphrase at the table.
- The returned **token is scoped to that session and that seat.** It is not a
  campaign credential, and it grants nothing outside the session.

#### Seats and characters

A seat names the character being played:

| Seat | Character | Visibility |
|---|---|---|
| `account` | One of the account's own characters | The owner sees it; other players see it only if the character is `party` or `public` |
| `account` | A character the GM assigned to the seat | As the character's own visibility says |
| `guest` | A character the GM assigned in advance — a guest owns nothing | The character is implicitly visible to that seat |
| `spectator` | None | The board and the public view only |
| `gm` | The GM's own, if any | Everything |

**The seat's bound character is implicitly visible to that seat**, as if the
seat had been granted it. That is what makes the guest case work: a guest
sees their character, and nothing else.

A seat naming a character the caller may not play is rejected — the
projection still applies to a seat, because a seat is not a privilege. The
GM can assign a character to a seat in advance, which is how a guest gets one.

The GM can add seats in advance, so a player who arrives late can be seated
without the GM re-sharing the code.

### The journal

```
GET  /api/v1/journal                  app-initiated changes, with diffs
POST /api/v1/journal/:id/revert       revert one change, or a batch
```

### Accounts (GM only)

```
GET  /api/v1/players                  accounts (id, player, campaigns — never passphrase hashes)
POST /api/v1/players                  create a player, returns a join code
POST /api/v1/players/:id/join-code    mint a fresh one  (not yet implemented)
```

`POST /api/v1/players` accepts an optional `campaign` (the campaign
link as written in notes, e.g. `[[The Salt Debt]]`); it defaults to
`[[Campaign]]`. The joined account inherits the code's campaign.

### Import

```
POST /api/v1/import                   download an archive, validate, apply
GET  /api/v1/import/report            what was imported and what was rejected
```

---

## 4. The compute boundary

**This is the gap that matters most for a native client**, because a character
sheet is mostly *computed* values: hit points, modifiers, spell save DC,
passive perception, attack bonus.

### The rule

> **The server computes. A client asks.**

A client never evaluates a pack formula itself. It receives computed values
from the server, and it can ask the server to compute more.

### Endpoints

```
GET  /api/v1/notes/:uid?expand=1      the note, plus its computed values

POST /api/v1/compute                  evaluate formulas in a context
{
  "formulas": ["max_hp", "spell_dc"],
  "context": { "uid": "01JBX…" }      // or an inline entity
}
→ { "values": { "max_hp": 38, "spell_dc": 15 } }
```

`?expand=1` returns the pack's *sheet-visible* formulas pre-computed, which is
what a sheet view actually needs. `/compute` is the explicit escape hatch, for
a wizard previewing a value as the user changes an ability score.

### Dice

```
POST /api/v1/roll
{ "workflow": "attack", "actor": "01JBX…", "target": "…", "shared": true }
→ { "roll": {…}, "result": {…}, "logged": true }
```

**The server rolls.** Not for trust — for a player on a phone is not going to
implement the pack's dice workflows and keep them consistent — but because a
roll has to land in the **shared** log, and the log lives on the server. A
client that rolled locally would have to post the result, which is exactly the
"untrusted input" case the roll endpoint exists to remove.

`shared: false` rolls privately — the GM rolling a secret check.

### The alternative, and why not

The formula engine could be compiled to WASM and shipped to clients, so they
could compute locally and offline. It has no dependencies, so it would
compile cleanly to `wasip1`.

That is the natural upgrade if client-side preview ever matters more than
simplicity — and it would keep *one* implementation, since the core would host
the same `.wasm`. It is not needed for v1, because the server is always
present and local: the GM's machine runs it, and players reach it over the
LAN. A round trip per keystroke in a wizard is a local round trip.

**Decision deferred**, and recorded here so it is a choice rather than an
oversight.

---

## 5. The WebSocket protocol

```
GET /ws?token=…            (or an Authorization header, for native clients)
```

A single connection carries everything. The server pushes; the client sends
commands. Both directions use one envelope:

```json
{ "v": 1, "type": "session.state", "id": "…", "data": { } }
```

### Server → client

| Type | Data |
|---|---|
| `hello` | session id, role, the vault's clock, the client's capabilities |
| `vault.changed` | `{ uid, kind: "created\|updated\|deleted" }` — also emitted for external (Obsidian-side) edits via the vault watcher, so a client that only listens here still converges |
| `session.state` | CUT (F5): no live session-state engine; clients read session state via REST |
| `session.patch` | CUT (F5): no live session-state engine; clients read session state via REST |
| `roll.result` | a roll that happened, with full provenance |
| `note.saved` | a note was written, by anyone |
| `secret.revealed` | a secret was revealed — `{ note_uid, block_id, audience, by }` |
| `error` | `{ code, message }` |

### Client → server

| Type | Data |
|---|---|
| `session.command` | move a token, change HP, add a condition, reorder initiative |
| `roll.request` | ask the server to roll |
| `note.command` | a structured edit to a note |
| `secret.reveal` | reveal a secret — `{ note_uid, block_id, audience }` |
| `ping` | keepalive |

**The client never sends authoritative state.** It sends *intents*, and the
server applies them and broadcasts the result. That is what keeps a player
from setting a token's HP by guessing the message format, and it is what makes
the WebSocket protocol safe to expose to a native app.

**A secret reveal is an intent, not a state change.** The GM sends
`secret.reveal`; the server applies it, persists it to the session tier, and
broadcasts `secret.revealed` to everyone. The server is authoritative, so
clients cannot diverge — and because the text was never sent to a non-GM
client, a reveal is a *permission* change rather than a data transfer.

Clients **refetch the note** on a `secret.revealed` event. One note is cheap,
and it avoids a second serialisation path that could drift from the first.

### Reconnection

The client reconnects with the same token and receives a full `session.state`.
State is recoverable from the server's session store (ADR-0030), so a dropped
connection — or a crashed player app — costs nothing.

---

## 6. Assets

```
GET /asset/<vault-relative-path>
```

- Requires the caller's token.
- **Role-scoped** (ADR-0026, F0b): served only if a note visible to the
  caller references it (GMs see everything).
- **404, not 403**, for anything the caller may not see.
- `ETag` from the content hash; `If-None-Match` yields a 304. Range requests
  supported, so a native app can stream a large map progressively.

---

## 7. The error envelope

Every error, from every endpoint:

```json
{
  "error": {
    "code": "not_found",
    "message": "no such note",
    "detail": { "uid": "01JBX…" }
  }
}
```

`code` is a stable machine-readable string; `message` is for humans and may
change; `detail` is optional structured context.

| Code | Meaning |
|---|---|
| `unauthenticated` | No token, or an expired one |
| `forbidden` | Authenticated, but the projection hides it |
| `not_found` | Does not exist, **or** is hidden by the projection |
| `conflict` | The write lost an optimistic-concurrency race — the note changed underneath |
| `invalid` | The request was malformed, or the pack was invalid |
| `unsupported` | The client is asking for a version this server does not speak |

**`not_found` and `forbidden` are deliberately distinct for notes, and
deliberately identical for assets.** A note's existence is not secret; an
asset's path space is not either, so an asset you may not see is a 404.

---

## 8. Versioning

- The API is under `/api/v1/`. The version is in the path, not a header.
- **Additive changes are non-breaking.** New endpoints, new optional request
  fields, new response fields, new `type` values in the WebSocket protocol.
- **Breaking changes need a new major.** A `v2` runs alongside `v1` for a
  deprecation window.
- The WebSocket envelope carries `v`, and the server **ignores** message
  types it does not recognise rather than erroring — so a newer client can
  talk to an older server.
- The plugin API has its own version (`api: 1` in a manifest, ADR-0020). It
  is independent of the HTTP version.
- The MCP endpoint speaks MCP's own version negotiation, unchanged.

## 8a. Deployment, and what it means for the contract

The server may run 24/7 and be reachable over the internet, behind TLS
termination the GM provides (ADR-0031). The app speaks plain HTTP and sits
behind the GM's proxy or tunnel.

Three things this forces on the contract:

**The server does not trust the request's origin by default.** It generates
absolute URLs — invite links, asset links — from a **configured public base
URL**, not from the `Host` header. A `Host` header is attacker-controlled,
and a poisoned invite link is a phishing primitive. Under `--expose`, the
public base URL is required.

**Forwarded headers are only trusted when configured.** `X-Forwarded-For` and
`X-Forwarded-Proto` are honoured only if the server is told which address its
proxy sits at (`--trust-proxy <addr>`). Otherwise a caller can spoof their
source address and walk straight through the rate limiter.

**CORS is off by default.** The browser app is served by the same server, so
it is same-origin and needs nothing. A native client is not a browser and
needs nothing either. A separate web client is a configuration, not a
default.

The cookie's `Secure` attribute follows the configured public scheme, so the
same build works behind an HTTPS proxy and on a plain-HTTP LAN.

---

## 9. What a native client still cannot and should not do

- **It cannot see anything the projection hides.** By design.
- **It cannot write a note the GM's server has not accepted.** Every write
  goes through the same per-key write path (ADR-0004, design doc §5.5), so the
  journal, the watcher and the frontmatter surgery behave identically no
  matter who is writing.
- **It cannot evaluate a pack formula locally.** It asks the server (§4).
- **It should not read the vault directory.** It would bypass the projection
  entirely. A native GM app is a client of the API like any other.
- **It cannot install a plugin.** Plugins are installed by the GM through the
  archive import (ADR-0025), and executable plugins require an explicit
  confirmation (ADR-0027).
- **It cannot make the server fetch a URL** except through the import
  endpoint, which is off by default and defends against SSRF (ADR-0031).

## 9a. The constraint that keeps a native client possible

The only requirement for mobile is the **possibility** of one, without a major
rewrite. That is a design constraint, and it is stated once so it does not
drift:

> **The server must not require a browser, and the browser client must hold no
> state the server does not know about.**

In practice, that means:

| Requirement | Where it is satisfied |
|---|---|
| The API is the contract, versioned and documented | §3–§8 |
| Auth works without a cookie jar | §2 — bearer tokens are first-class |
| The server computes formulas and dice | §4 |
| The pack's sheet layout and content declarations are declarative | ADR-0003, §6 |
| The WebSocket protocol is client-agnostic, and the client sends intents | §5 |
| No server-side rendering, no client-only state assumptions | throughout |

**No native SDK is implied.** The contract is the enabler; Swift and Kotlin
SDKs are a separate product decision, and a large one.

---

## 10. Open items

**Closed in F0.**

- `POST /api/v1/compute` — implemented (F0b); the `expression` field is
  rejected, not evaluated.
- `DELETE /api/v1/notes/:uid` — implemented (F0b), journaled.
- `GET /api/v1/notes/:uid/links` — implemented (F0b), outgoing links and
  backlinks, projection-filtered.
- `GET /api/v1/journal` and `POST /api/v1/journal/:id/revert` —
  implemented (F0b).
- `GET /api/v1/characters/:uid/requests`, `POST .../approve`,
  `POST .../reject` — implemented (F0b); approve/reject are journaled.
- `POST /api/v1/assets` (plus `/assets/upload`, `/upload`, `/asset/upload`,
  `/asset`) — implemented (F0b); server-generated basenames, confined to
  `Assets/`.
- `GET /asset/<path>` projection-scoping — closed by F0b (`assetVisible`
  over projected note bodies). Range, ETag and 304 behaviour as in §6
  (single range).
- `POST /api/v1/characters/:uid/submit` — implemented (was mislabeled
  unimplemented); sets `workflow: submitted`.
- Auth (F0a): `POST /api/v1/auth/set-passphrase` and
  `POST /api/v1/auth/gm/rotate`, with seven route aliases (three and four).
- Character computed values: `GET /api/v1/characters/:uid?expand=1` and
  generic evaluation via `POST /api/v1/compute` (F0b).
- Filter/query language (`list_by`) and opaque signed cursors — closed in
  O2 (ADR-0047).

**Scoped / not yet implemented.**

- `POST /api/v1/players/:id/join-code` — not implemented.
- All `/api/v1/sessions/*` — shipped in phase 4; see "Sessions and seats"
  above (create, join, seats, close with write-back, log, encounters,
  fog, tabletop).
- OpenAPI document generated from handlers — not yet; the endpoint list
  remains a sketch.
- Native-client SDK (Swift/Kotlin) — separate product decision, not
  implied by this contract.
- Offline behaviour for a native client — undecided.
- **Un-reveal (conceal a revealed block again)** — no endpoint, and not
  just a missing inverse: reveals are monotonic and journalled,
  an audience grant may already have propagated, and clients that
  received the broadcast hold the text. Revocation needs its own
  semantic (per-audience? per-block? what does the journal record?),
  so this stays a backlog item with a design pass, not a quick
  endpoint. The webapp's reveal affordances are one-way on purpose.
- **Per-player reveal UI** — the `secret.reveal` audience already takes
  `{ players: [...] }` and the display target, and reveals persist
  per-audience server-side; the webapp only exposes "reveal to the
  table". A targeted-reveal picker is UI work on an existing contract.
- **Inspector decision chain** — the webapp's "why can this role see
  this note" panel has no server source yet; the projection returns a
  verdict, not the rule chain that produced it. Exposing a
  stable, non-leaking chain (no GM-only rule text to non-GM callers)
  is a separate endpoint design.
- **Campaign-declared fields in note responses** — the wiki Home view
  needs `members`, `packs` and `home` from the campaign note in
  machine-readable form. Non-reserved fields are never serialised
  today; serving any of them is a visibility-model change (what
  audience may see `members`, a list of account ids?), so this needs
  its own ADR before any implementation. Until then the webapp
  resolves `home:` where the raw file is readable (GM view) and uses
  the `type: home` convention otherwise, and does not display
  members or packs.

**Cut.**

- None.
