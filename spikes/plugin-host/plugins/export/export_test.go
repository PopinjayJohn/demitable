package export

import (
	"os"
	"reflect"
	"testing"

	"github.com/PopinjayJohn/demitable/spikes/plugin-host/manifest"
)

// The compiled-in tier writes the manifest as a Go literal; the runtime
// tiers write it as YAML. This test is the witness that the two bindings
// are the same manifest.
// The Go literal and the YAML are the same *manifest*, but they are not the
// same *file*: the YAML carries comments, the Go literal cannot. So the
// invariant is semantic, not byte-for-byte.
func TestGoLiteralAndYAMLAreTheSameManifest(t *testing.T) {
	canonical, err := os.ReadFile("plugin.yaml")
	if err != nil {
		t.Fatal(err)
	}
	fromYAML, err := manifest.Parse(canonical)
	if err != nil {
		t.Fatalf("parse canonical YAML: %v", err)
	}
	literal := Manifest()

	if !reflect.DeepEqual(fromYAML, literal) {
		t.Fatalf("the Go literal and the YAML disagree:\nYAML:   %+v\nliteral: %+v", fromYAML, literal)
	}
}

func TestSerialisationIsIdempotent(t *testing.T) {
	rendered, err := manifest.Marshal(Manifest())
	if err != nil {
		t.Fatal(err)
	}
	again, err := manifest.Parse(rendered)
	if err != nil {
		t.Fatalf("parse rendered YAML: %v", err)
	}
	if !reflect.DeepEqual(again, Manifest()) {
		t.Fatalf("render → parse is not the identity:\n%+v\n%+v", again, Manifest())
	}
}
