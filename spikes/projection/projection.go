package main

import (
	"strings"

	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
	"gopkg.in/yaml.v3"
)

// PrincipalKind is the kind of caller (ADR-0038).
type PrincipalKind string

const (
	PrincipalGM        PrincipalKind = "gm"
	PrincipalAccount   PrincipalKind = "account"
	PrincipalSeat      PrincipalKind = "seat"
	PrincipalDisplay   PrincipalKind = "display"
	PrincipalAnonymous PrincipalKind = "anonymous"
)

// Principal is who is asking.
//
// A principal says *who*; the visibility model says *what they may see*.
type Principal struct {
	Kind PrincipalKind
	// ID is the account id or seat id.
	ID string
	// Campaign is the campaign a display principal represents.
	Campaign string
	// BoundUID is the character a seat is bound to.
	BoundUID string
	// Grants are grants made to this seat.
	Grants []string
	// Label is for reports.
	Label string
}

func (p Principal) String() string {
	if p.Label != "" {
		return p.Label
	}
	return string(p.Kind)
}

// ScopePolicy decides how rule 1 (campaign scope) interacts with rule 5
// (grants) and with the discovery grant.
//
// ADR-0033 does not settle this. Its rule 1 says "A note with `campaign: X`
// is visible **only** to members of X, plus the GM", while its rule 5 says
// grants are "additional parties who may see the note" and never says a grant
// is bounded by the campaign. session-writeback §6 lists the campaign as a
// *deny*, but — unlike the GM deny and the workflow deny — does not call it
// absolute.
//
// The two readings differ, and the spike measures the difference.
type ScopePolicy string

const (
	// ScopeStrict: the campaign is a hard boundary. A grant to a
	// non-member does not open the note.
	ScopeStrict ScopePolicy = "strict"
	// ScopeGrantOverrides: the campaign filters the *base level* only. A
	// grant or a discovery opens the note to a non-member.
	ScopeGrantOverrides ScopePolicy = "grant-overrides"
)

// Decision is the outcome of resolving one note against one principal.
type Decision struct {
	Visible bool
	// Rule names the rule that decided it, e.g. "rule 2".
	Rule string
	// Detail explains it, in words a human can check.
	Detail string
}

func (d Decision) String() string {
	if !d.Visible {
		return "hidden — " + d.Rule + ": " + d.Detail
	}
	return "visible — " + d.Rule + ": " + d.Detail
}

// Projector resolves notes against principals.
type Projector struct {
	Notes []*Note
	Pack  *Pack
	Scope ScopePolicy

	byPath    map[string]*Note
	byID      map[string]*Note
	byTitle   map[string]*Note
	byAlias   map[string]*Note
	campaigns map[string]*Note
	members   map[string][]string
}

// NewProjector indexes the vault and reads the campaign notes.
//
// A campaign's membership comes from the campaign note's `members` list
// (ADR-0037). A campaign with no campaign note has no members — and so no
// party. That is a finding, not an accident: `visibility: party` on a note
// whose campaign note is missing resolves to nobody, silently.
func NewProjector(notes []*Note, pack *Pack, scope ScopePolicy) *Projector {
	p := &Projector{
		Notes:     notes,
		Pack:      pack,
		Scope:     scope,
		byPath:    map[string]*Note{},
		byID:      map[string]*Note{},
		byTitle:   map[string]*Note{},
		byAlias:   map[string]*Note{},
		campaigns: map[string]*Note{},
		members:   map[string][]string{},
	}
	for _, n := range notes {
		p.byPath[n.Path] = n
		if _, dup := p.byID[n.ID()]; !dup {
			p.byID[n.ID()] = n
		}
		if n.Name != "" {
			if _, dup := p.byTitle[strings.ToLower(n.Name)]; !dup {
				p.byTitle[strings.ToLower(n.Name)] = n
			}
		}
		for _, a := range n.Aliases {
			if _, dup := p.byAlias[strings.ToLower(a)]; !dup {
				p.byAlias[strings.ToLower(a)] = n
			}
		}
		if strings.EqualFold(n.Type, "campaign") {
			id := n.Campaign
			if id == "" {
				id = n.UID
			}
			if id != "" {
				p.campaigns[id] = n
			}
		}
	}
	for id, c := range p.campaigns {
		p.members[id] = c.Members
	}
	return p
}

