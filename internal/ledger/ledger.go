package ledger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const ledgerFile = "session_events.jsonl"

var (
	mu       sync.Mutex
	handles  = make(map[string]*os.File)
)

type Event struct {
	TS      time.Time `json:"ts"`
	Kind    string    `json:"kind"`
	Action  string    `json:"action"`
	Summary string    `json:"summary,omitempty"`
	Handle  string    `json:"handle,omitempty"`
	Tokens  int       `json:"tokens,omitempty"`
	Query   string    `json:"query,omitempty"`
	Source  string    `json:"source,omitempty"`
	Files   int       `json:"files,omitempty"`
	Trigger string    `json:"trigger,omitempty"`
	ChangedFiles []string `json:"changed_files,omitempty"`
}

func Append(repoPath string, e Event) error {
	e.TS = time.Now().UTC()
	dir := filepath.Join(repoPath, ".mycli-fts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ledger mkdir: %w", err)
	}

	mu.Lock()
	f, ok := handles[repoPath]
	if !ok {
		var err error
		f, err = os.OpenFile(filepath.Join(dir, ledgerFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			mu.Unlock()
			return fmt.Errorf("ledger open: %w", err)
		}
		handles[repoPath] = f
	}
	mu.Unlock()

	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("ledger marshal: %w", err)
	}
	line = append(line, '\n')

	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("ledger write: %w", err)
	}
	return nil
}

func Close(repoPath string) {
	mu.Lock()
	defer mu.Unlock()
	if f, ok := handles[repoPath]; ok {
		f.Close()
		delete(handles, repoPath)
	}
}

func CloseAll() {
	mu.Lock()
	defer mu.Unlock()
	for path, f := range handles {
		f.Close()
		delete(handles, path)
	}
}

