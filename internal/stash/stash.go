// Package stash stores large blobs out of the model's context window, keyed by
// a short content handle, so they can be recalled query-scoped instead of being
// kept resident in the conversation (which the client re-caches every turn).
//
// It is per-repo and file-backed under <repoRoot>/.mycli-fts/stash/ — the same
// per-repo index dir used by treesitter.NewSymbolDB and cache.NewCache. Storing
// is lossless: the full blob lives on disk and is always re-fetchable, so moving
// it out of context never drops information.
package stash

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	stashDirName  = "stash"
	manifestName  = "manifest.json"
	handleBytes   = 6 // 12 hex chars
	defaultBudget = 500
	maxEntries    = 256 // soft cap; oldest blobs are pruned past this
)

// Entry is the metadata for one stashed blob. The content itself lives in a
// sibling file named "<handle>.txt".
type Entry struct {
	Handle    string    `json:"handle"`
	Label     string    `json:"label"`
	Tokens    int       `json:"tokens"`
	Bytes     int       `json:"bytes"`
	CreatedAt time.Time `json:"created_at"`
}

// Store is a per-repo, file-backed blob store. Safe for concurrent use.
type Store struct {
	mu       sync.Mutex
	dir      string
	manifest string
	idx      map[string]Entry // handle -> metadata
}

// New opens (creating if needed) the stash store for a repository root.
func New(repoRoot string) (*Store, error) {
	dir := filepath.Join(repoRoot, ".mycli-fts", stashDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating stash dir: %w", err)
	}
	s := &Store{
		dir:      dir,
		manifest: filepath.Join(dir, manifestName),
		idx:      make(map[string]Entry),
	}
	s.load()
	return s, nil
}

// Store writes content out of context and returns its handle. The handle is a
// content hash, so identical content always yields the same handle (idempotent).
func (s *Store) Store(content, label string) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sum := sha256.Sum256([]byte(content))
	handle := hex.EncodeToString(sum[:handleBytes])

	if err := os.WriteFile(filepath.Join(s.dir, handle+".txt"), []byte(content), 0o644); err != nil {
		return Entry{}, fmt.Errorf("writing stash: %w", err)
	}

	e := Entry{
		Handle:    handle,
		Label:     strings.TrimSpace(label),
		Tokens:    len(content) / 4,
		Bytes:     len(content),
		CreatedAt: time.Now().UTC(),
	}
	if existing, ok := s.idx[handle]; ok {
		e.CreatedAt = existing.CreatedAt // preserve original timestamp
		if e.Label == "" {
			e.Label = existing.Label
		}
	}
	s.idx[handle] = e
	s.pruneLocked(maxEntries)
	s.persist()
	return e, nil
}

// Prune keeps only the most recent max entries, deleting older blobs from disk.
// It returns the number removed. A non-positive max applies the default cap.
func (s *Store) Prune(max int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.pruneLocked(max)
	if n > 0 {
		s.persist()
	}
	return n
}

// pruneLocked removes the oldest entries beyond max (by CreatedAt), deleting
// their blob files. It does not persist; callers must hold s.mu and persist.
func (s *Store) pruneLocked(max int) int {
	if max <= 0 {
		max = maxEntries
	}
	if len(s.idx) <= max {
		return 0
	}
	entries := make([]Entry, 0, len(s.idx))
	for _, e := range s.idx {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].CreatedAt.After(entries[j].CreatedAt) })
	removed := 0
	for _, e := range entries[max:] {
		_ = os.Remove(filepath.Join(s.dir, e.Handle+".txt"))
		delete(s.idx, e.Handle)
		removed++
	}
	return removed
}

// Get returns the full content for a handle. ok is false if the handle is
// unknown or malformed.
func (s *Store) Get(handle string) (content string, ok bool, err error) {
	h := normalizeHandle(handle)
	if !isHexHandle(h) {
		return "", false, nil
	}
	data, readErr := os.ReadFile(filepath.Join(s.dir, h+".txt"))
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return "", false, nil
		}
		return "", false, readErr
	}
	return string(data), true, nil
}

// Query returns only the lines of a stashed blob that match query, trimmed to an
// approximate token budget (len/4). An empty query returns the head within
// budget. This is the "take output by necessary query" read path.
func (s *Store) Query(handle, query string, budget int) (string, error) {
	content, ok, err := s.Get(handle)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("stash %q not found", handle)
	}
	if budget <= 0 {
		budget = defaultBudget
	}
	maxChars := budget * 4

	q := strings.ToLower(strings.TrimSpace(query))
	var b strings.Builder
	added, matched := 0, 0
	for _, line := range strings.Split(content, "\n") {
		if q != "" && !strings.Contains(strings.ToLower(line), q) {
			continue
		}
		matched++
		if b.Len()+len(line)+1 > maxChars {
			continue // over budget: keep counting, stop appending
		}
		b.WriteString(line)
		b.WriteByte('\n')
		added++
	}

	if matched == 0 {
		return fmt.Sprintf("No lines in stash %s match %q.", normalizeHandle(handle), query), nil
	}
	out := b.String()
	if added < matched {
		out += fmt.Sprintf("... +%d more matching lines (narrow the query or raise budget)\n", matched-added)
	}
	return out, nil
}

// List returns all stash entries, most recent first.
func (s *Store) List() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Entry, 0, len(s.idx))
	for _, e := range s.idx {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (s *Store) load() {
	data, err := os.ReadFile(s.manifest)
	if err != nil {
		return
	}
	var entries []Entry
	if json.Unmarshal(data, &entries) != nil {
		return
	}
	for _, e := range entries {
		s.idx[e.Handle] = e
	}
}

// persist writes the manifest. Callers must hold s.mu.
func (s *Store) persist() {
	entries := make([]Entry, 0, len(s.idx))
	for _, e := range s.idx {
		entries = append(entries, e)
	}
	if data, err := json.MarshalIndent(entries, "", "  "); err == nil {
		_ = os.WriteFile(s.manifest, data, 0o644)
	}
}

// normalizeHandle strips an optional "stash://" / "stash:" prefix and lowercases.
func normalizeHandle(h string) string {
	h = strings.TrimSpace(h)
	h = strings.TrimPrefix(h, "stash://")
	h = strings.TrimPrefix(h, "stash:")
	return strings.ToLower(strings.TrimSpace(h))
}

// isHexHandle guards against path traversal: handles are hex content hashes, so
// anything else (slashes, dots, ..) is rejected before touching the filesystem.
func isHexHandle(h string) bool {
	if h == "" {
		return false
	}
	for _, c := range h {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
