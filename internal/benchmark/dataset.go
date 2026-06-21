// Package benchmark runs the ground-truth datasets under benchmarks/ against a
// checked-out repository, exercising the same retrieval+compression path as the
// search_code MCP tool, and scores file-hit accuracy, routing accuracy, and
// tokens per query.
package benchmark

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Task is one ground-truth benchmark entry. It tolerates both dataset schemas:
// the per-repo schema (expected_files array) and the costwise schema
// (expected_file singular).
type Task struct {
	ID                string   `json:"id"`
	Query             string   `json:"query"`
	Category          string   `json:"category"`
	ExpectedRetriever string   `json:"expected_retriever"`
	ExpectedFiles     []string `json:"expected_files"`
	ExpectedFile      string   `json:"expected_file"`
	ExpectedSymbols   []string `json:"expected_symbols"`
	ExpectedKeywords  []string `json:"expected_keywords"`
	Difficulty        string   `json:"difficulty"`

	// SourceDataset records which dataset dir this task came from (not in JSON).
	SourceDataset string `json:"-"`
}

// Files returns the expected files from whichever schema variant was used,
// dropping blanks. Several categories (reference/concept/flow) intentionally
// ship expected_files: [""] — they are scored by symbol, not file.
func (t Task) Files() []string {
	var files []string
	for _, f := range append(append([]string{}, t.ExpectedFiles...), t.ExpectedFile) {
		if strings.TrimSpace(f) != "" {
			files = append(files, f)
		}
	}
	return files
}

// Symbols returns the non-blank expected symbols for symbol-level scoring.
func (t Task) Symbols() []string {
	var syms []string
	for _, s := range t.ExpectedSymbols {
		if strings.TrimSpace(s) != "" {
			syms = append(syms, s)
		}
	}
	return syms
}

// SampleTasks deterministically selects up to n tasks spread evenly across the
// (category-sorted) list, so a capped run still covers multiple categories.
func SampleTasks(tasks []Task, n int) []Task {
	if n <= 0 || len(tasks) <= n {
		return tasks
	}
	out := make([]Task, 0, n)
	stride := float64(len(tasks)) / float64(n)
	for i := 0; i < n; i++ {
		out = append(out, tasks[int(float64(i)*stride)])
	}
	return out
}

// LoadDir loads every *.json task array in dir (non-recursive), skipping reports
// and any file that does not parse as a task array (e.g. config files).
func LoadDir(dir string) ([]Task, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var tasks []Task
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var batch []Task
		if err := json.Unmarshal(raw, &batch); err != nil {
			// Not a task array — skip quietly.
			continue
		}
		for i := range batch {
			if batch[i].Query == "" {
				continue
			}
			batch[i].SourceDataset = filepath.Base(dir)
			tasks = append(tasks, batch[i])
		}
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].Category != tasks[j].Category {
			return tasks[i].Category < tasks[j].Category
		}
		return tasks[i].ID < tasks[j].ID
	})
	return tasks, nil
}
