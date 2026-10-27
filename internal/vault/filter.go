package vault

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
)

type filterPair struct {
	key      string
	value    string
	contains bool
}

// parseFilters flattens Query.Fields and Query.Filter into a list of
// equality/contains predicates. Unknown syntax is ignored (returns true).
func parseFilters(q Query) []filterPair {
	var out []filterPair
	for k, v := range q.Fields {
		out = append(out, filterPair{key: k, value: v})
	}
	f := strings.TrimSpace(q.Filter)
	if f == "" {
		return out
	}
	// Allow several predicates separated by commas, semicolons or " and ".
	norm := strings.ReplaceAll(strings.ToLower(f), " and ", ",")
	for _, part := range strings.FieldsFunc(norm, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	}) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Restore original casing for the value by re-parsing from the
		// original filter is not possible after lowercasing; instead parse
		// case-insensitively from the lowercased text, which is fine for
		// equality over these fields.
		key, value, contains, ok := parseOneFilter(part)
		if ok {
			out = append(out, filterPair{key: key, value: value, contains: contains})
		}
	}
	// If the above lowercased the values, re-extract values preserving case
	// from the original filter string.
	out = reextractFilterValues(f, out)
	return out
}

func parseOneFilter(part string) (string, string, bool, bool) {
	part = strings.TrimSpace(part)
	lower := strings.ToLower(part)
	if i := strings.Index(lower, " contains "); i >= 0 {
		key := strings.TrimSpace(part[:i])
		value := strings.TrimSpace(part[i+len(" contains "):])
		value = strings.Trim(value, `"'`)
		return key, value, true, key != ""
	}
	for _, sep := range []string{"==", "=", ":"} {
		if i := indexOfSep(part, sep); i >= 0 {
			key := strings.TrimSpace(part[:i])
			value := strings.TrimSpace(part[i+len(sep):])
			value = strings.Trim(value, `"'`)
			// skip "equals" word? handled below
			if key != "" {
				return key, value, false, true
			}
		}
	}
	if i := strings.Index(lower, " equals "); i >= 0 {
		key := strings.TrimSpace(part[:i])
		value := strings.TrimSpace(part[i+len(" equals "):])
		value = strings.Trim(value, `"'`)
		return key, value, false, key != ""
	}
	return "", "", false, false
}

// indexOfSep finds the first occurrence of sep not inside quotes.
func indexOfSep(s, sep string) int {
	// simple: first occurrence
	return strings.Index(s, sep)
}

// reextractFilterValues preserves original casing for values by re-parsing
// the original (mixed-case) filter with the same separators.
func reextractFilterValues(original string, parsed []filterPair) []filterPair {
	// The parsed values may already have correct case if parseOneFilter was
	// run on the original-cased part. But parseFilters lowercased first, so
	// redo from original to be safe.
	// Rebuild by re-running parse on each original segment.
	norm := original
	// split the same way
	segs := splitFilterSegments(norm)
	if len(segs) != len(parsed) {
		// mismatch: keep lowercased values
		return parsed
	}
	out := make([]filterPair, len(parsed))
	for i, seg := range segs {
		key, value, contains, ok := parseOneFilter(strings.TrimSpace(seg))
		if !ok {
			out[i] = parsed[i]
			continue
		}
		out[i] = filterPair{key: key, value: value, contains: contains}
	}
	return out
}

