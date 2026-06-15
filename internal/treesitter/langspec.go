package treesitter

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/php"
	"github.com/smacker/go-tree-sitter/ruby"
	"github.com/smacker/go-tree-sitter/rust"
)

// LangSpec drives generic, config-based extraction for a language whose symbol,
// call, and function-scope node types follow tree-sitter's common conventions
// (most declaration nodes expose a `name` field). The hand-written extractors
// for Go/Python/JS/TS remain authoritative for those four; this registry covers
// the additional languages with one declarative entry each.
type LangSpec struct {
	Name        Language
	Extensions  []string
	Grammar     *sitter.Language
	SymbolKinds map[string]SymbolKind // node type -> emitted symbol kind
	FuncNodes   map[string]bool       // node types that open a caller scope
	CallNodes   map[string]bool       // node types that are a call/instantiation
}

// langRegistry holds the spec-driven languages. The four original languages are
// intentionally absent — they use the bespoke extractors in python.go/extract.go.
var langRegistry = map[Language]*LangSpec{}

func registerLang(s *LangSpec) {
	langRegistry[s.Name] = s
}

func init() {
	registerLang(&LangSpec{
		Name:       LangRust,
		Extensions: []string{".rs"},
		Grammar:    rust.GetLanguage(),
		SymbolKinds: map[string]SymbolKind{
			"function_item":    SymbolFunction,
			"struct_item":      SymbolStruct,
			"enum_item":        SymbolEnum,
			"trait_item":       SymbolInterface,
			"type_item":        SymbolType,
			"const_item":       SymbolConstant,
			"static_item":      SymbolVariable,
			"mod_item":         SymbolClass,
			"macro_definition": SymbolFunction,
			"union_item":       SymbolStruct,
		},
		FuncNodes: map[string]bool{"function_item": true},
		CallNodes: map[string]bool{"call_expression": true, "macro_invocation": true},
	})

	registerLang(&LangSpec{
		Name:       LangJava,
		Extensions: []string{".java"},
		Grammar:    java.GetLanguage(),
		SymbolKinds: map[string]SymbolKind{
			"class_declaration":       SymbolClass,
			"interface_declaration":   SymbolInterface,
			"enum_declaration":        SymbolEnum,
			"record_declaration":      SymbolStruct,
			"method_declaration":      SymbolMethod,
			"constructor_declaration": SymbolMethod,
			"annotation_type_declaration": SymbolInterface,
		},
		FuncNodes: map[string]bool{"method_declaration": true, "constructor_declaration": true},
		CallNodes: map[string]bool{"method_invocation": true, "object_creation_expression": true},
	})

	registerLang(&LangSpec{
		Name:       LangC,
		Extensions: []string{".c", ".h"},
		Grammar:    c.GetLanguage(),
		SymbolKinds: map[string]SymbolKind{
			"function_definition": SymbolFunction,
			"struct_specifier":    SymbolStruct,
			"enum_specifier":      SymbolEnum,
			"union_specifier":     SymbolStruct,
			"type_definition":     SymbolType,
		},
		FuncNodes: map[string]bool{"function_definition": true},
		CallNodes: map[string]bool{"call_expression": true},
	})

	registerLang(&LangSpec{
		Name:       LangCPP,
		Extensions: []string{".cpp", ".cc", ".cxx", ".hpp", ".hh"},
		Grammar:    cpp.GetLanguage(),
		SymbolKinds: map[string]SymbolKind{
			"function_definition":  SymbolFunction,
			"class_specifier":      SymbolClass,
			"struct_specifier":     SymbolStruct,
			"enum_specifier":       SymbolEnum,
			"union_specifier":      SymbolStruct,
			"namespace_definition": SymbolClass,
			"type_definition":      SymbolType,
		},
		FuncNodes: map[string]bool{"function_definition": true},
		CallNodes: map[string]bool{"call_expression": true},
	})

	registerLang(&LangSpec{
		Name:       LangCSharp,
		Extensions: []string{".cs"},
		Grammar:    csharp.GetLanguage(),
		SymbolKinds: map[string]SymbolKind{
			"class_declaration":     SymbolClass,
			"interface_declaration": SymbolInterface,
			"struct_declaration":    SymbolStruct,
			"enum_declaration":      SymbolEnum,
			"record_declaration":    SymbolStruct,
			"method_declaration":    SymbolMethod,
			"constructor_declaration": SymbolMethod,
			"property_declaration":  SymbolVariable,
			"namespace_declaration": SymbolClass,
		},
		FuncNodes: map[string]bool{"method_declaration": true, "constructor_declaration": true},
		CallNodes: map[string]bool{"invocation_expression": true, "object_creation_expression": true},
	})

	registerLang(&LangSpec{
		Name:       LangRuby,
		Extensions: []string{".rb"},
		Grammar:    ruby.GetLanguage(),
		SymbolKinds: map[string]SymbolKind{
			"method":           SymbolMethod,
			"singleton_method": SymbolMethod,
			"class":            SymbolClass,
			"module":           SymbolClass,
		},
		FuncNodes: map[string]bool{"method": true, "singleton_method": true},
		CallNodes: map[string]bool{"call": true},
	})

	registerLang(&LangSpec{
		Name:       LangPHP,
		Extensions: []string{".php"},
		Grammar:    php.GetLanguage(),
		SymbolKinds: map[string]SymbolKind{
			"function_definition":  SymbolFunction,
			"method_declaration":   SymbolMethod,
			"class_declaration":    SymbolClass,
			"interface_declaration": SymbolInterface,
			"trait_declaration":    SymbolClass,
			"enum_declaration":     SymbolEnum,
		},
		FuncNodes: map[string]bool{"function_definition": true, "method_declaration": true},
		CallNodes: map[string]bool{"function_call_expression": true, "member_call_expression": true, "object_creation_expression": true},
	})
}

