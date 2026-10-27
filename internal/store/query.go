package store

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrParse is returned (wrapped in *ParseError) when a query string cannot be
// parsed. It is exposed so callers can errors.Is against it.
var ErrParse = errors.New("query: parse error")

// ErrCursor is returned when a cursor is missing, tampered with, or does not
// match the current query.
var ErrCursor = errors.New("query: invalid cursor")

// ParseError describes a malformed query. It never contains SQL text.
type ParseError struct {
	Message  string
	Position int
}

func (e *ParseError) Error() string {
	if e.Position >= 0 {
		return fmt.Sprintf("query: parse error at %d: %s", e.Position, e.Message)
	}
	return "query: parse error: " + e.Message
}

func (e *ParseError) Is(target error) bool { return target == ErrParse }

// CursorError describes a rejected cursor.
type CursorError struct {
	Message string
}

func (e *CursorError) Error() string        { return "query: invalid cursor: " + e.Message }
func (e *CursorError) Is(target error) bool { return target == ErrCursor }

// QueryRequest is the store-level seam. O3 (MCP) and REST both call
// Store.Query with this shape. Allowed == nil means "unrestricted" (GM only);
// a non-nil empty slice means "no results".
type QueryRequest struct {
	Query   string
	Filter  string
	Q       string
	Where   string
	Sort    string
	Desc    bool
	Limit   int
	Offset  int
	Cursor  string
	Allowed []string
}

// QueryResult is what Store.Query returns. It carries uids (in result order),
// the full stored records, the count (which equals len(UIDs)), and an opaque
// next cursor. There is no total-matched count.
type QueryResult struct {
	UIDs       []string
	Items      []NoteRecord
	Notes      []NoteRecord
	Count      int
	NextCursor string
	Cursor     string
}