// CampaignNote returns the campaign note for an id, if there is one.
func (p *Projector) CampaignNote(id string) *Note { return p.campaigns[id] }

// Members returns a campaign's members.
func (p *Projector) Members(id string) []string { return p.members[id] }

// EffectiveVisibility is the note's visibility after the default rule
// (ADR-0033 rule 6).
//
// A note with a campaign defaults to `party`; a note without one defaults to
// `gm`. A note with an *unrecognised* visibility has no defined level at
// all — the default rule covers an *absent* value, not an unknown one — and
// the spike fails closed.
func (p *Projector) EffectiveVisibility(n *Note) (string, string) {
	if n.Visibility != "" {
		switch strings.ToLower(n.Visibility) {
		case "public", "party", "owner", "private", "discovered", "gm":
			return strings.ToLower(n.Visibility), ""
		}
		return "", "unrecognised visibility value " + n.Visibility
	}
	if n.Campaign != "" {
		return "party", "absent, and the note has a campaign, so it defaults to party"
	}
	return "gm", "absent, and the note has no campaign, so it defaults to gm"
}

// isMember reports whether the principal is a member of the note's campaign.
//
// A seat is not a campaign member (ADR-0038: "Guests are not campaign
// members"), and neither is the anonymous principal.
func (p *Projector) isMember(pr Principal, n *Note) bool {
	switch pr.Kind {
	case PrincipalAccount:
		if n.Campaign == "" {
			return false
		}
		return contains(p.members[n.Campaign], pr.ID)
	case PrincipalDisplay:
		if n.Campaign != "" && n.Campaign == pr.Campaign {
			// "Party" is defined by the campaign note (ADR-0037). A
			// campaign with no campaign note has no party, so the
			// display has nothing to represent.
			if p.CampaignNote(n.Campaign) == nil {
				return false
			}
			return true
		}
		return false
	}
	return false
}

