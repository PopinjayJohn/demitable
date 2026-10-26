package formula

import (
	"sync"
)

// Scope is the layered evaluation context (ADR-0042).
type Scope struct {
	Call     map[string]any
	Entity   map[string]any
	Pack     map[string]any
	Formulas map[string]string

	// alias fields
	CallScope   map[string]any
	EntityScope map[string]any
	PackScope   map[string]any
	Derived     map[string]string

	// internals
	mu      sync.Mutex
	memo    map[string]any
	memoErr map[string]error
}

// NewScope constructs a scope. Any layer may be nil.
func NewScope(call, entity, pack map[string]any) *Scope {
	return &Scope{Call: call, Entity: entity, Pack: pack, Formulas: map[string]string{}, memo: map[string]any{}}
}

// SetFormulas sets derived formulas.
func (s *Scope) SetFormulas(m map[string]string) {
	s.Formulas = m
	s.memo = map[string]any{}
}

// Eval parses and evaluates an expression against this scope with default limits.
func (s *Scope) Eval(src string) (any, error) {
	return s.EvalWithLimits(src, DefaultLimits())
}

// EvalWithLimits parses and evaluates with limits.
func (s *Scope) EvalWithLimits(src string, lim Limits) (any, error) {
	e, err := Parse(src)
	if err != nil {
		return nil, err
	}
	return s.EvalExprWithLimits(e, lim)
}

// EvalExpr evaluates a parsed expression.
func (s *Scope) EvalExpr(e Expr) (any, error) {
	return s.EvalExprWithLimits(e, DefaultLimits())
}

// EvalExprWithLimits evaluates with limits.
func (s *Scope) EvalExprWithLimits(e Expr, lim Limits) (any, error) {
	ec := newEvalCtx(s, lim)
	return ec.eval(e)
}

// EvalFormula evaluates a derived formula by name.
func (s *Scope) EvalFormula(name string) (any, error) {
	return s.EvalFormulaWithLimits(name, DefaultLimits())
}

// EvalFormulaWithLimits evaluates a derived formula with limits.
func (s *Scope) EvalFormulaWithLimits(name string, lim Limits) (any, error) {
	ec := newEvalCtx(s, lim)
	// if name is a formula, eval derived; else resolve as binding
	if _, ok := s.Formulas[name]; ok {
		return ec.evalDerived(name)
	}
	// maybe a top-level binding
	v, err := ec.resolveIdent(name)
	if err != nil {
		return nil, err
	}
	// if builtin
	if bv, ok := v.(builtinValue); ok {
		return bv, nil
	}
	return v, nil
}

// Package-level helpers.

func Evaluate(src string, scope *Scope) (any, error) {
	if scope == nil {
		scope = &Scope{}
	}
	return scope.Eval(src)
}

func Eval(src string, scope *Scope) (any, error) {
	return Evaluate(src, scope)
}

func EvalWithLimits(src string, scope *Scope, lim Limits) (any, error) {
	if scope == nil {
		scope = &Scope{}
	}
	return scope.EvalWithLimits(src, lim)
}

func EvalExpr(e Expr, scope *Scope) (any, error) {
	if scope == nil {
		scope = &Scope{}
	}
	return scope.EvalExpr(e)
}

func EvalExprWithLimits(e Expr, scope *Scope, lim Limits) (any, error) {
	if scope == nil {
		scope = &Scope{}
	}
	return scope.EvalExprWithLimits(e, lim)
}
