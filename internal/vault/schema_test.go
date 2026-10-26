package vault

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/pack"
)

func TestSchemaQueriesMechanical(t *testing.T) {
	data, err := os.ReadFile("../../docs/schema.md")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile("(?s)```dataview\\n(.*?)```")
	matches := re.FindAllStringSubmatch(string(data), -1)
	if len(matches) == 0 {
		t.Fatal("no dataview blocks found")
	}

	reserved := map[string]bool{}
	for name := range ReservedFields {
		reserved[strings.ToLower(name)] = true
	}
	pk, err := pack.Example()
	if err != nil {
		t.Fatal(err)
	}
	packFields := map[string]bool{}
	for _, ct := range pk.Content {
		for _, f := range ct.Fields {
			packFields[strings.ToLower(f.Key)] = true
		}
	}
	coreTypes := map[string]bool{
		"character": true, "location": true, "faction": true, "item": true,
		"quest": true, "session": true, "encounter": true, "event": true,
		"table": true, "content": true, "handout": true, "message": true,
		"journal": true, "document": true, "campaign": true, "home": true,
	}

	dataviewKeywords := map[string]bool{
		"table": true, "list": true, "from": true, "where": true, "and": true,
		"or": true, "sort": true, "group": true, "by": true, "limit": true,
		"flatten": true, "as": true,
	}

	for _, m := range matches {
		block := m[1]
		// check quoted type values
		typeRe := regexp.MustCompile(`type\s*=\s*"([^"]+)"`)
		for _, tm := range typeRe.FindAllStringSubmatch(block, -1) {
			if !coreTypes[tm[1]] {
				t.Errorf("query names non-core type %q in block:\n%s", tm[1], block)
			}
		}
		// tokenize identifiers
		for _, tok := range strings.FieldsFunc(block, func(r rune) bool {
			return !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
		}) {
			low := strings.ToLower(tok)
			if dataviewKeywords[low] {
				continue
			}
			if reserved[low] || packFields[low] || coreTypes[low] {
				continue
			}
			// skip numeric literals
			if _, err := parseUint(tok); err == nil {
				continue
			}
			// unknown identifier: could be a field we don't recognise
			t.Errorf("query names unknown field %q in block:\n%s", tok, block)
		}
	}
}

func parseUint(s string) (uint64, error) {
	// tiny helper to avoid strconv import noise
	var n uint64
	if s == "" {
		return 0, errParse
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errParse
		}
		n = n*10 + uint64(c-'0')
	}
	return n, nil
}

var errParse = &parseError{}

type parseError struct{}

func (e *parseError) Error() string { return "not a number" }
