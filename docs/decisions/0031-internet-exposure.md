# ADR-0031: The server may be internet-exposed, behind the GM's own TLS termination

- **Status:** Accepted
- **Date:** 2026-10-22
- **Supersedes:** ADR-0010 (LAN-first, no TLS in v1)

## Context

ADR-0010 assumed the server was reachable only on a LAN, with no TLS, and
recommended a VPN for remote play.

The real deployment is different:

- **The server runs 24/7**, on a machine that is always on.
- **It is reachable over the internet.** Players are not in the same room, and
  often not in the same country.
- **The GM provides TLS termination and takes responsibility for securing it.**
  A reverse proxy with a real certificate, or a tunnel — Caddy, nginx,
  Traefik, Tailscale Funnel, Cloudflare Tunnel. The GM owns the DNS, the
  certificate renewal, the firewall, and the hardening.
- **We still aim for fully offline and local use.** Nothing core may require
  the internet.

This changes the *attack surface*, not the architecture. The projection layer
(ADR-0002) is unchanged: the vault never leaves the GM's machine, and a
client only ever receives what its role allows. What changes is that the
process is now reachable by anyone on the internet, so the paths *into* it
matter in a way they did not when the audience was a living room.

### The one new serious risk

**ADR-0025's archive import fetches an arbitrary URL from the server.** On a
LAN, that is a convenience feature. On an internet-facing server, it is a
**server-side request forgery** primitive: a caller can make the server fetch
`http://169.254.169.254/…` (cloud instance metadata), `http://127.0.0.1:…`
(the local admin interface), or anything else the host can reach, and the
response comes back to the caller.

This is the sharpest consequence of the change and it is handled explicitly
below.

## Decision

**The app speaks plain HTTP and sits behind a proxy the GM operates. It
provides an `--expose` mode that tightens its defaults for that deployment.**

### Topology

```
        internet
            │  https, GM's certificate
            ▼
   ┌──────────────────┐
   │  Caddy / nginx / │   ← the GM's responsibility
   │  a tunnel        │      certificate, renewal, firewall
   └────────┬─────────┘
            │  http://127.0.0.1:8080
            ▼
   ┌──────────────────┐
   │  demitable       │   ← plain HTTP, loopback or LAN
   │  the vault       │      24/7, on the GM's machine
   └──────────────────┘
```

The app does not terminate TLS, does not manage certificates, and does not
open outbound connections except where a user explicitly asks for one.

### `--expose` mode

Running with `--expose` turns on the defaults that only matter when the
server is reachable from outside:

| Behaviour | Default | Under `--expose` |
|---|---|---|
| Bind address | `127.0.0.1` | `0.0.0.0`, with a warning printed |
| Cookie `Secure` flag | follows the configured public scheme | **always on** |
| `X-Forwarded-*` headers | ignored | trusted **only** from the configured proxy address |
| Auth rate limiting | on | on, stricter |
| GM authentication | implicit when local | **always required** — the GM secret, like any other client |
| MCP endpoint | off | off, and it logs a warning if enabled |
| Archive import | allowed | allowed, with SSRF defence always on |
| Audit log | auth events | auth events, plus reads of GM-only notes |
| Public base URL | unset | **required** — used to generate invite and asset links |

The point of the flag is that **the GM should not have to remember which
defaults are dangerous**. One flag says "this is exposed", and the settings
that matter change together.

### The GM's responsibilities, stated plainly

The app will not do these, and the documentation must say so without
weaselling:

- Terminate TLS and manage the certificate.
- Keep the host patched and the firewall correct.
- **Back up the vault.** This is the big one: if the box is compromised, the
  vault is gone.
- Rotate the GM secret and revoke player accounts.
- Keep the app updated.

**The backup story is the one that is easy, because of the architecture.**
The vault is plain files in a directory. The GM's backup is `git`, rsync, or a
file copy — and it is completely independent of this application. The app
cannot lose the user's data, because it does not own it. That is worth
stating loudly in an internet-exposed world: **losing the server loses the
*cache*, not the *campaign*.**

### The import endpoint must defend against SSRF

Always, not only under `--expose`, because a LAN is not a trusted network
either.

