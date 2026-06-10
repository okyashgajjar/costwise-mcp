package treesitter

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

func extractReferences(root *sitter.Node, filePath string, data []byte, lang Language) []Reference {
	switch lang {
	case LangGo:
		return extractGoRefs(root, filePath, data)
	case LangPython:
		return extractPythonRefs(root, filePath, data)
	case LangJavaScript:
		return extractJSTSRefs(root, filePath, data, "javascript")
	case LangTypeScript:
		return extractJSTSRefs(root, filePath, data, "typescript")
	default:
		return nil
	}
}

func extractGoRefs(root *sitter.Node, filePath string, data []byte) []Reference {
	var refs []Reference
	imports := collectGoImports(root, data)

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		switch n.Type() {
		case "import_declaration":
			extractGoImportRefs(n, filePath, data, &refs)
			return

		case "import_spec":
			return

		case "function_declaration", "method_declaration", "type_declaration":
			return
		}

		if n.Type() == "identifier" {
			name := n.Content(data)
			if len(name) < 2 {
				goto walkChildren
			}
			parent := n.Parent()
			if parent != nil && isGoDefContext(parent) {
				goto walkChildren
			}
			if isKnownGoSymbol(name, imports) {
				refs = append(refs, Reference{
					SymbolName: name,
					File:       filePath,
					Line:       int(n.StartPoint().Row) + 1,
					Column:     int(n.StartPoint().Column) + 1,
					RefType:    RefReference,
					Context:    extractLine(data, int(n.StartPoint().Row)),
				})
			}
		}

	walkChildren:
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(root)
	return refs
}

func extractGoImportRefs(n *sitter.Node, filePath string, data []byte, refs *[]Reference) {
	for i := 0; i < int(n.ChildCount()); i++ {
		c := n.Child(i)
		if c.Type() == "import_spec" {
			path := strings.Trim(c.Content(data), "\"")
			pkgName := guessPkgName(path)
			*refs = append(*refs, Reference{
				SymbolName: pkgName,
				File:       filePath,
				Line:       int(c.StartPoint().Row) + 1,
				Column:     int(c.StartPoint().Column) + 1,
				RefType:    RefImport,
				Context:    c.Content(data),
			})
		}
	}
}

func extractPythonRefs(root *sitter.Node, filePath string, data []byte) []Reference {
	var refs []Reference

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		switch n.Type() {
		case "import_statement":
			content := n.Content(data)
			names := parsePythonImportNames(content)
			for _, name := range names {
				refs = append(refs, Reference{
					SymbolName: name,
					File:       filePath,
					Line:       int(n.StartPoint().Row) + 1,
					Column:     int(n.StartPoint().Column) + 1,
					RefType:    RefImport,
					Context:    content,
				})
			}
			return

		case "import_from_statement":
			content := n.Content(data)
			names := parsePythonFromImportNames(content)
			for _, name := range names {
				refs = append(refs, Reference{
					SymbolName: name,
					File:       filePath,
					Line:       int(n.StartPoint().Row) + 1,
					Column:     int(n.StartPoint().Column) + 1,
					RefType:    RefImport,
					Context:    content,
				})
			}
			return

		case "function_definition", "class_definition":
			return
		}

		if n.Type() == "identifier" {
			name := n.Content(data)
			if len(name) < 2 {
				goto walkChildren
			}
			parent := n.Parent()
			if parent != nil && isPythonDefContext(parent) {
				goto walkChildren
			}
			if isUpperCase(name) || isCommonRef(name) {
				refs = append(refs, Reference{
					SymbolName: name,
					File:       filePath,
					Line:       int(n.StartPoint().Row) + 1,
					Column:     int(n.StartPoint().Column) + 1,
					RefType:    RefReference,
					Context:    extractLine(data, int(n.StartPoint().Row)),
				})
			}
		}

	walkChildren:
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(root)
	return refs
}

func extractJSTSRefs(root *sitter.Node, filePath string, data []byte, lang string) []Reference {
	var refs []Reference

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		switch n.Type() {
		case "import_statement":
			content := n.Content(data)
			names := parseJSTSImportNames(n, data)
			for _, name := range names {
				refs = append(refs, Reference{
					SymbolName: name,
					File:       filePath,
					Line:       int(n.StartPoint().Row) + 1,
					Column:     int(n.StartPoint().Column) + 1,
					RefType:    RefImport,
					Context:    content,
				})
			}
			return

		case "export_statement":
			names := extractJSTSExportNames(n, data)
			for _, name := range names {
				refs = append(refs, Reference{
					SymbolName: name,
					File:       filePath,
					Line:       int(n.StartPoint().Row) + 1,
					Column:     int(n.StartPoint().Column) + 1,
					RefType:    RefExport,
					Context:    n.Content(data),
				})
			}
			if hasInnerDeclaration(n) {
				return
			}

		case "function_declaration", "method_definition", "class_declaration", "interface_declaration":
			return

		case "lexical_declaration", "variable_declaration":
			return
		}

		if n.Type() == "identifier" {
			name := n.Content(data)
			if len(name) < 2 {
				goto walkChildren
			}
			parent := n.Parent()
			if parent != nil && isJSTSDefContext(parent) {
				goto walkChildren
			}
			if isUpperCase(name) || isCommonRef(name) {
				refs = append(refs, Reference{
					SymbolName: name,
					File:       filePath,
					Line:       int(n.StartPoint().Row) + 1,
					Column:     int(n.StartPoint().Column) + 1,
					RefType:    RefReference,
					Context:    extractLine(data, int(n.StartPoint().Row)),
				})
			}
		}

	walkChildren:
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(root)
	return refs
}

