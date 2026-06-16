package kmemory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type KnowledgeType int

const (
	SymbolKnowledge KnowledgeType = iota
	CallerKnowledge
	ReferenceKnowledge
	GrepKnowledge
	GlobKnowledge
	ArchitectureKnowledge
	RepoSummary
	ModuleOwnership
	FileOwnership
	// UserNote is a durable fact the user/model asked to `remember`. Appended
	// last so existing persisted Type ints keep their values.
	UserNote
)

func (kt KnowledgeType) String() string {
	switch kt {
	case SymbolKnowledge:
		return "symbol"
	case CallerKnowledge:
		return "caller"
	case ReferenceKnowledge:
		return "reference"
	case GrepKnowledge:
		return "grep"
	case GlobKnowledge:
		return "glob"
	case ArchitectureKnowledge:
		return "architecture"
	case RepoSummary:
		return "repo_summary"
	case ModuleOwnership:
		return "module_ownership"
	case FileOwnership:
		return "file_ownership"
	case UserNote:
		return "user_note"
	default:
		return "unknown"
	}
}

type KnowledgeEntry struct {
	Type       KnowledgeType `json:"type"`
	Key        string        `json:"key"`
	Value      string        `json:"value"`
	Metadata   []string      `json:"metadata,omitempty"`
	File       string        `json:"file,omitempty"`
	Line       int           `json:"line,omitempty"`
	Confidence float64       `json:"confidence"`
	CreatedAt  time.Time     `json:"created_at"`
	LastUsedAt time.Time     `json:"last_used_at"`
	HitCount   int           `json:"hit_count"`
}

type KnowledgeMemory struct {
	mu      sync.RWMutex
	entries map[string]*KnowledgeEntry
	index   map[string][]string
}

func NewKnowledgeMemory() *KnowledgeMemory {
	return &KnowledgeMemory{
		entries: make(map[string]*KnowledgeEntry),
		index:   make(map[string][]string),
	}
}

func entryKey(kt KnowledgeType, key string) string {
	return fmt.Sprintf("%s:%s", kt.String(), strings.ToLower(key))
}

func (km *KnowledgeMemory) Store(kt KnowledgeType, key string, entry *KnowledgeEntry) {
	km.mu.Lock()
	defer km.mu.Unlock()

	ek := entryKey(kt, key)
	entry.Key = key
	entry.Type = kt
	entry.CreatedAt = time.Now()
	entry.LastUsedAt = time.Now()
	entry.HitCount = 1
	km.entries[ek] = entry

	indexKey := strings.ToLower(key)
	km.index[indexKey] = append(km.index[indexKey], ek)

	for _, token := range tokenizeKey(key) {
		km.index[token] = append(km.index[token], ek)
	}
}

func (km *KnowledgeMemory) Lookup(kt KnowledgeType, key string) *KnowledgeEntry {
	km.mu.Lock()
	defer km.mu.Unlock()

	ek := entryKey(kt, key)
	if e, ok := km.entries[ek]; ok {
		e.LastUsedAt = time.Now()
		e.HitCount++
		return e
	}
	return nil
}

func (km *KnowledgeMemory) Search(kt KnowledgeType, query string) []*KnowledgeEntry {
	km.mu.RLock()
	defer km.mu.RUnlock()

	terms := queryTerms(query)
	if len(terms) == 0 {
		return nil
	}
	return rankByOverlap(km.entries, terms, func(e *KnowledgeEntry) bool {
		return e.Type == kt
	})
}

func (km *KnowledgeMemory) SearchAll(query string) []*KnowledgeEntry {
	km.mu.RLock()
	defer km.mu.RUnlock()

	terms := queryTerms(query)
	if len(terms) == 0 {
		return nil
	}
	return rankByOverlap(km.entries, terms, func(*KnowledgeEntry) bool { return true })
}

// queryTerms splits a free-text query into lowercased search terms (reusing the
// same tokenizer as the index, so matching is consistent).
func queryTerms(query string) []string {
	return tokenizeKey(query)
}

// matchScore counts how many distinct query terms hit an entry, weighting key
// matches above value/file matches. Zero means no overlap. This replaces the old
// whole-query substring check, which failed for any multi-word query.
func matchScore(e *KnowledgeEntry, terms []string) int {
	key := strings.ToLower(e.Key)
	val := strings.ToLower(e.Value)
	file := strings.ToLower(e.File)
	score := 0
	for _, t := range terms {
		switch {
		case strings.Contains(key, t):
			score += 2
		case strings.Contains(val, t), strings.Contains(file, t):
			score++
		}
	}
	return score
}

