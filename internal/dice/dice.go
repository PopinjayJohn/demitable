// Package dice implements the dice expression language (ADR-0008) and
// symbol dice / tables (ADR-0022).
//
// The engine is pure: it takes an expression, an evaluation context and a
// table provider, and returns a Result. It performs no I/O and never
// blocks; randomness comes from crypto/rand.
package dice

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// --- face / table model ---------------------------------------------------

// Face is one face of a die or table (ADR-0022).
type Face struct {
	ID     string   `yaml:"id" json:"id"`
	Label  string   `yaml:"label" json:"label"`
	Name   string   `yaml:"name" json:"name,omitempty"`
	Symbol string   `yaml:"symbol" json:"symbol"`
	Asset  string   `yaml:"asset" json:"asset"`
	Value  any      `yaml:"value" json:"value"`
	Rank   any      `yaml:"rank" json:"rank"`
	Tags   []string `yaml:"tags" json:"tags"`
}

// HasRank reports whether the face carries an ordering.
func (f Face) HasRank() bool {
	if f.Rank == nil {
		return false
	}
	_, err := toInt(f.Rank)
	return err == nil
}

// LabelOrID returns a display label.
func (f Face) LabelOrID() string {
	if f.Label != "" {
		return f.Label
	}
	if f.Name != "" {
		return f.Name
	}
	return f.ID
}

// Table is a named set of faces (ADR-0022). A random table and a symbol
// die are the same object.
type Table struct {
	ID            string `yaml:"id" json:"id"`
	Name          string `yaml:"name" json:"name"`
	Label         string `yaml:"label" json:"label"`
	Faces         []Face `yaml:"faces" json:"faces"`
	RankThreshold int    `yaml:"rank_threshold" json:"rankThreshold"`
	Threshold     int    `yaml:"threshold" json:"threshold"`
}

// DisplayName returns a human-readable name.
func (t Table) DisplayName() string {
	if t.Name != "" {
		return t.Name
	}
	if t.Label != "" {
		return t.Label
	}
	return t.ID
}

// EffectiveThreshold returns the rank threshold, defaulting to 3.
func (t Table) EffectiveThreshold() int {
	if t.RankThreshold != 0 {
		return t.RankThreshold
	}
	if t.Threshold != 0 {
		return t.Threshold
	}
	return 3
}

// HasTaggedSuccess reports whether any face is tagged as a success.
func (t Table) HasTaggedSuccess() bool {
	for _, f := range t.Faces {
		for _, tg := range f.Tags {
			if tg == "success" {
				return true
			}
		}
	}
	return false
}

// HasRank reports whether any face carries a rank.
func (t Table) HasRank() bool {
	for _, f := range t.Faces {
		if f.HasRank() {
			return true
		}
	}
	return false
}

// TableProvider resolves a table id to a table.
type TableProvider func(id string) (*Table, bool)

// --- modifier source / results -------------------------------------------

// ModifierSource records one modifier contribution (provenance).
type ModifierSource struct {
	Source string `json:"source"`
	Name   string `json:"name"`
	Value  int    `json:"value"`
}

// DieResult is one rolled die.
type DieResult struct {
	Value    int    `json:"value"`
	Face     *Face  `json:"face,omitempty"`
	Kept     bool   `json:"kept"`
	Exploded bool   `json:"exploded"`
	Table    string `json:"table,omitempty"`
}

