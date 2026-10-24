package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/PopinjayJohn/demitable/internal/vault/gen"
)

func main() {
	out := flag.String("out", "", "output dir")
	n := flag.Int("n", 5000, "number of notes")
	keys := flag.Int("keys", 40, "unknown keys")
	body := flag.Int("body", 2048, "body bytes")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "-out required")
		os.Exit(2)
	}
	if err := gen.Generate(*out, *n, *keys, *body); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("generated", *n, "notes in", *out)
}
