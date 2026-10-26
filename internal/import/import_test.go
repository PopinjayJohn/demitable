package importer

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostileArchivesRefused(t *testing.T) {
	cases := []struct {
		file string
		want string
	}{
		{"zip-slip.zip", "path traversal"},
		{"absolute-path.zip", "absolute path"},
		{"symlink.zip", "symlink"},
		{"entry-bomb.zip", "too many entries"},
		{"compression-ratio.zip", "compression ratio"},
		{"demitable-smuggled.zip", "private path"},
		{"git-smuggled.zip", "private path"},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			vaultDir := t.TempDir()
			initGit(t, vaultDir)
			abs, _ := filepath.Abs(filepath.Join("testdata", "hostile", c.file))

			_, err := Import(t.Context(), Options{VaultRoot: vaultDir, URL: "file://" + abs, AllowHTTP: true})
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), c.want)
			}
			out, _ := exec.Command("git", "-C", vaultDir, "status", "--porcelain").Output()
			if strings.TrimSpace(string(out)) != "" {
				t.Fatalf("vault mutated: %s", string(out))
			}
		})
	}
}

func TestExtractAndValidateHostile(t *testing.T) {
	cases := []string{"zip-slip.zip", "absolute-path.zip", "symlink.zip", "entry-bomb.zip", "compression-ratio.zip", "demitable-smuggled.zip", "git-smuggled.zip"}
	for _, f := range cases {
		t.Run(f, func(t *testing.T) {
			abs, _ := filepath.Abs(filepath.Join("testdata", "hostile", f))
			_, _, _, _, err := ExtractAndValidate(abs)
			if err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func initGit(t *testing.T, dir string) {
	t.Helper()
	if err := exec.Command("git", "init", "-q", dir).Run(); err != nil {
		t.Fatal(err)
	}
}
