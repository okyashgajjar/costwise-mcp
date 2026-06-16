package kmemory

import (
	"testing"
)

func TestNewKnowledgeMemory(t *testing.T) {
	km := NewKnowledgeMemory()
	if km == nil {
		t.Fatal("NewKnowledgeMemory() returned nil")
	}
	if len(km.entries) != 0 {
		t.Errorf("expected empty entries, got %d", len(km.entries))
	}
}

func TestStoreAndSearch(t *testing.T) {
	km := NewKnowledgeMemory()

	entry := NewSymbolEntry("RepoMap", "internal/repomap/repomap.go", "type RepoMap struct", []string{"indexer"})
	km.Store(SymbolKnowledge, "RepoMap", entry)

	results := km.Search(SymbolKnowledge, "RepoMap")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Key != "RepoMap" {
		t.Errorf("expected key RepoMap, got %s", results[0].Key)
	}
	if results[0].File != "internal/repomap/repomap.go" {
		t.Errorf("expected file internal/repomap/repomap.go, got %s", results[0].File)
	}
	if results[0].Type != SymbolKnowledge {
		t.Errorf("expected type SymbolKnowledge, got %s", results[0].Type.String())
	}
}

func TestStoreAndSearchAll(t *testing.T) {
	km := NewKnowledgeMemory()

	km.Store(SymbolKnowledge, "RepoMap", NewSymbolEntry("RepoMap", "a.go", "type RepoMap struct", nil))
	km.Store(CallerKnowledge, "RepoMapCallers", NewCallerEntry("RepoMap", []string{"indexer.go", "repomap.go"}))
	km.Store(ReferenceKnowledge, "RepoMapRefs", NewReferenceEntry("RepoMap", []string{"usage.go"}))

	results := km.SearchAll("RepoMap")
	if len(results) < 2 {
		t.Fatalf("expected at least 2 entries for RepoMap, got %d", len(results))
	}

	types := make(map[KnowledgeType]bool)
	for _, e := range results {
		types[e.Type] = true
	}
	if !types[SymbolKnowledge] {
		t.Error("expected SymbolKnowledge entry")
	}
}

