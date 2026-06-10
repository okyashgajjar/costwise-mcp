package repository

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type RepositoryInfo struct {
	Root       string   `json:"root"`
	GitRoot    string   `json:"git_root,omitempty"`
	HasGit     bool     `json:"has_git"`
	Languages  []string `json:"languages"`
	ReadmePath string   `json:"readme_path,omitempty"`
	DocsDir    string   `json:"docs_dir,omitempty"`
	TestsDir   string   `json:"tests_dir,omitempty"`
	Branch     string   `json:"branch,omitempty"`
}

type RepositoryStatus struct {
	Info      RepositoryInfo `json:"info"`
	FileCount int            `json:"file_count"`
	DirCount  int            `json:"dir_count"`
	SizeBytes int64          `json:"size_bytes"`
	GitStatus string         `json:"git_status,omitempty"`
}

type RepositoryMetadata struct {
	Info       RepositoryInfo `json:"info"`
	TopFiles   []string       `json:"top_files"`
	TopDirs    []string       `json:"top_dirs"`
	TotalFiles int            `json:"total_files"`
	TotalLines int            `json:"total_lines"`
}

type Manager struct {
	info *RepositoryInfo
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) DetectFrom(root string) (*RepositoryInfo, error) {
	info := &RepositoryInfo{Root: root}

	absRoot, err := filepath.Abs(root)
	if err == nil {
		info.Root = absRoot
	}

	if gitRoot, hasGit := findGitRoot(info.Root); hasGit {
		info.GitRoot = gitRoot
		info.HasGit = true
		info.Branch = getGitBranch(gitRoot)
	}

	info.Languages = detectLanguages(info.Root)
	info.ReadmePath = findReadme(info.Root)
	info.DocsDir = findDir(info.Root, []string{"docs", "doc", "documentation"})
	info.TestsDir = findDir(info.Root, []string{"tests", "test", "__tests__", "spec"})

	m.info = info
	return info, nil
}

func (m *Manager) Detect() (*RepositoryInfo, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	info := &RepositoryInfo{Root: cwd}

	if gitRoot, hasGit := findGitRoot(cwd); hasGit {
		info.GitRoot = gitRoot
		info.HasGit = true
		info.Root = gitRoot
		info.Branch = getGitBranch(gitRoot)
	}

	info.Languages = detectLanguages(info.Root)
	info.ReadmePath = findReadme(info.Root)
	info.DocsDir = findDir(info.Root, []string{"docs", "doc", "documentation"})
	info.TestsDir = findDir(info.Root, []string{"tests", "test", "__tests__", "spec"})

	m.info = info
	return info, nil
}

func (m *Manager) Status() (*RepositoryStatus, error) {
	if m.info == nil {
		if _, err := m.Detect(); err != nil {
			return nil, err
		}
	}

	status := &RepositoryStatus{Info: *m.info}

	_ = filepath.Walk(m.info.Root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() && shouldSkipDir(path) {
			return filepath.SkipDir
		}
		if fi.IsDir() {
			status.DirCount++
		} else {
			status.FileCount++
			status.SizeBytes += fi.Size()
		}
		return nil
	})

	if m.info.HasGit {
		status.GitStatus = getGitStatus(m.info.GitRoot)
	}

	return status, nil
}

func (m *Manager) Summary() (*RepositoryMetadata, error) {
	if m.info == nil {
		if _, err := m.Detect(); err != nil {
			return nil, err
		}
	}

	meta := &RepositoryMetadata{Info: *m.info}

	entries, _ := os.ReadDir(m.info.Root)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if entry.IsDir() {
			meta.TopDirs = append(meta.TopDirs, entry.Name())
		} else {
			meta.TopFiles = append(meta.TopFiles, entry.Name())
		}
	}
	sort.Strings(meta.TopFiles)
	sort.Strings(meta.TopDirs)

	_ = filepath.Walk(m.info.Root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() && shouldSkipDir(path) {
			return filepath.SkipDir
		}
		if !fi.IsDir() {
			meta.TotalFiles++
		}
		return nil
	})

	_ = filepath.Walk(m.info.Root, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		if fi.IsDir() && shouldSkipDir(path) {
			return filepath.SkipDir
		}
		if shouldSkipExt(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		meta.TotalLines += strings.Count(string(data), "\n") + 1
		return nil
	})

	return meta, nil
}

