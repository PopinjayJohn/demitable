---
aliases:
  - Giant Insect
attribution: Wizards of the Coast — D&D System Reference Document 5.2.1
content_type: monster
cssclasses:
  - json5e-monster
data:
  abilities:
    cha: 3
    con: 15
    dex: 13
    int: 4
    str: 17
    wis: 14
  alignment:
    - U
  condition_immunities: []
  immune: []
  languages:
    - understands the languages you know
  passive_perception: 12
  resist: []
  senses: []
  size: Large
  speed:
    climb: 40
    fly:
      condition: (Wasp only)
      number: 40
    walk: 40
  type: beast
  vulnerable: []
license: CC-BY-4.0
name: Giant Insect
obsidianUIMode: preview
source: SRD 5.2.1
src: xphb
system: dnd5e
tags:
  - compendium/src/5e/xphb
  - monster/cr/
  - monster/size/large
  - monster/type/beast
type: content
uid: 100B17E75B13C9FD24386FF73C
visibility: party
---
# Giant Insect
*Source: SRD 5.2*  

```ad-statblock
title: Giant Insect
![](https://raw.githubusercontent.com/5etools-mirror-3/5etools-img/main/bestiary/tokens/XPHB/Giant%20Insect.webp#token)
*Large beast, Unaligned*


- **Speed** 40 ft., climb 40 ft., fly 40 ft. (Wasp only)

|STR|DEX|CON|INT|WIS|CHA|
|:---:|:---:|:---:|:---:|:---:|:---:|
|17 (+3)|13 (+1)|15 (+2)| 4 (-3)|14 (+2)| 3 (-4)|

- **Proficiency Bonus** +2
- **Saving Throws** ⏤
- **Skills** ⏤
- **Senses** [Darkvision](rules/senses.md#Darkvision) 60 ft., passive Perception 12
- **Languages** understands the languages you know
- **Challenge** 

## Traits

***Spider Climb.*** The insect can climb difficult surfaces, including along ceilings, without needing to make an ability check.

## Actions

***Multiattack.*** The insect makes a number of attacks equal to half this spell's level (round down).

***Poison Jab.*** *Melee Attack Roll:* Bonus equals your spell attack modifier, reach 10 ft. *Hit:* `1d6 + 3` + the spell's level Piercing damage plus `1d4` Poison damage.

***Web Bolt (Spider Only).*** *Ranged Attack Roll:* Bonus equals your spell attack modifier, range 60 ft. *Hit:* `1d10 + 3` + the spell's level Bludgeoning damage, and the target's Speed is reduced to 0 until the start of the insect's next turn.

## Bonus Actions

***Venomous Spew (Centipede Only).*** *Constitution Saving Throw:* Your spell save DC, one creature the insect can see within 10 feet. *Failure:* The target has the [Poisoned](rules/conditions.md#Poisoned) condition until the start of the insect's next turn.
```
^statblock