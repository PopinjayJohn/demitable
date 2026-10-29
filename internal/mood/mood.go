// Package mood derives a mood tag from session/encounter state and
// transcript keywords. It is a PURE FUNCTION: no I/O, no global state,
// deterministic. The vocabulary is fixed (ADR-0034 non-audio equivalent:
// the mood is always shown as text).
package mood

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Vocabulary is the fixed, closed set of mood tags. Unknown input maps to
// DefaultMood (calm), never to nothing.
const (
	MoodTense      = "tense"
	MoodTriumphant = "triumphant"
	MoodSombre     = "sombre"
	MoodCalm       = "calm"
	DefaultMood    = MoodCalm
)

// Vocabulary lists every tag the function may return.
var Vocabulary = []string{MoodTense, MoodTriumphant, MoodSombre, MoodCalm}

// Mood is a mood tag (one of Vocabulary).
type Mood string

// EncounterState is the read-only input shape. Fields are optional; the
// function normalises aliases (hp / hp_fractions / side_hp, conditions /
// statuses / effects) so callers can use whichever spelling they prefer.
// EncounterState is also tolerant of map[string]any input.
type EncounterState struct {
	Active       bool               `json:"active"`
	IsActive     bool               `json:"is_active"`
	InCombat     bool               `json:"in_combat"`
	Combat       bool               `json:"combat"`
	Round        int                `json:"round"`
	CurrentRound int                `json:"current_round"`
	HP           map[string]float64 `json:"hp"`
	HPFractions  map[string]float64 `json:"hp_fractions"`
	SideHP       map[string]float64 `json:"side_hp"`
	Health       map[string]float64 `json:"health"`
	Fractions    map[string]float64 `json:"fractions"`
	Conditions   []string           `json:"conditions"`
	Statuses     []string           `json:"statuses"`
	Effects      []string           `json:"effects"`
}

// norm is the internal normalised encounter.
type norm struct {
	active     bool
	round      int
	hp         map[string]float64
	conditions []string
}

func toBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "true" || s == "yes" || s == "1" || s == "active" || s == "in_combat"
	case float64:
		return t != 0
	case int:
		return t != 0
	case int64:
		return t != 0
	}
	return false
}

func toFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case string:
		// best effort; ignore errors -> 0
		var f float64
		fmt.Sscan(strings.TrimSpace(t), &f)
		return f
	}
	return 0
}

func toStrings(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		return []string{t}
	case []string:
		out := make([]string, len(t))
		copy(out, t)
		return out
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			out = append(out, fmt.Sprint(e))
		}
		return out
	}
	return []string{fmt.Sprint(v)}
}

func mergeHP(dst map[string]float64, src map[string]float64) {
	for k, v := range src {
		dst[strings.ToLower(strings.TrimSpace(k))] = v
	}
}

func mergeHPAny(dst map[string]float64, v any) {
	switch t := v.(type) {
	case map[string]float64:
		mergeHP(dst, t)
	case map[string]any:
		for k, val := range t {
			dst[strings.ToLower(strings.TrimSpace(k))] = toFloat(val)
		}
	}
}

func mergeConditions(dst *[]string, v any) {
	for _, s := range toStrings(v) {
		*dst = append(*dst, s)
	}
}

func getMapKey(m map[string]any, keys ...string) (any, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v, true
		}
		// case-insensitive fallback
		lk := strings.ToLower(k)
		for mk, mv := range m {
			if strings.ToLower(mk) == lk {
				return mv, true
			}
		}
	}
	return nil, false
}

