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
	mu      sync.Mutex
	handles = make(map[string]*os.File)
)

type Event struct {
	TS           time.Time `json:"ts"`
	Kind         string    `json:"kind"`
	Action       string    `json:"action"`
	Summary      string    `json:"summary,omitempty"`
	Handle       string    `json:"handle,omitempty"`
	Tokens       int       `json:"tokens,omitempty"`
	Query        string    `json:"query,omitempty"`
	Source       string    `json:"source,omitempty"`
	Files        int       `json:"files,omitempty"`
	Trigger      string    `json:"trigger,omitempty"`
	ChangedFiles []string  `json:"changed_files,omitempty"`
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
// If sessions > 0, returns the last N sessions (overrides scope for "last" style queries).
func SessionBrief(repoPath string, scope Scope, budget int, sessions ...int) (string, error) {
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

	n := 0
	if len(sessions) > 0 {
		n = sessions[0]
	}

	events = filterByScope(events, scope, n)
	if len(events) == 0 {
		return "No session events match the requested scope.\n", nil
	}

	var b strings.Builder

	// Build the rendered lines (newest first)
	rendered := renderEvents(events)

	// Count kinds for the header
	kindCount := countByKind(events)

	// Header
	if len(events) > 0 {
		first := events[len(events)-1].TS
		last := events[0].TS
		if n > 1 {
			fmt.Fprintf(&b, "Last %d sessions – %s to %s (", n, first.Format("2006-01-02 15:04"), last.Format("2006-01-02 15:04"))
		} else if scope == ScopeLast || n == 1 {
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
		fmt.Fprintf(&b, "%s (0 events)\n", string(scope))
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

func filterByScope(events []Event, scope Scope, sessions ...int) []Event {
	n := 0
	if len(sessions) > 0 {
		n = sessions[0]
	}

	if n > 0 {
		return lastNSessions(events, n)
	}

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

func lastNSessions(events []Event, n int) []Event {
	if n <= 0 || len(events) == 0 {
		return nil
	}

	// Cap at max 5 sessions.
	if n > 5 {
		n = 5
	}

	// Events are chronological (oldest first).
	// Split into sessions at kind="session" markers (set on server start/restart).
	// Everything before the first marker is its own session.
	var boundaries []int
	for i, e := range events {
		if e.Kind == "session" && e.Action == "start" {
			boundaries = append(boundaries, i)
		}
	}

	// Build session intervals: each session is between two consecutive boundaries,
	// excluding the boundary events themselves.
	var sessions [][2]int // [start, end) pairs
	if len(boundaries) == 0 {
		// No markers — everything is one session
		sessions = append(sessions, [2]int{0, len(events)})
	} else {
		// Before first marker
		if boundaries[0] > 0 {
			sessions = append(sessions, [2]int{0, boundaries[0]})
		}
		for i := 0; i < len(boundaries); i++ {
			end := len(events)
			if i+1 < len(boundaries) {
				end = boundaries[i+1]
			}
			// Skip the boundary event itself
			start := boundaries[i] + 1
			if start < end {
				sessions = append(sessions, [2]int{start, end})
			}
		}
	}

	// Take last N sessions
	if len(sessions) > n {
		sessions = sessions[len(sessions)-n:]
	}

	// Flatten sessions into newest-first events, skipping watch events
	var out []Event
	for si := len(sessions) - 1; si >= 0; si-- {
		s := sessions[si]
		for i := s[1] - 1; i >= s[0]; i-- {
			if events[i].Kind != "watch" {
				out = append(out, events[i])
			}
		}
	}
	return out
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
		if line := renderEvent(e); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func renderEvent(e Event) string {
	prefix := "+"
	if e.Action == "read" || e.Action == "reindex" || e.Action == "auto_reindex" {
		prefix = "~"
	}

	switch e.Kind {
	case "session":
		return ""
	case "fact":
		return fmt.Sprintf(`+ fact:   remembered: "%s"`, e.Summary)
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
