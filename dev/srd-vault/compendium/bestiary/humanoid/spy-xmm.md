---
aliases:
  - Spy
attribution: Wizards of the Coast — D&D System Reference Document 5.2.1
content_type: monster
cssclasses:
  - json5e-monster
data:
  abilities:
    cha: 16
    con: 10
    dex: 15
    int: 12
    str: 10
    wis: 14
  ac: 12
  alignment:
    - "N"
  condition_immunities: []
  cr: "1"
  cr_xp: 200
  hp: 27
  immune: []
  languages:
    - Common plus one other language
  passive_perception: 16
  resist: []
  senses: []
  size: Small
  speed:
    climb: 30
    walk: 30
  type: humanoid
  vulnerable: []
license: CC-BY-4.0
name: Spy
obsidianUIMode: preview
source: SRD 5.2.1
src: xmm
system: dnd5e
tags:
  - compendium/src/5e/xmm
  - monster/cr/1
  - monster/environment/any
  - monster/size/small-or-medium
  - monster/type/humanoid
type: content
uid: AE0B9D369AAED923254CB4497E
visibility: party
---
# Spy
*Source: SRD 5.2 and the Free Rules (2024)*  

Spies use charm and deception to veil their true intentions. If forced into combat, they seek to end such conflicts quietly and decisively.

## Spies

*Infiltrators and Informants*

- **Habitat.** Any  
- **Treasure.** Implements, Individual  

Spies gather information and disseminate lies, manipulating people to gain the results the spies' patrons desire. They're trained to manipulate, infiltrate, and—when necessary—escape in a hurry. Many adopt disguises, aliases, or code names to maintain anonymity. Roll on or choose a result from the Spy Personas table to inspire a spy's disguise.

**Spy Personas**

| dice: 1d4 | The Spy Disguises Themself As... |
|-----------|----------------------------------|
| 1 | A bard or traveling performer. |
| 2 | A captive or servant of a monster or villain. |
| 3 | A dignitary or traveler from a distant land. |
| 4 | A visitor from a different time or world. |
^spy-personas

## Statblock

```ad-statblock
title: Spy
![](https://raw.githubusercontent.com/5etools-mirror-3/5etools-img/main/bestiary/tokens/XMM/Spy.webp#token)
*Small or Medium humanoid, Neutral*

- **Armor Class** 12
- **Hit Points** 27 (`6d8`)
- **Speed** 30 ft., climb 30 ft.

|STR|DEX|CON|INT|WIS|CHA|
|:---:|:---:|:---:|:---:|:---:|:---:|
|10 (+0)|15 (+2)|10 (+0)|12 (+1)|14 (+2)|16 (+3)|

- **Proficiency Bonus** +2
- **Saving Throws** ⏤
- **Skills** [Deception](rules/skills.md#Deception) +5, [Insight](rules/skills.md#Insight) +4, [Investigation](rules/skills.md#Investigation) +5, [Perception](rules/skills.md#Perception) +6, [Sleight of Hand](rules/skills.md#Sleight%20of%20Hand) +4, [Stealth](rules/skills.md#Stealth) +6
- **Senses** passive Perception 16
- **Gear** [hand crossbow](compendium/items/hand-crossbow-xphb.md), [shortsword](compendium/items/shortsword-xphb.md), [thieves' tools](compendium/items/thieves-tools-xphb.md)
- **Treasure** Implements, Individual
- **Languages** Common plus one other language
- **Challenge** 1

## Actions

***Shortsword.*** *Melee Attack Roll:* `+4`, reach 5 ft. *Hit:* 5 (`1d6 + 2`) Piercing damage plus 7 (`2d6`) Poison damage.

***Hand Crossbow.*** *Ranged Attack Roll:* `+4`, range 30/120 ft. *Hit:* 5 (`1d6 + 2`) Piercing damage plus 7 (`2d6`) Poison damage.

## Bonus Actions

***Cunning Action.*** The spy takes the [Dash](rules/actions.md#Dash), [Disengage](rules/actions.md#Disengage), or [Hide](rules/actions.md#Hide) action.
```
^statblock

## Environment

any