// Package discovery_memory caches tool discovery results (grep, glob, read_file) with TTL and confidence scores.
package discovery_memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ToolResult is a single cached discovery result stored as JSON.
type ToolResult struct {
	Query      string
	Tool       string
	Files      []string // discovered file paths
	Snippets   []string // short text snippets from the files
	Confidence float64
	CreatedAt  time.Time
	TTL        time.Duration
}

// DiscoveryMemory wraps the SQLite DB for tool discovery caching.
type DiscoveryMemory struct {
	db *sql.DB
}

// Init opens (or creates) the discovery memory DB.
func Init(dbPath string) (*DiscoveryMemory, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating dir for discovery memory: %w", err)
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening discovery memory DB: %w", err)
	}
	dm := &DiscoveryMemory{db: db}
	if err := dm.migrate(); err != nil {
		return nil, err
	}
	return dm, nil
}

func (dm *DiscoveryMemory) migrate() error {
	_, err := dm.db.Exec(`CREATE TABLE IF NOT EXISTS tool_discoveries (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		query      TEXT NOT NULL,
		tool       TEXT NOT NULL,
		result     TEXT NOT NULL,
		confidence REAL NOT NULL DEFAULT 1.0,
		ttl_secs   INTEGER NOT NULL DEFAULT 86400,
		created_at TEXT NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("migration error: %w", err)
	}
	_, _ = dm.db.Exec(`CREATE INDEX IF NOT EXISTS idx_discoveries_query_tool ON tool_discoveries(query, tool)`)
	return nil
}

// Store saves a tool discovery result. TTL defaults to 24h if zero.
func (dm *DiscoveryMemory) Store(query, tool string, files, snippets []string, confidence float64, ttl time.Duration) error {
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	payload, err := json.Marshal(map[string][]string{"files": files, "snippets": snippets})
	if err != nil {
		return err
	}
	_, err = dm.db.Exec(
		`INSERT INTO tool_discoveries(query, tool, result, confidence, ttl_secs, created_at) VALUES(?,?,?,?,?,?)`,
		query, tool, string(payload), confidence, int64(ttl.Seconds()), time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// Lookup retrieves a non-expired discovery result for the given query+tool combination.
// Returns nil, false, nil when no valid entry exists.
func (dm *DiscoveryMemory) Lookup(query, tool string) (*ToolResult, bool, error) {
	rows, err := dm.db.Query(
		`SELECT result, confidence, ttl_secs, created_at FROM tool_discoveries WHERE query=? AND tool=? ORDER BY id DESC LIMIT 1`,
		query, tool,
	)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, false, nil
	}
	var resultJSON string
	var confidence float64
	var ttlSecs int64
	var createdAtStr string
	if err := rows.Scan(&resultJSON, &confidence, &ttlSecs, &createdAtStr); err != nil {
		return nil, false, err
	}
	createdAt, err := time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return nil, false, err
	}
	if time.Since(createdAt) > time.Duration(ttlSecs)*time.Second {
		// Expired — clean up in the background
		go func() {
			_, _ = dm.db.Exec(`DELETE FROM tool_discoveries WHERE query=? AND tool=?`, query, tool)
		}()
		return nil, false, nil
	}
	var payload map[string][]string
	if err := json.Unmarshal([]byte(resultJSON), &payload); err != nil {
		return nil, false, err
	}
	tr := &ToolResult{
		Query:      query,
		Tool:       tool,
		Files:      payload["files"],
		Snippets:   payload["snippets"],
		Confidence: confidence,
		CreatedAt:  createdAt,
		TTL:        time.Duration(ttlSecs) * time.Second,
	}
	return tr, true, nil
}

// Purge deletes all expired entries.
func (dm *DiscoveryMemory) Purge() error {
	_, err := dm.db.Exec(`DELETE FROM tool_discoveries WHERE CAST(strftime('%s','now') AS INTEGER) - CAST(strftime('%s', created_at) AS INTEGER) > ttl_secs`)
	return err
}

// Close releases DB resources.
func (dm *DiscoveryMemory) Close() error {
	return dm.db.Close()
}
