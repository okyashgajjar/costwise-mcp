package doctor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/okyashgajjar/costaffective-mcp/internal/installer"
)

type Status string

const (
	PASS Status = "PASS"
	WARN Status = "WARN"
	FAIL Status = "FAIL"
)

type CheckResult struct {
	Name   string
	Status Status
	Detail string
}

func (r CheckResult) String() string {
	return fmt.Sprintf("%s %s", r.Status, r.Name)
}

func (r CheckResult) StringVerbose() string {
	if r.Detail == "" {
		return fmt.Sprintf("%s %s", r.Status, r.Name)
	}
	return fmt.Sprintf("%s %s\n       %s", r.Status, r.Name, r.Detail)
}

func RunAll() []CheckResult {
	var results []CheckResult
	results = append(results, CheckBinary()...)
	results = append(results, CheckPATH()...)
	results = append(results, CheckMCPConfigs()...)
	results = append(results, CheckMCPStartup()...)
	results = append(results, CheckRepository()...)
	return results
}

func CheckBinary() []CheckResult {
	r := installer.CheckBinary()

	if !r.Exists {
		return []CheckResult{{
			Name:   "Binary Found",
			Status: FAIL,
			Detail: "CostAffective binary was not found.\n\nInstall it:\n  costaffective install",
		}}
	}

	if !r.Executable {
		return []CheckResult{{
			Name:   "Binary Permissions",
			Status: FAIL,
			Detail: fmt.Sprintf("%s exists but is not executable.\n\nFix:\n  chmod +x %s", installer.Tildify(r.Path), installer.Tildify(r.Path)),
		}}
	}

	results := []CheckResult{{
		Name:   "Binary Found",
		Status: PASS,
		Detail: installer.Tildify(r.Path),
	}}

	results = append(results, CheckResult{
		Name:   "Binary Permissions",
		Status: PASS,
	})

	if r.Version != "" {
		results = append(results, CheckResult{
			Name:   "Binary Version",
			Status: PASS,
			Detail: r.Version,
		})
	} else {
		results = append(results, CheckResult{
			Name:   "Binary Version",
			Status: WARN,
			Detail: "Could not determine version",
		})
	}

	return results
}

func CheckPATH() []CheckResult {
	path, err := exec.LookPath("costaffective")
	if err != nil {
		return []CheckResult{{
			Name:   "Binary in PATH",
			Status: WARN,
			Detail: "Binary not found in PATH. MCP will still work because absolute paths are configured.",
		}}
	}
	return []CheckResult{{
		Name:   "Binary in PATH",
		Status: PASS,
		Detail: path,
	}}
}

func CheckMCPConfigs() []CheckResult {
	allTargets := installer.AllTargets()
	var results []CheckResult

	for _, t := range allTargets {
		d := t.Detect(installer.LocationGlobal)
		check := CheckResult{Name: t.DisplayName() + " Config"}

		if !d.AlreadyConfigured {
			if d.Installed {
				check.Status = WARN
				check.Detail = fmt.Sprintf("%s detected but not configured. Run: costaffective install --target %s", t.DisplayName(), t.ID())
			} else {
				check.Status = WARN
				check.Detail = fmt.Sprintf("%s not detected. Install it first.", t.DisplayName())
			}
			results = append(results, check)
			continue
		}

		configPath := d.ConfigPath
		if !installer.Exists(configPath) {
			check.Status = FAIL
			check.Detail = fmt.Sprintf("Config file not found: %s", installer.Tildify(configPath))
			results = append(results, check)
			continue
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			check.Status = FAIL
			check.Detail = fmt.Sprintf("Cannot read %s: %s", installer.Tildify(configPath), err)
			results = append(results, check)
			continue
		}

		status, detail := validateMCPConfig(configPath, data)
		check.Status = status
		check.Detail = detail

		results = append(results, check)
	}

	if len(results) == 0 {
		results = append(results, CheckResult{
			Name:   "MCP Configurations",
			Status: WARN,
			Detail: "No MCP clients detected or configured.",
		})
	}

	return results
}