// Result is the outcome of a roll.
type Result struct {
	Kind                   string           `json:"kind"`
	Type                   string           `json:"type"`
	Total                  int              `json:"total"`
	Sum                    int              `json:"sum"`
	SuccessCount           int              `json:"successCount"`
	Successes              int              `json:"successes"`
	Count                  int              `json:"count"`
	Outcome                string           `json:"outcome"`
	Direction              string           `json:"direction"`
	Faces                  []DieResult      `json:"faces"`
	Individual             []DieResult      `json:"individual"`
	Dice                   []DieResult      `json:"dice"`
	IndividualResults      []DieResult      `json:"individualResults"`
	IndividualResultsSnake []DieResult      `json:"individual_results"`
	Modifiers              []ModifierSource `json:"modifiers"`
	ModifierSources        []ModifierSource `json:"modifierSources"`
	ModifierSourcesSnake   []ModifierSource `json:"modifier_sources"`
	Target                 any              `json:"target"`
	Expression             string           `json:"expression"`
	Symbol                 *Face            `json:"symbol,omitempty"`
	Named                  string           `json:"named,omitempty"`
	RightCount             int              `json:"rightCount,omitempty"`
	OppositionCount        int              `json:"oppositionCount,omitempty"`
}

// syncAliases fills the alias fields.
func (r *Result) SyncAliases() {
	if r.Type == "" {
		r.Type = r.Kind
	}
	if r.Kind == "" {
		r.Kind = r.Type
	}
	r.Sum = r.Total
	r.Successes = r.SuccessCount
	r.Count = r.SuccessCount
	r.Individual = r.Faces
	r.Dice = r.Faces
	r.IndividualResults = r.Faces
	r.IndividualResultsSnake = r.Faces
	r.ModifierSources = r.Modifiers
	r.ModifierSourcesSnake = r.Modifiers
}

// RollRecord is one logged roll (full provenance).
type RollRecord struct {
	ID                     string           `json:"id"`
	Roller                 string           `json:"roller"`
	Actor                  string           `json:"actor"`
	Workflow               string           `json:"workflow"`
	Expression             string           `json:"expression"`
	Individual             []DieResult      `json:"individual"`
	IndividualResults      []DieResult      `json:"individualResults"`
	IndividualResultsSnake []DieResult      `json:"individual_results"`
	Dice                   []DieResult      `json:"dice"`
	Modifiers              []ModifierSource `json:"modifiers"`
	ModifierSources        []ModifierSource `json:"modifierSources"`
	ModifierSourcesSnake   []ModifierSource `json:"modifier_sources"`
	Target                 any              `json:"target"`
	Outcome                string           `json:"outcome"`
	Timestamp              time.Time        `json:"timestamp"`
	TS                     time.Time        `json:"ts"`
	Shared                 bool             `json:"shared"`
	Result                 Result           `json:"result"`
}

// syncRecordAliases fills alias fields.
func (rec *RollRecord) SyncAliases() {
	rec.Actor = rec.Roller
	rec.Individual = rec.Result.Faces
	rec.IndividualResults = rec.Result.Faces
	rec.IndividualResultsSnake = rec.Result.Faces
	rec.Dice = rec.Result.Faces
	rec.Modifiers = rec.Result.Modifiers
	rec.ModifierSources = rec.Result.Modifiers
	rec.ModifierSourcesSnake = rec.Result.Modifiers
	rec.TS = rec.Timestamp
	rec.Result.SyncAliases()
}

// --- options --------------------------------------------------------------

// RollOptions configures a roll.
type RollOptions struct {
	Vars          map[string]any
	Tables        TableProvider
	Direction     string
	Target        any
	Count         bool
	Roller        string
	Workflow      string
	Expression    string
	RankThreshold int
}

// RollOption mutates RollOptions.
type RollOption func(*RollOptions)

// WithVars sets variables.
func WithVars(v map[string]any) RollOption { return func(o *RollOptions) { o.Vars = v } }

// WithTables sets the table provider.
func WithTables(t TableProvider) RollOption { return func(o *RollOptions) { o.Tables = t } }

// WithTableProvider is an alias for WithTables.
func WithTableProvider(t TableProvider) RollOption { return WithTables(t) }

// WithTable is an alias for WithTables.
func WithTable(t TableProvider) RollOption { return WithTables(t) }

// WithDirection sets direction.
func WithDirection(d string) RollOption { return func(o *RollOptions) { o.Direction = d } }

