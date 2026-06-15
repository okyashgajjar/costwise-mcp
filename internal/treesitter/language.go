package treesitter

import (
	"path/filepath"
	"strings"
)

type Language string

const (
	LangGo         Language = "go"
	LangPython     Language = "python"
	LangJavaScript Language = "javascript"
	LangTypeScript Language = "typescript"
	// Spec-driven languages (see langspec.go).
	LangRust   Language = "rust"
	LangJava   Language = "java"
	LangC      Language = "c"
	LangCPP    Language = "cpp"
	LangCSharp Language = "csharp"
	LangRuby   Language = "ruby"
	LangPHP    Language = "php"
)

func DetectLanguage(filePath string) Language {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return LangGo
	case ".py":
		return LangPython
	case ".js", ".jsx", ".mjs":
		return LangJavaScript
	case ".ts", ".tsx":
		return LangTypeScript
	}
	// Spec-driven languages register their extensions in langspec.go's init().
	for lang, spec := range langRegistry {
		for _, e := range spec.Extensions {
			if e == ext {
				return lang
			}
		}
	}
	return ""
}

func IsSupported(path string) bool {
	return DetectLanguage(path) != ""
}

var SupportedLanguages = []Language{
	LangGo, LangPython, LangJavaScript, LangTypeScript,
	LangRust, LangJava, LangC, LangCPP, LangCSharp, LangRuby, LangPHP,
}
