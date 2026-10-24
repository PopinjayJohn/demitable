// Package gen generates a synthetic vault for performance testing.
package gen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Generate creates n notes in root, each with k unknown frontmatter keys and
// a body of about bodyBytes bytes.
func Generate(root string, n, keys, bodyBytes int) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	for i := 0; i < n; i++ {
		var b strings.Builder
		b.WriteString("---\n")
		for k := 0; k < keys; k++ {
			fmt.Fprintf(&b, "unknown_key_%d: value %d\n", k, i*k)
		}
		b.WriteString("---\n\n")
		// body
		for len(b.String()) < bodyBytes+100 {
			b.WriteString("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ")
		}
		b.WriteString("\n")
		name := fmt.Sprintf("note-%05d.md", i)
		dir := filepath.Join(root, fmt.Sprintf("shard-%02d", i%10))
		os.MkdirAll(dir, 0o755)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(b.String()), 0o644); err != nil {
			return err
		}
	}
	return nil
}
