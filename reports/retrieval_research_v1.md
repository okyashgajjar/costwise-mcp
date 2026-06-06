# Cross-Repository Retrieval Research Study V1

## 1. Executive Summary
This study establishes a scientifically rigorous, deterministic baseline for repository-level code intelligence retrieval architectures. By evaluating **9 retrieval techniques** across **5 distinct codebases** containing **700 ground-truth queries**, we resolve the primary research question: *"Which retrieval architecture provides the highest retrieval correctness per token across real-world coding repositories?"*

- **Most Accurate Retriever:** treesitter (22.3% mean accuracy)
- **Most Token Efficient Retriever:** treesitter (3.42 acc/100 toks)
- **Most Cost Efficient Retriever:** treesitter (68 correct/dollar)
- **Best Generalization:** flowgraph (StdDev: 0.00%)
- **Recommended Production Default:** treesitter (Research Score: 35.85)

## 2. Methodology
Every retriever was evaluated on exactly the same queries using strict validation rules. Substring file matches were strictly disabled to prevent accuracy inflation. Instead, exact file paths or basename matching was enforced. Evidence coverage was calculated using a validator that parses context snippets to check for the presence of ground-truth keywords and symbol names. The Research Score is computed as follows:
$$
ResearchScore = 0.40 \times Accuracy + 0.25 \times Coverage + 0.20 \times TokenEfficiency + 0.10 \times LatencyEfficiency + 0.05 \times ValidationPassRate
$$
All metrics were aggregated over 140 queries per repository (700 queries total).

## 3. Dataset Description
The evaluation uses a deterministic ground-truth suite containing 7 distinct query categories:
- **definition**: Queries looking for where symbols or classes are defined.
- **reference**: Queries locating occurrences or uses of symbols.
- **caller**: Call-graph query edges verifying callers of functions.
- **repository**: Overall high-level repository structure queries.
- **architecture**: Queries regarding subsystem designs and relationships.
- **flow**: Queries tracing functional execution graphs.
- **concept**: General semantic explanations of classes or packages.

## 4. Repository Overview
Five repositories of varying size and language composition were evaluated:
- **Aider**: located at `/home/mryg/Testings/aider`
- **OpenCode**: located at `/home/mryg/Testings/opencode`
- **Continue**: located at `/home/mryg/Testings/continue-main`
- **Claude Code**: located at `/home/mryg/Testings/claude-code`
- **MyCLI**: located at `/home/mryg/CostAffective-CLI/CLI`

## 5. Retriever Overview
The 9 benchmarked retrievers represent a matrix of indexing methods:
- **naive**: Standard linear scanner of core files.
- **grep**: Text-based ripgrep scanning.
- **fts**: SQLite-based full-text index.
- **treesitter**: AST parser lookup for definitions.
- **reference**: Symbol-reference cross-indexer.
- **callgraph**: Static callgraph builder.
- **architecture**: Directory structures and README documentation indexer.
- **flowgraph**: Traversed execution-flow paths indexer.
- **auto**: Auto-routing retriever.

## 6. Accuracy Results
The mean accuracy results across all codebases:

| Retriever | Mean Accuracy | StdDev Accuracy | Mean Retrieval Score |
|---|---|---|---|
| treesitter | 22.29% | 3.21% | 0.325 |
| auto | 17.71% | 4.85% | 0.371 |
| architecture | 11.71% | 7.78% | 0.211 |
| naive | 8.57% | 7.00% | 0.163 |
| grep | 7.00% | 9.17% | 0.264 |
| fts | 5.43% | 5.39% | 0.095 |
| flowgraph | 0.00% | 0.00% | 0.139 |
| reference | 0.00% | 0.00% | 0.132 |
| callgraph | 0.00% | 0.00% | 0.069 |

## 7. Evidence Coverage Results
Evidence keyword and symbol coverage rates inside retrieval outputs:

| Retriever | Keyword Coverage | Evidence Pass Rate |
|---|---|---|
| auto | 79.00% | 100.00% |
| treesitter | 71.71% | 100.00% |
| architecture | 51.93% | 100.00% |
| flowgraph | 42.93% | 100.00% |
| grep | 41.57% | 100.00% |
| reference | 26.86% | 100.00% |
| fts | 24.29% | 100.00% |
| callgraph | 18.93% | 100.00% |
| naive | 12.50% | 100.00% |

