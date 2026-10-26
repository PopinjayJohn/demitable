---
abilities:
  cha: 8
  con: 15
  dex: 10
  int: 10
  str: 17
  wis: 11
ac: 15
aliases:
  - Wereboar
alignment:
  - Neutral Evil
attribution: Wizards of the Coast — D&D System Reference Document 5.2.1
condition_immunities: []
content_type: monster
cr: "4"
cr_xp: 1100
cssclasses:
  - json5e-monster
data:
  abilities:
    cha: 8
    con: 15
    dex: 10
    int: 10
    str: 17
    wis: 11
  ac: 15
  alignment:
    - Neutral Evil
  condition_immunities: []
  cr: "4"
  cr_xp: 1100
  hp: 97
  immune: []
  languages:
    - Common (can't speak in boar form)
  passive_perception: 12
  resist: []
  senses: []
  size: Small
  speed:
    alternate:
      walk:
        - condition: (boar form only)
          number: 40
    walk: 30
  type: monstrosity
  vulnerable: []
hp: 97
immune: []
languages:
  - Common (can't speak in boar form)
license: CC-BY-4.0
name: Wereboar
obsidianUIMode: preview
passive_perception: 12
resist: []
senses: []
size: Small
source: SRD 5.2.1
speed:
  alternate:
    walk:
      - condition: (boar form only)
        number: 40
  walk: 30
src: xmm
system: dnd5e
tags:
  - compendium/src/5e/xmm
  - monster/cr/4
  - monster/environment/forest
  - monster/environment/grassland
  - monster/environment/hill
  - monster/size/small-or-medium
  - monster/type/monstrosity/lycanthrope
type: content
uid: 64CFB29FEA4571B41355F6343E
visibility: party
vulnerable: []
---
# Wereboar
*Source: SRD 5.2 and the Free Rules (2024)*  

## Wereboar

*Changed by the Hunger of the Boar*

- **Habitat.** Forest, Grassland, Hill  
- **Treasure.** Individual  

Wereboars shape-shift from their humanoid forms into powerful boars or humanoid-boar hybrids. Many wereboars suffer their shape-shifting nature as a curse, with some involuntarily transforming any time they perform a greedy act or indulge their selfish nature.

```ad-statblock
title: Wereboar
![](https://raw.githubusercontent.com/5etools-mirror-3/5etools-img/main/bestiary/tokens/XMM/Wereboar.webp#token)
*Small or Medium monstrosity (lycanthrope), Neutral Evil*

- **Armor Class** 15
- **Hit Points** 97 (`15d8 + 30`)
- **Speed** 30 ft., 40 ft. (boar form only)

|STR|DEX|CON|INT|WIS|CHA|
|:---:|:---:|:---:|:---:|:---:|:---:|
|17 (+3)|10 (+0)|15 (+2)|10 (+0)|11 (+0)| 8 (-1)|

- **Proficiency Bonus** +2
- **Saving Throws** ⏤
- **Skills** [Perception](rules/skills.md#Perception) +2
- **Senses** passive Perception 12
- **Gear** six [javelins](compendium/items/javelin-xphb.md)
- **Treasure** Individual
- **Languages** Common (can't speak in boar form)
- **Challenge** 4

## Actions

***Multiattack.*** The wereboar makes two attacks, using Javelin or Tusk in any combination. It can replace one attack with a Gore attack.

***Gore (Boar or Hybrid Form Only).*** *Melee Attack Roll:* `+5`, reach 5 ft. *Hit:* 12 (`2d8 + 3`) Piercing damage. If the target is a Humanoid, it is subjected to the following effect. *Constitution Saving Throw:* DC 12. *Failure:* The target is cursed. If the cursed target drops to 0 [Hit Points](rules/variant-rules/hit-points-xphb.md), it instead becomes a Wereboar under the DM's control and has 10 [Hit Points](rules/variant-rules/hit-points-xphb.md). *Success:* The target is immune to this wereboar's curse for 24 hours.

***Javelin (Humanoid or Hybrid Form Only).*** *Melee  or Ranged Attack Roll:* `+5`, reach 5 ft. or range 30/120 ft. *Hit:* 13 (`3d6 + 3`) Piercing damage.

***Tusk (Boar or Hybrid Form Only).*** *Melee Attack Roll:* `+5`, reach 5 ft. *Hit:* 10 (`2d6 + 3`) Piercing damage. If the target is a Medium or smaller creature and the wereboar moved 20+ feet straight toward it immediately before the hit, the target takes an extra 7 (`2d6`) Piercing damage and has the [Prone](rules/conditions.md#Prone) condition.

## Bonus Actions

***Shape-Shift.*** The wereboar shape-shifts into a Medium boar-humanoid hybrid or a Small boar, or it returns to its true humanoid form. Its game statistics, other than its size, are the same in each form. Any equipment it is wearing or carrying isn't transformed.
```
^statblock

## Environment

forest, grassland, hill