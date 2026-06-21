// Package skill delivers the costwise-session session-awareness guidance.
//
// The same canonical policy (policy.md, embedded) is surfaced two ways:
//   - via the MCP server `instructions` field (automatic, cross-IDE, zero install),
//   - as native rules files written by `costwise skill install`.
//
// Native delivery targets the file each client actually reads, choosing one path
// per client to avoid loading the policy twice (which would cost context budget):
//   - Claude Code  → .claude/skills/<name>/SKILL.md (own file; on-demand skill)
//   - Codex CLI    → AGENTS.md
//   - opencode     → AGENTS.md
//   - Antigravity/Gemini → GEMINI.md
//   - OpenClaw     → ~/.openclaw/workspace/AGENTS.md
//
// AGENTS.md is the cross-tool standard also read by Cursor, Windsurf, Copilot,
// Zed and Aider, so a single project-root AGENTS.md covers them at once.
//
// The goal is to make the model use the cache-reducing tools (stash_context,
// recall, remember) and narrow retrieval by default, without the user having to
// re-ask each turn.
package skill

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Name is the skill's identifier; it is also the SKILL.md directory name and the
// /command users type in Claude Code.
const Name = "costwise-session"

// description is the auto-trigger hint loaded into the model's context. Kept
// broad so the skill applies to general work in a costwise-connected repo.
const description = "Keep the session cheap when working in a costwise-connected repository: route large output through stash_context/recall, persist durable facts with remember, and prefer narrow retrieval (search_code, find_symbol, get_repository_summary) over reading whole files."

//go:embed policy.md
var policy string

// Policy returns the canonical session policy body.
func Policy() string {
	return strings.TrimSpace(policy)
}

// Instructions returns the text for the MCP server `instructions` field. This is
// the cross-IDE, zero-install delivery path.
func Instructions() string {
	return Policy()
}

// SkillMarkdown returns a complete SKILL.md document. Frontmatter is restricted
// to the portable Agent Skills fields (name, description) so the file is also
// usable by other tools that read the open standard.
func SkillMarkdown() string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: %s\n", Name)
	fmt.Fprintf(&b, "description: %s\n", description)
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", Name)
	b.WriteString(Policy())
	b.WriteString("\n")
	return b.String()
}

// Scope selects which locations install/uninstall act on.
type Scope int

const (
	ScopeGlobal  Scope = iota // per-user files (~/.codex/AGENTS.md, ~/.claude/skills, …)
	ScopeProject              // files in the current working directory
	ScopeBoth
)

// ParseScope maps a CLI string to a Scope. Empty defaults to ScopeBoth.
func ParseScope(s string) (Scope, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "both":
		return ScopeBoth, nil
	case "global":
		return ScopeGlobal, nil
	case "project", "local":
		return ScopeProject, nil
	default:
		return 0, fmt.Errorf("unknown scope %q (use global|project|both)", s)
	}
}

func scopeLabel(s Scope) string {
	if s == ScopeProject {
		return "project"
	}
	return "global"
}

// fileMode is how we manage a target file.
type fileMode int

const (
	ownFile   fileMode = iota // a file we fully own (SKILL.md): write/replace wholesale
	blockFile                 // a shared user file: manage only our delimited block
)

// block delimiters for shared files (AGENTS.md / GEMINI.md). We only ever touch
// the span between them, so user content is preserved.
const (
	blockStart = "<!-- costwise-session:start (managed by `costwise skill` — do not edit) -->"
	blockEnd   = "<!-- costwise-session:end -->"
)

type target struct {
	id    string
	scope Scope
	mode  fileMode
	path  func() (string, error)
}

func allTargets() []target {
	return []target{
		// ── Global (per-user) ────────────────────────────────────────────
		{id: "claude", scope: ScopeGlobal, mode: ownFile, path: func() (string, error) {
			return homeJoin(".claude", "skills", Name, "SKILL.md")
		}},
		{id: "codex", scope: ScopeGlobal, mode: blockFile, path: func() (string, error) {
			return homeJoin(".codex", "AGENTS.md")
		}},
		{id: "opencode", scope: ScopeGlobal, mode: blockFile, path: func() (string, error) {
			cfg, err := xdgConfigHome()
			if err != nil {
				return "", err
			}
			return filepath.Join(cfg, "opencode", "AGENTS.md"), nil
		}},
		{id: "gemini", scope: ScopeGlobal, mode: blockFile, path: func() (string, error) {
			return homeJoin(".gemini", "GEMINI.md")
		}},
		{id: "openclaw", scope: ScopeGlobal, mode: blockFile, path: func() (string, error) {
			return homeJoin(".openclaw", "workspace", "AGENTS.md")
		}},

		// ── Project (current working directory) ──────────────────────────
		// AGENTS.md is read by Codex, opencode, Cursor, Windsurf, Copilot,
		// Antigravity, Gemini, Zed and Aider — one file covers them all.
		{id: "agents", scope: ScopeProject, mode: blockFile, path: func() (string, error) {
			return projectJoin("AGENTS.md")
		}},
		{id: "claude", scope: ScopeProject, mode: ownFile, path: func() (string, error) {
			return projectJoin(".claude", "skills", Name, "SKILL.md")
		}},
	}
}