func ReadAll(repoPath string) ([]Event, error) {
	data, err := os.ReadFile(filepath.Join(repoPath, ".mycli-fts", ledgerFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var events []Event
	for _, line := range splitLines(data) {
		line = trimBOM(line)
		if len(line) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			log.Printf("ledger: skipping malformed line: %v", err)
			continue
		}
		events = append(events, e)
	}
	return events, nil
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			next := make([]byte, i-start)
			copy(next, data[start:i])
			lines = append(lines, next)
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

func trimBOM(line []byte) []byte {
	if len(line) >= 3 && line[0] == 0xEF && line[1] == 0xBB && line[2] == 0xBF {
		return line[3:]
	}
	return line
}

const (
	// IdleThreshold is the gap between events that qualifies as a new "session".
	IdleThreshold = 30 * time.Minute

	// defaultBudget for session_brief output.
	defaultBudget = 300

	// maxChangedFiles caps the number of file paths rendered in a watch event.
	maxChangedFiles = 10
)

type Scope string

const (
	ScopeLast  Scope = "last"
	ScopeToday Scope = "today"
	ScopeAll   Scope = "all"
)

// SessionBrief reads the ledger, filters by scope, renders a compact summary,
// and applies a token budget (oldest events dropped first if over budget).
func SessionBrief(repoPath string, scope Scope, budget int) (string, error) {
	if budget <= 0 {
		budget = defaultBudget
	}
	events, err := ReadAll(repoPath)
	if err != nil {
		return "", fmt.Errorf("reading ledger: %w", err)
	}
	if len(events) == 0 {
		return "No session events recorded for this repository yet.\n", nil
	}

	events = filterByScope(events, scope)
	if len(events) == 0 {
		return "No session events match the requested scope.\n", nil
	}

	var b strings.Builder

	// Build the rendered lines (newest first)
	rendered := renderEvents(events)

	// Count kinds for the header
	kindCount := countByKind(events)

	// Header
	scopeLabel := string(scope)
	if len(events) > 0 {
		first := events[len(events)-1].TS
		last := events[0].TS
		if scope == ScopeLast {
			fmt.Fprintf(&b, "Session %s – %s (", first.Format("15:04"), last.Format("15:04"))
		} else {
			fmt.Fprintf(&b, "%s (", first.Format("2006-01-02"))
		}
		parts := 0
		for _, k := range []string{"fact", "stash", "recall", "index", "watch"} {
			if n := kindCount[k]; n > 0 {
				if parts > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "%d %s", n, plural(k, n))
				parts++
			}
		}
		b.WriteString(")\n")
	} else {
		fmt.Fprintf(&b, "%s (0 events)\n", scopeLabel)
	}

	// Append rendered events, newest first, applying budget
	maxChars := budget * 4
	omitted := 0
	for _, line := range rendered {
		if b.Len()+len(line)+1 > maxChars && b.Len() > 0 {
			omitted++
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if omitted > 0 {
		fmt.Fprintf(&b, "... (%d earlier events omitted, budget %d tok)\n", omitted, budget)
	}

	return b.String(), nil
}

func plural(k string, n int) string {
	if n == 1 {
		return k
	}
	return k + "s"
}

func countByKind(events []Event) map[string]int {
	m := make(map[string]int)
	for _, e := range events {
		m[e.Kind]++
	}
	return m
}

func filterByScope(events []Event, scope Scope) []Event {
	if scope == ScopeAll {
		return reverseEvents(events)
	}

	now := time.Now()
	_ = now

	if scope == ScopeToday {
		year, month, day := now.UTC().Date()
		midnight := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		var out []Event
		for _, e := range events {
			if !e.TS.Before(midnight) {
				out = append(out, e)
			}
		}
		return reverseEvents(out)
	}

	// ScopeLast: events since the most recent gap > IdleThreshold before the latest event
	return latestSession(events)
}

func latestSession(events []Event) []Event {
	if len(events) == 0 {
		return nil
	}

	// Events come from ReadAll in chronological order (oldest first).
	// Find the cutoff: the most recent event that has a gap > IdleThreshold
	// before the next event, working from the end.
	latest := events[len(events)-1].TS
	cut := 0
	for i := len(events) - 2; i >= 0; i-- {
		if latest.Sub(events[i].TS) > IdleThreshold {
			cut = i + 1
			break
		}
	}

	out := make([]Event, len(events)-cut)
	for i, e := range events[cut:] {
		out[len(out)-1-i] = e
	}
	return out
}

func reverseEvents(events []Event) []Event {
	out := make([]Event, len(events))
	for i, e := range events {
		out[len(out)-1-i] = e
	}
	return out
}

func renderEvents(events []Event) []string {
	var lines []string
	for _, e := range events {
		lines = append(lines, renderEvent(e))
	}
	return lines
}

func renderEvent(e Event) string {
	prefix := "+"
	if e.Action == "read" || e.Action == "reindex" || e.Action == "auto_reindex" {
		prefix = "~"
	}

	switch e.Kind {
	case "fact":
		return fmt.Sprintf(`%s fact:   remembered: "%s"`, prefix, e.Summary)
	case "recall":
		src := e.Source
		if src == "" {
			src = "facts"
		}
		return fmt.Sprintf(`%s recall: recalled "%s" from %s`, prefix, e.Query, src)
	case "stash":
		return fmt.Sprintf(`%s stash:  stashed %s (handle: %s, ~%d tok)`, prefix, e.Summary, e.Handle, e.Tokens)
	case "index":
		return fmt.Sprintf(`%s index:  reindexed %d files (trigger: %s)`, prefix, e.Files, e.Trigger)
	case "watch":
		files := e.ChangedFiles
		if len(files) > maxChangedFiles {
			files = files[:maxChangedFiles]
			files = append(files, fmt.Sprintf("+%d more", len(e.ChangedFiles)-maxChangedFiles))
		}
		joined := strings.Join(files, ", ")
		return fmt.Sprintf(`%s watch:  auto-reindexed after file change: %s`, prefix, joined)
	default:
		return fmt.Sprintf(`%s %s: %s %s`, prefix, e.Kind, e.Action, e.Summary)
	}
}