func isGoDefContext(n *sitter.Node) bool {
	t := n.Type()
	return t == "function_declaration" || t == "method_declaration" ||
		t == "type_spec" || t == "field_declaration" ||
		t == "import_spec" || t == "import_declaration"
}

func isPythonDefContext(n *sitter.Node) bool {
	t := n.Type()
	return t == "function_definition" || t == "class_definition" ||
		t == "parameters" || t == "lambda"
}

func isJSTSDefContext(n *sitter.Node) bool {
	t := n.Type()
	return t == "function_declaration" || t == "method_definition" ||
		t == "class_declaration" || t == "interface_declaration" ||
		t == "variable_declarator" || t == "formal_parameters"
}

func isUpperCase(name string) bool {
	if len(name) == 0 {
		return false
	}
	return name[0] >= 'A' && name[0] <= 'Z'
}

var commonRefNames = map[string]bool{
	"self": true, "cls": true, "this": true, "it": true,
	"err": true, "nil": true, "true": true, "false": true,
	"None": true, "True": true, "False": true,
}

func isCommonRef(name string) bool {
	return commonRefNames[name]
}

func isKnownGoSymbol(name string, imports []string) bool {
	if isUpperCase(name) {
		return true
	}
	for _, imp := range imports {
		if name == imp {
			return true
		}
	}
	return false
}

func collectGoImports(root *sitter.Node, data []byte) []string {
	var imports []string
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		if n.Type() == "import_spec" {
			path := strings.Trim(n.Content(data), "\"")
			pkg := guessPkgName(path)
			if pkg != "" {
				imports = append(imports, pkg)
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return imports
}

func guessPkgName(path string) string {
	parts := strings.Split(path, "/")
	last := parts[len(parts)-1]
	last = strings.TrimSpace(last)
	last = strings.Trim(last, "\"")
	return last
}

func parsePythonImportNames(content string) []string {
	content = strings.TrimPrefix(content, "import ")
	parts := strings.Split(content, ",")
	var names []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Split(p, " as ")[0]
		p = strings.Split(p, ".")[0]
		if p != "" {
			names = append(names, p)
		}
	}
	return names
}

func parsePythonFromImportNames(content string) []string {
	parts := strings.Split(content, " import ")
	if len(parts) < 2 {
		return nil
	}
	rest := parts[1]
	rest = strings.TrimSpace(rest)
	if strings.HasPrefix(rest, "(") {
		rest = strings.Trim(rest, "()")
	}
	items := strings.Split(rest, ",")
	var names []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		item = strings.Split(item, " as ")[0]
		if item != "" {
			names = append(names, item)
		}
	}
	return names
}

func parseJSTSImportNames(n *sitter.Node, data []byte) []string {
	var names []string
	for i := 0; i < int(n.ChildCount()); i++ {
		c := n.Child(i)
		if c.Type() == "import_clause" {
			for j := 0; j < int(c.ChildCount()); j++ {
				clause := c.Child(j)
				switch clause.Type() {
				case "identifier":
					names = append(names, clause.Content(data))
				case "namespace_import":
					names = append(names, clause.Content(data))
				case "named_imports":
					for k := 0; k < int(clause.ChildCount()); k++ {
						spec := clause.Child(k)
						if spec.Type() == "import_specifier" {
							name := spec.Content(data)
							name = strings.TrimSpace(strings.Split(name, " as ")[0])
							if name != "" {
								names = append(names, name)
							}
						}
					}
				}
			}
		}
	}
	return names
}

func extractJSTSExportNames(n *sitter.Node, data []byte) []string {
	var names []string
	for i := 0; i < int(n.ChildCount()); i++ {
		c := n.Child(i)
		switch c.Type() {
		case "function_declaration", "class_declaration":
			name := extractNameFromChild(c, data)
			if name != "" {
				names = append(names, name)
			}
		case "variable_declaration":
			for j := 0; j < int(c.ChildCount()); j++ {
				vd := c.Child(j)
				if vd.Type() == "variable_declarator" {
					name := extractNameFromChild(vd, data)
					if name != "" {
						names = append(names, name)
					}
				}
			}
		}
	}
	return names
}

func hasInnerDeclaration(n *sitter.Node) bool {
	for i := 0; i < int(n.ChildCount()); i++ {
		c := n.Child(i)
		switch c.Type() {
		case "function_declaration", "class_declaration", "variable_declaration":
			return true
		}
	}
	return false
}

func extractLine(data []byte, lineNum int) string {
	lines := strings.Split(string(data), "\n")
	if lineNum >= 0 && lineNum < len(lines) {
		line := strings.TrimSpace(lines[lineNum])
		if len(line) > 120 {
			line = line[:120] + "..."
		}
		return line
	}
	return ""
}