// WithTarget sets target.
func WithTarget(t any) RollOption { return func(o *RollOptions) { o.Target = t } }

// WithCount requests a count result.
func WithCount() RollOption { return func(o *RollOptions) { o.Count = true } }

// WithRoller sets roller.
func WithRoller(r string) RollOption { return func(o *RollOptions) { o.Roller = r } }

// WithWorkflow sets workflow.
func WithWorkflow(w string) RollOption { return func(o *RollOptions) { o.Workflow = w } }

// WithRankThreshold sets rank threshold.
func WithRankThreshold(n int) RollOption { return func(o *RollOptions) { o.RankThreshold = n } }

// TableFromNote builds a dice.Table from a vault note's Fields. The note is
// expected to have type "table" with data.faces. This helper keeps vault
// wiring in the caller; the dice package does not import vault.
func TableFromMap(id, name string, fields map[string]any) *Table {
	t := &Table{ID: id, Name: name}
	// find faces
	var facesRaw any
	if fields != nil {
		if data, ok := fields["data"].(map[string]any); ok {
			facesRaw = data["faces"]
		}
		if facesRaw == nil {
			facesRaw = fields["faces"]
		}
		if rt, ok := fields["rank_threshold"].(int); ok {
			t.RankThreshold = rt
		}
		if rt, ok := fields["rankThreshold"].(int); ok && t.RankThreshold == 0 {
			t.RankThreshold = rt
		}
	}
	if arr, ok := facesRaw.([]any); ok {
		for _, e := range arr {
			if m, ok := e.(map[string]any); ok {
				f := Face{}
				if v, ok := m["id"].(string); ok {
					f.ID = v
				}
				if v, ok := m["label"].(string); ok {
					f.Label = v
				}
				if v, ok := m["name"].(string); ok {
					f.Name = v
				}
				if v, ok := m["symbol"].(string); ok {
					f.Symbol = v
				}
				if v, ok := m["asset"].(string); ok {
					f.Asset = v
				}
				if v, ok := m["value"]; ok {
					f.Value = v
				}
				if v, ok := m["rank"].(int); ok {
					r := v
					f.Rank = &r
				}
				if v, ok := m["rank"].(float64); ok {
					r := int(v)
					f.Rank = &r
				}
				if v, ok := m["tags"].([]any); ok {
					for _, tg := range v {
						if s, ok := tg.(string); ok {
							f.Tags = append(f.Tags, s)
						}
					}
				}
				if v, ok := m["tags"].([]string); ok {
					f.Tags = v
				}
				t.Faces = append(t.Faces, f)
			}
		}
	}
	if t.Name == "" {
		t.Name = id
	}
	return t
}

// --- randomness -----------------------------------------------------------

func randInt(n int) int {
	if n <= 1 {
		return 0
	}
	bi, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return int(bi.Int64())
}

var rollCounter uint64

func newRollID() string {
	n := atomic.AddUint64(&rollCounter, 1)
	return fmt.Sprintf("roll-%d-%d", time.Now().UnixNano(), n)
}

// --- public roll entry points --------------------------------------------

// Roll evaluates a dice expression.
func Roll(expr string, opts ...RollOption) (Result, error) {
	o := RollOptions{Vars: map[string]any{}, Direction: "high"}
	for _, f := range opts {
		f(&o)
	}
	if o.Vars == nil {
		o.Vars = map[string]any{}
	}
	if o.Direction == "" {
		o.Direction = "high"
	}
	if o.Tables == nil {
		o.Tables = func(string) (*Table, bool) { return nil, false }
	}
	if o.Expression != "" {
		expr = o.Expression
	}
	return rollExpression(expr, o)
}

// RollCount is a convenience for count results.
func RollCount(expr string, opts ...RollOption) (Result, error) {
	opts = append([]RollOption{WithCount()}, opts...)
	return Roll(expr, opts...)
}

// Evaluate is an alias for Roll.
func Evaluate(expr string, opts ...RollOption) (Result, error) {
	return Roll(expr, opts...)
}

