package formula

import "fmt"

// ParseExpr is an alias for Parse.
func ParseExpr(src string) (Expr, error) { return Parse(src) }

// Compile is an alias for Parse.
func Compile(src string) (Expr, error) { return Parse(src) }

// EvaluateExpr is an alias.
func EvaluateExpr(e Expr, scope *Scope) (any, error) { return EvalExpr(e, scope) }

// Compute evaluates an expression string.
func Compute(src string, scope *Scope) (any, error) { return Evaluate(src, scope) }

// Run evaluates an expression string.
func Run(src string, scope *Scope) (any, error) { return Evaluate(src, scope) }

// Context aliases.
type EvalContext = Context
type ScopeContext = Context

// Issue aliases.
type Diagnostic = Issue
type LintError = Issue

// LintErr returns issues as errors.
func LintErr(args ...any) []error {
	issues := Check(args...)
	out := make([]error, 0, len(issues))
	for _, is := range issues {
		out = append(out, fmt.Errorf("%s", is.Message))
	}
	return out
}

// NewContext builds a Context.
func NewContext(name string) Context { return Context{Name: name} }
