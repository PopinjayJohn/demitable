# ADR-0010: LAN-first networking, designed to survive a VPN without changes

- **Status:** Accepted, **superseded by ADR-0031** — the server may be
  internet-exposed, behind TLS termination the GM provides. The reasoning
  below about deriving URLs from the request and shelled-out `git` survives;
  the "LAN only, no TLS in v1" decision does not.
- **Date:** 2026-10-21

## Context

Players reach the server over the network. The options are: LAN only, LAN plus
VPN, or internet-facing.

Internet-facing implies TLS certificates, an account system that survives
outside the GM's house, credential rotation, and a hosting model — a very
different product with a very different privacy story. For a tool whose
session recordings contain unrehearsed player conversation, that is a weight
worth not carrying.

But many groups do not play in one room. A remote table is a real use case,
and "install a VPN" is a legitimate answer for it.

## Decision

**The server binds to a configurable address. LAN is the default. It is
designed so that a VPN overlay works with no changes.**

- Default bind: the LAN interface, so players on the same network can reach
  it. The GM can override with `--host`.
- The server prints the URLs players should use, including the LAN address.
- Nothing in the code assumes `localhost` or `127.0.0.1`. No absolute URL is
  ever baked into a response; the server derives the externally visible origin
  from the request, so a request arriving over a Tailscale interface returns
  Tailscale URLs.
- Cookies are host-only and SameSite=Strict, so the same server serves
  `192.168.1.x` and a `100.x.y.z` Tailscale address without cross-origin
  problems.

**No TLS in v1.** The traffic is plaintext HTTP on a trusted network. This is a
deliberate, stated limitation, not an oversight. The mitigation is that the
deployment guidance says: *for remote play, use Tailscale or WireGuard, which
gives you encrypted transport and a private network.*

**Player access to a session is by join code**, so an unauthorised person who
finds the LAN address still cannot join a table without a code the GM is
showing on screen.

### What this rules out, explicitly

- No public hosting mode.
- No "invite a player by email" flow.
- No third-party access without the GM's own network being extended.

If a future version wants internet play, that is a new ADR, and the natural
approach is to leave the server untouched and put a reverse proxy with TLS in
front of it.

## Consequences

**Good:**

- No certificate management, no credential hosting, no hosting costs, and no
  privacy story that requires a legal review.
- Remote play is available to anyone willing to run Tailscale, which is a
  zero-configuration path for most technical players and a dead end for some
  others. That is a real limitation, stated plainly.
- The threat model is small and honest: the network the GM is on, plus one
  join code.

**Bad:**

- Players without a VPN cannot join a remote table.
- The join code is the only barrier between a curious houseguest and the
  table. It must be short-lived and regenerated per session.
- Plaintext traffic means a session's dice rolls, chat and (in phase 4)
  audio-adjacent metadata are visible to anyone on the wire. On a shared
  office Wi-Fi, that is a real consideration.

**Neutral:**

- The choice is reversible. Binding to an interface and deriving URLs from
  requests means a future TLS/reverse-proxy mode is additive, not a rewrite.