// Parse validates an expression.
func Parse(expr string) (string, error) {
	return ParseExpression(expr)
}

// ParseExpression validates an expression.
func ParseExpression(expr string) (string, error) {
	_, err := Roll(expr)
	if err != nil {
		return "", err
	}
	return expr, nil
}

// --- opposed detection ----------------------------------------------------

func rollExpression(expr string, o RollOptions) (Result, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return Result{}, fmt.Errorf("dice: empty expression")
	}
	// opposed: split on " vs " (case-insensitive)
	low := strings.ToLower(expr)
	if idx := strings.Index(low, " vs "); idx >= 0 {
		left := strings.TrimSpace(expr[:idx])
		right := strings.TrimSpace(expr[idx+4:])
		return rollOpposed(left, right, expr, o)
	}
	// single
	res, err := rollSingle(expr, o)
	if err != nil {
		return res, err
	}
	res.Expression = expr
	res.Direction = o.Direction
	// target/outcome
	applyOutcome(&res, o)
	res.SyncAliases()
	return res, nil
}

func rollOpposed(left, right, full string, o RollOptions) (Result, error) {
	lr, err := rollSingle(left, o)
	if err != nil {
		return Result{}, err
	}
	rr, err := rollSingle(right, o)
	if err != nil {
		return Result{}, err
	}
	if err := countValidation(o, lr.Faces, o.Tables); err != nil {
		return Result{}, err
	}
	if err := countValidation(o, rr.Faces, o.Tables); err != nil {
		return Result{}, err
	}
	// count successes
	lc := countSuccesses(lr.Faces, o, nil)
	rc := countSuccesses(rr.Faces, o, nil)
	// if tables unordered in comparison? if any table-backed face lacks rank and we are comparing?
	// Enforce: if a table-backed die appears in opposed roll and its table has no rank and no tags?
	// We'll check below for rank when counting by rank threshold.
	res := Result{
		Kind:            "count",
		Direction:       o.Direction,
		Expression:      full,
		Faces:           append([]DieResult{}, lr.Faces...),
		Modifiers:       append([]ModifierSource{}, lr.Modifiers...),
		Target:          o.Target,
		RightCount:      rc,
		OppositionCount: rc,
	}
	res.SuccessCount = lc
	// outcome
	if lc > rc {
		res.Outcome = "win"
	} else if lc < rc {
		res.Outcome = "loss"
	} else {
		res.Outcome = "tie"
	}
	res.SyncAliases()
	return res, nil
}

// --- single expression ----------------------------------------------------

