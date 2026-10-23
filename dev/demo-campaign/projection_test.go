package democampaign

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
)

// ---------------------------------------------------------------------------
// The vault
// ---------------------------------------------------------------------------

// note is one campaign note, parsed.
type note struct {
	rel    string // path relative to vault/
	name   string
	typ    string
	uid    string
	vis    string
	wf     string
	player string

	campaignRef string   // the raw `campaign:` value, a link to the campaign note
	grants      []string //
	discovered  []string //
	revealed    []string //
	members     []string // campaign notes only

	body   []byte
	fields map[string]frontmatter.Value
}

// index is the campaign, indexed.
type index struct {
	notes []*note
	byUID map[string]*note
	byKey map[string]*note // filename stem, `name`, and `aliases`, lower-cased
}

const vaultRoot = "vault"

// loadVault parses every note in the campaign.
func loadVault(t *testing.T) *index {
	t.Helper()
	ix := &index{byUID: map[string]*note{}, byKey: map[string]*note{}}
	err := filepath.WalkDir(vaultRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		doc, err := frontmatter.Parse(data)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(vaultRoot, path)
		if err != nil {
			return err
		}
		n := &note{
			rel:    filepath.ToSlash(rel),
			body:   doc.Body,
			fields: map[string]frontmatter.Value{},
		}
		if doc.HasBlock {
			for _, f := range doc.Block.Fields {
				n.fields[f.Key] = frontmatter.ValueOf(f.ValueNode)
			}
		}
		n.name = n.str("name")
		n.typ = n.str("type")
		n.uid = n.str("uid")
		n.vis = n.str("visibility")
		n.wf = n.str("workflow")
		n.player = n.str("player")
		n.campaignRef = n.str("campaign")
		n.grants = n.strList("grants")
		n.discovered = n.strList("discovered")
		n.revealed = n.strList("revealed")
		n.members = n.strList("members")
		ix.notes = append(ix.notes, n)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", vaultRoot, err)
	}
	for _, n := range ix.notes {
		if n.uid != "" {
			if _, dup := ix.byUID[n.uid]; dup {
				t.Errorf("duplicate uid %s", n.uid)
			}
			ix.byUID[n.uid] = n
		}
		stem := strings.TrimSuffix(filepath.Base(n.rel), ".md")
		for _, k := range []string{stem, n.name} {
			if k == "" {
				continue
			}
			lower := strings.ToLower(k)
			if other, dup := ix.byKey[lower]; dup && other != n {
				t.Errorf("two notes answer to %q", k)
			}
			ix.byKey[lower] = n
		}
		for _, a := range n.strList("aliases") {
			ix.byKey[strings.ToLower(a)] = n
		}
	}
	return ix
}

// str returns a top-level string field.
func (n *note) str(key string) string {
	v, ok := n.fields[key]
	if !ok {
		return ""
	}
	return v.Str
}

// strList returns a top-level list-of-strings field.
func (n *note) strList(key string) []string {
	v, ok := n.fields[key]
	if !ok || v.Kind != frontmatter.KindList {
		return nil
	}
	out := make([]string, 0, len(v.List))
	for _, e := range v.List {
		out = append(out, e.Str)
	}
	return out
}

// resolve turns a wikilink into the note it names.
func (ix *index) resolve(target string) *note {
	t := strings.TrimSpace(target)
	t = strings.TrimPrefix(t, "[[")
	t = strings.TrimSuffix(t, "]]")
	if i := strings.Index(t, "|"); i >= 0 {
		t = t[:i]
	}
	if i := strings.Index(t, "#"); i >= 0 {
		t = t[:i]
	}
	return ix.byKey[strings.ToLower(strings.TrimSpace(t))]
}

// membersOf returns the membership of the campaign a note belongs to.
// For a campaign note, that is its own `members` list (ADR-0037).
func (ix *index) membersOf(n *note) []string {
	if n.typ == "campaign" {
		return n.members
	}
	if n.campaignRef == "" {
		return nil
	}
	if c := ix.resolve(n.campaignRef); c != nil {
		return c.members
	}
	return nil
}

// ---------------------------------------------------------------------------
// The schema, in two versions
// ---------------------------------------------------------------------------

