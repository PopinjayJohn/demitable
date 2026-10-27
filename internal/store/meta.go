package store

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
)

// noteMetaJSON derives the JSON metadata document for a note record. It is the
// single source of truth for what the query compiler sees: universal fields
// are stored under their own keys, the GM's own fields are stored both under
// their own keys and under "data" when that is where they came from, and a
// missing "data" object becomes an empty map so data.<path> never panics.
func noteMetaJSON(n NoteRecord) (string, error) {
	m := map[string]any{}
	if n.Meta != nil {
		for k, v := range n.Meta {
			m[k] = normalizeValue(v)
		}
	}
	// Universal fields.
	setMeta(m, "uid", n.UID)
	setMeta(m, "path", n.Path)
	name := n.Name
	if name == "" {
		name = n.Title
	}
	setMeta(m, "name", name)
	setMeta(m, "title", n.Title)
	setMeta(m, "type", n.Type)
	setMeta(m, "system", n.System)
	setMeta(m, "campaign", n.Campaign)
	setMeta(m, "kind", n.Kind)
	setMeta(m, "player", n.Player)
	setMeta(m, "workflow", n.Workflow)
	setMeta(m, "visibility", n.Visibility)
	if n.Grants != nil {
		m["grants"] = normalizeValue(n.Grants)
	}
	if n.Revealed != nil {
		m["revealed"] = normalizeValue(n.Revealed)
	}
	if n.Discovered != nil {
		m["discovered"] = normalizeValue(n.Discovered)
	}
	if n.Aliases != nil {
		m["aliases"] = normalizeValue(n.Aliases)
	}
	if n.Headings != nil {
		m["headings"] = normalizeValue(n.Headings)
	}
	if n.Tags != nil {
		m["tags"] = normalizeValue(n.Tags)
	}
	m["body"] = n.Body
	// GM-owned fields.
	if n.Fields != nil {
		for k, v := range n.Fields {
			m[k] = normalizeValue(v)
		}
	}
	if n.Data != nil {
		m["data"] = normalizeValue(n.Data)
	}
	if _, ok := m["data"]; !ok {
		m["data"] = map[string]any{}
	}
	// If "data" ended up as a non-map (a scalar field named data), wrap it.
	if _, ok := m["data"].(map[string]any); !ok {
		wrapped := map[string]any{"value": m["data"]}
		m["data"] = wrapped
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func setMeta(m map[string]any, key, val string) {
	if val == "" {
		return
	}
	m[key] = val
}

func normalizeValue(v any) any {
	if fv, ok := v.(frontmatter.Value); ok {
		return frontmatterValueToGo(fv)
	}
	switch val := v.(type) {
	case []string:
		out := make([]any, 0, len(val))
		for _, s := range val {
			out = append(out, s)
		}
		return out
	case []frontmatter.Value:
		out := make([]any, 0, len(val))
		for _, e := range val {
			out = append(out, frontmatterValueToGo(e))
		}
		return out
	case map[string]frontmatter.Value:
		out := map[string]any{}
		for k, e := range val {
			out[k] = frontmatterValueToGo(e)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for k, e := range val {
			out[k] = normalizeValue(e)
		}
		return out
	case []any:
		out := make([]any, 0, len(val))
		for _, e := range val {
			out = append(out, normalizeValue(e))
		}
		return out
	default:
		return v
	}
}

func frontmatterValueToGo(fv frontmatter.Value) any {
	switch fv.Kind {
	case frontmatter.KindNull:
		return nil
	case frontmatter.KindString, frontmatter.KindLink, frontmatter.KindDate:
		return fv.Str
	case frontmatter.KindBool:
		return fv.Bool
	case frontmatter.KindInt:
		return fv.Int
	case frontmatter.KindFloat:
		return fv.Flo
	case frontmatter.KindList:
		out := make([]any, 0, len(fv.List))
		for _, e := range fv.List {
			out = append(out, frontmatterValueToGo(e))
		}
		return out
	case frontmatter.KindMap:
		out := map[string]any{}
		for _, e := range fv.Map {
			out[e.Key] = frontmatterValueToGo(e.Value)
		}
		return out
	case frontmatter.KindRaw:
		return fv.Raw
	}
	return nil
}

func toFloat64(v any) (float64, error) {
	if fv, ok := v.(frontmatter.Value); ok {
		_ = fv
	}
	switch n := v.(type) {
	case bool:
		if n {
			return 1, nil
		}
		return 0, nil
	case int:
		return float64(n), nil
	case int8:
		return float64(n), nil
	case int16:
		return float64(n), nil
	case int32:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case uint:
		return float64(n), nil
	case uint8:
		return float64(n), nil
	case uint16:
		return float64(n), nil
	case uint32:
		return float64(n), nil
	case uint64:
		return float64(n), nil
	case float32:
		return float64(n), nil
	case float64:
		return n, nil
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return 0, err
		}
		return f, nil
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err != nil {
			return 0, err
		}
		return f, nil
	}
	return 0, fmt.Errorf("not number")
}
