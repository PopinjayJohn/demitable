---
aliases:
  - Ghast
attribution: Wizards of the Coast — D&D System Reference Document 5.2.1
content_type: monster
cssclasses:
  - json5e-monster
data:
  abilities:
    cha: 8
    con: 10
    dex: 17
    int: 11
    str: 16
    wis: 10
  ac: 13
  alignment:
    - Chaotic Evil
  condition_immunities:
    - charmed
    - exhaustion
    - poisoned
  cr: "2"
  cr_xp: 450
  hp: 36
  immune:
    - poison
  languages:
    - Common
  passive_perception: 10
  resist:
    - necrotic
  senses: []
  size: Medium
  speed:
    walk: 30
  type: undead
  vulnerable: []
license: CC-BY-4.0
name: Ghast
obsidianUIMode: preview
source: SRD 5.2.1
src: xmm
system: dnd5e
tags:
  - compendium/src/5e/xmm
  - monster/cr/2
  - monster/environment/swamp
  - monster/environment/underdark
  - monster/environment/urban
  - monster/size/medium
  - monster/type/undead
type: content
uid: F24A22C1D1E1E26E30EFCC3FF5
visibility: party
---
# Ghast
*Source: SRD 5.2 and the Free Rules (2024)*  

Ghasts frequently organize ghouls into packs to despoil crypts and steal the wealth within.

## Ghasts

*Tyrants among Corpses*

- **Habitat.** Swamp, Underdark, Urban  
- **Treasure.** Any  

Ghasts are reeking, undying corpses closely related to ghouls. They hunger for the vices they enjoyed in life as much as they do for rotting flesh.

## Statblock

```ad-statblock
title: Ghast
![](https://raw.githubusercontent.com/5etools-mirror-3/5etools-img/main/bestiary/tokens/XMM/Ghast.webp#token)
*Medium undead, Chaotic Evil*

- **Armor Class** 13
- **Hit Points** 36 (`8d8`)
- **Speed** 30 ft.

|STR|DEX|CON|INT|WIS|CHA|
|:---:|:---:|:---:|:---:|:---:|:---:|
|16 (+3)|17 (+3)|10 (+0)|11 (+0)|10 (+0)| 8 (-1)|

- **Proficiency Bonus** +2
- **Saving Throws** Wisdom +2
- **Skills** ⏤
- **Senses** [Darkvision](rules/senses.md#Darkvision) 60 ft., passive Perception 10
- **Damage Resistances** necrotic
- **Damage Immunities** poison
- **Condition Immunities** [charmed](rules/conditions.md#Charmed), [exhaustion](rules/conditions.md#Exhaustion), [poisoned](rules/conditions.md#Poisoned)
- **Treasure** Any
- **Languages** Common
- **Challenge** 2

## Traits

***Stench.*** *Constitution Saving Throw:* DC 10, any creature that starts its turn in a 5-foot [Emanation](rules/variant-rules/emanation-area-of-effect-xphb.md) originating from the ghast. *Failure:* The target has the [Poisoned](rules/conditions.md#Poisoned) condition until the start of its next turn. *Success:* The target is immune to this ghast's Stench for 24 hours.

## Actions

***Bite.*** *Melee Attack Roll:* `+5`, reach 5 ft. *Hit:* 7 (`1d8 + 3`) Piercing damage plus 9 (`2d8`) Necrotic damage.

***Claw.*** *Melee Attack Roll:* `+5`, reach 5 ft. *Hit:* 10 (`2d6 + 3`) Slashing damage. If the target is a non-Undead creature, it is subjected to the following effect. *Constitution Saving Throw:* DC 10. *Failure:* The target has the [Paralyzed](rules/conditions.md#Paralyzed) condition until the end of its next turn.
```
^statblock

## Environment

swamp, underdark, urban