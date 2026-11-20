package search

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestSideChannelTripwire_SetStoreHasNoProductionCallers is the checkable
// guardrail for the recorded deferral of the per-principal BM25 partition
// (ADR-0047, ADR-0054, ADR-0061): the single-index FTSIn path must not
// become the live player-ranking path. It fails if any production code
// (cmd/, plugins/, internal/, excluding *_test.go) calls SetStore, which is
// the only wiring point for the ranked (FTS5) index.
func TestSideChannelTripwire_SetStoreHasNoProductionCallers(t *testing.T) {
	root := moduleRootForTripwire(t)
	callRe := regexp.MustCompile(`\.SetStore\s*\(`)
	dirs := []string{"cmd", "plugins", "internal"}
	var hits []string
	for _, d := range dirs {
		base := filepath.Join(root, d)
		if _, err := os.Stat(base); err != nil {
			continue
		}
		err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(data), "\n") {
				if callRe.MatchString(line) {
					rel, _ := filepath.Rel(root, path)
					hits = append(hits, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(hits) > 0 {
		t.Fatalf("side-channel tripwire: SetStore must have no callers outside tests; found:\n%s", strings.Join(hits, "\n"))
	}
}

func moduleRootForTripwire(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("module root not found")
		}
		dir = parent
	}
}
