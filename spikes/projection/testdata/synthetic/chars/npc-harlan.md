---
type: character
uid: uid-harlan
name: Harlan
kind: npc
campaign: "[[Stormdark]]"
visibility: party
system: example
data:
  resources:
    hp:
      value: 27
    true-name:
      value: Sister Marrow
  allegiance: saltmarsh
---

# Harlan

A GM-created NPC: no `player:`. The pack declares `resources.hp` as
`visibility: owner`, so it must fall back to the note's visibility
(ADR-0033 rule 7) — otherwise no player can read an NPC's hit points.
