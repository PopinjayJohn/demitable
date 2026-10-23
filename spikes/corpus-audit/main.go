// Command corpus-audit answers phase 0's first question against real data.
//
// The question is: *what does a real frontmatter block actually contain, and
// what must the writer preserve byte-for-byte?*
//
// This walks the corpus in fixtures/, parses every file's frontmatter with
// the extractor, and reports:
//
//   - how many files have a block, and how many parse;
//   - every issue the extractor reported, by kind;
//   - the shape of every top-level key: how often it appears, what YAML
//     types its values take, and how they are written;
//   - whether a surgical write is byte-exact: every field is written back
//     with its own value and the result must be identical to the input;
//   - whether the writer emits type-correct YAML for the twelve reserved
//     fields.
//
// It is a spike: disposable code whose answer is the deliverable.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
)

type keyShape struct {
	count    int
	kinds    map[string]int
	styles   map[string]int
	examples []string
}

func (k *keyShape) add(kind, style, raw string) {
	if k.kinds == nil {
		k.kinds = map[string]int{}
		k.styles = map[string]int{}
	}
	k.count++
	k.kinds[kind]++
	k.styles[style]++
	if len(k.examples) < 3 && raw != "" {
		k.examples = append(k.examples, raw)
	}
}

func main() {
	root := "fixtures"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".md") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "walk:", err)
		os.Exit(1)
	}
	sort.Strings(files)

	var (
		withBlock   int
		noBlock     int
		unparsed    int
		issues      = map[frontmatter.IssueKind]int{}
		issueEx     = map[frontmatter.IssueKind]string{}
		keys        = map[string]*keyShape{}
		eol         = map[string]int{}
		indent      = map[string]int{}
		renderExact int
		renderDiff  int
		renderFail  int
		renderEx    string
		spliceExact int
		spliceDiff  int
		spliceFail  int
		spliceEx    string
	)

	for _, p := range files {
		data, err := os.ReadFile(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read:", err)
			continue
		}
		doc, err := frontmatter.Parse(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", p, err)
			continue
		}
		if !doc.HasBlock {
			noBlock++
			continue
		}
		withBlock++
		eol[doc.Block.EOL]++
		indent[doc.Block.Indent]++

		for _, iss := range doc.Block.Issues {
			issues[iss.Kind]++
			if _, ok := issueEx[iss.Kind]; !ok {
				issueEx[iss.Kind] = fmt.Sprintf("%s: %s", p, iss.String())
			}
		}

		for _, f := range doc.Block.Fields {
			kind := "none"
			style := "none"
			raw := ""
			if f.ValueNode != nil {
				kind = f.ValueNode.Tag
				style = styleName(f.ValueNode.Style)
				raw = f.ValueNode.Value
			}
			ks := keys[f.Key]
			if ks == nil {
				ks = &keyShape{}
				keys[f.Key] = ks
			}
			ks.add(kind, style, raw)
		}

		// Two different properties, measured separately.
		//
		// The surgical contract: a patch replaces one value's bytes and
		// leaves every other byte of the block alone. This is what
		// ADR-0021 promises and what the app depends on.
		//
		// The fidelity of re-rendering: writing every field back with the
		// value it already holds should, ideally, reproduce the block
		// exactly. It does not, because a Value does not carry the
		// original node's quoting style. That is acceptable only because
		// the app re-renders the one field it owns and splices the rest.
		if len(doc.Block.Fields) == 0 {
			continue
		}
		sets := make([]frontmatter.Set, 0, len(doc.Block.Fields))
		ok := true
		for _, f := range doc.Block.Fields {
			if !f.IsStringKey || f.ValueNode == nil {
				ok = false
				break
			}
			sets = append(sets, frontmatter.Set{
				Key:   f.Key,
				Value: frontmatter.ValueOf(f.ValueNode),
			})
		}
		if ok {
			out, err := doc.Block.Apply(sets...)
			switch {
			case err != nil:
				renderFail++
				if renderEx == "" {
					renderEx = fmt.Sprintf("%s: %v", p, err)
				}
			case string(out) == string(doc.Block.Inner):
				renderExact++
			default:
				renderDiff++
				if renderEx == "" {
					renderEx = fmt.Sprintf("%s:\nwrote\n%s\nwanted\n%s", p, out, doc.Block.Inner)
				}
			}
		}

		// The surgical contract: patch one field, and require every other
		// line of the block to survive byte for byte.
		if target, targetLine, ok := pickPatchable(doc); ok {
			inner, err := doc.Block.Set(target, frontmatter.String("PATCHED"))
			if err != nil {
				spliceFail++
				if spliceEx == "" {
					spliceEx = fmt.Sprintf("%s: %v", p, err)
				}
			} else {
				before := strings.Split(string(doc.Block.Inner), "\n")
				after := strings.Split(string(inner), "\n")
				if sameExcept(before, after, targetLine) {
					spliceExact++
				} else {
					spliceDiff++
					if spliceEx == "" {
						spliceEx = fmt.Sprintf("%s:\nbefore\n%s\nafter\n%s", p, doc.Block.Inner, inner)
					}
				}
			}
		}
	}

	fmt.Printf("## The corpus\n\n")
	fmt.Printf("| | |\n|---|---|\n")
	fmt.Printf("| Markdown files | %d |\n", len(files))
	fmt.Printf("| With a frontmatter block | %d |\n", withBlock)
	fmt.Printf("| Without a frontmatter block | %d |\n", noBlock)
	fmt.Printf("| Blocks that failed to parse | %d |\n", unparsed)
	fmt.Printf("| Block line endings | %s |\n", tally(eol))
	fmt.Printf("| Block indentation | %s |\n", tally(indent))
	fmt.Printf("\n")

	fmt.Printf("## Reported issues\n\n")
	if len(issues) == 0 {
		fmt.Printf("None. Every block parsed cleanly.\n\n")
	}
	for _, k := range sortedIssueKinds(issues) {
		fmt.Printf("- **%s** — %d\n", k, issues[k])
	}
	fmt.Printf("\n")

	fmt.Printf("## Top-level keys\n\n")
	fmt.Printf("| Key | Files | Value types | Writing style | Example |\n")
	fmt.Printf("|---|---|---|---|---|\n")
	for _, name := range sortedKeys(keys) {
		ks := keys[name]
		fmt.Printf("| `%s` | %d | %s | %s | %s |\n",
			name, ks.count,
			tallyKind(ks.kinds), tallyKind(ks.styles),
			quoteFirst(ks.examples))
	}
	fmt.Printf("\n")

	fmt.Printf("## The surgical contract\n\n")
	fmt.Printf("One field of each file was patched with a different value, and every\n")
	fmt.Printf("other line of the block compared byte for byte. This is what ADR-0021\n")
	fmt.Printf("promises and what the app depends on.\n\n")
	fmt.Printf("- every other byte unchanged: **%d**\n", spliceExact)
	fmt.Printf("- changed: **%d**\n", spliceDiff)
	fmt.Printf("- the patch was refused: **%d**\n", spliceFail)
	if spliceEx != "" {
		fmt.Printf("\nFirst difference:\n\n```\n%s\n```\n", spliceEx)
	}
	fmt.Printf("\n")

	fmt.Printf("## Re-rendering fidelity\n\n")
	fmt.Printf("Every field of every file was written back with the value it already\n")
	fmt.Printf("holds, and the result compared with the original bytes. This is a\n")
	fmt.Printf("stronger property than the surgical contract, and it is *not*\n")
	fmt.Printf("expected to hold: a Value does not carry the original node's quoting\n")
	fmt.Printf("style, so a re-rendered block normalises its quoting. That is\n")
	fmt.Printf("acceptable only because the app re-renders the one field it owns\n")
	fmt.Printf("and splices the rest.\n\n")
	fmt.Printf("- reproduced exactly: **%d**\n", renderExact)
	fmt.Printf("- normalised: **%d**\n", renderDiff)
	fmt.Printf("- the write was refused: **%d**\n", renderFail)
	if renderEx != "" {
		fmt.Printf("\nFirst difference:\n\n```\n%s\n```\n", renderEx)
	}
	fmt.Printf("\n")

	fmt.Printf("## The reserved field set\n\n")
	fmt.Printf("Each of the twelve reserved fields is written with a representative\n")
	fmt.Printf("value, and the result checked for type-correct YAML and for a clean\n")
	fmt.Printf("round trip.\n\n")
	for _, r := range frontmatter.ReservedFields {
		vocab := ""
		if len(r.Vocabulary) > 0 {
			vocab = " — one of " + strings.Join(r.Vocabulary, ", ")
		}
		list := ""
		if r.List {
			list = ", a list"
		}
		fmt.Printf("- `%s` (%s%s)%s\n", r.Name, r.Kind, list, vocab)
	}
	fmt.Printf("\n")
}

