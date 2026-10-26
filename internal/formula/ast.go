// Package formula implements the sandboxed expression language used by
// system packs (ADR-0003, ADR-0042).
//
// The package is stdlib-only and side-effect free: it parses and evaluates
// expressions against an explicit layered scope. It does not depend on the
// pack or vault packages, so it can be reused by the pack linter and the
// server's compute endpoint.
package formula

// Expr is the interface implemented by every AST node.
type Expr interface {
	exprNode()
}

type NumberLit struct{ Value float64 }

func (NumberLit) exprNode() {}

type StringLit struct{ Value string }

func (StringLit) exprNode() {}

type BoolLit struct{ Value bool }

func (BoolLit) exprNode() {}

type NullLit struct{}

func (NullLit) exprNode() {}

type ListLit struct{ Items []Expr }

func (ListLit) exprNode() {}

type MapLit struct {
	Keys     []string // may be empty when a key is a computed expression
	KeyExprs []Expr   // parallel to Keys; nil entry means use Keys[i]
	Values   []Expr
}

func (MapLit) exprNode() {}

type Ident struct{ Name string }

func (Ident) exprNode() {}

type Member struct {
	Target Expr
	Name   string
}

func (Member) exprNode() {}

type Index struct {
	Target Expr
	Index  Expr
}

func (Index) exprNode() {}

type Call struct {
	Callee Expr
	Args   []Expr
}

func (Call) exprNode() {}

type Unary struct {
	Op string
	X  Expr
}

func (Unary) exprNode() {}

type Binary struct {
	Op    string
	Left  Expr
	Right Expr
}

func (Binary) exprNode() {}

type Ternary struct {
	Cond Expr
	Then Expr
	Else Expr
}

func (Ternary) exprNode() {}
