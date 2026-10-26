package formula

import "fmt"

type parser struct {
	toks []token
	pos  int
}

const maxParseDepth = 1024

func Parse(src string) (Expr, error) {
	l := &lexer{src: src}
	toks, err := l.lexAll()
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	e, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tkEOF {
		return nil, fmt.Errorf("formula: unexpected token %q", p.peek().val)
	}
	return e, nil
}

func (p *parser) parseExpr(depth int) (Expr, error) {
	if depth > maxParseDepth {
		return nil, fmt.Errorf("formula: expression too deeply nested")
	}
	return p.parseTernary(depth + 1)
}

func (p *parser) parseTernary(depth int) (Expr, error) {
	cond, err := p.parseLogicOr(depth + 1)
	if err != nil {
		return nil, err
	}
	if p.peek().kind == tkPunct && p.peek().val == "?" {
		p.next()
		then, err := p.parseExpr(depth + 1)
		if err != nil {
			return nil, err
		}
		if !(p.peek().kind == tkPunct && p.peek().val == ":") {
			return nil, fmt.Errorf("formula: expected ':' in ternary")
		}
		p.next()
		els, err := p.parseExpr(depth + 1)
		if err != nil {
			return nil, err
		}
		return Ternary{Cond: cond, Then: then, Else: els}, nil
	}
	return cond, nil
}