func validateMCPConfig(configPath string, data []byte) (Status, string) {
	switch filepath.Ext(configPath) {
	case ".toml":
		return validateTOMLMCPConfig(configPath, string(data))
	case ".yaml", ".yml":
		return validateYAMLMCPConfig(configPath, string(data))
	default:
		var parsed interface{}
		if err := json.Unmarshal(data, &parsed); err != nil {
			return FAIL, fmt.Sprintf("Invalid JSON in %s: %s", installer.Tildify(configPath), err)
		}
		return validateJSONMCPConfig(configPath, string(data))
	}
}

func validateJSONMCPConfig(configPath, content string) (Status, string) {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return FAIL, fmt.Sprintf("Invalid JSON in %s: %s", installer.Tildify(configPath), err)
	}

	if command, ok := extractJSONCommand(parsed); ok {
		if isKnownBinaryPath(command) {
			return PASS, installer.Tildify(configPath)
		}
		if command == "costaffective" || command == "costaffective.exe" {
			return WARN, fmt.Sprintf("%s uses a relative binary path", installer.Tildify(configPath))
		}
		return FAIL, fmt.Sprintf("%s points at %q instead of an installed CostAffective binary", installer.Tildify(configPath), command)
	}

	return FAIL, fmt.Sprintf("Could not find MCP command entry in %s", installer.Tildify(configPath))
}

func extractJSONCommand(parsed map[string]interface{}) (string, bool) {
	if mcpServers, ok := parsed["mcpServers"].(map[string]interface{}); ok {
		if server, ok := mcpServers["costaffective"].(map[string]interface{}); ok {
			if command, ok := server["command"].(string); ok {
				return command, true
			}
		}
	}

	if mcp, ok := parsed["mcp"].(map[string]interface{}); ok {
		if server, ok := mcp["costaffective"].(map[string]interface{}); ok {
			if command, ok := server["command"]; ok {
				switch v := command.(type) {
				case string:
					return v, true
				case []interface{}:
					if len(v) > 0 {
						if s, ok := v[0].(string); ok {
							return s, true
						}
					}
				}
			}
		}
	}

	return "", false
}

func validateTOMLMCPConfig(configPath, content string) (Status, string) {
	section := "[mcp_servers.costaffective]"
	if !strings.Contains(content, section) {
		return FAIL, fmt.Sprintf("Missing %s in %s", section, installer.Tildify(configPath))
	}

	for _, candidate := range installer.GetBinaryCandidates() {
		if strings.Contains(content, fmt.Sprintf("command = \"%s\"", candidate)) {
			return PASS, installer.Tildify(configPath)
		}
	}

	if strings.Contains(content, "command = \"costaffective\"") || strings.Contains(content, "command = \"costaffective.exe\"") {
		return WARN, fmt.Sprintf("%s uses a relative binary path", installer.Tildify(configPath))
	}
	return FAIL, fmt.Sprintf("Codex config does not reference an installed binary: %s", installer.Tildify(configPath))
}

func validateYAMLMCPConfig(configPath, content string) (Status, string) {
	for _, candidate := range installer.GetBinaryCandidates() {
		if strings.Contains(content, candidate) {
			return PASS, installer.Tildify(configPath)
		}
	}
	if strings.Contains(content, "costaffective") || strings.Contains(content, "costaffective.exe") {
		return WARN, fmt.Sprintf("%s uses a relative binary path", installer.Tildify(configPath))
	}
	return PASS, installer.Tildify(configPath)
}

func isKnownBinaryPath(command string) bool {
	for _, candidate := range installer.GetBinaryCandidates() {
		if command == candidate {
			return true
		}
	}
	return false
}

