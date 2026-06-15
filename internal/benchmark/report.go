package benchmark

import (
	"fmt"
	"sort"
	"strings"
)

// CategoryStats aggregates scored results for one category (or overall).
type CategoryStats struct {
	Category      string
	Total         int
	Scorable      int // has expected files and is not stale
	FileHits      int
	SymScorable   int // has expected symbols
	SymHits       int
	RouteScorable int // has an expected_retriever
	RouteHits     int
	Stale         int
	Empty         int
	SumTokens     int
	SumLatencyMs  int64
}

func (c CategoryStats) FileHitRate() float64 {
	if c.Scorable == 0 {
		return -1
	}
	return float64(c.FileHits) / float64(c.Scorable) * 100
}

func (c CategoryStats) SymHitRate() float64 {
	if c.SymScorable == 0 {
		return -1
	}
	return float64(c.SymHits) / float64(c.SymScorable) * 100
}

func (c CategoryStats) RouteHitRate() float64 {
	if c.RouteScorable == 0 {
		return -1
	}
	return float64(c.RouteHits) / float64(c.RouteScorable) * 100
}

func (c CategoryStats) AvgTokens() float64 {
	if c.Total == 0 {
		return 0
	}
	return float64(c.SumTokens) / float64(c.Total)
}

// Aggregate rolls task results up into per-category stats and an overall total.
func Aggregate(results []TaskResult) (map[string]*CategoryStats, *CategoryStats) {
	cats := map[string]*CategoryStats{}
	overall := &CategoryStats{Category: "OVERALL"}
	for _, r := range results {
		c := cats[r.Task.Category]
		if c == nil {
			c = &CategoryStats{Category: r.Task.Category}
			cats[r.Task.Category] = c
		}
		hasFiles := len(r.Task.Files()) > 0
		for _, agg := range []*CategoryStats{c, overall} {
			agg.Total++
			agg.SumTokens += r.ContextTokens
			agg.SumLatencyMs += r.LatencyMs
			if r.Empty {
				agg.Empty++
			}
			if r.Task.ExpectedRetriever != "" {
				agg.RouteScorable++
				if r.RouteHit {
					agg.RouteHits++
				}
			}
			if len(r.Task.Symbols()) > 0 {
				agg.SymScorable++
				if r.SymHit {
					agg.SymHits++
				}
			}
			if hasFiles {
				if r.Stale {
					agg.Stale++
				} else {
					agg.Scorable++
					if r.FileHit {
						agg.FileHits++
					}
				}
			}
		}
	}
	return cats, overall
}

func pct(v float64) string {
	if v < 0 {
		return "  n/a"
	}
	return fmt.Sprintf("%4.0f%%", v)
}

// Report renders a human-readable scorecard for one dataset run.
func Report(repoName string, results []TaskResult) string {
	cats, overall := Aggregate(results)
	var b strings.Builder
	fmt.Fprintf(&b, "\n=== Baseline: %s (%d queries) ===\n\n", repoName, overall.Total)
	fmt.Fprintf(&b, "%-13s %7s %9s %8s %8s %9s %7s %7s\n",
		"Category", "Queries", "FileHit", "SymHit", "Route", "Tokens", "Stale", "Empty")
	fmt.Fprintln(&b, strings.Repeat("-", 78))

	names := make([]string, 0, len(cats))
	for n := range cats {
		names = append(names, n)
	}
	sort.Strings(names)
	row := func(c *CategoryStats) {
		fmt.Fprintf(&b, "%-13s %7d %9s %8s %8s %9.0f %7d %7d\n",
			c.Category, c.Total, pct(c.FileHitRate()), pct(c.SymHitRate()), pct(c.RouteHitRate()),
			c.AvgTokens(), c.Stale, c.Empty)
	}
	for _, n := range names {
		row(cats[n])
	}
	fmt.Fprintln(&b, strings.Repeat("-", 78))
	row(overall)

	fmt.Fprintf(&b, "\nFileHit = of queries with a known target file, how often it appears in our answer.\n")
	fmt.Fprintf(&b, "SymHit  = of queries with a known target symbol, how often it appears in our answer.\n")
	fmt.Fprintf(&b, "Route   = how often our classifier picked the expected retriever.\n")
	fmt.Fprintf(&b, "Tokens  = avg tokens in the compressed answer (what the model would see).\n")
	fmt.Fprintf(&b, "Stale   = target file not in this checkout (excluded from FileHit).\n")
	fmt.Fprintf(&b, "Empty   = pipeline returned no results.\n")
	return b.String()
}