// Decide resolves one note against one principal.
//
// The order is ADR-0033's: scope, absolute deny, workflow gate, base level,
// grants, default. The display principal is special-cased (ADR-0048): it
// resolves to party-visible content only.
func (p *Projector) Decide(pr Principal, n *Note) Decision {
	// ---- the GM --------------------------------------------------------
	if pr.Kind == PrincipalGM {
		// ADR-0048 says `private` is "the note's player, and nobody else
		// — not even the GM". ADR-0038 says the GM's view is "never
		// filtered". The two disagree; the spike follows ADR-0048, the
		// narrower and more specific rule, and reports the clash.
		if strings.EqualFold(n.Visibility, "private") {
			return Decision{false, "rule 4", "visibility: private — ADR-0048 hides it from the GM as well"}
		}
		return Decision{true, "principal", "the GM principal"}
	}

	// ---- rule 2: the absolute deny --------------------------------------
	if strings.EqualFold(n.Visibility, "gm") {
		return Decision{false, "rule 2", "visibility: gm — the absolute deny; no grant overrides it"}
	}

	// ---- rule 3: the workflow gate --------------------------------------
	if n.Workflow == "draft" || n.Workflow == "submitted" {
		if pr.Kind == PrincipalAccount && n.Player != "" && n.Player == pr.ID {
			return Decision{true, "rule 3", "workflow " + n.Workflow + " — the owner"}
		}
		return Decision{false, "rule 3", "workflow " + n.Workflow + " — the owner and the GM only, regardless of visibility and grants"}
	}

	// ---- the display principal (ADR-0048) --------------------------------
	if pr.Kind == PrincipalDisplay {
		eff, why := p.EffectiveVisibility(n)
		if eff != "party" {
			return Decision{false, "ADR-0048", "the display sees party-visible content only; this note's level is " + eff + " (" + why + ")"}
		}
		if !p.isMember(pr, n) {
			return Decision{false, "ADR-0048", "the display represents campaign " + pr.Campaign + ", and this note is in " + campaignOf(n)}
		}
		return Decision{true, "ADR-0048", "party-visible content in the display's campaign"}
	}

	// ---- rule 4 and rule 6: the base level ------------------------------
	eff, _ := p.EffectiveVisibility(n)
	base := false
	baseDetail := ""
	switch eff {
	case "public":
		if n.Campaign != "" {
			// D13: a public note inside a campaign is public to members of
			// that campaign and to the GM; not to anonymous callers or to
			// members of other campaigns.
			if pr.Kind == PrincipalGM || p.isMember(pr, n) || (pr.Kind == PrincipalAccount && pr.ID != "" && contains(p.Members(n.Campaign), pr.ID)) {
				base = true
				baseDetail = "visibility: public — inside campaign " + campaignOf(n) + ", visible to members and the GM"
			} else {
				baseDetail = "visibility: public — inside campaign " + campaignOf(n) + ", the caller is not a member"
			}
		} else {
			base = true
			baseDetail = "visibility: public — anyone, authenticated or not (ADR-0050 D13)"
		}
	case "party":
		if p.isMember(pr, n) {
			base = true
			baseDetail = "visibility: party — the caller is a member of " + campaignOf(n)
		} else {
			baseDetail = "visibility: party — the caller is not a member of " + campaignOf(n)
		}
	case "owner":
		if pr.Kind == PrincipalAccount && n.Player != "" && n.Player == pr.ID {
			base = true
			baseDetail = "visibility: owner — the caller owns the note"
		} else {
			baseDetail = "visibility: owner — the caller is not the note's player"
		}
	case "private":
		if pr.Kind == PrincipalAccount && n.Player != "" && n.Player == pr.ID {
			base = true
			baseDetail = "visibility: private — the caller owns the note"
		} else {
			baseDetail = "visibility: private — the caller is not the note's player"
		}
	case "discovered":
		base = pr.Kind == PrincipalGM || (pr.Kind == PrincipalAccount && p.discoveredBy(pr.ID, n))
		baseDetail = "visibility: discovered — the GM or a caller whose character discovered this note"
	case "gm":
		baseDetail = "visibility: gm — the GM only"
	default:
		// An unknown visibility value has no level at all. The default
		// rule covers an *absent* value, not an unknown one, so the
		// spike fails closed and says so.
		return Decision{false, "rule 4", "the note's visibility value (" + n.Visibility + ") is not one the model knows; there is no default for an unknown value, so the spike fails closed"}
	}

	// ---- rule 5: grants -------------------------------------------------
	granted := false
	switch pr.Kind {
	case PrincipalAccount:
		granted = contains(n.Grants, pr.ID)
	case PrincipalSeat:
		granted = contains(n.Grants, pr.ID)
	}

	// ---- the discovery grant (session-writeback §6) ----------------------
	discovered := false
	if pr.Kind == PrincipalAccount {
		discovered = p.discoveredBy(pr.ID, n)
	}

	// ---- the seat's bound character (ADR-0038) ---------------------------
	bound := pr.Kind == PrincipalSeat && pr.BoundUID != "" && pr.BoundUID == n.ID()

	// ---- rule 1: the campaign scope --------------------------------------
	if n.Campaign != "" && !p.isMember(pr, n) {
		if p.Scope == ScopeStrict {
			return Decision{false, "rule 1", "the note is in campaign " + n.Campaign + ", and the caller is not a member"}
		}
		// ScopeGrantOverrides: the scope filters the base level only, so
		// fall through to the grants.
	}

	if base {
		return Decision{true, "rule 4", baseDetail}
	}
	if granted {
		return Decision{true, "rule 5", "grants: [" + strings.Join(n.Grants, ", ") + "]"}
	}
	if discovered {
		return Decision{true, "discovery", "the caller's character has discovered this note"}
	}
	if bound {
		return Decision{true, "ADR-0038", "the seat's bound character is implicitly visible to the seat"}
	}
	return Decision{false, "", "no rule grants the " + string(pr.Kind) + " principal access"}
}

