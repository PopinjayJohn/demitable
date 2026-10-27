package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/PopinjayJohn/demitable/internal/pack"
	"github.com/PopinjayJohn/demitable/internal/session"
	"github.com/PopinjayJohn/demitable/internal/uid"
	"github.com/PopinjayJohn/demitable/internal/vault"
	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
)

// ---- encounter data model (F2b) --------------------------------------------

type encounterParticipant struct {
	UID  string `json:"uid"`
	Side string `json:"side"`
}

type encounterToken struct {
	ID        string   `json:"id"`
	UID       string   `json:"uid"`
	NoteUID   string   `json:"note_uid"`
	Side      string   `json:"side"`
	Hidden    bool     `json:"hidden"`
	VisibleTo []string `json:"visible_to"`
	X         any      `json:"x"`
	Y         any      `json:"y"`
}

type encounterState struct {
	ID               string                 `json:"id"`
	UID              string                 `json:"uid"`
	SessionID        string                 `json:"session_id"`
	Session          string                 `json:"session"`
	Name             string                 `json:"name"`
	System           string                 `json:"system"`
	Pack             string                 `json:"pack"`
	Campaign         string                 `json:"campaign"`
	Participants     []encounterParticipant `json:"participants"`
	Initiative       []string               `json:"initiative"`
	InitiativeOrder  []string               `json:"initiative_order"`
	CurrentTurn      int                    `json:"current_turn"`
	Turn             int                    `json:"turn"`
	Round            int                    `json:"round"`
	Tokens           []encounterToken       `json:"tokens"`
	Budget           map[string]any         `json:"budget"`
	OppositionBudget map[string]any         `json:"opposition_budget"`
	CreatedAt        string                 `json:"created_at"`
	UpdatedAt        string                 `json:"updated_at"`
}

func (e *encounterState) syncAliases() {
	e.UID = e.ID
	e.Session = e.SessionID
	e.InitiativeOrder = e.Initiative
	e.Turn = e.CurrentTurn
	if e.Budget != nil {
		e.OppositionBudget = e.Budget
	}
	if e.OppositionBudget != nil && e.Budget == nil {
		e.Budget = e.OppositionBudget
	}
}

