package treesitter

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestGenericExtractors verifies each spec-driven language parses a realistic
// snippet and extracts the expected top-level symbols. A wrong tree-sitter node
// type would surface here as a missing symbol.
func TestGenericExtractors(t *testing.T) {
	cases := []struct {
		lang   Language
		file   string
		source string
		expect []string // symbol names that must be found
	}{
		{
			lang: LangRust, file: "a.rs",
			source: `pub struct Config { name: String }
pub fn build_config(name: String) -> Config { Config { name } }
pub enum Mode { Fast, Slow }
pub trait Runner { fn run(&self); }`,
			expect: []string{"Config", "build_config", "Mode", "Runner"},
		},
		{
			lang: LangJava, file: "A.java",
			source: `package x;
public class Greeter {
  public String greet(String n) { return "hi " + n; }
}
interface Speaker { void speak(); }`,
			expect: []string{"Greeter", "greet", "Speaker"},
		},
		{
			lang: LangC, file: "a.c",
			source: `struct Point { int x; int y; };
int add(int a, int b) { return a + b; }`,
			expect: []string{"add"},
		},
		{
			lang: LangCPP, file: "a.cpp",
			source: `class Widget { public: void draw(); };
namespace ui { int count() { return 0; } }`,
			expect: []string{"Widget", "count"},
		},
		{
			lang: LangCSharp, file: "A.cs",
			source: `namespace App { public class Service { public int Compute() { return 1; } } }`,
			expect: []string{"Service", "Compute"},
		},
		{
			lang: LangRuby, file: "a.rb",
			source: `class Animal
  def speak
    "..."
  end
end
module Helpers
end`,
			expect: []string{"Animal", "speak", "Helpers"},
		},
		{
			lang: LangPHP, file: "a.php",
			source: `<?php
class UserController {
  public function index() { return 1; }
}
function helper() { return 2; }`,
			expect: []string{"UserController", "index", "helper"},
		},
	}

	for _, tc := range cases {
		t.Run(string(tc.lang), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tc.file)
			if err := os.WriteFile(path, []byte(tc.source), 0644); err != nil {
				t.Fatal(err)
			}
			if got := DetectLanguage(path); got != tc.lang {
				t.Fatalf("DetectLanguage(%s) = %q, want %q", tc.file, got, tc.lang)
			}
			p, err := NewParser(tc.lang)
			if err != nil {
				t.Fatalf("NewParser(%s): %v", tc.lang, err)
			}
			syms, err := p.ParseFile(context.Background(), path)
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			found := map[string]bool{}
			for _, s := range syms {
				found[s.Name] = true
			}
			for _, want := range tc.expect {
				if !found[want] {
					names := make([]string, 0, len(syms))
					for _, s := range syms {
						names = append(names, s.Name)
					}
					t.Errorf("%s: missing symbol %q; got %v", tc.lang, want, names)
				}
			}
		})
	}
}