// Resolve turns a note reference into the note it names.
//
// A reference is whatever a human writes: a wikilink (`[[The Drowned
// Quarter]]`), a title, an alias, a vault-relative path, or a uid. The
// vault index is what resolves it - design §5.3 says so, and ADR-0050 D4
// makes `discovered` a list of *note references* rather than a list of
// uids, because a reference is what a GM writes and a uid is what the
// tooling compares.
//
// Resolving here rather than comparing strings is the whole point: a
// string comparison against the uid would never match a wikilink, and the
// discovery rule would be inert for every note in the shipped format.
func (p *Projector) Resolve(ref string) *Note {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	// A wikilink: [[Target]] or [[Target|Alias]].
	if strings.HasPrefix(ref, "[[") && strings.HasSuffix(ref, "]]") {
		ref = ref[2 : len(ref)-2]
		if i := strings.Index(ref, "|"); i >= 0 {
			ref = ref[:i]
		}
		ref = strings.TrimSpace(ref)
	}
	// A section anchor or a heading: [[Target#Section]].
	if i := strings.Index(ref, "#"); i >= 0 {
		ref = strings.TrimSpace(ref[:i])
	}
	if n, ok := p.byID[ref]; ok {
		return n
	}
	if n, ok := p.byPath[ref]; ok {
		return n
	}
	lower := strings.ToLower(ref)
	if n, ok := p.byTitle[lower]; ok {
		return n
	}
	if n, ok := p.byAlias[lower]; ok {
		return n
	}
	// A bare file name, and the last path segment.
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		ref = ref[i+1:]
	}
	if i := strings.LastIndex(ref, "."); i > 0 {
		ref = ref[:i]
	}
	lower = strings.ToLower(ref)
	if n, ok := p.byTitle[lower]; ok {
		return n
	}
	if n, ok := p.byAlias[lower]; ok {
		return n
	}
	return nil
}

// discoveredBy reports whether one of the caller's characters lists this
// note in its `discovered` field.
func (p *Projector) discoveredBy(accountID string, n *Note) bool {
	if n.UID == "" && n.Path == "" {
		// A note with no uid and no path cannot be discovered: nothing can
		// name it.
		return false
	}
	for _, c := range p.Notes {
		if c.Player != accountID {
			continue
		}
		for _, ref := range c.Discovered {
			target := p.Resolve(ref)
			if target == nil {
				continue
			}
			if target == n || target.ID() == n.ID() {
				return true
			}
		}
	}
	return false
}

func campaignOf(n *Note) string {
	if n.Campaign == "" {
		return "(no campaign)"
	}
	return n.Campaign
}

// FieldView is one projected field.
type FieldView struct {
	// Path is the field's path: a top-level key, or a dotted path under
	// `data:`.
	Path string
	// Value is the value as it would be serialised.
	Value frontmatter.Value
	// Visible reports whether the principal may see it.
	Visible bool
	// Rule explains the decision.
	Rule string
	// Masked reports whether `display:` replaced the value.
	Masked bool
}

// ProjectedNote is one note as one principal sees it.
type ProjectedNote struct {
	Note *Note
	// Decision is the note-level decision.
	Decision Decision
	// Fields are the visible fields, in document order.
	Fields []FieldView
	// HiddenFields are the fields that were withheld.
	HiddenFields []FieldView
	// Body is the body as the principal would receive it.
	Body []byte
	// Stripped are the secret blocks removed from the body.
	Stripped []Secret
	// Kept are the secret blocks that were kept because they are revealed.
	Kept []Secret
}

// Project is the whole projection for one principal.
type Project struct {
	Principal Principal
	// Notes holds every note, with its decision.
	Notes []ProjectedNote
	// byPath indexes the notes by path.
	byPath map[string]*ProjectedNote
	// Visible counts the notes the principal may see.
	Visible int
	// VisibleFields counts the fields across the visible notes.
	VisibleFields int
	// VisibleBytes counts the body bytes across the visible notes.
	VisibleBytes int
	// Stripped counts the secret blocks withheld.
	Stripped int
}

