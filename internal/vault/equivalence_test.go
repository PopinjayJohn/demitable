package vault

import (
	"fmt"
	"os"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/store"
)

// TestQueryEquivalenceInMemoryVsSQL pins that the two predicate
// implementations (in-memory EvalExpr used by Vault.List, and the SQL
// compiler used by Store.Query/REST) agree on UID sets and order over a
// shared corpus. If either side diverges, this test fails.
func TestQueryEquivalenceInMemoryVsSQL(t *testing.T) {
	if _, err := os.Stat(srdVaultRoot); err != nil {
		t.Skip("no committed srd vault")
	}
	v, err := Open(srdVaultRoot)
	if err != nil {
		t.Fatal(err)
	}
	if v.Store == nil {
		t.Fatal("no store")
	}
	queries := []string{
		`system == dnd5e`,
		`content_type == monster`,
		`content_type == spell`,
		`data.abilities.str >= 14`,
		`tags contains "compendium/src/5e/xphb"`,
		`content_type == monster and data.abilities.str >= 14`,
		`content_type == spell or content_type == monster`,
		`not content_type == monster`,
	}
	for _, q := range queries {
		// in-memory path
		memNotes := v.List(GM(), Query{Filter: q, Sort: "uid"})
		memUIDs := make([]string, 0, len(memNotes))
		for _, n := range memNotes {
			memUIDs = append(memUIDs, n.UID)
		}
		// SQL path
		res, err := v.Store.Query(store.QueryRequest{Query: q, Sort: "uid", Allowed: nil})
		if err != nil {
			t.Fatalf("sql query %q: %v", q, err)
		}
		sqlUIDs := res.UIDs
		if len(memUIDs) != len(sqlUIDs) {
			t.Fatalf("query %q: count differs mem=%d sql=%d", q, len(memUIDs), len(sqlUIDs))
		}
		for i := range memUIDs {
			if memUIDs[i] != sqlUIDs[i] {
				t.Fatalf("query %q: order differs at %d: mem=%s sql=%s", q, i, memUIDs[i], sqlUIDs[i])
			}
		}
	}
	// Sorting variants
	for _, sortKey := range []string{"", "name", "uid"} {
		for _, desc := range []bool{false, true} {
			q := "system == dnd5e"
			memNotes := v.List(GM(), Query{Filter: q, Sort: sortKey, Desc: desc, Limit: 50})
			memUIDs := make([]string, 0, len(memNotes))
			for _, n := range memNotes {
				memUIDs = append(memUIDs, n.UID)
			}
			res, err := v.Store.Query(store.QueryRequest{Query: q, Sort: sortKey, Desc: desc, Limit: 50, Allowed: nil})
			if err != nil {
				t.Fatalf("sql: %v", err)
			}
			if len(memUIDs) != len(res.UIDs) {
				t.Fatalf("sort=%q desc=%v count differs mem=%d sql=%d", sortKey, desc, len(memUIDs), len(res.UIDs))
			}
			for i := range memUIDs {
				if memUIDs[i] != res.UIDs[i] {
					t.Fatalf("sort=%q desc=%v differs at %d: mem=%s sql=%s", sortKey, desc, i, memUIDs[i], res.UIDs[i])
				}
			}
		}
	}
	_ = fmt.Sprintf
}
