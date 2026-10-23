package frontmatter

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func mustParse(t *testing.T, src string) *Document {
	t.Helper()
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return doc
}

// patch returns the block's inner bytes after setting one key.
func patch(t *testing.T, blk *Block, key string, v Value) string {
	t.Helper()
	inner, err := blk.Set(key, v)
	if err != nil {
		t.Fatalf("Set(%q): %v", key, err)
	}
	return string(inner)
}

func TestParseFindsBlock(t *testing.T) {
	doc := mustParse(t, "---\ntype: character\n---\n# Body\n")
	if !doc.HasBlock {
		t.Fatal("expected a frontmatter block")
	}
	if got := string(doc.Block.Inner); got != "type: character\n" {
		t.Fatalf("inner = %q", got)
	}
	if got := string(doc.Body); got != "# Body\n" {
		t.Fatalf("body = %q", got)
	}
	if v, ok := doc.Block.Get("type"); !ok || v.Str != "character" {
		t.Fatalf("Get(type) = %v, %v", v, ok)
	}
}

func TestParseNoBlock(t *testing.T) {
	doc := mustParse(t, "# Just a note\n")
	if doc.HasBlock {
		t.Fatal("expected no frontmatter block")
	}
	if got := string(doc.Body); got != "# Just a note\n" {
		t.Fatalf("body = %q", got)
	}
}

func TestParseUnterminatedBlockIsNotABlock(t *testing.T) {
	doc := mustParse(t, "---\ntype: character\n")
	if doc.HasBlock {
		t.Fatal("an unterminated block is not a block")
	}
}

func TestParseReportsUnparsableBlock(t *testing.T) {
	doc := mustParse(t, "---\ntype: [unclosed\n---\nbody\n")
	if !doc.HasBlock {
		t.Fatal("expected a block")
	}
	if len(doc.Block.Issues) == 0 {
		t.Fatal("expected an issue")
	}
	if doc.Block.Issues[0].Kind != IssueUnparsed {
		t.Fatalf("issue kind = %v", doc.Block.Issues[0].Kind)
	}
	// The bytes are still there, untouched.
	if !bytes.Contains(doc.Block.Inner, []byte("unclosed")) {
		t.Fatal("the block's bytes were not preserved")
	}
}