// Project runs the projection for one principal.
func (p *Projector) Project(pr Principal) *Project {
	out := &Project{Principal: pr, byPath: map[string]*ProjectedNote{}}
	for _, n := range p.Notes {
		pn := p.projectOne(pr, n)
		out.byPath[pn.Note.Path] = &pn
		out.Notes = append(out.Notes, pn)
	}
	for _, pn := range out.Notes {
		if pn.Decision.Visible {
			out.Visible++
			out.VisibleFields += len(pn.Fields)
			out.VisibleBytes += len(pn.Body)
			out.Stripped += len(pn.Stripped)
		}
	}
	return out
}

// projectOne projects one note.
func (p *Projector) projectOne(pr Principal, n *Note) ProjectedNote {
	pn := ProjectedNote{Note: n, Decision: p.Decide(pr, n)}

	secrets := FindSecrets(n.Body)
	body := n.Body
	if pr.Kind != PrincipalGM && len(secrets) > 0 {
		members := []string{}
		if n.Campaign != "" {
			members = p.Members(n.Campaign)
		}
		body, _ = StripSecrets(body, secrets, n.Revealed, pr.Kind, pr.ID, members)
		pn.Stripped = []Secret{}
		pn.Kept = []Secret{}
		for _, s := range secrets {
			if s.isRevealed(n.Revealed, pr.Kind, pr.ID, members) {
				pn.Kept = append(pn.Kept, s)
			} else {
				pn.Stripped = append(pn.Stripped, s)
			}
		}
	}
	if pr.Kind != PrincipalGM {
		body = []byte(stripComments(string(body)))
	}
	pn.Body = body

	if !pn.Decision.Visible {
		// A hidden note contributes nothing: not even its fields.
		return pn
	}

	// Top-level fields.
	if n.Block != nil {
		for _, f := range n.Block.Fields {
			if strings.EqualFold(f.Key, "data") {
				// `data` is walked below, into its leaves.
				continue
			}
			view := p.projectField(pr, n, f.Key, f.ValueNode, nil, nil)
			if view.Visible {
				pn.Fields = append(pn.Fields, view)
			} else {
				pn.HiddenFields = append(pn.HiddenFields, view)
			}
		}
	}

	// The `data:` map, walked into leaf properties.
	if n.Block != nil {
		if v, ok := n.Block.Get("data"); ok && v.Kind == frontmatter.KindMap {
			for _, view := range p.projectMap(pr, n, "data", v) {
				if view.Visible {
					pn.Fields = append(pn.Fields, view)
				} else {
					pn.HiddenFields = append(pn.HiddenFields, view)
				}
			}
		}
	}
	return pn
}

// projectMap walks a mapping value, producing one view per leaf.
func (p *Projector) projectMap(pr Principal, n *Note, path string, v frontmatter.Value) []FieldView {
	var out []FieldView
	for _, e := range v.Map {
		child := path + "." + e.Key
		switch e.Value.Kind {
		case frontmatter.KindMap:
			// A map that carries `value` and/or `display` is a single
			// property with a mask, not a container.
			if _, hasValue := mapEntry(e.Value, "value"); hasValue {
				out = append(out, p.projectField(pr, n, child, nil, nil, &e.Value))
				continue
			}
			if _, hasDisplay := mapEntry(e.Value, "display"); hasDisplay {
				out = append(out, p.projectField(pr, n, child, nil, nil, &e.Value))
				continue
			}
			out = append(out, p.projectMap(pr, n, child, e.Value)...)
		default:
			out = append(out, p.projectField(pr, n, child, nil, &e.Value, nil))
		}
	}
	return out
}

func mapEntry(v frontmatter.Value, key string) (frontmatter.Value, bool) {
	for _, e := range v.Map {
		if strings.EqualFold(e.Key, key) {
			return e.Value, true
		}
	}
	return frontmatter.Value{}, false
}

