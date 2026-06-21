package benchmark

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/okyashgajjar/costwise-mcp/internal/answertype"
	"github.com/okyashgajjar/costwise-mcp/internal/repository"
	"github.com/okyashgajjar/costwise-mcp/internal/retrieval"
)

// CompareRow records the per-prompt cost of answering with our MCP versus the
// default no-MCP agent loop (grep the symbol, then read the most-relevant file).
type CompareRow struct {
	Task          Task
	WithTokens    int // tokens of our compressed answer (what the model sees)
	WithCalls     int // tool calls with our MCP (1)
	WithoutTokens int // grep output + file contents the agent must read
	WithoutCalls  int // grep + read(s)
	WithoutFiles  int
	Hit           bool // our answer contained the expected file or symbol
}

// estTokens mirrors the product's ~4-chars-per-token heuristic, applied to both
// sides so the comparison is apples-to-apples.
func estTokens(s string) int { return len(s) / 4 }

// firstLines returns the first n lines of s, modeling a read-tool's per-call cap.
func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

func grepTerm(t Task) string {
	if s := t.Symbols(); len(s) > 0 {
		return s[0]
	}
	best := ""
	for _, w := range strings.Fields(t.Query) {
		w = strings.Trim(w, "?.,!:;'\"()")
		if len(w) > len(best) {
			best = w
		}
	}
	return best
}

// grepSymbol returns the (capped) ripgrep output for term and the repo-relative
// file with the most matches — the file a no-MCP agent would open.
func grepSymbol(repoRoot, term string, capLines int) (output, topFile string) {
	if term == "" {
		return "", ""
	}
	out, _ := exec.Command("rg", "-n", "--no-heading", "--no-messages", "-F", term, repoRoot).Output()
	counts := map[string]int{}
	var kept []string
	for _, ln := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if ln == "" {
			continue
		}
		if i := strings.IndexByte(ln, ':'); i > 0 {
			if rel, err := filepath.Rel(repoRoot, ln[:i]); err == nil {
				counts[rel]++
			}
		}
		if len(kept) < capLines {
			kept = append(kept, ln)
		}
	}
	bestN := 0
	for f, n := range counts {
		if n > bestN {
			bestN, topFile = n, f
		}
	}
	return strings.Join(kept, "\n"), topFile
}

// RunCompare answers each task both ways and records the token/call/file cost.
func RunCompare(ctx context.Context, repoRoot string, tasks []Task) ([]CompareRow, error) {
	auto := retrieval.NewAutoRetriever("")
	if err := auto.Initialize(ctx, &repository.RepositoryInfo{Root: repoRoot}); err != nil {
		return nil, err
	}

	rows := make([]CompareRow, 0, len(tasks))
	for _, t := range tasks {
		row := CompareRow{Task: t, WithCalls: 1}

		// WITH our MCP: a single tool call returning a compressed answer.
		atc := answertype.Classify(t.Query, "chat")
		if raw, err := auto.Retrieve(ctx, t.Query); err == nil {
			filtered := retrieval.FilterResults(raw, 0.15, retrieval.GuessMaxResults(atc))
			compressed := retrieval.CompressForAnswerType(filtered, atc, retrieval.GuessContextBudget(atc))
			row.WithTokens = compressed.Tokens
			row.Hit = scoreFileHit(repoRoot, t.Files(), filtered, compressed.Context) ||
				scoreSymHit(t.Symbols(), compressed.Context)
		}

		// WITHOUT our MCP: grep the symbol, then read the most-relevant file.
		grepOut, topFile := grepSymbol(repoRoot, grepTerm(t), 80)
		row.WithoutTokens = estTokens(grepOut)
		row.WithoutCalls = 1
		if topFile != "" {
			if data, err := os.ReadFile(filepath.Join(repoRoot, topFile)); err == nil {
				// A real read-tool returns at most ~2000 lines per call (the
				// Claude Code Read default). Capping here keeps the baseline
				// realistic and conservative — a single huge file doesn't get
				// to claim millions of tokens in one read.
				row.WithoutTokens += estTokens(firstLines(string(data), 2000))
				row.WithoutCalls++
				row.WithoutFiles = 1
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// CompareReport renders per-prompt averages and the reduction our MCP delivers.
func CompareReport(name string, rows []CompareRow) string {
	var withTok, withoutTok, withCalls, withoutCalls, hits int
	for _, r := range rows {
		withTok += r.WithTokens
		withoutTok += r.WithoutTokens
		withCalls += r.WithCalls
		withoutCalls += r.WithoutCalls
		if r.Hit {
			hits++
		}
	}
	n := len(rows)
	if n == 0 {
		return "no prompts\n"
	}
	avg := func(x int) float64 { return float64(x) / float64(n) }
	red := func(with, without int) float64 {
		if without == 0 {
			return 0
		}
		return float64(without-with) / float64(without) * 100
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n=== With vs Without our MCP: %s (%d prompts) ===\n\n", name, n)
	fmt.Fprintf(&b, "%-22s %14s %14s %12s\n", "Metric", "With MCP", "Without MCP", "Reduction")
	fmt.Fprintln(&b, strings.Repeat("-", 66))
	fmt.Fprintf(&b, "%-22s %14.0f %14.0f %11.0f%%\n", "Avg input tokens", avg(withTok), avg(withoutTok), red(withTok, withoutTok))
	fmt.Fprintf(&b, "%-22s %14d %14d %11.0f%%\n", "Total input tokens", withTok, withoutTok, red(withTok, withoutTok))
	fmt.Fprintf(&b, "%-22s %14.2f %14.2f %11.0f%%\n", "Avg tool calls", avg(withCalls), avg(withoutCalls), red(withCalls, withoutCalls))
	fmt.Fprintf(&b, "%-22s %14d %14d %12s\n", "Files read (full)", 0, func() int {
		f := 0
		for _, r := range rows {
			f += r.WithoutFiles
		}
		return f
	}(), "")
	fmt.Fprintf(&b, "%-22s %13.0f%% %14s %12s\n", "Answer accuracy", float64(hits)/float64(n)*100, "(n/a)", "")
	fmt.Fprintf(&b, "\nWith MCP = one search_code call returning a compressed answer.\n")
	fmt.Fprintf(&b, "Without  = grep the symbol, then read the most-relevant file (the default agent loop).\n")
	fmt.Fprintf(&b, "Tokens estimated at ~4 chars/token on both sides (CSV: %d %d %d %d %d %d)\n",
		n, withTok, withoutTok, withCalls, withoutCalls, hits)
	return b.String()
}
