package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/plugin/wasm"
)

func TestLoadWasmFromDir(t *testing.T) {
	dir := t.TempDir()
	manifest := "id: test.loader\nname: Loader\nversion: 1.0.0\napi: 1\n"
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "logic.wasm"), wasm.GoldenABIModule(), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry()
	n, err := LoadWasmFromDir(dir, reg)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("loaded %d, want 1", n)
	}
	if _, ok := reg.Find("test.loader"); !ok {
		t.Fatal("plugin not registered")
	}
}
