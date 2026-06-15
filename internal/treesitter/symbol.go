package treesitter

type SymbolKind string

const (
	SymbolFunction  SymbolKind = "function"
	SymbolMethod    SymbolKind = "method"
	SymbolClass     SymbolKind = "class"
	SymbolInterface SymbolKind = "interface"
	SymbolStruct    SymbolKind = "struct"
	SymbolType      SymbolKind = "type"
	SymbolEnum      SymbolKind = "enum"
	SymbolConstant  SymbolKind = "constant"
	SymbolVariable  SymbolKind = "variable"
	SymbolImport    SymbolKind = "import"
	SymbolExport    SymbolKind = "export"
)

type Symbol struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Kind      SymbolKind `json:"kind"`
	Language  string     `json:"language"`
	File      string     `json:"file"`
	StartLine int        `json:"start_line"`
	EndLine   int        `json:"end_line"`
	Signature string     `json:"signature"`
	Content   string     `json:"content,omitempty"`
}

type SymbolMatch struct {
	Symbol  Symbol  `json:"symbol"`
	Score   float64 `json:"score"`
	Reason  string  `json:"reason"`
	Snippet string  `json:"snippet"`
}
