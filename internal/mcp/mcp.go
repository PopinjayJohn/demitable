// Package mcp implements the compiled-in MCP server (ADR-0014).
//
// It is deliberately a service with no UI: read-only, token-gated, off by
// default. The transport is streamable HTTP over /mcp. It speaks JSON-RPC
// 2.0 and returns the GM's full view of the vault.
package mcp

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/PopinjayJohn/demitable/internal/formula"
	"github.com/PopinjayJohn/demitable/internal/pack"
	"github.com/PopinjayJohn/demitable/internal/vault"
	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
)

// ProtocolVersion is the MCP protocol version this server pins. It is a
// constant so a change shows up in review (ADR-0014, O3b).
const ProtocolVersion = "2025-06-18"

// PrivacyPrompt is the text printed when MCP is enabled. It states plainly
// that connecting an LLM takes the campaign off the machine.
const PrivacyPrompt = "MCP is enabled. Connecting an LLM to this server means your campaign leaves the machine, depending on where the model runs — including unrehearsed player behaviour, unreleased secrets and your GM notes. Local by default, and never silently remote. (ADR-0014)"

// Server is the MCP server bound to one vault.
type Server struct {
	Vault  *vault.Vault
	Token  string
	Logger *slog.Logger
}

// NewServer builds a server.
func NewServer(v *vault.Vault, token string, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{Vault: v, Token: token, Logger: logger}
}

// ServeHTTP implements the streamable HTTP transport (JSON-RPC over POST).
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Token gate: no token or wrong token -> refused.
	if !tokenOK(r, s.Token) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "mcp: token required"})
		return
	}
	if r.Method == http.MethodGet {
		// No server-initiated SSE stream in this implementation.
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(": keepalive\n\n"))
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req jsonrpcRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		writeJSONRPCError(w, nil, -32700, "parse error")
		return
	}
	// Notifications (no id) get 202 with no body.
	if req.ID == nil && isNotification(req.Method) {
		handleNotification(req.Method)
		w.WriteHeader(http.StatusAccepted)
		return
	}
	res, rpcErr := s.dispatch(r.Context(), req)
	if rpcErr != nil {
		writeJSONRPCError(w, req.ID, rpcErr.code, rpcErr.msg)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      req.ID,
		"result":  res,
	})
}

type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcError struct {
	code int
	msg  string
}

func isNotification(m string) bool {
	return strings.HasPrefix(m, "notifications/")
}

func handleNotification(string) {}

func (s *Server) dispatch(ctx interface{}, req jsonrpcRequest) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities": map[string]any{
				"resources": map[string]any{"listChanged": false, "subscribe": false},
				"tools":     map[string]any{"listChanged": false},
			},
			"serverInfo": map[string]any{
				"name":    "demitable",
				"version": "dev",
			},
			"instructions": "Demitable MCP server: read-only GM view.",
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "resources/list":
		return map[string]any{"resources": s.listResources()}, nil
	case "resources/templates/list":
		return map[string]any{"resourceTemplates": s.listResourceTemplates()}, nil
	case "resources/read":
		var p struct {
			URI string `json:"uri"`
		}
		_ = json.Unmarshal(req.Params, &p)
		contents, err := s.readResource(p.URI)
		if err != nil {
			return nil, &rpcError{code: -32002, msg: err.Error()}
		}
		return map[string]any{"contents": contents}, nil
	case "tools/list":
		return map[string]any{"tools": toolDefs()}, nil
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		_ = json.Unmarshal(req.Params, &p)
		if isWriteShaped(p.Name) {
			return nil, &rpcError{code: -32601, msg: "mcp: read-only: write operations are refused"}
		}
		res, err := s.callTool(p.Name, p.Arguments)
		if err != nil {
			// tool error reported as result with isError, per MCP.
			return map[string]any{
				"content": []map[string]any{{"type": "text", "text": err.Error()}},
				"isError": true,
			}, nil
		}
		return res, nil
	default:
		return nil, &rpcError{code: -32601, msg: "method not found"}
	}
}

