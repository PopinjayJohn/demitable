package frontmatter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The SRD corpus in fixtures/ is a *clean* corpus: 1,692 frontmatter blocks,
// four top-level keys, and not one comment, block scalar, anchor, alias,
// merge key, flow collection, nested mapping, wikilink, quoted key, tab or
// CRLF line ending. It proves the parser handles real files; it cannot prove
// the parser handles hard ones.
//
// The fixtures in testdata/ are the adversarial corpus. Each one holds a
// construct the SRD corpus does not have, and each is checked for the
// behaviour the writer depends on.

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return data
}

func TestFixturesParse(t *testing.T) {
	for _, name := range []string{
		"typical.md", "crlf.md", "bom.md", "blockscalars.md", "anchors.md",
		"duplicates.md", "types.md", "nested.md", "unterminated.md",
		"none.md", "empty.md", "notamapping.md",
	} {
		t.Run(name, func(t *testing.T) {
			doc, err := Parse(readFixture(t, name))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			// A parse never fails on a malformed block, so there is
			// nothing to assert beyond "it did not panic". The
			// interesting assertions are below.
			if doc == nil {
				t.Fatal("Parse returned nil")
			}
		})
	}
}

// TestBooleanIsOnlyReportedForAReservedField pins ADR-0050's decision D11.
//
// The extractor must not ask questions about the schema it cannot see. A
// GM's own `dark_mode: true` is a perfectly good boolean and the app has no
// opinion about it. A boolean in a *reserved* field is a value the app
// cannot use, because the reserved field's declared type is not boolean.
func TestBooleanIsOnlyReportedForAReservedField(t *testing.T) {
	doc := mustParse(t, "---\ndark_mode: true\nname: Ilia Vane\n---\n")
	kinds := map[IssueKind]bool{}
	for _, iss := range doc.Block.Issues {
		kinds[iss.Kind] = true
	}
	if kinds[IssueBool] {
		t.Fatalf("an unreserved boolean should not be reported, got %v", doc.Block.Issues)
	}

	doc = mustParse(t, "---\nworkflow: true\n---\n")
	kinds = map[IssueKind]bool{}
	for _, iss := range doc.Block.Issues {
		kinds[iss.Kind] = true
	}
	if !kinds[IssueBool] {
		t.Fatalf("a boolean in a reserved field should be reported, got %v", doc.Block.Issues)
	}
}

