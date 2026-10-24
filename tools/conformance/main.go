package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/PopinjayJohn/demitable/internal/vault"
	"github.com/PopinjayJohn/demitable/internal/vault/frontmatter"
)

type out struct {
	Text  string       `json:"text"`
	Links []vault.Link `json:"links"`
}

func main() {
	root := "fixtures"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	var files []string
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".md") {
			files = append(files, p)
		}
		return nil
	})
	res := map[string]out{}
	for _, p := range files {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		doc, _ := frontmatter.Parse(data)
		body := string(data)
		if doc != nil && doc.HasBlock {
			body = string(doc.Body)
		}
		t, l := vault.ExtractMarkdown(body)
		res[p] = out{Text: t, Links: l}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(res)
}
