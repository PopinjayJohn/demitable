package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PopinjayJohn/demitable/internal/plugin/manifest"
)

// LoadWasmFromDir walks root and registers every plugin bundle that contains
// a logic.wasm file. It returns the number of plugins registered.
func LoadWasmFromDir(root string, reg *Registry) (int, error) {
	if reg == nil {
		reg = DefaultRegistry()
	}
	count := 0
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if filepath.Base(p) != "plugin.yaml" && filepath.Base(p) != "plugin.yml" && filepath.Base(p) != "manifest.yaml" && filepath.Base(p) != "manifest.yml" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		m, err := manifest.Parse(data)
		if err != nil {
			return nil
		}
		if errs := manifest.Validate(m); len(errs) > 0 {
			return nil
		}
		dir := filepath.Dir(p)
		wasmPath := filepath.Join(dir, "logic.wasm")
		wasmBytes, err := os.ReadFile(wasmPath)
		if err != nil {
			// not a wasm bundle
			return nil
		}
		if err := reg.RegisterWasm(m, wasmBytes); err != nil {
			// duplicate or invalid; skip
			return nil
		}
		count++
		_ = wasmPath
		return nil
	})
	if err != nil {
		return count, fmt.Errorf("plugin: walk %s: %w", root, err)
	}
	return count, nil
}

// LoadWasmPlugins is an alias.
func LoadWasmPlugins(root string, reg *Registry) (int, error) {
	return LoadWasmFromDir(root, reg)
}

// LoadWasmPluginsFromVault scans a vault root for plugin bundles.
func LoadWasmPluginsFromVault(vaultRoot string, reg *Registry) (int, error) {
	return LoadWasmFromDir(vaultRoot, reg)
}
