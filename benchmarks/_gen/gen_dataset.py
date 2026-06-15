#!/usr/bin/env python3
"""Generate an independent 'definition' ground-truth dataset for a repo.

Ground truth comes from regex declaration matching (NOT our tree-sitter parser),
so the resulting benchmark is a fair, independent test. Only uniquely-named
declarations (the symbol's declaration appears in exactly one source file) are
kept, to avoid the inherent ambiguity of generic names. Test/vendor dirs are
skipped to match what the retriever is allowed to return.
"""
import os, re, json, sys, random

random.seed(7)  # deterministic sampling

SKIP_DIRS = {".git", "node_modules", "vendor", "target", "build", "dist",
             "test", "tests", "spec", "specs", "examples", "fixtures", "testdata", "benchmark", "benchmarks"}

LANGS = {
    "rust":   {"exts": [".rs"],
               "pats": [r'^\s*(?:pub\s+)?(?:async\s+)?fn\s+([a-zA-Z_]\w{3,})',
                        r'^\s*(?:pub\s+)?struct\s+([A-Z]\w{3,})',
                        r'^\s*(?:pub\s+)?enum\s+([A-Z]\w{3,})',
                        r'^\s*(?:pub\s+)?trait\s+([A-Z]\w{3,})']},
    "java":   {"exts": [".java"],
               "pats": [r'\b(?:class|interface|enum)\s+([A-Z]\w{3,})']},
    "c":      {"exts": [".c"],
               "pats": [r'^[A-Za-z_][\w\s\*]*?\b([a-z_]\w{3,})\s*\([^;{]*\)\s*\{',
                        r'\bstruct\s+([a-zA-Z_]\w{3,})\s*\{',
                        r'\btypedef\s+.*\b([A-Za-z_]\w{3,})\s*;']},
    "cpp":    {"exts": [".cc", ".cpp", ".cxx", ".h", ".hpp"],
               "pats": [r'\bclass\s+([A-Z]\w{3,})',
                        r'\bstruct\s+([A-Za-z_]\w{3,})\s*\{']},
    "csharp": {"exts": [".cs"],
               "pats": [r'\b(?:class|interface|struct|enum)\s+([A-Z]\w{3,})']},
    "ruby":   {"exts": [".rb"],
               "pats": [r'^\s*(?:class|module)\s+([A-Z]\w{3,})',
                        r'^\s*def\s+([a-z_]\w{3,})']},
    "php":    {"exts": [".php"],
               "pats": [r'\b(?:class|interface|trait)\s+([A-Z]\w{3,})',
                        r'^\s*(?:public|private|protected|static|\s)*function\s+([a-zA-Z_]\w{3,})']},
}

def walk(root, exts):
    for dp, dns, fns in os.walk(root):
        dns[:] = [d for d in dns if d.lower() not in SKIP_DIRS and not d.startswith(".")]
        for f in fns:
            if any(f.endswith(e) for e in exts):
                yield os.path.join(dp, f)

def main():
    repo, lang, outdir = sys.argv[1], sys.argv[2], sys.argv[3]
    cfg = LANGS[lang]
    pats = [re.compile(p) for p in cfg["pats"]]
    sym_files = {}  # name -> set(relfile)
    for path in walk(repo, cfg["exts"]):
        rel = os.path.relpath(path, repo)
        try:
            with open(path, errors="ignore") as fh:
                for line in fh:
                    for pat in pats:
                        m = pat.search(line)
                        if m:
                            sym_files.setdefault(m.group(1), set()).add(rel)
        except Exception:
            continue
    # Keep uniquely-declared symbols only.
    unique = [(n, list(fs)[0]) for n, fs in sym_files.items() if len(fs) == 1]
    random.shuffle(unique)
    sample = unique[:50]
    ds = [{"query": f"Where is {n} defined?", "category": "definition",
           "expected_retriever": "treesitter", "expected_file": f,
           "expected_symbols": [n]} for n, f in sample]
    os.makedirs(outdir, exist_ok=True)
    with open(os.path.join(outdir, "definitions.json"), "w") as fh:
        json.dump(ds, fh, indent=2)
    print(f"{lang}: {len(sym_files)} decls, {len(unique)} unique, sampled {len(sample)}")

if __name__ == "__main__":
    main()
