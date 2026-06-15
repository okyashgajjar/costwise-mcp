package treesitter

import (
	"context"
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	ts "github.com/smacker/go-tree-sitter/typescript/typescript"
)

type Parser struct {
	parser   *sitter.Parser
	language Language
	langObj  *sitter.Language
}

func NewParser(lang Language) (*Parser, error) {
	var langObj *sitter.Language
	switch lang {
	case LangGo:
		langObj = golang.GetLanguage()
	case LangPython:
		langObj = python.GetLanguage()
	case LangJavaScript:
		langObj = javascript.GetLanguage()
	case LangTypeScript:
		langObj = ts.GetLanguage()
	default:
		if spec, ok := langRegistry[lang]; ok {
			langObj = spec.Grammar
		} else {
			return nil, fmt.Errorf("unsupported language: %s", lang)
		}
	}

	p := sitter.NewParser()
	p.SetLanguage(langObj)

	return &Parser{
		parser:   p,
		language: lang,
		langObj:  langObj,
	}, nil
}

func (p *Parser) ParseFile(ctx context.Context, filePath string) ([]Symbol, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	tree, err := p.parser.ParseCtx(ctx, nil, data)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	root := tree.RootNode()
	var symbols []Symbol

	switch p.language {
	case LangGo:
		symbols = extractGoSymbols(root, filePath, data)
	case LangPython:
		symbols = extractPythonSymbols(root, filePath, data)
	case LangJavaScript:
		symbols = extractJavaScriptSymbols(root, filePath, data)
	case LangTypeScript:
		symbols = extractTypeScriptSymbols(root, filePath, data)
	default:
		if spec, ok := langRegistry[p.language]; ok {
			symbols = extractGenericSymbols(root, filePath, data, spec)
		}
	}

	return symbols, nil
}

func (p *Parser) ParseReferences(ctx context.Context, filePath string) ([]Reference, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	tree, err := p.parser.ParseCtx(ctx, nil, data)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	root := tree.RootNode()
	refs := extractReferences(root, filePath, data, p.language)
	return refs, nil
}

func (p *Parser) ParseCalls(ctx context.Context, filePath string) ([]CallEdge, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	tree, err := p.parser.ParseCtx(ctx, nil, data)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	root := tree.RootNode()
	calls := extractCalls(root, filePath, data, p.language)
	return calls, nil
}

func extractSourceLine(data []byte, start, end sitter.Point) string {
	lines := strings.Split(string(data), "\n")
	if int(start.Row) >= len(lines) {
		return ""
	}
	if int(end.Row) >= len(lines) {
		end.Row = uint32(len(lines) - 1)
	}
	out := strings.Join(lines[start.Row:end.Row+1], "\n")
	if len(out) > 200 {
		out = out[:200] + "..."
	}
	return out
}

func extractNameFromChild(n *sitter.Node, data []byte) string {
	for i := 0; i < int(n.ChildCount()); i++ {
		c := n.Child(i)
		if c.Type() == "identifier" || c.Type() == "field_identifier" ||
			c.Type() == "type_identifier" || c.Type() == "property_identifier" {
			return c.Content(data)
		}
	}
	content := n.Content(data)
	parts := strings.Fields(content)
	if len(parts) > 0 {
		return parts[0]
	}
	return "unknown"
}