func targetsForScope(s Scope) []target {
	var out []target
	for _, t := range allTargets() {
		if s == ScopeBoth || t.scope == s {
			out = append(out, t)
		}
	}
	return out
}

// Result describes one install/uninstall action for reporting.
type Result struct {
	Target string // client id, e.g. "codex"
	Scope  string // "global" | "project"
	Path   string // file path acted on
	Action string // created | updated | unchanged | removed | not-found
}

// Install writes the session guidance to every target in scope. Shared files
// (AGENTS.md / GEMINI.md) get a delimited block; Claude's SKILL.md is written
// wholesale. Re-running is idempotent (reports "unchanged").
func Install(scope Scope) ([]Result, error) {
	var results []Result
	for _, t := range targetsForScope(scope) {
		path, err := t.path()
		if err != nil {
			return nil, err
		}
		var action string
		if t.mode == ownFile {
			action, err = writeOwnFile(path, SkillMarkdown())
		} else {
			action, err = upsertBlock(path, skillBlock())
		}
		if err != nil {
			return nil, fmt.Errorf("%s (%s): %w", t.id, path, err)
		}
		results = append(results, Result{Target: t.id, Scope: scopeLabel(t.scope), Path: path, Action: action})
	}
	return results, nil
}

// Uninstall removes the session guidance from every target in scope. For shared
// files only our block is stripped; the file is deleted only if nothing else
// remains.
func Uninstall(scope Scope) ([]Result, error) {
	var results []Result
	for _, t := range targetsForScope(scope) {
		path, err := t.path()
		if err != nil {
			return nil, err
		}
		var action string
		if t.mode == ownFile {
			action, err = removeOwnFile(path)
		} else {
			action, err = removeBlock(path)
		}
		if err != nil {
			return nil, fmt.Errorf("%s (%s): %w", t.id, path, err)
		}
		results = append(results, Result{Target: t.id, Scope: scopeLabel(t.scope), Path: path, Action: action})
	}
	return results, nil
}

// skillBlock is the managed region inserted into shared files. It has no trailing
// newline so replacement is byte-stable for idempotency.
func skillBlock() string {
	return blockStart + "\n\n## " + Name + "\n\n" + Policy() + "\n\n" + blockEnd
}

// writeOwnFile writes a file we fully own, creating parents as needed.
func writeOwnFile(path, content string) (string, error) {
	if existing, err := os.ReadFile(path); err == nil {
		if string(existing) == content {
			return "unchanged", nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	action := "created"
	if _, err := os.Stat(path); err == nil {
		action = "updated"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return action, nil
}

func removeOwnFile(path string) (string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "not-found", nil
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	_ = os.Remove(filepath.Dir(path)) // best-effort; only succeeds if empty
	return "removed", nil
}

// upsertBlock inserts or replaces the managed block in a shared file, preserving
// all surrounding user content.
func upsertBlock(path, blk string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		if err := os.WriteFile(path, []byte(blk+"\n"), 0o644); err != nil {
			return "", err
		}
		return "created", nil
	}

	existing := string(data)
	if s := strings.Index(existing, blockStart); s != -1 {
		if rel := strings.Index(existing[s:], blockEnd); rel != -1 {
			e := s + rel + len(blockEnd)
			updated := existing[:s] + blk + existing[e:]
			if updated == existing {
				return "unchanged", nil
			}
			if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
				return "", err
			}
			return "updated", nil
		}
	}

	// Append, separated from existing content by a blank line.
	prefix := existing
	if !strings.HasSuffix(prefix, "\n") {
		prefix += "\n"
	}
	prefix += "\n"
	if err := os.WriteFile(path, []byte(prefix+blk+"\n"), 0o644); err != nil {
		return "", err
	}
	return "updated", nil
}

// removeBlock strips the managed block from a shared file. If nothing else
// remains the file is deleted; otherwise the surrounding content is kept.
func removeBlock(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "not-found", nil
		}
		return "", err
	}
	existing := string(data)
	s := strings.Index(existing, blockStart)
	if s == -1 {
		return "not-found", nil
	}
	rel := strings.Index(existing[s:], blockEnd)
	if rel == -1 {
		return "not-found", nil
	}
	e := s + rel + len(blockEnd)

	rest := strings.TrimSpace(existing[:s] + existing[e:])
	if rest == "" {
		if err := os.Remove(path); err != nil {
			return "", err
		}
		return "removed", nil
	}
	if err := os.WriteFile(path, []byte(rest+"\n"), 0o644); err != nil {
		return "", err
	}
	return "removed", nil
}

func homeJoin(parts ...string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{home}, parts...)...), nil
}

func xdgConfigHome() (string, error) {
	if x := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); x != "" {
		return x, nil
	}
	return homeJoin(".config")
}

func projectJoin(parts ...string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{cwd}, parts...)...), nil
}