func CheckMCPStartup() []CheckResult {
	binaryPath := installer.BinaryPath()
	if filepath.Base(binaryPath) == filepath.Base(installer.DefaultBinaryPath()) {
		binaryPath = installer.DefaultBinaryPath()
	}

	if !installer.Exists(binaryPath) {
		// Try fallback
		if p, err := exec.LookPath(filepath.Base(binaryPath)); err == nil {
			binaryPath = p
		} else {
			return []CheckResult{{
				Name:   "MCP Startup",
				Status: FAIL,
				Detail: fmt.Sprintf("Binary not found at %s or in PATH", installer.Tildify(binaryPath)),
			}}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "serve")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return []CheckResult{{
			Name:   "MCP Startup",
			Status: FAIL,
			Detail: fmt.Sprintf("Cannot create stdin pipe: %s", err),
		}}
	}

	stdoutReader, stdoutWriter := io.Pipe()
	var stderrBuf strings.Builder
	cmd.Stdout = stdoutWriter
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return []CheckResult{{
			Name:   "MCP Startup",
			Status: FAIL,
			Detail: fmt.Sprintf("Cannot start MCP server: %s", err),
		}}
	}

	// MCP uses Content-Length framing: "Content-Length: N\r\n\r\n{...}"
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"doctor","version":"1.0.0"}}}`
	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
	fmt.Fprint(stdin, frame)

	responded := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(stdoutReader)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, `"jsonrpc"`) || strings.Contains(line, `"result"`) {
				responded <- true
				return
			}
		}
	}()

	select {
	case <-responded:
		stdin.Close()
		cmd.Process.Kill()
		cmd.Wait()
		return []CheckResult{{
			Name:   "MCP Startup",
			Status: PASS,
			Detail: "Server responds to JSON-RPC initialize",
		}}
	case <-time.After(3 * time.Second):
		stdin.Close()
		cmd.Process.Kill()
		cmd.Wait()
		stderr := strings.TrimSpace(stderrBuf.String())
		if stderr != "" {
			return []CheckResult{{
				Name:   "MCP Startup",
				Status: FAIL,
				Detail: fmt.Sprintf("Server did not respond. Stderr: %s", stderr),
			}}
		}
		return []CheckResult{{
			Name:   "MCP Startup",
			Status: FAIL,
			Detail: "Server started but did not respond to initialize in 3s",
		}}
	}
}

func CheckRepository() []CheckResult {
	var results []CheckResult

	cwd, err := os.Getwd()
	if err != nil {
		return []CheckResult{{
			Name:   "Repository",
			Status: FAIL,
			Detail: fmt.Sprintf("Cannot get working directory: %s", err),
		}}
	}

	if _, err := os.Stat(cwd); err != nil {
		return []CheckResult{{
			Name:   "Repository",
			Status: FAIL,
			Detail: fmt.Sprintf("Cannot read current directory: %s", err),
		}}
	}

	results = append(results, CheckResult{
		Name:   "Repository",
		Status: PASS,
		Detail: cwd,
	})

	indexDir := filepath.Join(cwd, ".mycli-fts")
	if installer.Exists(indexDir) {
		if installer.IsDirWritable(indexDir) {
			results = append(results, CheckResult{
				Name:   "Index Directory",
				Status: PASS,
				Detail: installer.Tildify(indexDir),
			})
		} else {
			results = append(results, CheckResult{
				Name:   "Index Directory",
				Status: FAIL,
				Detail: fmt.Sprintf("Index directory %s is not writable", installer.Tildify(indexDir)),
			})
		}
	} else {
		if installer.IsDirWritable(cwd) {
			results = append(results, CheckResult{
				Name:   "Index Directory",
				Status: PASS,
				Detail: fmt.Sprintf("%s (will be created on first index)", installer.Tildify(indexDir)),
			})
		} else {
			results = append(results, CheckResult{
				Name:   "Index Directory",
				Status: FAIL,
				Detail: fmt.Sprintf("Cannot create index in %s (not writable)", cwd),
			})
		}
	}

	return results
}

func FinalStatus(results []CheckResult) (Status, int, int) {
	passCount := 0
	failCount := 0
	for _, r := range results {
		switch r.Status {
		case PASS:
			passCount++
		case FAIL:
			failCount++
		}
	}
	if failCount > 0 {
		return FAIL, passCount, failCount
	}
	return PASS, passCount, failCount
}