func (m *Manager) Files() ([]string, error) {
	if m.info == nil {
		if _, err := m.Detect(); err != nil {
			return nil, err
		}
	}

	var files []string
	_ = filepath.Walk(m.info.Root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() && shouldSkipDir(path) {
			return filepath.SkipDir
		}
		if !fi.IsDir() {
			rel, err := filepath.Rel(m.info.Root, path)
			if err == nil {
				files = append(files, rel)
			}
		}
		return nil
	})

	sort.Strings(files)
	return files, nil
}

func (m *Manager) GetInfo() *RepositoryInfo {
	return m.info
}

func findGitRoot(dir string) (string, bool) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func getGitBranch(gitRoot string) string {
	data, err := os.ReadFile(filepath.Join(gitRoot, ".git", "HEAD"))
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(string(data))
	if strings.HasPrefix(content, "ref: refs/heads/") {
		return strings.TrimPrefix(content, "ref: refs/heads/")
	}
	return content
}

func getGitStatus(gitRoot string) string {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = gitRoot
	data, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	out := strings.TrimSpace(string(data))
	if out == "" {
		return "clean"
	}
	return fmt.Sprintf("%d files changed", len(strings.Split(out, "\n")))
}

var extToLang = map[string]string{
	".go": "Go", ".py": "Python", ".js": "JavaScript",
	".ts": "TypeScript", ".tsx": "TypeScript", ".jsx": "JavaScript",
	".rs": "Rust", ".rb": "Ruby", ".java": "Java",
	".c": "C", ".cpp": "C++", ".h": "C", ".hpp": "C++",
	".cs": "C#", ".php": "PHP", ".swift": "Swift",
	".kt": "Kotlin", ".scala": "Scala", ".sh": "Shell",
	".sql": "SQL", ".html": "HTML", ".css": "CSS",
	".json": "JSON", ".yaml": "YAML", ".yml": "YAML",
	".md": "Markdown", ".toml": "TOML", ".lua": "Lua",
	".zig": "Zig", ".ex": "Elixir", ".exs": "Elixir",
	".go.mod": "Go", ".sum": "Go",
}

func detectLanguages(root string) []string {
	extensions := make(map[string]int)
	_ = filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() && shouldSkipDir(path) {
			return filepath.SkipDir
		}
		if !fi.IsDir() {
			ext := filepath.Ext(path)
			if ext != "" {
				extensions[ext]++
			}
		}
		return nil
	})

	seen := make(map[string]bool)
	var langs []string
	for ext, count := range extensions {
		if count == 0 {
			continue
		}
		if lang, ok := extToLang[ext]; ok && !seen[lang] {
			seen[lang] = true
			langs = append(langs, lang)
		}
	}
	sort.Strings(langs)
	return langs
}

func findReadme(root string) string {
	candidates := []string{"README.md", "README.txt", "README", "Readme.md", "readme.md"}
	for _, name := range candidates {
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func findDir(root string, candidates []string) string {
	for _, name := range candidates {
		path := filepath.Join(root, name)
		if fi, err := os.Stat(path); err == nil && fi.IsDir() {
			return path
		}
	}
	return ""
}

func shouldSkipDir(path string) bool {
	base := filepath.Base(path)
	switch base {
	case ".git", "node_modules", "vendor", ".venv", "venv",
		"__pycache__", ".next", "dist", "build", "target",
		".idea", ".vscode", ".DS_Store":
		return true
	}
	return false
}

var binaryExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".ico": true, ".svg": true, ".woff": true, ".woff2": true,
	".ttf": true, ".eot": true, ".pdf": true, ".zip": true,
	".tar": true, ".gz": true, ".exe": true, ".dll": true,
	".so": true, ".dylib": true, ".bin": true, ".o": true,
	".a": true, ".pyc": true, ".class": true,
}

func shouldSkipExt(path string) bool {
	return binaryExts[filepath.Ext(path)]
}