// schema is a version of the visibility model. `current` is what the
// accepted ADRs say today; `proposed` is what this spike's findings change
// it to.
type schema struct {
	name string

	// discoveredVisibility is true when `visibility: discovered` exists.
	// When it does not, a note carrying that value has an unknown
	// visibility, and the projection must fail closed.
	discoveredVisibility bool

	// ownerScopedLists is true when `discovered` and `grants` are
	// owner-scoped fields rather than fields that inherit the note's
	// visibility.
	ownerScopedLists bool

	// retiredIsGated is true when `workflow: retired` hides a note from
	// everybody but its owner and the GM. ADR-0009's table says so;
	// ADR-0033's rule 3 does not.
	retiredIsGated bool

	// stripComments is true when Obsidian comments are removed from a
	// non-GM projection.
	stripComments bool
}

var (
	current = schema{name: "current"}

	proposed = schema{
		name:                 "proposed",
		discoveredVisibility: true,
		ownerScopedLists:     true,
		stripComments:        true,
	}
)

// ---------------------------------------------------------------------------
// Principals
// ---------------------------------------------------------------------------

// principal is who is asking (ADR-0038).
type principal struct {
	kind string // "gm", "account", "display", "anonymous"
	id   string
}

var (
	gmPrincipal      = principal{kind: "gm"}
	displayPrincipal = principal{kind: "display"}
	anonymous        = principal{kind: "anonymous"}
	ilva             = principal{kind: "account", id: "p_ilva"}
	orson            = principal{kind: "account", id: "p_orson"}
	wren             = principal{kind: "account", id: "p_wren"}
	everyPlayer      = []principal{ilva, orson, wren}
	everyNonGM       = []principal{ilva, orson, wren, displayPrincipal, anonymous}
	everyPrincipal   = []principal{gmPrincipal, ilva, orson, wren, displayPrincipal, anonymous}
)

// key identifies a principal in the expectations table.
func (p principal) key() string {
	if p.id != "" {
		return p.id
	}
	return p.kind
}

// ---------------------------------------------------------------------------
// The resolver
// ---------------------------------------------------------------------------

// visible resolves ADR-0033's model for one note and one principal. The
// order is the ADR's: scope, absolute deny, workflow gate, base level,
// grants, default.
func (ix *index) visible(n *note, p principal, s schema) bool {
	// `private` is the one value that is not even visible to the GM
	// (ADR-0048). It is checked before the GM shortcut.
	if n.vis == "private" {
		return p.kind == "account" && p.id == n.player
	}

	if p.kind == "gm" {
		return true
	}

	// Rule 2: the absolute deny. No grant overrides it.
	if n.vis == "gm" {
		return false
	}

	members := ix.membersOf(n)

	// Rule 1: scope. A note with a campaign is visible only to that
	// campaign's members (and the GM, who has already returned).
	switch p.kind {
	case "account":
		if n.campaignRef != "" && !slices.Contains(members, p.id) {
			return false
		}
	case "display":
		if n.campaignRef != "" && len(members) == 0 {
			return false
		}
	case "anonymous":
		if n.campaignRef != "" {
			return false
		}
	}

	// Rule 3: the workflow gate. A grant-suppressing deny.
	if n.wf == "draft" || n.wf == "submitted" || (s.retiredIsGated && n.wf == "retired") {
		return p.kind == "account" && p.id == n.player
	}

	// Rule 4: the base level.
	base := false
	switch n.vis {
	case "public":
		// The display resolves to party-visible content only (ADR-0048),
		// so a public document is not on the shared screen.
		base = p.kind != "display"
	case "party":
		base = p.kind == "display" || (p.kind == "account" && slices.Contains(members, p.id))
	case "owner":
		base = p.kind == "account" && p.id == n.player
	case "discovered":
		// A value that does not exist yet. The projection must fail
		// closed rather than guess.
		base = s.discoveredVisibility && ix.discoveredBy(p, n)
	default:
		base = false
	}
	if base {
		return true
	}

	// Rule 5: grants. They widen the base level, but they never override
	// the absolute deny, the workflow gate, or `private` (ADR-0048).
	if n.vis == "private" {
		return false
	}
	return p.kind == "account" && slices.Contains(n.grants, p.id)
}

// discoveredBy reports whether the principal owns a character that has
// discovered this note.
func (ix *index) discoveredBy(p principal, target *note) bool {
	if p.kind != "account" {
		return false
	}
	for _, c := range ix.notes {
		if c.player != p.id {
			continue
		}
		for _, ref := range c.discovered {
			if ix.resolve(ref) == target {
				return true
			}
		}
	}
	return false
}

