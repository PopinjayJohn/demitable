---
abilities:
  cha: 3
  con: 13
  dex: 13
  int: 2
  str: 15
  wis: 10
ac: 11
aliases:
  - Giant Toad
alignment:
  - U
attribution: Wizards of the Coast — D&D System Reference Document 5.2.1
condition_immunities: []
content_type: monster
cr: "1"
cr_xp: 200
cssclasses:
  - json5e-monster
data:
  abilities:
    cha: 3
    con: 13
    dex: 13
    int: 2
    str: 15
    wis: 10
  ac: 11
  alignment:
    - U
  condition_immunities: []
  cr: "1"
  cr_xp: 200
  hp: 39
  immune: []
  languages: []
  passive_perception: 10
  resist: []
  senses: []
  size: Large
  speed:
    swim: 30
    walk: 30
  type: beast
  vulnerable: []
hp: 39
immune: []
languages: []
license: CC-BY-4.0
name: Giant Toad
obsidianUIMode: preview
passive_perception: 10
resist: []
senses: []
size: Large
source: SRD 5.2.1
speed:
  swim: 30
  walk: 30
src: xmm
system: dnd5e
tags:
  - compendium/src/5e/xmm
  - monster/cr/1
  - monster/environment/coastal
  - monster/environment/forest
  - monster/environment/swamp
  - monster/environment/underdark
  - monster/size/large
  - monster/type/beast
type: content
uid: 00FC029A9D7BEF2D6834450E1B
visibility: party
vulnerable: []
---
# Giant Toad
*Source: SRD 5.2 and the Free Rules (2024)*  

## Animals

Use these stat blocks to represent the creatures they're named for or other similar creatures. For example, the [Panther](compendium/bestiary/beast/panther-xmm.md) stat block can also represent a mountain lion, while the [Giant Goat](compendium/bestiary/beast/giant-goat-xmm.md) stat block might represent a buffalo. Any of these stat blocks might also serve as fantastical animals with distinctive names and cosmetic details unique to your D&D adventures.

```ad-statblock
title: Giant Toad
![](https://raw.githubusercontent.com/5etools-mirror-3/5etools-img/main/bestiary/tokens/XMM/Giant%20Toad.webp#token)
*Large beast, Unaligned*

- **Armor Class** 11
- **Hit Points** 39 (`6d10 + 6`)
- **Speed** 30 ft., swim 30 ft.

|STR|DEX|CON|INT|WIS|CHA|
|:---:|:---:|:---:|:---:|:---:|:---:|
|15 (+2)|13 (+1)|13 (+1)| 2 (-4)|10 (+0)| 3 (-4)|

- **Proficiency Bonus** +2
- **Saving Throws** ⏤
- **Skills** ⏤
- **Senses** [Darkvision](rules/senses.md#Darkvision) 60 ft., passive Perception 10
- **Languages** —
- **Challenge** 1

## Traits

***Amphibious.*** The toad can breathe air and water.

***Standing Leap.*** The toad's [Long Jump](rules/variant-rules/long-jump-xphb.md) is up to 20 feet and its [High Jump](rules/variant-rules/high-jump-xphb.md) is up to 10 feet with or without a running start.

## Actions

***Bite.*** *Melee Attack Roll:* `+4`, reach 5 ft. *Hit:* 5 (`1d6 + 2`) Piercing damage plus 5 (`2d4`) Poison damage. If the target is a Medium or smaller creature, it has the [Grappled](rules/conditions.md#Grappled) condition (escape DC 12).

***Swallow.*** The toad swallows a Medium or smaller target it is grappling. While swallowed, the target isn't [Grappled](rules/conditions.md#Grappled) but has the [Blinded](rules/conditions.md#Blinded) and [Restrained](rules/conditions.md#Restrained) conditions, and it has [Total Cover](rules/variant-rules/cover-xphb.md) against attacks and other effects outside the toad. In addition, the target takes 10 (`3d6`) Acid damage at the end of each of the toad's turns. The toad can have only one target swallowed at a time, and it can't use Bite while it has a swallowed target. If the toad dies, a swallowed creature is no longer [Restrained](rules/conditions.md#Restrained) and can escape from the corpse using 5 feet of movement, exiting with the [Prone](rules/conditions.md#Prone) condition.
```
^statblock

## Environment

coastal, forest, swamp, underdark