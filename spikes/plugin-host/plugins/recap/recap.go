// Package recap is the spike's UI plugin: a sidebar panel and a light-DOM
// strip beside a session note.
//
// The manifest is a YAML file next to the code, embedded into the binary.
// The alternative — a Go literal — is what plugin B does, so the two
// bindings can be compared.
package recap

import (
	"embed"
	"fmt"
	"io/fs"

	"github.com/PopinjayJohn/demitable/spikes/plugin-host/manifest"
)

//go:embed plugin.yaml
var manifestYAML []byte

//go:embed ui
var uiFS embed.FS

// Manifest parses the embedded manifest and validates it.
func Manifest() (*manifest.Manifest, error) {
	m, err := manifest.Parse(manifestYAML)
	if err != nil {
		return nil, err
	}
	if errs := manifest.Validate(m); len(errs) > 0 {
		return nil, fmt.Errorf("recap: invalid manifest: %w", manifest.Errors(errs))
	}
	return m, nil
}

// UI is the plugin's asset filesystem: the modules the surfaces load and the
// stylesheet the host puts in the shadow root.
func UI() fs.FS {
	sub, err := fs.Sub(uiFS, "ui")
	if err != nil {
		panic(err)
	}
	return sub
}
