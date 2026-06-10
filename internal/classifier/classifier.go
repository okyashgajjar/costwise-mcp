package classifier

import (
	"fmt"
	"strings"
	"unicode"
)

type QueryClass int

const (
	SymbolQuery QueryClass = iota
	TextQuery
	RepositoryQuery
	ReferenceQuery
	CallQuery
	ArchitectureQuery
	FlowQuery
)

func (c QueryClass) String() string {
	switch c {
	case SymbolQuery:
		return "symbol"
	case TextQuery:
		return "text"
	case RepositoryQuery:
		return "repository"
	case ReferenceQuery:
		return "reference"
	case CallQuery:
		return "call"
	case ArchitectureQuery:
		return "architecture"
	case FlowQuery:
		return "flow"
	default:
		return "unknown"
	}
}

type Classification struct {
	Class      QueryClass
	Label      string
	Confidence float64
	Reason     string
}

var symbolIndicators = []string{
	"implemented", "defined", "declared",
	"function", "method", "class", "struct",
	"interface", "type", "variable", "constant",
	"field", "property", "func", "def",
	"definition", "implementation",
	"export", "import", "package", "module",
	"constructor", "alias",
}

var textQuestionWords = []string{"how", "what", "why", "when"}

var textActionWords = []string{
	"find", "search", "explain", "describe",
	"show", "tell", "list",
	"work", "handle", "process", "manage",
	"connect", "configure", "setup",
	"use", "using", "usage",
	"example", "pattern",
	"error", "bug", "fix", "test", "debug",
	"performance", "optimize",
}

var callPatterns = []string{
	"who calls ", "find callers of ", "show callers of ",
	"show call sites of ",
}

var callActionWords = []string{
	"calls", "callers", "call sites", "called by",
}

var refPatterns = []string{
	"who uses ", "who imports ",
	"find references to ", "find usages of ", "find all references to ",
	"show references to ", "show imports of ",
	"which files reference ", "which files import ",
	"references to ", "usages of ", "imports of ",
	"files that import ", "files referencing ",
	"dependents of ", "dependants of ",
}

var refActionWords = []string{
	"references", "usages", "imports", "exports",
	"dependents", "dependants", "callers", "call sites",
}

var archIndicators = []string{
	"architecture", "mechanism", "workflow", "lifecycle",
	"system", "design",
}

var repoIndicators = []string{
	"about", "overview", "purpose",
	"summary", "description", "introduction",
	"architecture", "design", "structure",
	"goal", "objective", "motivation",
	"features", "capabilities", "scope",
	"readme", "getting started", "quick start",
	"installation", "tutorial", "guide",
	"contributing", "license", "changelog", "roadmap",
	"this project", "this repo", "project is",
	"project does", "the project", "project organized",
	"the codebase", "this codebase",
}

var repoWordIndicators = []string{
	"project", "repository", "codebase",
}

var repoPhrasePatterns = []string{
	"what does", "how does", "what is",
	"how is", "what are", "how are",
	"describe this", "what problem",
}