func normalizeEncounter(v any) norm {
	n := norm{hp: map[string]float64{}}
	if v == nil {
		return n
	}
	// pointer unwrap
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return n
		}
		rv = rv.Elem()
		v = rv.Interface()
	}
	switch t := v.(type) {
	case norm:
		return t
	case *norm:
		if t != nil {
			return *t
		}
		return n
	case EncounterState:
		n.active = t.Active || t.IsActive || t.InCombat || t.Combat
		n.round = t.Round
		if t.CurrentRound != 0 {
			n.round = t.CurrentRound
		}
		mergeHP(n.hp, t.HP)
		mergeHP(n.hp, t.HPFractions)
		mergeHP(n.hp, t.SideHP)
		mergeHP(n.hp, t.Health)
		mergeHP(n.hp, t.Fractions)
		n.conditions = append(n.conditions, t.Conditions...)
		n.conditions = append(n.conditions, t.Statuses...)
		n.conditions = append(n.conditions, t.Effects...)
	case map[string]any:
		if nv, ok := getMapKey(t, "active", "is_active", "active_encounter", "in_combat", "combat"); ok {
			n.active = toBool(nv)
		}
		if nv, ok := getMapKey(t, "round", "current_round", "round_number"); ok {
			n.round = int(toFloat(nv))
		}
		for _, key := range []string{"hp", "hp_fractions", "hpFractions", "side_hp", "sideHp", "health", "fractions", "hp_fraction"} {
			if nv, ok := getMapKey(t, key); ok {
				mergeHPAny(n.hp, nv)
			}
		}
		for _, key := range []string{"conditions", "statuses", "effects", "condition_list", "status_list"} {
			if nv, ok := getMapKey(t, key); ok {
				mergeConditions(&n.conditions, nv)
			}
		}
		// nested encounter
		if nv, ok := getMapKey(t, "encounter", "encounter_state", "state"); ok {
			inner := normalizeEncounter(nv)
			if inner.active {
				n.active = true
			}
			if inner.round != 0 {
				n.round = inner.round
			}
			mergeHP(n.hp, inner.hp)
			n.conditions = append(n.conditions, inner.conditions...)
		}
		// sides array shape: [{"side":"party","hp_fraction":0.5}]
		if nv, ok := getMapKey(t, "sides", "participants"); ok {
			if arr, ok := nv.([]any); ok {
				for _, e := range arr {
					if m, ok := e.(map[string]any); ok {
						side := ""
						if sv, ok := getMapKey(m, "side", "name", "team", "faction"); ok {
							side = fmt.Sprint(sv)
						}
						var frac float64
						found := false
						for _, key := range []string{"hp", "hp_fraction", "fraction", "health", "hp_percent"} {
							if fv, ok := getMapKey(m, key); ok {
								frac = toFloat(fv)
								found = true
								break
							}
						}
						if side != "" && found {
							n.hp[strings.ToLower(side)] = frac
						}
					}
				}
			}
		}
	default:
		// struct via reflection
		if rv.Kind() == reflect.Struct {
			for i := 0; i < rv.NumField(); i++ {
				ft := rv.Type().Field(i)
				name := ft.Name
				lname := strings.ToLower(name)
				fv := rv.Field(i)
				if !fv.CanInterface() {
					continue
				}
				val := fv.Interface()
				switch {
				case strings.Contains(lname, "active") || strings.Contains(lname, "combat"):
					if fv.Kind() == reflect.Bool {
						if toBool(val) {
							n.active = true
						}
					}
				case strings.Contains(lname, "round"):
					if fv.Kind() == reflect.Int || fv.Kind() == reflect.Int64 || fv.Kind() == reflect.Float64 {
						if r := int(toFloat(val)); r != 0 {
							n.round = r
						}
					}
				case strings.Contains(lname, "hp") || strings.Contains(lname, "health") || strings.Contains(lname, "fraction"):
					mergeHPAny(n.hp, val)
				case strings.Contains(lname, "condition") || strings.Contains(lname, "status") || strings.Contains(lname, "effect"):
					mergeConditions(&n.conditions, val)
				}
			}
		}
	}
	// normalise condition strings
	outConds := make([]string, 0, len(n.conditions))
	for _, c := range n.conditions {
		outConds = append(outConds, strings.ToLower(strings.TrimSpace(c)))
	}
	n.conditions = outConds
	// normalise hp keys
	normHP := map[string]float64{}
	for k, v := range n.hp {
		normHP[strings.ToLower(strings.TrimSpace(k))] = v
	}
	n.hp = normHP
	return n
}

func normalizeKeywords(v any) []string {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case string:
		fields := strings.Fields(strings.ReplaceAll(t, ",", " "))
		out := make([]string, 0, len(fields))
		for _, f := range fields {
			out = append(out, strings.TrimSpace(f))
		}
		return out
	case []string:
		out := make([]string, len(t))
		copy(out, t)
		return out
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			out = append(out, fmt.Sprint(e))
		}
		return out
	}
	return []string{fmt.Sprint(v)}
}

var (
	combatTerms = map[string]bool{
		"combat": true, "attack": true, "attacks": true, "attacked": true,
		"initiative": true, "damage": true, "hit": true, "hits": true,
		"strike": true, "strikes": true, "wound": true, "wounded": true,
		"battle": true, "fight": true, "fighting": true, "weapon": true,
		"spell": true, "cast": true, "bloodied": true, "enemy": true,
		"monster": true, "hostile": true,
	}
	restTerms = map[string]bool{
		"rest": true, "camp": true, "heal": true, "healing": true,
		"recover": true, "recovery": true, "safe": true, "breather": true,
		"shortrest": true, "longrest": true,
	}
	victoryTerms = map[string]bool{
		"victory": true, "victorious": true, "triumph": true, "triumphant": true,
		"defeated": true, "win": true, "won": true, "success": true, "cleared": true,
	}
	sombreTerms = map[string]bool{
		"death": true, "died": true, "dead": true, "mourning": true, "mourn": true,
		"funeral": true, "grief": true, "loss": true, "lost": true, "tragedy": true,
		"tears": true, "fallen": true, "tpk": true,
	}
	tenseConditions = map[string]bool{
		"bloodied": true, "stunned": true, "poisoned": true, "prone": true,
		"frightened": true, "charmed": true, "restrained": true, "paralyzed": true,
		"unconscious": true, "dying": true, "wounded": true, "staggered": true,
	}
	sombreConditions = map[string]bool{
		"dead": true, "deceased": true, "mourning": true, "fallen": true,
	}
	partyKeys = []string{"party", "players", "allies", "ally", "pc", "pcs", "heroes", "hero"}
	enemyKeys = []string{"enemy", "enemies", "monster", "monsters", "opposition", "hostile", "hostiles", "foe", "foes", "npc"}
)

