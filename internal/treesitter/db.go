package treesitter

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type SymbolDB struct {
	db       *sql.DB
	repoRoot string
}

func NewSymbolDB(repoRoot string) (*SymbolDB, error) {
	hash := sha256.Sum256([]byte(repoRoot))
	dbName := fmt.Sprintf("symbols_%x.db", hash[:8])
	dbDir := filepath.Join(repoRoot, ".mycli-fts")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dbDir, dbName)

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open symbol DB: %w", err)
	}

	if err := createSymbolTables(db); err != nil {
		db.Close()
		return nil, err
	}

	return &SymbolDB{db: db, repoRoot: repoRoot}, nil
}

const schemaVersion = 4

func createSymbolTables(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER NOT NULL DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS symbols (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			kind TEXT NOT NULL,
			language TEXT NOT NULL DEFAULT '',
			file TEXT NOT NULL,
			start_line INTEGER NOT NULL DEFAULT 0,
			end_line INTEGER NOT NULL DEFAULT 0,
			signature TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_symbols_name ON symbols(name);
		CREATE INDEX IF NOT EXISTS idx_symbols_kind ON symbols(kind);
		CREATE INDEX IF NOT EXISTS idx_symbols_file ON symbols(file);
		CREATE INDEX IF NOT EXISTS idx_symbols_name_kind ON symbols(name, kind);

		CREATE TABLE IF NOT EXISTS symbol_files (
			file_path TEXT PRIMARY KEY,
			file_hash TEXT NOT NULL DEFAULT '',
			last_indexed TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS references_t (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol_name TEXT NOT NULL,
			file TEXT NOT NULL,
			line INTEGER NOT NULL DEFAULT 0,
			col INTEGER NOT NULL DEFAULT 0,
			ref_type TEXT NOT NULL DEFAULT 'reference',
			context TEXT NOT NULL DEFAULT ''
		);

		CREATE INDEX IF NOT EXISTS idx_refs_name ON references_t(symbol_name);
		CREATE INDEX IF NOT EXISTS idx_refs_file ON references_t(file);
		CREATE INDEX IF NOT EXISTS idx_refs_type ON references_t(ref_type);
		CREATE INDEX IF NOT EXISTS idx_refs_name_type ON references_t(symbol_name, ref_type);

		CREATE TABLE IF NOT EXISTS call_edges (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			caller_name TEXT NOT NULL,
			caller_file TEXT NOT NULL DEFAULT '',
			callee_name TEXT NOT NULL,
			file TEXT NOT NULL,
			line INTEGER NOT NULL DEFAULT 0,
			language TEXT NOT NULL DEFAULT ''
		);

		CREATE INDEX IF NOT EXISTS idx_calls_callee ON call_edges(callee_name);
		CREATE INDEX IF NOT EXISTS idx_calls_caller ON call_edges(caller_name);
		CREATE INDEX IF NOT EXISTS idx_calls_file ON call_edges(file);
	`)
	if err != nil {
		return err
	}

	var ver int
	_ = db.QueryRow("SELECT version FROM schema_version").Scan(&ver)
	if ver != schemaVersion {
		if _, err := db.Exec("DELETE FROM schema_version"); err != nil {
			return err
		}
		if _, err := db.Exec("INSERT INTO schema_version (version) VALUES (?)", schemaVersion); err != nil {
			return err
		}
		if _, err := db.Exec("DELETE FROM symbols"); err != nil {
			return err
		}
		if _, err := db.Exec("DELETE FROM symbol_files"); err != nil {
			return err
		}
		if _, err := db.Exec("DELETE FROM references_t"); err != nil {
			return err
		}
		if _, err := db.Exec("DELETE FROM call_edges"); err != nil {
			return err
		}
	}
	return nil
}

func (s *SymbolDB) Close() error {
	return s.db.Close()
}

func symbolID(name, kind, file string, line int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%d", name, kind, file, line)))
	return hex.EncodeToString(h[:8])
}

func (s *SymbolDB) StoreSymbols(symbols []Symbol) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO symbols (id, name, kind, language, file, start_line, end_line, signature, content, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, sym := range symbols {
		id := symbolID(sym.Name, string(sym.Kind), sym.File, sym.StartLine)
		if _, err := stmt.Exec(id, sym.Name, string(sym.Kind), sym.Language, sym.File, sym.StartLine, sym.EndLine, sym.Signature, sym.Content); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SymbolDB) ClearFile(filePath string) error {
	_, err := s.db.Exec("DELETE FROM symbols WHERE file = ?", filePath)
	return err
}

func (s *SymbolDB) MarkFileIndexed(filePath, hash string) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO symbol_files (file_path, file_hash, last_indexed)
		VALUES (?, ?, ?)
	`, filePath, hash, time.Now())
	return err
}

func (s *SymbolDB) GetFileHash(filePath string) string {
	var h string
	_ = s.db.QueryRow("SELECT file_hash FROM symbol_files WHERE file_path = ?", filePath).Scan(&h)
	return h
}

func (s *SymbolDB) StoreReferences(refs []Reference) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO references_t (symbol_name, file, line, col, ref_type, context)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, ref := range refs {
		refType := ref.RefType.String()
		if _, err := stmt.Exec(ref.SymbolName, ref.File, ref.Line, ref.Column, refType, ref.Context); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SymbolDB) ClearFileReferences(filePath string) error {
	_, err := s.db.Exec("DELETE FROM references_t WHERE file = ?", filePath)
	return err
}

func (s *SymbolDB) SearchReferences(symbolName string) ([]Reference, error) {
	rows, err := s.db.Query(`
		SELECT symbol_name, file, line, col, ref_type, context
		FROM references_t
		WHERE symbol_name = ?
		ORDER BY ref_type, file, line
	`, symbolName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []Reference
	for rows.Next() {
		var ref Reference
		var refTypeStr string
		if err := rows.Scan(&ref.SymbolName, &ref.File, &ref.Line, &ref.Column, &refTypeStr, &ref.Context); err != nil {
			continue
		}
		switch refTypeStr {
		case "definition":
			ref.RefType = RefDefinition
		case "reference":
			ref.RefType = RefReference
		case "import":
			ref.RefType = RefImport
		case "export":
			ref.RefType = RefExport
		}
		refs = append(refs, ref)
	}

	return refs, nil
}

func (s *SymbolDB) SearchReferencesLike(partial string) ([]Reference, error) {
	rows, err := s.db.Query(`
		SELECT symbol_name, file, line, col, ref_type, context
		FROM references_t
		WHERE symbol_name LIKE ?
		ORDER BY ref_type, file, line
	`, "%"+partial+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []Reference
	for rows.Next() {
		var ref Reference
		var refTypeStr string
		if err := rows.Scan(&ref.SymbolName, &ref.File, &ref.Line, &ref.Column, &refTypeStr, &ref.Context); err != nil {
			continue
		}
		switch refTypeStr {
		case "definition":
			ref.RefType = RefDefinition
		case "reference":
			ref.RefType = RefReference
		case "import":
			ref.RefType = RefImport
		case "export":
			ref.RefType = RefExport
		}
		refs = append(refs, ref)
	}

	return refs, nil
}

func (s *SymbolDB) StoreCallEdges(edges []CallEdge) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO call_edges (caller_name, caller_file, callee_name, file, line, language)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range edges {
		if _, err := stmt.Exec(e.CallerName, e.CallerFile, e.CalleeName, e.File, e.Line, e.Language); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SymbolDB) ClearFileCallEdges(filePath string) error {
	_, err := s.db.Exec("DELETE FROM call_edges WHERE file = ?", filePath)
	return err
}

func (s *SymbolDB) SearchCallEdgesByCaller(callerName string) ([]CallEdge, error) {
	rows, err := s.db.Query(`
		SELECT id, caller_name, caller_file, callee_name, file, line, language
		FROM call_edges
		WHERE caller_name = ?
		ORDER BY callee_name, file, line
	`, callerName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []CallEdge
	for rows.Next() {
		var e CallEdge
		if err := rows.Scan(&e.ID, &e.CallerName, &e.CallerFile, &e.CalleeName, &e.File, &e.Line, &e.Language); err != nil {
			continue
		}
		edges = append(edges, e)
	}
	return edges, nil
}

func (s *SymbolDB) SearchCallEdges(calleeName string) ([]CallEdge, error) {
	rows, err := s.db.Query(`
		SELECT id, caller_name, caller_file, callee_name, file, line, language
		FROM call_edges
		WHERE callee_name = ?
		ORDER BY caller_name, file, line
	`, calleeName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []CallEdge
	for rows.Next() {
		var e CallEdge
		if err := rows.Scan(&e.ID, &e.CallerName, &e.CallerFile, &e.CalleeName, &e.File, &e.Line, &e.Language); err != nil {
			continue
		}
		edges = append(edges, e)
	}
	return edges, nil
}

func (s *SymbolDB) SearchCallEdgesLike(partial string) ([]CallEdge, error) {
	rows, err := s.db.Query(`
		SELECT id, caller_name, caller_file, callee_name, file, line, language
		FROM call_edges
		WHERE callee_name LIKE ?
		ORDER BY caller_name, file, line
	`, "%"+partial+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []CallEdge
	for rows.Next() {
		var e CallEdge
		if err := rows.Scan(&e.ID, &e.CallerName, &e.CallerFile, &e.CalleeName, &e.File, &e.Line, &e.Language); err != nil {
			continue
		}
		edges = append(edges, e)
	}
	return edges, nil
}

func (s *SymbolDB) GetCallEdgeCount() int {
	var count int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM call_edges").Scan(&count)
	return count
}

func (s *SymbolDB) GetReferenceCount() int {
	var count int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM references_t").Scan(&count)
	return count
}

func (s *SymbolDB) Search(query string, limit int) ([]SymbolMatch, error) {
	if limit <= 0 {
		limit = 20
	}

	q := `SELECT name, kind, language, file, start_line, end_line, signature
		FROM symbols WHERE (
			name LIKE ? OR
			name LIKE ? OR
			signature LIKE ?
		)
		ORDER BY
			CASE
				WHEN name LIKE ? THEN 0
				WHEN name LIKE ? THEN 1
				ELSE 2
			END,
			start_line ASC
		LIMIT ?`

	exact := query
	prefix := query + "%"
	partial := "%" + query + "%"

	rows, err := s.db.Query(q, exact, prefix, partial, exact, prefix, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []SymbolMatch
	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(&sym.Name, &sym.Kind, &sym.Language, &sym.File, &sym.StartLine, &sym.EndLine, &sym.Signature); err != nil {
			continue
		}
		score := computeSymbolScore(query, sym)
		matches = append(matches, SymbolMatch{
			Symbol: sym,
			Score:  score,
		})
	}

	return matches, nil
}

func (s *SymbolDB) SearchByKind(kind SymbolKind, limit int) ([]SymbolMatch, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.Query(`
		SELECT name, kind, language, file, start_line, end_line, signature
		FROM symbols WHERE kind = ?
		ORDER BY file, start_line ASC
		LIMIT ?
	`, string(kind), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []SymbolMatch
	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(&sym.Name, &sym.Kind, &sym.Language, &sym.File, &sym.StartLine, &sym.EndLine, &sym.Signature); err != nil {
			continue
		}
		matches = append(matches, SymbolMatch{
			Symbol: sym,
			Score:  0.8,
		})
	}

	return matches, nil
}

func (s *SymbolDB) GetSymbolCount() int {
	var count int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM symbols").Scan(&count)
	return count
}

// GetSymbolRange returns the start and end line for a symbol by name and file.
func (s *SymbolDB) GetSymbolRange(name, file string) (int, int, error) {
	var startLine, endLine int
	err := s.db.QueryRow(`
		SELECT start_line, end_line FROM symbols
		WHERE name = ? AND file = ?
		ORDER BY start_line ASC LIMIT 1
	`, name, file).Scan(&startLine, &endLine)
	if err != nil {
		return 0, 0, err
	}
	return startLine, endLine, nil
}

// GetFileSymbols returns all symbols for a given file path.
func (s *SymbolDB) GetFileSymbols(filePath string) ([]Symbol, error) {
	rows, err := s.db.Query(`
		SELECT name, kind, language, file, start_line, end_line, signature
		FROM symbols WHERE file = ?
		ORDER BY start_line ASC
	`, filePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var symbols []Symbol
	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(&sym.Name, &sym.Kind, &sym.Language, &sym.File, &sym.StartLine, &sym.EndLine, &sym.Signature); err != nil {
			continue
		}
		symbols = append(symbols, sym)
	}
	return symbols, nil
}

// GetAllFiles returns all indexed file paths.
func (s *SymbolDB) GetAllFiles() ([]string, error) {
	rows, err := s.db.Query("SELECT file_path FROM symbol_files ORDER BY file_path")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			continue
		}
		files = append(files, f)
	}
	return files, nil
}

// GetFilesByHash returns a map of file_path -> file_hash for all indexed files.
func (s *SymbolDB) GetFilesByHash() (map[string]string, error) {
	rows, err := s.db.Query("SELECT file_path, file_hash FROM symbol_files")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var path, hash string
		if err := rows.Scan(&path, &hash); err != nil {
			continue
		}
		result[path] = hash
	}
	return result, nil
}

func computeSymbolScore(query string, sym Symbol) float64 {
	q := strings.ToLower(query)
	name := strings.ToLower(sym.Name)

	if name == q {
		return 1.0
	}

	parts := splitCamel(sym.Name)
	qParts := splitCamel(query)

	matchCount := 0
	for _, qp := range qParts {
		qpLower := strings.ToLower(qp)
		for _, np := range parts {
			if strings.ToLower(np) == qpLower {
				matchCount++
				break
			}
		}
	}

	if len(qParts) > 0 {
		ratio := float64(matchCount) / float64(len(qParts))
		if stringsContains(name, q) {
			return 0.8 + ratio*0.2
		}
		return ratio * 0.7
	}

	if stringsContains(name, q) {
		return 0.6
	}

	return 0.3
}

func splitCamel(s string) []string {
	var parts []string
	var cur []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' && len(cur) > 0 {
			parts = append(parts, string(cur))
			cur = []byte{c}
		} else {
			cur = append(cur, c)
		}
	}
	if len(cur) > 0 {
		parts = append(parts, string(cur))
	}
	return parts
}
