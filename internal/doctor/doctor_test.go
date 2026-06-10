package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/okyashgajjar/costaffective-mcp/internal/installer"
	_ "github.com/okyashgajjar/costaffective-mcp/internal/installer/targets"
)

func TestDoctorBinaryCheck_PASS(t *testing.T) {
	binPath := buildTempBinary(t)
	installer.SetBinaryPath(binPath)
	defer installer.SetBinaryPath("")

	results := CheckBinary()
	passCount := 0
	failCount := 0
	for _, r := range results {
		if r.Status == PASS {
			passCount++
		}
		if r.Status == FAIL {
			failCount++
		}
	}

	if failCount > 0 {
		t.Fatalf("expected 0 FAIL, got %d. Results: %+v", failCount, results)
	}
	if passCount == 0 {
		t.Fatal("expected at least 1 PASS")
	}
}

func TestDoctorBinaryCheck_verifyNonexistent(t *testing.T) {
	// Test that VerifyBinary produces proper error for nonexistent path
	err := installer.VerifyBinary("/nonexistent/costaffective")
	if err == nil {
		t.Fatal("VerifyBinary should fail for nonexistent path")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error should mention 'not found': %v", err)
	}

	// Test that an installer.ActionableError is returned
	var actionable installer.ActionableError
	if !asActionable(err, &actionable) {
		t.Fatalf("error should be ActionableError: %T %v", err, err)
	}
	if actionable.Action == "" {
		t.Fatal("ActionableError should have non-empty Action")
	}
}

func asActionable(err error, target *installer.ActionableError) bool {
	a, ok := err.(installer.ActionableError)
	if ok {
		*target = a
		return true
	}
	// Check through fmt.Errorf wrapping
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return asActionable(wrapped.Unwrap(), target)
	}
	return false
}

func TestDoctorPATH(t *testing.T) {
	results := CheckPATH()
	if len(results) == 0 {
		t.Fatal("expected at least 1 result from CheckPATH")
	}
}

func TestDoctorMCPConfigs(t *testing.T) {
	binPath := buildTempBinary(t)
	installer.SetBinaryPath(binPath)
	defer installer.SetBinaryPath("")

	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	// Install binary to the default path so config validation passes
	defaultBin := installer.DefaultBinaryPath()
	if err := os.MkdirAll(filepath.Dir(defaultBin), 0755); err != nil {
		t.Fatalf("mkdir all: %v", err)
	}
	data, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if err := os.WriteFile(defaultBin, data, 0755); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cursorDir := filepath.Join(dir, ".cursor")
	if err := os.MkdirAll(cursorDir, 0755); err != nil {
		t.Fatalf("mkdir all cursor: %v", err)
	}
	cursorConfig := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"costaffective": map[string]interface{}{
				"command": defaultBin,
				"args":    []string{"serve"},
				"type":    "stdio",
			},
		},
	}
	cursorFile := filepath.Join(cursorDir, "mcp.json")
	if err := installer.WriteJSONFile(cursorFile, cursorConfig); err != nil {
		t.Fatalf("write json file: %v", err)
	}

	results := CheckMCPConfigs()
	foundCursor := false
	for _, r := range results {
		if strings.Contains(r.Name, "Cursor") {
			foundCursor = true
			if r.Status != PASS {
				t.Fatalf("Cursor config should PASS, got %s: %s", r.Status, r.Detail)
			}
			break
		}
	}
	if !foundCursor {
		for _, r := range results {
			t.Logf("result: %s %s", r.Status, r.Name)
		}
		t.Fatal("expected Cursor config check result — targets may not be registered")
	}
}

func TestDoctorCodexRepairRoundTrip(t *testing.T) {
	binPath := buildTempBinary(t)

	homeDir := t.TempDir()
	origHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", homeDir); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	defer os.Setenv("HOME", origHome)

	defaultBin := installer.DefaultBinaryPath()
	if err := os.MkdirAll(filepath.Dir(defaultBin), 0755); err != nil {
		t.Fatalf("mkdir default bin dir: %v", err)
	}

	data, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("read temp binary: %v", err)
	}
	if err := os.WriteFile(defaultBin, data, 0755); err != nil {
		t.Fatalf("write default binary: %v", err)
	}

	installer.SetBinaryPath(defaultBin)
	defer installer.SetBinaryPath("")

	target := installer.GetTarget("codex")
	if target == nil {
		t.Fatal("expected codex target to be registered")
	}

	results := target.Install(installer.LocationGlobal, installer.InstallOptions{AutoAllow: true})
	if len(results) == 0 {
		t.Fatal("expected codex install to write at least one file")
	}

	codexFile := filepath.Join(homeDir, ".codex", "config.toml")
	content, err := os.ReadFile(codexFile)
	if err != nil {
		t.Fatalf("read codex config: %v", err)
	}
	if !strings.Contains(string(content), "[mcp_servers.costaffective]") {
		t.Fatalf("codex config missing MCP entry: %s", string(content))
	}
	if !strings.Contains(string(content), defaultBin) {
		t.Fatalf("codex config should reference installed binary %q: %s", defaultBin, string(content))
	}

	found := false
	for _, r := range CheckMCPConfigs() {
		if strings.Contains(r.Name, "Codex") {
			found = true
			if r.Status != PASS {
				t.Fatalf("Codex config should PASS after install, got %s: %s", r.Status, r.Detail)
			}
			break
		}
	}
	if !found {
		t.Fatal("expected Codex config check result")
	}
}

func TestDoctorRepository(t *testing.T) {
	results := CheckRepository()
	hasPass := false
	for _, r := range results {
		if r.Status == PASS {
			hasPass = true
		}
	}
	if !hasPass {
		t.Fatal("expected at least 1 PASS from Repository check")
	}
}

func TestDoctorRunAll(t *testing.T) {
	binPath := buildTempBinary(t)
	installer.SetBinaryPath(binPath)
	defer installer.SetBinaryPath("")

	results := RunAll()
	if len(results) == 0 {
		t.Fatal("RunAll should return at least 1 check")
	}
}

func TestDoctorFinalStatus(t *testing.T) {
	allPass := []CheckResult{
		{Name: "Test1", Status: PASS},
		{Name: "Test2", Status: PASS},
	}
	status, pass, fail := FinalStatus(allPass)
	if status != PASS || pass != 2 || fail != 0 {
		t.Fatalf("all pass: status=%s pass=%d fail=%d", status, pass, fail)
	}

	mixed := []CheckResult{
		{Name: "Test1", Status: PASS},
		{Name: "Test2", Status: FAIL},
	}
	status, pass, fail = FinalStatus(mixed)
	if status != FAIL || pass != 1 || fail != 1 {
		t.Fatalf("mixed: status=%s pass=%d fail=%d", status, pass, fail)
	}
}

func buildTempBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, binaryNameForTest())

	cmd := exec.Command("go", "build", "-o", out, "../../cmd/costaffective/")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build temp binary: %v", err)
	}
	return out
}

func binaryNameForTest() string {
	if runtime.GOOS == "windows" {
		return "costaffective_test.exe"
	}
	return "costaffective_test"
}
