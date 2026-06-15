package benchmark

import (
	"path/filepath"
	"strings"
)

// depDirs are dependency/build directories that a code-intelligence tool does
// not index. Benchmark entries whose ground-truth lives here are unwinnable by
// design and don't reflect first-party retrieval quality.
var depDirs = []string{
	"node_modules/", "/vendor/", "/.venv/", "/venv/",
	"/dist/", "/build/", "/.next/", "/target/",
}

// CleanTasks drops low-quality benchmark entries so the score reflects real
// first-party source retrieval: (1) targets inside dependency/build dirs,
// (2) entries whose expected files aren't present in this checkout (stale
// paths), and (3) symbol-only entries whose symbols are all minified/obfuscated
// (e.g. "w", "R", "safeObjectKeys$4"). Returns the kept tasks and a count of
// how many were dropped per reason.
func CleanTasks(tasks []Task, repoRoot string) (kept []Task, dep, stale, minified int) {
	for _, t := range tasks {
		switch {
		case hasDepFile(t):
			dep++
		case len(t.Files()) > 0 && !expectedFilesExist(repoRoot, t.Files()):
			stale++
		case len(t.Files()) == 0 && len(t.Symbols()) > 0 && allMinified(t.Symbols()):
			minified++
		default:
			kept = append(kept, t)
		}
	}
	return kept, dep, stale, minified
}

func hasDepFile(t Task) bool {
	for _, f := range t.Files() {
		lf := strings.ToLower(filepath.ToSlash(f))
		for _, d := range depDirs {
			if strings.Contains(lf, d) {
				return true
			}
		}
	}
	return false
}

// allMinified reports whether every symbol looks obfuscated: shorter than 3
// chars or containing a '$' mangling marker.
func allMinified(syms []string) bool {
	for _, s := range syms {
		s = strings.TrimSpace(s)
		if len(s) >= 3 && !strings.Contains(s, "$") {
			return false
		}
	}
	return true
}