func TestFixtureTypical(t *testing.T) {
	doc, err := Parse(readFixture(t, "typical.md"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !doc.HasBlock {
		t.Fatal("expected a frontmatter block")
	}
	if len(doc.Block.Fields) != 9 {
		t.Fatalf("expected 9 fields, got %d", len(doc.Block.Fields))
	}
	// `revealed: false` is a boolean, and the extractor reports every value
	// whose YAML type is not a string. The report is not an error: the file
	// parsed. The caller decides which types are a problem.
	kinds := map[IssueKind]bool{}
	for _, iss := range doc.Block.Issues {
		kinds[iss.Kind] = true
	}
	if len(doc.Block.Issues) != 1 || !kinds[IssueBool] {
		t.Fatalf("expected exactly one bool issue, got %v", doc.Block.Issues)
	}
	// The body is everything after the closing delimiter.
	if !strings.HasPrefix(string(doc.Body), "The body.") {
		t.Fatalf("body = %q", doc.Body)
	}
	// The aliases are quoted in the source and must stay quoted in the
	// value, not become a nested sequence.
	al, ok := doc.Block.Get("aliases")
	if !ok {
		t.Fatal("no aliases field")
	}
	if al.Kind != KindList || len(al.List) != 2 {
		t.Fatalf("aliases = %v", al)
	}
	if al.List[0].Kind != KindString || al.List[0].Str != "Ilia" {
		t.Fatalf("aliases[0] = %v", al.List[0])
	}
	// The wikilink in `discovered` is quoted in the source, so it is a
	// string, not a nested sequence.
	dis, ok := doc.Block.Get("discovered")
	if !ok {
		t.Fatal("no discovered field")
	}
	if dis.Kind != KindList || len(dis.List) != 1 {
		t.Fatalf("discovered = %v", dis)
	}
	if dis.List[0].Kind != KindString || dis.List[0].Str != "[[The Saltmarsh]]" {
		t.Fatalf("discovered[0] = %v", dis.List[0])
	}
}

func TestFixtureCRLF(t *testing.T) {
	doc, err := Parse(readFixture(t, "crlf.md"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc.Block.EOL != "\r\n" {
		t.Fatalf("EOL = %q", doc.Block.EOL)
	}
	if bytesContains(doc.Block.Inner, []byte("\n")) && !bytesContains(doc.Block.Inner, []byte("\r\n")) {
		t.Fatal("expected CRLF throughout")
	}
	// A patch must keep the CRLF endings.
	inner, err := doc.Block.Set("uid", String("01JBX8K2M9Q"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !bytesContains(inner, []byte("\r\n")) {
		t.Fatalf("the patch lost the CRLF endings: %q", inner)
	}
	if bytesContains(inner, []byte("\n\n")) {
		t.Fatalf("the patch doubled a line ending: %q", inner)
	}
	// The trailing comment on `type` must survive a patch of another key.
	if !bytesContains(inner, []byte("# a trailing comment")) {
		t.Fatalf("the patch lost the comment: %q", inner)
	}
}

func TestFixtureBOM(t *testing.T) {
	doc, err := Parse(readFixture(t, "bom.md"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !doc.HasBlock {
		t.Fatal("a BOM before the opening delimiter should still be a frontmatter block")
	}
	if len(doc.Block.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(doc.Block.Fields))
	}
}

func TestFixtureBlockScalars(t *testing.T) {
	doc, err := Parse(readFixture(t, "blockscalars.md"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	lit, ok := doc.Block.Get("literal")
	if !ok {
		t.Fatal("no literal field")
	}
	if lit.Kind != KindString || !strings.Contains(lit.Str, "line one") {
		t.Fatalf("literal = %q", lit.Str)
	}
	// Patching a *different* key must not disturb the block scalar's
	// bytes, its header or its indentation.
	inner, err := doc.Block.Set("keep", String("untouched"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !strings.Contains(string(inner), "literal: |\n  line one\n  line two\n") {
		t.Fatalf("the patch disturbed the block scalar: %q", inner)
	}
	if !strings.Contains(string(inner), "keep: untouched") {
		t.Fatalf("the patch did not apply: %q", inner)
	}
	// The folded scalar and its blank line are still exactly as they were.
	if !strings.Contains(string(inner), "folded: >\n  wrapped\n  text\n") {
		t.Fatalf("the patch disturbed the folded scalar: %q", inner)
	}
}

func TestFixtureAnchors(t *testing.T) {
	doc, err := Parse(readFixture(t, "anchors.md"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	kinds := map[IssueKind]bool{}
	for _, iss := range doc.Block.Issues {
		kinds[iss.Kind] = true
	}
	for _, want := range []IssueKind{IssueAnchor, IssueAlias, IssueMergeKey} {
		if !kinds[want] {
			t.Errorf("expected a %s issue, got %v", want, doc.Block.Issues)
		}
	}
}

func TestFixtureDuplicates(t *testing.T) {
	doc, err := Parse(readFixture(t, "duplicates.md"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	kinds := map[IssueKind]bool{}
	for _, iss := range doc.Block.Issues {
		kinds[iss.Kind] = true
	}
	if !kinds[IssueDuplicateKey] {
		t.Errorf("expected a duplicate-key issue, got %v", doc.Block.Issues)
	}
	if !kinds[IssueNonStringKey] {
		t.Errorf("expected a non-string-key issue, got %v", doc.Block.Issues)
	}
	// The writer must refuse to patch a duplicated key rather than patch
	// the wrong occurrence.
	if _, err := doc.Block.Set("dup", String("x")); err == nil {
		t.Fatal("expected the patch to be refused")
	}
}

func TestFixtureTypes(t *testing.T) {
	doc, err := Parse(readFixture(t, "types.md"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// `date` resolves to a timestamp in both parsers, so it is reported.
	kinds := map[IssueKind]bool{}
	for _, iss := range doc.Block.Issues {
		kinds[iss.Kind] = true
	}
	if !kinds[IssueTimestamp] {
		t.Errorf("expected a timestamp issue, got %v", doc.Block.Issues)
	}
	// `notbool: yes` is a string in yaml.v3 and in js-yaml alike, so it
	// must *not* be reported as a boolean.
	b, ok := doc.Block.Get("notbool")
	if !ok || b.Kind != KindString || b.Str != "yes" {
		t.Fatalf("notbool = %v", b)
	}
	// `link: [[The Saltmarsh]]` is a nested sequence, not a string. This
	// is the trap ADR-0040 exists for: the app must quote a wikilink it
	// writes, because an unquoted one is not a string.
	l, ok := doc.Block.Get("link")
	if !ok {
		t.Fatal("no link field")
	}
	if l.Kind != KindList {
		t.Fatalf("link = %v; an unquoted wikilink is a nested sequence, not a string", l)
	}
	// The quoted one is a string.
	ql, ok := doc.Block.Get("quoted_link")
	if !ok || ql.Kind != KindString || ql.Str != "[[The Saltmarsh]]" {
		t.Fatalf("quoted_link = %v", ql)
	}
}

func TestFixtureNested(t *testing.T) {
	doc, err := Parse(readFixture(t, "nested.md"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// The comment between the list items is part of the block and must
	// survive a patch of another key.
	inner, err := doc.Block.Set("type", String("location"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !bytesContains(inner, []byte("# a comment between the items")) {
		t.Fatalf("the patch lost the comment: %q", inner)
	}
	// A flow sequence reads as a list.
	fs, ok := doc.Block.Get("flowseq")
	if !ok || fs.Kind != KindList || len(fs.List) != 3 {
		t.Fatalf("flowseq = %v", fs)
	}
	// A nested mapping reads as a map.
	co, ok := doc.Block.Get("coords")
	if !ok || co.Kind != KindMap || len(co.Map) != 2 {
		t.Fatalf("coords = %v", co)
	}
}

func TestFixtureUnterminatedAndNone(t *testing.T) {
	doc, err := Parse(readFixture(t, "unterminated.md"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc.HasBlock {
		t.Fatal("an unterminated block is not a frontmatter block")
	}
	if string(doc.Body) != string(readFixture(t, "unterminated.md")) {
		t.Fatal("a file with no block has the whole file as its body")
	}

	doc, err = Parse(readFixture(t, "none.md"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc.HasBlock {
		t.Fatal("expected no frontmatter block")
	}
}

func TestFixtureEmptyAndNotAMapping(t *testing.T) {
	doc, err := Parse(readFixture(t, "empty.md"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !doc.HasBlock {
		t.Fatal("expected a frontmatter block")
	}
	if len(doc.Block.Fields) != 0 {
		t.Fatalf("expected no fields, got %d", len(doc.Block.Fields))
	}
	// A patch to an empty block appends the key.
	inner, err := doc.Block.Set("uid", String("01JBX8K2M9Q"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if string(inner) != "uid: 01JBX8K2M9Q\n" {
		t.Fatalf("inner = %q", inner)
	}

	doc, err = Parse(readFixture(t, "notamapping.md"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	kinds := map[IssueKind]bool{}
	for _, iss := range doc.Block.Issues {
		kinds[iss.Kind] = true
	}
	if !kinds[IssueNested] {
		t.Fatalf("expected a not-a-mapping issue, got %v", doc.Block.Issues)
	}
}

// TestSurgicalWriteLeavesEveryOtherByteAlone is the contract from ADR-0021
// stated as a test: a patch replaces one value's bytes and touches nothing
// else.
func TestSurgicalWriteLeavesEveryOtherByteAlone(t *testing.T) {
	for _, name := range []string{
		"typical.md", "crlf.md", "blockscalars.md", "anchors.md",
		"duplicates.md", "types.md", "nested.md",
	} {
		t.Run(name, func(t *testing.T) {
			doc, err := Parse(readFixture(t, name))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if !doc.HasBlock || len(doc.Block.Fields) == 0 {
				t.Skip("no block to patch")
			}
			// Pick a field that is safe to patch: a plain string with a
			// value, and a key that is not duplicated.
			dupes := map[string]bool{}
			for _, iss := range doc.Block.Issues {
				if iss.Kind == IssueDuplicateKey {
					dupes[strings.ToLower(iss.Key)] = true
				}
			}
			target := ""
			for _, f := range doc.Block.Fields {
				if dupes[strings.ToLower(f.Key)] {
					continue
				}
				if f.IsStringKey && f.HasValue && f.ValueNode.Kind == 8 &&
					f.ValueNode.Style == 0 {
					target = f.Key
					break
				}
			}
			if target == "" {
				t.Skip("no safely patchable field")
			}
			before := string(doc.Block.Inner)
			after, err := doc.Block.Set(target, String("PATCHED"))
			if err != nil {
				t.Fatalf("Set(%q): %v", target, err)
			}
			// Every line of the original that does not mention the
			// patched key must be present, unchanged, in the result.
			for _, line := range strings.Split(before, "\n") {
				if strings.TrimSpace(line) == "" {
					continue
				}
				if strings.HasPrefix(strings.TrimSpace(line), target+":") {
					continue
				}
				// The key may be quoted in the source.
				if strings.HasPrefix(strings.TrimSpace(line), `"`+target+`":`) {
					continue
				}
				if !strings.Contains(string(after), line) {
					t.Fatalf("the patch removed or changed the line %q\nbefore:\n%s\nafter:\n%s",
						line, before, after)
				}
			}
		})
	}
}

func bytesContains(b, sub []byte) bool {
	return strings.Contains(string(b), string(sub))
}
