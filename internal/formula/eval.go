package formula

import (
	"fmt"
	"math"
	"time"
)

type evalCtx struct {
	scope    *Scope
	limits   Limits
	depth    int
	steps    int
	start    time.Time
	visiting map[string]bool
}

func newEvalCtx(s *Scope, lim Limits) *evalCtx {
	if s == nil {
		s = &Scope{}
	}
	// merge alias fields
	if s.Call == nil {
		s.Call = map[string]any{}
	}
	for k, v := range s.CallScope {
		if _, ok := s.Call[k]; !ok {
			s.Call[k] = v
		}
	}
	if s.Entity == nil {
		s.Entity = map[string]any{}
	}
	for k, v := range s.EntityScope {
		if _, ok := s.Entity[k]; !ok {
			s.Entity[k] = v
		}
	}
	if s.Pack == nil {
		s.Pack = map[string]any{}
	}
	for k, v := range s.PackScope {
		if _, ok := s.Pack[k]; !ok {
			s.Pack[k] = v
		}
	}
	if s.Formulas == nil {
		s.Formulas = map[string]string{}
	}
	for k, v := range s.Derived {
		if _, ok := s.Formulas[k]; !ok {
			s.Formulas[k] = v
		}
	}
	if s.memo == nil {
		s.memo = map[string]any{}
	}
	return &evalCtx{
		scope:    s,
		limits:   lim.normalize(),
		start:    time.Now(),
		visiting: map[string]bool{},
	}
}

func (ec *evalCtx) eval(e Expr) (any, error) {
	// step + timeout
	ec.steps++
	if ec.steps > ec.limits.MaxSteps {
		return nil, fmt.Errorf("formula: step limit exceeded")
	}
	if ec.limits.Timeout > 0 && time.Since(ec.start) > ec.limits.Timeout {
		return nil, fmt.Errorf("formula: timeout exceeded")
	}
	ec.depth++
	if ec.depth > ec.limits.MaxDepth {
		ec.depth--
		return nil, fmt.Errorf("formula: depth limit exceeded")
	}
	defer func() { ec.depth-- }()

	switch v := e.(type) {
	case NumberLit:
		return v.Value, nil
	case StringLit:
		return v.Value, nil
	case BoolLit:
		return v.Value, nil
	case NullLit:
		return nil, nil
	case ListLit:
		out := make([]any, 0, len(v.Items))
		for _, it := range v.Items {
			val, err := ec.eval(it)
			if err != nil {
				return nil, err
			}
			out = append(out, val)
		}
		return out, nil
	case MapLit:
		out := map[string]any{}
		for i := range v.Values {
			var key string
			if i < len(v.KeyExprs) && v.KeyExprs[i] != nil {
				kv, err := ec.eval(v.KeyExprs[i])
				if err != nil {
					return nil, err
				}
				key = toString(kv)
			} else if i < len(v.Keys) {
				key = v.Keys[i]
			}
			val, err := ec.eval(v.Values[i])
			if err != nil {
				return nil, err
			}
			out[key] = val
		}
		return out, nil
	case Ident:
		return ec.resolveIdent(v.Name)
	case Member:
		target, err := ec.eval(v.Target)
		if err != nil {
			return nil, err
		}
		// special: formulas derived
		if id, ok := v.Target.(Ident); ok && id.Name == "formulas" {
			if src, ok := ec.scope.Formulas[v.Name]; ok {
				_ = src
				return ec.evalDerived(v.Name)
			}
		}
		if id, ok := v.Target.(Ident); ok && id.Name == "formula" {
			if src, ok := ec.scope.Formulas[v.Name]; ok {
				_ = src
				return ec.evalDerived(v.Name)
			}
		}
		switch t := target.(type) {
		case map[string]any:
			if val, ok := t[v.Name]; ok {
				return val, nil
			}
			// derived formula named v.Name?
			if src, ok := ec.scope.Formulas[v.Name]; ok {
				_ = src
				return ec.evalDerived(v.Name)
			}
			return nil, fmt.Errorf("formula: missing key %q", v.Name)
		case nil:
			return nil, fmt.Errorf("formula: cannot access %q on null", v.Name)
		default:
			// try formula named
			if src, ok := ec.scope.Formulas[v.Name]; ok {
				_ = src
				return ec.evalDerived(v.Name)
			}
			return nil, fmt.Errorf("formula: cannot access %q on %T", v.Name, target)
		}
	case Index:
		target, err := ec.eval(v.Target)
		if err != nil {
			return nil, err
		}
		idx, err := ec.eval(v.Index)
		if err != nil {
			return nil, err
		}
		switch t := target.(type) {
		case []any:
			i := int(toFloat(idx))
			if i < 0 || i >= len(t) {
				return nil, fmt.Errorf("formula: index out of range")
			}
			return t[i], nil
		case map[string]any:
			key := toString(idx)
			if val, ok := t[key]; ok {
				return val, nil
			}
			return nil, fmt.Errorf("formula: missing key %q", key)
		case string:
			i := int(toFloat(idx))
			if i < 0 || i >= len(t) {
				return nil, fmt.Errorf("formula: index out of range")
			}
			return string(t[i]), nil
		default:
			return nil, fmt.Errorf("formula: cannot index %T", target)
		}
	case Unary:
		x, err := ec.eval(v.X)
		if err != nil {
			return nil, err
		}
		switch v.Op {
		case "-":
			return -toFloat(x), nil
		case "not":
			return !toBool(x), nil
		}
		return nil, fmt.Errorf("formula: unknown unary op %q", v.Op)
	case Binary:
		// short-circuit for && ||
		if v.Op == "&&" {
			l, err := ec.eval(v.Left)
			if err != nil {
				return nil, err
			}
			if !toBool(l) {
				return false, nil
			}
			r, err := ec.eval(v.Right)
			if err != nil {
				return nil, err
			}
			return toBool(r), nil
		}
		if v.Op == "||" {
			l, err := ec.eval(v.Left)
			if err != nil {
				return nil, err
			}
			if toBool(l) {
				return true, nil
			}
			r, err := ec.eval(v.Right)
			if err != nil {
				return nil, err
			}
			return toBool(r), nil
		}
		l, err := ec.eval(v.Left)
		if err != nil {
			return nil, err
		}
		r, err := ec.eval(v.Right)
		if err != nil {
			return nil, err
		}
		switch v.Op {
		case "+":
			// string concat if either string
			if ls, ok := l.(string); ok {
				return ls + toString(r), nil
			}
			if rs, ok := r.(string); ok {
				return toString(l) + rs, nil
			}
			return toFloat(l) + toFloat(r), nil
		case "-":
			return toFloat(l) - toFloat(r), nil
		case "*":
			return toFloat(l) * toFloat(r), nil
		case "/":
			den := toFloat(r)
			if den == 0 {
				return nil, fmt.Errorf("formula: division by zero")
			}
			return toFloat(l) / den, nil
		case "%":
			den := toFloat(r)
			if den == 0 {
				return nil, fmt.Errorf("formula: modulo by zero")
			}
			return math.Mod(toFloat(l), den), nil
		case "==":
			return valuesEqual(l, r), nil
		case "!=":
			return !valuesEqual(l, r), nil
		case "<":
			return toFloat(l) < toFloat(r), nil
		case "<=":
			return toFloat(l) <= toFloat(r), nil
		case ">":
			return toFloat(l) > toFloat(r), nil
		case ">=":
			return toFloat(l) >= toFloat(r), nil
		}
		return nil, fmt.Errorf("formula: unknown binary op %q", v.Op)
	case Ternary:
		c, err := ec.eval(v.Cond)
		if err != nil {
			return nil, err
		}
		if toBool(c) {
			return ec.eval(v.Then)
		}
		return ec.eval(v.Else)
	case Call:
		// builtin?
		if id, ok := v.Callee.(Ident); ok && isBuiltin(id.Name) {
			// resolve scope first? if scope has binding with same name, use it (not callable) -> error
			// check scope
			if _, ok := ec.scope.Call[id.Name]; !ok {
				if _, ok := ec.scope.Entity[id.Name]; !ok {
					if _, ok := ec.scope.Pack[id.Name]; !ok {
						if _, ok := ec.scope.Formulas[id.Name]; !ok {
							// builtin
							args := make([]any, 0, len(v.Args))
							for _, a := range v.Args {
								val, err := ec.eval(a)
								if err != nil {
									return nil, err
								}
								args = append(args, val)
							}
							return callBuiltin(id.Name, args)
						}
					}
				}
			}
		}
		callee, err := ec.eval(v.Callee)
		if err != nil {
			return nil, err
		}
		if bv, ok := callee.(builtinValue); ok {
			args := make([]any, 0, len(v.Args))
			for _, a := range v.Args {
				val, err := ec.eval(a)
				if err != nil {
					return nil, err
				}
				args = append(args, val)
			}
			return callBuiltin(bv.name, args)
		}
		// if callee is a formula value? not callable
		return nil, fmt.Errorf("formula: value is not callable")
	}
	return nil, fmt.Errorf("formula: unsupported expression %T", e)
}