func isWriteShaped(name string) bool {
	l := strings.ToLower(name)
	for _, w := range []string{"write", "create", "update", "delete", "put", "patch", "remove", "modify", "set", "add", "insert"} {
		if strings.Contains(l, w) {
			return true
		}
	}
	return false
}

func tokenOK(r *http.Request, token string) bool {
	if token == "" {
		return false
	}
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		// also allow ?token= for websockets-style, and x-mcp-token
		if t := r.URL.Query().Get("token"); t != "" {
			h = "Bearer " + t
		} else if t := r.Header.Get("X-MCP-Token"); t != "" {
			h = "Bearer " + t
		} else {
			return false
		}
	}
	got := strings.TrimPrefix(h, "Bearer ")
	return got == token
}

func writeJSONRPCError(w http.ResponseWriter, id any, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]any{"code": code, "message": msg},
	})
}

// --- resources ---

func (s *Server) listResources() []map[string]any {
	out := []map[string]any{}
	if s.Vault != nil {
		out = append(out, map[string]any{
			"uri":         "demitable://campaign",
			"name":        "campaign",
			"description": "Campaign index",
			"mimeType":    "application/json",
		})
		// packs
		ids, _ := pack.ListIDs()
		for _, id := range ids {
			out = append(out, map[string]any{
				"uri":         "demitable://pack/" + id,
				"name":        id,
				"description": "System pack",
				"mimeType":    "application/json",
			})
		}
	}
	return out
}

func (s *Server) listResourceTemplates() []map[string]any {
	return []map[string]any{
		{"uriTemplate": "demitable://note/{uid}", "name": "note", "description": "A note", "mimeType": "application/json"},
		{"uriTemplate": "demitable://character/{uid}", "name": "character", "description": "A character", "mimeType": "application/json"},
		{"uriTemplate": "demitable://pack/{id}", "name": "pack", "description": "A system pack", "mimeType": "application/json"},
	}
}

func (s *Server) readResource(uri string) ([]map[string]any, error) {
	switch {
	case uri == "demitable://campaign":
		return []map[string]any{s.jsonContent(uri, s.campaignJSON())}, nil
	case strings.HasPrefix(uri, "demitable://note/"):
		uid := strings.TrimPrefix(uri, "demitable://note/")
		n := s.Vault.GetNote(uid)
		if n == nil {
			return nil, fmt.Errorf("note not found: %s", uid)
		}
		return []map[string]any{s.jsonContent(uri, noteJSON(n))}, nil
	case strings.HasPrefix(uri, "demitable://character/"):
		uid := strings.TrimPrefix(uri, "demitable://character/")
		n := s.Vault.GetNote(uid)
		if n == nil {
			return nil, fmt.Errorf("character not found: %s", uid)
		}
		return []map[string]any{s.jsonContent(uri, characterJSON(n))}, nil
	case strings.HasPrefix(uri, "demitable://pack/"):
		id := strings.TrimPrefix(uri, "demitable://pack/")
		pk, err := pack.Load(id)
		if err != nil || pk == nil {
			// fallback example
			pk, _ = pack.Example()
		}
		if pk == nil {
			return nil, fmt.Errorf("pack not found: %s", id)
		}
		return []map[string]any{s.jsonContent(uri, packJSON(pk))}, nil
	}
	return nil, fmt.Errorf("unknown resource uri: %s", uri)
}

func (s *Server) jsonContent(uri string, v any) map[string]any {
	b, _ := json.Marshal(v)
	return map[string]any{"uri": uri, "mimeType": "application/json", "text": string(b)}
}

func noteJSON(n *vault.Note) map[string]any {
	out := map[string]any{
		"uid":        n.UID,
		"path":       n.Path,
		"name":       n.Name,
		"type":       n.Type,
		"system":     n.System,
		"campaign":   n.Campaign,
		"kind":       n.Kind,
		"visibility": n.Visibility,
		"workflow":   n.Workflow,
		"player":     n.Player,
		"grants":     n.Grants,
		"revealed":   n.Revealed,
		"discovered": n.Discovered,
		"headings":   n.Headings,
		"aliases":    n.Aliases,
		"tags":       n.Tags,
		"body":       n.Body,
		"links":      n.Links,
		"hash":       n.Hash,
	}
	// fields
	fields := map[string]any{}
	for k, v := range n.Fields {
		if fv, ok := v.(frontmatter.Value); ok {
			fields[k] = fmValueToAny(fv)
		} else {
			fields[k] = v
		}
	}
	out["fields"] = fields
	return out
}

