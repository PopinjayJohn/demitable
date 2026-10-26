package importer

import (
	"context"
	"fmt"
	"os"
	"strings"
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
		if strings.TrimSpace(m.Requires.App) != "" {
			current := opts.AppVersion
			if strings.TrimSpace(current) == "" {
				current = AppVersion
			}
			if !appVersionOK(current, m.Requires.App) {
				err := fmt.Errorf("import: requires.app: required app version %q exceeds current %q", m.Requires.App, current)
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

// appVersionOK reports whether the running app (current) satisfies the
// archive's required app version. A required version equal to or lower
// than current is acceptable; a higher (or unparseable) one is not.
func appVersionOK(current, required string) bool {
	current = strings.TrimSpace(current)
	required = strings.TrimSpace(required)
	if required == "" {
		return true
	}
	if required == current {
		return true
	}
	c, ok1 := parseAppVersion(current)
	r, ok2 := parseAppVersion(required)
	if !ok1 || !ok2 {
		// Fail closed on anything we cannot compare.
		return false
	}
	for i := 0; i < len(c) || i < len(r); i++ {
		cv, rv := 0, 0
		if i < len(c) {
			cv = c[i]
		}
		if i < len(r) {
			rv = r[i]
		}
		if rv > cv {
			return false
		}
		if rv < cv {
			return true
		}
	}
	return true
}

func parseAppVersion(v string) ([]int, bool) {
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return nil, false
		}
		n := 0
		for _, r := range p {
			if r < '0' || r > '9' {
				return nil, false
			}
			n = n*10 + int(r-'0')
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}
