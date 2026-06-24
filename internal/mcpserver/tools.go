package mcpserver

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/okyashgajjar/costwise-mcp/internal/answertype"
	"github.com/okyashgajjar/costwise-mcp/internal/ledger"
	"github.com/okyashgajjar/costwise-mcp/internal/repository"
	"github.com/okyashgajjar/costwise-mcp/internal/retrieval"
)

const (
	maxStashContentBytes = 1 << 20  // 1 MB
	maxFactBytes         = 10 << 10 // 10 KB
)

func RegisterTools(s *server.MCPServer) {
	// search_code
	s.AddTool(mcp.NewTool("search_code",
		mcp.WithDescription("Search the repository with an intelligent query pipeline. Best for natural language questions, architecture, or general code search."),
		mcp.WithString("repo_path", mcp.Required(), mcp.Description("Absolute path to the repository root")),
		mcp.WithString("query", mcp.Required(), mcp.Description("The search query or question")),
		mcp.WithString("budget", mcp.Description("Token budget: small (500), medium (1500), large (3000). Default is medium."), mcp.Enum("small", "medium", "large")),
	), searchCodeHandler)

	// find_symbol
	s.AddTool(mcp.NewTool("find_symbol",
		mcp.WithDescription("Find the exact definition location of a symbol (class, function, variable)."),
		mcp.WithString("repo_path", mcp.Required(), mcp.Description("Absolute path to the repository root")),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Symbol name to search for")),
		mcp.WithString("budget", mcp.Description("Token budget: small (500), medium (1500), large (3000). Default is small."), mcp.Enum("small", "medium", "large")),
	), findSymbolHandler)

	// read_symbol — the full implementation body of a symbol, not just its location.
	s.AddTool(mcp.NewTool("read_symbol",
		mcp.WithDescription("Return the full source code of a symbol (function, method, or type) by name — the implementation body itself, not just its location. Use this instead of reading a whole file when you need to see how something is implemented."),
		mcp.WithString("repo_path", mcp.Required(), mcp.Description("Absolute path to the repository root")),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Symbol name to read (function/method/type)")),
		mcp.WithString("budget", mcp.Description("Token budget: small (500), medium (1500), large (3000). Default is medium."), mcp.Enum("small", "medium", "large")),
	), readSymbolHandler)

	// find_references
	s.AddTool(mcp.NewTool("find_references",
		mcp.WithDescription("Find all references and usages of a symbol across the repository."),
		mcp.WithString("repo_path", mcp.Required(), mcp.Description("Absolute path to the repository root")),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Symbol name to find references for")),
		mcp.WithString("budget", mcp.Description("Token budget: small (500), medium (1500), large (3000). Default is medium."), mcp.Enum("small", "medium", "large")),
	), findReferencesHandler)

	// find_callers
	s.AddTool(mcp.NewTool("find_callers",
		mcp.WithDescription("Find all functions that call a specific function."),
		mcp.WithString("repo_path", mcp.Required(), mcp.Description("Absolute path to the repository root")),
		mcp.WithString("function", mcp.Required(), mcp.Description("Function name to find callers for")),
		mcp.WithString("budget", mcp.Description("Token budget: small (500), medium (1500), large (3000). Default is medium."), mcp.Enum("small", "medium", "large")),
	), findCallersHandler)

	// get_repository_summary
	s.AddTool(mcp.NewTool("get_repository_summary",
		mcp.WithDescription("Get a compact, token-budgeted overview of the repository (top modules, languages, file counts). Pass `module` to drill into one directory instead of the whole repo."),
		mcp.WithString("repo_path", mcp.Required(), mcp.Description("Absolute path to the repository root")),
		mcp.WithString("budget", mcp.Description("Token budget: small (~500), medium (~1500), large (~3000). Default small."), mcp.Enum("small", "medium", "large")),
		mcp.WithString("module", mcp.Description("Optional: a module name or directory path to drill into (its files + symbols) instead of the whole-repo overview.")),
	), repoSummaryHandler)

	// index_repository
	s.AddTool(mcp.NewTool("index_repository",
		mcp.WithDescription("Manually trigger a re-index of the repository. Usually unnecessary as the watcher auto-reindexes."),
		mcp.WithString("repo_path", mcp.Required(), mcp.Description("Absolute path to the repository root")),
	), indexRepoHandler)

	// remember — persist a small durable fact so it need not be repeated inline.
	s.AddTool(mcp.NewTool("remember",
		mcp.WithDescription("Persist a small durable fact (a decision, a path, a gotcha) so you don't have to keep repeating it inline in the conversation. Recall it later with `recall` instead of re-deriving it. Keeps the context window small."),
		mcp.WithString("repo_path", mcp.Required(), mcp.Description("Absolute path to the repository root")),
		mcp.WithString("key", mcp.Required(), mcp.Description("Short label for the fact, e.g. 'auth-entrypoint'")),
		mcp.WithString("fact", mcp.Required(), mcp.Description("The fact to remember, in one or two sentences")),
	), rememberHandler)

	// stash_context — park a large blob out of the window, return a tiny handle.
	s.AddTool(mcp.NewTool("stash_context",
		mcp.WithDescription("Park a large blob (a long file, a big command output, a generated report) OUT of the conversation and get back a tiny handle. Nothing is lost — the full content stays on disk and is re-fetchable with `recall`. Use this instead of pasting large output inline, which would be re-cached every turn."),
		mcp.WithString("repo_path", mcp.Required(), mcp.Description("Absolute path to the repository root")),
		mcp.WithString("content", mcp.Required(), mcp.Description("The large text to stash out of context")),
		mcp.WithString("label", mcp.Description("Optional short label describing the content")),
	), stashContextHandler)

	// recall — query-scoped read of remembered facts and stashed blobs.
	s.AddTool(mcp.NewTool("recall",
		mcp.WithDescription("Take back ONLY what you need: returns the budgeted slice of a stashed blob (by handle) or matching remembered facts, instead of the whole thing. With no `source`, lists matching facts and stash handles."),
		mcp.WithString("repo_path", mcp.Required(), mcp.Description("Absolute path to the repository root")),
		mcp.WithString("query", mcp.Required(), mcp.Description("What to look for within the source")),
		mcp.WithString("source", mcp.Description("A stash handle to read from, or 'facts' for remembered facts. Omit to search both.")),
		mcp.WithString("budget", mcp.Description("Token budget: small (~500), medium (~1500), large (~3000). Default small."), mcp.Enum("small", "medium", "large")),
	), recallHandler)

	// session_brief — compact catch-up on past sessions in this repo.
	s.AddTool(mcp.NewTool("session_brief",
		mcp.WithDescription("Get a compact summary of what happened in past session(s) on this repo — facts remembered, content stashed, files reindexed. Use this to catch up before starting work, instead of re-deriving context from scratch."),
		mcp.WithString("repo_path", mcp.Required(), mcp.Description("Absolute path to the repository root")),
		mcp.WithString("scope", mcp.Description("Scope: \"last\" (default, since last session boundary), \"today\", \"all\".")),
		mcp.WithString("budget", mcp.Description("Token budget (default 300). Events are oldest-first within scope.")),
		mcp.WithString("sessions", mcp.Description("Number of past sessions to return (e.g. \"5\" for last 5). Overrides scope. Default: 1 (last session).")),
	), sessionBriefHandler)
}

