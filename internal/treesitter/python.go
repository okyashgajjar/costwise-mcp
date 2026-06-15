package treesitter

import (
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
)

func extractGoSymbols(root *sitter.Node, filePath string, data []byte) []Symbol {
	var symbols []Symbol

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		switch n.Type() {
		case "function_declaration":
			name := extractNameFromChild(n, data)
			sig := extractSourceLine(data, n.StartPoint(), n.EndPoint())
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      SymbolFunction,
				Language:  "go",
				File:      filePath,
				StartLine: int(n.StartPoint().Row) + 1,
				EndLine:   int(n.EndPoint().Row) + 1,
				Signature: truncateSig(sig),
			})

		case "method_declaration":
			name := extractNameFromChild(n, data)
			sig := extractSourceLine(data, n.StartPoint(), n.EndPoint())
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      SymbolMethod,
				Language:  "go",
				File:      filePath,
				StartLine: int(n.StartPoint().Row) + 1,
				EndLine:   int(n.EndPoint().Row) + 1,
				Signature: truncateSig(sig),
			})

		case "type_declaration":
			for i := 0; i < int(n.ChildCount()); i++ {
				c := n.Child(i)
				if c.Type() == "type_spec" {
					typeName := extractNameFromChild(c, data)
					kind := SymbolClass
					for j := 0; j < int(c.ChildCount()); j++ {
						inner := c.Child(j)
						if inner.Type() == "struct_type" {
							kind = SymbolStruct
							break
						}
						if inner.Type() == "interface_type" {
							kind = SymbolInterface
							break
						}
					}
					sig := extractSourceLine(data, n.StartPoint(), n.EndPoint())
					symbols = append(symbols, Symbol{
						Name:      typeName,
						Kind:      kind,
						Language:  "go",
						File:      filePath,
						StartLine: int(n.StartPoint().Row) + 1,
						EndLine:   int(n.EndPoint().Row) + 1,
						Signature: truncateSig(sig),
					})
				}
			}

		case "import_declaration":
			content := n.Content(data)
			symbols = append(symbols, Symbol{
				Name:      fmt.Sprintf("import %s", truncateSig(content)),
				Kind:      SymbolImport,
				Language:  "go",
				File:      filePath,
				StartLine: int(n.StartPoint().Row) + 1,
				EndLine:   int(n.EndPoint().Row) + 1,
				Signature: truncateSig(content),
			})

		case "const_declaration", "var_declaration":
			// Only package-level declarations — indexing function-local consts
			// and vars would flood the symbol table with ambiguous names.
			if n.Parent() == nil || n.Parent().Type() != "source_file" {
				break
			}
			kind := SymbolConstant
			if n.Type() == "var_declaration" {
				kind = SymbolVariable
			}
			sig := extractSourceLine(data, n.StartPoint(), n.EndPoint())
			for i := 0; i < int(n.ChildCount()); i++ {
				spec := n.Child(i)
				if spec.Type() != "const_spec" && spec.Type() != "var_spec" {
					continue
				}
				for j := 0; j < int(spec.ChildCount()); j++ {
					id := spec.Child(j)
					if id.Type() != "identifier" {
						continue
					}
					symbols = append(symbols, Symbol{
						Name:      id.Content(data),
						Kind:      kind,
						Language:  "go",
						File:      filePath,
						StartLine: int(n.StartPoint().Row) + 1,
						EndLine:   int(n.EndPoint().Row) + 1,
						Signature: truncateSig(sig),
					})
				}
			}
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(root)
	return symbols
}

func extractPythonSymbols(root *sitter.Node, filePath string, data []byte) []Symbol {
	var symbols []Symbol

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		switch n.Type() {
		case "function_definition":
			name := extractNameFromChild(n, data)
			sig := extractSourceLine(data, n.StartPoint(), n.EndPoint())
			kind := SymbolFunction
			for i := 0; i < int(n.ChildCount()); i++ {
				if n.Child(i).Type() == "parameters" && hasSelfParam(n.Child(i), data) {
					kind = SymbolMethod
					break
				}
			}
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      kind,
				Language:  "python",
				File:      filePath,
				StartLine: int(n.StartPoint().Row) + 1,
				EndLine:   int(n.EndPoint().Row) + 1,
				Signature: truncateSig(sig),
			})

		case "class_definition":
			name := extractNameFromChild(n, data)
			sig := extractSourceLine(data, n.StartPoint(), n.EndPoint())
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      SymbolClass,
				Language:  "python",
				File:      filePath,
				StartLine: int(n.StartPoint().Row) + 1,
				EndLine:   int(n.EndPoint().Row) + 1,
				Signature: truncateSig(sig),
			})

		case "import_statement":
			content := n.Content(data)
			symbols = append(symbols, Symbol{
				Name:      truncateSig(content),
				Kind:      SymbolImport,
				Language:  "python",
				File:      filePath,
				StartLine: int(n.StartPoint().Row) + 1,
				EndLine:   int(n.EndPoint().Row) + 1,
				Signature: truncateSig(content),
			})

		case "import_from_statement":
			content := n.Content(data)
			symbols = append(symbols, Symbol{
				Name:      truncateSig(content),
				Kind:      SymbolImport,
				Language:  "python",
				File:      filePath,
				StartLine: int(n.StartPoint().Row) + 1,
				EndLine:   int(n.EndPoint().Row) + 1,
				Signature: truncateSig(content),
			})
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(root)
	return symbols
}

