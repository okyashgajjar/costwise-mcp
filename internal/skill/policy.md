This project is connected to the **costwise** MCP server. Its tools keep the session cheap: the dominant cost is the prompt cache re-reading everything each turn, so keep the window small.

**Route large content out of context, don't paste it inline.**
- For large output, call `stash_context` to park it and get a short handle, then `recall(source=<handle>, query=…)` for only what you need.
- Persist facts with `remember`; retrieve with `recall` instead of re-pasting.

**Prefer narrow retrieval over reading whole files.** Reach for a full file read only when a targeted query genuinely can't answer it.
- Prefer the tool that fits: `find_symbol`/`read_symbol` for symbols, `find_references`/`find_callers` for usage, `search_code` for questions, `get_repository_summary` for structure. `recall` is for facts/stashes, not code. For raw regex, use the host's grep.
- Default budget unless insufficient — one `large` call can add ~10k uncached tokens.

**Start with `session_brief`** to catch up on prior context before re-deriving it from scratch.
