package formula

import (
	"fmt"
	"math"
	"strings"
)

var builtinNames = map[string]bool{
	"min": true, "max": true, "floor": true, "ceil": true, "round": true,
	"abs": true, "clamp": true, "sum": true, "len": true, "has": true,
	"coalesce": true, "count": true, "lower": true, "upper": true,
}

type builtinValue struct{ name string }

func isBuiltin(name string) bool { return builtinNames[name] }

func callBuiltin(name string, args []any) (any, error) {
	switch name {
	case "min":
		if len(args) < 2 {
			return nil, fmt.Errorf("formula: min needs at least 2 args")
		}
		m := toFloat(args[0])
		for _, a := range args[1:] {
			v := toFloat(a)
			if v < m {
				m = v
			}
		}
		return m, nil
	case "max":
		if len(args) < 2 {
			return nil, fmt.Errorf("formula: max needs at least 2 args")
		}
		m := toFloat(args[0])
		for _, a := range args[1:] {
			v := toFloat(a)
			if v > m {
				m = v
			}
		}
		return m, nil
	case "floor":
		if len(args) != 1 {
			return nil, fmt.Errorf("formula: floor needs 1 arg")
		}
		return math.Floor(toFloat(args[0])), nil
	case "ceil":
		if len(args) != 1 {
			return nil, fmt.Errorf("formula: ceil needs 1 arg")
		}
		return math.Ceil(toFloat(args[0])), nil
	case "round":
		if len(args) != 1 {
			return nil, fmt.Errorf("formula: round needs 1 arg")
		}
		return math.Round(toFloat(args[0])), nil
	case "abs":
		if len(args) != 1 {
			return nil, fmt.Errorf("formula: abs needs 1 arg")
		}
		return math.Abs(toFloat(args[0])), nil
	case "clamp":
		if len(args) != 3 {
			return nil, fmt.Errorf("formula: clamp needs 3 args")
		}
		v, lo, hi := toFloat(args[0]), toFloat(args[1]), toFloat(args[2])
		if v < lo {
			v = lo
		}
		if v > hi {
			v = hi
		}
		return v, nil
	case "sum":
		// sum of list or args
		total := 0.0
		if len(args) == 1 {
			if sl, ok := args[0].([]any); ok {
				for _, a := range sl {
					total += toFloat(a)
				}
				return total, nil
			}
		}
		for _, a := range args {
			total += toFloat(a)
		}
		return total, nil
	case "len":
		if len(args) != 1 {
			return nil, fmt.Errorf("formula: len needs 1 arg")
		}
		switch v := args[0].(type) {
		case string:
			return float64(len(v)), nil
		case []any:
			return float64(len(v)), nil
		case map[string]any:
			return float64(len(v)), nil
		case nil:
			return float64(0), nil
		default:
			return nil, fmt.Errorf("formula: len unsupported type %T", v)
		}
	case "has":
		// has(container, key) or has(list, value)
		if len(args) != 2 {
			return nil, fmt.Errorf("formula: has needs 2 args")
		}
		switch c := args[0].(type) {
		case map[string]any:
			_, ok := c[toString(args[1])]
			return ok, nil
		case []any:
			for _, a := range c {
				if valuesEqual(a, args[1]) {
					return true, nil
				}
			}
			return false, nil
		case nil:
			return false, nil
		default:
			// maybe string contains
			if s, ok := c.(string); ok {
				return strings.Contains(s, toString(args[1])), nil
			}
			return false, nil
		}
	case "coalesce":
		for _, a := range args {
			if !isNilOrFalse(a) {
				return a, nil
			}
		}
		if len(args) > 0 {
			return args[len(args)-1], nil
		}
		return nil, nil
	case "count":
		// count(list) or count(list, predicate?) simplified: count truthy?
		if len(args) == 1 {
			if sl, ok := args[0].([]any); ok {
				return float64(len(sl)), nil
			}
			return float64(0), nil
		}
		// count(list, value)
		if len(args) == 2 {
			if sl, ok := args[0].([]any); ok {
				n := 0
				for _, a := range sl {
					if valuesEqual(a, args[1]) {
						n++
					}
				}
				return float64(n), nil
			}
		}
		return float64(len(args)), nil
	case "lower":
		if len(args) != 1 {
			return nil, fmt.Errorf("formula: lower needs 1 arg")
		}
		return strings.ToLower(toString(args[0])), nil
	case "upper":
		if len(args) != 1 {
			return nil, fmt.Errorf("formula: upper needs 1 arg")
		}
		return strings.ToUpper(toString(args[0])), nil
	}
	return nil, fmt.Errorf("formula: unknown builtin %q", name)
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case bool:
		if x {
			return 1
		}
		return 0
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int8:
		return float64(x)
	case int16:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case uint:
		return float64(x)
	case uint8:
		return float64(x)
	case uint16:
		return float64(x)
	case uint32:
		return float64(x)
	case uint64:
		return float64(x)
	case string:
		f, _ := strconvParse(x)
		return f
	case nil:
		return 0
	}
	return 0
}

func strconvParse(s string) (float64, error) {
	// local helper to avoid import cycle? strconv available
	importStdconv := true
	_ = importStdconv
	return parseFloat(s)
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		// format ints without decimal
		if x == math.Trunc(x) {
			return fmt.Sprintf("%.0f", x)
		}
		return fmt.Sprintf("%g", x)
	case string:
		return x
	}
	return fmt.Sprintf("%v", v)
}

func isNilOrFalse(v any) bool {
	if v == nil {
		return true
	}
	if b, ok := v.(bool); ok && !b {
		return true
	}
	return false
}

func valuesEqual(a, b any) bool {
	// handle numbers
	switch av := a.(type) {
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case nil:
		return b == nil
	}
	// numbers
	an, aok := asNumber(a)
	bn, bok := asNumber(b)
	if aok && bok {
		return an == bn
	}
	return false
}

func asNumber(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int8:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint8:
		return float64(x), true
	case uint16:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}
