package pack

import (
	"os"
	"testing"
)

func TestShapedPacksLintClean(t *testing.T) {
	for _, name := range []string{"testdata/shaped_5e.yaml", "testdata/shaped_blades.yaml"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		p, err := Parse(data)
		if err != nil {
			t.Fatalf("%s: parse: %v", name, err)
		}
		if issues := Lint(p); len(issues) != 0 {
			t.Fatalf("%s: lint not clean: %+v", name, issues)
		}
	}
}