func extractJavaScriptSymbols(root *sitter.Node, filePath string, data []byte) []Symbol {
	return extractCommonJSTS(root, filePath, data, "javascript")
}

func extractTypeScriptSymbols(root *sitter.Node, filePath string, data []byte) []Symbol {
	return extractCommonJSTS(root, filePath, data, "typescript")
}

func extractCommonJSTS(root *sitter.Node, filePath string, data []byte, lang string) []Symbol {
	var symbols []Symbol

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		switch n.Type() {
		case "function_declaration":
			name := extractNameFromChild(n, data)
			sig := extractSourceLine(data, n.StartPoint(), n.EndPoint())
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      SymbolFunction,
				Language:  lang,
				File:      filePath,
				StartLine: int(n.StartPoint().Row) + 1,
				EndLine:   int(n.EndPoint().Row) + 1,
				Signature: truncateSig(sig),
			})

		case "method_definition":
			name := extractNameFromChild(n, data)
			sig := extractSourceLine(data, n.StartPoint(), n.EndPoint())
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      SymbolMethod,
				Language:  lang,
				File:      filePath,
				StartLine: int(n.StartPoint().Row) + 1,
				EndLine:   int(n.EndPoint().Row) + 1,
				Signature: truncateSig(sig),
			})

		case "class_declaration", "abstract_class_declaration":
			name := extractNameFromChild(n, data)
			sig := extractSourceLine(data, n.StartPoint(), n.EndPoint())
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      SymbolClass,
				Language:  lang,
				File:      filePath,
				StartLine: int(n.StartPoint().Row) + 1,
				EndLine:   int(n.EndPoint().Row) + 1,
				Signature: truncateSig(sig),
			})

		case "type_alias_declaration":
			if lang == "typescript" {
				name := extractNameFromChild(n, data)
				sig := extractSourceLine(data, n.StartPoint(), n.EndPoint())
				symbols = append(symbols, Symbol{
					Name:      name,
					Kind:      SymbolType,
					Language:  lang,
					File:      filePath,
					StartLine: int(n.StartPoint().Row) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					Signature: truncateSig(sig),
				})
			}

		case "enum_declaration":
			if lang == "typescript" {
				name := extractNameFromChild(n, data)
				sig := extractSourceLine(data, n.StartPoint(), n.EndPoint())
				symbols = append(symbols, Symbol{
					Name:      name,
					Kind:      SymbolEnum,
					Language:  lang,
					File:      filePath,
					StartLine: int(n.StartPoint().Row) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					Signature: truncateSig(sig),
				})
			}

		case "interface_declaration":
			if lang == "typescript" {
				name := extractNameFromChild(n, data)
				sig := extractSourceLine(data, n.StartPoint(), n.EndPoint())
				symbols = append(symbols, Symbol{
					Name:      name,
					Kind:      SymbolInterface,
					Language:  lang,
					File:      filePath,
					StartLine: int(n.StartPoint().Row) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					Signature: truncateSig(sig),
				})
			}

		case "lexical_declaration", "variable_declaration":
			for i := 0; i < int(n.ChildCount()); i++ {
				c := n.Child(i)
				if c.Type() == "variable_declarator" {
					name := extractNameFromChild(c, data)
					if c.Child(0) != nil && c.Child(0).Type() == "identifier" {
						full := extractSourceLine(data, n.StartPoint(), n.EndPoint())
						kind := SymbolFunction
						if lang == "typescript" {
							for j := 0; j < int(c.ChildCount()); j++ {
								t := c.Child(j).Type()
								if t == "type_annotation" {
									ann := c.Child(j).Content(data)
									if stringsContains(ann, "Function") || stringsContains(ann, "=>") {
										kind = SymbolFunction
									}
								}
							}
						}
						symbols = append(symbols, Symbol{
							Name:      name,
							Kind:      kind,
							Language:  lang,
							File:      filePath,
							StartLine: int(n.StartPoint().Row) + 1,
							EndLine:   int(n.EndPoint().Row) + 1,
							Signature: truncateSig(full),
						})
					}
				}
			}

		case "export_statement":
			for i := 0; i < int(n.ChildCount()); i++ {
				c := n.Child(i)
				name := ""
				kind := SymbolExport
				switch c.Type() {
				case "function_declaration":
					name = extractNameFromChild(c, data)
				case "class_declaration":
					name = extractNameFromChild(c, data)
				case "variable_declaration":
					name = extractNameFromChild(c, data)
				}
				if name != "" {
					sig := extractSourceLine(data, n.StartPoint(), n.EndPoint())
					symbols = append(symbols, Symbol{
						Name:      "export " + name,
						Kind:      kind,
						Language:  lang,
						File:      filePath,
						StartLine: int(n.StartPoint().Row) + 1,
						EndLine:   int(n.EndPoint().Row) + 1,
						Signature: truncateSig(sig),
					})
				}
			}
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(root)
	return symbols
}

func hasSelfParam(n *sitter.Node, data []byte) bool {
	for i := 0; i < int(n.ChildCount()); i++ {
		c := n.Child(i)
		if c.Type() == "identifier" && c.Content(data) == "self" {
			return true
		}
	}
	return false
}

func truncateSig(s string) string {
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
