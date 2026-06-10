package retrieval

import (
	"regexp"
	"strings"
)

var (
	rePyDocstring = regexp.MustCompile(`"""([^"]*)"""`)
	rePyClass     = regexp.MustCompile(`(?m)^\s*class\s+(\w+)`)
	rePyFunction  = regexp.MustCompile(`(?m)^\s*def\s+(\w+)`)
	reGoType      = regexp.MustCompile(`(?m)^\s*type\s+(\w+)`)
	reGoFunc      = regexp.MustCompile(`(?m)^\s*func\s+(\w+)`)
	reJSClass     = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:default\s+)?class\s+(\w+)`)
	reJSFunction  = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s+(\w+)`)
	rePyImport    = regexp.MustCompile(`(?m)^(?:from\s+\S+\s+)?import\s+(.+)$`)
)

func splitCamelWords(s string) []string {
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

type fileSummary struct {
	tags        []string
	classes     []string
	functions   []string
	description string
}

func extractFileSummary(relPath, content string) fileSummary {
	ext := ""
	if idx := strings.LastIndex(relPath, "."); idx >= 0 {
		ext = relPath[idx:]
	}

	summary := fileSummary{
		description: extractDescription(content, ext),
	}

	switch ext {
	case ".py":
		summary.classes = extractMatches(content, rePyClass)
		summary.functions = extractMatches(content, rePyFunction)
		imports := extractImportTargets(content)
		summary.tags = buildTags(relPath, summary.classes, summary.functions, imports)
	case ".go":
		summary.classes = extractMatches(content, reGoType)
		summary.functions = extractMatches(content, reGoFunc)
		summary.tags = buildTags(relPath, summary.classes, summary.functions, nil)
	case ".js", ".ts", ".jsx", ".tsx":
		summary.classes = extractMatches(content, reJSClass)
		summary.functions = extractMatches(content, reJSFunction)
		summary.tags = buildTags(relPath, summary.classes, summary.functions, nil)
	default:
		summary.tags = extractKeywordsFromContent(content)
	}

	uniq := make(map[string]bool)
	var filtered []string
	for _, t := range summary.tags {
		lower := strings.ToLower(t)
		if !uniq[lower] && len(lower) >= 2 {
			uniq[lower] = true
			filtered = append(filtered, lower)
		}
	}
	summary.tags = filtered

	return summary
}

func buildSummaryLine(summary fileSummary) string {
	var b strings.Builder
	b.WriteString("__SUMMARY__:")
	for _, t := range summary.tags {
		b.WriteString(" ")
		b.WriteString(t)
	}
	b.WriteString(" __END__\n")
	if summary.description != "" {
		b.WriteString(summary.description)
		b.WriteString("\n")
	}
	return b.String()
}

func extractDescription(content, ext string) string {
	switch ext {
	case ".py":
		docstring := matchDocstring(content)
		if docstring != "" {
			return cleanupDocstring(docstring)
		}
		return extractLeadingComments(content, "#")
	case ".go":
		return extractLeadingComments(content, "//")
	case ".js", ".ts", ".jsx", ".tsx":
		docstring := matchStarDocstring(content)
		if docstring != "" {
			return cleanupDocstring(docstring)
		}
		return extractLeadingComments(content, "//")
	}
	return ""
}

func matchDocstring(content string) string {
	m := rePyDocstring.FindStringSubmatch(content)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

func matchStarDocstring(content string) string {
	re := regexp.MustCompile(`/\*\*([^*]*)\*/`)
	m := re.FindStringSubmatch(content)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func extractLeadingComments(content, prefix string) string {
	lines := strings.Split(content, "\n")
	var comments []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			text := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
			comments = append(comments, text)
		} else if len(comments) > 0 {
			break
		}
	}
	if len(comments) > 0 {
		return strings.Join(comments, " ")
	}
	return ""
}

func cleanupDocstring(docstring string) string {
	docstring = strings.ReplaceAll(docstring, "\n", " ")
	docstring = strings.ReplaceAll(docstring, "\t", " ")
	parts := strings.Fields(docstring)
	return strings.Join(parts, " ")
}

func extractMatches(content string, re *regexp.Regexp) []string {
	matches := re.FindAllStringSubmatch(content, -1)
	var result []string
	seen := make(map[string]bool)
	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	return result
}

func extractImportTargets(content string) []string {
	matches := rePyImport.FindAllStringSubmatch(content, -1)
	var targets []string
	for _, m := range matches {
		line := m[1]
		line = strings.TrimPrefix(line, "import ")
		parts := strings.Fields(line)
		for _, p := range parts {
			p = strings.Trim(p, ",")
			if strings.Contains(p, ".") {
				segments := strings.Split(p, ".")
				p = segments[0]
			}
			if !isStandardLib(p) && len(p) >= 2 {
				targets = append(targets, p)
			}
		}
	}
	return targets
}

func isStandardLib(pkg string) bool {
	stdLibs := map[string]bool{
		"os": true, "io": true, "fmt": true, "sys": true,
		"json": true, "re": true, "time": true, "math": true,
		"path": true, "typing": true, "abc": true, "enum": true,
		"dataclasses": true, "collections": true, "functools": true,
		"itertools": true, "copy": true, "hashlib": true,
		"logging": true, "warnings": true, "contextlib": true,
		"subprocess": true, "tempfile": true, "shutil": true,
		"glob": true, "fnmatch": true, "linecache": true,
		"ast": true, "inspect": true, "textwrap": true, "pprint": true,
		"net": true, "http": true, "urllib": true, "email": true,
		"xml": true, "html": true, "csv": true, "stringio": true,
		"sqlite3": true, "threading": true, "multiprocessing": true,
		"signal": true, "traceback": true, "pickle": true,
		"struct": true, "base64": true, "binascii": true,
		"uuid": true, "datetime": true, "calendar": true,
		"random": true, "statistics": true, "decimal": true,
		"fractions": true, "unittest": true, "doctest": true,
		"pdb": true, "profile": true, "cProfile": true,
		"ctypes": true, "curses": true, "platform": true,
		"errno": true, "gc": true, "sysconfig": true,
	}
	return stdLibs[pkg]
}

func buildTags(relPath string, classes, functions, imports []string) []string {
	var tags []string

	filename := strings.ToLower(relPath)
	filename = strings.TrimSuffix(filename, ".py")
	filename = strings.TrimSuffix(filename, ".go")
	dirParts := strings.Split(filename, "/")
	for _, part := range dirParts {
		if part != "" && part != "." && !isStopTag(part) {
			tags = append(tags, part)
		}
	}

	for _, c := range classes {
		tags = append(tags, c)
		words := splitCamelWords(c)
		tags = append(tags, words...)
	}

	for _, f := range functions {
		words := splitCamelWords(f)
		tags = append(tags, words...)
	}

	if imports != nil {
		tags = append(tags, imports...)
	}

	return tags
}

func isStopTag(tag string) bool {
	stops := map[string]bool{
		"aider": true, "coders": true, "src": true, "lib": true,
		"test": true, "tests": true, "cmd": true, "internal": true,
	}
	return stops[tag]
}

func extractKeywordsFromContent(content string) []string {
	lines := strings.Split(content, "\n")
	limit := 30
	if len(lines) > limit {
		lines = lines[:limit]
	}
	var keywords []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		words := strings.Fields(trimmed)
		if len(words) > 8 {
			words = words[:8]
		}
		for _, w := range words {
			w = strings.Trim(w, ".,!?;:'\"()[]{}/\\#")
			if len(w) >= 3 {
				keywords = append(keywords, w)
			}
		}
	}
	return keywords
}