func (ec *evalCtx) resolveIdent(name string) (any, error) {
	// layers
	if v, ok := ec.scope.Call[name]; ok {
		return v, nil
	}
	if v, ok := ec.scope.Entity[name]; ok {
		return v, nil
	}
	if v, ok := ec.scope.Pack[name]; ok {
		return v, nil
	}
	if _, ok := ec.scope.Formulas[name]; ok {
		return ec.evalDerived(name)
	}
	if isBuiltin(name) {
		return builtinValue{name: name}, nil
	}
	return nil, fmt.Errorf("formula: unresolvable name %q", name)
}

func (ec *evalCtx) evalDerived(name string) (any, error) {
	// memo
	ec.scope.mu.Lock()
	if v, ok := ec.scope.memo[name]; ok {
		ec.scope.mu.Unlock()
		return v, nil
	}
	ec.scope.mu.Unlock()

	if ec.visiting[name] {
		// cycle: list visiting
		names := []string{name}
		for n := range ec.visiting {
			if n != name {
				names = append(names, n)
			}
		}
		return nil, fmt.Errorf("formula: cyclic dependency among %v", names)
	}
	src, ok := ec.scope.Formulas[name]
	if !ok {
		return nil, fmt.Errorf("formula: unresolvable name %q", name)
	}
	e, err := Parse(src)
	if err != nil {
		return nil, err
	}
	child := *ec
	child.visiting = map[string]bool{}
	for k, v := range ec.visiting {
		child.visiting[k] = v
	}
	child.visiting[name] = true
	val, err := child.eval(e)
	if err != nil {
		return nil, err
	}
	ec.scope.mu.Lock()
	ec.scope.memo[name] = val
	ec.scope.mu.Unlock()
	return val, nil
}

func toBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x != ""
	case nil:
		return false
	}
	return true
}
