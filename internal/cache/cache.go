package cache

import (
	"container/list"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// CacheKey identifies a unique retrieval+context combination.
type CacheKey struct {
	RepoHash     string `json:"repo_hash"`
	Query        string `json:"query"`
	Retriever    string `json:"retriever"`
	ContextLevel string `json:"context_level"`
	TokenBudget  int    `json:"token_budget"`
}

// CacheEntry holds the cached retrieval and context results.
type CacheEntry struct {
	ResultsJSON []byte    `json:"results_json"`
	Context     string    `json:"context"`
	Tokens      int       `json:"tokens"`
	CreatedAt   time.Time `json:"created_at"`
}

// CacheStats holds cache performance metrics.
type CacheStats struct {
	Hits    int64   `json:"hits"`
	Misses  int64   `json:"misses"`
	HitRate float64 `json:"hit_rate"`
	Size    int     `json:"size"`
	MaxSize int     `json:"max_size"`
}

type cacheItem struct {
	keyHash string
	entry   *CacheEntry
}

// Cache is an LRU cache with both in-memory and persistent (SQLite) layers.
type Cache struct {
	maxSize int
	entries map[string]*list.Element
	lru     *list.List
	db      *sql.DB
	mu      sync.RWMutex
	hits    int64
	misses  int64
}

// NewCache creates a new LRU cache with SQLite persistence.
func NewCache(repoRoot string, maxSize int) (*Cache, error) {
	if maxSize <= 0 {
		maxSize = 100
	}

	dbDir := filepath.Join(repoRoot, ".mycli-fts")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}
	dbPath := filepath.Join(dbDir, "cache.db")

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open cache DB: %w", err)
	}

	if err := createCacheTables(db); err != nil {
		db.Close()
		return nil, err
	}

	c := &Cache{
		maxSize: maxSize,
		entries: make(map[string]*list.Element),
		lru:     list.New(),
		db:      db,
	}

	// Load existing entries from SQLite into memory LRU
	c.loadFromDB()

	return c, nil
}

func createCacheTables(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS cache_entries (
			key_hash TEXT PRIMARY KEY,
			repo_hash TEXT NOT NULL,
			query TEXT NOT NULL,
			retriever TEXT NOT NULL,
			context_level TEXT NOT NULL,
			token_budget INTEGER NOT NULL,
			results_json BLOB,
			context_text TEXT,
			tokens INTEGER,
			created_at TIMESTAMP,
			last_accessed TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_cache_repo ON cache_entries(repo_hash);
	`)
	return err
}

// RepoHash computes a SHA256 hash of the repository root path.
func RepoHash(repoRoot string) string {
	h := sha256.Sum256([]byte(repoRoot))
	return hex.EncodeToString(h[:8])
}

func (k CacheKey) hash() string {
	raw := fmt.Sprintf("%s|%s|%s|%s|%d", k.RepoHash, k.Query, k.Retriever, k.ContextLevel, k.TokenBudget)
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// Get retrieves an entry from the cache. Returns nil, false on miss.
func (c *Cache) Get(key CacheKey) (*CacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	keyHash := key.hash()
	if elem, ok := c.entries[keyHash]; ok {
		c.lru.MoveToFront(elem)
		c.hits++
		item := elem.Value.(*cacheItem)

		// Update last_accessed in DB
		_, _ = c.db.Exec("UPDATE cache_entries SET last_accessed = ? WHERE key_hash = ?", time.Now(), keyHash)

		return item.entry, true
	}

	c.misses++
	return nil, false
}

// Put adds an entry to the cache, evicting the least-recently-used if at capacity.
func (c *Cache) Put(key CacheKey, entry *CacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	keyHash := key.hash()

	// Update existing entry
	if elem, ok := c.entries[keyHash]; ok {
		c.lru.MoveToFront(elem)
		elem.Value.(*cacheItem).entry = entry
		c.persistEntry(keyHash, key, entry)
		return
	}

	// Evict if at capacity
	for c.lru.Len() >= c.maxSize {
		back := c.lru.Back()
		if back == nil {
			break
		}
		evicted := c.lru.Remove(back).(*cacheItem)
		delete(c.entries, evicted.keyHash)
		_, _ = c.db.Exec("DELETE FROM cache_entries WHERE key_hash = ?", evicted.keyHash)
	}

	// Add new entry
	item := &cacheItem{keyHash: keyHash, entry: entry}
	elem := c.lru.PushFront(item)
	c.entries[keyHash] = elem
	c.persistEntry(keyHash, key, entry)
}

// Invalidate removes all cache entries for a given repo hash.
func (c *Cache) Invalidate(repoHash string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Invalidate matching entries in memory using keys loaded from DB below

	// Query DB for entries belonging to this repo
	rows, err := c.db.Query("SELECT key_hash FROM cache_entries WHERE repo_hash = ?", repoHash)
	if err == nil {
		defer rows.Close()
		repoKeys := make(map[string]bool)
		for rows.Next() {
			var kh string
			if rows.Scan(&kh) == nil {
				repoKeys[kh] = true
			}
		}

		// Remove only repo-specific entries from memory
		for keyHash, elem := range c.entries {
			if repoKeys[keyHash] {
				c.lru.Remove(elem)
				delete(c.entries, keyHash)
			}
		}
	}

	// Remove from DB
	_, _ = c.db.Exec("DELETE FROM cache_entries WHERE repo_hash = ?", repoHash)
}

// Stats returns cache performance metrics.
func (c *Cache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}

	return CacheStats{
		Hits:    c.hits,
		Misses:  c.misses,
		HitRate: hitRate,
		Size:    c.lru.Len(),
		MaxSize: c.maxSize,
	}
}

// Close closes the cache database.
func (c *Cache) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

func (c *Cache) persistEntry(keyHash string, key CacheKey, entry *CacheEntry) {
	now := time.Now()
	_, _ = c.db.Exec(`
		INSERT OR REPLACE INTO cache_entries
			(key_hash, repo_hash, query, retriever, context_level, token_budget,
			 results_json, context_text, tokens, created_at, last_accessed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, keyHash, key.RepoHash, key.Query, key.Retriever, key.ContextLevel, key.TokenBudget,
		entry.ResultsJSON, entry.Context, entry.Tokens, entry.CreatedAt, now)
}

func (c *Cache) loadFromDB() {
	rows, err := c.db.Query(`
		SELECT key_hash, results_json, context_text, tokens, created_at
		FROM cache_entries
		ORDER BY last_accessed DESC
		LIMIT ?
	`, c.maxSize)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var keyHash string
		var resultsJSON []byte
		var contextText string
		var tokens int
		var createdAt time.Time

		if err := rows.Scan(&keyHash, &resultsJSON, &contextText, &tokens, &createdAt); err != nil {
			continue
		}

		entry := &CacheEntry{
			ResultsJSON: resultsJSON,
			Context:     contextText,
			Tokens:      tokens,
			CreatedAt:   createdAt,
		}
		item := &cacheItem{keyHash: keyHash, entry: entry}
		elem := c.lru.PushBack(item)
		c.entries[keyHash] = elem
	}
}

// MarshalResults serializes retrieval results to JSON for caching.
func MarshalResults(results interface{}) []byte {
	data, err := json.Marshal(results)
	if err != nil {
		return nil
	}
	return data
}

// UnmarshalResults deserializes cached retrieval results.
func UnmarshalResults(data []byte, target interface{}) error {
	return json.Unmarshal(data, target)
}
