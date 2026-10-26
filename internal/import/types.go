package importer

import (
	"log/slog"
	"time"

	"github.com/PopinjayJohn/demitable/internal/logging"
)

// Manifest is the optional demitable-import.json descriptor at an archive root.
type Manifest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Kind        string   `json:"kind"`
	Target      string   `json:"target"`
	Requires    Requires `json:"requires"`
	License     string   `json:"license"`
	Attribution string   `json:"attribution"`
	Checksum    string   `json:"checksum"`
	SHA256      string   `json:"sha256"`
}

// Requires declares the archive's dependencies.
type Requires struct {
	Packs []string `json:"packs"`
	App   string   `json:"app"`
}

// PluginBundle is one plugin bundle detected inside an archive.
type PluginBundle struct {
	Path         string   `json:"path"`
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities"`
	DomInline    bool     `json:"dom_inline"`
}

// Report is the outcome of one import attempt.
type Report struct {
	URL              string         `json:"url"`
	StartedAt        time.Time      `json:"started_at"`
	FinishedAt       time.Time      `json:"finished_at"`
	FilesImported    int            `json:"files_imported"`
	FilesSkipped     int            `json:"files_skipped"`
	Rejected         []string       `json:"rejected,omitempty"`
	PluginBundles    []PluginBundle `json:"plugin_bundles,omitempty"`
	PluginsInstalled int            `json:"plugins_installed"`
	PluginsSkipped   int            `json:"plugins_skipped"`
	Message          string         `json:"message,omitempty"`
}

// Options controls one import run.
// AppVersion is the running app's version, used to enforce Requires.App.
// An empty value falls back to the package default.
var AppVersion = "1.0.0"

type Options struct {
	VaultRoot      string
	URL            string
	AppVersion     string
	Force          bool
	Skip           bool
	IncludePlugins bool
	NoPlugins      bool
	AllowHTTP      bool
	Interactive    bool
	Audit          *logging.Audit
	Logger         *slog.Logger
}
