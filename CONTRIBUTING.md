# Contributing to CostAffective

We're thrilled you want to contribute. This project is early, every bit helps.

## What we need help with

- **More language parsers** — we use Tree-sitter. Adding grammar support for your language of choice makes the tools useful to more people.
- **Better retrievers** — the pipeline supports Auto, Symbol, Reference, CallGraph, and File retrievers. New retrieval strategies welcome.
- **Compression improvements** — smarter ways to shrink output per answer type while keeping what matters.
- **Transport layer** — SSE/HTTP transport so Smithery can host this as a remote MCP server.
- **Bug fixes** — especially the known `/tmp/` DB clobbering issue across repos.
- **Documentation** — clearer README, better examples, more benchmarks.

## Quick start

```bash
git clone https://github.com/okyashgajjar/costaffective-mcp.git
cd costaffective-mcp
```

# CGO is mandatory (go-sqlite3 + tree-sitter)
CGO_ENABLED=1 go build ./...
CGO_ENABLED=1 go test ./...
On Ubuntu: sudo apt install gcc libsqlite3-dev
On macOS: xcode-select --install
How to contribute
1. Fork the repo
2. Create a branch: git checkout -b feat/my-feature
3. Make your changes
4. Run the tests: CGO_ENABLED=1 go test ./...
5. Push and open a PR
Keep PRs focused on one thing. Write a clear title and description.
Code style
- Go standard gofmt formatting
- No commented-out code
- Test what you add
- Match the existing patterns (check neighboring files before writing something new)

Questions?
Open an issue at https://github.com/okyashgajjar/costaffective-mcp/issues
