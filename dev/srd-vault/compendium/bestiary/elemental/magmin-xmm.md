---
abilities:
  cha: 10
  con: 12
  dex: 15
  int: 8
  str: 7
  wis: 11
ac: 14
aliases:
  - Magmin
alignment:
  - Chaotic Neutral
attribution: Wizards of the Coast — D&D System Reference Document 5.2.1
condition_immunities: []
content_type: monster
cr: 1/2
cr_xp: 100
cssclasses:
  - json5e-monster
data:
  abilities:
    cha: 10
    con: 12
    dex: 15
    int: 8
    str: 7
    wis: 11
  ac: 14
  alignment:
    - Chaotic Neutral
  condition_immunities: []
  cr: 1/2
  cr_xp: 100
  hp: 13
  immune:
    - fire
  languages:
    - Primordial (Ignan)
  passive_perception: 10
  resist: []
  senses: []
  size: Small
  speed:
    walk: 30
  type: elemental
  vulnerable: []
hp: 13
immune:
  - fire
languages:
  - Primordial (Ignan)
license: CC-BY-4.0
name: Magmin
obsidianUIMode: preview
passive_perception: 10
resist: []
senses: []
size: Small
source: SRD 5.2.1
speed:
  walk: 30
src: xmm
system: dnd5e
tags:
  - compendium/src/5e/xmm
  - monster/cr/1-2
  - monster/environment/fire
  - monster/environment/planar
  - monster/size/small
  - monster/type/elemental
type: content
uid: CE92C50F13B51034A0744E7C1A
visibility: party
vulnerable: []
---
# Magmin
*Source: SRD 5.2 and the Free Rules (2024)*  

## Magmin

*Reckless Elemental Arsonist*

- **Habitat.** Planar (Elemental Plane of Fire)  
- **Treasure.** None  

Magmins divide all things into two categories: things that are on fire and things that should be on fire. With bodies of flame and magmatic rock, these halfling-size creatures delight in setting fires. They do so not out of malice but out of enthusiasm for primal fire. They don't consider that objects have value beyond kindling or that creatures can be harmed by flames. If such concepts are explained to them, they find the ideas difficult to grasp and don't remember them for long. Rather, they relish every opportunity to set flammable things alight, delighting in igniting paper, wooden structures, and explosives. Magmins are dangerous even in death, since they explode when they're destroyed, their flames igniting combustible materials nearby.

Magmins might be conjured by magic-users to harry foes or might escape the Elemental Plane of Fire through portals or rifts that lead to other realms. They're attracted to places of intense heat, such as volcanoes and rivers of magma. If they can't find such favored conditions, magmins eagerly burn structures or start wildfires to entertain themselves.

```ad-statblock
title: Magmin
![](https://raw.githubusercontent.com/5etools-mirror-3/5etools-img/main/bestiary/tokens/XMM/Magmin.webp#token)
*Small elemental, Chaotic Neutral*

- **Armor Class** 14
- **Hit Points** 13 (`3d6 + 3`)
- **Speed** 30 ft.

|STR|DEX|CON|INT|WIS|CHA|
|:---:|:---:|:---:|:---:|:---:|:---:|
| 7 (-2)|15 (+2)|12 (+1)| 8 (-1)|11 (+0)|10 (+0)|

- **Proficiency Bonus** +2
- **Saving Throws** ⏤
- **Skills** ⏤
- **Senses** [Darkvision](rules/senses.md#Darkvision) 60 ft., passive Perception 10
- **Damage Immunities** fire
- **Languages** Primordial (Ignan)
- **Challenge** 1/2

## Traits

***Death Burst.*** The magmin explodes when it dies. *Dexterity Saving Throw:* DC 11, each creature in a 10-foot [Emanation](rules/variant-rules/emanation-area-of-effect-xphb.md) originating from the magmin. *Failure:* 7 (`2d6`) Fire damage. *Success:* Half damage.

## Actions

***Touch.*** *Melee Attack Roll:* `+4`, reach 5 ft. *Hit:* 7 (`2d4 + 2`) Fire damage. If the target is a creature or a flammable object that isn't being worn or carried, it starts [burning](compendium/traps-hazards/burning-xphb.md).

## Bonus Actions

***Ignited Illumination.*** The magmin sets itself ablaze or extinguishes its flames. While ablaze, the magmin sheds [Bright Light](rules/variant-rules/bright-light-xphb.md) in a 10-foot radius and [Dim Light](rules/variant-rules/dim-light-xphb.md) for an additional 10 feet.
```
^statblock

## Environment

planar, fire