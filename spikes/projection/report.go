package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Report is the whole projection, for every principal.
type Report struct {
	Vault    string
	Notes    []*Note
	Projects []*Project
	Scope    ScopePolicy
	PackPath string
}

// keyShape is the histogram of top-level frontmatter keys.
type keyShape struct {
	Key   string
	Notes int
	Types map[string]int
}

// keyShapes tabulates the top-level keys across the vault.
func keyShapes(notes []*Note) []keyShape {
	byKey := map[string]*keyShape{}
	for _, n := range notes {
		if n.Block == nil {
			continue
		}
		for _, f := range n.Block.Fields {
			ks := byKey[f.Key]
			if ks == nil {
				ks = &keyShape{Key: f.Key, Types: map[string]int{}}
				byKey[f.Key] = ks
			}
			ks.Notes++
			tag := "!!none"
			if f.ValueNode != nil {
				tag = f.ValueNode.Tag
			}
			ks.Types[tag]++
		}
	}
	out := make([]keyShape, 0, len(byKey))
	for _, ks := range byKey {
		out = append(out, *ks)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Notes != out[j].Notes {
			return out[i].Notes > out[j].Notes
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// reservedFields are ADR-0040's twelve names.
var reservedFields = []string{
	"type", "uid", "name", "system", "campaign", "kind", "player",
	"workflow", "visibility", "grants", "revealed", "discovered",
}

// fieldCounts counts the notes carrying each reserved field.
func fieldCounts(notes []*Note) map[string]int {
	out := map[string]int{}
	for _, n := range notes {
		if n.Block == nil {
			continue
		}
		for _, f := range n.Block.Fields {
			out[strings.ToLower(f.Key)]++
		}
	}
	return out
}

// Markdown renders the report.
func (r *Report) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Projection report — `%s`\n\n", r.Vault)
	fmt.Fprintf(&b, "Scope policy: **%s**. Pack: `%s`.\n\n", r.Scope, r.PackPath)

	// ---- the vault -------------------------------------------------------
	withBlock := 0
	issues := 0
	for _, n := range r.Notes {
		if n.HasBlock {
			withBlock++
		}
		issues += len(n.Issues)
	}
	fmt.Fprintf(&b, "## The vault\n\n")
	fmt.Fprintf(&b, "| | |\n|---|---|\n")
	fmt.Fprintf(&b, "| markdown files | %d |\n", len(r.Notes))
	fmt.Fprintf(&b, "| with a frontmatter block | %d |\n", withBlock)
	fmt.Fprintf(&b, "| without one | %d |\n", len(r.Notes)-withBlock)
	fmt.Fprintf(&b, "| extractor issues | %d |\n", issues)
	fmt.Fprintf(&b, "| notes with a `uid` | %d |\n", countWith(r.Notes, func(n *Note) bool { return n.UID != "" }))
	fmt.Fprintf(&b, "| notes with a `campaign` | %d |\n", countWith(r.Notes, func(n *Note) bool { return n.Campaign != "" }))
	fmt.Fprintf(&b, "| notes with a `visibility` | %d |\n", countWith(r.Notes, func(n *Note) bool { return n.Visibility != "" }))
	fmt.Fprintf(&b, "| notes with a `workflow` | %d |\n", countWith(r.Notes, func(n *Note) bool { return n.Workflow != "" }))
	fmt.Fprintf(&b, "| notes with `grants` | %d |\n", countWith(r.Notes, func(n *Note) bool { return len(n.Grants) > 0 }))
	fmt.Fprintf(&b, "| notes with `discovered` | %d |\n", countWith(r.Notes, func(n *Note) bool { return len(n.Discovered) > 0 }))
	fmt.Fprintf(&b, "| notes with `revealed` | %d |\n", countWith(r.Notes, func(n *Note) bool { return len(n.Revealed) > 0 }))
	fmt.Fprintf(&b, "| notes with `player` | %d |\n", countWith(r.Notes, func(n *Note) bool { return n.Player != "" }))
	fmt.Fprintf(&b, "| notes with `type` | %d |\n", countWith(r.Notes, func(n *Note) bool { return n.Type != "" }))
	fmt.Fprintf(&b, "\n")

	// ---- reserved fields -------------------------------------------------
	fc := fieldCounts(r.Notes)
	fmt.Fprintf(&b, "### ADR-0040's reserved fields, as they appear in this vault\n\n")
	fmt.Fprintf(&b, "| Field | Notes carrying it |\n|---|---|\n")
	for _, f := range reservedFields {
		fmt.Fprintf(&b, "| `%s` | %d |\n", f, fc[f])
	}
	fmt.Fprintf(&b, "\n")

	// ---- every top-level key ----------------------------------------------
	fmt.Fprintf(&b, "### Every top-level key in the vault\n\n")
	fmt.Fprintf(&b, "| Key | Notes | YAML types seen |\n|---|---|---|\n")
	for _, ks := range keyShapes(r.Notes) {
		types := []string{}
		for t, c := range ks.Types {
			types = append(types, fmt.Sprintf("%s×%d", t, c))
		}
		sort.Strings(types)
		fmt.Fprintf(&b, "| `%s` | %d | %s |\n", ks.Key, ks.Notes, strings.Join(types, ", "))
	}
	fmt.Fprintf(&b, "\n")

	// ---- the principals ---------------------------------------------------
	fmt.Fprintf(&b, "## What each principal sees\n\n")
	fmt.Fprintf(&b, "| Principal | Notes visible | Notes hidden | Fields visible | Body bytes | Secret blocks stripped |\n|---|---|---|---|---|---|\n")
	for _, p := range r.Projects {
		fmt.Fprintf(&b, "| `%s` | %d | %d | %d | %d | %d |\n",
			p.Principal, p.Visible, len(p.Notes)-p.Visible, p.VisibleFields, p.VisibleBytes, p.Stripped)
	}
	fmt.Fprintf(&b, "\n")

	// ---- the diffs ---------------------------------------------------------
	fmt.Fprintf(&b, "## The diffs, principal against principal\n\n")
	for i := 0; i < len(r.Projects); i++ {
		for j := i + 1; j < len(r.Projects); j++ {
			a, bb := r.Projects[i], r.Projects[j]
			onlyA, onlyB, both := diff(a, bb)
			fmt.Fprintf(&b, "### `%s` vs `%s`\n\n", a.Principal, bb.Principal)
			fmt.Fprintf(&b, "- both see: **%d**\n", len(both))
			fmt.Fprintf(&b, "- only `%s` sees: **%d**\n", a.Principal, len(onlyA))
			fmt.Fprintf(&b, "- only `%s` sees: **%d**\n", bb.Principal, len(onlyB))
			if len(onlyA) > 0 {
				fmt.Fprintf(&b, "\nNotes only `%s` sees:\n\n", a.Principal)
				for _, n := range onlyA {
					fmt.Fprintf(&b, "- `%s` — %s\n", n.Note.Path, n.Decision)
				}
			}
			if len(onlyB) > 0 {
				fmt.Fprintf(&b, "\nNotes only `%s` sees:\n\n", bb.Principal)
				for _, n := range onlyB {
					fmt.Fprintf(&b, "- `%s` — %s\n", n.Note.Path, n.Decision)
				}
			}
			fmt.Fprintf(&b, "\n")
		}
	}

	// ---- the per-note table ------------------------------------------------
	fmt.Fprintf(&b, "## Every note, and every principal's verdict\n\n")
	headers := []string{"note", "type", "campaign", "visibility", "workflow", "player"}
	for _, p := range r.Projects {
		headers = append(headers, p.Principal.Label)
	}
	w := csv.NewWriter(&r2b{&b})
	_ = w.Write(headers)
	for _, n := range r.Notes {
		row := []string{
			n.Path, n.Type, n.Campaign, n.Visibility, n.Workflow, n.Player,
		}
		for _, p := range r.Projects {
			pn := p.byPath[n.Path]
			verdict := "hidden"
			if pn.Decision.Visible {
				verdict = "visible"
			}
			row = append(row, verdict)
		}
		_ = w.Write(row)
	}
	w.Flush()
	fmt.Fprintf(&b, "\n")

	// ---- the per-note reasons ----------------------------------------------
	fmt.Fprintf(&b, "## Every note's decision, with the rule that decided it\n\n")
	for _, p := range r.Projects {
		fmt.Fprintf(&b, "### `%s`\n\n", p.Principal)
		for _, pn := range p.Notes {
			fmt.Fprintf(&b, "- `%s` — %s\n", pn.Note.Path, pn.Decision)
		}
		fmt.Fprintf(&b, "\n")
	}

	return b.String()
}

// r2b adapts a strings.Builder to an io.Writer, so the CSV writer can
// target the report.
type r2b struct{ b *strings.Builder }

func (r *r2b) Write(p []byte) (int, error) { return r.b.Write(p) }

// diff compares two projections.
func diff(a, b *Project) (onlyA, onlyB, both []ProjectedNote) {
	visibleB := map[string]bool{}
	for _, n := range b.Notes {
		visibleB[n.Note.Path] = n.Decision.Visible
	}
	visibleA := map[string]bool{}
	for _, n := range a.Notes {
		visibleA[n.Note.Path] = n.Decision.Visible
	}
	for _, n := range a.Notes {
		switch {
		case n.Decision.Visible && !visibleB[n.Note.Path]:
			onlyA = append(onlyA, n)
		case n.Decision.Visible && visibleB[n.Note.Path]:
			both = append(both, n)
		}
	}
	for _, n := range b.Notes {
		if n.Decision.Visible && !visibleA[n.Note.Path] {
			onlyB = append(onlyB, n)
		}
	}
	return onlyA, onlyB, both
}

func countWith(notes []*Note, pred func(*Note) bool) int {
	n := 0
	for _, note := range notes {
		if pred(note) {
			n++
		}
	}
	return n
}

// WriteCSV writes the per-note decision table as CSV.
func (r *Report) WriteCSV(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	head := []string{"note", "uid", "type", "campaign", "visibility", "workflow", "player", "grants"}
	for _, p := range r.Projects {
		head = append(head, p.Principal.Label+"_visible", p.Principal.Label+"_rule")
	}
	if err := w.Write(head); err != nil {
		return err
	}
	for _, n := range r.Notes {
		row := []string{
			n.Path, n.UID, n.Type, n.Campaign, n.Visibility, n.Workflow, n.Player,
			strings.Join(n.Grants, " "),
		}
		for _, p := range r.Projects {
			pn := p.byPath[n.Path]
			vis := "false"
			if pn.Decision.Visible {
				vis = "true"
			}
			row = append(row, vis, pn.Decision.Rule+": "+pn.Decision.Detail)
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// FieldDetail renders the field-level projection for every principal, for
// the notes that at least one principal can see.
func (r *Report) FieldDetail() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Field-level projection\n\n")
	for _, p := range r.Projects {
		fmt.Fprintf(&b, "## `%s`\n\n", p.Principal)
		for _, pn := range p.Notes {
			if !pn.Decision.Visible {
				continue
			}
			if len(pn.Fields) == 0 && len(pn.HiddenFields) == 0 {
				fmt.Fprintf(&b, "### `%s`\n\n- *(no frontmatter)*\n\n", pn.Note.Path)
				continue
			}
			fmt.Fprintf(&b, "### `%s`\n\n", pn.Note.Path)
			for _, f := range pn.Fields {
				mark := ""
				if f.Masked {
					mark = " *(masked)*"
				}
				fmt.Fprintf(&b, "- `%s` = `%s`%s — %s\n", f.Path, f.Value.String(), mark, f.Rule)
			}
			for _, f := range pn.HiddenFields {
				fmt.Fprintf(&b, "- `%s` — **withheld** — %s\n", f.Path, f.Rule)
			}
			if len(pn.Stripped) > 0 {
				ids := []string{}
				for _, s := range pn.Stripped {
					ids = append(ids, s.ID)
				}
				fmt.Fprintf(&b, "- body: **%d secret block(s) stripped** (%s)\n", len(pn.Stripped), strings.Join(ids, ", "))
			}
			if len(pn.Kept) > 0 {
				ids := []string{}
				for _, s := range pn.Kept {
					ids = append(ids, s.ID)
				}
				fmt.Fprintf(&b, "- body: %d secret block(s) kept (revealed: %s)\n", len(pn.Kept), strings.Join(ids, ", "))
			}
			fmt.Fprintf(&b, "\n")
		}
	}
	return b.String()
}

// Itoa is a tiny helper for the JSON output.
func Itoa(i int) string { return strconv.Itoa(i) }
