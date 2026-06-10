package treesitter

type RefType int

const (
	RefDefinition RefType = iota
	RefReference
	RefImport
	RefExport
)

func (r RefType) String() string {
	switch r {
	case RefDefinition:
		return "definition"
	case RefReference:
		return "reference"
	case RefImport:
		return "import"
	case RefExport:
		return "export"
	default:
		return "unknown"
	}
}

type Reference struct {
	SymbolID   string  `json:"symbol_id"`
	SymbolName string  `json:"symbol_name"`
	File       string  `json:"file"`
	Line       int     `json:"line"`
	Column     int     `json:"column"`
	RefType    RefType `json:"ref_type"`
	Context    string  `json:"context"`
}

type RefQueryResult struct {
	Symbol     Symbol      `json:"symbol"`
	Definition *Reference  `json:"definition,omitempty"`
	References []Reference `json:"references"`
	Imports    []Reference `json:"imports"`
	Exports    []Reference `json:"exports"`
	Score      float64     `json:"score"`
}