func TestSearchNoMatch(t *testing.T) {
	km := NewKnowledgeMemory()
	results := km.Search(SymbolKnowledge, "NonExistent")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchAllNoMatch(t *testing.T) {
	km := NewKnowledgeMemory()
	results := km.SearchAll("NonExistent")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// A multi-word query that is NOT a verbatim substring of the stored fact must
// still match via term overlap. The old whole-query Contains check failed this,
// which made `recall` return nothing for natural-language queries.
func TestSearchMultiWordTermOverlap(t *testing.T) {
	km := NewKnowledgeMemory()
	km.Store(UserNote, "sess-verify", NewUserNote("sess-verify", "9 tools, grep_code removed, multi-client skills"))

	results := km.SearchAll("multi-client skills native rules files AGENTS.md GEMINI.md")
	if len(results) == 0 || results[0].Key != "sess-verify" {
		t.Fatalf("multi-word query should match the fact via term overlap, got %v", results)
	}

	typed := km.Search(UserNote, "did we remove the grep_code tool")
	if len(typed) == 0 || typed[0].Key != "sess-verify" {
		t.Errorf("typed Search should match via term overlap, got %v", typed)
	}

	// A query sharing no terms still returns nothing.
	if got := km.SearchAll("database migration rollback"); len(got) != 0 {
		t.Errorf("unrelated query should not match, got %v", got)
	}
}

func TestStoreMultipleTypes(t *testing.T) {
	km := NewKnowledgeMemory()

	entry1 := NewSymbolEntry("SymbolDB", "db/symbol.go", "type SymbolDB struct", nil)
	entry2 := NewCallerEntry("SymbolDB", []string{"retriever.go", "indexer.go"})
	entry3 := NewReferenceEntry("SymbolDB", []string{"test_helpers.go"})

	km.Store(SymbolKnowledge, "SymbolDB", entry1)
	km.Store(CallerKnowledge, "SymbolDB", entry2)
	km.Store(ReferenceKnowledge, "SymbolDB", entry3)

	snapshot := km.Snapshot()
	if len(snapshot) != 3 {
		t.Errorf("expected 3 entries, got %d", len(snapshot))
	}

	for _, e := range snapshot {
		if e.HitCount != 1 {
			t.Errorf("expected initial HitCount=1, got %d for %s/%s", e.HitCount, e.Type.String(), e.Key)
		}
	}
}

func TestHitCountIncrement(t *testing.T) {
	km := NewKnowledgeMemory()

	km.Store(SymbolKnowledge, "RepoMap", NewSymbolEntry("RepoMap", "repomap.go", "type RepoMap struct", nil))

	km.Lookup(SymbolKnowledge, "RepoMap")
	km.Lookup(SymbolKnowledge, "RepoMap")

	e := km.Lookup(SymbolKnowledge, "RepoMap")
	if e == nil {
		t.Fatal("expected to find RepoMap entry")
	}
	if e.HitCount != 4 {
		t.Errorf("expected HitCount=4 after 3 extra lookups, got %d", e.HitCount)
	}
}

func TestGrepGlobKnowledge(t *testing.T) {
	km := NewKnowledgeMemory()

	km.Store(GrepKnowledge, "find config", &KnowledgeEntry{
		Type:       GrepKnowledge,
		Key:        "find config",
		Value:      "config.go, config_test.go",
		Metadata:   []string{"config.go", "config_test.go"},
		File:       "",
		Confidence: 0.7,
	})

	km.Store(GlobKnowledge, "*.go", &KnowledgeEntry{
		Type:       GlobKnowledge,
		Key:        "*.go",
		Value:      "many files",
		Metadata:   []string{"main.go", "app.go"},
		File:       "",
		Confidence: 0.6,
	})

	results := km.Search(GrepKnowledge, "find config")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Type != GrepKnowledge {
		t.Errorf("expected GrepKnowledge, got %s", results[0].Type.String())
	}

	results = km.Search(GlobKnowledge, "*.go")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Type != GlobKnowledge {
		t.Errorf("expected GlobKnowledge, got %s", results[0].Type.String())
	}
}

func TestFileOwnership(t *testing.T) {
	km := NewKnowledgeMemory()

	km.Store(FileOwnership, "internal/retrieval/pipeline.go", &KnowledgeEntry{
		Type:       FileOwnership,
		Key:        "internal/retrieval/pipeline.go",
		Value:      "retrieval",
		File:       "internal/retrieval/pipeline.go",
		Confidence: 0.8,
	})

	results := km.Search(FileOwnership, "pipeline")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Type != FileOwnership {
		t.Errorf("expected FileOwnership, got %s", results[0].Type.String())
	}
	if results[0].Value != "retrieval" {
		t.Errorf("expected value retrieval, got %s", results[0].Value)
	}
}

func TestNewSymbolEntry(t *testing.T) {
	e := NewSymbolEntry("RepoMap", "repomap.go", "type RepoMap struct", []string{"indexer"})
	if e.Type != SymbolKnowledge {
		t.Errorf("expected SymbolKnowledge, got %s", e.Type.String())
	}
	if e.Key != "RepoMap" {
		t.Errorf("expected key RepoMap, got %s", e.Key)
	}
	if e.File != "repomap.go" {
		t.Errorf("expected file repomap.go, got %s", e.File)
	}
	if e.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %f", e.Confidence)
	}
}

func TestNewCallerEntry(t *testing.T) {
	e := NewCallerEntry("RepoMap", []string{"caller1", "caller2"})
	if e.Type != CallerKnowledge {
		t.Errorf("expected CallerKnowledge, got %s", e.Type.String())
	}
	if e.Key != "RepoMap" {
		t.Errorf("expected key RepoMap, got %s", e.Key)
	}
	if e.Value != "caller1, caller2" {
		t.Errorf("expected value 'caller1, caller2', got %s", e.Value)
	}
}

func TestNewReferenceEntry(t *testing.T) {
	e := NewReferenceEntry("RepoMap", []string{"ref1", "ref2"})
	if e.Type != ReferenceKnowledge {
		t.Errorf("expected ReferenceKnowledge, got %s", e.Type.String())
	}
	if e.Key != "RepoMap" {
		t.Errorf("expected key RepoMap, got %s", e.Key)
	}
	if e.Value != "ref1, ref2" {
		t.Errorf("expected value 'ref1, ref2', got %s", e.Value)
	}
}

func TestKnowledgeTypeString(t *testing.T) {
	tests := []struct {
		kt   KnowledgeType
		want string
	}{
		{SymbolKnowledge, "symbol"},
		{CallerKnowledge, "caller"},
		{ReferenceKnowledge, "reference"},
		{GrepKnowledge, "grep"},
		{GlobKnowledge, "glob"},
		{ArchitectureKnowledge, "architecture"},
		{RepoSummary, "repo_summary"},
		{ModuleOwnership, "module_ownership"},
		{FileOwnership, "file_ownership"},
		{KnowledgeType(99), "unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := tc.kt.String()
			if got != tc.want {
				t.Errorf("(%d).String() = %q, want %q", tc.kt, got, tc.want)
			}
		})
	}
}