func isPartyKey(k string) bool {
	for _, p := range partyKeys {
		if strings.Contains(k, p) {
			return true
		}
	}
	return false
}

func isEnemyKey(k string) bool {
	for _, p := range enemyKeys {
		if strings.Contains(k, p) {
			return true
		}
	}
	return false
}

func partyFraction(hp map[string]float64) float64 {
	vals := []float64{}
	for k, v := range hp {
		if isPartyKey(k) {
			vals = append(vals, v)
		}
	}
	if len(vals) == 0 {
		// unknown: assume healthy
		return 1.0
	}
	mn := vals[0]
	for _, v := range vals[1:] {
		if v < mn {
			mn = v
		}
	}
	return mn
}

func enemyFraction(hp map[string]float64) float64 {
	vals := []float64{}
	for k, v := range hp {
		if isEnemyKey(k) {
			vals = append(vals, v)
		}
	}
	if len(vals) == 0 {
		return 1.0
	}
	mn := vals[0]
	for _, v := range vals[1:] {
		if v < mn {
			mn = v
		}
	}
	return mn
}

// Derive is the canonical mood function.
func Derive(encounter any, keywords any) string {
	n := normalizeEncounter(encounter)
	kws := normalizeKeywords(keywords)
	lower := make([]string, 0, len(kws))
	for _, k := range kws {
		lower = append(lower, strings.ToLower(strings.TrimSpace(k)))
	}

	hasSombre := false
	hasCombat := n.active || n.round > 0
	hasVictory := false
	hasRest := false

	for _, k := range lower {
		if sombreTerms[k] {
			hasSombre = true
		}
		if combatTerms[k] {
			hasCombat = true
		}
		if victoryTerms[k] {
			hasVictory = true
		}
		if restTerms[k] {
			hasRest = true
		}
		// phrase containment for multi-word keywords
		for term := range sombreTerms {
			if strings.Contains(k, term) {
				hasSombre = true
			}
		}
		for term := range combatTerms {
			if strings.Contains(k, term) {
				hasCombat = true
			}
		}
		for term := range victoryTerms {
			if strings.Contains(k, term) {
				hasVictory = true
			}
		}
		for term := range restTerms {
			if strings.Contains(k, term) {
				hasRest = true
			}
		}
	}
	for _, c := range n.conditions {
		if sombreConditions[c] {
			hasSombre = true
		}
		if tenseConditions[c] {
			hasCombat = true
		}
	}
	pf := partyFraction(n.hp)
	ef := enemyFraction(n.hp)
	if pf <= 0 {
		hasSombre = true
	}
	if pf < 1.0 {
		hasCombat = true
	}
	if n.active && ef <= 0 && len(n.hp) > 0 {
		// enemies defeated mid/after encounter
		hasVictory = true
	}
	if !n.active && ef <= 0 && pf > 0 && len(n.hp) > 0 {
		hasVictory = true
	}

	// decision
	if hasSombre {
		return MoodSombre
	}
	if hasCombat {
		return MoodTense
	}
	if hasVictory {
		return MoodTriumphant
	}
	if hasRest {
		// rest maps to calm per vocabulary default
		return MoodCalm
	}
	return DefaultMood
}

// Aliases for discoverability.
func DeriveMood(encounter any, keywords any) string { return Derive(encounter, keywords) }
func DeriveMoodTag(encounter any, keywords any) string {
	return Derive(encounter, keywords)
}
func DeriveMoods(encounter any, keywords any) []string {
	return []string{Derive(encounter, keywords)}
}
func DeriveMoodTags(encounter any, keywords any) []string {
	return []string{Derive(encounter, keywords)}
}

// IsVocabulary reports whether s is a valid mood tag.
func IsVocabulary(s string) bool {
	for _, v := range Vocabulary {
		if v == s {
			return true
		}
	}
	return false
}

// Ensure deterministic ordering of exported vocabulary (no-op guard).
func init() {
	sort.Strings(Vocabulary)
	// restore intended order
	Vocabulary = []string{MoodTense, MoodTriumphant, MoodSombre, MoodCalm}
}