// Query is the single compiled implementation of the ADR-0047 predicate
// language. It parses, compiles to SQL over the notes table, executes, sorts,
// and paginates with opaque cursors.
func (s *Store) Query(in any, opts ...any) (QueryResult, error) {
	req, err := normalizeQueryRequest(in, opts...)
	if err != nil {
		return QueryResult{}, err
	}
	// Pick the query string.
	qstr := strings.TrimSpace(req.Query)
	if qstr == "" {
		qstr = strings.TrimSpace(req.Filter)
	}
	if qstr == "" {
		qstr = strings.TrimSpace(req.Q)
	}
	if qstr == "" {
		qstr = strings.TrimSpace(req.Where)
	}

	// Parse (may be empty -> match all).
	var whereSQL string
	var args []any
	if qstr != "" {
		expr, perr := parseQuery(qstr)
		if perr != nil {
			return QueryResult{}, perr
		}
		w, a, cerr := compileExpr(expr)
		if cerr != nil {
			return QueryResult{}, cerr
		}
		whereSQL = w
		args = a
	}

	// Allowed set semantics.
	if req.Allowed != nil && len(req.Allowed) == 0 {
		return QueryResult{UIDs: []string{}, Items: []NoteRecord{}, Notes: []NoteRecord{}, Count: 0}, nil
	}

	sortField := strings.TrimSpace(req.Sort)
	if sortField == "" {
		sortField = "uid"
	}
	// Whitelist the sort field exactly like validPath does for column
	// paths: an unvalidated field would be interpolated into ORDER BY
	// and json_extract, which is an injection vector.
	if sortField != "uid" {
		if !validPath(sortField) {
			return QueryResult{}, &ParseError{Message: "invalid sort field " + sortField, Position: -1}
		}
	}
	desc := req.Desc

	// Build SQL.
	sqlStr := "SELECT uid, meta FROM notes"
	var clauses []string
	if whereSQL != "" {
		clauses = append(clauses, "("+whereSQL+")")
	}
	if req.Allowed != nil {
		placeholders := make([]string, len(req.Allowed))
		aargs := make([]any, len(req.Allowed))
		for i, u := range req.Allowed {
			placeholders[i] = "?"
			aargs[i] = u
		}
		clauses = append(clauses, "uid IN ("+strings.Join(placeholders, ",")+")")
		args = append(args, aargs...)
	}
	if len(clauses) > 0 {
		sqlStr += " WHERE " + strings.Join(clauses, " AND ")
	}
	// ORDER BY
	expr := "uid"
	if sortField != "" && sortField != "uid" {
		expr = "json_extract(meta, '$." + sortField + "')"
	}
	direction := "ASC"
	if desc {
		direction = "DESC"
	}
	tieDir := "ASC"
	if desc {
		tieDir = "DESC"
	}
	sqlStr += " ORDER BY " + expr + " " + direction + ", uid " + tieDir

	// NOTE: no SQL-level LIMIT is applied here. Every page scans and
	// materialises the full matching set in Go (cursor keyset filtering
	// and tie-breaking need the collected rows). Measured on the SRD
	// corpus (1690 notes): ~63-81 ms per call regardless of page size.
	// O5 must budget for this or push cursor predicates into SQL; do
	// not silently assume pagination is cheap.
	rows, err := s.db.Query(sqlStr, args...)
	if err != nil {
		return QueryResult{}, &ParseError{Message: "query execution failed", Position: -1}
	}
	defer rows.Close()

	type row struct {
		UID  string
		Meta map[string]any
		Rec  NoteRecord
	}
	var collected []row
	for rows.Next() {
		var uid, metaStr string
		if err := rows.Scan(&uid, &metaStr); err != nil {
			return QueryResult{}, &ParseError{Message: "query scan failed", Position: -1}
		}
		var m map[string]any
		if metaStr != "" {
			_ = json.Unmarshal([]byte(metaStr), &m)
		}
		if m == nil {
			m = map[string]any{}
		}
		rec := recordFromMeta(uid, m)
		collected = append(collected, row{UID: uid, Meta: m, Rec: rec})
	}
	if err := rows.Err(); err != nil {
		return QueryResult{}, &ParseError{Message: "query rows failed", Position: -1}
	}

	// Decode cursor.
	var cursorPayload *cursorPayload
	if strings.TrimSpace(req.Cursor) != "" {
		cp, err := s.decodeCursor(req.Cursor)
		if err != nil {
			return QueryResult{}, err
		}
		// Validate it matches this query.
		if cp.Sort != sortField || cp.Desc != req.Desc || cp.QueryHash != queryHash(qstr, sortField, req.Desc, req.Allowed) {
			return QueryResult{}, &CursorError{Message: "cursor does not match query"}
		}
		cursorPayload = cp
	}

	// Apply cursor (keyset).
	if cursorPayload != nil {
		var filtered []row
		for _, r := range collected {
			v := metaValue(r.Meta, sortField)
			c := compareAny(v, cursorPayload.Value)
			if c == 0 {
				c = strings.Compare(r.UID, cursorPayload.UID)
			}
			if desc {
				// descending: next page has values less than cursor, or equal uid less
				if c < 0 {
					filtered = append(filtered, r)
				}
			} else {
				if c > 0 {
					filtered = append(filtered, r)
				}
			}
		}
		collected = filtered
	}

	// Offset (legacy) support when no cursor.
	if cursorPayload == nil && req.Offset > 0 {
		if req.Offset >= len(collected) {
			collected = nil
		} else {
			collected = collected[req.Offset:]
		}
	}

	// Limit.
	limit := req.Limit
	if limit <= 0 {
		limit = 0
	}
	var page []row
	if limit > 0 && len(collected) > limit {
		page = collected[:limit]
	} else {
		page = collected
	}
	if page == nil {
		page = []row{}
	}

	// Build result.
	uids := make([]string, 0, len(page))
	items := make([]NoteRecord, 0, len(page))
	for _, r := range page {
		uids = append(uids, r.UID)
		items = append(items, r.Rec)
	}

	// Next cursor.
	var nextCursor string
	if limit > 0 && len(page) == limit && len(collected) > limit {
		last := page[len(page)-1]
		nextCursor = s.encodeCursor(cursorPayloadEncode{
			Sort:      sortField,
			Desc:      desc,
			Value:     metaValue(last.Meta, sortField),
			UID:       last.UID,
			QueryHash: queryHash(qstr, sortField, desc, req.Allowed),
		})
	}

	res := QueryResult{
		UIDs:       uids,
		Items:      items,
		Notes:      items,
		Count:      len(uids),
		NextCursor: nextCursor,
		Cursor:     nextCursor,
	}
	if res.UIDs == nil {
		res.UIDs = []string{}
	}
	if res.Items == nil {
		res.Items = []NoteRecord{}
	}
	if res.Notes == nil {
		res.Notes = []NoteRecord{}
	}
	return res, nil
}