// projectField decides whether one property is visible to the principal.
//
// A property with no declared visibility inherits the note's. A property
// declared `gm` is absolute. A property declared `owner` resolves against
// the note's `player`, and falls back to the note's visibility when the note
// has no player (ADR-0033 rule 7).
func (p *Projector) projectField(pr Principal, n *Note, path string, node *yaml.Node, rawVal *frontmatter.Value, mapVal *frontmatter.Value) FieldView {
	view := FieldView{Path: path}

	// The value and any mask.
	var raw frontmatter.Value
	var display map[string]frontmatter.Value
	switch {
	case node != nil:
		raw = frontmatter.ValueOf(node)
	case rawVal != nil:
		raw = *rawVal
	case mapVal != nil:
		if v, ok := mapEntry(*mapVal, "value"); ok {
			raw = v
		}
		if d, ok := mapEntry(*mapVal, "display"); ok && d.Kind == frontmatter.KindMap {
			display = map[string]frontmatter.Value{}
			for _, e := range d.Map {
				display[strings.ToLower(e.Key)] = e.Value
			}
		}
	}

	view.Value = raw

	// ADR-0050 D5: the reserved fields declare their own visibility.
	if strings.EqualFold(path, "discovered") || strings.EqualFold(path, "grants") {
		switch pr.Kind {
		case PrincipalGM:
			view.Visible = true
			view.Rule = "reserved-layer visibility: owner — the GM (ADR-0050 D5)"
		case PrincipalAccount:
			if n.Player == "" {
				view.Visible = true
				view.Rule = "reserved-layer visibility: owner — no player, falls back to note visibility (ADR-0050 D5)"
			} else if n.Player == pr.ID {
				view.Visible = true
				view.Rule = "reserved-layer visibility: owner — the note's player (ADR-0050 D5)"
			} else {
				view.Visible = false
				view.Rule = "reserved-layer visibility: owner — the note's player only (ADR-0050 D5)"
			}
		default:
			view.Visible = false
			view.Rule = "reserved-layer visibility: owner — the note's player only (ADR-0050 D5)"
		}
		if view.Visible && len(display) > 0 {
			aud := p.audience(pr, n)
			if m, ok := display[aud]; ok {
				view.Value = m
				view.Masked = true
				view.Rule += "; display:[" + aud + "] masked the value"
			}
		}
		return view
	}

	// The pack's declaration, if any.
	rel := strings.TrimPrefix(path, "data.")
	decl, declared := p.Pack.contentFor(n).declFor(rel)
	if !declared {
		decl, declared = p.Pack.contentFor(n).declFor(path)
	}

	if !declared {
		view.Visible = true
		view.Rule = "inherits the note's visibility — the pack declares nothing for it"
	} else {
		switch strings.ToLower(decl) {
		case "gm":
			view.Visible = false
			view.Rule = "property visibility: gm — the absolute deny"
		case "private":
			view.Visible = pr.Kind == PrincipalAccount && n.Player != "" && n.Player == pr.ID
			view.Rule = "property visibility: private — the note's player only"
		case "owner":
			if n.Player == "" {
				view.Visible = true
				view.Rule = "property visibility: owner — the note has no player, so it falls back to the note's visibility"
			} else {
				view.Visible = pr.Kind == PrincipalAccount && n.Player == pr.ID
				view.Rule = "property visibility: owner — the note's player"
			}
		case "party":
			view.Visible = p.isMember(pr, n)
			view.Rule = "property visibility: party"
		case "public":
			view.Visible = true
			view.Rule = "property visibility: public"
		default:
			view.Visible = false
			view.Rule = "property visibility: " + decl + " — a value the model does not know"
		}
	}

	// Rule 8: masking, applied last.
	if view.Visible && len(display) > 0 {
		aud := p.audience(pr, n)
		if m, ok := display[aud]; ok {
			view.Value = m
			view.Masked = true
			view.Rule += "; display:[" + aud + "] masked the value"
		}
	}
	return view
}

// audience resolves the masking audience for a principal.
func (p *Projector) audience(pr Principal, n *Note) string {
	switch {
	case pr.Kind == PrincipalGM:
		return "gm"
	case pr.Kind == PrincipalDisplay:
		return "party"
	case pr.Kind == PrincipalAccount && n.Player != "" && n.Player == pr.ID:
		return "owner"
	case pr.Kind == PrincipalAccount && p.isMember(pr, n):
		return "party"
	default:
		return "public"
	}
}