// view returns the notes a principal may see.
func (ix *index) view(p principal, s schema) []*note {
	var out []*note
	for _, n := range ix.notes {
		if ix.visible(n, p, s) {
			out = append(out, n)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Projection
// ---------------------------------------------------------------------------

// secretCallout matches the opening line of a secret callout (ADR-0035).
var secretCallout = regexp.MustCompile(`^>\s*\[!secret\]`)

// secretBlock is a GM-only block in a note body.
type secretBlock struct {
	start, end int
	id         string
}

// findSecretBlocks locates the GM-only blocks in a body: a `[!secret]`
// callout, or an explicitly marked block.
func findSecretBlocks(body []byte) []secretBlock {
	var out []secretBlock
	s := string(body)
	pos := 0
	for pos < len(s) {
		line, next := lineAt(s, pos)
		trimmed := strings.TrimRight(line, "\r\n")

		if secretCallout.MatchString(trimmed) {
			end := next
			blk := []string{line}
			for end < len(s) {
				l, n := lineAt(s, end)
				if !strings.HasPrefix(strings.TrimSpace(l), ">") {
					break
				}
				blk = append(blk, l)
				end = n
			}
			out = append(out, secretBlock{start: pos, end: end, id: blockID(blk)})
			pos = end
			continue
		}

		if strings.TrimSpace(trimmed) == "<!-- secret:begin -->" {
			end := len(s)
			if j := strings.Index(s[next:], "<!-- secret:end -->"); j >= 0 {
				end = next + j + len("<!-- secret:end -->")
			}
			out = append(out, secretBlock{start: pos, end: end})
			pos = end
			continue
		}

		pos = next
	}
	return out
}

// lineAt returns the line starting at pos and the offset of the next one.
func lineAt(s string, pos int) (string, int) {
	if i := strings.IndexByte(s[pos:], '\n'); i >= 0 {
		return s[pos : pos+i+1], pos + i + 1
	}
	return s[pos:], len(s)
}

// blockID finds an Obsidian block reference at the end of a block.
func blockID(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		t := strings.TrimSpace(lines[i])
		t = strings.TrimSpace(strings.TrimPrefix(t, ">"))
		if t == "" {
			continue
		}
		if j := strings.LastIndex(t, "^"); j >= 0 {
			id := t[j+1:]
			if id != "" && !strings.ContainsAny(id, " \t\r\n") {
				return id
			}
		}
		break
	}
	return ""
}

// projectBody returns the body a principal is allowed to read. The secret
// text is stripped server-side, so it never reaches a non-GM client.
func (ix *index) projectBody(n *note, p principal, s schema) string {
	body := string(n.body)
	if p.kind == "gm" {
		return body
	}
	var b strings.Builder
	last := 0
	for _, blk := range findSecretBlocks(n.body) {
		if blk.id != "" && slices.Contains(n.revealed, blk.id) {
			continue // revealed: the party has already seen it
		}
		b.WriteString(body[last:blk.start])
		last = blk.end
	}
	b.WriteString(body[last:])
	out := b.String()
	if s.stripComments {
		out = stripObsidianComments(out)
	}
	return out
}

// projectFields returns the frontmatter a principal is allowed to read.
func (ix *index) projectFields(n *note, p principal, s schema) map[string]string {
	out := map[string]string{}
	for k, v := range n.fields {
		if s.ownerScopedLists && (k == "discovered" || k == "grants") {
			// A reserved field with a declared visibility of its own.
			if !(p.kind == "account" && p.id == n.player) {
				continue
			}
		}
		out[k] = v.String()
	}
	return out
}

// stripObsidianComments removes `%%...%%` spans.
func stripObsidianComments(s string) string {
	var b strings.Builder
	for {
		i := strings.Index(s, "%%")
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		j := strings.Index(s[i+2:], "%%")
		if j < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		s = s[i+2+j+2:]
	}
}

// squash collapses whitespace, so that a phrase can be matched across the
// line breaks of a paragraph.
func squash(s string) string { return strings.Join(strings.Fields(s), " ") }

// plain strips the markdown callout markers from a body and collapses
// whitespace, so that a phrase can be matched across the line break inside
// a callout.
func plain(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		t = strings.TrimSpace(strings.TrimPrefix(t, ">"))
		if t == "" {
			continue
		}
		b.WriteString(t)
		b.WriteString(" ")
	}
	return strings.TrimSpace(b.String())
}

// ---------------------------------------------------------------------------
// The expectations
// ---------------------------------------------------------------------------

// shared is the set every campaign member sees, with no per-player
// exceptions. It is what the shared display shows (ADR-0048).
var shared = []string{
	"Bel Orrery",
	"Hessa Cray",
	"Ilva Sarn",
	"Orson Pike",
	"Pay the Tide",
	"Saltmere",
	"Saltmere Faces",
	"Session 01 - The Cormorant Wreck",
	"The Brinebacks",
	"The Drowned Quarter",
	"The Night of the Cormorant",
	"The Old Salt Road",
	"The Salt Debt",
	"The Tideglass",
}

// expected is the exact set of note names each principal may see, under the
// proposed schema.
var expected = map[string][]string{
	// The GM sees everything except a player's private journal, which is
	// private even from the GM (ADR-0048).
	"gm": {
		"Bel Orrery",
		"Cormorant Stair",
		"Corvin Sallow",
		"GM Only - The Reeve Collects",
		"Hessa Cray",
		"Ilva Sarn",
		"Orson Pike",
		"Pay the Tide",
		"Saltmere",
		"Saltmere Faces",
		"Session 01 - The Cormorant Wreck",
		"The Bond of Salt",
		"The Brinebacks",
		"The Causeway Notice",
		"The Drowned Quarter",
		"The Night of the Cormorant",
		"The Old Salt Road",
		"The Salt Debt",
		"The Tideglass",
		"Wren Lock",
	},
	// Ilva owns the journal and the bond, and has discovered the Drowned
	// Quarter. She has not discovered the Cormorant Stair.
	"p_ilva": append(append([]string{}, shared...),
		"Ilva - Salt and Iron",
		"The Bond of Salt",
		"The Causeway Notice",
	),
	// Orson is granted the bond and has discovered the Cormorant Stair.
	// He cannot see Ilva's journal, and he cannot see Wren's draft.
	"p_orson": append(append([]string{}, shared...),
		"Cormorant Stair",
		"The Bond of Salt",
		"The Causeway Notice",
	),
	// Wren is a member with a character in draft. The draft is invisible
	// to the party: the workflow gate is a grant-suppressing deny.
	"p_wren": append(append([]string{}, shared...),
		"The Causeway Notice",
		"Wren Lock",
	),
	// The display is a principal, and it sees party-visible content only.
	// It is not the union of what the members can see.
	"display": shared,
	// An anonymous caller sees `public` notes. There is one: the notice
	// nailed to the tide wall.
	"anonymous": {"The Causeway Notice"},
}

// ---------------------------------------------------------------------------
// The tests
// ---------------------------------------------------------------------------

// TestEveryNoteParsesCleanly is the extractor's contract applied to the
// demo campaign: every note parses, and every note reports no issues.
func TestEveryNoteParsesCleanly(t *testing.T) {
	err := filepath.WalkDir(vaultRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		doc, err := frontmatter.Parse(data)
		if err != nil {
			t.Errorf("%s: parse: %v", path, err)
			return nil
		}
		if !doc.HasBlock {
			t.Errorf("%s: no frontmatter block", path)
			return nil
		}
		for _, iss := range doc.Block.Issues {
			t.Errorf("%s: %s", path, iss.String())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", vaultRoot, err)
	}
}

// TestTheCampaignIsWhole checks the inventory: the demo campaign has to
// contain the things it promises to contain.
func TestTheCampaignIsWhole(t *testing.T) {
	ix := loadVault(t)
	if len(ix.notes) != 21 {
		t.Errorf("expected 21 notes, got %d", len(ix.notes))
	}
	want := map[string]int{
		"campaign":  1,
		"character": 6,
		"encounter": 1,
		"event":     1,
		"faction":   1,
		"handout":   2,
		"home":      1,
		"item":      1,
		"journal":   1,
		"location":  2,
		"quest":     2,
		"session":   1,
		"table":     1,
	}
	got := map[string]int{}
	for _, n := range ix.notes {
		got[n.typ]++
	}
	for typ, count := range want {
		if got[typ] != count {
			t.Errorf("expected %d note(s) of type %q, got %d", count, typ, got[typ])
		}
	}
}

// TestNoDanglingLinks checks that every wikilink in the campaign resolves.
func TestNoDanglingLinks(t *testing.T) {
	ix := loadVault(t)
	link := regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	for _, n := range ix.notes {
		for _, m := range link.FindAllStringSubmatch(string(n.body), -1) {
			if ix.resolve(m[1]) == nil {
				t.Errorf("%s: dangling link [[%s]]", n.rel, m[1])
			}
		}
		if n.campaignRef != "" && ix.resolve(n.campaignRef) == nil {
			t.Errorf("%s: dangling campaign reference %q", n.rel, n.campaignRef)
		}
	}
}

// TestProjectionBothDirections is the acceptance criterion: no GM-only
// note is visible to a player, and every party-visible note is visible to
// a player.
func TestProjectionBothDirections(t *testing.T) {
	ix := loadVault(t)
	for _, p := range everyPrincipal {
		want := slices.Clone(expected[p.key()])
		slices.Sort(want)
		got := names(ix.view(p, proposed))
		if !slices.Equal(got, want) {
			t.Errorf("%s:\n got %v\nwant %v", p.key(), got, want)
		}
	}
}

// TestNoGMOnlyNoteReachesAPlayer is the first half of the criterion,
// stated separately so that the failure message says what it means.
func TestNoGMOnlyNoteReachesAPlayer(t *testing.T) {
	ix := loadVault(t)
	for _, n := range ix.notes {
		if n.vis != "gm" {
			continue
		}
		for _, p := range everyNonGM {
			if ix.visible(n, p, proposed) {
				t.Errorf("%s: GM-only note %q is visible to %s", n.rel, n.name, p.kind)
			}
		}
	}
}

// TestTheDisplayIsNotTheUnion is ADR-0048's rule: the shared display sees
// party-visible content only, never the union of what the members see.
func TestTheDisplayIsNotTheUnion(t *testing.T) {
	ix := loadVault(t)
	var union []string
	for _, p := range everyPlayer {
		for _, n := range ix.view(p, proposed) {
			union = append(union, n.name)
		}
	}
	slices.Sort(union)
	union = slices.Compact(union)
	display := names(ix.view(displayPrincipal, proposed))
	slices.Sort(display)
	if slices.Equal(union, display) {
		t.Fatal("the display sees the union of the members' views; it must not")
	}
	for _, leaked := range []string{"The Bond of Salt", "Ilva - Salt and Iron", "Cormorant Stair"} {
		if slices.Contains(display, leaked) {
			t.Errorf("the display sees %q, which is not party-visible", leaked)
		}
	}
}

// TestSecretTextNeverReachesAPlayer is ADR-0035: the secret text is
// stripped server-side, so a player who inspects the response sees
// nothing.
func TestSecretTextNeverReachesAPlayer(t *testing.T) {
	ix := loadVault(t)
	secrets := map[string]string{
		// the note, and a phrase that only appears inside its secret
		"Hessa Cray":          "She knew the salt was meant to go into the water",
		"Cormorant Stair":     "where the Brinebacks keep the ledger",
		"Orson Pike":          "came back with a line that does not match",
		"Saltmere Faces":      "Corvin Sallow is not a face",
		"Ilva Sarn":           "a courier who is paid eleven marks",
		"The Drowned Quarter": "The bloom on the Causeway water ran a mile",
	}
	for _, p := range everyNonGM {
		for _, n := range ix.view(p, proposed) {
			phrase, ok := secrets[n.name]
			if !ok {
				continue
			}
			if strings.Contains(plain(ix.projectBody(n, p, proposed)), plain(phrase)) {
				t.Errorf("%s: the secret in %q reached %s", n.rel, n.name, p.kind)
			}
		}
	}
	// The GM sees all of it.
	for name, phrase := range secrets {
		n := ix.byKey[strings.ToLower(name)]
		if n == nil {
			t.Fatalf("no note named %q", name)
		}
		if !strings.Contains(plain(ix.projectBody(n, gmPrincipal, proposed)), plain(phrase)) {
			t.Errorf("the GM cannot see the secret in %q", name)
		}
	}
}

// TestARevealedSecretIsVisible checks the other direction: a secret whose
// id is in the note's `revealed` list has been revealed, and the party
// sees it.
func TestARevealedSecretIsVisible(t *testing.T) {
	ix := loadVault(t)
	n := ix.byKey["the drowned quarter"]
	if n == nil {
		t.Fatal("no note named The Drowned Quarter")
	}
	if !slices.Contains(n.revealed, "saltgate") {
		t.Fatalf("revealed = %v, want saltgate", n.revealed)
	}
	body := ix.projectBody(n, ilva, proposed)
	if !strings.Contains(body, "customs door") {
		t.Error("a revealed secret is not visible to the party")
	}
	if strings.Contains(body, "The bloom on the Causeway water ran a mile") {
		t.Error("an unrevealed secret in the same note is visible to the party")
	}
}

// TestTheGMOnlyCharacterIsUnnamed is the strongest form of "no GM-only
// note is visible to a player": the hidden character is not named
// anywhere in any player's view.
func TestTheGMOnlyCharacterIsUnnamed(t *testing.T) {
	ix := loadVault(t)
	for _, p := range everyNonGM {
		for _, n := range ix.view(p, proposed) {
			for k, v := range ix.projectFields(n, p, proposed) {
				if strings.Contains(v, "Corvin Sallow") {
					t.Errorf("%s: the field %q of %q names a GM-only character (%s)", n.rel, k, n.name, p.kind)
				}
			}
			if strings.Contains(ix.projectBody(n, p, proposed), "Corvin Sallow") {
				t.Errorf("%s: the body of %q names a GM-only character (%s)", n.rel, n.name, p.kind)
			}
		}
	}
}

// TestTheDiscoveryGrantIsUnreachableToday is finding 3: under the schema
// as it stands, there is no way to say "the party has not discovered this
// yet", so the discovery grant listed in session-writeback.md §6 cannot
// fire.
func TestTheDiscoveryGrantIsUnreachableToday(t *testing.T) {
	ix := loadVault(t)
	stair := ix.byKey["cormorant stair"]
	if stair == nil {
		t.Fatal("no note named Cormorant Stair")
	}
	// Under the schema as it stands, `discovered` is not in the
	// visibility vocabulary.
	if slices.Contains([]string{"public", "party", "owner", "private", "gm"}, stair.vis) {
		t.Fatalf("the Cormorant Stair's visibility %q is already a known value; the finding is stale", stair.vis)
	}
	// An unknown visibility value must fail closed, so the location is
	// invisible to everybody -- including the player who has discovered it.
	if ix.visible(stair, orson, current) {
		t.Error("the Cormorant Stair is visible to Orson under the current schema; " +
			"it should not be, because `visibility: discovered` does not exist yet")
	}
	if !ix.visible(stair, orson, proposed) {
		t.Error("the Cormorant Stair is not visible to Orson under the proposed schema; " +
			"the discovery grant should make it visible")
	}
	for _, p := range []principal{ilva, wren, displayPrincipal} {
		if ix.visible(stair, p, proposed) {
			t.Errorf("the Cormorant Stair is visible to %s, who has not discovered it", p.kind)
		}
	}
}

// TestTheDiscoveredListLeaksNames is finding 4: `discovered` is a
// reserved field with no visibility of its own, so it inherits the note's,
// and a party-visible character note publishes the names of notes the
// party has not discovered.
func TestTheDiscoveredListLeaksNames(t *testing.T) {
	ix := loadVault(t)
	orsonNote := ix.byKey["orson pike"]
	if orsonNote == nil {
		t.Fatal("no note named Orson Pike")
	}
	currentFields := ix.projectFields(orsonNote, ilva, current)
	if !strings.Contains(currentFields["discovered"], "Cormorant Stair") {
		t.Error("under the current schema the discovered list does not leak; " +
			"the finding is that it does")
	}
	proposedFields := ix.projectFields(orsonNote, ilva, proposed)
	if _, ok := proposedFields["discovered"]; ok {
		t.Error("under the proposed schema the discovered list is still visible to another player")
	}
	// The owner still sees it.
	ownFields := ix.projectFields(orsonNote, orson, proposed)
	if !strings.Contains(ownFields["discovered"], "Cormorant Stair") {
		t.Error("the owner cannot see the character's own discovered list")
	}
}

// TestRetiredIsGatedOrNot is finding 5: ADR-0009's table says a retired
// note is visible to its author and the GM; ADR-0033's rule 3 gates only
// draft and submitted. The two ADRs disagree about a note that is in the
// demo campaign.
func TestRetiredIsGatedOrNot(t *testing.T) {
	ix := loadVault(t)
	road := ix.byKey["the old salt road"]
	if road == nil {
		t.Fatal("no note named The Old Salt Road")
	}
	if !ix.visible(road, ilva, proposed) {
		t.Error("the retired quest is not visible to the party under ADR-0033's rule 3")
	}
	gated := schema{name: "adr-0009", retiredIsGated: true}
	if ix.visible(road, ilva, gated) {
		t.Error("the retired quest is visible to the party under ADR-0009's table")
	}
}

// TestObsidianCommentsLeak is finding 6: an Obsidian comment is hidden in
// the preview and serialised into the projection, so it is not a secret
// mechanism.
func TestObsidianCommentsLeak(t *testing.T) {
	ix := loadVault(t)
	quest := ix.byKey["pay the tide"]
	if quest == nil {
		t.Fatal("no note named Pay the Tide")
	}
	quest.body = append(quest.body, []byte("\n%% the reeve is bought %%\n")...)
	if !strings.Contains(ix.projectBody(quest, ilva, current), "the reeve is bought") {
		t.Error("an Obsidian comment did not leak into a player's projection; " +
			"the finding is that it does")
	}
	if strings.Contains(ix.projectBody(quest, ilva, proposed), "the reeve is bought") {
		t.Error("an Obsidian comment leaked into a player's projection under the proposed schema")
	}
}

// TestTheCampaignNoteDefinesTheParty is ADR-0037: a campaign note's
// `party` resolves against its own `members` list.
func TestTheCampaignNoteDefinesTheParty(t *testing.T) {
	ix := loadVault(t)
	campaign := ix.byKey["the salt debt"]
	if campaign == nil {
		t.Fatal("no note named The Salt Debt")
	}
	if campaign.typ != "campaign" {
		t.Errorf("the campaign note's type is %q", campaign.typ)
	}
	want := []string{"p_ilva", "p_orson", "p_wren"}
	if !slices.Equal(campaign.members, want) {
		t.Errorf("members = %v, want %v", campaign.members, want)
	}
	for _, p := range everyPlayer {
		if !slices.Contains(campaign.members, p.id) {
			t.Errorf("%s is not a member of the campaign", p.id)
		}
	}
	// Every note in the campaign names the campaign, so `party` means the
	// same thing everywhere -- except the one public document, which
	// belongs to no campaign and says so by being public (ADR-0033's
	// rule 6).
	for _, n := range ix.notes {
		if n.campaignRef == "" {
			if n.vis != "public" {
				t.Errorf("%s: a note with no campaign must be public", n.rel)
			}
			continue
		}
		if ix.resolve(n.campaignRef) == nil {
			t.Errorf("%s: dangling campaign reference %q", n.rel, n.campaignRef)
		}
	}
}

// TestEveryNoteCarriesItsIdentity checks the fields the app relies on.
func TestEveryNoteCarriesItsIdentity(t *testing.T) {
	ix := loadVault(t)
	for _, n := range ix.notes {
		if n.uid == "" {
			t.Errorf("%s: no uid", n.rel)
		}
		if n.typ == "" {
			t.Errorf("%s: no type", n.rel)
		}
		if n.name == "" {
			t.Errorf("%s: no name", n.rel)
		}
		if n.vis == "" {
			t.Errorf("%s: no visibility", n.rel)
		}
		if n.wf == "" {
			t.Errorf("%s: no workflow", n.rel)
		}
		switch n.vis {
		case "public", "party", "owner", "private", "gm", "discovered":
		default:
			t.Errorf("%s: unknown visibility %q", n.rel, n.vis)
		}
		switch n.wf {
		case "draft", "submitted", "active", "retired":
		default:
			t.Errorf("%s: unknown workflow %q", n.rel, n.wf)
		}
		switch n.typ {
		case "character", "location", "faction", "item", "quest", "session",
			"encounter", "event", "table", "content", "handout", "message",
			"journal", "home", "campaign":
		default:
			t.Errorf("%s: unknown type %q", n.rel, n.typ)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// names returns the names of a set of notes, sorted.
func names(notes []*note) []string {
	out := make([]string, 0, len(notes))
	for _, n := range notes {
		out = append(out, n.name)
	}
	slices.Sort(out)
	return out
}