// QueryUIDs is a convenience wrapper returning uids + cursor.
func (s *Store) QueryUIDs(in any, opts ...any) ([]string, string, error) {
	res, err := s.Query(in, opts...)
	if err != nil {
		return nil, "", err
	}
	return res.UIDs, res.NextCursor, nil
}

// --- cursor encoding ---

type cursorPayload struct {
	Sort      string `json:"sort"`
	Desc      bool   `json:"desc"`
	Value     any    `json:"value"`
	UID       string `json:"uid"`
	QueryHash string `json:"qh"`
}

type cursorPayloadEncode struct {
	Sort      string
	Desc      bool
	Value     any
	UID       string
	QueryHash string
}

func (s *Store) encodeCursor(cp cursorPayloadEncode) string {
	p := cursorPayload{Sort: cp.Sort, Desc: cp.Desc, Value: cp.Value, UID: cp.UID, QueryHash: cp.QueryHash}
	b, _ := json.Marshal(p)
	mac := hmac.New(sha256.New, s.cursorKey)
	mac.Write(b)
	sig := mac.Sum(nil)
	out := append(b, sig...)
	return base64.URLEncoding.EncodeToString(out)
}

func (s *Store) decodeCursor(tok string) (*cursorPayload, error) {
	raw, err := base64.URLEncoding.DecodeString(tok)
	if err != nil || len(raw) < 32 {
		return nil, &CursorError{Message: "bad encoding"}
	}
	b, sig := raw[:len(raw)-32], raw[len(raw)-32:]
	mac := hmac.New(sha256.New, s.cursorKey)
	mac.Write(b)
	if !hmac.Equal(mac.Sum(nil), sig) {
		return nil, &CursorError{Message: "bad signature"}
	}
	var cp cursorPayload
	if err := json.Unmarshal(b, &cp); err != nil {
		return nil, &CursorError{Message: "bad payload"}
	}
	return &cp, nil
}