func getStringArg(args interface{}, key string) string {
	if argsMap, ok := args.(map[string]interface{}); ok {
		if v, ok := argsMap[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func parseBudget(args interface{}, defaultTokens int) int {
	b := getStringArg(args, "budget")
	switch b {
	case "small":
		return 500
	case "medium":
		return 1500
	case "large":
		return 3000
	default:
		return defaultTokens
	}
}

func searchCodeHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repoPath := getStringArg(request.Params.Arguments, "repo_path")
	query := getStringArg(request.Params.Arguments, "query")

	if repoPath == "" || query == "" {
		return mcp.NewToolResultError("repo_path and query are required"), nil
	}

	rs, err := GetOrCreateRepoSession(ctx, repoPath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedQuery := rs.ResolveQuery(query)
	atc := answertype.Classify(resolvedQuery, "chat")

	auto := retrieval.NewAutoRetriever("")
	if err := auto.Initialize(ctx, rs.Repo); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to initialize retriever: %v", err)), nil
	}

	results, err := auto.Retrieve(ctx, resolvedQuery)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Retrieve error: %v", err)), nil
	}

	results = retrieval.FilterResults(results, 0.15, retrieval.GuessMaxResults(atc))

	if len(results) == 0 {
		return mcp.NewToolResultText("No relevant code found."), nil
	}

	rs.StoreResult(resolvedQuery, results)

	budget := parseBudget(request.Params.Arguments, retrieval.GuessContextBudget(atc))
	compressed := retrieval.CompressForAnswerType(results, atc, budget)

	return mcp.NewToolResultText(compressed.Context), nil
}

func findSymbolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repoPath := getStringArg(request.Params.Arguments, "repo_path")
	symbol := getStringArg(request.Params.Arguments, "symbol")

	if repoPath == "" || symbol == "" {
		return mcp.NewToolResultError("repo_path and symbol are required"), nil
	}

	rs, err := GetOrCreateRepoSession(ctx, repoPath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ret := retrieval.NewSymbolRetriever()
	if err := ret.Initialize(ctx, rs.Repo); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	results, err := ret.Retrieve(ctx, symbol)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if len(results) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("Symbol '%s' not found.", symbol)), nil
	}

	atc := answertype.Classification{Type: answertype.Location}
	budget := parseBudget(request.Params.Arguments, 500)
	compressed := retrieval.CompressForAnswerType(results, atc, budget)

	return mcp.NewToolResultText(compressed.Context), nil
}

func readSymbolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repoPath := getStringArg(request.Params.Arguments, "repo_path")
	symbol := getStringArg(request.Params.Arguments, "symbol")

	if repoPath == "" || symbol == "" {
		return mcp.NewToolResultError("repo_path and symbol are required"), nil
	}

	rs, err := GetOrCreateRepoSession(ctx, repoPath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ret := retrieval.NewSymbolRetriever()
	if err := ret.Initialize(ctx, rs.Repo); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	results, err := ret.Retrieve(ctx, symbol)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if len(results) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("Symbol '%s' not found.", symbol)), nil
	}

	// Read the top match's body straight from its known line range — one file
	// slice, independent of repo size.
	top := results[0]

	// Defense-in-depth: verify the symbol DB result doesn't point outside the repo
	if !isPathWithinRoot(rs.Repo.Root, top.File) {
		return mcp.NewToolResultError(fmt.Sprintf("symbol path %s is outside repo root", top.File)), nil
	}

	budget := parseBudget(request.Params.Arguments, 1500)
	body, err := readLineRange(top.File, top.LineFrom, top.LineTo, budget*4)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("read %s: %v", top.File, err)), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s:%d-%d\n%s", relToRepo(rs.Repo.Root, top.File), top.LineFrom, top.LineTo, body)
	if len(results) > 1 {
		others := make([]string, 0, 4)
		for _, r := range results[1:] {
			others = append(others, fmt.Sprintf("%s:%d", relToRepo(rs.Repo.Root, r.File), r.LineFrom))
			if len(others) >= 4 {
				break
			}
		}
		fmt.Fprintf(&b, "\n\nOther matches (read_symbol returns the top one): %s", strings.Join(others, ", "))
	}
	return mcp.NewToolResultText(b.String()), nil
}

// readLineRange returns lines [from, to] (1-based, inclusive) of a file, capped
// to maxChars. Reading a single source file is cheap regardless of repo size.
func readLineRange(path string, from, to, maxChars int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	if from < 1 {
		from = 1
	}
	if to > len(lines) {
		to = len(lines)
	}
	if from > to {
		return "", nil
	}
	body := strings.Join(lines[from-1:to], "\n")
	if maxChars > 0 && len(body) > maxChars {
		body = body[:maxChars] + "\n… (truncated; raise budget to see the rest)"
	}
	return body, nil
}

// isPathWithinRoot checks that abs stays inside root (defense-in-depth against
// corrupt symbol DB results or path traversal).
func isPathWithinRoot(root, abs string) bool {
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
}

// relToRepo renders an absolute file path relative to the repo root for display.
func relToRepo(root, abs string) string {
	rel := strings.TrimPrefix(abs, root)
	return strings.TrimPrefix(rel, string(os.PathSeparator))
}

func findReferencesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repoPath := getStringArg(request.Params.Arguments, "repo_path")
	symbol := getStringArg(request.Params.Arguments, "symbol")

	rs, err := GetOrCreateRepoSession(ctx, repoPath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ret := retrieval.NewReferenceRetriever()
	if err := ret.Initialize(ctx, rs.Repo); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	results, err := ret.Retrieve(ctx, symbol)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	atc := answertype.Classification{Type: answertype.Reference}
	budget := parseBudget(request.Params.Arguments, 1500)
	compressed := retrieval.CompressForAnswerType(results, atc, budget)

	return mcp.NewToolResultText(compressed.Context), nil
}

func findCallersHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repoPath := getStringArg(request.Params.Arguments, "repo_path")
	function := getStringArg(request.Params.Arguments, "function")

	rs, err := GetOrCreateRepoSession(ctx, repoPath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ret := retrieval.NewCallGraphRetriever()
	if err := ret.Initialize(ctx, rs.Repo); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	results, err := ret.Retrieve(ctx, function)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	atc := answertype.Classification{Type: answertype.Caller}
	budget := parseBudget(request.Params.Arguments, 1500)
	compressed := retrieval.CompressForAnswerType(results, atc, budget)

	return mcp.NewToolResultText(compressed.Context), nil
}

func repoSummaryHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repoPath := getStringArg(request.Params.Arguments, "repo_path")

	rs, err := GetOrCreateRepoSession(ctx, repoPath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	budget := parseBudget(request.Params.Arguments, 500)
	module := getStringArg(request.Params.Arguments, "module")

	summaryText := retrieval.BuildRepositorySummaryCompact(rs.Knowledge, budget, module)
	if summaryText == "" {
		return mcp.NewToolResultText("Repository summary is empty or unavailable."), nil
	}

	return mcp.NewToolResultText(summaryText), nil
}

func indexRepoHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repoPath := getStringArg(request.Params.Arguments, "repo_path")

	mgr := repository.NewManager()
	info, err := mgr.DetectFrom(repoPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to detect repo: %v", err)), nil
	}

	rs, err := GetOrCreateRepoSession(ctx, info.Root)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, err := rs.Indexer.Index(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Indexing failed: %v", err)), nil
	}

	if err := ledger.Append(repoPath, ledger.Event{
		Kind:    "index",
		Action:  "reindex",
		Files:   result.Changed,
		Trigger: "manual",
	}); err != nil {
		log.Printf("ledger: append error: %v", err)
	}

	return mcp.NewToolResultText(fmt.Sprintf("Repository re-indexed successfully.\nChanged: %d\nSkipped: %d\nDeleted: %d\nTotal: %d", result.Changed, result.Skipped, result.Deleted, result.Total)), nil
}

func rememberHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repoPath := getStringArg(request.Params.Arguments, "repo_path")
	key := getStringArg(request.Params.Arguments, "key")
	fact := getStringArg(request.Params.Arguments, "fact")

	if repoPath == "" || key == "" || fact == "" {
		return mcp.NewToolResultError("repo_path, key, and fact are required"), nil
	}

	if len(fact) > maxFactBytes {
		return mcp.NewToolResultError(fmt.Sprintf("fact too large: %d bytes (max %d)", len(fact), maxFactBytes)), nil
	}

	rs, err := GetOrCreateRepoSession(ctx, repoPath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := rs.RememberFact(key, fact); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to remember: %v", err)), nil
	}

	summary := fact
	if len([]rune(summary)) > 200 {
		summary = string([]rune(summary)[:200])
	}
	if err := ledger.Append(repoPath, ledger.Event{
		Kind:    "fact",
		Action:  "add",
		Summary: summary,
	}); err != nil {
		log.Printf("ledger: append error: %v", err)
	}

	return mcp.NewToolResultText(fmt.Sprintf("Remembered %q. Use recall(query=%q) to retrieve it.", key, key)), nil
}

func stashContextHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repoPath := getStringArg(request.Params.Arguments, "repo_path")
	content := getStringArg(request.Params.Arguments, "content")
	label := getStringArg(request.Params.Arguments, "label")

	if repoPath == "" || content == "" {
		return mcp.NewToolResultError("repo_path and content are required"), nil
	}

	if len(content) > maxStashContentBytes {
		return mcp.NewToolResultError(fmt.Sprintf("content too large: %d bytes (max %d)", len(content), maxStashContentBytes)), nil
	}

	rs, err := GetOrCreateRepoSession(ctx, repoPath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	e, err := rs.Stash.Store(content, label)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to stash: %v", err)), nil
	}

	summary := label
	if summary == "" {
		r := []rune(content)
		if len(r) > 40 {
			summary = string(r[:40]) + "..."
		} else {
			summary = string(r)
		}
	}
	if err := ledger.Append(repoPath, ledger.Event{
		Kind:    "stash",
		Action:  "create",
		Handle:  e.Handle,
		Tokens:  e.Tokens,
		Summary: summary,
	}); err != nil {
		log.Printf("ledger: append error: %v", err)
	}

	labelPart := ""
	if e.Label != "" {
		labelPart = fmt.Sprintf(" %q", e.Label)
	}
	return mcp.NewToolResultText(fmt.Sprintf(
		"Stashed%s → %s (~%d tokens kept out of context). Read only what you need with recall(source=%q, query=...).",
		labelPart, e.Handle, e.Tokens, e.Handle,
	)), nil
}

func recallHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repoPath := getStringArg(request.Params.Arguments, "repo_path")
	query := getStringArg(request.Params.Arguments, "query")
	source := getStringArg(request.Params.Arguments, "source")

	if repoPath == "" {
		return mcp.NewToolResultError("repo_path is required"), nil
	}

	rs, err := GetOrCreateRepoSession(ctx, repoPath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	budget := parseBudget(request.Params.Arguments, 500)

	// A concrete stash handle: query-scope just that blob.
	if source != "" && source != "facts" {
		out, err := rs.Stash.Query(source, query, budget)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := ledger.Append(repoPath, ledger.Event{
			Kind:   "recall",
			Action: "read",
			Query:  query,
			Source: source,
		}); err != nil {
			log.Printf("ledger: append error: %v", err)
		}
		return mcp.NewToolResultText(out), nil
	}

	// Otherwise search remembered facts (and, when source is unset, list stashes).
	var lines []string
	if facts := rs.RecallFacts(query); len(facts) > 0 {
		lines = append(lines, "Facts:")
		for _, f := range facts {
			lines = append(lines, "  "+f)
		}
	}

	if source == "" {
		ql := strings.ToLower(strings.TrimSpace(query))
		var stashLines []string
		for _, e := range rs.Stash.List() {
			if ql == "" || strings.Contains(strings.ToLower(e.Label+" "+e.Handle), ql) {
				stashLines = append(stashLines, fmt.Sprintf("  %s — %s (~%d tok)", e.Handle, e.Label, e.Tokens))
			}
		}
		if len(stashLines) > 0 {
			lines = append(lines, "Stashes (recall with source=<handle>):")
			lines = append(lines, stashLines...)
		}
	}

	if err := ledger.Append(repoPath, ledger.Event{
		Kind:   "recall",
		Action: "read",
		Query:  query,
		Source: source,
	}); err != nil {
		log.Printf("ledger: append error: %v", err)
	}

	if len(lines) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("Nothing remembered or stashed matches %q.", query)), nil
	}

	return mcp.NewToolResultText(trimToBudget(lines, budget)), nil
}

func sessionBriefHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repoPath := getStringArg(request.Params.Arguments, "repo_path")
	if repoPath == "" {
		return mcp.NewToolResultError("repo_path is required"), nil
	}

	scope := ledger.ScopeLast
	if s := getStringArg(request.Params.Arguments, "scope"); s != "" {
		switch s {
		case "last", "today", "all":
			scope = ledger.Scope(s)
		default:
			return mcp.NewToolResultError(fmt.Sprintf("invalid scope %q; must be 'last', 'today', or 'all'", s)), nil
		}
	}

	budgetStr := getStringArg(request.Params.Arguments, "budget")
	budget := 300
	if b, err := fmt.Sscanf(budgetStr, "%d", &budget); budgetStr != "" && (err != nil || b != 1) {
		switch budgetStr {
		case "small":
			budget = 300
		case "medium":
			budget = 1500
		case "large":
			budget = 3000
		default:
			budget = 300
		}
	}

	sessions := 0
	if s := getStringArg(request.Params.Arguments, "sessions"); s != "" {
		if n, err := fmt.Sscanf(s, "%d", &sessions); err != nil || n != 1 || sessions < 0 {
			sessions = 0
		}
	}

	summary, err := ledger.SessionBrief(repoPath, scope, budget, sessions)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("session_brief error: %v", err)), nil
	}
	return mcp.NewToolResultText(summary), nil
}

// trimToBudget joins lines up to an approximate token budget (len/4), appending
// a truncation note when it cuts off.
func trimToBudget(lines []string, budget int) string {
	maxChars := budget * 4
	var b strings.Builder
	for i, ln := range lines {
		if b.Len()+len(ln)+1 > maxChars && b.Len() > 0 {
			fmt.Fprintf(&b, "... +%d more (raise budget to see them)\n", len(lines)-i)
			break
		}
		b.WriteString(ln)
		b.WriteByte('\n')
	}
	return b.String()
}
