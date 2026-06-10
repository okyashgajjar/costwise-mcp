package architecture

import (
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

const schemaVersion = 1

type DB struct {
	root string
	db   *sql.DB
	mu   sync.Mutex
}

func NewDB(repoRoot string) (*DB, error) {
	indexDir := filepath.Join(repoRoot, ".mycli")
	if err := os.MkdirAll(indexDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir index dir: %w", err)
	}
	dbPath := filepath.Join(indexDir, "architecture.db")

	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL")
	if err != nil {
		return nil, fmt.Errorf("open architecture db: %w", err)
	}
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping architecture db: %w", err)
	}

	d := &DB{root: repoRoot, db: conn}
	if err := d.initSchema(); err != nil {
		conn.Close()
		return nil, err
	}
	return d, nil
}

func (d *DB) initSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS schema_meta (key TEXT PRIMARY KEY, value TEXT)`,
		`CREATE TABLE IF NOT EXISTS module_summaries (
			file_path TEXT PRIMARY KEY,
			language TEXT NOT NULL,
			classes TEXT,
			functions TEXT,
			imports TEXT,
			exports TEXT,
			topics TEXT,
			description TEXT,
			content_hash TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_topics ON module_summaries(topics)`,
		`CREATE INDEX IF NOT EXISTS idx_classes ON module_summaries(classes)`,
	}
	for _, s := range stmts {
		if _, err := d.db.Exec(s); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}
	_, err := d.db.Exec(`INSERT OR REPLACE INTO schema_meta(key, value) VALUES('version', ?)`, fmt.Sprintf("%d", schemaVersion))
	return err
}

func (d *DB) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

func (d *DB) NeedsReindex(filePath, contentHash string) (bool, error) {
	var existing string
	err := d.db.QueryRow(`SELECT content_hash FROM module_summaries WHERE file_path = ?`, filePath).Scan(&existing)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return true, err
	}
	return existing != contentHash, nil
}

func (d *DB) Store(summary *ModuleSummary, contentHash string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	classesJSON, _ := json.Marshal(summary.Classes)
	functionsJSON, _ := json.Marshal(summary.Functions)
	importsJSON, _ := json.Marshal(summary.Imports)
	exportsJSON, _ := json.Marshal(summary.Exports)
	topicsJSON, _ := json.Marshal(summary.Topics)

	_, err := d.db.Exec(`
		INSERT OR REPLACE INTO module_summaries
		(file_path, language, classes, functions, imports, exports, topics, description, content_hash, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		summary.FilePath, summary.Language,
		string(classesJSON), string(functionsJSON), string(importsJSON), string(exportsJSON), string(topicsJSON),
		summary.Description, contentHash, time.Now().Unix())
	return err
}

func (d *DB) LoadByFile(filePath string) (*ModuleSummary, bool) {
	row := d.db.QueryRow(`SELECT file_path, language, classes, functions, imports, exports, topics, description FROM module_summaries WHERE file_path = ?`, filePath)
	s := &ModuleSummary{}
	var classes, functions, imports, exports, topics string
	if err := row.Scan(&s.FilePath, &s.Language, &classes, &functions, &imports, &exports, &topics, &s.Description); err != nil {
		return nil, false
	}
	if err := json.Unmarshal([]byte(classes), &s.Classes); err != nil {
		return nil, false
	}
	if err := json.Unmarshal([]byte(functions), &s.Functions); err != nil {
		return nil, false
	}
	if err := json.Unmarshal([]byte(imports), &s.Imports); err != nil {
		return nil, false
	}
	if err := json.Unmarshal([]byte(exports), &s.Exports); err != nil {
		return nil, false
	}
	if err := json.Unmarshal([]byte(topics), &s.Topics); err != nil {
		return nil, false
	}
	return s, true
}

func (d *DB) LoadAll() ([]*ModuleSummary, error) {
	rows, err := d.db.Query(`SELECT file_path, language, classes, functions, imports, exports, topics, description FROM module_summaries`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*ModuleSummary
	for rows.Next() {
		s := &ModuleSummary{}
		var classes, functions, imports, exports, topics string
		if err := rows.Scan(&s.FilePath, &s.Language, &classes, &functions, &imports, &exports, &topics, &s.Description); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(classes), &s.Classes); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(functions), &s.Functions); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(imports), &s.Imports); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(exports), &s.Exports); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(topics), &s.Topics); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (d *DB) Count() (int, error) {
	var n int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM module_summaries`).Scan(&n)
	return n, err
}

func HashContent(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:16])
}
