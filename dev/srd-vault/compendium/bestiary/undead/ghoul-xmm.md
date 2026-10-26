---
abilities:
  cha: 6
  con: 10
  dex: 15
  int: 7
  str: 13
  wis: 10
ac: 12
aliases:
  - Ghoul
alignment:
  - Chaotic Evil
attribution: Wizards of the Coast — D&D System Reference Document 5.2.1
condition_immunities:
  - charmed
  - exhaustion
  - poisoned
content_type: monster
cr: "1"
cr_xp: 200
cssclasses:
  - json5e-monster
data:
  abilities:
    cha: 6
    con: 10
    dex: 15
    int: 7
    str: 13
    wis: 10
  ac: 12
  alignment:
    - Chaotic Evil
  condition_immunities:
    - charmed
    - exhaustion
    - poisoned
  cr: "1"
  cr_xp: 200
  hp: 22
  immune:
    - poison
  languages:
    - Common
  passive_perception: 10
  resist: []
  senses: []
  size: Medium
  speed:
    walk: 30
  type: undead
  vulnerable: []
hp: 22
immune:
  - poison
languages:
  - Common
license: CC-BY-4.0
name: Ghoul
obsidianUIMode: preview
passive_perception: 10
resist: []
senses: []
size: Medium
source: SRD 5.2.1
speed:
  walk: 30
src: xmm
system: dnd5e
tags:
  - compendium/src/5e/xmm
  - monster/cr/1
  - monster/environment/swamp
  - monster/environment/underdark
  - monster/environment/urban
  - monster/size/medium
  - monster/type/undead
type: content
uid: CCD1D36247AA38669F763C83D8
visibility: party
vulnerable: []
---
# Ghoul
*Source: SRD 5.2 and the Free Rules (2024)*  

Ghouls rise from the bodies of cannibals and villains with depraved hungers. They form packs out of shared voracity.

## Ghouls

*Eaters of the Dead*

- **Habitat.** Swamp, Underdark, Urban  
- **Treasure.** Any  

Packs of ghouls haunt the rotten corners of the world, ravenously hunting for corpses and those soon to be corpses. These gaunt, animate cadavers with unnaturally long tongues dwell in catacombs and ruins where they devour the contents of graves and paralyze foes with vicious claws.

> [!quote]  
> 
> On a plain of teeth, in a temple of filth, the starving king wastes no morsel. Every coffin a banquet. Every slab a platter. Now is the time of feasting!


## Statblock

```ad-statblock
title: Ghoul
![](https://raw.githubusercontent.com/5etools-mirror-3/5etools-img/main/bestiary/tokens/XMM/Ghoul.webp#token)
*Medium undead, Chaotic Evil*

- **Armor Class** 12
- **Hit Points** 22 (`5d8`)
- **Speed** 30 ft.

|STR|DEX|CON|INT|WIS|CHA|
|:---:|:---:|:---:|:---:|:---:|:---:|
|13 (+1)|15 (+2)|10 (+0)| 7 (-2)|10 (+0)| 6 (-2)|

- **Proficiency Bonus** +2
- **Saving Throws** ⏤
- **Skills** ⏤
- **Senses** [Darkvision](rules/senses.md#Darkvision) 60 ft., passive Perception 10
- **Damage Immunities** poison
- **Condition Immunities** [charmed](rules/conditions.md#Charmed), [exhaustion](rules/conditions.md#Exhaustion), [poisoned](rules/conditions.md#Poisoned)
- **Treasure** Any
- **Languages** Common
- **Challenge** 1

## Actions

***Multiattack.*** The ghoul makes two Bite attacks.

***Bite.*** *Melee Attack Roll:* `+4`, reach 5 ft. *Hit:* 5 (`1d6 + 2`) Piercing damage plus 3 (`1d6`) Necrotic damage.

***Claw.*** *Melee Attack Roll:* `+4`, reach 5 ft. *Hit:* 4 (`1d4 + 2`) Slashing damage. If the target is a creature that isn't an Undead or elf, it is subjected to the following effect. *Constitution Saving Throw:* DC 10. *Failure:* The target has the [Paralyzed](rules/conditions.md#Paralyzed) condition until the end of its next turn.
```
^statblock

## Environment

swamp, underdark, urban