func characterJSON(n *vault.Note) map[string]any {
	out := noteJSON(n)
	// computed formulas
	packID := n.System
	if packID == "" {
		packID = "example"
	}
	pk, _ := pack.Load(packID)
	if pk == nil {
		pk, _ = pack.Example()
	}
	computed := map[string]any{}
	attrs := map[string]any{}
	resources := map[string]any{}
	if raw, ok := n.Fields["attributes"]; ok {
		if fv, ok := raw.(frontmatter.Value); ok {
			if m, ok := fmValueToAny(fv).(map[string]any); ok {
				attrs = m
			}
		} else if m, ok := raw.(map[string]any); ok {
			attrs = m
		}
	}
	if raw, ok := n.Fields["resources"]; ok {
		if fv, ok := raw.(frontmatter.Value); ok {
			if m, ok := fmValueToAny(fv).(map[string]any); ok {
				resources = m
			}
		} else if m, ok := raw.(map[string]any); ok {
			resources = m
		}
	}
	if pk != nil && pk.Formulas != nil {
		entity := map[string]any{}
		for k, v := range n.Fields {
			if fv, ok := v.(frontmatter.Value); ok {
				entity[k] = fmValueToAny(fv)
			} else {
				entity[k] = v
			}
		}
		if len(attrs) > 0 {
			entity["attributes"] = attrs
		}
		if len(resources) > 0 {
			entity["resources"] = resources
		}
		scope := formula.NewScope(nil, entity, nil)
		scope.SetFormulas(pk.Formulas)
		for name := range pk.Formulas {
			if val, err := scope.EvalFormula(name); err == nil {
				computed[name] = val
			}
		}
	}
	out["attributes"] = attrs
	out["resources"] = resources
	out["computed"] = computed
	return out
}

func packJSON(pk *pack.Pack) map[string]any {
	return map[string]any{
		"id":         pk.ID,
		"name":       pk.Name,
		"version":    pk.Version,
		"content":    pk.Content,
		"attributes": pk.Attributes,
		"resources":  pk.Resources,
		"formulas":   pk.Formulas,
		"views":      pk.Views,
		"dice":       pk.Dice,
	}
}

func (s *Server) campaignJSON() map[string]any {
	counts := map[string]int{}
	for _, n := range s.Vault.Notes {
		counts[n.Type]++
	}
	return map[string]any{
		"note_count":     len(s.Vault.Notes),
		"counts_by_type": counts,
		"packs": func() []string {
			ids, _ := pack.ListIDs()
			return ids
		}(),
	}
}

func fmValueToAny(v frontmatter.Value) any {
	switch v.Kind {
	case frontmatter.KindNull:
		return nil
	case frontmatter.KindString, frontmatter.KindLink, frontmatter.KindDate:
		return v.Str
	case frontmatter.KindBool:
		return v.Bool
	case frontmatter.KindInt:
		return int(v.Int)
	case frontmatter.KindFloat:
		return v.Flo
	case frontmatter.KindList:
		out := make([]any, 0, len(v.List))
		for _, e := range v.List {
			out = append(out, fmValueToAny(e))
		}
		return out
	case frontmatter.KindMap:
		out := map[string]any{}
		for _, e := range v.Map {
			out[e.Key] = fmValueToAny(e.Value)
		}
		return out
	case frontmatter.KindRaw:
		return v.Raw
	}
	return nil
}

// --- tools ---

