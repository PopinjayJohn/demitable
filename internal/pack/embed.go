package pack

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

//go:embed all:packs
var embeddedPacks embed.FS

// PacksFS returns the embedded packs filesystem rooted at "packs".
func PacksFS() (fs.FS, error) {
	return fs.Sub(embeddedPacks, "packs")
}

// Example loads the shipped example pack.
func Example() (*Pack, error) {
	sub, err := PacksFS()
	if err != nil {
		return nil, err
	}
	data, err := fs.ReadFile(sub, "example.yaml")
	if err != nil {
		return nil, fmt.Errorf("pack: read example.yaml: %w", err)
	}
	return Parse(data)
}

// ListIDs returns the ids of embedded packs: parsed pack IDs, sorted, so
// every returned id resolves through Load.
func ListIDs() ([]string, error) {
	all, err := LoadAll()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(all))
	for id := range all {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}
