package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAPIGeneratedFromHandlers(t *testing.T) {
	got, _ := json.MarshalIndent(generateOpenAPI(), "", "  ")
	got = append(got, '\n')
	for _, rel := range []string{"../../openapi.json", "openapi.json"} {
		want, err := os.ReadFile(filepath.Join(".", rel))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(want)) {
			t.Fatalf("OpenAPI drift: generated document differs from %s", rel)
		}
	}
}
