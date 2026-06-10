package repository

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type RepositoryState int

const (
	StateUnindexed RepositoryState = iota
	StateStale
	StateReady
)

func (s RepositoryState) String() string {
	switch s {
	case StateUnindexed:
		return "unindexed"
	case StateStale:
		return "stale"
	case StateReady:
		return "ready"
	default:
		return "unknown"
	}
}

type StateInfo struct {
	State        RepositoryState
	IndexedFiles int
	ChangedFiles int
	DeletedFiles int
	TotalFiles   int
	IndexAge     time.Duration
	DbPath       string
}

func DetectRepositoryState(repoRoot string) (*StateInfo, error) {
	info := &StateInfo{}

	dbPath := findSymbolDB(repoRoot)
	if dbPath == "" {
		info.State = StateUnindexed
		return info, nil
	}
	info.DbPath = dbPath

	indexAge, err := getIndexAge(dbPath)
	if err == nil {
		info.IndexAge = indexAge
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		info.State = StateUnindexed
		return info, nil
	}
	defer db.Close()

	row := db.QueryRow(`SELECT COUNT(*) FROM symbol_files`)
	if err := row.Scan(&info.IndexedFiles); err != nil {
		info.State = StateUnindexed
		return info, nil
	}

	if info.IndexedFiles == 0 {
		info.State = StateUnindexed
		return info, nil
	}

	currentFiles := make(map[string]string)
	_ = filepath.Walk(repoRoot, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		if fi.IsDir() && shouldSkipDirState(path) {
			return filepath.SkipDir
		}
		if fi.IsDir() || fi.Size() == 0 {
			return nil
		}
		if !isIndexableExt(path) {
			return nil
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		h := sha256.Sum256(data)
		currentFiles[rel] = fmt.Sprintf("%x", h)
		return nil
	})
	info.TotalFiles = len(currentFiles)

	rows, err := db.Query(`SELECT file_path, file_hash FROM symbol_files`)
	if err != nil {
		info.State = StateReady
		return info, nil
	}
	defer rows.Close()

	for rows.Next() {
		var filePath, fileHash string
		if err := rows.Scan(&filePath, &fileHash); err != nil {
			continue
		}
		if currentHash, exists := currentFiles[filePath]; exists {
			if currentHash != fileHash {
				info.ChangedFiles++
			}
		} else if fileHash != "" {
			info.DeletedFiles++
		}
	}

	if info.ChangedFiles > 0 || info.DeletedFiles > 0 {
		info.State = StateStale
	} else {
		info.State = StateReady
	}

	return info, nil
}

func findSymbolDB(repoRoot string) string {
	ftsDir := filepath.Join(repoRoot, ".mycli-fts")
	dirents, err := os.ReadDir(ftsDir)
	if err != nil {
		return ""
	}
	for _, de := range dirents {
		if !de.IsDir() && len(de.Name()) > 12 && de.Name()[:8] == "symbols_" && filepath.Ext(de.Name()) == ".db" {
			return filepath.Join(ftsDir, de.Name())
		}
	}
	return ""
}

func getIndexAge(dbPath string) (time.Duration, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return 0, err
	}
	defer db.Close()

	var ts string
	row := db.QueryRow(`SELECT last_indexed FROM symbol_files ORDER BY last_indexed DESC LIMIT 1`)
	if err := row.Scan(&ts); err != nil {
		return 0, err
	}
	t, err := time.Parse("2006-01-02 15:04:05.999999999-07:00", ts)
	if err != nil {
		t, err = time.Parse(time.RFC3339, ts)
		if err != nil {
			return 0, err
		}
	}
	return time.Since(t), nil
}

func shouldSkipDirState(path string) bool {
	base := filepath.Base(path)
	switch base {
	case ".git", "node_modules", "vendor", ".venv", "venv",
		"__pycache__", ".next", "dist", "build", "target",
		".idea", ".vscode", ".DS_Store", ".mycli-fts":
		return true
	}
	return false
}

func isIndexableExt(path string) bool {
	ext := filepath.Ext(path)
	switch ext {
	case ".go", ".py", ".js", ".jsx", ".ts", ".tsx", ".mjs",
		".rs", ".rb", ".java", ".c", ".cpp", ".h", ".hpp",
		".cs", ".php", ".swift", ".kt", ".scala", ".sh",
		".sql", ".html", ".css", ".json", ".yaml", ".yml",
		".md", ".toml", ".lua", ".zig":
		return true
	}
	return false
}

func EnsureRepositoryReady(repoRoot string, mode string) (*StateInfo, bool, error) {
	info, err := DetectRepositoryState(repoRoot)
	if err != nil {
		return info, false, fmt.Errorf("detecting repository state: %w", err)
	}

	if info.State == StateReady {
		return info, true, nil
	}

	if mode == "agent" {
		return info, true, nil
	}

	return info, false, nil
}

func NeedsAutoIndex(mode string, state RepositoryState) bool {
	if mode == "agent" {
		return state != StateReady
	}
	return false
}

func NeedsPrompt(mode string, state RepositoryState) bool {
	if mode == "agent" {
		return false
	}
	return state == StateUnindexed || state == StateStale
}