func splitFilterSegments(f string) []string {
	return strings.FieldsFunc(f, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
}

func splitFilter(f string) []string {
	return strings.FieldsFunc(f, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
}

// fieldValue returns the value of a universal or pack-declared field.
func fieldValue(n Note, key string) (any, bool) {
	k := strings.ToLower(strings.TrimSpace(key))
	switch k {
	case "type":
		return n.Type, true
	case "uid":
		return n.UID, true
	case "name", "title":
		return n.Name, true
	case "system":
		return n.System, true
	case "campaign":
		return n.Campaign, true
	case "kind":
		return n.Kind, true
	case "player":
		return n.Player, true
	case "workflow":
		return n.Workflow, true
	case "visibility":
		return n.Visibility, true
	case "grants":
		return n.Grants, true
	case "revealed":
		return n.Revealed, true
	case "discovered":
		return n.Discovered, true
	case "tags":
		return n.Tags, true
	case "aliases":
		return n.Aliases, true
	case "path":
		return n.Path, true
	}
	// pack-declared / GM-owned fields (case-insensitive lookup)
	for fk, fv := range n.Fields {
		if strings.EqualFold(fk, key) {
			return fv, true
		}
	}
	return nil, false
}

func matchFilters(n Note, filters []filterPair) bool {
	for _, f := range filters {
		v, ok := fieldValue(n, f.key)
		if !ok {
			return false
		}
		if f.contains {
			if ss, ok := v.([]string); ok {
				if !containsString(ss, f.value) {
					// substring fallback
					joined := strings.ToLower(strings.Join(ss, " "))
					if !strings.Contains(joined, strings.ToLower(f.value)) {
						return false
					}
				}
			} else {
				s := strings.ToLower(fmt.Sprintf("%v", v))
				if !strings.Contains(s, strings.ToLower(f.value)) {
					return false
				}
			}
			continue
		}
		if !valuesEqual(v, f.value) {
			return false
		}
	}
	return true
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}

func valuesEqual(v any, want string) bool {
	if fv, ok := v.(frontmatter.Value); ok {
		switch fv.Kind {
		case frontmatter.KindBool:
			b, err := strconv.ParseBool(want)
			if err != nil {
				return strings.EqualFold(want, "true") == fv.Bool
			}
			return fv.Bool == b
		case frontmatter.KindInt:
			nv, err := strconv.ParseInt(want, 10, 64)
			if err != nil {
				return false
			}
			return fv.Int == nv
		case frontmatter.KindFloat:
			nv, err := strconv.ParseFloat(want, 64)
			if err != nil {
				return false
			}
			return fv.Flo == nv
		case frontmatter.KindList:
			for _, e := range fv.List {
				if valuesEqual(e, want) {
					return true
				}
			}
			return false
		default:
			return strings.EqualFold(fv.String(), want)
		}
	}
	switch val := v.(type) {
	case bool:
		b, err := strconv.ParseBool(want)
		if err != nil {
			return strings.EqualFold(want, "true") == val
		}
		return val == b
	case []string:
		return containsString(val, want)
	case string:
		return strings.EqualFold(val, want)
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		nv, err1 := strconv.ParseFloat(want, 64)
		if err1 != nil {
			return false
		}
		// convert v to float64
		fv, err2 := toFloat64(val)
		if err2 != nil {
			return false
		}
		return fv == nv
	default:
		return strings.EqualFold(fmt.Sprintf("%v", val), want)
	}
}

func toFloat64(v any) (float64, error) {
	if fv, ok := v.(frontmatter.Value); ok {
		switch fv.Kind {
		case frontmatter.KindInt:
			return float64(fv.Int), nil
		case frontmatter.KindFloat:
			return fv.Flo, nil
		}
		return 0, fmt.Errorf("not number")
	}
	switch n := v.(type) {
	case int:
		return float64(n), nil
	case int8:
		return float64(n), nil
	case int16:
		return float64(n), nil
	case int32:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case uint:
		return float64(n), nil
	case uint8:
		return float64(n), nil
	case uint16:
		return float64(n), nil
	case uint32:
		return float64(n), nil
	case uint64:
		return float64(n), nil
	case float32:
		return float64(n), nil
	case float64:
		return n, nil
	}
	return 0, fmt.Errorf("not number")
}

// sortNotes sorts the notes in place.
//
// The sort key is precomputed into a parallel slice, which is what keeps a
// 5,000-note sort inside the 100 ms criterion (the comparator is called
// O(n log n) times, and re-reading the field each time is what made it
// slow).
//
// **The keys must travel with the notes.** A `sort.SliceStable` over the
// notes alone, comparing against a parallel key slice indexed by the
// original position, desynchronises the two the moment the first swap
// happens - the keys stay where they were while the notes move. The
// result is a sort that is not sorted.
//
// So the notes and their keys are sorted together, as one slice of pairs.
func sortNotes(out []Note, sortKey string, desc bool) {
	if sortKey == "" {
		sort.SliceStable(out, func(i, j int) bool {
			if desc {
				return out[i].UID > out[j].UID
			}
			return out[i].UID < out[j].UID
		})
		return
	}
	// Precompute sort keys once, to avoid repeated allocations in the
	// comparator.
	keys := make([]noteSortKey, len(out))
	for i := range out {
		v, _ := fieldValue(out[i], sortKey)
		keys[i] = noteSortKey{value: v, index: i}
	}
	// Detect numeric sort.
	numeric := true
	for _, k := range keys {
		if _, err := toFloat64(k.value); err != nil {
			numeric = false
			break
		}
	}
	if numeric {
		for i := range keys {
			f, _ := toFloat64(keys[i].value)
			keys[i].num = f
			keys[i].isNum = true
		}
		sort.SliceStable(keys, func(i, j int) bool {
			if keys[i].num != keys[j].num {
				if desc {
					return keys[i].num > keys[j].num
				}
				return keys[i].num < keys[j].num
			}
			// tie-break by uid for parity with the SQL store path
			if desc {
				return out[keys[i].index].UID > out[keys[j].index].UID
			}
			return out[keys[i].index].UID < out[keys[j].index].UID
		})
	} else {
		// String sort keys, kept in their original case to match the SQL
		// store path's BINARY collation (case-sensitive). Lowercasing
		// here previously made the in-memory path disagree with SQL.
		for i := range keys {
			keys[i].str = sortKeyString(keys[i].value)
		}
		sort.SliceStable(keys, func(i, j int) bool {
			if keys[i].str != keys[j].str {
				if desc {
					return keys[i].str > keys[j].str
				}
				return keys[i].str < keys[j].str
			}
			// tie-break by uid for parity with the SQL store path
			if desc {
				return out[keys[i].index].UID > out[keys[j].index].UID
			}
			return out[keys[i].index].UID < out[keys[j].index].UID
		})
	}
	// Put the notes back in the order the keys sorted to.
	sorted := make([]Note, len(out))
	for i := range keys {
		sorted[i] = out[keys[i].index]
	}
	copy(out, sorted)
}

// noteSortKey is one note's sort key, and the note's original position,
// so that the two can travel together through the sort.
type noteSortKey struct {
	value any
	// index is the note's position in the slice being sorted.
	index int
	// num and str are the precomputed comparison forms.
	num   float64
	isNum bool
	str   string
}

// sortKeyString renders a sort key as the string it compares by.
func sortKeyString(v any) string {
	if fv, ok := v.(frontmatter.Value); ok {
		return fv.String()
	}
	return fmt.Sprintf("%v", v)
}

func compareValues(a, b any) int {
	// Fast path for frontmatter.Value: compare by kind without allocating.
	if fa, oka := a.(frontmatter.Value); oka {
		if fb, okb := b.(frontmatter.Value); okb {
			// same kind: direct compare
			if fa.Kind == fb.Kind {
				switch fa.Kind {
				case frontmatter.KindInt:
					if fa.Int < fb.Int {
						return -1
					}
					if fa.Int > fb.Int {
						return 1
					}
					return 0
				case frontmatter.KindFloat:
					if fa.Flo < fb.Flo {
						return -1
					}
					if fa.Flo > fb.Flo {
						return 1
					}
					return 0
				case frontmatter.KindBool:
					if fa.Bool == fb.Bool {
						return 0
					}
					if !fa.Bool {
						return -1
					}
					return 1
				case frontmatter.KindString, frontmatter.KindLink, frontmatter.KindDate:
					sa := strings.ToLower(fa.Str)
					sb := strings.ToLower(fb.Str)
					if sa < sb {
						return -1
					}
					if sa > sb {
						return 1
					}
					return 0
				}
			}
			// different kinds: fall through to string compare using String()
			a = fa.String()
			b = fb.String()
		} else {
			a = fa.String()
		}
	} else if fb, ok := b.(frontmatter.Value); ok {
		b = fb.String()
	}
	// numbers
	fa, ea := toFloat64(a)
	fb, eb := toFloat64(b)
	if ea == nil && eb == nil {
		if fa < fb {
			return -1
		}
		if fa > fb {
			return 1
		}
		return 0
	}
	// strings / fallback
	sa := strings.ToLower(fmt.Sprintf("%v", a))
	sb := strings.ToLower(fmt.Sprintf("%v", b))
	if sa < sb {
		return -1
	}
	if sa > sb {
		return 1
	}
	return 0
}
