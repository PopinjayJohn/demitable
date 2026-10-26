package importer

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/PopinjayJohn/demitable/internal/plugin/manifest"
)

// Archive caps.
const (
	MaxEntries           = 5000
	MaxTotalUncompressed = 2 * 1024 * 1024 * 1024
	MaxCompressionRatio  = 50
)

// ExtractAndValidate extracts zipPath to a temp dir, validates the whole
// tree, and returns the temp dir, parsed manifest, detected plugin bundles,
// and rejected entries. On error the temp dir is removed.
func ExtractAndValidate(zipPath string) (string, *Manifest, []PluginBundle, []string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("import: open zip: %w", err)
	}
	defer r.Close()

	if len(r.File) > MaxEntries {
		return "", nil, nil, nil, fmt.Errorf("import: too many entries: %d > %d", len(r.File), MaxEntries)
	}

	var totalUncompressed uint64
	var totalCompressed uint64
	for _, f := range r.File {
		totalUncompressed += f.UncompressedSize64
		totalCompressed += f.CompressedSize64
	}
	if totalUncompressed > MaxTotalUncompressed {
		return "", nil, nil, nil, fmt.Errorf("import: total uncompressed size %d exceeds %d", totalUncompressed, MaxTotalUncompressed)
	}
	if totalCompressed > 0 {
		ratio := totalUncompressed / totalCompressed
		if totalUncompressed%totalCompressed != 0 {
			ratio++
		}
		if ratio > MaxCompressionRatio {
			return "", nil, nil, nil, fmt.Errorf("import: compression ratio too high: %d > %d", ratio, MaxCompressionRatio)
		}
	}

	// Validate every entry path first.
	var rejected []string
	for _, f := range r.File {
		if f.Name == "" {
			continue
		}
		rel, err := safeEntryPath(f.Name)
		if err != nil {
			return "", nil, nil, nil, err
		}
		if rel == "" {
			continue
		}
		// private paths
		segs := strings.Split(rel, "/")
		for _, seg := range segs {
			if seg == ".demitable" || seg == ".git" {
				rejected = append(rejected, rel)
				return "", nil, nil, rejected, fmt.Errorf("import: private path not importable: %s", rel)
			}
		}
		// symlinks
		mode := f.Mode()
		if mode&os.ModeSymlink != 0 {
			return "", nil, nil, rejected, fmt.Errorf("import: symlink entry refused: %s", rel)
		}
		// also detect via unix mode
		if mode&0o170000 == 0o120777 {
			return "", nil, nil, rejected, fmt.Errorf("import: symlink entry refused: %s", rel)
		}
	}

	tempDir, err := os.MkdirTemp("", "demitable-extract-*")
	if err != nil {
		return "", nil, nil, nil, err
	}

	// Extract.
	for _, f := range r.File {
		rel, err := safeEntryPath(f.Name)
		if err != nil {
			os.RemoveAll(tempDir)
			return "", nil, nil, nil, err
		}
		if rel == "" {
			continue
		}
		target := filepath.Join(tempDir, filepath.FromSlash(rel))
		if !strings.HasPrefix(target+string(filepath.Separator), tempDir+string(filepath.Separator)) {
			os.RemoveAll(tempDir)
			return "", nil, nil, nil, fmt.Errorf("import: path traversal: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				os.RemoveAll(tempDir)
				return "", nil, nil, nil, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			os.RemoveAll(tempDir)
			return "", nil, nil, nil, err
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			os.RemoveAll(tempDir)
			return "", nil, nil, nil, err
		}
		rc, err := f.Open()
		if err != nil {
			out.Close()
			os.RemoveAll(tempDir)
			return "", nil, nil, nil, err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			os.RemoveAll(tempDir)
			return "", nil, nil, nil, err
		}
		out.Close()
		rc.Close()
	}

	// Parse manifest if present.
	var m *Manifest
	manifestPath := filepath.Join(tempDir, "demitable-import.json")
	if data, err := os.ReadFile(manifestPath); err == nil {
		var mm Manifest
		if err := json.Unmarshal(data, &mm); err != nil {
			os.RemoveAll(tempDir)
			return "", nil, nil, nil, fmt.Errorf("import: bad manifest: %w", err)
		}
		m = &mm
	}

	// Checksum verification.
	if m != nil {
		want := m.Checksum
		if want == "" {
			want = m.SHA256
		}
		if want != "" {
			want = strings.TrimPrefix(want, "sha256:")
			f, err := os.Open(zipPath)
			if err != nil {
				os.RemoveAll(tempDir)
				return "", nil, nil, nil, err
			}
			h := sha256.New()
			if _, err := io.Copy(h, f); err != nil {
				f.Close()
				os.RemoveAll(tempDir)
				return "", nil, nil, nil, err
			}
			f.Close()
			got := fmt.Sprintf("%x", h.Sum(nil))
			if got != want {
				os.RemoveAll(tempDir)
				return "", nil, nil, nil, fmt.Errorf("import: checksum mismatch: got %s want %s", got, want)
			}
		}
	}

	// Requires check (packs).
	if m != nil {
		for _, id := range m.Requires.Packs {
			// validated later in Import; but fail fast here too? keep here
			_ = id
		}
	}

	bundles := detectPluginBundles(tempDir)

	return tempDir, m, bundles, rejected, nil
}

func safeEntryPath(name string) (string, error) {
	// backslash normalization is not done: zip uses forward slashes.
	// absolute
	if strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("import: absolute path refused: %s", name)
	}
	if len(name) >= 2 && name[1] == ':' {
		return "", fmt.Errorf("import: absolute path refused: %s", name)
	}
	// traversal in original segments
	for _, seg := range strings.Split(name, "/") {
		if seg == ".." {
			return "", fmt.Errorf("import: path traversal refused: %s", name)
		}
	}
	// clean
	cleaned := path.Clean("/" + name)
	rel := strings.TrimPrefix(cleaned, "/")
	if rel == "." {
		return "", nil
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == ".." {
			return "", fmt.Errorf("import: path traversal refused: %s", name)
		}
	}
	// ensure cleaned path doesn't escape: compare
	if rel == "" {
		return "", nil
	}
	return rel, nil
}

func detectPluginBundles(root string) []PluginBundle {
	var bundles []PluginBundle
	seen := map[string]bool{}
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		base := filepath.Base(p)
		if base != "plugin.yaml" && base != "plugin.yml" && base != "manifest.yaml" && base != "manifest.yml" {
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
		manifest.Validate(m)
		dir := filepath.Dir(p)
		rel, _ := filepath.Rel(root, dir)
		if seen[rel] {
			return nil
		}
		seen[rel] = true
		caps := []string{}
		domInline := false
		for _, c := range m.Capabilities {
			caps = append(caps, c.ID)
			if c.ID == "dom.inline" {
				domInline = true
			}
		}
		bundles = append(bundles, PluginBundle{
			Path:         rel,
			ID:           m.ID,
			Name:         m.Name,
			Version:      m.Version,
			Capabilities: caps,
			DomInline:    domInline,
		})
		return nil
	})
	return bundles
}
