package importer

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/PopinjayJohn/demitable/internal/pack"
)

var (
	reportMu   sync.Mutex
	lastReport Report
	hasReport  bool
)

// StoreReport records the most recent report for GET /api/v1/import/report.
func StoreReport(r Report) {
	reportMu.Lock()
	defer reportMu.Unlock()
	lastReport = r
	hasReport = true
}

// LastReport returns the most recent report.
func LastReport() (Report, bool) {
	reportMu.Lock()
	defer reportMu.Unlock()
	if !hasReport {
		return Report{}, false
	}
	return lastReport, true
}

// Import runs the full import pipeline.
func Import(ctx context.Context, opts Options) (Report, error) {
	rep := Report{URL: opts.URL, StartedAt: time.Now().UTC()}
	if opts.URL == "" {
		return rep, fmt.Errorf("import: url is required")
	}
	if opts.VaultRoot == "" {
		return rep, fmt.Errorf("import: vault root is required")
	}

	zipPath, err := FetchArchive(ctx, opts.URL, opts.AllowHTTP)
	if err != nil {
		rep.FinishedAt = time.Now().UTC()
		StoreReport(rep)
		return rep, err
	}
	defer os.Remove(zipPath)

	extractedDir, m, bundles, rejected, err := ExtractAndValidate(zipPath)
	if err != nil {
		rep.Rejected = rejected
		rep.FinishedAt = time.Now().UTC()
		StoreReport(rep)
		return rep, fmt.Errorf("import: %w", err)
	}
	defer os.RemoveAll(extractedDir)

	rep.Rejected = rejected
	rep.PluginBundles = bundles

	// Checksums already verified in ExtractAndValidate.

	// requires check
	if m != nil {
		for _, id := range m.Requires.Packs {
			if _, err := pack.Load(id); err != nil {
				err = fmt.Errorf("import: requires pack %q: %w", id, err)
				rep.FinishedAt = time.Now().UTC()
				StoreReport(rep)
				return rep, err
			}
		}
	}

	// plugin policy
	if len(bundles) > 0 {
		if opts.NoPlugins {
			err := fmt.Errorf("import: archive contains %d plugin bundle(s) and --no-plugins was given", len(bundles))
			rep.FinishedAt = time.Now().UTC()
			StoreReport(rep)
			return rep, err
		}
		if opts.IncludePlugins {
			rep.PluginsInstalled = len(bundles)
		} else if opts.Interactive {
			printPluginWarning(bundles)
			if promptYes() {
				rep.PluginsInstalled = len(bundles)
			} else {
				skipPluginDirs(extractedDir, bundles)
				rep.PluginsSkipped = len(bundles)
			}
		} else {
			skipPluginDirs(extractedDir, bundles)
			rep.PluginsSkipped = len(bundles)
			if rep.Message == "" {
				rep.Message = fmt.Sprintf("skipped %d plugin bundle(s) (use --include-plugins)", len(bundles))
			}
		}
	}

	if err := applyExtracted(opts.VaultRoot, extractedDir, m, opts.Force, opts.Skip, &rep); err != nil {
		rep.FinishedAt = time.Now().UTC()
		StoreReport(rep)
		return rep, fmt.Errorf("import: %w", err)
	}

	rep.FinishedAt = time.Now().UTC()
	if rep.Message == "" {
		rep.Message = fmt.Sprintf("imported %d files", rep.FilesImported)
	}
	StoreReport(rep)
	if opts.Audit != nil {
		opts.Audit.Record(ctx, "import.completed")
	}
	return rep, nil
}
