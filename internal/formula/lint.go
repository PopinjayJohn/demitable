package formula

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Issue is a lint finding.
type Issue struct {
	Severity string `json:"severity"`
	Kind     string `json:"kind"`
	Formula  string `json:"formula"`
	Name     string `json:"name"`
	Scope    string `json:"scope"`
	Context  string `json:"context"`
	Message  string `json:"message"`
}

// Context declares the bindings available in each scope layer for a declared
// evaluation context.
type Context struct {
	Name string

	Call      []string
	CallScope map[string]any
	CallNames []string

	Entity      []string
	EntityScope map[string]any
	EntityNames []string

	Pack      []string
	PackScope map[string]any
	PackNames []string
}

func (c Context) name() string {
	if c.Name != "" {
		return c.Name
	}
	return "default"
}

func (c Context) callNames() []string {
	out := append([]string{}, c.Call...)
	out = append(out, c.CallNames...)
	for k := range c.CallScope {
		out = append(out, k)
	}
	return dedupStr(out)
}

func (c Context) entityNames() []string {
	out := append([]string{}, c.Entity...)
	out = append(out, c.EntityNames...)
	for k := range c.EntityScope {
		out = append(out, k)
	}
	return dedupStr(out)
}

func (c Context) packNames() []string {
	out := append([]string{}, c.Pack...)
	out = append(out, c.PackNames...)
	for k := range c.PackScope {
		out = append(out, k)
	}
	return dedupStr(out)
}

func dedupStr(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func containsStr(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// --- argument parsing (flexible) ---

func parseArgs(args []any) (map[string]string, []Context, bool) {
	formulas := map[string]string{}
	var contexts []Context
	unrecognized := false
	for _, a := range args {
		switch v := a.(type) {
		case map[string]string:
			for k, val := range v {
				formulas[k] = val
			}
		case map[string]any:
			// could be formulas (string values) or a single context? treat as formulas if values strings
			isFormula := true
			for _, val := range v {
				if _, ok := val.(string); !ok {
					isFormula = false
					break
				}
			}
			if isFormula && len(v) > 0 {
				for k, val := range v {
					formulas[k] = val.(string)
				}
			} else {
				// try as context
				ctx, _ := anyToContext(v)
				contexts = append(contexts, ctx)
			}
		case []Context:
			contexts = append(contexts, v...)
		case Context:
			contexts = append(contexts, v)
		case []any:
			for _, e := range v {
				switch ev := e.(type) {
				case Context:
					contexts = append(contexts, ev)
				case map[string]any:
					// maybe a context spec; try reflect
					ctx, ok := anyToContext(ev)
					if ok {
						contexts = append(contexts, ctx)
					}
				}
			}
		case string:
			unrecognized = true
		default:
			rv := reflect.ValueOf(a)
			if rv.Kind() != reflect.Struct && rv.Kind() != reflect.Map && rv.Kind() != reflect.Slice {
				unrecognized = true
				continue
			}
			// reflect
			if rv.Kind() == reflect.Struct {
				beforeF := len(formulas)
				beforeC := len(contexts)
				extractStruct(rv, formulas, &contexts)
				if len(formulas) == beforeF && len(contexts) == beforeC && !hasContextFields(rv) {
					unrecognized = true
				}
			} else if rv.Kind() == reflect.Map {
				// try formula map
				mm, ok := a.(map[string]any)
				if ok {
					isFormula := true
					for _, val := range mm {
						if _, ok := val.(string); !ok {
							isFormula = false
							break
						}
					}
					if isFormula && len(mm) > 0 {
						for k, val := range mm {
							formulas[k] = val.(string)
						}
					}
				}
			} else if rv.Kind() == reflect.Slice {
				for i := 0; i < rv.Len(); i++ {
					el := rv.Index(i).Interface()
					if c, ok := el.(Context); ok {
						contexts = append(contexts, c)
					} else if m, ok := el.(map[string]any); ok {
						if ctx, ok2 := anyToContext(m); ok2 {
							contexts = append(contexts, ctx)
						}
					}
				}
			}
		}
	}
	if len(contexts) == 0 {
		contexts = []Context{{Name: "default"}}
	}
	return formulas, contexts, unrecognized
}

func extractStruct(rv reflect.Value, formulas map[string]string, contexts *[]Context) {
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		name := f.Name
		fv := rv.Field(i)
		switch name {
		case "Formulas", "Formula", "FormulasMap", "Expressions":
			if fv.Kind() == reflect.Map {
				iter := fv.MapRange()
				for iter.Next() {
					k := toStringAny(iter.Key().Interface())
					v := iter.Value().Interface()
					if s, ok := v.(string); ok {
						formulas[k] = s
					}
				}
			}
		case "Contexts", "Context", "Scopes", "DeclaredContexts", "ContextList":
			if fv.Kind() == reflect.Slice {
				for j := 0; j < fv.Len(); j++ {
					el := fv.Index(j).Interface()
					if c, ok := el.(Context); ok {
						*contexts = append(*contexts, c)
					} else if m, ok := el.(map[string]any); ok {
						if ctx, ok2 := anyToContext(m); ok2 {
							*contexts = append(*contexts, ctx)
						}
					} else {
						// reflect struct -> context?
						if rv2 := reflect.ValueOf(el); rv2.Kind() == reflect.Struct {
							c := structToContext(rv2)
							*contexts = append(*contexts, c)
						}
					}
				}
			} else if fv.Kind() == reflect.Struct {
				*contexts = append(*contexts, structToContext(fv))
			}
		}
	}
	// If struct itself looks like a context (has Call/Entity/Pack fields), convert
	if hasContextFields(rv) {
		*contexts = append(*contexts, structToContext(rv))
	}
}

func hasContextFields(rv reflect.Value) bool {
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		n := rt.Field(i).Name
		if n == "Call" || n == "Entity" || n == "Pack" || n == "CallNames" || n == "EntityNames" || n == "PackNames" || n == "CallScope" || n == "EntityScope" || n == "PackScope" {
			return true
		}
	}
	return false
}

