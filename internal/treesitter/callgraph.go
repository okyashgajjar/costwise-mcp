package treesitter

import (
	sitter "github.com/smacker/go-tree-sitter"
)

type CallEdge struct {
	ID         int
	CallerName string
	CallerFile string
	CalleeName string
	File       string
	Line       int
	Language   string
}

func extractCalls(root *sitter.Node, filePath string, data []byte, lang Language) []CallEdge {
	switch lang {
	case LangGo:
		return extractGoCalls(root, filePath, data)
	case LangPython:
		return extractPythonCalls(root, filePath, data)
	case LangJavaScript:
		return extractJSTSCalls(root, filePath, data, "javascript")
	case LangTypeScript:
		return extractJSTSCalls(root, filePath, data, "typescript")
	default:
		if spec, ok := langRegistry[lang]; ok {
			return extractGenericCalls(root, filePath, data, spec)
		}
		return nil
	}
}

func extractCallName(n *sitter.Node, data []byte) string {
	if n.ChildCount() == 0 {
		return ""
	}
	fn := n.Child(0)
	switch fn.Type() {
	case "identifier":
		return fn.Content(data)
	case "attribute":
		for i := int(fn.ChildCount()) - 1; i >= 0; i-- {
			c := fn.Child(i)
			if c.Type() == "identifier" || c.Type() == "property_identifier" {
				return c.Content(data)
			}
		}
	case "selector_expression":
		for i := int(fn.ChildCount()) - 1; i >= 0; i-- {
			c := fn.Child(i)
			if c.Type() == "identifier" || c.Type() == "field_identifier" || c.Type() == "property_identifier" {
				return c.Content(data)
			}
		}
	}
	return ""
}

func extractPythonCalls(root *sitter.Node, filePath string, data []byte) []CallEdge {
	var edges []CallEdge
	var callerStack []string

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		switch n.Type() {
		case "function_definition":
			name := extractNameFromChild(n, data)
			if name != "" {
				callerStack = append(callerStack, name)
			}
			for i := 0; i < int(n.ChildCount()); i++ {
				walk(n.Child(i))
			}
			if name != "" {
				callerStack = callerStack[:len(callerStack)-1]
			}
			return
		}

		if n.Type() == "call" && len(callerStack) > 0 {
			callee := extractCallName(n, data)
			if callee != "" {
				caller := callerStack[len(callerStack)-1]
				edges = append(edges, CallEdge{
					CallerName: caller,
					CallerFile: filePath,
					CalleeName: callee,
					File:       filePath,
					Line:       int(n.StartPoint().Row) + 1,
					Language:   "python",
				})
			}
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(root)
	return edges
}

func extractGoCalls(root *sitter.Node, filePath string, data []byte) []CallEdge {
	var edges []CallEdge
	var callerStack []string

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		switch n.Type() {
		case "function_declaration":
			name := extractNameFromChild(n, data)
			if name != "" {
				callerStack = append(callerStack, name)
			}
			for i := 0; i < int(n.ChildCount()); i++ {
				walk(n.Child(i))
			}
			if name != "" {
				callerStack = callerStack[:len(callerStack)-1]
			}
			return

		case "method_declaration":
			name := extractNameFromChild(n, data)
			if name != "" {
				callerStack = append(callerStack, name)
			}
			for i := 0; i < int(n.ChildCount()); i++ {
				walk(n.Child(i))
			}
			if name != "" {
				callerStack = callerStack[:len(callerStack)-1]
			}
			return
		}

		if n.Type() == "call_expression" && len(callerStack) > 0 {
			callee := extractCallName(n, data)
			if callee != "" {
				caller := callerStack[len(callerStack)-1]
				edges = append(edges, CallEdge{
					CallerName: caller,
					CallerFile: filePath,
					CalleeName: callee,
					File:       filePath,
					Line:       int(n.StartPoint().Row) + 1,
					Language:   "go",
				})
			}
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(root)
	return edges
}

func extractJSTSCalls(root *sitter.Node, filePath string, data []byte, lang string) []CallEdge {
	var edges []CallEdge
	var callerStack []string

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		switch n.Type() {
		case "function_declaration":
			name := extractNameFromChild(n, data)
			if name != "" {
				callerStack = append(callerStack, name)
			}
			for i := 0; i < int(n.ChildCount()); i++ {
				walk(n.Child(i))
			}
			if name != "" {
				callerStack = callerStack[:len(callerStack)-1]
			}
			return

		case "method_definition":
			name := extractNameFromChild(n, data)
			if name != "" {
				callerStack = append(callerStack, name)
			}
			for i := 0; i < int(n.ChildCount()); i++ {
				walk(n.Child(i))
			}
			if name != "" {
				callerStack = callerStack[:len(callerStack)-1]
			}
			return

		case "arrow_function":
			return
		}

		if n.Type() == "call_expression" && len(callerStack) > 0 {
			callee := extractCallName(n, data)
			if callee != "" {
				caller := callerStack[len(callerStack)-1]
				edges = append(edges, CallEdge{
					CallerName: caller,
					CallerFile: filePath,
					CalleeName: callee,
					File:       filePath,
					Line:       int(n.StartPoint().Row) + 1,
					Language:   lang,
				})
			}
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(root)
	return edges
}