func toolDefs() []map[string]any {
	return []map[string]any{
		{
			"name":        "search_notes",
			"description": "Search notes by text, type and tags.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
					"type":  map[string]any{"type": "string"},
					"tags":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"limit": map[string]any{"type": "integer"},
				},
			},
		},
		{
			"name":        "get_character",
			"description": "Get a character with computed formula values.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
					"uid":  map[string]any{"type": "string"},
				},
			},
		},
		{
			"name":        "list_by",
			"description": "Query notes with the ADR-0047 predicate language.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"type":   map[string]any{"type": "string"},
					"filter": map[string]any{"type": "string"},
					"query":  map[string]any{"type": "string"},
					"sort":   map[string]any{"type": "string"},
					"desc":   map[string]any{"type": "boolean"},
					"limit":  map[string]any{"type": "integer"},
					"cursor": map[string]any{"type": "string"},
				},
			},
		},
		{
			"name":        "get_links",
			"description": "Get outgoing and incoming links for a note.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"uid":       map[string]any{"type": "string"},
					"direction": map[string]any{"type": "string"},
				},
				"required": []string{"uid"},
			},
		},
		{
			"name":        "resolve_reference",
			"description": "Resolve a wikilink or name to a note.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"text": map[string]any{"type": "string"},
				},
				"required": []string{"text"},
			},
		},
		{
			"name":        "evaluate_formula",
			"description": "Evaluate a pack formula expression with call-scope binding.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"expression": map[string]any{"type": "string"},
					"context":    map[string]any{"type": "object"},
				},
				"required": []string{"expression"},
			},
		},
	}
}