func Classify(query string) Classification {
	query = strings.TrimSpace(query)
	lower := strings.ToLower(query)
	words := strings.Fields(lower)
	rawWords := strings.Fields(query)

	if len(words) == 0 {
		return Classification{
			Class:      RepositoryQuery,
			Label:      "repository",
			Confidence: 0.5,
			Reason:     "empty query",
		}
	}

	callScore, callReason := scoreCall(lower, rawWords)

	if callScore > 0 {
		conf := callScore
		if conf > 0.95 {
			conf = 0.95
		}
		return Classification{
			Class:      CallQuery,
			Label:      "call",
			Confidence: conf,
			Reason:     callReason,
		}
	}

	refScore, refReason := scoreReference(lower, rawWords)

	if refScore > 0 {
		conf := refScore
		if conf > 0.95 {
			conf = 0.95
		}
		return Classification{
			Class:      ReferenceQuery,
			Label:      "reference",
			Confidence: conf,
			Reason:     refReason,
		}
	}

	symScore, symReason := scoreSymbol(lower, rawWords)
	txtScore, txtReason := scoreText(lower, words)
	repScore, repReason := scoreRepo(lower, words)
	archScore, archReason := scoreArchitecture(lower, rawWords, symScore)
	flowScore, flowReason := scoreFlow(lower, rawWords)

	total := symScore + txtScore + repScore + archScore + flowScore
	if total == 0 {
		return Classification{
			Class:      TextQuery,
			Label:      "text",
			Confidence: 0.5,
			Reason:     "default",
		}
	}

	if archScore > symScore && archScore > txtScore && archScore > repScore && archScore > flowScore {
		conf := archScore / total
		if conf > 0.95 {
			conf = 0.95
		}
		return Classification{
			Class:      ArchitectureQuery,
			Label:      "architecture",
			Confidence: conf,
			Reason:     archReason,
		}
	}

	if flowScore > symScore && flowScore > txtScore && flowScore > repScore && flowScore > archScore {
		conf := flowScore / total
		if conf > 0.95 {
			conf = 0.95
		}
		return Classification{
			Class:      FlowQuery,
			Label:      "flow",
			Confidence: conf,
			Reason:     flowReason,
		}
	}

	if symScore >= txtScore && symScore >= repScore && symScore > 0 {
		conf := symScore / total
		if conf > 0.95 {
			conf = 0.95
		}
		return Classification{
			Class:      SymbolQuery,
			Label:      "symbol",
			Confidence: conf,
			Reason:     symReason,
		}
	}

	if txtScore > repScore && txtScore > 0 {
		conf := txtScore / total
		if conf > 0.95 {
			conf = 0.95
		}
		return Classification{
			Class:      TextQuery,
			Label:      "text",
			Confidence: conf,
			Reason:     txtReason,
		}
	}

	if repScore > 0 {
		conf := repScore / total
		if conf > 0.95 {
			conf = 0.95
		}
		return Classification{
			Class:      RepositoryQuery,
			Label:      "repository",
			Confidence: conf,
			Reason:     repReason,
		}
	}

	return Classification{
		Class:      TextQuery,
		Label:      "text",
		Confidence: 0.5,
		Reason:     "default",
	}
}

func scoreCall(lower string, rawWords []string) (float64, string) {
	score := 0.0
	var reasons []string

	for _, pat := range callPatterns {
		if strings.Contains(lower, pat) {
			score += 0.6
			reasons = append(reasons, "call_pattern")
			break
		}
	}

	for _, act := range callActionWords {
		if strings.Contains(lower, act) {
			score += 0.3
			reasons = append(reasons, act)
		}
	}

	hasUpper := false
	for _, w := range rawWords {
		w = clean(w)
		if len(w) >= 2 && w[0] >= 'A' && w[0] <= 'Z' {
			hasUpper = true
			break
		}
	}
	if score > 0 && hasUpper {
		score += 0.3
		reasons = append(reasons, "with_symbol")
	}

	return score, strings.Join(reasons, ",")
}

func scoreReference(lower string, rawWords []string) (float64, string) {
	score := 0.0
	var reasons []string

	for _, pat := range refPatterns {
		if strings.Contains(lower, pat) {
			score += 0.6
			reasons = append(reasons, "ref_pattern")
			break
		}
	}

	for _, act := range refActionWords {
		if strings.Contains(lower, act) {
			score += 0.3
			reasons = append(reasons, act)
		}
	}

	hasUpper := false
	for _, w := range rawWords {
		w = clean(w)
		if len(w) >= 2 && w[0] >= 'A' && w[0] <= 'Z' {
			hasUpper = true
			break
		}
	}
	if score > 0 && hasUpper {
		score += 0.3
		reasons = append(reasons, "with_symbol")
	}

	return score, strings.Join(reasons, ",")
}

