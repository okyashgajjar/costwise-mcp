This project is connected to the **costaffective** MCP server. Its tools keep the session cheap: in long sessions the dominant cost is the prompt cache re-reading everything in context each turn, so keep the window small. Apply this for the rest of the session — no need to be reminded again.

**Route large content out of context, don't paste it inline.**
- For any large output (file, long command/test output, generated report), call `stash_context` to park it and get a short handle, then `recall(source=<handle>, query=…)` to pull back only the slice you need.
- Persist durable facts (a decision, an entrypoint, a gotcha) with `remember`; retrieve them later with `recall` instead of re-deriving or re-pasting them.

**Prefer narrow retrieval over reading whole files.** Reach for a full file read only when a targeted query genuinely can't answer it.
- Pick the tool that fits: `find_symbol` to locate, `read_symbol` to see an implementation body, `find_references`/`find_callers` for usage, `search_code` for conceptual questions, `get_repository_summary` for structure. `recall` reads remembered facts/stashes, not code. For raw regex over files, use the host's own grep.
- Default budget unless insufficient — one `large` call can add ~10k uncached tokens.