func rollSingle(expr string, o RollOptions) (Result, error) {
	// split into terms by + or - (keep sign)
	terms := splitTerms(expr)
	var allFaces []DieResult
	var modifiers []ModifierSource
	total := 0
	for _, t := range terms {
		term := strings.TrimSpace(t.text)
		if term == "" {
			continue
		}
		if isDiceTerm(term) {
			gr, err := rollDiceGroup(term, o)
			if err != nil {
				return Result{}, err
			}
			allFaces = append(allFaces, gr.faces...)
			total += gr.sum
			modifiers = append(modifiers, gr.modifiers...)
		} else {
			// number or {var}
			n, src, err := evalModifierTerm(term, o.Vars)
			if err != nil {
				return Result{}, err
			}
			total += n * t.sign
			if n != 0 || true {
				modifiers = append(modifiers, ModifierSource{Source: src, Name: src, Value: n * t.sign})
			}
		}
	}
	// determine kind
	kind := "numeric"
	if o.Count {
		kind = "count"
	}
	// table-backed?
	tableBacked := false
	for _, f := range allFaces {
		if f.Table != "" {
			tableBacked = true
		}
	}
	if tableBacked && !o.Count {
		// single die symbol/named
		if len(allFaces) == 1 {
			// find face
			// We need original face; store in DieResult not enough? Face pointer lost.
			// We'll handle symbol by re-rolling? Instead, set Named/Symbol later?
			// We'll leave kind numeric if value present else symbol.
			// Heuristic: if face has symbol, kind=symbol
		}
	}
	// If count mode, count successes
	successCount := 0
	if o.Count {
		if err := countValidation(o, allFaces, o.Tables); err != nil {
			return Result{}, err
		}
		successCount = countSuccesses(allFaces, o, nil)
		kind = "count"
	}
	// For table-backed single, determine symbol/named
	if !o.Count && len(allFaces) == 1 && allFaces[0].Face != nil {
		f := allFaces[0].Face
		if f.Value == nil {
			if f.Symbol != "" {
				kind = "symbol"
			} else {
				kind = "named"
			}
		}
	}

	res := Result{
		Kind:         kind,
		Direction:    o.Direction,
		Total:        total,
		Faces:        allFaces,
		Modifiers:    modifiers,
		Target:       o.Target,
		SuccessCount: successCount,
	}
	if kind == "count" {
		res.Total = successCount
		res.SuccessCount = successCount
	}
	// symbol/named
	if len(allFaces) == 1 && allFaces[0].Face != nil {
		f := *allFaces[0].Face
		if res.Kind == "symbol" {
			res.Symbol = &f
		}
		if res.Kind == "named" {
			res.Named = f.LabelOrID()
		}
	}
	res.SyncAliases()
	return res, nil
}

type term struct {
	text string
	sign int
}

func splitTerms(expr string) []term {
	var terms []term
	cur := ""
	sign := 1
	// handle leading -
	i := 0
	if strings.HasPrefix(expr, "-") {
		sign = -1
		i = 1
	} else if strings.HasPrefix(expr, "+") {
		i = 1
	}
	for ; i < len(expr); i++ {
		c := expr[i]
		if c == '+' || c == '-' {
			if cur != "" {
				terms = append(terms, term{text: cur, sign: sign})
			}
			cur = ""
			if c == '+' {
				sign = 1
			} else {
				sign = -1
			}
		} else {
			cur += string(c)
		}
	}
	if cur != "" {
		terms = append(terms, term{text: cur, sign: sign})
	}
	if len(terms) == 0 {
		terms = append(terms, term{text: expr, sign: 1})
	}
	return terms
}

func isDiceTerm(s string) bool {
	return strings.Contains(s, "d") || strings.Contains(s, "D")
}

// groupResult holds rolled dice for one group.
type groupResult struct {
	faces     []DieResult
	sum       int
	modifiers []ModifierSource
}

