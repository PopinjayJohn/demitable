// Command srdcheck verifies the corpus's provenance.
//
// # Why it verifies a hash and not a URL
//
// The corpus is a transient build input: it is fetched by hand, it is
// gitignored, and it is deleted after a build. The licence boundary of the
// published vault rests on the SRD's own CC-BY-4.0 terms, which are
// authoritative and independently checkable.
//
// What srdcheck adds is the assurance that the corpus in hand is the corpus
// that was reviewed. It does that by pinning the corpus's **content**, not
// its source:
//
//   - the tree hash below is a hash over every file's path and bytes, so a
//     changed, added or removed file changes the hash;
//   - the version and its release date are read out of the corpus's own
//     changelog, so they are self-declared by the artifact rather than
//     asserted by us.
//
// Pinning a commit hash against a repository would be the usual thing to do,
// and it is deliberately not done here: the upstream's canonical repository
// is no longer publicly resolvable, so a commit hash could only ever have
// been pinned against an unofficial mirror. A hash of a mirror is not
// evidence of anything. A hash of the artifact is.
//
// See docs/corpus-provenance.md for the record and the method.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	var (
		verify = flag.Bool("verify", false, "verify the corpus against the recorded provenance")
		record = flag.Bool("record", false, "recompute the provenance and write it out")
		root   = flag.String("root", "fixtures", "the corpus directory")
		out    = flag.String("out", "tools/srdcheck/provenance.json", "the provenance file")
	)
	flag.Parse()

	switch {
	case *record:
		if err := doRecord(*root, *out); err != nil {
			fmt.Fprintln(os.Stderr, "srdcheck:", err)
			os.Exit(1)
		}
	case *verify:
		if err := doVerify(*root, *out); err != nil {
			fmt.Fprintln(os.Stderr, "srdcheck:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: srdcheck -verify | -record [-root dir] [-out file]")
		os.Exit(2)
	}
}

// Provenance is the recorded provenance of one corpus.
type Provenance struct {
	// CorpusVersion is the upstream's own version string.
	CorpusVersion string `json:"corpusVersion"`
	// CorpusReleased is the date the corpus's changelog gives for that
	// version.
	CorpusReleased string `json:"corpusReleased"`
	// TreeSHA256 is the hash of the corpus tree.
	TreeSHA256 string `json:"treeSha256"`
	// MarkdownSHA256 is the hash of the markdown tree alone.
	MarkdownSHA256 string `json:"markdownSha256"`
	// DataSHA256 is the hash of the JSON data tree alone.
	DataSHA256 string `json:"dataSha256"`
	// FileCount and MarkdownCount describe what was hashed.
	FileCount      int    `json:"fileCount"`
	MarkdownCount  int    `json:"markdownCount"`
	ContentLicence string `json:"contentLicence"`
	SPDXID         string `json:"spdxId"`
	// Acquisition is how the corpus is obtained.
	Acquisition string `json:"acquisition"`
}

func doRecord(root, out string) error {
	p, err := compute(root)
	if err != nil {
		return err
	}
	p.ContentLicence = "CC-BY-4.0"
	p.SPDXID = "CC-BY-4.0"
	p.Acquisition = "manual"

	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(out, b, 0o644)
}

func doVerify(root, out string) error {
	b, err := os.ReadFile(out)
	if err != nil {
		return fmt.Errorf("read the provenance file: %w", err)
	}
	var want Provenance
	if err := json.Unmarshal(b, &want); err != nil {
		return fmt.Errorf("parse the provenance file: %w", err)
	}
	got, err := compute(root)
	if err != nil {
		return err
	}
	var bad []string
	check := func(name, want, got string) {
		if want != got {
			bad = append(bad, fmt.Sprintf("%s: recorded %s, corpus %s", name, want, got))
		}
	}
	check("treeSha256", want.TreeSHA256, got.TreeSHA256)
	check("markdownSha256", want.MarkdownSHA256, got.MarkdownSHA256)
	check("dataSha256", want.DataSHA256, got.DataSHA256)
	check("corpusVersion", want.CorpusVersion, got.CorpusVersion)
	check("fileCount", fmt.Sprint(want.FileCount), fmt.Sprint(got.FileCount))
	if len(bad) > 0 {
		return fmt.Errorf("the corpus does not match its recorded provenance:\n  %s", strings.Join(bad, "\n  "))
	}
	fmt.Printf("corpus verified: version %s (%s), %d files, tree %s\n",
		got.CorpusVersion, got.CorpusReleased, got.FileCount, got.TreeSHA256)
	return nil
}

// compute hashes the corpus tree.
//
// The tree hash covers every file's path and its bytes, sorted by path, so
// that it does not depend on the filesystem's ordering or on the walk's
// timing. A file that is added, removed or changed changes the hash.
func compute(root string) (*Provenance, error) {
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		paths = append(paths, p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)

	tree := sha256.New()
	markdown := sha256.New()
	data := sha256.New()
	nMarkdown := 0
	for _, p := range paths {
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(b)
		fmt.Fprintf(tree, "%s  %s\n", rel, hex.EncodeToString(sum[:]))
		if strings.HasSuffix(rel, ".md") {
			nMarkdown++
			fmt.Fprintf(markdown, "%s  %s\n", rel, hex.EncodeToString(sum[:]))
		} else {
			fmt.Fprintf(data, "%s  %s\n", rel, hex.EncodeToString(sum[:]))
		}
	}

	p := &Provenance{
		TreeSHA256:     hex.EncodeToString(tree.Sum(nil)),
		MarkdownSHA256: hex.EncodeToString(markdown.Sum(nil)),
		DataSHA256:     hex.EncodeToString(data.Sum(nil)),
		FileCount:      len(paths),
		MarkdownCount:  nMarkdown,
	}
	ver, released, err := readVersion(root)
	if err != nil {
		return nil, err
	}
	p.CorpusVersion = ver
	p.CorpusReleased = released
	return p, nil
}

// readVersion reads the corpus's own version out of its changelog.
//
// The changelog is the artifact's own record of itself, so the version is
// self-declared rather than asserted by us, and it can be checked against a
// release announcement without trusting this repository.
func readVersion(root string) (string, string, error) {
	b, err := os.ReadFile(filepath.Join(root, "data", "changelog.json"))
	if err != nil {
		return "", "", fmt.Errorf("read the corpus changelog: %w", err)
	}
	var entries []struct {
		Ver  string `json:"ver"`
		Date string `json:"date"`
	}
	if err := json.Unmarshal(b, &entries); err != nil {
		return "", "", fmt.Errorf("parse the corpus changelog: %w", err)
	}
	if len(entries) == 0 {
		return "", "", fmt.Errorf("the corpus changelog is empty")
	}
	// The changelog runs oldest first, so the corpus's own version is the
	// entry with the most recent date.
	best := entries[0]
	for _, e := range entries[1:] {
		if e.Date > best.Date {
			best = e
		}
	}
	return best.Ver, best.Date, nil
}

var _ = io.Discard