func scoreSymbol(lower string, rawWords []string) (float64, string) {
	score := 0.0
	var reasons []string

	pascalCount := 0
	snakeCount := 0
	for _, w := range rawWords {
		w = clean(w)
		if len(w) >= 2 && unicode.IsUpper(rune(w[0])) {
			hasUpperAfterFirst := false
			for _, ch := range w[1:] {
				if unicode.IsUpper(ch) {
					hasUpperAfterFirst = true
					break
				}
			}
			if hasUpperAfterFirst {
				pascalCount++
			}
		}
		if strings.Contains(w, "_") && len(w) >= 3 {
			snakeCount++
		}
	}

	allCapsCount := 0
	for _, w := range rawWords {
		w = clean(w)
		if len(w) >= 2 && len(w) <= 8 {
			allUpper := true
			for _, ch := range w {
				if !unicode.IsUpper(ch) {
					allUpper = false
					break
				}
			}
			if allUpper {
				allCapsCount++
			}
		}
	}

	if pascalCount > 0 {
		s := 0.3 + 0.15*float64(pascalCount-1)
		if s > 0.7 {
			s = 0.7
		}
		score += s
		reasons = append(reasons, fmt.Sprintf("pascal_x%d", pascalCount))
	}

	if snakeCount > 0 {
		score += 0.3 + 0.1*float64(snakeCount-1)
		reasons = append(reasons, fmt.Sprintf("snake_x%d", snakeCount))
	}

	if allCapsCount > 0 {
		score += 0.2 * float64(allCapsCount)
		reasons = append(reasons, fmt.Sprintf("allcaps_x%d", allCapsCount))
	}

	camelCount := 0
	for _, w := range rawWords {
		w = clean(w)
		if len(w) >= 3 && unicode.IsLower(rune(w[0])) {
			for _, ch := range w[1:] {
				if unicode.IsUpper(ch) {
					camelCount++
					break
				}
			}
		}
	}
	if camelCount > 0 {
		score += 0.2 * float64(camelCount)
		reasons = append(reasons, fmt.Sprintf("camel_x%d", camelCount))
	}

	for _, ind := range symbolIndicators {
		if strings.Contains(lower, ind) {
			score += 0.15
			reasons = append(reasons, ind)
		}
	}

	if len(rawWords) <= 3 && (pascalCount > 0 || allCapsCount > 0 || camelCount > 0 || snakeCount > 0) {
		score += 0.2
		reasons = append(reasons, "short_with_symbol")
	}

	if len(rawWords) >= 2 {
		first := strings.ToLower(rawWords[0])
		if first == "where" || first == "find" || first == "show" {
			for _, w := range rawWords[1:] {
				wc := clean(w)
				if len(wc) >= 2 && unicode.IsUpper(rune(wc[0])) {
					score += 0.25
					reasons = append(reasons, "lookup")
					break
				}
			}
		}
	}

	return score, strings.Join(reasons, ",")
}

func scoreText(lower string, words []string) (float64, string) {
	score := 0.0
	var reasons []string

	for _, qw := range textQuestionWords {
		for _, w := range words {
			if w == qw {
				score += 0.3
				reasons = append(reasons, qw)
				break
			}
		}
	}

	for _, act := range textActionWords {
		if strings.Contains(lower, act) {
			score += 0.15
			reasons = append(reasons, act)
		}
	}

	if len(words) >= 4 {
		score += 0.1
		reasons = append(reasons, "long_query")
	}

	return score, strings.Join(reasons, ",")
}

func scoreRepo(lower string, words []string) (float64, string) {
	score := 0.0
	var reasons []string

	for _, ind := range repoIndicators {
		if strings.Contains(lower, ind) {
			score += 0.3
			reasons = append(reasons, ind)
		}
	}

	for _, pat := range repoPhrasePatterns {
		if containsPhrase(lower, pat) {
			score += 0.4
			reasons = append(reasons, pat)
		}
	}

	for _, word := range repoWordIndicators {
		for _, w := range words {
			if w == word {
				score += 0.2
				reasons = append(reasons, "word_"+word)
				break
			}
		}
	}

	if strings.Contains(lower, "what is this") || strings.Contains(lower, "what's this") {
		score += 0.5
		reasons = append(reasons, "what_is_this")
	}

	if len(words) <= 2 {
		for _, ind := range repoIndicators {
			for _, w := range words {
				if w == ind {
					score += 0.2
					reasons = append(reasons, "short_repo")
					break
				}
			}
		}
	}

	return score, strings.Join(reasons, ",")
}

