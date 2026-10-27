# ADR-0062: The session cookie is SameSite=Lax, not Strict

- **Status:** Accepted (landed 2026-10-27; the implementation it records
  shipped in phase 4, F0a, and is pinned by committed tests)
- **Date:** 2026-10-27
- **Amends:** ADR-0009 (player accounts — the `SameSite=Strict` session-cookie
  sentence)

## Context

ADR-0009 specifies an HttpOnly, `SameSite=Strict` session cookie. The
implementation ships `SameSite=Lax` (`setAuthCookie` in `cmd/demitable/auth.go`),
and F0a deliberately left it unchanged while gathering the evidence for this
amendment: the passphrase-choice endpoint (join → set-passphrase → login) is
now the proof that auth works as designed, so the cookie attribute can be
judged on its own merits rather than changed silently inside a feature.

## Decision

The session cookie is `HttpOnly`, `Secure` following the configured public
scheme, and **`SameSite=Lax`**. ADR-0009's `Strict` is superseded for the
session cookie. Nothing else in ADR-0009 changes.

## Evidence

- Under `Strict`, browsers omit the cookie on every cross-site request,
  **including top-level navigations**. For this app's shape that is a real
  regression, not a corner: a player who clicks a shared link (chat message,
  email, wiki page, QR code) to `http://<gm-lan-ip>:<port>/…`, or who reaches
  the app through a reverse proxy whose registrable domain differs from the
  navigation origin, arrives anonymous and is forced to re-authenticate.
  Link-sharing is a core tabletop workflow (session codes are read aloud and
  pasted into chats), so `Strict` taxes the normal path. Verified by
  inspection of the cookie code and the F0a review (phase-4 F0 gate).
- `Lax` keeps the protection that matters here: it withholds the cookie on
  cross-site `POST`/fetch/WebSocket-style requests — the CSRF-relevant case —
  while attaching it on safe top-level `GET` navigations so shared links work.
  The app's mutations are JSON POSTs, and auth is bearer-or-cookie on every
  route with the cookie `HttpOnly`, so the dominant risk (cross-site mutation)
  is covered under `Lax`.
- The `Strict`→`Lax` delta for this app is therefore usability, not a widened
  trust boundary: no new principal, no new credential, no change to what any
  caller may see. Agreed by the F0a builder and the independent F0a reviewer.

## Consequences

- `docs/decisions/0009-player-accounts.md` is **not** edited; this ADR is the
  amendment. New code and reviews cite Lax as the decision.
- If the app ever serves state-changing `GET`s, this decision must be
  re-opened — `Lax` permits top-level-GET cookies, and that assumption is
  load-bearing.