func (p *parser) parseLogicOr(depth int) (Expr, error) {
	left, err := p.parseLogicAnd(depth + 1)
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tkOp && p.peek().val == "||" {
		p.next()
		right, err := p.parseLogicAnd(depth + 1)
		if err != nil {
			return nil, err
		}
		left = Binary{Op: "||", Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseLogicAnd(depth int) (Expr, error) {
	left, err := p.parseEquality(depth + 1)
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tkOp && p.peek().val == "&&" {
		p.next()
		right, err := p.parseEquality(depth + 1)
		if err != nil {
			return nil, err
		}
		left = Binary{Op: "&&", Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseEquality(depth int) (Expr, error) {
	left, err := p.parseComparison(depth + 1)
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tkOp && (p.peek().val == "==" || p.peek().val == "!=") {
		op := p.peek().val
		p.next()
		right, err := p.parseComparison(depth + 1)
		if err != nil {
			return nil, err
		}
		left = Binary{Op: op, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseComparison(depth int) (Expr, error) {
	left, err := p.parseAdditive(depth + 1)
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tkOp && (p.peek().val == "<" || p.peek().val == "<=" || p.peek().val == ">" || p.peek().val == ">=") {
		op := p.peek().val
		p.next()
		right, err := p.parseAdditive(depth + 1)
		if err != nil {
			return nil, err
		}
		left = Binary{Op: op, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseAdditive(depth int) (Expr, error) {
	left, err := p.parseMultiplicative(depth + 1)
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tkPunct && (p.peek().val == "+" || p.peek().val == "-") {
		op := p.peek().val
		p.next()
		right, err := p.parseMultiplicative(depth + 1)
		if err != nil {
			return nil, err
		}
		left = Binary{Op: op, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseMultiplicative(depth int) (Expr, error) {
	left, err := p.parseUnary(depth + 1)
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tkPunct && (p.peek().val == "*" || p.peek().val == "/" || p.peek().val == "%") {
		op := p.peek().val
		p.next()
		right, err := p.parseUnary(depth + 1)
		if err != nil {
			return nil, err
		}
		left = Binary{Op: op, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseUnary(depth int) (Expr, error) {
	if p.peek().kind == tkPunct && p.peek().val == "-" {
		p.next()
		x, err := p.parseUnary(depth + 1)
		if err != nil {
			return nil, err
		}
		return Unary{Op: "-", X: x}, nil
	}
	if p.peek().kind == tkIdent && p.peek().val == "not" {
		p.next()
		x, err := p.parseUnary(depth + 1)
		if err != nil {
			return nil, err
		}
		return Unary{Op: "not", X: x}, nil
	}
	return p.parsePostfix(depth + 1)
}

func (p *parser) parsePostfix(depth int) (Expr, error) {
	e, err := p.parsePrimary(depth + 1)
	if err != nil {
		return nil, err
	}
	for {
		tok := p.peek()
		if tok.kind == tkPunct && tok.val == "." {
			p.next()
			if p.peek().kind != tkIdent {
				return nil, fmt.Errorf("formula: expected name after '.'")
			}
			name := p.peek().val
			p.next()
			if isForbiddenName(name) {
				return nil, fmt.Errorf("formula: access to %q is not allowed", name)
			}
			e = Member{Target: e, Name: name}
			continue
		}
		if tok.kind == tkPunct && tok.val == "(" {
			p.next()
			var args []Expr
			if !(p.peek().kind == tkPunct && p.peek().val == ")") {
				for {
					a, err := p.parseExpr(depth + 1)
					if err != nil {
						return nil, err
					}
					args = append(args, a)
					if p.peek().kind == tkPunct && p.peek().val == "," {
						p.next()
						continue
					}
					break
				}
			}
			if !(p.peek().kind == tkPunct && p.peek().val == ")") {
				return nil, fmt.Errorf("formula: expected ')'")
			}
			p.next()
			e = Call{Callee: e, Args: args}
			continue
		}
		if tok.kind == tkPunct && tok.val == "[" {
			p.next()
			idx, err := p.parseExpr(depth + 1)
			if err != nil {
				return nil, err
			}
			if !(p.peek().kind == tkPunct && p.peek().val == "]") {
				return nil, fmt.Errorf("formula: expected ']'")
			}
			p.next()
			// reject forbidden string-literal index
			if sl, ok := idx.(StringLit); ok && isForbiddenName(sl.Value) {
				return nil, fmt.Errorf("formula: access to %q is not allowed", sl.Value)
			}
			e = Index{Target: e, Index: idx}
			continue
		}
		break
	}
	return e, nil
}

func (p *parser) parsePrimary(depth int) (Expr, error) {
	tok := p.peek()
	switch tok.kind {
	case tkNumber:
		p.next()
		// parse number
		v, err := parseNumber(tok.val)
		if err != nil {
			return nil, err
		}
		return NumberLit{Value: v}, nil
	case tkString:
		p.next()
		return StringLit{Value: tok.val}, nil
	case tkIdent:
		p.next()
		switch tok.val {
		case "true":
			return BoolLit{Value: true}, nil
		case "false":
			return BoolLit{Value: false}, nil
		case "null":
			return NullLit{}, nil
		}
		return Ident{Name: tok.val}, nil
	case tkPunct:
		if tok.val == "(" {
			p.next()
			e, err := p.parseExpr(depth + 1)
			if err != nil {
				return nil, err
			}
			if !(p.peek().kind == tkPunct && p.peek().val == ")") {
				return nil, fmt.Errorf("formula: expected ')'")
			}
			p.next()
			return e, nil
		}
		if tok.val == "[" {
			p.next()
			var items []Expr
			if !(p.peek().kind == tkPunct && p.peek().val == "]") {
				for {
					e, err := p.parseExpr(depth + 1)
					if err != nil {
						return nil, err
					}
					items = append(items, e)
					if p.peek().kind == tkPunct && p.peek().val == "," {
						p.next()
						continue
					}
					break
				}
			}
			if !(p.peek().kind == tkPunct && p.peek().val == "]") {
				return nil, fmt.Errorf("formula: expected ']'")
			}
			p.next()
			return ListLit{Items: items}, nil
		}
		if tok.val == "{" {
			p.next()
			ml := MapLit{}
			if !(p.peek().kind == tkPunct && p.peek().val == "}") {
				for {
					// key
					var key string
					var keyExpr Expr
					if p.peek().kind == tkString {
						key = p.peek().val
						p.next()
					} else if p.peek().kind == tkIdent {
						key = p.peek().val
						p.next()
					} else {
						// computed key expression
						ke, err := p.parseExpr(depth + 1)
						if err != nil {
							return nil, err
						}
						keyExpr = ke
					}
					if !(p.peek().kind == tkPunct && p.peek().val == ":") {
						return nil, fmt.Errorf("formula: expected ':' in map")
					}
					p.next()
					v, err := p.parseExpr(depth + 1)
					if err != nil {
						return nil, err
					}
					if keyExpr == nil {
						ml.Keys = append(ml.Keys, key)
						ml.KeyExprs = append(ml.KeyExprs, nil)
					} else {
						ml.Keys = append(ml.Keys, "")
						ml.KeyExprs = append(ml.KeyExprs, keyExpr)
					}
					ml.Values = append(ml.Values, v)
					if p.peek().kind == tkPunct && p.peek().val == "," {
						p.next()
						continue
					}
					break
				}
			}
			if !(p.peek().kind == tkPunct && p.peek().val == "}") {
				return nil, fmt.Errorf("formula: expected '}'")
			}
			p.next()
			return ml, nil
		}
	}
	return nil, fmt.Errorf("formula: unexpected token %q", tok.val)
}

func (p *parser) peek() token {
	if p.pos >= len(p.toks) {
		return token{kind: tkEOF}
	}
	return p.toks[p.pos]
}

func (p *parser) next() {
	if p.pos < len(p.toks) {
		p.pos++
	}
}

func parseNumber(s string) (float64, error) {
	return parseFloat(s)
}

func isForbiddenName(name string) bool {
	switch name {
	case "__proto__", "constructor", "prototype":
		return true
	}
	return false
}
