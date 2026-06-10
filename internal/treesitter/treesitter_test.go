package treesitter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestParseGoFile(t *testing.T) {
	parser, err := NewParser(LangGo)
	if err != nil {
		t.Fatal(err)
	}

	code := []byte(`
package main

import "fmt"

type User struct {
	Name string
}

func main() {
	fmt.Println("hello")
}

func (u *User) GetName() string {
	return u.Name
}
`)
	tree, err := parser.parser.ParseCtx(context.Background(), nil, code)
	if err != nil {
		t.Fatal(err)
	}

	symbols := extractGoSymbols(tree.RootNode(), "test.go", code)
	fmt.Printf("Found %d symbols:\n", len(symbols))
	for _, s := range symbols {
		fmt.Printf("  %s: %s (%s:%d-%d)\n", s.Kind, s.Name, s.File, s.StartLine, s.EndLine)
	}

	if len(symbols) == 0 {
		t.Fatal("expected at least 1 symbol")
	}
}

func TestParsePythonFile(t *testing.T) {
	parser, err := NewParser(LangPython)
	if err != nil {
		t.Fatal(err)
	}

	code := []byte(`
import os
from typing import Optional

class UserManager:
    def __init__(self):
        pass
    
    def get_user(self, id: int):
        return None

def helper_function():
    pass
`)
	tree, err := parser.parser.ParseCtx(context.Background(), nil, code)
	if err != nil {
		t.Fatal(err)
	}

	symbols := extractPythonSymbols(tree.RootNode(), "test.py", code)
	fmt.Printf("Found %d symbols:\n", len(symbols))
	for _, s := range symbols {
		fmt.Printf("  %s: %s (%s:%d-%d)\n", s.Kind, s.Name, s.File, s.StartLine, s.EndLine)
	}
}

func TestSymbolDB(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := NewSymbolDB(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	symbols := []Symbol{
		{Name: "main", Kind: SymbolFunction, Language: "go", File: "main.go", StartLine: 1, EndLine: 5, Signature: "func main()"},
		{Name: "User", Kind: SymbolStruct, Language: "go", File: "user.go", StartLine: 1, EndLine: 4, Signature: "type User struct"},
		{Name: "GetName", Kind: SymbolMethod, Language: "go", File: "user.go", StartLine: 6, EndLine: 9, Signature: "func (u *User) GetName()"},
	}

	if err := db.StoreSymbols(symbols); err != nil {
		t.Fatal(err)
	}

	count := db.GetSymbolCount()
	if count != 3 {
		t.Fatalf("expected 3 symbols, got %d", count)
	}

	matches, err := db.Search("User", 10)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("Search 'User': %d results\n", len(matches))
	for _, m := range matches {
		fmt.Printf("  %s (score=%.2f, kind=%s)\n", m.Symbol.Name, m.Score, m.Symbol.Kind)
	}
}

func TestParseRepoFiles(t *testing.T) {
	cwd, _ := os.Getwd()
	repoRoot := filepath.Join(cwd, "..", "..")

	goFiles := 0
	parsed := 0
	_ = filepath.Walk(repoRoot, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		if !IsSupported(path) {
			return nil
		}
		goFiles++
		lang := DetectLanguage(path)
		parser, err := NewParser(lang)
		if err != nil {
			return nil
		}
		syms, err := parser.ParseFile(context.Background(), path)
		if err != nil {
			return nil
		}
		if len(syms) > 0 {
			parsed++
			if goFiles <= 3 {
				fmt.Printf("  %s: %d symbols\n", filepath.Base(path), len(syms))
			}
		}
		return nil
	})
	fmt.Printf("Repo scan: %d supported files, %d with symbols\n", goFiles, parsed)
}