func structToContext(rv reflect.Value) Context {
	c := Context{}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		fv := rv.Field(i)
		switch f.Name {
		case "Name":
			if s, ok := fv.Interface().(string); ok {
				c.Name = s
			}
		case "Call":
			if sl, ok := fv.Interface().([]string); ok {
				c.Call = sl
			}
		case "CallNames":
			if sl, ok := fv.Interface().([]string); ok {
				c.CallNames = sl
			}
		case "CallScope":
			if m, ok := fv.Interface().(map[string]any); ok {
				c.CallScope = m
			}
		case "Entity":
			if sl, ok := fv.Interface().([]string); ok {
				c.Entity = sl
			}
		case "EntityNames":
			if sl, ok := fv.Interface().([]string); ok {
				c.EntityNames = sl
			}
		case "EntityScope":
			if m, ok := fv.Interface().(map[string]any); ok {
				c.EntityScope = m
			}
		case "Pack":
			if sl, ok := fv.Interface().([]string); ok {
				c.Pack = sl
			}
		case "PackNames":
			if sl, ok := fv.Interface().([]string); ok {
				c.PackNames = sl
			}
		case "PackScope":
			if m, ok := fv.Interface().(map[string]any); ok {
				c.PackScope = m
			}
		}
	}
	return c
}

func anyToContext(m map[string]any) (Context, bool) {
	c := Context{}
	if v, ok := m["Name"].(string); ok {
		c.Name = v
	}
	if v, ok := m["Call"].([]string); ok {
		c.Call = v
	}
	if v, ok := m["Call"].([]any); ok {
		for _, e := range v {
			c.Call = append(c.Call, toStringAny(e))
		}
	}
	if v, ok := m["CallNames"].([]string); ok {
		c.CallNames = v
	}
	if v, ok := m["CallScope"].(map[string]any); ok {
		c.CallScope = v
	}
	if v, ok := m["Entity"].([]string); ok {
		c.Entity = v
	}
	if v, ok := m["Entity"].([]any); ok {
		for _, e := range v {
			c.Entity = append(c.Entity, toStringAny(e))
		}
	}
	if v, ok := m["EntityNames"].([]string); ok {
		c.EntityNames = v
	}
	if v, ok := m["EntityScope"].(map[string]any); ok {
		c.EntityScope = v
	}
	if v, ok := m["Pack"].([]string); ok {
		c.Pack = v
	}
	if v, ok := m["Pack"].([]any); ok {
		for _, e := range v {
			c.Pack = append(c.Pack, toStringAny(e))
		}
	}
	if v, ok := m["PackNames"].([]string); ok {
		c.PackNames = v
	}
	if v, ok := m["PackScope"].(map[string]any); ok {
		c.PackScope = v
	}
	// require at least one field
	if c.Name == "" && len(c.callNames()) == 0 && len(c.entityNames()) == 0 && len(c.packNames()) == 0 {
		// maybe formulas? no
	}
	return c, true
}

