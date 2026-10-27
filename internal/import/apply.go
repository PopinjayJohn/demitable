package importer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

// applyExtracted moves the validated extracted tree into the vault.
func applyExtracted(vaultRoot string, extractedDir string, m *Manifest, force, skip bool, rep *Report) error {
	targetRoot := vaultRoot
	if m != nil && strings.TrimSpace(m.Target) != "" {
		t := pathClean(m.Target)
		if t == "" || t == "." {
			targetRoot = vaultRoot
		} else if strings.HasPrefix(t, "..") || strings.HasPrefix(t, "/") || strings.Contains(t, "..") {
			return fmt.Errorf("import: bad manifest target %q", m.Target)
		} else {
			targetRoot = filepath.Join(vaultRoot, filepath.FromSlash(t))
		}
	}
	if !underPath(vaultRoot, targetRoot) {
		return fmt.Errorf("import: manifest target escapes vault")
	}

	var files []string
	_ = filepath.WalkDir(extractedDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(extractedDir, p)
		rel = filepath.ToSlash(rel)
		if rel == "demitable-import.json" {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	sort.Strings(files)

	batchID := "import-" + time.Now().UTC().Format("20060102T150405")
	journal := vault.NewJournal(vaultRoot)

	for _, rel := range files {
		src := filepath.Join(extractedDir, filepath.FromSlash(rel))
		dstRel := rel
		if m != nil && strings.TrimSpace(m.Target) != "" && m.Target != "." {
			dstRel = filepath.ToSlash(filepath.Join(m.Target, rel))
		}
		dst := filepath.Join(vaultRoot, filepath.FromSlash(dstRel))
		if !underPath(vaultRoot, dst) {
			return fmt.Errorf("import: path escapes vault: %s", dstRel)
		}

		if _, err := os.Stat(dst); err == nil {
			if skip {
				rep.FilesSkipped++
				continue
			}
			if !force {
				return fmt.Errorf("import: conflict: %s already exists (use --force or --skip)", dstRel)
			}
		}

		var before []byte
		if b, err := os.ReadFile(dst); err == nil {
			before = b
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
		rep.FilesImported++

		after, _ := os.ReadFile(dst)
		_, _ = journal.AppendWrite("import", "import", "import", "", dstRel, batchID, before, after, nil)
	}
	return nil
}

func pathClean(s string) string {
	return strings.TrimPrefix(filepath.ToSlash(filepath.Clean(s)), "./")
}

func underPath(root, p string) bool {
	root = filepath.Clean(root)
	p = filepath.Clean(p)
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
}

// skipPluginDirs removes detected plugin bundle directories from extracted tree.
func skipPluginDirs(extractedDir string, bundles []PluginBundle) {
	for _, b := range bundles {
		p := filepath.Join(extractedDir, filepath.FromSlash(b.Path))
		_ = os.RemoveAll(p)
	}
}

// printPluginWarning prints the ADR-0025/0027 warning.
func printPluginWarning(bundles []PluginBundle) {
	fmt.Printf("This archive contains %d plugin bundle(s):\n", len(bundles))
	for _, b := range bundles {
		caps := "none"
		if len(b.Capabilities) > 0 {
			caps = strings.Join(b.Capabilities, ", ")
		}
		fmt.Printf("  %s   %s   capabilities: %s\n", b.ID, b.Version, caps)
	}
	if hasDomInline(bundles) {
		fmt.Println()
		fmt.Println("One of these contributes components directly to the app's interface.")
		fmt.Println("A component with dom.inline runs with your own permissions: in a GM's session it can read every note in the campaign, including ones marked GM-only, and act on your behalf.")
	}
	fmt.Println()
	fmt.Println("Plugins are executable code. A plugin runs with the permissions of whoever opens it — for the GM, that means the entire vault, including notes marked GM-only. WASM plugins are sandboxed (no filesystem); network access happens only when the http.client capability is granted, and even then only to public addresses.")
	fmt.Println()
	fmt.Print("Install them? [y/N] ")
}

func hasDomInline(bundles []PluginBundle) bool {
	for _, b := range bundles {
		if b.DomInline {
			return true
		}
	}
	return false
}

func promptYes() bool {
	var line string
	fmt.Scanln(&line)
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}
