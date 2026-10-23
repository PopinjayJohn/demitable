// Package pluginhost is the spike's shell assets.
//
// The spike's web shell lives here so it can be embedded in the binary the
// way a real app's shell would be.
package pluginhost

import (
	"embed"
	"io/fs"
)

//go:embed web
var webFS embed.FS

// Web is the shell's asset filesystem.
func Web() fs.FS {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	return sub
}