// genericSymbolName resolves a declaration's name using the `name` field when
// present (true for most grammars), descending into a `declarator` for C/C++
// function definitions, and falling back to a shallow identifier scan.
func genericSymbolName(n *sitter.Node, data []byte) string {
	if name := n.ChildByFieldName("name"); name != nil {
		return name.Content(data)
	}
	if decl := n.ChildByFieldName("declarator"); decl != nil {
		if id := findFirstIdentifier(decl, data); id != "" {
			return id
		}
	}
	return extractNameFromChild(n, data)
}

// findFirstIdentifier descends to the first identifier-like node, used for
// C/C++ where the function name is nested inside the declarator.
func findFirstIdentifier(n *sitter.Node, data []byte) string {
	switch n.Type() {
	case "identifier", "field_identifier", "type_identifier", "name":
		return n.Content(data)
	}
	for i := 0; i < int(n.ChildCount()); i++ {
		if id := findFirstIdentifier(n.Child(i), data); id != "" {
			return id
		}
	}
	return ""
}

// genericCalleeName resolves the called symbol's name from a call/instantiation
// node via the `function` or `name`/`constructor` field, taking the last
// identifier segment (so a.b.c() -> c).
func genericCalleeName(n *sitter.Node, data []byte) string {
	for _, field := range []string{"function", "name", "constructor"} {
		if fn := n.ChildByFieldName(field); fn != nil {
			if id := lastIdentifier(fn, data); id != "" {
				return id
			}
		}
	}
	return ""
}

// lastIdentifier returns the final identifier-like token within n.
func lastIdentifier(n *sitter.Node, data []byte) string {
	switch n.Type() {
	case "identifier", "field_identifier", "type_identifier", "property_identifier", "name", "constant", "scoped_identifier":
		txt := n.Content(data)
		// For scoped/qualified names keep the last segment.
		for _, sep := range []string{"::", ".", "\\"} {
			if idx := lastIndex(txt, sep); idx >= 0 {
				txt = txt[idx+len(sep):]
			}
		}
		return txt
	}
	var last string
	for i := 0; i < int(n.ChildCount()); i++ {
		if id := lastIdentifier(n.Child(i), data); id != "" {
			last = id
		}
	}
	return last
}

func lastIndex(s, sub string) int {
	for i := len(s) - len(sub); i >= 0; i-- {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// extractGenericSymbols walks the tree emitting a symbol for every node whose
// type is in the spec's SymbolKinds map.
func extractGenericSymbols(root *sitter.Node, filePath string, data []byte, spec *LangSpec) []Symbol {
	var symbols []Symbol
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		if kind, ok := spec.SymbolKinds[n.Type()]; ok {
			name := genericSymbolName(n, data)
			if name != "" && name != "unknown" {
				sig := extractSourceLine(data, n.StartPoint(), n.EndPoint())
				symbols = append(symbols, Symbol{
					Name:      name,
					Kind:      kind,
					Language:  string(spec.Name),
					File:      filePath,
					StartLine: int(n.StartPoint().Row) + 1,
					EndLine:   int(n.EndPoint().Row) + 1,
					Signature: truncateSig(sig),
				})
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return symbols
}

// extractGenericCalls walks the tree maintaining a caller scope stack (pushed by
// FuncNodes) and emits an edge for each CallNode encountered within a scope.
func extractGenericCalls(root *sitter.Node, filePath string, data []byte, spec *LangSpec) []CallEdge {
	var edges []CallEdge
	var stack []string
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		if spec.FuncNodes[n.Type()] {
			name := genericSymbolName(n, data)
			if name != "" {
				stack = append(stack, name)
				for i := 0; i < int(n.ChildCount()); i++ {
					walk(n.Child(i))
				}
				stack = stack[:len(stack)-1]
				return
			}
		}
		if spec.CallNodes[n.Type()] && len(stack) > 0 {
			if callee := genericCalleeName(n, data); callee != "" {
				edges = append(edges, CallEdge{
					CallerName: stack[len(stack)-1],
					CallerFile: filePath,
					CalleeName: callee,
					File:       filePath,
					Line:       int(n.StartPoint().Row) + 1,
					Language:   string(spec.Name),
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

// extractGenericRefs captures upper-case identifier usages as references, the
// same heuristic the Python/JS extractors use for cross-file symbol linking.
func extractGenericRefs(root *sitter.Node, filePath string, data []byte, spec *LangSpec) []Reference {
	var refs []Reference
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		if _, isDecl := spec.SymbolKinds[n.Type()]; isDecl {
			// Skip the declaration's own name child; descend into the body.
			for i := 0; i < int(n.ChildCount()); i++ {
				walk(n.Child(i))
			}
			return
		}
		t := n.Type()
		if t == "identifier" || t == "type_identifier" || t == "constant" {
			name := n.Content(data)
			if len(name) >= 2 && isUpperCase(name) {
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
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return refs
}
