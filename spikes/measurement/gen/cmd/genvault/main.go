// Command genvault writes a synthetic vault for the baseline measurements.
//
// The phase-1 plan's acceptance criterion says "a generated vault of 5,000
// notes, each with 40 unknown frontmatter keys and a 2 KB body". The plan
// says the generator must be committed, so it is written once, in the gen
// package, and driven from here.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/PopinjayJohn/demitable/spikes/measurement/gen"
)

func main() {
	root := flag.String("root", "", "where to write the vault")
	notes := flag.Int("notes", 5000, "how many notes")
	keys := flag.Int("frontmatter-keys", 40, "unknown frontmatter keys per note")
	body := flag.Int("body-bytes", 2048, "target body size")
	dirs := flag.Int("dirs", 50, "how many directories")
	seed := flag.Int("seed", 1, "content seed")
	flag.Parse()
	if *root == "" {
		fmt.Fprintln(os.Stderr, "-root is required")
		os.Exit(2)
	}
	start := time.Now()
	n, err := gen.Write(gen.Options{
		Root: *root, Notes: *notes, FrontmatterKeys: *keys,
		BodyBytes: *body, Dirs: *dirs, Seed: *seed,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %d notes (%d bytes) to %s in %v\n", *notes, n, *root, time.Since(start).Round(time.Millisecond))
}