func toStringAny(v any) string {
	switch x := v.(type) {
	case string:
		return x
	default:
		return fmt.Sprintf("%v", x)
	}
}

// --- Check entry points ---

func Check(args ...any) []Issue {
	formulas, contexts, unrecognized := parseArgs(args)
	if formulas == nil {
		formulas = map[string]string{}
	}
	var issues []Issue

	if len(args) == 0 || unrecognized {
		issues = append(issues, Issue{
			Severity: "error",
			Kind:     "invalid-input",
			Message:  "formula: invalid or empty input to Check",
		})
	}
	// empty formulas with no contexts? If no formulas and no recognized context, already invalid above.
	// If formulas empty but contexts provided, allow shadowing checks; but mark invalid if no formulas at all?
	// Per investigator, empty inputs must error. Treat zero formulas as invalid unless a context was explicitly provided.
	// cycle detection
	deps := buildDeps(formulas)
	for _, cyc := range findCycles(deps) {
		other := cyc[0]
		if len(cyc) > 1 {
			other = cyc[1]
		}
		issues = append(issues, Issue{
			Severity: "error",
			Kind:     "cycle",
			Formula:  cyc[0],
			Name:     other,
			Scope:    "derived",
			Message:  fmt.Sprintf("formula cycle: %s", strings.Join(append(cyc, cyc[0]), " -> ")),
		})
	}

	// shadowing
	seenShadow := map[string]bool{}
	for _, ctx := range contexts {
		ec := ctx
		for _, pn := range ec.packNames() {
			if containsStr(ec.entityNames(), pn) {
				if seenShadow[pn] {
					continue
				}
				seenShadow[pn] = true
				issues = append(issues, Issue{
					Severity: "warning",
					Kind:     "shadowing",
					Name:     pn,
					Scope:    "pack/entity",
					Context:  ec.name(),
					Message:  fmt.Sprintf("shadowing: pack scope defines %q which shadows entity scope %q", pn, pn),
				})
			}
		}
	}

	// unresolvable + parse errors
	seenUnres := map[string]bool{}
	for _, ctx := range contexts {
		ec := ctx
		for fname, src := range formulas {
			e, err := Parse(src)
			if err != nil {
				issues = append(issues, Issue{
					Severity: "error",
					Kind:     "parse",
					Formula:  fname,
					Context:  ec.name(),
					Message:  err.Error(),
				})
				continue
			}
			idents := collectIdents(e)
			for _, id := range idents {
				if isResolvableIdent(id, ec, formulas) {
					continue
				}
				key := fname + "|" + id
				if seenUnres[key] {
					continue
				}
				seenUnres[key] = true
				scope := expectedScope(id)
				issues = append(issues, Issue{
					Severity: "error",
					Kind:     "unresolvable",
					Formula:  fname,
					Name:     id,
					Scope:    scope,
					Context:  ec.name(),
					Message:  fmt.Sprintf("formula %q: unresolvable name %q (expected %s scope)", fname, id, scope),
				})
			}
		}
	}

	return issues
}

func Lint(args ...any) []Issue { return Check(args...) }

func Validate(args ...any) error {
	issues := Check(args...)
	for _, is := range issues {
		if is.Severity == "error" {
			return fmt.Errorf("formula: %s", is.Message)
		}
	}
	return nil
}

// aliases
func LintFormulas(args ...any) []Issue   { return Check(args...) }
func CheckFormulas(args ...any) []Issue  { return Check(args...) }
func ValidateFormulas(args ...any) error { return Validate(args...) }
func CheckFormula(args ...any) []Issue   { return Check(args...) }
func LintFormula(args ...any) []Issue    { return Check(args...) }

func isResolvableIdent(name string, ctx Context, formulas map[string]string) bool {
	if isBuiltin(name) {
		return true
	}
	if _, ok := formulas[name]; ok {
		return true
	}
	if name == "formulas" || name == "formula" {
		return true
	}
	if containsStr(ctx.callNames(), name) {
		return true
	}
	if containsStr(ctx.entityNames(), name) {
		return true
	}
	if containsStr(ctx.packNames(), name) {
		return true
	}
	return false
}

func expectedScope(name string) string {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "weapon") || strings.HasPrefix(lower, "target") ||
		strings.HasPrefix(lower, "actor") || lower == "round" {
		return "call"
	}
	if strings.HasPrefix(lower, "proficiency") || strings.HasPrefix(lower, "spell_dc_base") ||
		strings.HasPrefix(lower, "hit_die") {
		return "pack"
	}
	// default to entity for everything else (level, attributes, resources, ...).
	return "entity"
}