## 8. Token Efficiency Results
The token budget constraints mandate minimizing prompt tokens while retaining correctness:

| Retriever | Acc/100 Tokens | Mean Avg Tokens |
|---|---|---|
| treesitter | 3.42 | 685 |
| auto | 1.65 | 1077 |
| architecture | 1.19 | 1219 |
| fts | 0.78 | 673 |
| grep | 0.37 | 1724 |
| naive | 0.37 | 1744 |
| flowgraph | 0.00 | 57 |
| reference | 0.00 | 208 |
| callgraph | 0.00 | 269 |

## 9. Latency Results
Speed performance and query throughput:

| Retriever | Mean Latency | Correct per Second |
|---|---|---|
| naive | 0.0ms | 0.00 |
| flowgraph | 34.6ms | 0.00 |
| callgraph | 36.8ms | 0.00 |
| reference | 42.3ms | 0.00 |
| architecture | 57.1ms | 34.33 |
| fts | 116.4ms | 0.46 |
| treesitter | 119.8ms | 33.24 |
| auto | 351.6ms | 1.29 |
| grep | 967.9ms | 0.24 |

## 10. Cross-Repository Analysis
Standard deviation measures consistency. Lower StdDev shows better generalization across different codebase architectures:

| Retriever | StdDev Accuracy | Mean Accuracy |
|---|---|---|
| flowgraph | 0.00% | 0.00% |
| reference | 0.00% | 0.00% |
| callgraph | 0.00% | 0.00% |
| treesitter | 3.21% | 22.29% |
| auto | 4.85% | 17.71% |
| fts | 5.39% | 5.43% |
| naive | 7.00% | 8.57% |
| architecture | 7.78% | 11.71% |
| grep | 9.17% | 7.00% |

## 11. Category Analysis
Retrieval performance segmented by benchmark query categories:

| Category | Best Accuracy Retriever | Best Token Eff | Best Latency | Worst Retriever |
|---|---|---|---|---|
| definition | treesitter | treesitter | naive | reference |
| reference | naive | naive | naive | grep |
| caller | treesitter | treesitter | naive | reference |
| repository | naive | fts | naive | treesitter |
| architecture | naive | naive | naive | grep |
| flow | naive | naive | naive | grep |
| concept | naive | naive | naive | grep |

## 12. Global Leaderboards
Ranks sorted by Research Score:

| Rank | Retriever | Research Score | Mean Accuracy | Mean Avg Tokens | Mean Latency |
|---|---|---|---|---|---|
| 1 | **treesitter** | 35.85 | 22.3% | 685 | 120ms |
| 2 | **auto** | 32.29 | 17.7% | 1077 | 352ms |
| 3 | **architecture** | 26.18 | 11.7% | 1219 | 57ms |
| 4 | **grep** | 18.29 | 7.0% | 1724 | 968ms |
| 5 | **flowgraph** | 15.73 | 0.0% | 57 | 35ms |
| 6 | **fts** | 13.44 | 5.4% | 673 | 116ms |
| 7 | **reference** | 11.71 | 0.0% | 208 | 42ms |
| 8 | **naive** | 11.63 | 8.6% | 1744 | 0ms |
| 9 | **callgraph** | 9.73 | 0.0% | 269 | 37ms |

## 13. Recommendations
Based on the baseline Research Score, we suggest:
1. **Production Default:** Use `treesitter` as the default code search engine today. It provides the optimal balance of accuracy, speed, and validation safety.
2. **For Token-Constrained Environments:** Use `treesitter` because it minimizes context inflation.
3. **Category-Aware Routing:** Deploy the auto-router to dispatch definitions to `treesitter`, architectural queries to `architecture`, and text-based keywords to `fts`.

## 14. Future Work
1. **Index Compression:** Investigate ways to prune symbol trees to lower average tokens for AST-based retrievers.
2. **Dynamic Call graphs:** Extend callgraph parsing to handle dynamic runtime configurations.
3. **Enhanced Auto Router:** Leverage FTS context heuristics to make routing decisions without incurring LLM prompt overhead.