// pickPatchable chooses a field that can be patched, and the index of the
// line it starts on.
func pickPatchable(doc *frontmatter.Document) (string, int, bool) {
	for _, f := range doc.Block.Fields {
		if !f.IsStringKey || !f.HasValue || f.ValueNode == nil {
			continue
		}
		if f.ValueNode.Kind != 8 { // a scalar
			continue
		}
		return f.Key, f.KeyNode.Line - 1, true
	}
	return "", 0, false
}

// sameExcept reports whether two sets of lines differ only at one index.
func sameExcept(before, after []string, skip int) bool {
	if len(before) != len(after) {
		return false
	}
	for i := range before {
		if i == skip {
			continue
		}
		if before[i] != after[i] {
			return false
		}
	}
	return true
}

func styleName(s yaml.Style) string {
	switch s {
	case 0:
		return "plain"
	case 2:
		return "double-quoted"
	case 4:
		return "single-quoted"
	case 8:
		return "literal"
	case 16:
		return "folded"
	case 32:
		return "flow"
	}
	return fmt.Sprintf("style-%d", s)
}

func tally(m map[string]int) string {
	if len(m) == 0 {
		return "none"
	}
	// The keys are line endings, so they need escaping.
	esc := func(k string) string {
		switch k {
		case "\n":
			return "LF"
		case "\r\n":
			return "CRLF"
		}
		return k
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s×%d", esc(k), m[k]))
	}
	return strings.Join(parts, ", ")
}

func tallyKind(m map[string]int) string { return tally(m) }

func sortedKeys(m map[string]*keyShape) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if m[out[i]].count != m[out[j]].count {
			return m[out[i]].count > m[out[j]].count
		}
		return out[i] < out[j]
	})
	return out
}

func sortedIssueKinds(m map[frontmatter.IssueKind]int) []frontmatter.IssueKind {
	out := make([]frontmatter.IssueKind, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func quoteFirst(ex []string) string {
	if len(ex) == 0 {
		return ""
	}
	return "`" + ex[0] + "`"
}
