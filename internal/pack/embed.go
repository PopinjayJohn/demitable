package pack

import (
	"embed"
	"io/fs"
)

//go:embed all:packs
var embeddedPacks embed.FS

// Embedded is the embedded filesystem containing the shipped packs.
var Embedded embed.FS = embeddedPacks

// PacksFS returns the embedded packs filesystem rooted at "packs".
func PacksFS() (fs.FS, error) {
	return fs.Sub(embeddedPacks, "packs")
}

// Example loads the shipped example pack.
func Example() (*Pack, error) {
	sub, err := PacksFS()
	if err != nil {
		return nil, err
	}
	return LoadFS(sub, "example.yaml")
}

// ExamplePack is an alias for Example.
func ExamplePack() (*Pack, error) { return Example() }