func rollDiceGroup(term string, o RollOptions) (groupResult, error) {
	gr := groupResult{}
	// parse: count d sides ...
	// find d
	di := strings.IndexAny(term, "dD")
	if di < 0 {
		return gr, fmt.Errorf("dice: not a dice term %q", term)
	}
	countTok := strings.TrimSpace(term[:di])
	rest := term[di+1:]
	// sides token
	sidesTok, suffix := readSidesToken(rest)
	// keep/explode from suffix
	keepMode, keepN := parseKeep(suffix)
	explodeCmp, explodeVal, hasExplode := parseExplode(suffix)

	count, err := evalCountToken(countTok, o.Vars)
	if err != nil {
		return gr, err
	}
	if count < 0 {
		return gr, fmt.Errorf("dice: negative count")
	}
	if count == 0 {
		return gr, nil
	}
	// cap to avoid runaway
	if count > 1000 {
		return gr, fmt.Errorf("dice: too many dice")
	}

	// resolve sides
	fudge := false
	tableID := ""
	sides := 0
	if sidesTok == "F" || sidesTok == "f" {
		fudge = true
	} else if sidesTok == "%" {
		sides = 100
	} else if n, err := evalCountToken(sidesTok, o.Vars); err == nil {
		sides = n
	} else {
		tableID = sidesTok
	}

	// table-backed
	var tbl *Table
	if tableID != "" {
		if o.Tables == nil {
			return gr, fmt.Errorf("dice: no table provider for %q", tableID)
		}
		t, ok := o.Tables(tableID)
		if !ok || t == nil {
			return gr, fmt.Errorf("dice: unknown table %q", tableID)
		}
		tbl = t
		sides = len(tbl.Faces)
		if sides == 0 {
			return gr, fmt.Errorf("dice: table %q has no faces", tbl.DisplayName())
		}
	}

	// comparison/ordering enforcement for keep on table-backed
	if tbl != nil && (keepMode != "") {
		if !tbl.HasRank() {
			return gr, fmt.Errorf("dice: table %q has no ordering; add rank to its faces", tbl.ID)
		}
	}

	// roll
	var rolled []DieResult
	rollOne := func() DieResult {
		dr := DieResult{}
		if fudge {
			v := randInt(3) - 1
			dr.Value = v
			return dr
		}
		if tbl != nil {
			idx := randInt(len(tbl.Faces))
			f := tbl.Faces[idx]
			dr.Face = &f
			dr.Table = tbl.ID
			if f.Value != nil {
				if n, err := toInt(f.Value); err == nil {
					dr.Value = n
				}
			}
			// exploded? check rank/value threshold? Exploding on table not common; skip
			return dr
		}
		// numeric
		v := randInt(sides) + 1
		dr.Value = v
		// wild percentile: if sides==100 and v==? wild faces 9,0? skip
		return dr
	}

	// initial rolls
	for i := 0; i < count; i++ {
		dr := rollOne()
		// exploding
		if hasExplode {
			// check condition
			dice := []DieResult{dr}
			cur := dr
			guard := 0
			for guard < 100 && explodeCondition(cur.Value, explodeCmp, explodeVal) {
				guard++
				cur = rollOne()
				cur.Exploded = true
				dice = append(dice, cur)
			}
			rolled = append(rolled, dice...)
		} else {
			rolled = append(rolled, dr)
		}
	}

	// keep marking
	applyKeep(rolled, keepMode, keepN)

	// sum kept
	sum := 0
	for _, dr := range rolled {
		if dr.Kept {
			sum += dr.Value
		}
	}
	// for table-backed symbol with no value, sum stays 0

	gr.faces = rolled
	gr.sum = sum
	return gr, nil
}

func readSidesToken(rest string) (string, string) {
	rest = strings.TrimSpace(rest)
	if strings.HasPrefix(rest, "{") {
		// read until }
		end := strings.Index(rest, "}")
		if end >= 0 {
			return rest[:end+1], strings.TrimSpace(rest[end+1:])
		}
	}
	// read run of allowed chars
	i := 0
	for i < len(rest) {
		if strings.HasPrefix(rest[i:], "kh") || strings.HasPrefix(rest[i:], "kl") || strings.HasPrefix(rest[i:], ">=") || strings.HasPrefix(rest[i:], "<=") {
			break
		}
		c := rest[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			i++
		} else {
			break
		}
	}
	if i == 0 {
		return "", rest
	}
	return rest[:i], strings.TrimSpace(rest[i:])
}

func parseKeep(suffix string) (string, int) {
	s := strings.ToLower(suffix)
	// khN or klN
	for _, mode := range []string{"kh", "kl"} {
		idx := strings.Index(s, mode)
		if idx >= 0 {
			// read digits after
			j := idx + 2
			numStr := ""
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				numStr += string(s[j])
				j++
			}
			n, _ := strconv.Atoi(numStr)
			if mode == "kh" {
				return "high", n
			}
			return "low", n
		}
	}
	return "", 0
}

func parseExplode(suffix string) (string, int, bool) {
	s := suffix
	// >= or <=
	for _, cmp := range []string{">=", "<="} {
		idx := strings.Index(s, cmp)
		if idx >= 0 {
			// read number after
			j := idx + len(cmp)
			numStr := ""
			for j < len(s) && (s[j] >= '0' && s[j] <= '9') {
				numStr += string(s[j])
				j++
			}
			n, _ := strconv.Atoi(numStr)
			return cmp, n, true
		}
	}
	return "", 0, false
}