- **The URL must be `https`**, unless `--allow-http-import` is passed.
- **Private, loopback and link-local address ranges are refused**, after DNS
  resolution: `127.0.0.0/8`, `10/8`, `172.16/12`, `192.168/16`, `169.254/16`,
  `::1`, `fc00::/7`, `fe80::/10`.
- **The resolved address is checked, and the connection is made to the
  resolved address** — otherwise a DNS rebinding attack resolves to a public
  IP, passes the check, and re-resolves to a private one for the connection.
- **Size, time and content-type caps**, and a hard timeout.
- **Redirects are followed at most a few times, and re-checked** at each hop.
- The import is **disabled by default** and must be enabled per installation.

This is a real amount of security code, and it is security code in a project
whose main claim is that it is a local-first game tool. It is the price of
ADR-0025, and it is the right price to pay for a feature this useful.

### Offline remains a hard requirement

No core feature may require the internet. Specifically:

- The vault, packs, formulas, dice, projection, accounts, the editor, the
  browser, the tabletop and the session recorder all work with no network at
  all.
- The **only** optional network features are: archive import, the MCP
  endpoint, a cloud transcription engine, and an advisory "a new version is
  available" notice.
- **There is no automatic update.** The app may *notice* a newer release and
  say so; it must not install one. An update that changes the vault format
  under a running campaign is not a thing the app should do to someone.
- The app must start and work with the network interface down.

### A native client must not require a rewrite

The only requirement for mobile is the *possibility* of one, without a major
rewrite. That is a constraint on the design, stated once:

> **The server must not require a browser, and the browser client must hold no
> state the server does not know about.**

Concretely, that means:

- The API is the contract, and it is versioned and documented
  ([`api.md`](../api.md)).
- Authentication accepts a bearer token as well as a cookie, so a native
  client is not forced into browser-shaped auth.
- **The server computes** — formulas, dice and the shared log — so a client
  never has to reimplement the pack's expression languages.
- The pack's `views.sheet` and `content:` declarations are **declarative
  layouts**, so a client renders them rather than reimplementing them. A native
  client writes its own rendering code; it does not need the server to change.
- The WebSocket protocol is client-agnostic, and the client sends *intents*
  rather than state.
- Nothing server-side assumes a browser: no server-side rendering, no
  session state that only exists in a cookie jar, no client-only assumptions
  about ordering or timing.

**No native SDK is implied.** The contract is the enabler; an SDK is a
separate product decision.

## Consequences

**Good:**

- Remote play works without a VPN, and without the GM running a proxy for
  every group.
- **24/7 operation makes the between-sessions view real.** Players can read
  their sheet and their world's discoveries at any hour, because the server
  is up (ADR-0017). That is a genuine product improvement, not just a
  deployment change.
- The `--expose` flag makes the safe configuration the easy one.
- The vault's plain-file nature means the backup is trivial and
  app-independent — a real strength in a world where the server is on the
  internet.

**Bad:**

- **The GM becomes a sysadmin.** Certificate renewal, firewalling, patching,
  backups, secret rotation. That is a genuine cost, and the documentation has
  to be excellent or the tool will be deployed insecurely and blamed.
- **The blast radius of a compromise is the whole vault.** There is no
  partial exposure — a compromised server yields everything. The app
  mitigates what it can (projection, secrets, hashing) and the rest is the
  GM's.
- **The import endpoint's SSRF defence is real security code** that must be
  written well and tested adversarially. It is the least attractive code in
  the project and it is non-negotiable.
- **A 24/7 server is a machine that is never asleep**, with the disk churn,
  the updates and the power that implies. Running the vault from a laptop
  that sleeps is no longer a supported posture.
- Sessions accumulate. The session store (ADR-0030) needs retention and
  cleanup, or it grows without bound over months.
- **No automatic updates** means the GM must notice updates themselves. That
  is a real operational burden, and it is the price of not letting the app
  change a vault format under a running campaign.

**Neutral:**

- The projection, the roles, the account model and the vault format are all
  unchanged. This ADR changes what is *around* the core, not the core.
- The "local-first" claim survives: the app is local, and internet exposure
  is an option the GM takes.
