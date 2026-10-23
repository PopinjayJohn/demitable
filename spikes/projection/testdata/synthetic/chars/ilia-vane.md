---
type: character
uid: uid-ilia
name: Ilia Vane
kind: pc
player: p_ilia
workflow: active
campaign: "[[Stormdark]]"
visibility: party
system: example
discovered:
  - "uid-undead-tomb"
  - "uid-lost-relic"
  - "uid-discovered-location"
data:
  resources:
    hp:
      value: 23
  attributes:
    str:
      value: 10
      display:
        party: "??"
---

# Ilia Vane

An active PC. Her sheet is party-visible; her hit points are hers; her
strength is masked from the party but not from her; and her `discovered`
list — a per-character private record — is a bare top-level field on a
party-visible note.
