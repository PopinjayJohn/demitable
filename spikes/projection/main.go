// Command projection runs the visibility model of ADR-0033 (as amended by
// ADR-0037, ADR-0038, ADR-0040, ADR-0048 and ADR-0050) over a vault, and writes the
// per-principal visibility diff.
//
// It is a disposable spike. Its answer is the report it writes, not the code.
//
//	go run ./spikes/projection --vault fixtures --out /tmp/opencode/report
//	go run ./spikes/projection --vault spikes/projection/testdata/synthetic \
//	    --pack spikes/projection/testdata/synthetic-pack.yaml --out /tmp/opencode/synth
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	var (
		vault   = flag.String("vault", "", "the vault directory to project")
		pack    = flag.String("pack", "", "a pack file, for property-level visibility")
		out     = flag.String("out", "", "a directory to write the report into")
		scope   = flag.String("scope", string(ScopeStrict), "the scope policy: strict | grant-overrides")
		verbose = flag.Bool("verbose", false, "print the per-note table to stdout")
	)
	flag.Parse()

	if *vault == "" {
		fmt.Fprintln(os.Stderr, "projection: --vault is required")
		flag.Usage()
		os.Exit(2)
	}

	notes, err := LoadVault(*vault)
	if err != nil {
		fmt.Fprintf(os.Stderr, "projection: load: %v\n", err)
		os.Exit(1)
	}

	var pk *Pack
	packPath := "(none)"
	if *pack != "" {
		pk, err = LoadPack(*pack)
		if err != nil {
			fmt.Fprintf(os.Stderr, "projection: pack: %v\n", err)
			os.Exit(1)
		}
		packPath = *pack
	}

	policy := ScopeStrict
	switch *scope {
	case string(ScopeStrict):
		policy = ScopeStrict
	case string(ScopeGrantOverrides):
		policy = ScopeGrantOverrides
	default:
		fmt.Fprintf(os.Stderr, "projection: unknown scope %q\n", *scope)
		os.Exit(2)
	}

	proj := NewProjector(notes, pk, policy)

	principals := Principals(notes, proj)

	rep := &Report{Vault: *vault, Notes: notes, Scope: policy, PackPath: packPath}
	for _, pr := range principals {
		rep.Projects = append(rep.Projects, proj.Project(pr))
	}

	fmt.Printf("projection: %s — %d notes, %d principals, scope=%s\n",
		*vault, len(notes), len(principals), policy)
	fmt.Printf("%-22s %8s %8s %8s %10s %8s\n", "principal", "visible", "hidden", "fields", "bodybytes", "stripped")
	for _, p := range rep.Projects {
		fmt.Printf("%-22s %8d %8d %8d %10d %8d\n",
			p.Principal, p.Visible, len(p.Notes)-p.Visible, p.VisibleFields, p.VisibleBytes, p.Stripped)
	}

	if *out != "" {
		if err := os.MkdirAll(*out, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "projection: out: %v\n", err)
			os.Exit(1)
		}
		write := func(name, content string) {
			path := filepath.Join(*out, name)
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "projection: write %s: %v\n", path, err)
				os.Exit(1)
			}
			fmt.Printf("wrote %s\n", path)
		}
		write("report.md", rep.Markdown())
		write("fields.md", rep.FieldDetail())
		if err := rep.WriteCSV(filepath.Join(*out, "decisions.csv")); err != nil {
			fmt.Fprintf(os.Stderr, "projection: csv: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s\n", filepath.Join(*out, "decisions.csv"))
	}

	if *verbose {
		for _, p := range rep.Projects {
			fmt.Printf("\n== %s ==\n", p.Principal)
			for _, pn := range p.Notes {
				fmt.Printf("  %-60s %s\n", pn.Note.Path, pn.Decision)
			}
		}
	}
}

// Principals derives the set of callers to project for, from the vault
// itself: the GM, every account the vault names, a display principal for
// every campaign, a guest seat bound to the first character it finds, and
// the anonymous caller.
func Principals(notes []*Note, proj *Projector) []Principal {
	accounts := map[string]bool{}
	haveActiveSeat := false
	haveDraftSeat := false
	for _, n := range notes {
		if n.Player != "" {
			accounts[n.Player] = true
		}
		for _, m := range n.Members {
			accounts[m] = true
		}
		for _, g := range n.Grants {
			accounts[g] = true
		}
	}
	ids := make([]string, 0, len(accounts))
	for id := range accounts {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := []Principal{
		{Kind: PrincipalGM, Label: "gm"},
	}
	for _, id := range ids {
		out = append(out, Principal{Kind: PrincipalAccount, ID: id, Label: "account:" + id})
	}
	campaigns := map[string]bool{}
	for _, n := range notes {
		if n.Campaign != "" {
			campaigns[n.Campaign] = true
		}
	}
	cids := make([]string, 0, len(campaigns))
	for id := range campaigns {
		cids = append(cids, id)
	}
	sort.Strings(cids)
	for _, id := range cids {
		out = append(out, Principal{Kind: PrincipalDisplay, Campaign: id, Label: "display:" + id})
	}
	// Two guest seats: one bound to an active character, one bound to a
	// character still in draft. The second exercises the collision between
	// the seat's implicit grant and the workflow gate.
	for _, n := range notes {
		if !strings.EqualFold(n.Type, "character") || n.Player == "" {
			continue
		}
		switch n.Workflow {
		case "active":
			if !haveActiveSeat {
				out = append(out, Principal{Kind: PrincipalSeat, ID: "s_guest", BoundUID: n.ID(), Label: "seat:s_guest"})
				haveActiveSeat = true
			}
		case "draft":
			if !haveDraftSeat {
				out = append(out, Principal{Kind: PrincipalSeat, ID: "s_apprentice", BoundUID: n.ID(), Label: "seat:s_apprentice"})
				haveDraftSeat = true
			}
		}
	}
	out = append(out, Principal{Kind: PrincipalAnonymous, Label: "anonymous"})
	return out
}
