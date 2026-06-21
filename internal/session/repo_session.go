package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/okyashgajjar/costwise-mcp/internal/cache"
	"github.com/okyashgajjar/costwise-mcp/internal/discovery_memory"
	"github.com/okyashgajjar/costwise-mcp/internal/kmemory"
	"github.com/okyashgajjar/costwise-mcp/internal/repo_memory"
	"github.com/okyashgajjar/costwise-mcp/internal/repository"
	"github.com/okyashgajjar/costwise-mcp/internal/retrieval"
	"github.com/okyashgajjar/costwise-mcp/internal/stash"
	"github.com/okyashgajjar/costwise-mcp/internal/treesitter"
)

// RepoSession represents the stateful repository context for a conversation.
// It owns the underlying data structures to prevent redundant initialization
// and holds lightweight memory of recently resolved entities.
type RepoSession struct {
	SessionID string
	Repo      *repository.RepositoryInfo

	DB        *treesitter.SymbolDB
	Indexer   *retrieval.SharedIndexer
	Knowledge *retrieval.KnowledgeStore
	Cache     *cache.Cache

	// Conversation memory
	RecentSymbols map[string]retrieval.RetrievalResult
	LastResolved  string // Name of the last major symbol discussed

	// Persistent memories
	RepoMem *repo_memory.RepoMemory
	DiscMem *discovery_memory.DiscoveryMemory

	// Session knowledge memory (structured facts across turns)
	KnowledgeMem *kmemory.KnowledgeMemory

	// V2 cache-reducing stores: large-blob stash + per-repo path for the
	// persisted user facts that back `remember`.
	Stash     *stash.Store
	FactsPath string
}

// NewRepoSession creates a new repository session, initializing the DB, Indexer,
// KnowledgeStore, and Cache for the given repository root.
func NewRepoSession(ctx context.Context, repoRoot string, sessionID string) (*RepoSession, error) {
	return newRepoSession(ctx, repoRoot, sessionID, true)
}

// NewRepoSessionWithoutIndex creates a new repository session without running the indexer.
// Use when the index already exists and is up-to-date.
func NewRepoSessionWithoutIndex(ctx context.Context, repoRoot string, sessionID string) (*RepoSession, error) {
	return newRepoSession(ctx, repoRoot, sessionID, false)
}

func newRepoSession(ctx context.Context, repoRoot string, sessionID string, shouldIndex bool) (*RepoSession, error) {
	mgr := repository.NewManager()
	var info *repository.RepositoryInfo
	var err error
	if repoRoot != "" {
		info = &repository.RepositoryInfo{Root: repoRoot}
	} else {
		info, err = mgr.Detect()
		if err != nil {
			return nil, fmt.Errorf("failed to detect repository: %w", err)
		}
	}

	db, err := treesitter.NewSymbolDB(info.Root)
	if err != nil {
		return nil, fmt.Errorf("failed to open symbol DB: %w", err)
	}

	indexer := retrieval.NewSharedIndexer(db, info.Root, retrieval.IndexConfig{
		ParseSymbols:    true,
		ParseReferences: true,
		ParseCalls:      true,
	})

	if shouldIndex {
		if _, err := indexer.Index(ctx); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to index repository: %w", err)
		}
	}

	knowledge := retrieval.NewKnowledgeStore(db, info.Root)

	c, err := cache.NewCache(info.Root, 100)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to open cache: %w", err)
	}

	repoMemPath := filepath.Join(os.TempDir(), "repo_memory.db")
	repoMem, err := repo_memory.Init(repoMemPath)
	if err != nil {
		return nil, fmt.Errorf("init repo memory: %w", err)
	}
	discMemPath := filepath.Join(os.TempDir(), "discovery_memory.db")
	discMem, err := discovery_memory.Init(discMemPath)
	if err != nil {
		return nil, fmt.Errorf("init discovery memory: %w", err)
	}

	// V2: per-repo large-blob stash and durable user facts (NOT os.TempDir —
	// those would clobber across repos). Both live under the repo index dir.
	stashStore, err := stash.New(info.Root)
	if err != nil {
		return nil, fmt.Errorf("init stash: %w", err)
	}
	factsPath := filepath.Join(info.Root, ".mycli-fts", "session_facts.json")
	km := kmemory.NewKnowledgeMemory()
	_ = km.LoadFromFile(factsPath) // missing file is fine

	return &RepoSession{
		SessionID:     sessionID,
		Repo:          info,
		DB:            db,
		Indexer:       indexer,
		Knowledge:     knowledge,
		Cache:         c,
		RecentSymbols: make(map[string]retrieval.RetrievalResult),
		RepoMem:       repoMem,
		DiscMem:       discMem,
		KnowledgeMem:  km,
		Stash:         stashStore,
		FactsPath:     factsPath,
	}, nil
}

