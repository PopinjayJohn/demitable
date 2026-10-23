package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/PopinjayJohn/demitable/spikes/measurement/corpus"
)

// The operator set, observed.
//
// The method: write down what a GM actually asks for, in their own words,
// and express each as the smallest predicate that answers it. The operators
// that appear in those predicates are the operators the language needs. No
// operator is included because it seems useful.
//
// The corpus cannot answer this question on its own. Its frontmatter has
// four keys — obsidianUIMode, cssclasses, aliases, tags — all Obsidian-
// native, none of them a universal field. So the corpus can exercise a
// list-membership operator over tags and aliases, and nothing else. The
// rest is observed from the utterances below, against the reserved field
// set of ADR-0040 and the content declarations of ADR-0033.

// utterance is one thing a GM asks for.
type utterance struct {
	// Say is the GM's own words.
	Say string
	// Predicate is the smallest expression that answers it.
	Predicate string
	// Ops are the operators that expression needs.
	Ops []string
	// Field is the path it reads.
	Field string
	// CorpusCanExercise reports whether the SRD corpus contains data of
	// this shape, so the operator can be tested against real data.
	CorpusCanExercise bool
}

var utterances = []utterance{
	{"all the characters in this campaign", `type == "character" and campaign == "stormdark"`,
		[]string{"==", "and"}, "type, campaign", true},
	{"the characters I own", `player == "p_8f2a" and type == "character"`,
		[]string{"==", "and"}, "player, type", true},
	{"notes that are still in draft", `workflow == "draft"`,
		[]string{"=="}, "workflow", true},
	{"the GM-only notes", `visibility == "gm"`,
		[]string{"=="}, "visibility", true},
	{"notes tagged npc", `tags contains "npc"`,
		[]string{"contains"}, "tags", true},
	{"anything that mentions Ilia", `name contains "Ilia"`,
		[]string{"contains"}, "name", false},
	{"the notes that mention a given player", `grants contains "p_8f2a"`,
		[]string{"contains"}, "grants", false},
	{"spells of level 3 or higher", `data.attributes.level >= 3`,
		[]string{">="}, "data.attributes.level", false},
	{"spells of level exactly 3", `data.attributes.level == 3`,
		[]string{"=="}, "data.attributes.level", false},
	{"spells below level 3", `data.attributes.level < 3`,
		[]string{"<"}, "data.attributes.level", false},
	{"spells at most level 3", `data.attributes.level <= 3`,
		[]string{"<="}, "data.attributes.level", false},
	{"monsters with armour class over 15", `data.attributes.armor_class > 15`,
		[]string{">"}, "data.attributes.armor_class", false},
	{"notes that are not characters", `type != "character"`,
		[]string{"!="}, "type", true},
	{"characters and NPCs", `type == "character" or type == "npc"`,
		[]string{"==", "or"}, "type", true},
	{"characters, or NPCs that are tagged boss", `type == "character" or (type == "npc" and tags contains "boss")`,
		[]string{"==", "or", "and", "contains", "()"}, "type, tags", false},
	{"notes of one of several types", `type in ["character", "npc", "faction"]`,
		[]string{"in", "=="}, "type", true},
	{"notes that are not yet revealed", `not (revealed contains "s_42")`,
		[]string{"not", "contains", "()"}, "revealed", false},
	{"the note with this uid", `uid == "01HQ8R..."`,
		[]string{"=="}, "uid", true},
	{"notes of this kind, but not that one", `type == "character" and name != "Ilia Vane"`,
		[]string{"==", "and", "!="}, "type, name", false},
	{"notes whose tag list has the whole tag monster/cr/1-4", `tags contains "monster/cr/1-4"`,
		[]string{"contains"}, "tags", true},
	{"notes whose preview mode mentions prev", `obsidianUIMode contains "prev"`,
		[]string{"contains"}, "obsidianUIMode", true},
}

