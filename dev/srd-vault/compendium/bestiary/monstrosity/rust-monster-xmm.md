---
abilities:
  cha: 6
  con: 13
  dex: 12
  int: 2
  str: 13
  wis: 13
ac: 14
aliases:
  - Rust Monster
alignment:
  - U
attribution: Wizards of the Coast — D&D System Reference Document 5.2.1
condition_immunities: []
content_type: monster
cr: 1/2
cr_xp: 100
cssclasses:
  - json5e-monster
data:
  abilities:
    cha: 6
    con: 13
    dex: 12
    int: 2
    str: 13
    wis: 13
  ac: 14
  alignment:
    - U
  condition_immunities: []
  cr: 1/2
  cr_xp: 100
  hp: 33
  immune: []
  languages: []
  passive_perception: 11
  resist: []
  senses: []
  size: Medium
  speed:
    walk: 40
  type: monstrosity
  vulnerable: []
hp: 33
immune: []
languages: []
license: CC-BY-4.0
name: Rust Monster
obsidianUIMode: preview
passive_perception: 11
resist: []
senses: []
size: Medium
source: SRD 5.2.1
speed:
  walk: 40
src: xmm
system: dnd5e
tags:
  - compendium/src/5e/xmm
  - monster/cr/1-2
  - monster/environment/underdark
  - monster/size/medium
  - monster/type/monstrosity
type: content
uid: A408BD9F96BAC845C404CC1BCD
visibility: party
vulnerable: []
---
# Rust Monster
*Source: SRD 5.2 and the Free Rules (2024)*  

## Rust Monster

*Corrosive, Equipment-Eating Scavenger*

- **Habitat.** Underdark  
- **Treasure.** None  

Rust monsters roam the Underdark searching for ferrous metal. When they find this material—whether natural veins, subterranean structures, or creatures' equipment—these beetle-like scavengers rush to feed. Using their feathery antennae, rust monsters dissolve metals such as iron and steel into rusted scrap. They easily gnaw through this corroded metal using their mandibles. Rust monsters usually ignore creatures without metal equipment, but they defend themselves if attacked.

```ad-statblock
title: Rust Monster
![](https://raw.githubusercontent.com/5etools-mirror-3/5etools-img/main/bestiary/tokens/XMM/Rust%20Monster.webp#token)
*Medium monstrosity, Unaligned*

- **Armor Class** 14
- **Hit Points** 33 (`6d8 + 6`)
- **Speed** 40 ft.

|STR|DEX|CON|INT|WIS|CHA|
|:---:|:---:|:---:|:---:|:---:|:---:|
|13 (+1)|12 (+1)|13 (+1)| 2 (-4)|13 (+1)| 6 (-2)|

- **Proficiency Bonus** +2
- **Saving Throws** ⏤
- **Skills** ⏤
- **Senses** [Darkvision](rules/senses.md#Darkvision) 60 ft., passive Perception 11
- **Languages** —
- **Challenge** 1/2

## Traits

***Iron Scent.*** The rust monster can pinpoint the location of ferrous metal within 30 feet of itself.

## Actions

***Multiattack.*** The rust monster makes one Bite attack and uses Antennae twice.

***Bite.*** *Melee Attack Roll:* `+3`, reach 5 ft. *Hit:* 5 (`1d8 + 1`) Piercing damage.

***Antennae.*** The rust monster targets one nonmagical metal object—armor or a weapon—worn or carried by a creature within 5 feet of itself. *Dexterity Saving Throw:* DC 11, the creature with the object. *Failure:* The object takes a -1 penalty to the AC it offers (armor) or to its attack rolls (weapon). Armor is destroyed if the penalty reduces its AC to 10, and a weapon is destroyed if its penalty reaches -5. The penalty can be removed by casting the [Mending](compendium/spells/mending-xphb.md) spell on the armor or weapon.

***Destroy Metal.*** The rust monster touches a nonmagical metal object within 5 feet of itself that isn't being worn or carried. The touch destroys a 1-foot [Cube](rules/variant-rules/cube-area-of-effect-xphb.md) of the object.

## Reactions

***Reflexive Antennae.*** Trigger: An attack roll hits the rust monster. _Response:_ The rust monster uses Antennae.
```
^statblock

## Environment

underdark