func (s *Server) callTool(name string, args json.RawMessage) (map[string]any, error) {
	var m map[string]any
	if len(args) > 0 {
		_ = json.Unmarshal(args, &m)
	}
	if m == nil {
		m = map[string]any{}
	}
	switch name {
	case "search_notes":
		return s.toolSearchNotes(m)
	case "get_character":
		return s.toolGetCharacter(m)
	case "list_by":
		return s.toolListBy(m)
	case "get_links":
		return s.toolGetLinks(m)
	case "resolve_reference":
		return s.toolResolveReference(m)
	case "evaluate_formula":
		return s.toolEvaluateFormula(m)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *Server) toolSearchNotes(m map[string]any) (map[string]any, error) {
	q, _ := m["query"].(string)
	typ, _ := m["type"].(string)
	limit := 20
	if v, ok := m["limit"].(float64); ok {
		limit = int(v)
	}
	tags := []string{}
	if v, ok := m["tags"].([]any); ok {
		for _, t := range v {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
	}
	// Substring search over the GM's full view (not the query language).
	var notes []vault.Note
	for _, n := range s.Vault.Notes {
		notes = append(notes, *n)
	}
	out := []map[string]any{}
	for _, n := range notes {
		if typ != "" && n.Type != typ {
			continue
		}
		if q != "" {
			hay := strings.ToLower(n.Name + " " + n.Body)
			if !strings.Contains(hay, strings.ToLower(q)) {
				continue
			}
		}
		if len(tags) > 0 {
			ok := false
			for _, tag := range tags {
				for _, nt := range n.Tags {
					if nt == tag {
						ok = true
					}
				}
			}
			if !ok {
				continue
			}
		}
		out = append(out, map[string]any{"uid": n.UID, "name": n.Name, "type": n.Type})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	b, _ := json.Marshal(out)
	return map[string]any{"content": []map[string]any{{"type": "text", "text": string(b)}}}, nil
}

func (s *Server) toolGetCharacter(m map[string]any) (map[string]any, error) {
	uid, _ := m["uid"].(string)
	name, _ := m["name"].(string)
	n := findNote(s.Vault, uid, name)
	if n == nil {
		return nil, fmt.Errorf("character not found")
	}
	b, _ := json.Marshal(characterJSON(n))
	return map[string]any{"content": []map[string]any{{"type": "text", "text": string(b)}}}, nil
}

func (s *Server) toolListBy(m map[string]any) (map[string]any, error) {
	typ, _ := m["type"].(string)
	filter, _ := m["filter"].(string)
	if filter == "" {
		if q, ok := m["query"].(string); ok {
			filter = q
		}
	}
	sortField, _ := m["sort"].(string)
	desc := false
	if v, ok := m["desc"].(bool); ok {
		desc = v
	}
	limit := 0
	if v, ok := m["limit"].(float64); ok {
		limit = int(v)
	}
	cursor, _ := m["cursor"].(string)
	q := vault.Query{Type: typ, Filter: filter, Sort: sortField, Desc: desc, Limit: limit, Cursor: cursor}
	notes, next, err := s.Vault.ListPage(vault.GM(), q)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(notes))
	for _, n := range notes {
		items = append(items, noteJSON(&n))
	}
	res := map[string]any{"items": items, "count": len(items)}
	if next != "" {
		res["next_cursor"] = next
		res["cursor"] = next
	}
	b, _ := json.Marshal(res)
	return map[string]any{"content": []map[string]any{{"type": "text", "text": string(b)}}}, nil
}

func (s *Server) toolGetLinks(m map[string]any) (map[string]any, error) {
	uid, _ := m["uid"].(string)
	n := s.Vault.GetNote(uid)
	if n == nil {
		return nil, fmt.Errorf("note not found: %s", uid)
	}
	// outgoing: parse wikilinks from body
	outgoing := []map[string]any{}
	for _, m := range findAllWikiLinks(n.Body) {
		outgoing = append(outgoing, map[string]any{"kind": "wikilink", "target": m, "text": m})
	}
	// backlinks: notes whose body references this note
	backlinks := []map[string]any{}
	for _, other := range s.Vault.Notes {
		if other.UID == uid {
			continue
		}
		if strings.Contains(other.Body, "[["+uid+"]]") || strings.Contains(other.Body, "[["+n.Name+"]]") {
			backlinks = append(backlinks, map[string]any{"uid": other.UID, "name": other.Name})
		}
	}
	res := map[string]any{"uid": uid, "outgoing": outgoing, "backlinks": backlinks}
	b, _ := json.Marshal(res)
	return map[string]any{"content": []map[string]any{{"type": "text", "text": string(b)}}}, nil
}

func (s *Server) toolResolveReference(m map[string]any) (map[string]any, error) {
	text, _ := m["text"].(string)
	t := strings.TrimSpace(text)
	t = strings.TrimPrefix(t, "[[")
	t = strings.TrimSuffix(t, "]]")
	// search
	for _, n := range s.Vault.Notes {
		if strings.EqualFold(n.UID, t) || strings.EqualFold(n.Name, t) {
			b, _ := json.Marshal(map[string]any{"uid": n.UID, "name": n.Name, "path": n.Path})
			return map[string]any{"content": []map[string]any{{"type": "text", "text": string(b)}}}, nil
		}
		for _, a := range n.Aliases {
			if strings.EqualFold(a, t) {
				b, _ := json.Marshal(map[string]any{"uid": n.UID, "name": n.Name, "path": n.Path})
				return map[string]any{"content": []map[string]any{{"type": "text", "text": string(b)}}}, nil
			}
		}
	}
	return nil, fmt.Errorf("unresolved reference: %s", text)
}

func (s *Server) toolEvaluateFormula(m map[string]any) (map[string]any, error) {
	expr, _ := m["expression"].(string)
	ctxMap := map[string]any{}
	if v, ok := m["context"].(map[string]any); ok {
		ctxMap = v
	}
	scope := formula.NewScope(ctxMap, nil, nil)
	val, err := formula.Evaluate(expr, scope)
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(map[string]any{"result": val})
	return map[string]any{"content": []map[string]any{{"type": "text", "text": string(b)}}}, nil
}

func findNote(v *vault.Vault, uid, name string) *vault.Note {
	if uid != "" {
		if n := v.GetNote(uid); n != nil {
			return n
		}
	}
	if name != "" {
		for _, n := range v.Notes {
			if strings.EqualFold(n.Name, name) || strings.EqualFold(n.UID, name) {
				return n
			}
		}
	}
	return nil
}

func findAllWikiLinks(body string) []string {
	var out []string
	i := 0
	for {
		idx := strings.Index(body[i:], "[[")
		if idx < 0 {
			break
		}
		start := i + idx + 2
		end := strings.Index(body[start:], "]]")
		if end < 0 {
			break
		}
		target := strings.TrimSpace(body[start : start+end])
		if j := strings.Index(target, "|"); j >= 0 {
			target = strings.TrimSpace(target[:j])
		}
		if j := strings.Index(target, "#"); j >= 0 {
			target = strings.TrimSpace(target[:j])
		}
		out = append(out, target)
		i = start + end + 2
	}
	return out
}