// adr0047 is the operator set ADR-0047 declares.
var adr0047 = []string{"==", "!=", ">", ">=", "<", "<=", "contains", "in", "and", "or", "not", "()"}

func cmdOps(args []string) {
	fs := newFlagSet("ops")
	fs.Parse(args)

	used := map[string]int{}
	corpusExercised := map[string]int{}
	for _, u := range utterances {
		for _, op := range u.Ops {
			used[op]++
			if u.CorpusCanExercise {
				corpusExercised[op]++
			}
		}
	}
	fmt.Println("=== the utterances, and the smallest predicate that answers each ===")
	for _, u := range utterances {
		can := "synthetic only"
		if u.CorpusCanExercise {
			can = "corpus can exercise"
		}
		fmt.Printf("  %-46s %-52s %-10s %s\n", u.Say, u.Predicate, strings.Join(u.Ops, " "), can)
	}
	fmt.Println()
	fmt.Println("=== operator tally ===")
	ops := make([]string, 0, len(used))
	for op := range used {
		ops = append(ops, op)
	}
	sort.Strings(ops)
	for _, op := range ops {
		inADR := false
		for _, a := range adr0047 {
			if a == op {
				inADR = true
			}
		}
		mark := "not in ADR-0047"
		if inADR {
			mark = "in ADR-0047"
		}
		fmt.Printf("  %-10s used %2d times  (%d exercisable against the corpus)  %s\n",
			op, used[op], corpusExercised[op], mark)
	}
	fmt.Println()
	unused := []string{}
	for _, a := range adr0047 {
		if used[a] == 0 {
			unused = append(unused, a)
		}
	}
	fmt.Printf("operators ADR-0047 declares that no utterance needed: %s\n", strings.Join(unused, ", "))
	missing := []string{}
	for op := range used {
		found := false
		for _, a := range adr0047 {
			if a == op {
				found = true
			}
		}
		if !found {
			missing = append(missing, op)
		}
	}
	sort.Strings(missing)
	fmt.Printf("operators an utterance needed that ADR-0047 does not declare: %s\n", strings.Join(missing, ", "))

	fmt.Println()
	fmt.Println("=== what the corpus's own frontmatter can exercise ===")
	notes, err := corpus.Load(corpus.Options{Root: corpusRoot})
	if err != nil {
		fatal(err)
	}
	fields := map[string]int{}
	kinds := map[string]map[string]int{}
	for _, n := range notes {
		for _, k := range n.FMKeys {
			fields[k]++
			if kinds[k] == nil {
				kinds[k] = map[string]int{}
			}
			switch k {
			case "aliases", "tags", "cssclasses":
				kinds[k]["list"]++
			default:
				kinds[k]["scalar"]++
			}
		}
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %-16s %4d notes  %v\n", k, fields[k], kinds[k])
	}
	fmt.Println("  -> none of these is a universal field, so `type`, `campaign`, `visibility`,")
	fmt.Println("     `player`, `workflow`, `uid` and `data.*` are unexercisable by the corpus.")
	fmt.Println()
	fmt.Println("=== the corpus pins down `contains` for two different field shapes ===")
	fmt.Println("  tags          is a YAML list of strings: contains is LIST MEMBERSHIP.")
	fmt.Println("                The values are hierarchical (monster/cr/1-4), so under a")
	fmt.Println("                substring reading `contains \"monster/cr\"` would be a hit")
	fmt.Println("                and `contains \"monster/cr/1-4\"` a different one.")
	fmt.Println("  obsidianUIMode is a scalar string: contains is SUBSTRING.")
	fmt.Println("  One operator, two semantics, decided by the field's type — which is")
	fmt.Println("  what ADR-0040's type registry is for.")

	os.Exit(0)
}

// newFlagSet is a small helper so the ops command can parse its own flags.
func newFlagSet(name string) *flag.FlagSet {
	return flag.NewFlagSet(name, flag.ExitOnError)
}
