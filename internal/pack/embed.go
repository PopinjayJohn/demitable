package pack

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
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

// Load reads a pack by id from the embedded packs filesystem.
func Load(id string) (*Pack, error) {
	sub, err := PacksFS()
	if err != nil {
		return nil, err
	}
	data, err := fs.ReadFile(sub, id+".yaml")
	if err != nil {
		return nil, fmt.Errorf("pack: read %s.yaml: %w", id, err)
	}
	return Parse(data)
}

// ListIDs returns the ids of embedded packs.
func ListIDs() ([]string, error) {
	sub, err := PacksFS()
	if err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(sub, ".")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".yaml") {
			out = append(out, strings.TrimSuffix(name, ".yaml"))
		}
	}
	return out, nil
}
