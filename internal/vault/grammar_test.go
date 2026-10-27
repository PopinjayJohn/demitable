package vault

import (
	"errors"
	"os"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/store"
)

func TestGrammarADR0047Shapes(t *testing.T) {
	if _, err := os.Stat(srdVaultRoot); err != nil {
		t.Skip("no committed srd vault")
	}
	v, err := Open(srdVaultRoot)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		query string
		want  int
	}{
		{`type == character`, 0},
		{`system == dnd5e`, 1690},
		{`data.attributes.str >= 14`, 0},
		{`tags contains "npc"`, 0},
		// additional grammar shapes exercised by ADR-0047 operators
		{`content_type == monster`, 370},
		{`content_type == spell`, 349},
		{`data.abilities.str >= 14`, 227},
		{`tags contains "compendium/src/5e/xphb"`, 834},
		{`(type == content or type == monster) and system == dnd5e`, 1690},
		{`not type == character`, 1690},
		{`content_type in ["monster","spell"]`, 719},
	}
	for _, c := range cases {
		res, err := v.Store.Query(store.QueryRequest{Query: c.query, Limit: 0})
		if err != nil {
			t.Fatalf("query %q failed: %v", c.query, err)
		}
		if len(res.UIDs) != c.want {
			t.Errorf("query %q: got %d, want %d", c.query, len(res.UIDs), c.want)
		}
	}
}

func TestGrammarMalformedAssertsErrParse(t *testing.T) {
	if _, err := os.Stat(srdVaultRoot); err != nil {
		t.Skip("no committed srd vault")
	}
	v, err := Open(srdVaultRoot)
	if err != nil {
		t.Fatal(err)
	}
	malformed := []string{
		`type =`,
		`uid ==`,
		`foo bar`,
		`type ==`,
		`>= 5`,
		`type == "x" and`,
		`type == '`,
		`uid') OR ('1'=='1`,
	}
	for _, q := range malformed {
		_, err := v.Store.Query(store.QueryRequest{Query: q})
		if err == nil || !errors.Is(err, store.ErrParse) {
			t.Errorf("malformed %q: expected ErrParse, got %v", q, err)
		}
	}
}