func queryHash(q, sort string, desc bool, allowed []string) string {
	h := sha256.New()
	h.Write([]byte(q))
	h.Write([]byte(sort))
	if desc {
		h.Write([]byte("desc"))
	} else {
		h.Write([]byte("asc"))
	}
	for _, u := range allowed {
		h.Write([]byte(u))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// --- meta helpers ---

func recordFromMeta(uid string, m map[string]any) NoteRecord {
	rec := NoteRecord{UID: uid, Meta: m}
	if v, ok := m["path"].(string); ok {
		rec.Path = v
	}
	if v, ok := m["title"].(string); ok {
		rec.Title = v
	}
	if v, ok := m["name"].(string); ok {
		rec.Name = v
	}
	if v, ok := m["type"].(string); ok {
		rec.Type = v
	}
	if v, ok := m["system"].(string); ok {
		rec.System = v
	}
	if v, ok := m["campaign"].(string); ok {
		rec.Campaign = v
	}
	if v, ok := m["kind"].(string); ok {
		rec.Kind = v
	}
	if v, ok := m["player"].(string); ok {
		rec.Player = v
	}
	if v, ok := m["workflow"].(string); ok {
		rec.Workflow = v
	}
	if v, ok := m["visibility"].(string); ok {
		rec.Visibility = v
	}
	return rec
}

func metaValue(m map[string]any, path string) any {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	// Support universal field names directly.
	if v, ok := m[path]; ok && !strings.Contains(path, ".") {
		return v
	}
	// data.<path> and arbitrary nested.
	parts := strings.Split(path, ".")
	cur := any(m)
	for _, p := range parts {
		switch c := cur.(type) {
		case map[string]any:
			v, ok := c[p]
			if !ok {
				return nil
			}
			cur = v
		default:
			return nil
		}
	}
	return cur
}

func compareAny(a, b any) int {
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
	// bools
	ba, oka := a.(bool)
	bb, okb := b.(bool)
	if oka && okb {
		if ba == bb {
			return 0
		}
		if !ba {
			return -1
		}
		return 1
	}
	// strings / fallback
	sa := strings.ToLower(fmt.Sprintf("%v", a))
	sb := strings.ToLower(fmt.Sprintf("%v", b))
	if a == nil {
		sa = ""
	}
	if b == nil {
		sb = ""
	}
	if sa < sb {
		return -1
	}
	if sa > sb {
		return 1
	}
	return 0
}

// --- parser ---

type qToken struct {
	kind string // ident, str, num, op, punct, eof
	val  string
	pos  int
}

type parser struct {
	toks []qToken
	i    int
}

func parseQuery(s string) (Expr, *ParseError) {
	toks, err := tokenizeQuery(s)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	e, perr := p.parseExpr()
	if perr != nil {
		return nil, perr
	}
	if p.peek().kind != "eof" {
		return nil, &ParseError{Message: "unexpected trailing token " + p.peek().val, Position: p.peek().pos}
	}
	return e, nil
}

func tokenizeQuery(s string) ([]qToken, *ParseError) {
	var out []qToken
	i := 0
	n := len(s)
	for i < n {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		pos := i
		// strings
		if c == '"' || c == '\'' {
			quote := c
			i++
			var b strings.Builder
			for i < n && s[i] != quote {
				if s[i] == '\\' {
					i++
					if i < n {
						b.WriteByte(s[i])
						i++
					}
					continue
				}
				b.WriteByte(s[i])
				i++
			}
			if i >= n {
				return nil, &ParseError{Message: "unterminated string", Position: pos}
			}
			i++ // closing quote
			out = append(out, qToken{kind: "str", val: b.String(), pos: pos})
			continue
		}
		// numbers
		if (c >= '0' && c <= '9') || (c == '-' && i+1 < n && s[i+1] >= '0' && s[i+1] <= '9') {
			start := i
			if c == '-' {
				i++
			}
			for i < n && ((s[i] >= '0' && s[i] <= '9') || s[i] == '.') {
				i++
			}
			out = append(out, qToken{kind: "num", val: s[start:i], pos: pos})
			continue
		}
		// operators multi-char
		if i+1 < n {
			two := s[i : i+2]
			if two == "==" || two == "!=" || two == ">=" || two == "<=" {
				out = append(out, qToken{kind: "op", val: two, pos: pos})
				i += 2
				continue
			}
		}
		if c == '>' || c == '<' || c == '=' || c == ':' {
			// '=' or ':' treated as equality alias
			if c == '=' {
				out = append(out, qToken{kind: "op", val: "==", pos: pos})
			} else if c == ':' {
				out = append(out, qToken{kind: "op", val: "==", pos: pos})
			} else {
				out = append(out, qToken{kind: "op", val: string(c), pos: pos})
			}
			i++
			continue
		}
		if c == '(' || c == ')' || c == ',' || c == '[' || c == ']' {
			out = append(out, qToken{kind: "punct", val: string(c), pos: pos})
			i++
			continue
		}
		// identifiers / keywords
		if isIdentStart(c) {
			start := i
			for i < n && isIdentPart(s[i]) {
				i++
			}
			word := s[start:i]
			lw := strings.ToLower(word)
			if lw == "and" || lw == "or" || lw == "not" || lw == "contains" || lw == "in" {
				out = append(out, qToken{kind: "kw", val: lw, pos: pos})
			} else {
				out = append(out, qToken{kind: "ident", val: word, pos: pos})
			}
			continue
		}
		return nil, &ParseError{Message: "unexpected character " + string(c), Position: pos}
	}
	out = append(out, qToken{kind: "eof", val: "", pos: n})
	return out, nil
}

func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == '.'
}
func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9') || c == '-'
}