// rankByOverlap returns the entries passing keep that share at least one query
// term, ordered by descending overlap score (ties broken by most-recently-used).
// The search space is bounded by remembered/cached entries — not repo size — so
// this stays microsecond-fast regardless of how large the indexed repo is.
func rankByOverlap(entries map[string]*KnowledgeEntry, terms []string, keep func(*KnowledgeEntry) bool) []*KnowledgeEntry {
	type scored struct {
		e *KnowledgeEntry
		s int
	}
	var hits []scored
	for _, e := range entries {
		if !keep(e) {
			continue
		}
		if s := matchScore(e, terms); s > 0 {
			hits = append(hits, scored{e, s})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].s != hits[j].s {
			return hits[i].s > hits[j].s
		}
		return hits[i].e.LastUsedAt.After(hits[j].e.LastUsedAt)
	})
	out := make([]*KnowledgeEntry, len(hits))
	for i, h := range hits {
		out[i] = h.e
	}
	return out
}

func (km *KnowledgeMemory) Stats() map[string]int {
	km.mu.RLock()
	defer km.mu.RUnlock()

	stats := make(map[string]int)
	for _, e := range km.entries {
		stats[e.Type.String()]++
	}
	return stats
}

func (km *KnowledgeMemory) Snapshot() []*KnowledgeEntry {
	km.mu.RLock()
	defer km.mu.RUnlock()

	entries := make([]*KnowledgeEntry, 0, len(km.entries))
	for _, e := range km.entries {
		entries = append(entries, e)
	}
	return entries
}

func (km *KnowledgeMemory) SaveToFile(path string) error {
	km.mu.RLock()
	defer km.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	snapshot := struct {
		UpdatedAt time.Time         `json:"updated_at"`
		Entries   []*KnowledgeEntry `json:"entries"`
	}{
		UpdatedAt: time.Now(),
	}

	for _, e := range km.entries {
		snapshot.Entries = append(snapshot.Entries, e)
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (km *KnowledgeMemory) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var snapshot struct {
		UpdatedAt time.Time         `json:"updated_at"`
		Entries   []*KnowledgeEntry `json:"entries"`
	}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return err
	}

	km.mu.Lock()
	defer km.mu.Unlock()

	for _, e := range snapshot.Entries {
		ek := entryKey(e.Type, e.Key)
		e.HitCount = 1
		km.entries[ek] = e

		indexKey := strings.ToLower(e.Key)
		km.index[indexKey] = append(km.index[indexKey], ek)
		for _, token := range tokenizeKey(e.Key) {
			km.index[token] = append(km.index[token], ek)
		}
	}

	return nil
}

func tokenizeKey(key string) []string {
	key = strings.ToLower(key)
	words := strings.FieldsFunc(key, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})

	var tokens []string
	seen := make(map[string]bool)
	for _, w := range words {
		if len(w) >= 2 && !seen[w] {
			seen[w] = true
			tokens = append(tokens, w)
		}
	}
	return tokens
}

func NewSymbolEntry(symbolName, file, definition string, callers []string) *KnowledgeEntry {
	return &KnowledgeEntry{
		Type:       SymbolKnowledge,
		Key:        symbolName,
		Value:      definition,
		File:       file,
		Confidence: 1.0,
	}
}

func NewReferenceEntry(symbol string, files []string) *KnowledgeEntry {
	return &KnowledgeEntry{
		Type:       ReferenceKnowledge,
		Key:        symbol,
		Value:      strings.Join(files, ", "),
		Metadata:   files,
		Confidence: 0.8,
	}
}

func NewCallerEntry(symbol string, callers []string) *KnowledgeEntry {
	return &KnowledgeEntry{
		Type:       CallerKnowledge,
		Key:        symbol,
		Value:      strings.Join(callers, ", "),
		Metadata:   callers,
		Confidence: 0.85,
	}
}

// NewUserNote builds a durable user-remembered fact entry.
func NewUserNote(key, fact string) *KnowledgeEntry {
	return &KnowledgeEntry{
		Type:       UserNote,
		Key:        key,
		Value:      fact,
		Confidence: 1.0,
	}
}
