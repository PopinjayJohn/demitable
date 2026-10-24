package vault

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/pack"
)

func loadDemoVault(t *testing.T) *Vault {
	t.Helper()
	v, err := Open(filepath.Join("..", "..", "dev", "demo-campaign", "vault"))
	if err != nil {
		t.Fatalf("open demo vault: %v", err)
	}
	return v
}

func demoPrincipal(v *Vault, kind, id string) Principal {
	switch kind {
	case "gm":
		return GM()
	case "display":
		return Display("[[The Salt Debt]]")
	case "anonymous":
		return Anonymous()
	default:
		p := Account(id, "[[The Salt Debt]]")
		var discovered []string
		for _, n := range v.AllNotes() {
			if n.Player == id {
				discovered = append(discovered, n.Discovered...)
			}
		}
		p.Discovered = discovered
		return p
	}
}

var demoExpected = map[string][]string{
	"gm": {
		"Bel Orrery", "Cormorant Stair", "Corvin Sallow", "GM Only - The Reeve Collects",
		"Hessa Cray", "Ilva Sarn", "Orson Pike", "Pay the Tide", "Saltmere",
		"Saltmere Faces", "Session 01 - The Cormorant Wreck", "The Bond of Salt",
		"The Brinebacks", "The Causeway Notice", "The Drowned Quarter",
		"The Night of the Cormorant", "The Old Salt Road", "The Salt Debt",
		"The Tideglass", "Wren Lock",
	},
	"p_ilva": {
		"Bel Orrery", "Hessa Cray", "Ilva Sarn", "Ilva - Salt and Iron",
		"Orson Pike", "Pay the Tide", "Saltmere", "Saltmere Faces",
		"Session 01 - The Cormorant Wreck", "The Bond of Salt", "The Brinebacks",
		"The Causeway Notice", "The Drowned Quarter", "The Night of the Cormorant",
		"The Old Salt Road", "The Salt Debt", "The Tideglass",
	},
	"p_orson": {
		"Bel Orrery", "Cormorant Stair", "Hessa Cray", "Ilva Sarn", "Orson Pike",
		"Pay the Tide", "Saltmere", "Saltmere Faces", "Session 01 - The Cormorant Wreck",
		"The Bond of Salt", "The Brinebacks", "The Causeway Notice", "The Drowned Quarter",
		"The Night of the Cormorant", "The Old Salt Road", "The Salt Debt", "The Tideglass",
	},
	"p_wren": {
		"Bel Orrery", "Hessa Cray", "Ilva Sarn", "Orson Pike", "Pay the Tide",
		"Saltmere", "Saltmere Faces", "Session 01 - The Cormorant Wreck", "The Brinebacks",
		"The Causeway Notice", "The Drowned Quarter", "The Night of the Cormorant",
		"The Old Salt Road", "The Salt Debt", "The Tideglass", "Wren Lock",
	},
	"display": {
		"Bel Orrery", "Hessa Cray", "Ilva Sarn", "Orson Pike", "Pay the Tide",
		"Saltmere", "Saltmere Faces", "Session 01 - The Cormorant Wreck",
		"The Brinebacks", "The Drowned Quarter", "The Night of the Cormorant",
		"The Old Salt Road", "The Salt Debt", "The Tideglass",
	},
	"anonymous": {"The Causeway Notice"},
}

func TestM5DemoProjectionBothDirections(t *testing.T) {
	v := loadDemoVault(t)
	notes := v.AllNotes()
	if len(notes) != 21 {
		t.Fatalf("expected 21 notes, got %d", len(notes))
	}
	principals := []struct {
		key  string
		kind string
		id   string
	}{
		{"gm", "gm", ""},
		{"p_ilva", "account", "p_ilva"},
		{"p_orson", "account", "p_orson"},
		{"p_wren", "account", "p_wren"},
		{"display", "display", ""},
		{"anonymous", "anonymous", ""},
	}
	if len(principals) != 6 {
		t.Fatal("expected 6 principals")
	}
	for _, pc := range principals {
		p := demoPrincipal(v, pc.kind, pc.id)
		var got []string
		for _, n := range notes {
			if Visible(p, *n) {
				got = append(got, n.Name)
			}
		}
		slices.Sort(got)
		want := slices.Clone(demoExpected[pc.key])
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s:\n got %v\nwant %v", pc.key, got, want)
		}
	}
}

func TestM5DemoGMSeesAllNonPrivate(t *testing.T) {
	v := loadDemoVault(t)
	gm := GM()
	count := 0
	for _, n := range v.AllNotes() {
		if n.Visibility == "private" {
			if Visible(gm, *n) {
				t.Errorf("GM sees private note %s", n.Name)
			}
			continue
		}
		if !Visible(gm, *n) {
			t.Errorf("GM cannot see %s", n.Name)
		}
		count++
	}
	if count == 0 {
		t.Fatal("vacuous: GM saw nothing")
	}
}

func TestM5DemoNoGMOnlyCharacterNamed(t *testing.T) {
	v := loadDemoVault(t)
	for _, id := range []string{"p_ilva", "p_orson", "p_wren"} {
		p := demoPrincipal(v, "account", id)
		for _, n := range v.AllNotes() {
			if !Visible(p, *n) {
				continue
			}
			vw := Project(p, *n)
			if containsStr2(vw.Body, "Corvin Sallow") {
				t.Errorf("%s: body names GM-only character", n.Name)
			}
		}
	}
}

func containsStr2(s, sub string) bool { return strings.Contains(s, sub) }

func TestM5DeclaredPropertiesNonVacuous(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "pack", "packs", "example.yaml"))
	if err != nil {
		// try relative to module root
		data, err = os.ReadFile(filepath.Join("internal", "pack", "packs", "example.yaml"))
	}
	if err != nil {
		t.Skip("example pack not found")
	}
	p, err := pack.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	ownerCount, gmCount := 0, 0
	for _, ct := range p.Content {
		for _, f := range ct.Fields {
			switch f.Visibility {
			case "owner":
				ownerCount++
			case "gm":
				gmCount++
			}
		}
	}
	if ownerCount == 0 || gmCount == 0 {
		t.Fatalf("vacuous: owner=%d gm=%d", ownerCount, gmCount)
	}
	t.Logf("declared properties: owner=%d gm=%d", ownerCount, gmCount)
}

func TestM5SyntheticVaultLoads(t *testing.T) {
	v, err := Open(filepath.Join("..", "..", "spikes", "projection", "testdata", "synthetic"))
	if err != nil {
		t.Skipf("synthetic vault: %v", err)
	}
	if len(v.AllNotes()) != 36 {
		t.Fatalf("expected 36 synthetic notes, got %d", len(v.AllNotes()))
	}
}