// heuristic cleanup: refine expectedScope
func init() {
	// no-op
}

func collectIdents(e Expr) []string {
	var out []string
	var walk func(Expr)
	walk = func(expr Expr) {
		switch v := expr.(type) {
		case Ident:
			out = append(out, v.Name)
		case Member:
			walk(v.Target)
		case Index:
			walk(v.Target)
			walk(v.Index)
		case Call:
			walk(v.Callee)
			for _, a := range v.Args {
				walk(a)
			}
		case Unary:
			walk(v.X)
		case Binary:
			walk(v.Left)
			walk(v.Right)
		case Ternary:
			walk(v.Cond)
			walk(v.Then)
			walk(v.Else)
		case ListLit:
			for _, it := range v.Items {
				walk(it)
			}
		case MapLit:
			for _, ke := range v.KeyExprs {
				if ke != nil {
					walk(ke)
				}
			}
			for _, val := range v.Values {
				walk(val)
			}
		}
	}
	walk(e)
	// dedup
	seen := map[string]bool{}
	var dedup []string
	for _, s := range out {
		if seen[s] {
			continue
		}
		seen[s] = true
		dedup = append(dedup, s)
	}
	return dedup
}

func buildDeps(formulas map[string]string) map[string][]string {
	deps := map[string][]string{}
	for name, src := range formulas {
		e, err := Parse(src)
		if err != nil {
			continue
		}
		idents := collectIdents(e)
		for _, id := range idents {
			if _, ok := formulas[id]; ok {
				deps[name] = append(deps[name], id)
			}
		}
		// also member formulas.x
		// walk for Member Target Ident formulas
		// collect handled by idents? Member root not collected as ident? collectIdents walks Member.Target which is Ident "formulas", recorded as "formulas". Need derived name from Member.Name. Let's do extra walk.
		var walk func(Expr)
		walk = func(expr Expr) {
			switch v := expr.(type) {
			case Member:
				if id, ok := v.Target.(Ident); ok && (id.Name == "formulas" || id.Name == "formula") {
					if _, ok2 := formulas[v.Name]; ok2 {
						deps[name] = append(deps[name], v.Name)
					}
				}
				walk(v.Target)
			case Index:
				walk(v.Target)
				walk(v.Index)
			case Call:
				walk(v.Callee)
				for _, a := range v.Args {
					walk(a)
				}
			case Unary:
				walk(v.X)
			case Binary:
				walk(v.Left)
				walk(v.Right)
			case Ternary:
				walk(v.Cond)
				walk(v.Then)
				walk(v.Else)
			case ListLit:
				for _, it := range v.Items {
					walk(it)
				}
			case MapLit:
				for _, ke := range v.KeyExprs {
					if ke != nil {
						walk(ke)
					}
				}
				for _, val := range v.Values {
					walk(val)
				}
			}
		}
		walk(e)
	}
	// dedup deps
	for k := range deps {
		seen := map[string]bool{}
		var out []string
		for _, d := range deps[k] {
			if seen[d] {
				continue
			}
			seen[d] = true
			out = append(out, d)
		}
		deps[k] = out
	}
	return deps
}

func findCycles(deps map[string][]string) [][]string {
	// DFS
	var cycles [][]string
	visited := map[string]bool{}
	onStack := map[string]bool{}
	path := []string{}
	var dfs func(node string)
	dfs = func(node string) {
		visited[node] = true
		onStack[node] = true
		path = append(path, node)
		for _, nb := range deps[node] {
			if !visited[nb] {
				dfs(nb)
			} else if onStack[nb] {
				// found cycle from nb to node
				// extract cycle nodes
				idx := -1
				for i, n := range path {
					if n == nb {
						idx = i
						break
					}
				}
				if idx >= 0 {
					cyc := append([]string{}, path[idx:]...)
					cycles = append(cycles, cyc)
				}
			}
		}
		path = path[:len(path)-1]
		onStack[node] = false
	}
	// ensure all nodes
	nodes := make([]string, 0, len(deps))
	for k := range deps {
		nodes = append(nodes, k)
	}
	sort.Strings(nodes)
	for _, n := range nodes {
		if !visited[n] {
			dfs(n)
		}
	}
	// also nodes in formulas not in deps
	// dedup cycles by sorted join
	seen := map[string]bool{}
	var out [][]string
	for _, c := range cycles {
		key := strings.Join(sortedCopy(c), ",")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	return out
}

func sortedCopy(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}