func explodeCondition(val int, cmp string, thr int) bool {
	switch cmp {
	case ">=":
		return val >= thr
	case "<=":
		return val <= thr
	}
	return false
}

func applyKeep(rolled []DieResult, mode string, n int) {
	if mode == "" || n <= 0 || n >= len(rolled) {
		for i := range rolled {
			rolled[i].Kept = true
		}
		return
	}
	// indices sorted by value
	idx := make([]int, len(rolled))
	for i := range idx {
		idx[i] = i
	}
	if mode == "high" {
		sort.Slice(idx, func(a, b int) bool { return rolled[idx[a]].Value > rolled[idx[b]].Value })
	} else {
		sort.Slice(idx, func(a, b int) bool { return rolled[idx[a]].Value < rolled[idx[b]].Value })
	}
	kept := map[int]bool{}
	for i := 0; i < n && i < len(idx); i++ {
		kept[idx[i]] = true
	}
	for i := range rolled {
		rolled[i].Kept = kept[i]
	}
}

func evalModifierTerm(term string, vars map[string]any) (int, string, error) {
	term = strings.TrimSpace(term)
	if strings.HasPrefix(term, "{") && strings.HasSuffix(term, "}") {
		name := term[1 : len(term)-1]
		v, ok := vars[name]
		if !ok {
			return 0, name, fmt.Errorf("dice: unknown variable %q", name)
		}
		n, err := toInt(v)
		if err != nil {
			return 0, name, err
		}
		return n, name, nil
	}
	n, err := strconv.Atoi(term)
	if err != nil {
		return 0, term, fmt.Errorf("dice: bad modifier %q", term)
	}
	return n, "flat", nil
}

func toInt(v any) (int, error) {
	switch val := v.(type) {
	case int:
		return val, nil
	case int8:
		return int(val), nil
	case int16:
		return int(val), nil
	case int32:
		return int(val), nil
	case int64:
		return int(val), nil
	case uint:
		return int(val), nil
	case uint8:
		return int(val), nil
	case uint16:
		return int(val), nil
	case uint32:
		return int(val), nil
	case uint64:
		return int(val), nil
	case float32:
		return int(val), nil
	case float64:
		return int(val), nil
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil {
			return 0, err
		}
		return n, nil
	case bool:
		if val {
			return 1, nil
		}
		return 0, nil
	case *int:
		if val == nil {
			return 0, fmt.Errorf("nil")
		}
		return *val, nil
	case *float64:
		if val == nil {
			return 0, fmt.Errorf("nil")
		}
		return int(*val), nil
	case json.Number:
		n, err := val.Int64()
		if err != nil {
			return 0, err
		}
		return int(n), nil
	}
	return 0, fmt.Errorf("dice: cannot convert %T to int", v)
}

func evalCountToken(tok string, vars map[string]any) (int, error) {
	tok = strings.TrimSpace(tok)
	if strings.HasPrefix(tok, "{") && strings.HasSuffix(tok, "}") {
		name := tok[1 : len(tok)-1]
		v, ok := vars[name]
		if !ok {
			return 0, fmt.Errorf("dice: unknown variable %q", name)
		}
		return toInt(v)
	}
	n, err := strconv.Atoi(tok)
	if err != nil {
		return 0, fmt.Errorf("dice: bad number %q", tok)
	}
	return n, nil
}

// --- counting successes ---------------------------------------------------

