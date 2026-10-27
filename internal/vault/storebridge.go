package vault

import (
	"strings"

	"github.com/PopinjayJohn/demitable/internal/store"
	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
)

func noteToRecord(n *Note) store.NoteRecord {
	rec := store.NoteRecord{
		UID:        n.UID,
		Path:       n.Path,
		Title:      n.Name,
		Name:       n.Name,
		Type:       n.Type,
		System:     n.System,
		Campaign:   n.Campaign,
		Kind:       n.Kind,
		Player:     n.Player,
		Workflow:   n.Workflow,
		Visibility: n.Visibility,
		Grants:     n.Grants,
		Revealed:   n.Revealed,
		Discovered: n.Discovered,
		Aliases:    n.Aliases,
		Headings:   n.Headings,
		Tags:       n.Tags,
		Body:       n.Prose,
	}
	if n.Fields != nil {
		m := map[string]any{}
		for k, v := range n.Fields {
			m[k] = frontmatterValueToAny(v)
		}
		rec.Fields = m
	}
	return rec
}

func frontmatterValueToAny(v any) any {
	if fv, ok := v.(frontmatter.Value); ok {
		return frontmatterValueToGo(fv)
	}
	return v
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

func vaultPathValue(n Note, path string) any {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	// Universal fields.
	switch strings.ToLower(path) {
	case "uid":
		return n.UID
	case "name", "title":
		return n.Name
	case "type":
		return n.Type
	case "system":
		return n.System
	case "campaign":
		return n.Campaign
	case "kind":
		return n.Kind
	case "player":
		return n.Player
	case "workflow":
		return n.Workflow
	case "visibility":
		return n.Visibility
	case "grants":
		return n.Grants
	case "revealed":
		return n.Revealed
	case "discovered":
		return n.Discovered
	case "tags":
		return n.Tags
	case "aliases":
		return n.Aliases
	case "headings":
		return n.Headings
	case "path":
		return n.Path
	}
	// data.<path> or arbitrary GM field.
	parts := strings.Split(path, ".")
	// If first segment is "data", navigate into Fields["data"].
	cur := any(n.Fields)
	if strings.ToLower(parts[0]) == "data" {
		// start from Fields["data"] if present
		if v, ok := n.Fields["data"]; ok {
			cur = frontmatterValueToAny(v)
		} else {
			cur = nil
		}
		parts = parts[1:]
	} else {
		// arbitrary top-level GM field
		if v, ok := n.Fields[parts[0]]; ok {
			cur = frontmatterValueToAny(v)
			parts = parts[1:]
		} else {
			cur = nil
			parts = nil
		}
	}
	for _, p := range parts {
		switch c := cur.(type) {
		case map[string]any:
			v, ok := c[p]
			if !ok {
				// case-insensitive
				for k, vv := range c {
					if strings.EqualFold(k, p) {
						v = vv
						ok = true
						break
					}
				}
				if !ok {
					return nil
				}
				cur = v
			} else {
				cur = v
			}
		case frontmatter.Value:
			cur = frontmatterValueToAny(c)
			// re-evaluate same part
			continue
		default:
			return nil
		}
	}
	if cur == nil && len(parts) == 0 {
		// top-level field already consumed? handle data empty
	}
	return cur
}