func scoreArchitecture(lower string, rawWords []string, symScore float64) (float64, string) {
	score := 0.0
	var reasons []string

	if strings.Contains(lower, "project architecture") || strings.Contains(lower, "repository architecture") {
		return 0, ""
	}

	hasArchWord := false
	for _, ind := range archIndicators {
		if strings.Contains(lower, ind) {
			hasArchWord = true
			reasons = append(reasons, ind)
			break
		}
	}

	if !hasArchWord {
		return 0, ""
	}

	hasExplain := strings.Contains(lower, "explain") || strings.Contains(lower, "describe")

	if hasExplain {
		score += 0.4
		reasons = append(reasons, "explain_describe")
	}

	if hasArchWord && hasExplain {
		hasSpecific := false
		for _, w := range rawWords {
			if hasSymbolPattern(w) {
				score += 0.5
				reasons = append(reasons, "with_symbol")
				hasSpecific = true
				break
			}
		}
		if !hasSpecific {
			score += 0.2
			reasons = append(reasons, "generic_explain")
		}
		score += 0.3
	} else if hasArchWord && symScore > 0 {
		score += 0.4
		reasons = append(reasons, "arch+symbol")
	} else if hasExplain {
		score += 0.3
		reasons = append(reasons, "explain_generic")
	}

	return score, strings.Join(reasons, ",")
}

func scoreFlow(lower string, rawWords []string) (float64, string) {
	score := 0.0
	var reasons []string

	if strings.Contains(lower, "trace") || strings.Contains(lower, "flow of") {
		score += 0.5
		reasons = append(reasons, "trace_flow")
	}

	if strings.Contains(lower, "startup sequence") || strings.Contains(lower, "startup flow") {
		score += 0.6
		reasons = append(reasons, "startup")
	}

	if strings.Contains(lower, "lifecycle") || strings.Contains(lower, "request lifecycle") {
		score += 0.5
		reasons = append(reasons, "lifecycle")
	}

	if strings.Contains(lower, "pipeline") {
		score += 0.4
		reasons = append(reasons, "pipeline")
	}

	hasSpecific := false
	for _, w := range rawWords {
		if hasSymbolPattern(w) {
			hasSpecific = true
			break
		}
	}
	if score > 0 && hasSpecific {
		score += 0.3
		reasons = append(reasons, "with_symbol")
	}

	return score, strings.Join(reasons, ",")
}

func hasSymbolPattern(w string) bool {
	w = clean(w)
	if len(w) < 3 {
		return false
	}
	upperAfterLower := false
	for i, ch := range w {
		if i == 0 {
			continue
		}
		if ch >= 'A' && ch <= 'Z' {
			upperAfterLower = true
			break
		}
	}
	if upperAfterLower {
		return true
	}
	if strings.Contains(w, "_") {
		return true
	}
	if strings.Contains(w, "-") {
		return true
	}
	allUpper := len(w) >= 2 && len(w) <= 8
	for _, ch := range w {
		if ch < 'A' || ch > 'Z' {
			allUpper = false
			break
		}
	}
	return allUpper
}

func clean(w string) string {
	return strings.Trim(w, ".,!?;:'\"()[]{}/\\")
}

func containsPhrase(lower, phrase string) bool {
	phraseWords := strings.Fields(phrase)
	lowerWords := strings.Fields(lower)
	if len(phraseWords) == 0 || len(lowerWords) == 0 {
		return false
	}
	start := -1
	for i, w := range lowerWords {
		if w == phraseWords[0] {
			start = i
			break
		}
	}
	if start < 0 {
		return false
	}
	lowerWords = lowerWords[start:]
	wi := 0
	for _, lw := range lowerWords {
		if wi < len(phraseWords) && lw == phraseWords[wi] {
			wi++
		}
	}
	return wi == len(phraseWords)
}