func TestParseReportsDuplicateKey(t *testing.T) {
	doc := mustParse(t, "---\na: 1\na: 2\n---\n")
	found := false
	for _, iss := range doc.Block.Issues {
		if iss.Kind == IssueDuplicateKey {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a duplicate-key issue, got %v", doc.Block.Issues)
	}
}

func TestParseReportsNonStringKey(t *testing.T) {
	doc := mustParse(t, "---\n1: one\ntrue: yes\n---\n")
	kinds := map[IssueKind]bool{}
	for _, iss := range doc.Block.Issues {
		kinds[iss.Kind] = true
	}
	if !kinds[IssueNonStringKey] {
		t.Fatalf("expected a non-string-key issue, got %v", doc.Block.Issues)
	}
}

func TestParseReportsMergeKey(t *testing.T) {
	doc := mustParse(t, "---\nbase: &b\n  x: 1\nchild:\n  <<: *b\n  y: 2\n---\n")
	kinds := map[IssueKind]bool{}
	for _, iss := range doc.Block.Issues {
		kinds[iss.Kind] = true
	}
	if !kinds[IssueMergeKey] {
		t.Fatalf("expected a merge-key issue, got %v", doc.Block.Issues)
	}
	if !kinds[IssueAnchor] || !kinds[IssueAlias] {
		t.Fatalf("expected anchor and alias issues, got %v", doc.Block.Issues)
	}
}

func TestParseReportsTimestamp(t *testing.T) {
	doc := mustParse(t, "---\nwhen: 2026-10-23\n---\n")
	found := false
	for _, iss := range doc.Block.Issues {
		if iss.Kind == IssueTimestamp {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a timestamp issue, got %v", doc.Block.Issues)
	}
}

func TestParseCRLF(t *testing.T) {
	doc := mustParse(t, "---\r\ntype: character\r\n---\r\nbody\r\n")
	if !doc.HasBlock {
		t.Fatal("expected a block")
	}
	if got := string(doc.Block.Inner); got != "type: character\r\n" {
		t.Fatalf("inner = %q", got)
	}
	if doc.Block.EOL != "\r\n" {
		t.Fatalf("EOL = %q", doc.Block.EOL)
	}
}

func TestParseBOM(t *testing.T) {
	doc := mustParse(t, "\xef\xbb\xbf---\ntype: character\n---\n")
	if !doc.HasBlock {
		t.Fatal("expected a block")
	}
}

// --- the surgical write -------------------------------------------------

func TestSurgicalWriteLeavesEveryOtherByteIdentical(t *testing.T) {
	src := "---\n" +
		"# a comment about the block\n" +
		"type: character\n" +
		"name: \"Ilia Vane\"\n" +
		"tags:\n" +
		"  - npc\n" +
		"  - rival\n" +
		"unknown_key: keep me exactly\n" +
		"folded: >\n" +
		"  one\n" +
		"  two\n" +
		"---\n" +
		"body\n"
	doc := mustParse(t, src)
	before := doc.Block.Inner

	inner, err := doc.Block.Set("name", String("Ilia Vane-Reed"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	want := strings.Replace(string(before), `name: "Ilia Vane"`, `name: Ilia Vane-Reed`, 1)
	if string(inner) != want {
		t.Fatalf("inner changed unexpectedly:\n got: %q\nwant: %q", inner, want)
	}
}

func TestSurgicalWriteKeepsQuotingStyleOfOtherKeys(t *testing.T) {
	src := "---\na: \"quoted\"\nb: plain\nc: 'single'\n---\n"
	doc := mustParse(t, src)
	inner, err := doc.Block.Set("b", String("changed"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	got := string(inner)
	if !strings.Contains(got, `a: "quoted"`) {
		t.Fatalf("a lost its quoting: %q", got)
	}
	if !strings.Contains(got, `c: 'single'`) {
		t.Fatalf("c lost its quoting: %q", got)
	}
}

func TestSurgicalWriteKeepsKeyOrder(t *testing.T) {
	src := "---\nzeta: 1\nalpha: 2\nmu: 3\n---\n"
	doc := mustParse(t, src)
	inner, err := doc.Block.Set("alpha", String("x"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	want := "---\nzeta: 1\nalpha: x\nmu: 3\n---\n"
	got := "---\n" + string(inner) + "---\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSurgicalWriteCRLF(t *testing.T) {
	src := "---\r\na: 1\r\nb: 2\r\n---\r\n"
	doc := mustParse(t, src)
	inner, err := doc.Block.Set("a", String("x"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := string(inner); got != "a: x\r\nb: 2\r\n" {
		t.Fatalf("inner = %q", got)
	}
}

func TestSurgicalWriteAddsNewKey(t *testing.T) {
	src := "---\na: 1\n---\n"
	doc := mustParse(t, src)
	inner, err := doc.Block.Set("uid", String("01JBX8K2M9Q"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := string(inner); got != "a: 1\nuid: 01JBX8K2M9Q\n" {
		t.Fatalf("inner = %q", got)
	}
}

func TestSurgicalWriteAddsNewKeyCRLF(t *testing.T) {
	src := "---\r\na: 1\r\n---\r\n"
	doc := mustParse(t, src)
	inner, err := doc.Block.Set("uid", String("01JBX8K2M9Q"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := string(inner); got != "a: 1\r\nuid: 01JBX8K2M9Q\r\n" {
		t.Fatalf("inner = %q", got)
	}
}

func TestSurgicalWriteRefusesDuplicateKey(t *testing.T) {
	doc := mustParse(t, "---\na: 1\na: 2\n---\n")
	_, err := doc.Block.Set("a", String("x"))
	if err == nil || !strings.Contains(err.Error(), "appears more than once") {
		t.Fatalf("expected a duplicate-key error, got %v", err)
	}
}

func TestSurgicalWriteCaseInsensitiveKey(t *testing.T) {
	doc := mustParse(t, "---\nType: character\n---\n")
	inner, err := doc.Block.Set("type", String("monster"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := string(inner); got != "Type: monster\n" {
		t.Fatalf("inner = %q", got)
	}
}

func TestSurgicalWriteReplacesBlockSequence(t *testing.T) {
	src := "---\ntags:\n  - a\n  - b\nother: keep\n---\n"
	doc := mustParse(t, src)
	inner, err := doc.Block.Set("tags", Strings("x", "y"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	want := "tags:\n  - x\n  - y\nother: keep\n"
	if got := string(inner); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSurgicalWriteReplacesScalarWithBlockSequence(t *testing.T) {
	src := "---\ntags: a\nother: keep\n---\n"
	doc := mustParse(t, src)
	inner, err := doc.Block.Set("tags", Strings("x", "y"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	// The block had no sequence to learn a style from, so the writer uses
	// the compact style — the one every file in the SRD corpus uses.
	want := "tags:\n- x\n- y\nother: keep\n"
	if got := string(inner); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSurgicalWriteReplacesBlockSequenceWithScalar(t *testing.T) {
	src := "---\ntags:\n  - a\n  - b\nother: keep\n---\n"
	doc := mustParse(t, src)
	inner, err := doc.Block.Set("tags", String("x"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	want := "tags: x\nother: keep\n"
	if got := string(inner); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSurgicalWriteReplacesFlowSequence(t *testing.T) {
	src := "---\ntags: [a, b]\nother: keep\n---\n"
	doc := mustParse(t, src)
	inner, err := doc.Block.Set("tags", Strings("x", "y"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	want := "tags:\n- x\n- y\nother: keep\n"
	if got := string(inner); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// TestWriterPreservesTheBlocksSequenceStyle pins the rule that a list is
// written in the style of the block it is written into.
//
// The SRD corpus uses the compact style — the items sit at the same column
// as their key — in all 1,692 of its files. A block that uses the indented
// style keeps it. Only a block with no sequence at all falls back to the
// default, which is the compact style.
func TestWriterPreservesTheBlocksSequenceStyle(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "compact",
			src:  "---\ntags:\n- a\nother: keep\n---\n",
			want: "tags:\n- x\n- y\nother: keep\n",
		},
		{
			name: "indented",
			src:  "---\ntags:\n  - a\nother: keep\n---\n",
			want: "tags:\n  - x\n  - y\nother: keep\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := mustParse(t, c.src)
			inner, err := doc.Block.Set("tags", Strings("x", "y"))
			if err != nil {
				t.Fatalf("Set: %v", err)
			}
			if got := string(inner); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}

// TestWriterNeverInlinesAList pins a bug the corpus audit found.
//
// A one-item list rendered as a single line was written as `key: - item`,
// which is not YAML. The parser then refused it.
func TestWriterNeverInlinesAList(t *testing.T) {
	doc := mustParse(t, "---\n---\n")
	inner, err := doc.Block.Set("tags", Strings("only"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	want := "tags:\n- only\n"
	if got := string(inner); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	// And it reads back as a list, not as a string.
	doc2 := mustParse(t, "---\n"+string(inner)+"---\n")
	v, ok := doc2.Block.Get("tags")
	if !ok || v.Kind != KindList || len(v.List) != 1 {
		t.Fatalf("read back as %v", v)
	}
}

func TestSurgicalWritePreservesComments(t *testing.T) {
	src := "---\n# about a\na: 1\n# about b\nb: 2\n---\n"
	doc := mustParse(t, src)
	inner, err := doc.Block.Set("a", String("x"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	got := string(inner)
	if !strings.Contains(got, "# about a") {
		t.Fatalf("lost a comment: %q", got)
	}
	if !strings.Contains(got, "# about b") {
		t.Fatalf("lost a comment: %q", got)
	}
}

func TestSurgicalWritePreservesInlineComment(t *testing.T) {
	src := "---\na: 1 # inline\nb: 2\n---\n"
	doc := mustParse(t, src)
	inner, err := doc.Block.Set("a", String("x"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := string(inner); got != "a: x # inline\nb: 2\n" {
		t.Fatalf("inner = %q", got)
	}
}

func TestSurgicalWritePreservesFoldedScalar(t *testing.T) {
	src := "---\nfolded: >\n  one\n  two\nother: keep\n---\n"
	doc := mustParse(t, src)
	inner, err := doc.Block.Set("other", String("x"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := string(inner); got != "folded: >\n  one\n  two\nother: x\n" {
		t.Fatalf("inner = %q", got)
	}
}

func TestSurgicalWritePreservesMergeKey(t *testing.T) {
	src := "---\nbase: &b\n  x: 1\nchild:\n  <<: *b\n  y: 2\nother: keep\n---\n"
	doc := mustParse(t, src)
	inner, err := doc.Block.Set("other", String("x"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := string(inner); got != "base: &b\n  x: 1\nchild:\n  <<: *b\n  y: 2\nother: x\n" {
		t.Fatalf("inner = %q", got)
	}
}

func TestSurgicalWriteRefusesNonStringKey(t *testing.T) {
	doc := mustParse(t, "---\n1: one\n---\n")
	_, err := doc.Block.Set("1", String("x"))
	if err == nil {
		t.Fatal("expected an error")
	}
}

// --- type-correct emission ---------------------------------------------

func TestWriterEmitsBooleansNotYesNo(t *testing.T) {
	doc := mustParse(t, "---\nflag: true\n---\n")
	inner, err := doc.Block.Set("flag", Bool(false))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := string(inner); got != "flag: false\n" {
		t.Fatalf("inner = %q", got)
	}
}

func TestWriterQuotesWikilinks(t *testing.T) {
	doc := mustParse(t, "---\n---\n")
	inner, err := doc.Block.Set("faction", Link("[[Saltmarsh Cartel]]"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := string(inner); got != "faction: \"[[Saltmarsh Cartel]]\"\n" {
		t.Fatalf("inner = %q", got)
	}
	// And it reads back as a string, not as a nested sequence.
	doc2 := mustParse(t, "---\n"+string(inner)+"---\n")
	v, ok := doc2.Block.Get("faction")
	if !ok || v.Kind != KindString || v.Str != "[[Saltmarsh Cartel]]" {
		t.Fatalf("read back as %v (%v)", v, v.Kind)
	}
}

func TestWriterQuotesWikilinksInAList(t *testing.T) {
	doc := mustParse(t, "---\n---\n")
	inner, err := doc.Block.Set("related", Value{Kind: KindList, List: []Value{
		Link("[[A]]"), Link("[[B]]"),
	}})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	want := "related:\n- \"[[A]]\"\n- \"[[B]]\"\n"
	if got := string(inner); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestWriterQuotesStringsThatWouldParseAsOtherTypes(t *testing.T) {
	// The list is what yaml.v3 and js-yaml together require. It was
	// measured, not guessed; see ADR-0049.
	cases := []struct {
		in   string
		want string
	}{
		{"true", "\"true\""},
		{"True", "\"True\""},
		{"false", "\"false\""},
		{"null", "\"null\""},
		{"Null", "\"Null\""},
		{"~", "\"~\""},
		{"123", "\"123\""},
		{"1.5", "\"1.5\""},
		{"1_000", "\"1_000\""},
		{"1e3", "\"1e3\""},
		{"2026-10-23", "\"2026-10-23\""},
		{"2026-13-45", "\"2026-13-45\""},
		{"- dash", "\"- dash\""},
		{"a: b", "\"a: b\""},
		{"has # hash", "\"has # hash\""},
		{"", "\"\""},
	}
	for _, c := range cases {
		doc := mustParse(t, "---\n---\n")
		inner, err := doc.Block.Set("k", String(c.in))
		if err != nil {
			t.Fatalf("Set(%q): %v", c.in, err)
		}
		want := "k: " + c.want + "\n"
		if got := string(inner); got != want {
			t.Errorf("Set(%q) = %q, want %q", c.in, got, want)
		}
	}
}

// TestWriterLeavesYAML11BooleansUnquoted pins a deliberate decision.
//
// YAML 1.1 readers read `yes`, `no`, `on`, `off`, `y` and `n` as booleans,
// but yaml.v3 and js-yaml both read them as strings, and no tool in the
// vault's toolchain is a YAML 1.1 reader. The app therefore leaves them
// plain, and writes its own booleans as `true` and `false`.
func TestWriterLeavesYAML11BooleansUnquoted(t *testing.T) {
	for _, in := range []string{"yes", "no", "on", "off", "y", "n", "Yes", "No", "ON", "OFF"} {
		doc := mustParse(t, "---\n---\n")
		inner, err := doc.Block.Set("k", String(in))
		if err != nil {
			t.Fatalf("Set(%q): %v", in, err)
		}
		want := "k: " + in + "\n"
		if got := string(inner); got != want {
			t.Errorf("Set(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWriterLeavesPlainStringsUnquoted(t *testing.T) {
	doc := mustParse(t, "---\n---\n")
	inner, err := doc.Block.Set("k", String("Ilia Vane"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := string(inner); got != "k: Ilia Vane\n" {
		t.Fatalf("inner = %q", got)
	}
}

func TestWriterQuotesWikiLinkInPlainStringContext(t *testing.T) {
	// A plain string that happens to look like a wikilink is still a
	// wikilink, and must be quoted.
	doc := mustParse(t, "---\n---\n")
	inner, err := doc.Block.Set("k", String("[[Note]]"))
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := string(inner); got != "k: \"[[Note]]\"\n" {
		t.Fatalf("inner = %q", got)
	}
}

// --- the reserved field set ---------------------------------------------

func TestReservedFieldCoercion(t *testing.T) {
	r := ReservedByName["visibility"]
	v, err := r.Coerce(String("gm"))
	if err != nil {
		t.Fatalf("Coerce: %v", err)
	}
	if v.Str != "gm" {
		t.Fatalf("got %v", v)
	}
	if err := r.CheckVocabulary(v); err != nil {
		t.Fatalf("CheckVocabulary: %v", err)
	}
	if err := r.CheckVocabulary(String("everyone")); err == nil {
		t.Fatal("expected a vocabulary error")
	}
}

func TestReservedFieldListCoercion(t *testing.T) {
	r := ReservedByName["grants"]
	v, err := r.Coerce(String("p_8f2a"))
	if err != nil {
		t.Fatalf("Coerce: %v", err)
	}
	if v.Kind != KindList || len(v.List) != 1 {
		t.Fatalf("got %v", v)
	}
}

func TestIsReserved(t *testing.T) {
	if !IsReserved("TYPE") {
		t.Fatal("TYPE is reserved")
	}
	if IsReserved("status") {
		t.Fatal("status is not reserved")
	}
	if got := CanonicalName("TYPE"); got != "type" {
		t.Fatalf("CanonicalName = %q", got)
	}
}

// --- the type registry ---------------------------------------------------

func TestTypeRegistry(t *testing.T) {
	reg, err := ParseTypeRegistry([]byte(`{"types":{"hp":"number","tags":"tags","when":"date"}}`))
	if err != nil {
		t.Fatalf("ParseTypeRegistry: %v", err)
	}
	if got, ok := reg.Type("HP"); !ok || got != PropertyNumber {
		t.Fatalf("Type(HP) = %v, %v", got, ok)
	}
	if got, ok := reg.Type("tags"); !ok || got != PropertyList {
		t.Fatalf("Type(tags) = %v, %v", got, ok)
	}
	if got, ok := reg.Type("when"); !ok || got != PropertyDate {
		t.Fatalf("Type(when) = %v, %v", got, ok)
	}
	if _, ok := reg.Type("nope"); ok {
		t.Fatal("expected no type")
	}
}

func TestTypeRegistryFlatShape(t *testing.T) {
	reg, err := ParseTypeRegistry([]byte(`{"hp":"number"}`))
	if err != nil {
		t.Fatalf("ParseTypeRegistry: %v", err)
	}
	if got, ok := reg.Type("hp"); !ok || got != PropertyNumber {
		t.Fatalf("Type(hp) = %v, %v", got, ok)
	}
}

func TestTypeRegistryMissingFile(t *testing.T) {
	reg, err := LoadTypeRegistry(filepath.Join(t.TempDir(), "types.json"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if reg == nil {
		t.Fatal("expected a registry")
	}
}
