package main

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// §T86/§V43: import isolation — core (internal/tui) must not import any
// features/* package, and no features/* package may import internal/tui. The
// only allowed coupling is via the public feature/lang APIs. Scans non-test Go
// files for forbidden import paths.
func TestImportIsolation(t *testing.T) {
	const (
		corePkg = "github.com/jxsl13/teetui/internal/tui"
		featPkg = "github.com/jxsl13/teetui/features/"
	)

	check := func(root string, forbidden func(imp string) bool) {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			f, perr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if perr != nil {
				t.Errorf("parse %s: %v", path, perr)
				return nil
			}
			for _, imp := range f.Imports {
				p := strings.Trim(imp.Path.Value, `"`)
				if forbidden(p) {
					t.Errorf("%s imports forbidden %q", path, p)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// core ⊥ features/*
	check("internal/tui", func(imp string) bool { return strings.HasPrefix(imp, featPkg) })
	// features/* ⊥ core
	check("features", func(imp string) bool { return imp == corePkg })
}