// RememberFact stores a durable user note and persists it to the per-repo facts
// file so it survives server restarts.
func (rs *RepoSession) RememberFact(key, fact string) error {
	rs.KnowledgeMem.Store(kmemory.UserNote, key, kmemory.NewUserNote(key, fact))
	if rs.FactsPath != "" {
		return rs.KnowledgeMem.SaveToFile(rs.FactsPath)
	}
	return nil
}

// RecallFacts returns remembered user notes matching query as "key: value"
// lines. An empty query returns all notes.
func (rs *RepoSession) RecallFacts(query string) []string {
	var entries []*kmemory.KnowledgeEntry
	if strings.TrimSpace(query) == "" {
		entries = rs.KnowledgeMem.Snapshot()
	} else {
		entries = rs.KnowledgeMem.Search(kmemory.UserNote, query)
		if len(entries) == 0 {
			entries = rs.KnowledgeMem.SearchAll(query)
		}
	}
	var out []string
	for _, e := range entries {
		if e.Type != kmemory.UserNote {
			continue
		}
		out = append(out, fmt.Sprintf("%s: %s", e.Key, e.Value))
	}
	return out
}

// Close releases all underlying resources.
func (rs *RepoSession) Close() {
	if rs.DB != nil {
		rs.DB.Close()
	}
	if rs.Cache != nil {
		rs.Cache.Close()
	}
}

// StoreResult saves a retrieval result in the session memory and extracts
// the primary symbol name from the query to use for pronoun resolution.
func (rs *RepoSession) StoreResult(query string, results []retrieval.RetrievalResult) {
	for _, res := range results {
		if res.File != "" {
			rs.RecentSymbols[strings.ToLower(res.File)] = res
		}
	}
	// Prefer a CamelCase/symbol word from the query over a raw file path.
	if sym := extractSymbolFromQuery(query); sym != "" {
		rs.LastResolved = sym
	} else if len(results) > 0 && results[0].File != "" {
		// Fall back to the base filename without extension
		base := filepath.Base(results[0].File)
		if idx := strings.LastIndexByte(base, '.'); idx > 0 {
			base = base[:idx]
		}
		rs.LastResolved = base
	}
}

// extractSymbolFromQuery returns the first CamelCase or ALL_CAPS word from
// the query, which is typically the symbol the user is asking about.
// For example: "Where is RepoMap implemented?" → "RepoMap"
func extractSymbolFromQuery(query string) string {
	words := strings.Fields(query)
	for _, w := range words {
		// Strip trailing punctuation
		w = strings.TrimRight(w, "?,.:;!")
		if len(w) < 2 {
			continue
		}
		// CamelCase: starts with uppercase and has at least one more uppercase letter or underscore
		if w[0] >= 'A' && w[0] <= 'Z' {
			hasInternalUpper := false
			for _, c := range w[1:] {
				if c >= 'A' && c <= 'Z' || c == '_' {
					hasInternalUpper = true
					break
				}
			}
			if hasInternalUpper {
				return w
			}
		}
	}
	return ""
}

// ResolveQuery attempts to replace pronouns like "it" or "this" with the last resolved entity.
func (rs *RepoSession) ResolveQuery(query string) string {
	if rs.LastResolved == "" {
		return query
	}

	// Regex to match "it" or "this" as whole words
	reIt := regexp.MustCompile(`\b(it|this)\b`)
	lower := strings.ToLower(query)
	if reIt.MatchString(lower) {
		return reIt.ReplaceAllStringFunc(query, func(_ string) string {
			return rs.LastResolved
		})
	}

	return query
}

// GetCachedContext attempts to retrieve full context strings directly from the session
// memory if the query exactly matches a known entity.
func (rs *RepoSession) GetCachedContext(query string) ([]retrieval.RetrievalResult, bool) {
	lower := strings.ToLower(query)

	// Direct symbol match
	if res, ok := rs.RecentSymbols[lower]; ok {
		return []retrieval.RetrievalResult{res}, true
	}

	return nil, false
}
