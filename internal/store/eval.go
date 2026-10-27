package store

import (
	"fmt"
	"strings"
)

// ParseQuery exposes the parser.
func ParseQuery(s string) (Expr, error) {
	e, err := parseQuery(s)
	if err != nil {
		return nil, err
	}
	return e, nil
}

// EvalExpr evaluates a parsed expression against a value lookup function.
func EvalExpr(e Expr, lookup func(path string) any) bool {
	if e == nil {
		return true
	}
	switch v := e.(type) {
	case *binaryExpr:
		l := EvalExpr(v.Left, lookup)
		r := EvalExpr(v.Right, lookup)
		if strings.ToLower(v.Op) == "and" {
			return l && r
		}
		if strings.ToLower(v.Op) == "or" {
			return l || r
		}
	case *notExpr:
		return !EvalExpr(v.X, lookup)
	case *compareExpr:
		return evalCompare(v, lookup)
	}
	return true
}

func evalCompare(c *compareExpr, lookup func(path string) any) bool {
	val := lookup(c.Path)
	op := strings.ToLower(c.Op)
	switch op {
	case "==", "=":
		return valuesEqualAny(val, c.Value)
	case "!=":
		return !valuesEqualAny(val, c.Value)
	case ">":
		return compareAny(val, c.Value) > 0
	case ">=":
		return compareAny(val, c.Value) >= 0
	case "<":
		return compareAny(val, c.Value) < 0
	case "<=":
		return compareAny(val, c.Value) <= 0
	case "contains":
		if ss, ok := val.([]string); ok {
			for _, s := range ss {
				if strings.EqualFold(s, fmt.Sprint(c.Value)) {
					return true
				}
			}
			joined := strings.ToLower(strings.Join(ss, " "))
			return strings.Contains(joined, strings.ToLower(fmt.Sprint(c.Value)))
		}
		if arr, ok := val.([]any); ok {
			for _, e := range arr {
				if valuesEqualAny(e, c.Value) {
					return true
				}
			}
			// substring fallback
			return strings.Contains(strings.ToLower(fmt.Sprint(val)), strings.ToLower(fmt.Sprint(c.Value)))
		}
		return strings.Contains(strings.ToLower(fmt.Sprint(val)), strings.ToLower(fmt.Sprint(c.Value)))
	case "in":
		list, ok := c.Value.([]any)
		if !ok {
			return false
		}
		for _, item := range list {
			if valuesEqualAny(val, item) {
				return true
			}
		}
		return false
	}
	return false
}

func valuesEqualAny(v, want any) bool {
	// handle slices
	if ss, ok := v.([]string); ok {
		for _, s := range ss {
			if strings.EqualFold(s, fmt.Sprint(want)) {
				return true
			}
		}
		return false
	}
	if arr, ok := v.([]any); ok {
		for _, e := range arr {
			if valuesEqualAny(e, want) {
				return true
			}
		}
		return false
	}
	return compareAny(v, want) == 0
}