// encounterHandlers registers F2b encounter routes.
func encounterHandlers(m *http.ServeMux, vlt *vault.Vault, sess *session.Store, tk string) {
	if sess == nil {
		if s, err := session.Open(":memory:"); err == nil {
			sess = s
		}
	}
	if tk == "" && vlt != nil {
		tk = tokenKeyFor(vlt)
	}

	allowedSession := func(principal vault.Principal, sid string) bool {
		if principal.IsGM() {
			return true
		}
		if principal.IsSeat() && principal.Session == sid {
			if _, ok, _ := sess.FindSeat(sid, principal.ID); ok {
				return true
			}
		}
		return false
	}

	loadState := func(sid, eid string) (*encounterState, bool) {
		if sess == nil {
			return nil, false
		}
		raw, ok, err := sess.GetEncounterState(sid, eid)
		if err != nil || !ok {
			return nil, false
		}
		var st encounterState
		if err := json.Unmarshal(raw, &st); err != nil {
			return nil, false
		}
		st.ID = eid
		st.SessionID = sid
		st.syncAliases()
		return &st, true
	}

	saveState := func(sid string, st *encounterState) error {
		if sess == nil {
			return fmt.Errorf("no session store")
		}
		st.ID = st.UID
		if st.ID == "" {
			st.ID = st.UID
		}
		if st.UID == "" {
			st.UID = st.ID
		}
		st.SessionID = sid
		st.Session = sid
		st.syncAliases()
		now := time.Now().UTC().Format(time.RFC3339)
		if st.CreatedAt == "" {
			st.CreatedAt = now
		}
		st.UpdatedAt = now
		raw, _ := json.Marshal(st)
		err := sess.UpdateEncounterState(sid, st.ID, raw)
		if err != nil {
			if errors.Is(err, session.ErrSessionClosed) {
				return err
			}
			if strings.Contains(err.Error(), "not found") {
				return sess.CreateEncounter(sid, st.ID, raw)
			}
			return err
		}
		return nil
	}

	filterParticipants := func(principal vault.Principal, parts []encounterParticipant) []encounterParticipant {
		if principal.IsGM() {
			return parts
		}
		out := []encounterParticipant{}
		for _, pp := range parts {
			if pp.UID == "" {
				continue
			}
			if vlt == nil {
				continue
			}
			n := vlt.GetNote(pp.UID)
			if n == nil {
				continue
			}
			if !vault.Visible(principal, *n) {
				continue
			}
			out = append(out, pp)
		}
		return out
	}

	projectEncounter := func(sid string, st *encounterState, principal vault.Principal, seatID string) map[string]any {
		visibleTokens := []string{}
		visibleTokenObjs := []encounterToken{}
		hiddenTokens := []string{}
		for _, tok := range st.Tokens {
			if tok.Hidden {
				if principal.IsGM() {
					visibleTokens = append(visibleTokens, tok.ID)
					visibleTokenObjs = append(visibleTokenObjs, tok)
					continue
				}
				hiddenTokens = append(hiddenTokens, tok.ID)
				continue
			}
			if len(tok.VisibleTo) > 0 {
				allowed := false
				for _, s := range tok.VisibleTo {
					if s == seatID || s == principal.ID || s == "*" {
						allowed = true
					}
				}
				if !allowed && !principal.IsGM() {
					hiddenTokens = append(hiddenTokens, tok.ID)
					continue
				}
			}
			if tok.NoteUID != "" && vlt != nil {
				n := vlt.GetNote(tok.NoteUID)
				if n != nil {
					if !vault.Visible(principal, *n) {
						hiddenTokens = append(hiddenTokens, tok.ID)
						continue
					}
				} else {
					if !principal.IsGM() {
						hiddenTokens = append(hiddenTokens, tok.ID)
						continue
					}
				}
			}
			visibleTokens = append(visibleTokens, tok.ID)
			visibleTokenObjs = append(visibleTokenObjs, tok)
		}
		visibleNotes := []string{}
		seen := map[string]bool{}
		for _, tok := range visibleTokenObjs {
			if tok.NoteUID != "" && !seen[tok.NoteUID] {
				seen[tok.NoteUID] = true
				visibleNotes = append(visibleNotes, tok.NoteUID)
			}
		}
		for _, pp := range st.Participants {
			if pp.UID != "" && vlt != nil {
				n := vlt.GetNote(pp.UID)
				if n == nil {
					// skip unknown notes (latent fix i)
					continue
				}
				if vault.Visible(principal, *n) {
					if !seen[pp.UID] {
						seen[pp.UID] = true
						visibleNotes = append(visibleNotes, pp.UID)
					}
				}
			}
		}
		sort.Strings(visibleTokens)
		sort.Strings(hiddenTokens)
		sort.Strings(visibleNotes)
		res := map[string]any{
			"seat":           seatID,
			"seat_id":        seatID,
			"principal":      principal.Kind,
			"encounter":      st.ID,
			"encounter_id":   st.ID,
			"session":        sid,
			"session_id":     sid,
			"visible_tokens": visibleTokens,
			"visibleTokens":  visibleTokens,
			"tokens":         visibleTokenObjs,
			"visible_notes":  visibleNotes,
			"visibleNotes":   visibleNotes,
			"note_refs":      visibleNotes,
			"hidden_tokens":  hiddenTokens,
			"hiddenTokens":   hiddenTokens,
		}
		if principal.IsGM() {
			res["encounter_obj"] = st
			res["encounter"] = st
		} else {
			// For non-GM, return projected view - do not include raw hidden tokens/GM notes; structure per fog semantics
			res["encounter"] = map[string]any{
				"id":                st.ID,
				"uid":               st.UID,
				"name":              st.Name,
				"system":            st.System,
				"pack":              st.Pack,
				"campaign":          st.Campaign,
				"session_id":        st.SessionID,
				"session":           st.Session,
				"initiative":        st.Initiative,
				"initiative_order":  st.InitiativeOrder,
				"current_turn":      st.CurrentTurn,
				"turn":              st.Turn,
				"round":             st.Round,
				"tokens":            visibleTokenObjs,
				"participants":      filterParticipants(principal, st.Participants),
				"budget":            st.Budget,
				"opposition_budget": st.OppositionBudget,
			}
		}
		return res
	}

	budgetFor := func(system string, st *encounterState, vlt *vault.Vault) map[string]any {
		pk, err := pack.Load(system)
		if err != nil {
			// try example? no
			pk = nil
		}
		if pk == nil || pk.Difficulty == nil {
			return map[string]any{
				"model":   "",
				"error":   "unsupported",
				"code":    "unsupported_difficulty_model",
				"message": "pack has no difficulty model",
			}
		}
		model, _ := pk.Difficulty["model"].(string)
		if strings.TrimSpace(model) == "" {
			return map[string]any{
				"model":   "",
				"error":   "unsupported",
				"code":    "unsupported_difficulty_model",
				"message": "pack difficulty model is empty",
			}
		}
		if strings.TrimSpace(model) == "none" {
			return map[string]any{
				"model":   "none",
				"error":   "unsupported",
				"code":    "unsupported_difficulty_model",
				"message": "pack has no difficulty model",
			}
		}
		out := map[string]any{"model": model}
		switch model {
		case "xp-budget":
			bpl := 0
			switch v := pk.Difficulty["budgetPerLevel"].(type) {
			case int:
				bpl = v
			case float64:
				bpl = int(v)
			}
			if bpl == 0 {
				bpl = 1
			}
			// sum levels of party-side participants (default level 1)
			partyLevels := 0
			partyCount := 0
			for _, pp := range st.Participants {
				side := strings.ToLower(pp.Side)
				if side == "" || side == "party" || side == "pc" || side == "player" || side == "ally" || side == "allies" {
					partyCount++
					lvl := 1
					if vlt != nil {
						if n := vlt.GetNote(pp.UID); n != nil {
							for _, key := range []string{"level", "lvl"} {
								if v, ok := n.Fields[key]; ok {
									av := v
									if fv, ok := v.(frontmatter.Value); ok {
										av = fmValueToAny(fv)
									}
									switch t := av.(type) {
									case int:
										if t > 0 {
											lvl = t
										}
									case float64:
										if int(t) > 0 {
											lvl = int(t)
										}
									case int64:
										if int(t) > 0 {
											lvl = int(t)
										}
									}
								}
							}
						}
					}
					partyLevels += lvl
				}
			}
			if partyCount == 0 {
				partyCount = len(st.Participants)
				if partyCount == 0 {
					partyCount = 1
				}
				partyLevels = partyCount
			}
			budget := bpl * partyLevels
			out["budget"] = budget
			out["xp"] = budget
			out["xp_budget"] = budget
			out["budgetPerLevel"] = bpl
			out["partyLevels"] = partyLevels
			out["partyCount"] = partyCount
		case "opposition-dice":
			// opposition = sum of enemy opposition ratings (default 1 per enemy)
			opp := 0
			enemies := 0
			for _, pp := range st.Participants {
				side := strings.ToLower(pp.Side)
				if side == "enemy" || side == "gm" || side == "opposition" || side == "npc" {
					enemies++
					rating := 1
					if vlt != nil {
						if n := vlt.GetNote(pp.UID); n != nil {
							for _, key := range []string{"opposition", "threat", "tier"} {
								if v, ok := n.Fields[key]; ok {
									av := v
									if fv, ok := v.(frontmatter.Value); ok {
										av = fmValueToAny(fv)
									}
									switch t := av.(type) {
									case int:
										if t > 0 {
											rating = t
										}
									case float64:
										if int(t) > 0 {
											rating = int(t)
										}
									case int64:
										if int(t) > 0 {
											rating = int(t)
										}
									}
								}
							}
						}
					}
					opp += rating
				}
			}
			if enemies == 0 {
				enemies = len(st.Participants)
				opp = enemies
				if opp == 0 {
					opp = 1
					enemies = 1
				}
			}
			out["opposition"] = opp
			out["opposition_dice"] = opp
			out["oppositionDice"] = opp
			out["enemies"] = enemies
			if cp, ok := pk.Difficulty["clockPressure"].(bool); ok {
				out["clockPressure"] = cp
			} else {
				out["clockPressure"] = false
			}
			// clocks = enemy count (pack-native clock pressure)
			out["clocks"] = enemies
		default:
			// generic: expose difficulty map
			for k, v := range pk.Difficulty {
				out[k] = v
			}
		}
		out["difficulty"] = pk.Difficulty
		return out
	}

	// helper to parse participants
	parseParticipants := func(body map[string]any) []encounterParticipant {
		out := []encounterParticipant{}
		raw, _ := body["participants"].([]any)
		for _, p := range raw {
			pm, _ := p.(map[string]any)
			if pm == nil {
				continue
			}
			pp := encounterParticipant{}
			if v, ok := pm["uid"].(string); ok {
				pp.UID = v
			}
			if pp.UID == "" {
				if v, ok := pm["character"].(string); ok {
					pp.UID = v
				}
			}
			if pp.UID == "" {
				if v, ok := pm["note"].(string); ok {
					pp.UID = v
				}
			}
			if pp.UID == "" {
				if v, ok := pm["note_uid"].(string); ok {
					pp.UID = v
				}
			}
			if v, ok := pm["side"].(string); ok {
				pp.Side = v
			}
			if v, ok := pm["team"].(string); ok && pp.Side == "" {
				pp.Side = v
			}
			out = append(out, pp)
		}
		return out
	}

	parseTokens := func(body map[string]any) []encounterToken {
		out := []encounterToken{}
		raw, _ := body["tokens"].([]any)
		for _, t := range raw {
			tm, _ := t.(map[string]any)
			if tm == nil {
				continue
			}
			tok := encounterToken{}
			if v, ok := tm["id"].(string); ok {
				tok.ID = v
			}
			if tok.ID == "" {
				if v, ok := tm["token_id"].(string); ok {
					tok.ID = v
				}
			}
			if tok.ID == "" {
				id, _ := uid.New()
				tok.ID = id
			}
			if v, ok := tm["uid"].(string); ok {
				tok.UID = v
				if tok.NoteUID == "" {
					tok.NoteUID = v
				}
			}
			if v, ok := tm["note_uid"].(string); ok {
				tok.NoteUID = v
			}
			if v, ok := tm["note"].(string); ok && tok.NoteUID == "" {
				tok.NoteUID = v
			}
			if v, ok := tm["side"].(string); ok {
				tok.Side = v
			}
			if v, ok := tm["hidden"].(bool); ok {
				tok.Hidden = v
			}
			if v, ok := tm["visible_to"].([]any); ok {
				for _, x := range v {
					if s, ok := x.(string); ok {
						tok.VisibleTo = append(tok.VisibleTo, s)
					}
				}
			}
			if v, ok := tm["x"].(float64); ok {
				tok.X = v
			} else if v, ok := tm["x"].(int); ok {
				tok.X = v
			}
			if v, ok := tm["y"].(float64); ok {
				tok.Y = v
			} else if v, ok := tm["y"].(int); ok {
				tok.Y = v
			}
			out = append(out, tok)
		}
		return out
	}

	getSystem := func(body map[string]any, vlt *vault.Vault) string {
		if v, ok := body["system"].(string); ok && v != "" {
			return v
		}
		if v, ok := body["pack"].(string); ok && v != "" {
			return v
		}
		return "example"
	}

	// create handler
	createHandler := func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("sid")
		principal := principalFromRequest(r, tk)
		if !principal.IsGM() {
			writeErr(w, "forbidden", "gm required", http.StatusForbidden)
			return
		}
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if sid == "" {
			if v, ok := body["session"].(string); ok {
				sid = v
			}
			if sid == "" {
				if v, ok := body["session_id"].(string); ok {
					sid = v
				}
			}
			if sid == "" {
				if v, ok := body["sessionId"].(string); ok {
					sid = v
				}
			}
		}
		if sid == "" {
			writeErr(w, "invalid", "session required", http.StatusBadRequest)
			return
		}
		ss, err := sess.GetSession(sid)
		if err != nil || ss == nil {
			writeErr(w, "not_found", "no such session", http.StatusNotFound)
			return
		}
		if ss.Status == session.StatusClosed {
			writeErr(w, "session_closed", "session is closed", http.StatusConflict)
			return
		}
		name := "Encounter"
		if v, ok := body["name"].(string); ok && v != "" {
			name = v
		}
		system := getSystem(body, vlt)
		campaign := ""
		if v, ok := body["campaign"].(string); ok {
			campaign = v
		}
		uidVal, _ := uid.New()
		if uidVal == "" {
			uidVal = fmt.Sprintf("enc-%d", time.Now().UnixNano())
		}
		parts := parseParticipants(body)
		tokens := parseTokens(body)
		st := &encounterState{
			ID:           uidVal,
			UID:          uidVal,
			SessionID:    sid,
			Session:      sid,
			Name:         name,
			System:       system,
			Pack:         system,
			Campaign:     campaign,
			Participants: parts,
			Initiative:   []string{},
			CurrentTurn:  0,
			Round:        1,
			Tokens:       tokens,
		}
		st.Budget = budgetFor(system, st, vlt)
		st.syncAliases()
		// create vault note
		// NOTE (F2b): live encounter state is stored in the session store as a snapshot at mutation time;
		// mutations do not sync back to the vault note on every turn/update (vault-note/create-time snapshot divergence).
		// The vault note is a registration/artifact created at create time; F4 must not assume per-turn vault mutation.
		if vlt != nil {
			rel := "Encounters/" + safeFileName(name) + ".md"
			full := filepath.Join(vlt.Root, filepath.FromSlash(rel))
			os.MkdirAll(filepath.Dir(full), 0o755)
			var b strings.Builder
			b.WriteString("---\n")
			b.WriteString("type: encounter\n")
			b.WriteString(fmt.Sprintf("uid: %q\n", uidVal))
			b.WriteString(fmt.Sprintf("name: %s\n", name))
			b.WriteString(fmt.Sprintf("system: %s\n", system))
			b.WriteString(fmt.Sprintf("campaign: %q\n", campaign))
			b.WriteString("visibility: gm\n")
			if len(parts) > 0 {
				pb, _ := json.Marshal(parts)
				b.WriteString("participants: " + string(pb) + "\n")
			}
			b.WriteString(fmt.Sprintf("session: %s\n", sid))
			b.WriteString("---\n\n# " + name + "\n\n")
			_ = os.WriteFile(full, []byte(b.String()), 0o644)
			_ = vlt.Load()
		}
		raw, _ := json.Marshal(st)
		if err := sess.CreateEncounter(sid, uidVal, raw); err != nil {
			// ignore duplicate
		}
		if globalHub != nil {
			globalHub.broadcast(map[string]any{"v": 1, "type": "vault.changed", "data": map[string]any{"uid": uidVal, "kind": "created"}})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"encounter":         st,
			"uid":               uidVal,
			"id":                uidVal,
			"session":           sid,
			"session_id":        sid,
			"budget":            st.Budget,
			"opposition_budget": st.Budget,
		})
	}

	getHandler := func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("sid")
		eid := r.PathValue("eid")
		if eid == "" {
			eid = r.PathValue("id")
		}
		if sid == "" {
			// try query
			sid = r.URL.Query().Get("session")
			if sid == "" {
				sid = r.URL.Query().Get("session_id")
			}
		}
		principal := principalFromRequest(r, tk)
		if !allowedSession(principal, sid) && !principal.IsGM() {
			if sid == "" {
				if !principal.IsGM() {
					writeErr(w, "unauthorized", "not allowed", http.StatusUnauthorized)
					return
				}
			} else {
				writeErr(w, "unauthorized", "not allowed", http.StatusUnauthorized)
				return
			}
		}
		// note visibility check
		if vlt != nil {
			if n := vlt.GetNote(eid); n != nil {
				if !vault.Visible(principal, *n) && !allowedSession(principal, sid) && !principal.IsGM() {
					writeErr(w, "not_found", "no such encounter", http.StatusNotFound)
					return
				}
			}
		}
		st, ok := loadState(sid, eid)
		if !ok {
			writeErr(w, "not_found", "no such encounter", http.StatusNotFound)
			return
		}
		res := projectEncounter(sid, st, principal, "")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
	}

	listHandler := func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("sid")
		if sid == "" {
			sid = r.URL.Query().Get("session")
		}
		if sid == "" {
			sid = r.URL.Query().Get("session_id")
		}
		principal := principalFromRequest(r, tk)
		if sid != "" && !allowedSession(principal, sid) && !principal.IsGM() {
			writeErr(w, "unauthorized", "not allowed", http.StatusUnauthorized)
			return
		}
		ids := []string{}
		if sess != nil && sid != "" {
			if l, err := sess.ListEncounters(sid); err == nil {
				ids = l
			}
		}
		// also vault notes of type encounter for session/campaign
		items := []map[string]any{}
		for _, id := range ids {
			if st, ok := loadState(sid, id); ok {
				items = append(items, map[string]any{"id": id, "uid": id, "encounter": st})
			} else {
				items = append(items, map[string]any{"id": id, "uid": id})
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "count": len(items)})
	}

	updateHandler := func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("sid")
		eid := r.PathValue("eid")
		if eid == "" {
			eid = r.PathValue("id")
		}
		principal := principalFromRequest(r, tk)
		if !principal.IsGM() {
			writeErr(w, "forbidden", "gm required", http.StatusForbidden)
			return
		}
		if sid == "" {
			sid = r.URL.Query().Get("session")
			if sid == "" {
				sid = r.URL.Query().Get("session_id")
			}
		}
		// enforce session scoping explicitly
		if sid == "" {
			writeErr(w, "bad_request", "session required", http.StatusBadRequest)
			return
		}
		if sid != "" {
			if s2, _ := sess.GetSession(sid); s2 != nil && s2.Status == session.StatusClosed {
				writeErr(w, "session_closed", "session is closed", http.StatusConflict)
				return
			}
		}
		// closed check via store update will catch; but pre-check if possible
		st, ok := loadState(sid, eid)
		if !ok {
			st = &encounterState{ID: eid, UID: eid, SessionID: sid}
		}
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if v, ok := body["name"].(string); ok && v != "" {
			st.Name = v
		}
		if v, ok := body["system"].(string); ok && v != "" {
			st.System = v
			st.Pack = v
		}
		if v, ok := body["pack"].(string); ok && v != "" {
			st.System = v
			st.Pack = v
		}
		if v, ok := body["campaign"].(string); ok {
			st.Campaign = v
		}
		if p := parseParticipants(body); len(p) > 0 {
			st.Participants = p
		}
		if t := parseTokens(body); len(t) > 0 {
			st.Tokens = t
		}
		if v, ok := body["initiative"].([]any); ok {
			st.Initiative = toStringSlice(v)
		}
		if v, ok := body["initiative_order"].([]any); ok {
			st.Initiative = toStringSlice(v)
		}
		if v, ok := body["round"].(float64); ok {
			st.Round = int(v)
		}
		if v, ok := body["current_turn"].(float64); ok {
			st.CurrentTurn = int(v)
		}
		if v, ok := body["turn"].(float64); ok {
			st.CurrentTurn = int(v)
		}
		st.Budget = budgetFor(st.System, st, vlt)
		if err := saveState(sid, st); err != nil {
			if err == session.ErrSessionClosed {
				writeErr(w, "session_closed", "session is closed", http.StatusConflict)
				return
			}
			writeErr(w, "internal", "update failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"encounter": st, "id": st.ID, "uid": st.UID})
	}

	// initiative roll
	initRollHandler := func(w http.ResponseWriter, r *http.Request) {
		sid, eid := pathSE(r)
		principal := principalFromRequest(r, tk)
		if !principal.IsGM() {
			writeErr(w, "forbidden", "gm required", http.StatusForbidden)
			return
		}
		if sid == "" {
			writeErr(w, "bad_request", "session required", http.StatusBadRequest)
			return
		}
		if sid != "" {
			if ss, _ := sess.GetSession(sid); ss != nil && ss.Status == session.StatusClosed {
				writeErr(w, "session_closed", "session is closed", http.StatusConflict)
				return
			}
		}
		st, ok := loadState(sid, eid)
		if !ok {
			writeErr(w, "not_found", "no such encounter", http.StatusNotFound)
			return
		}
		// build participant uids
		uids := []string{}
		for _, p := range st.Participants {
			uids = append(uids, p.UID)
		}
		if len(uids) == 0 {
			uids = st.Initiative
		}
		// roll 1d20 for each, sort desc
		type roll struct {
			uid string
			val int
		}
		rolls := make([]roll, 0, len(uids))
		for _, u := range uids {
			rolls = append(rolls, roll{uid: u, val: rand.Intn(20) + 1})
		}
		sort.Slice(rolls, func(i, j int) bool { return rolls[i].val > rolls[j].val })
		order := make([]string, 0, len(rolls))
		for _, rr := range rolls {
			order = append(order, rr.uid)
		}
		st.Initiative = order
		st.CurrentTurn = 0
		st.Round = 1
		if err := saveState(sid, st); err != nil {
			if err == session.ErrSessionClosed {
				writeErr(w, "session_closed", "session is closed", http.StatusConflict)
				return
			}
			writeErr(w, "internal", "initiative failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"encounter": st, "initiative": order, "order": order})
	}

	initSetHandler := func(w http.ResponseWriter, r *http.Request) {
		sid, eid := pathSE(r)
		principal := principalFromRequest(r, tk)
		if !principal.IsGM() {
			writeErr(w, "forbidden", "gm required", http.StatusForbidden)
			return
		}
		if sid == "" {
			writeErr(w, "bad_request", "session required", http.StatusBadRequest)
			return
		}
		if sid != "" {
			if ss, _ := sess.GetSession(sid); ss != nil && ss.Status == session.StatusClosed {
				writeErr(w, "session_closed", "session is closed", http.StatusConflict)
				return
			}
		}
		st, ok := loadState(sid, eid)
		if !ok {
			writeErr(w, "not_found", "no such encounter", http.StatusNotFound)
			return
		}
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		order := []string{}
		for _, key := range []string{"order", "initiative", "initiative_order", "uids"} {
			if v, ok := body[key].([]any); ok {
				order = toStringSlice(v)
				break
			}
		}
		if len(order) == 0 {
			writeErr(w, "invalid", "order required", http.StatusBadRequest)
			return
		}
		st.Initiative = order
		st.CurrentTurn = 0
		if v, ok := body["round"].(float64); ok {
			st.Round = int(v)
		}
		if v, ok := body["current_turn"].(float64); ok {
			st.CurrentTurn = int(v)
		}
		if err := saveState(sid, st); err != nil {
			if err == session.ErrSessionClosed {
				writeErr(w, "session_closed", "session is closed", http.StatusConflict)
				return
			}
			writeErr(w, "internal", "set failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"encounter": st, "initiative": st.Initiative, "order": st.Initiative})
	}

	reorderHandler := initSetHandler

	advanceRoundHandler := func(w http.ResponseWriter, r *http.Request) {
		sid, eid := pathSE(r)
		principal := principalFromRequest(r, tk)
		if !principal.IsGM() {
			writeErr(w, "forbidden", "gm required", http.StatusForbidden)
			return
		}
		if sid == "" {
			writeErr(w, "bad_request", "session required", http.StatusBadRequest)
			return
		}
		if sid != "" {
			if ss, _ := sess.GetSession(sid); ss != nil && ss.Status == session.StatusClosed {
				writeErr(w, "session_closed", "session is closed", http.StatusConflict)
				return
			}
		}
		st, ok := loadState(sid, eid)
		if !ok {
			writeErr(w, "not_found", "no such encounter", http.StatusNotFound)
			return
		}
		st.Round++
		st.CurrentTurn = 0
		if err := saveState(sid, st); err != nil {
			if err == session.ErrSessionClosed {
				writeErr(w, "session_closed", "session is closed", http.StatusConflict)
				return
			}
			writeErr(w, "internal", "advance failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"encounter": st, "round": st.Round})
	}

	advanceTurnHandler := func(w http.ResponseWriter, r *http.Request) {
		sid, eid := pathSE(r)
		principal := principalFromRequest(r, tk)
		if !principal.IsGM() {
			writeErr(w, "forbidden", "gm required", http.StatusForbidden)
			return
		}
		if sid == "" {
			writeErr(w, "bad_request", "session required", http.StatusBadRequest)
			return
		}
		if sid != "" {
			if ss, _ := sess.GetSession(sid); ss != nil && ss.Status == session.StatusClosed {
				writeErr(w, "session_closed", "session is closed", http.StatusConflict)
				return
			}
		}
		st, ok := loadState(sid, eid)
		if !ok {
			writeErr(w, "not_found", "no such encounter", http.StatusNotFound)
			return
		}
		if len(st.Initiative) > 0 {
			st.CurrentTurn++
			if st.CurrentTurn >= len(st.Initiative) {
				st.CurrentTurn = 0
				st.Round++
			}
		}
		if err := saveState(sid, st); err != nil {
			if err == session.ErrSessionClosed {
				writeErr(w, "session_closed", "session is closed", http.StatusConflict)
				return
			}
			writeErr(w, "internal", "advance failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"encounter": st, "current_turn": st.CurrentTurn, "turn": st.CurrentTurn, "round": st.Round})
	}

	// generic advance with type
	advanceHandler := func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		typ, _ := body["type"].(string)
		if typ == "round" {
			advanceRoundHandler(w, r)
			return
		}
		advanceTurnHandler(w, r)
	}

	// fog handler
	fogHandler := func(w http.ResponseWriter, r *http.Request) {
		sid, eid := pathSE(r)
		principal := principalFromRequest(r, tk)
		// seat override from query/body
		seatID := r.URL.Query().Get("seat")
		if seatID == "" {
			seatID = r.URL.Query().Get("seat_id")
		}
		if seatID == "" {
			b := map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&b)
			if v, ok := b["seat"].(string); ok {
				seatID = v
			}
			if seatID == "" {
				if v, ok := b["seat_id"].(string); ok {
					seatID = v
				}
			}
		}
		if seatID != "" {
			if !principal.IsGM() {
				writeErr(w, "forbidden", "gm required", http.StatusForbidden)
				return
			}
			if sid != "" {
				if st, ok, _ := sess.FindSeat(sid, seatID); ok {
					p := vault.Seat(st.ID, st.Character, principal.Campaigns...)
					p.Session = sid
					principal = p
				} else {
					p := vault.Seat(seatID, "", principal.Campaigns...)
					p.Session = sid
					principal = p
				}
			} else {
				p := vault.Seat(seatID, "", principal.Campaigns...)
				p.Session = ""
				principal = p
			}
		}
		if !allowedSession(principal, sid) && !principal.IsGM() {
			if sid != "" {
				writeErr(w, "unauthorized", "not allowed", http.StatusUnauthorized)
				return
			}
		}
		st, ok := loadState(sid, eid)
		if !ok {
			writeErr(w, "not_found", "no such encounter", http.StatusNotFound)
			return
		}
		res := projectEncounter(sid, st, principal, seatID)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
	}

	budgetHandler := func(w http.ResponseWriter, r *http.Request) {
		sid, eid := pathSE(r)
		principal := principalFromRequest(r, tk)
		if !principal.IsGM() && !allowedSession(principal, sid) {
			writeErr(w, "unauthorized", "not allowed", http.StatusUnauthorized)
			return
		}
		st, ok := loadState(sid, eid)
		if !ok {
			writeErr(w, "not_found", "no such encounter", http.StatusNotFound)
			return
		}
		b := budgetFor(st.System, st, vlt)
		st.Budget = b
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"budget":            b,
			"opposition_budget": b,
			"model":             b["model"],
		})
	}

	// pathSE helper defined below as closure? We'll use function.
	// Register routes.
	// Session-scoped create/list
	m.HandleFunc("POST /api/v1/sessions/{sid}/encounters", createHandler)
	m.HandleFunc("GET /api/v1/sessions/{sid}/encounters", listHandler)
	// top-level create/list
	m.HandleFunc("POST /api/v1/encounters", createHandler)
	m.HandleFunc("GET /api/v1/encounters", listHandler)

	// get/update (top-level and session-scoped)
	getH := getHandler
	updH := updateHandler
	m.HandleFunc("GET /api/v1/encounters/{eid}", getH)
	m.HandleFunc("PATCH /api/v1/encounters/{eid}", updH)
	m.HandleFunc("PUT /api/v1/encounters/{eid}", updH)
	m.HandleFunc("POST /api/v1/encounters/{eid}", updH)
	m.HandleFunc("GET /api/v1/sessions/{sid}/encounters/{eid}", getH)
	m.HandleFunc("PATCH /api/v1/sessions/{sid}/encounters/{eid}", updH)
	m.HandleFunc("PUT /api/v1/sessions/{sid}/encounters/{eid}", updH)
	m.HandleFunc("POST /api/v1/sessions/{sid}/encounters/{eid}", updH)

	// initiative
	m.HandleFunc("POST /api/v1/encounters/{eid}/initiative/roll", initRollHandler)
	m.HandleFunc("POST /api/v1/encounters/{eid}/initiative/set", initSetHandler)
	m.HandleFunc("POST /api/v1/encounters/{eid}/reorder", reorderHandler)
	m.HandleFunc("POST /api/v1/encounters/{eid}/initiative", func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["order"]; ok {
			initSetHandler(w, r)
			return
		}
		if _, ok := body["initiative"]; ok {
			initSetHandler(w, r)
			return
		}
		if act, _ := body["action"].(string); act == "roll" {
			initRollHandler(w, r)
			return
		}
		initRollHandler(w, r)
	})
	m.HandleFunc("POST /api/v1/sessions/{sid}/encounters/{eid}/initiative/roll", initRollHandler)
	m.HandleFunc("POST /api/v1/sessions/{sid}/encounters/{eid}/initiative/set", initSetHandler)
	m.HandleFunc("POST /api/v1/sessions/{sid}/encounters/{eid}/reorder", reorderHandler)
	m.HandleFunc("POST /api/v1/sessions/{sid}/encounters/{eid}/initiative", func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["order"]; ok {
			initSetHandler(w, r)
			return
		}
		if _, ok := body["initiative"]; ok {
			initSetHandler(w, r)
			return
		}
		if act, _ := body["action"].(string); act == "roll" {
			initRollHandler(w, r)
			return
		}
		initRollHandler(w, r)
	})

	// round/turn advance
	for _, pth := range []string{
		"POST /api/v1/encounters/{eid}/round/advance",
		"POST /api/v1/encounters/{eid}/advance-round",
		"POST /api/v1/encounters/{eid}/round",
	} {
		m.HandleFunc(pth, advanceRoundHandler)
	}
	for _, pth := range []string{
		"POST /api/v1/encounters/{eid}/turn/advance",
		"POST /api/v1/encounters/{eid}/advance-turn",
		"POST /api/v1/encounters/{eid}/next",
		"POST /api/v1/encounters/{eid}/next-turn",
	} {
		m.HandleFunc(pth, advanceTurnHandler)
	}
	m.HandleFunc("POST /api/v1/encounters/{eid}/advance", advanceHandler)
	for _, pth := range []string{
		"POST /api/v1/sessions/{sid}/encounters/{eid}/round/advance",
		"POST /api/v1/sessions/{sid}/encounters/{eid}/advance-round",
		"POST /api/v1/sessions/{sid}/encounters/{eid}/round",
	} {
		m.HandleFunc(pth, advanceRoundHandler)
	}
	for _, pth := range []string{
		"POST /api/v1/sessions/{sid}/encounters/{eid}/turn/advance",
		"POST /api/v1/sessions/{sid}/encounters/{eid}/advance-turn",
		"POST /api/v1/sessions/{sid}/encounters/{eid}/next",
		"POST /api/v1/sessions/{sid}/encounters/{eid}/next-turn",
	} {
		m.HandleFunc(pth, advanceTurnHandler)
	}
	m.HandleFunc("POST /api/v1/sessions/{sid}/encounters/{eid}/advance", advanceHandler)

	// fog/visibility
	for _, pth := range []string{
		"GET /api/v1/encounters/{eid}/fog",
		"GET /api/v1/encounters/{eid}/visibility",
		"GET /api/v1/encounters/{eid}/visible",
		"POST /api/v1/encounters/{eid}/fog",
	} {
		m.HandleFunc(pth, fogHandler)
	}
	for _, pth := range []string{
		"GET /api/v1/sessions/{sid}/encounters/{eid}/fog",
		"GET /api/v1/sessions/{sid}/encounters/{eid}/visibility",
		"GET /api/v1/sessions/{sid}/encounters/{eid}/visible",
		"POST /api/v1/sessions/{sid}/encounters/{eid}/fog",
	} {
		m.HandleFunc(pth, fogHandler)
	}

	// budget
	for _, pth := range []string{
		"GET /api/v1/encounters/{eid}/budget",
		"POST /api/v1/encounters/{eid}/budget",
	} {
		m.HandleFunc(pth, budgetHandler)
	}
	for _, pth := range []string{
		"GET /api/v1/sessions/{sid}/encounters/{eid}/budget",
		"POST /api/v1/sessions/{sid}/encounters/{eid}/budget",
	} {
		m.HandleFunc(pth, budgetHandler)
	}

	// rolls (session tier, encounter scoped)
	rollsHandler := func(w http.ResponseWriter, r *http.Request) {
		sid, eid := pathSE(r)
		principal := principalFromRequest(r, tk)
		if sid != "" && !allowedSession(principal, sid) && !principal.IsGM() {
			writeErr(w, "unauthorized", "not allowed", http.StatusUnauthorized)
			return
		}
		if sid == "" && !principal.IsGM() {
			writeErr(w, "unauthorized", "not allowed", http.StatusUnauthorized)
			return
		}
		var items []any
		if sess != nil {
			if recs, err := sess.ListRolls(sid, eid); err == nil {
				for _, rec := range recs {
					if !rec.Shared && !principal.IsGM() {
						continue
					}
					items = append(items, rec)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "count": len(items)})
	}
	for _, pth := range []string{
		"GET /api/v1/sessions/{sid}/encounters/{eid}/rolls",
		"GET /api/v1/sessions/{sid}/rolls",
		"GET /api/v1/encounters/{eid}/rolls",
	} {
		m.HandleFunc(pth, rollsHandler)
	}

	_ = slog.Default
}

func pathSE(r *http.Request) (string, string) {
	sid := r.PathValue("sid")
	eid := r.PathValue("eid")
	if eid == "" {
		eid = r.PathValue("id")
	}
	if sid == "" {
		// query fallback
		sid = r.URL.Query().Get("session")
		if sid == "" {
			sid = r.URL.Query().Get("session_id")
		}
	}
	return sid, eid
}

func toStringSlice(v []any) []string {
	out := []string{}
	for _, x := range v {
		switch t := x.(type) {
		case string:
			out = append(out, t)
		case map[string]any:
			if u, ok := t["uid"].(string); ok {
				out = append(out, u)
			} else if u, ok := t["id"].(string); ok {
				out = append(out, u)
			}
		}
	}
	return out
}

func safeFileName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		s = "Encounter"
	}
	// simple
	out := ""
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == ' ' {
			out += string(r)
		}
	}
	out = strings.TrimSpace(out)
	if out == "" {
		out = "Encounter"
	}
	return out
}
