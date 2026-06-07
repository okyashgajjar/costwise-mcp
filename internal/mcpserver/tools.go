package mcpserver

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/okyashgajjar/costaffective-mcp/internal/answertype"
	"github.com/okyashgajjar/costaffective-mcp/internal/repository"
	"github.com/okyashgajjar/costaffective-mcp/internal/retrieval"
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

	// grep_code
	s.AddTool(mcp.NewTool("grep_code",
		mcp.WithDescription("Exact text search across the repository using ripgrep-like functionality."),
		mcp.WithString("repo_path", mcp.Required(), mcp.Description("Absolute path to the repository root")),
		mcp.WithString("pattern", mcp.Required(), mcp.Description("Regex or exact text pattern to search")),
		mcp.WithString("budget", mcp.Description("Token budget: small (500), medium (1500), large (3000). Default is medium."), mcp.Enum("small", "medium", "large")),
	), grepCodeHandler)

	// get_repository_summary
	s.AddTool(mcp.NewTool("get_repository_summary",
		mcp.WithDescription("Get a high-level overview of the entire repository (modules, languages, file counts)."),
		mcp.WithString("repo_path", mcp.Required(), mcp.Description("Absolute path to the repository root")),
	), repoSummaryHandler)

	// index_repository
	s.AddTool(mcp.NewTool("index_repository",
		mcp.WithDescription("Manually trigger a re-index of the repository. Usually unnecessary as the watcher auto-reindexes."),
		mcp.WithString("repo_path", mcp.Required(), mcp.Description("Absolute path to the repository root")),
	), indexRepoHandler)
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

func grepCodeHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repoPath := getStringArg(request.Params.Arguments, "repo_path")
	pattern := getStringArg(request.Params.Arguments, "pattern")

	rs, err := GetOrCreateRepoSession(ctx, repoPath)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ret := retrieval.NewGrepRetriever()
	if err := ret.Initialize(ctx, rs.Repo); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	results, err := ret.Retrieve(ctx, pattern)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	atc := answertype.Classification{Type: answertype.Explanation} // Generic
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

	_, summaryText := retrieval.BuildRepositorySummary(rs.Knowledge)
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

	return mcp.NewToolResultText(fmt.Sprintf("Repository re-indexed successfully.\nChanged: %d\nSkipped: %d\nDeleted: %d\nTotal: %d", result.Changed, result.Skipped, result.Deleted, result.Total)), nil
}