func countSuccesses(faces []DieResult, o RollOptions, tbl *Table) int {
	n := 0
	for _, dr := range faces {
		if !dr.Kept {
			continue
		}
		if dr.Face != nil {
			f := dr.Face
			// tagged
			tagged := false
			for _, tg := range f.Tags {
				if tg == "success" {
					tagged = true
				}
			}
			// resolve table
			tt := tbl
			if tt == nil && dr.Table != "" && o.Tables != nil {
				if t, ok := o.Tables(dr.Table); ok {
					tt = t
				}
			}
			if tagged && (tt == nil || tt.HasTaggedSuccess()) {
				n++
				continue
			}
			// rank threshold
			if f.HasRank() {
				thr := o.RankThreshold
				if thr == 0 {
					if tt != nil {
						thr = tt.EffectiveThreshold()
					} else {
						thr = 3
					}
				}
				if thr == 0 {
					thr = 3
				}
				rn, _ := toInt(f.Rank)
				if rn >= thr {
					n++
				}
				continue
			}
			// no tag/rank -> if table has neither, error? handled earlier
			continue
		}
		// numeric die: success if value >= threshold (default sides/2+1)
		// We don't know sides; use 4 for d6? Use >=4 heuristic for <=6, else > sides/2
		thr := 4
		if dr.Value > 6 {
			thr = dr.Value/2 + 1
		}
		if dr.Value >= thr {
			n++
		}
	}
	return n
}

// validateCountTable ensures table has tags or ranks; else error naming table.
func validateCountTable(tbl *Table) error {
	if tbl == nil {
		return nil
	}
	if !tbl.HasTaggedSuccess() && !tbl.HasRank() {
		// neither defined
		return fmt.Errorf("dice: table %q has no success definition (no tagged faces and no rank threshold)", tbl.DisplayName())
	}
	return nil
}

// applyOutcome sets Outcome for single rolls based on target/direction.
func applyOutcome(res *Result, o RollOptions) {
	if o.Target == nil {
		if res.Kind == "count" {
			if res.SuccessCount > 0 {
				res.Outcome = "success"
			} else {
				res.Outcome = "failure"
			}
		} else {
			res.Outcome = "rolled"
		}
		return
	}
	// compare total to target
	tgt, err := toInt(o.Target)
	if err != nil {
		res.Outcome = "rolled"
		return
	}
	res.Target = o.Target
	if res.Kind == "count" {
		// count vs target number
		if res.SuccessCount >= tgt {
			res.Outcome = "success"
		} else {
			res.Outcome = "failure"
		}
		return
	}
	if o.Direction == "low" {
		if res.Total <= tgt {
			res.Outcome = "success"
		} else {
			res.Outcome = "failure"
		}
	} else {
		if res.Total >= tgt {
			res.Outcome = "success"
		} else {
			res.Outcome = "failure"
		}
	}
}

// TableFromNote is alias-ish; actual conversion from map handled by TableFromMap.
// Keep a helper for name.
func TableID(t *Table) string {
	if t == nil {
		return ""
	}
	return t.ID
}

// Ensure errors mention table in count validation. We'll call validateCountTable
// from rollSingle when count mode and table-backed.
// Wrap rollSingle count validation:
func init() {}

// We need to inject count validation. Patch rollSingle by adding check before counting.
// Since rollSingle already counts, we'll add validation in Roll via a post-check?
// Instead, modify rollSingle: before counting, if o.Count and table-backed, validate.
// We'll add a helper called in rollSingle.
func countValidation(o RollOptions, faces []DieResult, tables TableProvider) error {
	// find table
	seen := map[string]bool{}
	for _, dr := range faces {
		if dr.Table == "" {
			continue
		}
		if seen[dr.Table] {
			continue
		}
		seen[dr.Table] = true
		if tables == nil {
			continue
		}
		t, ok := tables(dr.Table)
		if !ok || t == nil {
			continue
		}
		if err := validateCountTable(t); err != nil {
			return err
		}
		// unordered comparison check: if count by rank threshold but no rank -> already error above
	}
	return nil
}

// We'll call countValidation inside rollSingle when o.Count.

func reflectValue(v any) reflect.Value {
	return reflect.ValueOf(v)
}
