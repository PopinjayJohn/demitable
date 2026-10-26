package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PopinjayJohn/demitable/internal/import"
	"github.com/PopinjayJohn/demitable/internal/vault"
)

func runImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	vaultFlag := fs.String("vault", "", "target vault directory")
	newFlag := fs.String("new", "", "create a new vault at this path")
	force := fs.Bool("force", false, "overwrite conflicting files")
	skip := fs.Bool("skip", false, "keep existing files and skip conflicts")
	includePlugins := fs.Bool("include-plugins", false, "install plugin bundles without prompting")
	noPlugins := fs.Bool("no-plugins", false, "refuse plugin bundles entirely")
	allowHTTP := fs.Bool("allow-http-import", false, "allow plain http imports")
	// Allow flags before or after the positional URL.
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flags = append(flags, args[i])
			if !strings.Contains(args[i], "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags = append(flags, args[i+1])
				i++
			}
		} else {
			positional = append(positional, args[i])
		}
	}
	if err := fs.Parse(append(flags, positional...)); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("import: a url is required")
	}
	urlStr := fs.Arg(0)

	vaultRoot := ""
	if *newFlag != "" {
		vaultRoot = *newFlag
		if err := os.MkdirAll(vaultRoot, 0o755); err != nil {
			return fmt.Errorf("import: mkdir %s: %w", vaultRoot, err)
		}
	} else if *vaultFlag != "" {
		vaultRoot = *vaultFlag
	} else {
		vaultRoot = "."
	}
	vaultRoot, _ = filepath.Abs(vaultRoot)

	opts := importer.Options{
		VaultRoot:      vaultRoot,
		URL:            urlStr,
		Force:          *force,
		Skip:           *skip,
		IncludePlugins: *includePlugins,
		NoPlugins:      *noPlugins,
		AllowHTTP:      *allowHTTP,
		Interactive:    isTerminal(os.Stdin),
	}
	rep, err := importer.Import(context.Background(), opts)
	printImportReport(rep)
	if err != nil {
		return err
	}
	// Reload vault if present (best effort).
	if v, err := vault.Open(vaultRoot); err == nil {
		_ = v
	}
	return nil
}

func printImportReport(rep importer.Report) {
	fmt.Printf("Import report:\n")
	fmt.Printf("  url: %s\n", rep.URL)
	fmt.Printf("  files imported: %d\n", rep.FilesImported)
	fmt.Printf("  files skipped: %d\n", rep.FilesSkipped)
	if len(rep.Rejected) > 0 {
		fmt.Printf("  rejected: %s\n", strings.Join(rep.Rejected, ", "))
	}
	if len(rep.PluginBundles) > 0 {
		fmt.Printf("  plugin bundles: %d\n", len(rep.PluginBundles))
	}
	if rep.PluginsInstalled > 0 {
		fmt.Printf("  plugins installed: %d\n", rep.PluginsInstalled)
	}
	if rep.PluginsSkipped > 0 {
		fmt.Printf("  plugins skipped: %d\n", rep.PluginsSkipped)
	}
	if rep.Message != "" {
		fmt.Println(rep.Message)
	}
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