// grammar
func (p *parser) peek() qToken {
	if p.i < len(p.toks) {
		return p.toks[p.i]
	}
	return qToken{kind: "eof"}
}
func (p *parser) next() qToken {
	t := p.peek()
	p.i++
	return t
}

func (p *parser) parseExpr() (Expr, *ParseError) { return p.parseOr() }

func (p *parser) parseOr() (Expr, *ParseError) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == "kw" && p.peek().val == "or" {
		p.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &binaryExpr{Op: "or", Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (Expr, *ParseError) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == "kw" && p.peek().val == "and" {
		p.next()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &binaryExpr{Op: "and", Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseNot() (Expr, *ParseError) {
	if p.peek().kind == "kw" && p.peek().val == "not" {
		p.next()
		x, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &notExpr{X: x}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (Expr, *ParseError) {
	t := p.peek()
	if t.kind == "punct" && t.val == "(" {
		p.next()
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.peek().kind != "punct" || p.peek().val != ")" {
			return nil, &ParseError{Message: "expected )", Position: p.peek().pos}
		}
		p.next()
		return e, nil
	}
	// comparison: path op value
	if t.kind == "ident" {
		path := t.val
		p.next()
		// path may include dots already captured as ident? Our tokenizer allows dots in ident, so data.attributes.str is one ident token. Good.
		opTok := p.peek()
		var op string
		if opTok.kind == "op" {
			op = opTok.val
			p.next()
		} else if opTok.kind == "kw" && (opTok.val == "contains" || opTok.val == "in") {
			op = opTok.val
			p.next()
		} else {
			return nil, &ParseError{Message: "expected operator after " + path, Position: opTok.pos}
		}
		// value
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		// validate path
		if !validPath(path) {
			return nil, &ParseError{Message: "invalid path " + path, Position: t.pos}
		}
		return &compareExpr{Path: path, Op: op, Value: val}, nil
	}
	// bare boolean/null? not a valid standalone
	return nil, &ParseError{Message: "expected expression", Position: t.pos}
}

func (p *parser) parseValue() (any, *ParseError) {
	t := p.peek()
	// Unquoted multi-word values (e.g. "value 0") are consumed until a
	// logical operator or closing delimiter, for compatibility with the
	// pre-existing filter syntax.
	if t.kind == "ident" || t.kind == "num" || (t.kind == "kw" && t.val != "true" && t.val != "false" && t.val != "null" && t.val != "and" && t.val != "or" && t.val != "not" && t.val != "contains" && t.val != "in") {
		parts := []string{t.val}
		p.next()
		for {
			nt := p.peek()
			if nt.kind == "eof" || nt.kind == "punct" {
				break
			}
			if nt.kind == "kw" && (nt.val == "and" || nt.val == "or" || nt.val == "not" || nt.val == "contains" || nt.val == "in") {
				break
			}
			if nt.kind != "ident" && nt.kind != "num" && nt.kind != "kw" && nt.kind != "str" {
				break
			}
			parts = append(parts, nt.val)
			p.next()
		}
		joined := strings.Join(parts, " ")
		// try number
		if len(parts) == 1 {
			if strings.Contains(joined, ".") {
				if f, err := strconv.ParseFloat(joined, 64); err == nil {
					return f, nil
				}
			} else {
				if i, err := strconv.ParseInt(joined, 10, 64); err == nil {
					return i, nil
				}
			}
		}
		return joined, nil
	}
	switch t.kind {
	case "str":
		p.next()
		return t.val, nil
	case "num":
		p.next()
		// int vs float
		if strings.Contains(t.val, ".") {
			f, err := strconv.ParseFloat(t.val, 64)
			if err != nil {
				return nil, &ParseError{Message: "bad number", Position: t.pos}
			}
			return f, nil
		}
		i, err := strconv.ParseInt(t.val, 10, 64)
		if err != nil {
			// maybe float
			f, ferr := strconv.ParseFloat(t.val, 64)
			if ferr != nil {
				return nil, &ParseError{Message: "bad number", Position: t.pos}
			}
			return f, nil
		}
		return i, nil
	case "kw":
		if t.val == "true" {
			p.next()
			return true, nil
		}
		if t.val == "false" {
			p.next()
			return false, nil
		}
		if t.val == "null" {
			p.next()
			return nil, nil
		}
	case "punct":
		if t.val == "[" {
			p.next()
			var list []any
			for !(p.peek().kind == "punct" && p.peek().val == "]") {
				v, err := p.parseValue()
				if err != nil {
					return nil, err
				}
				list = append(list, v)
				if p.peek().kind == "punct" && p.peek().val == "," {
					p.next()
					continue
				}
				break
			}
			if p.peek().kind != "punct" || p.peek().val != "]" {
				return nil, &ParseError{Message: "expected ]", Position: p.peek().pos}
			}
			p.next()
			return list, nil
		}
		if t.val == "(" {
			// allow parenthesized value list for `in`
			p.next()
			var list []any
			v, err := p.parseValue()
			if err != nil {
				return nil, err
			}
			list = append(list, v)
			for p.peek().kind == "punct" && p.peek().val == "," {
				p.next()
				v2, err := p.parseValue()
				if err != nil {
					return nil, err
				}
				list = append(list, v2)
			}
			if p.peek().kind != "punct" || p.peek().val != ")" {
				return nil, &ParseError{Message: "expected )", Position: p.peek().pos}
			}
			p.next()
			return list, nil
		}
	}
	return nil, &ParseError{Message: "expected value", Position: t.pos}
}

// AST
type Expr interface{ isExpr() }
type binaryExpr struct {
	Op    string
	Left  Expr
	Right Expr
}

func (binaryExpr) isExpr() {}

type notExpr struct{ X Expr }

func (notExpr) isExpr() {}

type compareExpr struct {
	Path  string
	Op    string
	Value any
}

func (compareExpr) isExpr() {}

func validPath(path string) bool {
	for _, seg := range strings.Split(path, ".") {
		if seg == "" {
			return false
		}
		if !isIdentStart(byte(seg[0])) && !(seg[0] >= '0' && seg[0] <= '9') {
			// allow first char letter/underscore
			if !(seg[0] == '_') {
				return false
			}
		}
		for i := 0; i < len(seg); i++ {
			c := seg[i]
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
				return false
			}
		}
	}
	return true
}

// compile
func compileExpr(e Expr) (string, []any, error) {
	switch v := e.(type) {
	case *binaryExpr:
		l, la, err := compileExpr(v.Left)
		if err != nil {
			return "", nil, err
		}
		r, ra, err := compileExpr(v.Right)
		if err != nil {
			return "", nil, err
		}
		op := strings.ToLower(v.Op)
		if op == "and" {
			return "(" + l + " AND " + r + ")", append(la, ra...), nil
		}
		if op == "or" {
			return "(" + l + " OR " + r + ")", append(la, ra...), nil
		}
		return "", nil, &ParseError{Message: "unknown binary op " + v.Op, Position: -1}
	case *notExpr:
		x, xa, err := compileExpr(v.X)
		if err != nil {
			return "", nil, err
		}
		return "(NOT " + x + ")", xa, nil
	case *compareExpr:
		return compileCompare(v)
	}
	return "", nil, &ParseError{Message: "unknown expression", Position: -1}
}

func compileCompare(c *compareExpr) (string, []any, error) {
	col := "json_extract(meta, '$." + c.Path + "')"
	// Actually path uses dots; JSON path needs $.a.b (already dots). Good.
	// But if path contains dots, JSON path is $.data.attributes.str. Good.
	op := strings.ToLower(c.Op)
	switch op {
	case "==", "=":
		return "(LOWER(CAST(" + col + " AS TEXT)) = LOWER(CAST(? AS TEXT)))", []any{c.Value}, nil
	case "!=":
		return "(LOWER(CAST(" + col + " AS TEXT)) <> LOWER(CAST(? AS TEXT)))", []any{c.Value}, nil
	case ">":
		return "(" + col + " > ?)", []any{c.Value}, nil
	case ">=":
		return "(" + col + " >= ?)", []any{c.Value}, nil
	case "<":
		return "(" + col + " < ?)", []any{c.Value}, nil
	case "<=":
		return "(" + col + " <= ?)", []any{c.Value}, nil
	case "contains":
		// list membership if array, else substring (case-insensitive)
		sql := "(CASE WHEN json_type(meta, '$." + c.Path + "') = 'array' " +
			"THEN EXISTS (SELECT 1 FROM json_each(" + col + ") WHERE LOWER(CAST(value AS TEXT)) = LOWER(CAST(? AS TEXT))) " +
			"ELSE LOWER(CAST(COALESCE(" + col + ",'') AS TEXT)) LIKE '%' || LOWER(CAST(? AS TEXT)) || '%' END)"
		return sql, []any{c.Value, c.Value}, nil
	case "in":
		list, ok := c.Value.([]any)
		if !ok {
			return "", nil, &ParseError{Message: "in expects a list", Position: -1}
		}
		if len(list) == 0 {
			return "(0)", nil, nil
		}
		placeholders := make([]string, len(list))
		args := make([]any, len(list))
		for i, v := range list {
			placeholders[i] = "LOWER(CAST(? AS TEXT))"
			args[i] = v
		}
		return "(LOWER(CAST(" + col + " AS TEXT)) IN (" + strings.Join(placeholders, ",") + "))", args, nil
	}
	return "", nil, &ParseError{Message: "unknown operator " + c.Op, Position: -1}
}

func normalizeQueryRequest(in any, opts ...any) (QueryRequest, error) {
	var req QueryRequest
	// handle context.Context first
	args := []any{in}
	args = append(args, opts...)
	for _, a := range args {
		switch v := a.(type) {
		case nil:
		case QueryRequest:
			mergeReq(&req, v)
		case *QueryRequest:
			if v != nil {
				mergeReq(&req, *v)
			}
		case string:
			if strings.TrimSpace(v) != "" && req.Query == "" && req.Filter == "" && req.Q == "" && req.Where == "" {
				req.Query = v
			}
		case map[string]any:
			if x, ok := v["query"].(string); ok {
				req.Query = x
			}
			if x, ok := v["filter"].(string); ok {
				req.Filter = x
			}
			if x, ok := v["q"].(string); ok {
				req.Q = x
			}
			if x, ok := v["where"].(string); ok {
				req.Where = x
			}
			if x, ok := v["sort"].(string); ok {
				req.Sort = x
			}
			if x, ok := v["desc"].(bool); ok {
				req.Desc = x
			}
			if x, ok := v["limit"].(int); ok {
				req.Limit = x
			}
			if x, ok := v["offset"].(int); ok {
				req.Offset = x
			}
			if x, ok := v["cursor"].(string); ok {
				req.Cursor = x
			}
			if x, ok := v["allowed"].([]string); ok {
				req.Allowed = x
			}
		}
	}
	return req, nil
}

func mergeReq(dst *QueryRequest, src QueryRequest) {
	if src.Query != "" {
		dst.Query = src.Query
	}
	if src.Filter != "" {
		dst.Filter = src.Filter
	}
	if src.Q != "" {
		dst.Q = src.Q
	}
	if src.Where != "" {
		dst.Where = src.Where
	}
	if src.Sort != "" {
		dst.Sort = src.Sort
	}
	if src.Desc {
		dst.Desc = true
	}
	if src.Limit != 0 {
		dst.Limit = src.Limit
	}
	if src.Offset != 0 {
		dst.Offset = src.Offset
	}
	if src.Cursor != "" {
		dst.Cursor = src.Cursor
	}
	if src.Allowed != nil {
		dst.Allowed = src.Allowed
	}
}
